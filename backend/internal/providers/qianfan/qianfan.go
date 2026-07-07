// Package qianfan 实现百度千帆（qianfan.baidubce.com/v2）渠道的 Provider。
//
// 千帆 v2 接口的 SSE 格式与 OpenAI 基本兼容，但有如下差异：
//   - usage 与 finish_reason 出现在同一个末尾 chunk（而非独立的空 choices chunk）；
//   - choices 元素多一个 flag 字段（忽略）；
//   - delta.content 在仅有推理内容时为 null；
//   - usage 可能携带 completion_tokens_details.reasoning_tokens（推理 token 明细）。
package qianfan

import (
	"encoding/json"

	"icode.baidu.com/baidu/blockchain/llm-studio/backend/internal/llm"
)

// Provider 是千帆渠道的实现。
type Provider struct{}

// chunk 是千帆 SSE data 的反序列化结构。
// usage 与 finish_reason 出现在同一个末尾 chunk 中。
type chunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		Flag         int     `json:"flag"` // 千帆特有，忽略
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details,omitempty"`
		CompletionTokensDetails *struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details,omitempty"`
	} `json:"usage"`
}

// BuildRequest 对千帆渠道无需额外修改请求体。
func (Provider) BuildRequest(_ *llm.ChatRequest) {}

// ParseChunk 解析一行千帆 SSE data。
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
		if c.Usage.CompletionTokensDetails != nil {
			usage.CompletionTokensDetails = &llm.TokenDetails{
				ReasoningTokens: c.Usage.CompletionTokensDetails.ReasoningTokens,
			}
		}
	}
	return
}
