// Package llm 封装 OpenAI 兼容的大模型请求服务。
// 通过 /chat/completions 接口以 SSE 流式方式与模型对话。
package llm

import "time"

// Message 表示一条对话消息。
type Message struct {
	Role    string `json:"role"` // system / user / assistant
	Content string `json:"content"`
}

// ChatRequest 是 OpenAI 兼容 /chat/completions 的请求体。
type ChatRequest struct {
	Model         string         `json:"model"`
	Messages      []Message      `json:"messages"`
	Temperature   *float64       `json:"temperature,omitempty"`
	Stream        bool           `json:"stream"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
	// Thinking 控制推理/思维链输出（infini-ai cloud 等渠道使用对象形式）。
	// nil 表示不传、使用模型默认；{type:"disabled"} 关闭思维链，模型直接输出正文；
	// {type:"enabled"} 开启思维链，结果中 ReasoningContent 有值。
	Thinking *ThinkingConfig `json:"thinking,omitempty"`
}

// ThinkingConfig 是 thinking 字段的对象形式，与 infini-ai cloud 的接口一致。
// Type 取 "enabled"（开启思维链）或 "disabled"（关闭思维链）。
type ThinkingConfig struct {
	Type string `json:"type"`
}

// StreamOptions 控制流式响应的附加选项。
type StreamOptions struct {
	// IncludeUsage 为 true 时，服务端会在最后一个 chunk 返回 usage 统计。
	IncludeUsage bool `json:"include_usage"`
}

// ChatChunk 是流式响应中的单个 SSE 数据块。
// 对非流式响应，Choices[].Message 有值；对流式响应，Choices[].Delta 有值。
type ChatChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice 是模型的一个候选回复。
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message,omitempty"`
	Delta        Delta   `json:"delta,omitempty"`
	FinishReason *string `json:"finish_reason"`
}

// Delta 是流式响应中的增量内容。
type Delta struct {
	Role             string `json:"role,omitempty"`
	Content          string `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// Usage 是 token 用量统计（仅在 IncludeUsage 打开时由服务端返回）。
type Usage struct {
	PromptTokens        int           `json:"prompt_tokens"`
	CompletionTokens    int           `json:"completion_tokens"`
	TotalTokens         int           `json:"total_tokens"`
	PromptTokensDetails *TokenDetails `json:"prompt_tokens_details,omitempty"`
	// CompletionTokensDetails 记录 completion token 的细分用量
	//（如千帆返回的 reasoning_tokens）。
	CompletionTokensDetails *TokenDetails `json:"completion_tokens_details,omitempty"`
}

// TokenDetails 记录 token 的细分用量。
// 用于 prompt 时填充 CachedTokens；用于 completion 时填充 ReasoningTokens。
type TokenDetails struct {
	CachedTokens    int `json:"cached_tokens,omitempty"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
}

// ChatResult 是一次流式对话的汇总结果，由 StreamChat 累积填充。
// OpenAI 流式协议中 finish_reason 与 usage 位于不同的 chunk（最后的 usage
// chunk 其 choices 为空数组），本结构将它们合并，便于调用方一次获取完整返回信息。
type ChatResult struct {
	Model            string // 实际返回的模型名
	Content          string // 累积拼接的完整回复内容
	ReasoningContent string // 累积拼接的思维链内容（thinking）
	FinishReason     string // 停止原因（stop / length / tool_calls 等），无则为空
	Usage            *Usage // token 用量，仅服务端返回时有值

	// 以下为客户端视角的性能时间点，由 StreamChat 自动记录。
	// 配合调用方记录的“程序启动时刻”，可构成完整时间线：
	// 程序启动 → StartedAt → FirstTokenAt → EndedAt。
	StartedAt    time.Time // 请求发起时刻（httpClient.Do 调用前）
	FirstTokenAt time.Time // 首个内容 token 到达时刻；无内容 token 时为零值
	EndedAt      time.Time // 流结束时刻
}
