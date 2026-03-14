package inworld

import (
	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
)

const inworldBaseURL = "https://api.inworld.ai/v1"

// NewLLMClient returns an OpenAI-compatible client for Inworld router API.
func NewLLMClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("INWORLD_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = inworldBaseURL
	return openai.NewClientWithConfig(cfg)
}
