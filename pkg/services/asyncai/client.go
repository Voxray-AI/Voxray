package asyncai

import (
	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
)

func getBaseURL() string {
	return config.GetEnv("ASYNC_AI_BASE_URL", "https://api.async.ai/v1")
}

// NewClient returns an OpenAI-compatible client for AsyncAI.
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("ASYNC_AI_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = getBaseURL()
	return openai.NewClientWithConfig(cfg)
}
