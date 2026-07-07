package nange_test

import (
	"testing"

	"github.com/SmilingXinyi/go-llm-speedtest/backend/internal/providers/nange"
)

var p nange.Provider

func TestParseChunk_content(t *testing.T) {
	data := []byte(`{"choices":[{"delta":{"content":"Hello","role":"assistant"},"finish_reason":null,"index":0}],"model":"deepseek-v4-flash","usage":null}`)
	content, _, finishReason, usage, err := p.ParseChunk(data)
	if err != nil {
		t.Fatal(err)
	}
	if content != "Hello" {
		t.Errorf("content = %q, want Hello", content)
	}
	if finishReason != "" {
		t.Errorf("finishReason = %q, want empty", finishReason)
	}
	if usage != nil {
		t.Errorf("usage = %+v, want nil", usage)
	}
}

func TestParseChunk_finishReason(t *testing.T) {
	data := []byte(`{"choices":[{"delta":{"content":""},"finish_reason":"stop","index":0}],"usage":null}`)
	_, _, finishReason, _, err := p.ParseChunk(data)
	if err != nil {
		t.Fatal(err)
	}
	if finishReason != "stop" {
		t.Errorf("finishReason = %q, want stop", finishReason)
	}
}

func TestParseChunk_usageWithCachedTokens(t *testing.T) {
	data := []byte(`{"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":9,"total_tokens":14,"prompt_tokens_details":{"cached_tokens":3}}}`)
	_, _, _, usage, err := p.ParseChunk(data)
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil {
		t.Fatal("usage is nil")
	}
	if usage.TotalTokens != 14 {
		t.Errorf("total_tokens = %d, want 14", usage.TotalTokens)
	}
	if usage.PromptTokensDetails == nil || usage.PromptTokensDetails.CachedTokens != 3 {
		t.Errorf("cached_tokens = %+v, want 3", usage.PromptTokensDetails)
	}
}

func TestParseChunk_invalidJSON(t *testing.T) {
	_, _, _, _, err := p.ParseChunk([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
