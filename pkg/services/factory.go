// Package services defines interfaces and implementations for LLM, STT, and TTS.
// Use the factory functions (NewLLMFromConfig, NewSTTFromConfig, NewTTSFromConfig) to construct
// services by provider name; see Supported*Providers for capability matrix. For RealtimeService
// use realtime.NewFromConfig(cfg, provider) to avoid an import cycle.
package services

import (
	"context"

	"voxray-go/pkg/config"
	"voxray-go/pkg/services/anthropic"
	"voxray-go/pkg/services/asyncai"
	"voxray-go/pkg/services/aws"
	"voxray-go/pkg/services/camb"
	"voxray-go/pkg/services/cerebras"
	"voxray-go/pkg/services/deepseek"
	"voxray-go/pkg/services/elevenlabs"
	"voxray-go/pkg/services/fish"
	"voxray-go/pkg/services/google"
	"voxray-go/pkg/services/gradium"
	"voxray-go/pkg/services/grok"
	"voxray-go/pkg/services/groq"
	"voxray-go/pkg/services/hume"
	"voxray-go/pkg/services/inworld"
	"voxray-go/pkg/services/mistral"
	"voxray-go/pkg/services/minimax"
	"voxray-go/pkg/services/moondream"
	"voxray-go/pkg/services/neuphonic"
	"voxray-go/pkg/services/ollama"
	"voxray-go/pkg/services/openai"
	"voxray-go/pkg/services/openpipe"
	"voxray-go/pkg/services/qwen"
	"voxray-go/pkg/services/sarvam"
	"voxray-go/pkg/services/soniox"
	"voxray-go/pkg/services/stt"
	"voxray-go/pkg/services/tts"
	"voxray-go/pkg/services/whisper"
	"voxray-go/pkg/services/xtts"
)

const (
	ProviderOpenAI       = "openai"
	ProviderGroq         = "groq"
	ProviderSarvam       = "sarvam"
	ProviderGrok         = "grok"
	ProviderCerebras     = "cerebras"
	ProviderElevenLabs   = "elevenlabs"
	ProviderAWS          = "aws"
	ProviderMistral      = "mistral"
	ProviderDeepSeek     = "deepseek"
	ProviderAnthropic    = "anthropic"
	ProviderGoogle       = "google"
	ProviderGoogleVertex = "google_vertex"
	ProviderOllama       = "ollama"
	ProviderQwen         = "qwen"
	ProviderWhisper      = "whisper"
	// Pipecat-integrated providers
	ProviderAsyncAI   = "asyncai"
	ProviderCamb      = "camb"
	ProviderFish      = "fish"
	ProviderGradium   = "gradium"
	ProviderHume       = "hume"
	ProviderInworld   = "inworld"
	ProviderMinimax   = "minimax"
	ProviderMoondream = "moondream"
	ProviderNeuphonic = "neuphonic"
	ProviderOpenPipe  = "openpipe"
	ProviderSoniox    = "soniox"
	ProviderXTTS      = "xtts"
)

// SupportedLLMProviders lists provider keys that can be passed to NewLLMFromConfig.
var SupportedLLMProviders = []string{
	ProviderOpenAI,
	ProviderGroq,
	ProviderGrok,
	ProviderCerebras,
	ProviderAWS,
	ProviderMistral,
	ProviderDeepSeek,
	ProviderAnthropic,
	ProviderGoogle,
	ProviderGoogleVertex,
	ProviderOllama,
	ProviderQwen,
	ProviderAsyncAI,
	ProviderFish,
	ProviderInworld,
	ProviderMinimax,
	ProviderMoondream,
	ProviderOpenPipe,
}

// SupportedSTTProviders lists provider keys that can be passed to NewSTTFromConfig.
var SupportedSTTProviders = []string{
	ProviderOpenAI, ProviderGroq, ProviderSarvam, ProviderElevenLabs, ProviderAWS, ProviderGoogle, ProviderWhisper,
	ProviderCamb, ProviderGradium, ProviderSoniox,
}

// SupportedTTSProviders lists provider keys that can be passed to NewTTSFromConfig.
var SupportedTTSProviders = []string{
	ProviderOpenAI, ProviderGroq, ProviderSarvam, ProviderElevenLabs, ProviderAWS, ProviderGoogle,
	ProviderHume, ProviderInworld, ProviderMinimax, ProviderNeuphonic, ProviderXTTS,
}

// SupportedRealtimeProviders lists provider keys for realtime (use realtime.NewFromConfig to construct).
var SupportedRealtimeProviders = []string{ProviderOpenAI, ProviderHume, ProviderInworld}

// apiKeyForProvider returns the API key for the given provider (e.g. openai -> OPENAI_API_KEY, groq -> GROQ_API_KEY).
func apiKeyForProvider(cfg *config.Config, provider string) string {
	switch provider {
	case ProviderAnthropic:
		return cfg.GetAPIKey("anthropic", "ANTHROPIC_API_KEY")
	case ProviderGroq:
		return cfg.GetAPIKey("groq", "GROQ_API_KEY")
	case ProviderSarvam:
		return cfg.GetAPIKey("sarvam", "SARVAM_API_KEY")
	case ProviderGrok:
		return cfg.GetAPIKey("xai", "XAI_API_KEY")
	case ProviderCerebras:
		return cfg.GetAPIKey("cerebras", "CEREBRAS_API_KEY")
	case ProviderElevenLabs:
		return cfg.GetAPIKey("elevenlabs", "ELEVENLABS_API_KEY")
	case ProviderAWS:
		return cfg.GetAPIKey("aws", "AWS_SECRET_ACCESS_KEY")
	case ProviderMistral:
		return cfg.GetAPIKey("mistral", "MISTRAL_API_KEY")
	case ProviderDeepSeek:
		return cfg.GetAPIKey("deepseek", "DEEPSEEK_API_KEY")
	case ProviderOpenAI:
		return cfg.GetAPIKey("openai", "OPENAI_API_KEY")
	case ProviderGoogle:
		return cfg.GetAPIKey("google", "GOOGLE_API_KEY")
	case ProviderGoogleVertex:
		return "" // Vertex uses ADC, no API key
	case ProviderOllama:
		return cfg.GetAPIKey("ollama", "OLLAMA_API_KEY")
	case ProviderQwen:
		key := cfg.GetAPIKey("qwen", "DASHSCOPE_API_KEY")
		if key == "" {
			key = cfg.GetAPIKey("qwen", "QWEN_API_KEY")
		}
		return key
	case ProviderWhisper:
		key := cfg.GetAPIKey("whisper", "WHISPER_API_KEY")
		if key == "" {
			key = cfg.GetAPIKey("openai", "OPENAI_API_KEY")
		}
		return key
	case ProviderAsyncAI:
		return cfg.GetAPIKey("asyncai", "ASYNC_AI_API_KEY")
	case ProviderCamb:
		return cfg.GetAPIKey("camb", "CAMB_API_KEY")
	case ProviderFish:
		return cfg.GetAPIKey("fish", "FISH_API_KEY")
	case ProviderGradium:
		return cfg.GetAPIKey("gradium", "GRADIUM_API_KEY")
	case ProviderHume:
		return cfg.GetAPIKey("hume", "HUME_API_KEY")
	case ProviderInworld:
		return cfg.GetAPIKey("inworld", "INWORLD_API_KEY")
	case ProviderMinimax:
		return cfg.GetAPIKey("minimax", "MINIMAX_API_KEY")
	case ProviderMoondream:
		return cfg.GetAPIKey("moondream", "MOONDREAM_API_KEY")
	case ProviderNeuphonic:
		return cfg.GetAPIKey("neuphonic", "NEUPHONIC_API_KEY")
	case ProviderOpenPipe:
		return cfg.GetAPIKey("openpipe", "OPENPIPE_API_KEY")
	case ProviderSoniox:
		return cfg.GetAPIKey("soniox", "SONIOX_API_KEY")
	case ProviderXTTS:
		return cfg.GetAPIKey("xtts", "XTTS_API_KEY")
	default:
		return cfg.GetAPIKey(provider, "OPENAI_API_KEY")
	}
}

func getGoogleProject(cfg *config.Config) string {
	p := cfg.GetAPIKey("google_cloud_project", "GOOGLE_CLOUD_PROJECT")
	if p == "" {
		p = cfg.GetAPIKey("google_project", "GOOGLE_CLOUD_PROJECT")
	}
	return p
}

func getGoogleLocation(cfg *config.Config) string {
	l := cfg.GetAPIKey("google_cloud_location", "GOOGLE_CLOUD_LOCATION")
	if l == "" {
		l = cfg.GetAPIKey("google_location", "GOOGLE_CLOUD_LOCATION")
	}
	if l == "" {
		return "us-central1"
	}
	return l
}

func getAWSRegion(cfg *config.Config) string {
	r := cfg.GetAPIKey("aws_region", "AWS_REGION")
	if r == "" {
		return "us-east-1"
	}
	return r
}

// NewLLMFromConfig returns an LLMService for the given provider and model.
// Provider must be one of SupportedLLMProviders; model is the chat model (e.g. cfg.Model).
func NewLLMFromConfig(cfg *config.Config, provider, model string) LLMService {
	apiKey := apiKeyForProvider(cfg, provider)
	switch provider {
	case ProviderAnthropic:
		return anthropic.NewLLMService(apiKey, model)
	case ProviderGroq:
		return groq.NewLLMService(apiKey, model)
	case ProviderGrok:
		return grok.NewLLMService(apiKey, model)
	case ProviderCerebras:
		return cerebras.NewLLMService(apiKey, model)
	case ProviderAWS:
		svc, err := aws.NewLLMWithRegion(context.Background(), getAWSRegion(cfg), model)
		if err != nil {
			return nil
		}
		return svc
	case ProviderMistral:
		return mistral.NewLLMService(apiKey, model)
	case ProviderDeepSeek:
		return deepseek.NewLLMService(apiKey, model)
	case ProviderGoogle:
		svc, err := google.NewLLMService(context.Background(), apiKey, model)
		if err != nil {
			return nil
		}
		return svc
	case ProviderGoogleVertex:
		svc, err := google.NewVertexLLMService(context.Background(), getGoogleProject(cfg), getGoogleLocation(cfg), model)
		if err != nil {
			return nil
		}
		return svc
	case ProviderOllama:
		return ollama.NewLLMService(apiKey, model)
	case ProviderQwen:
		return qwen.NewLLMService(apiKey, model)
	case ProviderOpenPipe:
		return openpipe.NewLLMService(apiKey, model)
	case ProviderAsyncAI:
		return asyncai.NewLLMService(apiKey, model)
	case ProviderFish:
		return fish.NewLLMService(apiKey, model)
	case ProviderMoondream:
		return moondream.NewLLMService(apiKey, model)
	case ProviderMinimax:
		return minimax.NewLLMService(apiKey, model)
	case ProviderInworld:
		return inworld.NewLLMService(apiKey, model)
	case ProviderOpenAI:
		fallthrough
	default:
		if model == "" {
			model = "gpt-3.5-turbo"
		}
		return openai.NewService(apiKey, model)
	}
}

// NewSTTFromConfig returns an STTService for the given provider.
// Provider must be one of SupportedSTTProviders; cfg.STTModel is used when supported (e.g. Groq).
func NewSTTFromConfig(cfg *config.Config, provider string) STTService {
	apiKey := apiKeyForProvider(cfg, provider)
	switch provider {
	case ProviderGroq:
		return stt.NewGroqWithModel(apiKey, cfg.STTModel)
	case ProviderSarvam:
		return sarvam.NewSTTWithLanguage(apiKey, cfg.STTModel, cfg.STTLanguage)
	case ProviderElevenLabs:
		return elevenlabs.NewSTT(apiKey, cfg.STTModel)
	case ProviderAWS:
		svc, err := aws.NewSTTWithRegion(context.Background(), getAWSRegion(cfg), "")
		if err != nil {
			return nil
		}
		return svc
	case ProviderGoogle:
		svc, err := google.NewSTT(context.Background(), getGoogleProject(cfg), getGoogleLocation(cfg), cfg.STTModel, cfg.STTLanguage)
		if err != nil {
			return nil
		}
		return svc
	case ProviderWhisper:
		return whisper.NewService(apiKey, config.GetEnv("WHISPER_BASE_URL", ""))
	case ProviderCamb:
		return camb.NewSTT(apiKey, config.GetEnv("CAMB_BASE_URL", ""), cfg.STTModel)
	case ProviderGradium:
		return gradium.NewSTT(apiKey, config.GetEnv("GRADIUM_BASE_URL", ""), cfg.STTModel)
	case ProviderSoniox:
		return soniox.NewSTT(apiKey, config.GetEnv("SONIOX_WS_URL", ""), cfg.STTModel)
	case ProviderOpenAI:
		fallthrough
	default:
		return stt.NewOpenAI(apiKey)
	}
}

// NewTTSFromConfig returns a TTSService for the given provider, model, and voice.
// Provider must be one of SupportedTTSProviders; model and voice are typically cfg.TTSModel and cfg.TTSVoice.
func NewTTSFromConfig(cfg *config.Config, provider, model, voice string) TTSService {
	apiKey := apiKeyForProvider(cfg, provider)
	switch provider {
	case ProviderGroq:
		return tts.NewGroq(apiKey, model, voice)
	case ProviderSarvam:
		return sarvam.NewTTS(apiKey, model, voice)
	case ProviderElevenLabs:
		return elevenlabs.NewTTS(apiKey, voice, model, "")
	case ProviderAWS:
		if voice == "" {
			voice = "Joanna"
		}
		svc, err := aws.NewTTSWithRegion(context.Background(), getAWSRegion(cfg), voice, "")
		if err != nil {
			return nil
		}
		return svc
	case ProviderGoogle:
		lang := cfg.STTLanguage
		if lang == "" {
			lang = "en-US"
		}
		svc, err := google.NewTTS(context.Background(), lang, voice)
		if err != nil {
			return nil
		}
		return svc
	case ProviderXTTS:
		baseURL := config.GetEnv("XTTS_BASE_URL", "")
		lang := cfg.STTLanguage
		if lang == "" {
			lang = "en"
		}
		return xtts.NewTTS(apiKey, baseURL, voice, lang)
	case ProviderHume:
		return hume.NewTTS(apiKey, voice)
	case ProviderNeuphonic:
		lang := cfg.STTLanguage
		if lang == "" {
			lang = "en"
		}
		return neuphonic.NewTTS(apiKey, config.GetEnv("NEUPHONIC_BASE_URL", ""), voice, lang, 1.0)
	case ProviderMinimax:
		if model == "" {
			model = "speech-01"
		}
		return minimax.NewTTS(apiKey, config.GetEnv("MINIMAX_BASE_URL", ""), model, voice)
	case ProviderInworld:
		return inworld.NewTTS(apiKey, voice)
	case ProviderOpenAI:
		fallthrough
	default:
		return tts.NewOpenAI(apiKey, model)
	}
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
