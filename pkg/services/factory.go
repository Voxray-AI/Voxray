// Package services defines interfaces and implementations for LLM, STT, and TTS.
package services

import (
	"voila-go/pkg/config"
	"voila-go/pkg/services/groq"
	"voila-go/pkg/services/openai"
	"voila-go/pkg/services/sarvam"
	"voila-go/pkg/services/stt"
	"voila-go/pkg/services/tts"
)

const (
	ProviderOpenAI = "openai"
	ProviderGroq   = "groq"
	ProviderSarvam = "sarvam"
)

// apiKeyForProvider returns the API key for the given provider (e.g. openai -> OPENAI_API_KEY, groq -> GROQ_API_KEY).
func apiKeyForProvider(cfg *config.Config, provider string) string {
	switch provider {
	case ProviderGroq:
		return cfg.GetAPIKey("groq", "GROQ_API_KEY")
	case ProviderSarvam:
		return cfg.GetAPIKey("sarvam", "SARVAM_API_KEY")
	case ProviderOpenAI:
		return cfg.GetAPIKey("openai", "OPENAI_API_KEY")
	default:
		return cfg.GetAPIKey(provider, "OPENAI_API_KEY")
	}
}

// NewLLMFromConfig returns an LLMService for the given provider and model.
// Provider is the resolved LLM provider; model is the chat model (e.g. cfg.Model).
func NewLLMFromConfig(cfg *config.Config, provider, model string) LLMService {
	apiKey := apiKeyForProvider(cfg, provider)
	if provider == ProviderGroq {
		return groq.NewLLMService(apiKey, model)
	}
	if model == "" {
		model = "gpt-3.5-turbo"
	}
	return openai.NewService(apiKey, model)
}

// NewSTTFromConfig returns an STTService for the given provider.
// Provider is the resolved STT provider; cfg.STTModel is used when supported (e.g. Groq).
func NewSTTFromConfig(cfg *config.Config, provider string) STTService {
	apiKey := apiKeyForProvider(cfg, provider)
	if provider == ProviderGroq {
		return stt.NewGroqWithModel(apiKey, cfg.STTModel)
	}
	if provider == ProviderSarvam {
		return sarvam.NewSTT(apiKey, cfg.STTModel)
	}
	return stt.NewOpenAI(apiKey)
}

// NewTTSFromConfig returns a TTSService for the given provider, model, and voice.
// Provider is the resolved TTS provider; model and voice are typically cfg.TTSModel and cfg.TTSVoice.
func NewTTSFromConfig(cfg *config.Config, provider, model, voice string) TTSService {
	apiKey := apiKeyForProvider(cfg, provider)
	if provider == ProviderGroq {
		return tts.NewGroq(apiKey, model, voice)
	}
	if provider == ProviderSarvam {
		return sarvam.NewTTS(apiKey, model, voice)
	}
	return tts.NewOpenAI(apiKey, model)
}

// NewServicesFromConfig returns LLM, STT, and TTS services based on cfg.
// Resolves provider per task (stt_provider/llm_provider/tts_provider or provider); uses task-specific model/voice when set.
func NewServicesFromConfig(cfg *config.Config) (LLMService, STTService, TTSService) {
	sttProvider := cfg.STTProvider()
	if sttProvider == "" {
		sttProvider = ProviderOpenAI
	}
	llmProvider := cfg.LLMProvider()
	if llmProvider == "" {
		llmProvider = ProviderOpenAI
	}
	ttsProvider := cfg.TTSProvider()
	if ttsProvider == "" {
		ttsProvider = ProviderOpenAI
	}
	model := cfg.Model
	if model == "" && llmProvider == ProviderOpenAI {
		model = "gpt-3.5-turbo"
	}
	llm := NewLLMFromConfig(cfg, llmProvider, model)
	sttSvc := NewSTTFromConfig(cfg, sttProvider)
	ttsSvc := NewTTSFromConfig(cfg, ttsProvider, cfg.TTSModel, cfg.TTSVoice)
	return llm, sttSvc, ttsSvc
}
