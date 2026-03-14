package fish

import (
	openai "github.com/sashabaranov/go-openai"
	"voxray-go/pkg/config"
)

func getBaseURL() string {
	return config.GetEnv("FISH_BASE_URL", "https://api.fish.audio/v1")
}

// NewClient returns an OpenAI-compatible client for Fish (Fish Audio).
func NewClient(apiKey string) *openai.Client {
	if apiKey == "" {
		apiKey = config.GetEnv("FISH_API_KEY", "")
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = getBaseURL()
	return openai.NewClientWithConfig(cfg)
}
