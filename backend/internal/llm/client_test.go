package llm

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 模拟一段标准的 OpenAI 兼容 SSE 流式响应。
const fakeSSEStream = `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"test-model","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","model":"test-model","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","model":"test-model","choices":[{"index":0,"delta":{"content":", world"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"chatcmpl-1","model":"test-model","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}

data: [DONE]

`

func TestStreamChat(t *testing.T) {
	var gotPath, gotAuth, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, fakeSSEStream)
	}))
	defer srv.Close()

	client := New(srv.URL, "sk-test", "test-model")

	var sb strings.Builder
	result, err := client.StreamChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil, func(content string) {
		sb.WriteString(content)
	})
	if err != nil {
		t.Fatalf("StreamChat 返回错误: %v", err)
	}

	// 请求侧：路径、方法、鉴权头
	if gotMethod != http.MethodPost {
		t.Errorf("请求方法 = %q, 期望 POST", gotMethod)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("请求路径 = %q, 期望 /chat/completions", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q, 期望 Bearer sk-test", gotAuth)
	}

	// onDelta 回调应收到完整增量内容
	if got := sb.String(); got != "Hello, world" {
		t.Errorf("onDelta 增量内容 = %q, 期望 \"Hello, world\"", got)
	}

	// 汇总结果：内容、finish_reason、usage 来自不同 chunk，应被合并
	if result == nil {
		t.Fatal("结果为 nil")
	}
	if result.Model != "test-model" {
		t.Errorf("model = %q, 期望 test-model", result.Model)
	}
	if result.Content != "Hello, world" {
		t.Errorf("content = %q, 期望 \"Hello, world\"", result.Content)
	}
	if result.FinishReason != "stop" {
		t.Errorf("finish_reason = %q, 期望 stop", result.FinishReason)
	}
	if result.Usage == nil || result.Usage.TotalTokens != 5 {
		t.Errorf("usage 不正确: %+v", result.Usage)
	}
	// 性能时间点应被记录且顺序合理（本地 mock 耗时极短）
	if result.StartedAt.IsZero() || result.FirstTokenAt.IsZero() || result.EndedAt.IsZero() {
		t.Errorf("时间点未记录: start=%v first=%v end=%v", result.StartedAt, result.FirstTokenAt, result.EndedAt)
	}
	if result.FirstTokenAt.Before(result.StartedAt) || result.EndedAt.Before(result.FirstTokenAt) {
		t.Errorf("时间点顺序异常: start=%v first=%v end=%v", result.StartedAt, result.FirstTokenAt, result.EndedAt)
	}
}

func TestStreamChatMissingConfig(t *testing.T) {
	cases := []struct {
		name    string
		client  *Client
		wantErr string
	}{
		{"缺 base url", New("", "k", "m"), "base url"},
		{"缺 api key", New("https://x", "", "m"), "api key"},
		{"缺 model", New("https://x", "k", ""), "model"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.client.StreamChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil, nil)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("期望错误包含 %q, 实际: %v", c.wantErr, err)
			}
		})
	}
}

// TestStreamChatDebugWriter 验证开启 DebugWriter 时，会输出完整的 HTTP 响应：
// 状态行、响应头以及每一行原始 SSE 数据（含 [DONE]）。
func TestStreamChatDebugWriter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Test-Header", "yes")
		_, _ = io.WriteString(w, fakeSSEStream)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	client := New(srv.URL, "sk-test", "test-model")
	client.DebugWriter = &buf

	if _, err := client.StreamChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, nil, nil); err != nil {
		t.Fatalf("StreamChat 返回错误: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"HTTP/",         // 状态行
		"200",           // 状态码
		"X-Test-Header", // 响应头
		`data: {"id"`,   // 原始 SSE 数据
		"Hello",         // 内容片段
		"[DONE]",        // 流结束标记
	} {
		if !strings.Contains(out, want) {
			t.Errorf("调试输出缺少 %q\n--- 完整输出 ---\n%s", want, out)
		}
	}
}
