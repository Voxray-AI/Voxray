// Package cerebras provides Cerebras inference API-backed LLM via OpenAI-compatible API.
package cerebras

import (
	"github.com/Voxray-AI/Voxray/pkg/config"
	openai "github.com/sashabaranov/go-openai"
)

const cerebrasBaseURL = "https://api.cerebras.ai/v1"

// NewClient returns an OpenAI-compatible client configured for Cerebras inference.
// If apiKey is empty, config.GetEnv("CEREBRAS_API_KEY", "") is used.
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("CEREBRAS_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = cerebrasBaseURL
	return openai.NewClientWithConfig(cfg)
}
