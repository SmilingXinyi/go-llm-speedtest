package qianfan_test

import (
	"testing"

	"github.com/SmilingXinyi/go-llm-speedtest/backend/internal/providers/qianfan"
)

var p qianfan.Provider

func TestParseChunk_content(t *testing.T) {
	data := []byte(`{"id":"as-1","object":"chat.completion.chunk","created":1782475415,"model":"qwen3-32b","choices":[{"index":0,"delta":{"content":"你好","role":"assistant"},"flag":0}]}`)
	content, reasoning, finishReason, usage, err := p.ParseChunk(data)
	if err != nil {
		t.Fatal(err)
	}
	if content != "你好" {
		t.Errorf("content = %q, want 你好", content)
	}
	if reasoning != "" {
		t.Errorf("reasoning = %q, want empty", reasoning)
	}
	if finishReason != "" {
		t.Errorf("finishReason = %q, want empty", finishReason)
	}
	if usage != nil {
		t.Errorf("usage = %+v, want nil", usage)
	}
}

func TestParseChunk_nullContent(t *testing.T) {
	// thinking 模式下 content 为 null，仅有 reasoning_content
	data := []byte(`{"id":"as-1","object":"chat.completion.chunk","created":1782475415,"model":"qwen3-32b","choices":[{"index":0,"delta":{"content":null,"reasoning_content":"好的"},"flag":0}]}`)
	content, reasoning, _, _, err := p.ParseChunk(data)
	if err != nil {
		t.Fatal(err)
	}
	if content != "" {
		t.Errorf("content = %q, want empty", content)
	}
	if reasoning != "好的" {
		t.Errorf("reasoning = %q, want 好的", reasoning)
	}
}

func TestParseChunk_finishReasonAndUsageInSameChunk(t *testing.T) {
	// 千帆的 usage 与 finish_reason 在同一个末尾 chunk
	data := []byte(`{"id":"as-1","object":"chat.completion.chunk","created":1782475415,"model":"qwen3-32b","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop","flag":0}],"usage":{"prompt_tokens":13,"completion_tokens":12,"total_tokens":25}}`)
	_, _, finishReason, usage, err := p.ParseChunk(data)
	if err != nil {
		t.Fatal(err)
	}
	if finishReason != "stop" {
		t.Errorf("finishReason = %q, want stop", finishReason)
	}
	if usage == nil {
		t.Fatal("usage is nil")
	}
	if usage.TotalTokens != 25 || usage.PromptTokens != 13 || usage.CompletionTokens != 12 {
		t.Errorf("usage = %+v", usage)
	}
}

func TestParseChunk_usageWithReasoningTokens(t *testing.T) {
	data := []byte(`{"id":"as-1","object":"chat.completion.chunk","created":1782475464,"model":"qwen3-32b","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop","flag":0}],"usage":{"prompt_tokens":17,"completion_tokens":253,"total_tokens":270,"completion_tokens_details":{"reasoning_tokens":245}}}`)
	_, _, _, usage, err := p.ParseChunk(data)
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil {
		t.Fatal("usage is nil")
	}
	if usage.CompletionTokensDetails == nil || usage.CompletionTokensDetails.ReasoningTokens != 245 {
		t.Errorf("reasoning_tokens = %+v, want 245", usage.CompletionTokensDetails)
	}
}

func TestParseChunk_invalidJSON(t *testing.T) {
	_, _, _, _, err := p.ParseChunk([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
