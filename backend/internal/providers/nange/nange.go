// Package nange 实现百度 nange 渠道（BLB 网关）的 Provider。
// SSE 格式与 OpenAI 兼容，额外支持 prompt_tokens_details.cached_tokens。
package nange

import (
	"encoding/json"

	"github.com/SmilingXinyi/go-llm-speedtest/backend/internal/llm"
)

// Provider 是 nange 渠道的实现。
type Provider struct{}

// chunk 是 nange SSE data 的反序列化结构。
// 与 OpenAI 格式一致，usage 独立出现在末尾 chunk（choices 为空数组）。
type chunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details,omitempty"`
	} `json:"usage"`
}

// BuildRequest 对 nange 渠道无需额外修改请求体。
func (Provider) BuildRequest(_ *llm.ChatRequest) {}

// ParseChunk 解析一行 nange SSE data。
func (Provider) ParseChunk(data []byte) (content, reasoning, finishReason string, usage *llm.Usage, err error) {
	var c chunk
	if err = json.Unmarshal(data, &c); err != nil {
		return
	}
	for _, ch := range c.Choices {
		content += ch.Delta.Content
		reasoning += ch.Delta.ReasoningContent
		if ch.FinishReason != nil && *ch.FinishReason != "" {
			finishReason = *ch.FinishReason
		}
	}
	if c.Usage != nil {
		usage = &llm.Usage{
			PromptTokens:     c.Usage.PromptTokens,
			CompletionTokens: c.Usage.CompletionTokens,
			TotalTokens:      c.Usage.TotalTokens,
		}
		if c.Usage.PromptTokensDetails != nil {
			usage.PromptTokensDetails = &llm.TokenDetails{
				CachedTokens: c.Usage.PromptTokensDetails.CachedTokens,
			}
		}
	}
	return
}
