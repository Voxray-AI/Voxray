package moondream

import (
	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
)

func getBaseURL() string {
	return config.GetEnv("MOONDREAM_BASE_URL", "https://api.moondream.com/v1")
}

// NewClient returns an OpenAI-compatible client for Moondream.
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("MOONDREAM_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = getBaseURL()
	return openai.NewClientWithConfig(cfg)
}
