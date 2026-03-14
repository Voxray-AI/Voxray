package minimax

import (
	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
)

func getBaseURL() string {
	return config.GetEnv("MINIMAX_BASE_URL", "https://api.minimax.io/v1")
}

// NewClient returns an OpenAI-compatible client for Minimax (use MINIMAX_BASE_URL for proxies or minimax-m2).
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("MINIMAX_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = getBaseURL()
	return openai.NewClientWithConfig(cfg)
}
