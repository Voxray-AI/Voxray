// Package openpipe provides OpenPipe-backed LLM via OpenAI-compatible API.
package openpipe

import (
	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
)

const openpipeBaseURL = "https://app.openpipe.ai/api/v1"

// NewClient returns an OpenAI-compatible client configured for OpenPipe.
// If apiKey is empty, config.GetEnv("OPENPIPE_API_KEY", "") is used.
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("OPENPIPE_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = openpipeBaseURL
	return openai.NewClientWithConfig(cfg)
}
