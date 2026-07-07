package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
)

// Provider 封装单个渠道的请求构建与 SSE chunk 解析逻辑。
type Provider interface {
	BuildRequest(req *ChatRequest)
	ParseChunk(data []byte) (content, reasoning, finishReason string, usage *Usage, err error)
}

// Client 是 OpenAI 兼容大模型服务的客户端。
type Client struct {
	BaseURL string       // 例如 https://api.example.com/v1 ，末尾的 / 会被去掉
	APIKey  string       // Bearer Token
	Model   string       // 默认使用的模型名
	HTTP    *http.Client // 为 nil 时使用默认 client
	// DebugWriter 非 nil 时，完整的 HTTP 响应（状态行、响应头、SSE 原始行）
	// 会被原样写入，便于查看服务端返回的完整内容。默认 nil 不输出。
	DebugWriter io.Writer
	// EnableThinking 控制推理/思维链输出（部分模型支持）。
	// nil 表示用模型默认；false 关闭思维链、直接输出正文；true 开启思维链。
	// 实际请求时会翻译为 thinking:{type:"enabled"|"disabled"} 对象形式发出。
	EnableThinking *bool
	// Provider 渠道实现；nil 时使用内置 OpenAI 兼容解析。
	Provider Provider
}

// New 创建一个使用默认 http.Client 的客户端。
func New(baseURL, apiKey, model string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{},
	}
}

// StreamChat 以流式方式发起一次对话。
// messages 为对话历史；temperature 传 nil 表示使用服务端默认。
// 每收到一段增量文本都会调用 onDelta（可能为 nil）。
// 返回累积汇总结果（含跨 chunk 合并的 finish_reason / usage 与性能时间点）。
// 若设置了 DebugWriter，完整的 HTTP 状态行、响应头与 SSE 原始数据会写入它。
func (c *Client) StreamChat(ctx context.Context, messages []Message, temperature *float64, onDelta func(content string)) (*ChatResult, error) {
	if c.BaseURL == "" {
		return nil, errors.New("llm: base url 未配置")
	}
	if c.APIKey == "" {
		return nil, errors.New("llm: api key 未配置")
	}
	if c.Model == "" {
		return nil, errors.New("llm: model 未配置")
	}

	reqBody := ChatRequest{
		Model:         c.Model,
		Messages:      messages,
		Stream:        true,
		StreamOptions: &StreamOptions{IncludeUsage: true},
	}
	if c.EnableThinking != nil {
		t := "disabled"
		if *c.EnableThinking {
			t = "enabled"
		}
		reqBody.Thinking = &ThinkingConfig{Type: t}
	}
	if temperature != nil {
		reqBody.Temperature = temperature
	}
	if c.Provider != nil {
		c.Provider.BuildRequest(&reqBody)
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("llm: 序列化请求失败: %w", err)
	}

	endpoint := c.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("llm: 构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// 请求发起时刻。
	result := &ChatResult{StartedAt: time.Now()}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 调试模式：先输出 HTTP 状态行与响应头（响应体在下方逐行输出）。
	if c.DebugWriter != nil {
		spew.Fdump(c.DebugWriter, resp.Proto, resp.Status, resp.Header)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if c.DebugWriter != nil {
			_, _ = c.DebugWriter.Write(body) // 输出完整的错误响应体
		}
		result.EndedAt = time.Now()
		return nil, fmt.Errorf("llm: 服务端返回非 200 状态码 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// OpenAI 流式协议中，finish_reason 与 usage 可能分布在不同的 chunk，
	// 这里边读边累积到 result，避免只保留最后一个 chunk 而丢失 finish_reason。
	result.Model = c.Model
	var readErr error
	reader := bufio.NewReaderSize(resp.Body, 64*1024)
	for {
		rawLine, err := reader.ReadString('\n')
		// 调试模式：逐行写入原始 SSE 数据（含换行），保留完整响应体。
		if c.DebugWriter != nil && rawLine != "" {
			_, _ = c.DebugWriter.Write([]byte(rawLine))
		}
		line := strings.TrimRight(rawLine, "\r\n")

		if line != "" && strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				break
			}
			if c.Provider != nil {
				content, reasoning, finishReason, usage, e := c.Provider.ParseChunk([]byte(data))
				if e == nil {
					if reasoning != "" {
						result.ReasoningContent += reasoning
					}
					if content != "" {
						if result.FirstTokenAt.IsZero() {
							result.FirstTokenAt = time.Now()
						}
						result.Content += content
						if onDelta != nil {
							onDelta(content)
						}
					}
					if finishReason != "" {
						result.FinishReason = finishReason
					}
					if usage != nil {
						result.Usage = usage
					}
				}
			} else {
				var chunk ChatChunk
				if e := json.Unmarshal([]byte(data), &chunk); e == nil {
					if chunk.Model != "" {
						result.Model = chunk.Model
					}
					for _, ch := range chunk.Choices {
						if ch.Delta.ReasoningContent != "" {
							result.ReasoningContent += ch.Delta.ReasoningContent
						}
						if ch.Delta.Content != "" {
							if result.FirstTokenAt.IsZero() {
								result.FirstTokenAt = time.Now() // 记录首 token 时刻
							}
							result.Content += ch.Delta.Content
							if onDelta != nil {
								onDelta(ch.Delta.Content)
							}
						}
						if ch.FinishReason != nil && *ch.FinishReason != "" {
							result.FinishReason = *ch.FinishReason
						}
					}
					if chunk.Usage != nil {
						result.Usage = chunk.Usage
					}
				}
			}
			// 单行 JSON 解析失败不致命（如服务端心跳/注释），跳过继续。
		}

		if err != nil {
			if err != io.EOF {
				readErr = err
			}
			break
		}
	}

	// 流结束时刻。
	result.EndedAt = time.Now()

	if readErr != nil {
		return result, fmt.Errorf("llm: 读取流失败: %w", readErr)
	}
	return result, nil
}
