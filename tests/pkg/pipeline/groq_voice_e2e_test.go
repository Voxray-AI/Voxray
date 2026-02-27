package pipeline_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"voila-go/pkg/config"
	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors"
	"voila-go/pkg/processors/voice"
	"voila-go/pkg/services"
)

// #region agent log
const debugLogPath = "debug-ac28aa.log"

func debugLog(location, message string, data map[string]interface{}, hypothesisId string) {
	payload := map[string]interface{}{
		"sessionId":     "ac28aa",
		"location":      location,
		"message":       message,
		"data":          data,
		"timestamp":     time.Now().UnixMilli(),
		"hypothesisId": hypothesisId,
	}
	j, _ := json.Marshal(payload)
	f, err := os.OpenFile(debugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	f.Write(append(j, '\n'))
	f.Close()
}

func saveTTSAudioAsWAV(path string, pcm []byte, sampleRate, numChannels int) error {
	if numChannels <= 0 {
		numChannels = 1
	}
	const bitsPerSample = 16
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := uint32(len(pcm))

	buf := make([]byte, 44+len(pcm))
	copy(buf[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(buf[4:8], 36+dataSize)
	copy(buf[8:12], []byte("WAVE"))
	copy(buf[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], uint16(numChannels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], bitsPerSample)
	copy(buf[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(buf[40:44], dataSize)
	copy(buf[44:], pcm)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}

// textLoggingProcessor is a small helper that logs specific frame types and forwards them downstream.
type textLoggingProcessor struct {
	*processors.BaseProcessor
	logFn func(f frames.Frame)
}

func newTextLoggingProcessor(name string, logFn func(f frames.Frame)) *textLoggingProcessor {
	return &textLoggingProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		logFn:         logFn,
	}
}

func (p *textLoggingProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	if p.logFn != nil {
		p.logFn(f)
	}
	return p.PushDownstream(ctx, f)
}

// sttInputLogger logs the raw audio heading into STT.
var sttInputLogger = newTextLoggingProcessor("stt-input-logger", func(f frames.Frame) {
	if af, ok := f.(*frames.AudioRawFrame); ok {
		debugLog(
			"groq_voice_e2e_test.go:sttInput",
			"STT input audio",
			map[string]interface{}{
				"audioLen":   len(af.Audio),
				"sampleRate": af.SampleRate,
				"channels":   af.NumChannels,
			},
			"H-STT-IN",
		)
	}
})

// sttOutputLLMInputLogger logs STT output text that will be used as LLM input.
var sttOutputLLMInputLogger = newTextLoggingProcessor("stt-output-llm-input-logger", func(f frames.Frame) {
	if tf, ok := f.(*frames.TranscriptionFrame); ok {
		text := tf.Text
		preview := text
		if len(preview) > 160 {
			preview = preview[:160] + "..."
		}
		logger.Info("LLM input (user): %q\n", text)
		debugLog(
			"groq_voice_e2e_test.go:llmInput",
			"LLM input from STT",
			map[string]interface{}{
				"text":      text,
				"preview":   preview,
				"finalized": tf.Finalized,
				"language":  tf.Language,
			},
			"H-LLM-IN",
		)
	}
})

// llmOutputTTSInputLogger logs LLM output text that will be sent to TTS.
var llmOutputTTSInputLogger = newTextLoggingProcessor("llm-output-tts-input-logger", func(f frames.Frame) {
	if tf, ok := f.(*frames.LLMTextFrame); ok {
		text := tf.Text
		preview := text
		if len(preview) > 160 {
			preview = preview[:160] + "..."
		}
		logger.Info("LLM output (assistant): %q\n", text)
		debugLog(
			"groq_voice_e2e_test.go:llmOutput",
			"LLM output to TTS",
			map[string]interface{}{
				"text":    text,
				"preview": preview,
			},
			"H-LLM-OUT",
		)
	}
})

// ttsOutputLogger logs synthesized audio coming out of TTS.
var ttsOutputLogger = newTextLoggingProcessor("tts-output-logger", func(f frames.Frame) {
	if af, ok := f.(*frames.TTSAudioRawFrame); ok {
		debugLog(
			"groq_voice_e2e_test.go:ttsOutput",
			"TTS output audio",
			map[string]interface{}{
				"audioLen":   len(af.Audio),
				"sampleRate": af.SampleRate,
			},
			"H-TTS-OUT",
		)
	}
})

// #endregion

// TestGroqVoicePipeline_E2E exercises an end-to-end voice pipeline:
// AudioRawFrame (hello.wav) -> Groq STT -> Groq LLM -> Groq TTS -> TTSAudioRawFrame.
func TestGroqVoicePipeline_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Groq voice E2E test in short mode")
	}

	// Locate the hello.wav fixture. Prefer the shared tests/testdata location,
	// but be resilient to the current working directory used by `go test`.
	candidatePaths := []string{
		filepath.Join("tests", "testdata", "hello.wav"),
		filepath.Join("..", "..", "testdata", "hello.wav"),
	}

	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			candidatePaths = append(candidatePaths,
				filepath.Join(dir, "tests", "testdata", "hello.wav"),
			)
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	var audioPath string
	for _, p := range candidatePaths {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		audioPath = p
		break
	}
	if audioPath == "" {
		t.Skipf("hello.wav fixture missing; add a small spoken-phrase WAV file at tests/testdata/hello.wav")
	}
	// #region agent log
	debugLog("groq_voice_e2e_test.go:audioPath", "audio fixture resolved", map[string]interface{}{"audioPath": audioPath}, "H1")
	// #endregion

	// Locate config.json starting from common relative locations and then
	// walking up parent directories.
	configPaths := []string{
		"config.json",
		filepath.Join("..", "..", "config.json"),
	}

	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			configPaths = append(configPaths,
				filepath.Join(dir, "config.json"),
			)
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	var (
		cfg *config.Config
		err error
	)
	for _, p := range configPaths {
		cfg, err = config.LoadConfig(p)
		if err == nil {
			break
		}
	}
	if cfg == nil {
		t.Skipf("config.json not available in expected locations: %v", err)
	}

	// Force Groq for all tasks and set task-specific models for this test.
	cfg.Provider = services.ProviderGroq
	cfg.SttProvider = services.ProviderGroq
	cfg.LlmProvider = services.ProviderGroq
	cfg.TtsProvider = services.ProviderGroq
	if cfg.Model == "" {
		cfg.Model = "llama-3.1-8b-instant"
	}
	if cfg.STTModel == "" {
		cfg.STTModel = "whisper-large-v3"
	}
	if cfg.TTSModel == "" {
		cfg.TTSModel = "canopylabs/orpheus-v1-english"
	}
	// Groq orpheus-v1-english only accepts: autumn, diana, hannah, austin, daniel, troy
	if cfg.TTSVoice == "" || cfg.TTSVoice == "alloy" {
		cfg.TTSVoice = "hannah"
	}
	// #region agent log
	debugLog("groq_voice_e2e_test.go:config", "TTS config for pipeline", map[string]interface{}{"TTSVoice": cfg.TTSVoice, "TTSModel": cfg.TTSModel}, "H2")
	// #endregion

	// Ensure we have a Groq API key either from config or environment.
	apiKey := cfg.GetAPIKey("groq", "GROQ_API_KEY")
	if apiKey == "" {
		t.Skip("GROQ_API_KEY not configured and no Groq key in config; skipping Groq voice E2E pipeline test")
	}
	if cfg.APIKeys == nil {
		cfg.APIKeys = map[string]string{}
	}
	cfg.APIKeys["groq"] = apiKey

	llmSvc, sttSvc, ttsSvc := services.NewServicesFromConfig(cfg)

	pl := pipeline.New()
	outCh := make(chan frames.Frame, 64)

	sttProc := voice.NewSTTProcessor("stt", sttSvc, 16000, 1)
	// LLM receives STT output (TranscriptionFrame) from the previous processor and replies; system prompt ensures assistant-style reply.
	llmProc := voice.NewLLMProcessorWithSystemPrompt("llm", llmSvc, "You are a helpful voice assistant. Reply briefly and naturally to the user.")
	ttsProc := voice.NewTTSProcessor("tts", ttsSvc, 24000)
	sink := pipeline.NewSink("sink", outCh)

	t.Logf("LLM model: %s", cfg.Model)
	// Pipeline: STT input logger → STT → STT output/LLM input logger → LLM → LLM output/TTS input logger → TTS → TTS output logger → sink.
	pl.Link(sttInputLogger, sttProc, sttOutputLLMInputLogger, llmProc, llmOutputTTSInputLogger, ttsProc, ttsOutputLogger, sink)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := pl.Setup(ctx); err != nil {
		t.Fatalf("pipeline setup failed: %v", err)
	}
	defer pl.Cleanup(ctx)

	if err := pl.Start(ctx, frames.NewStartFrame()); err != nil {
		t.Fatalf("pipeline start failed: %v", err)
	}

	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		t.Fatalf("reading audio fixture %s: %v", audioPath, err)
	}
	// #region agent log
	debugLog("groq_voice_e2e_test.go:audioData", "audio file read", map[string]interface{}{"lenBytes": len(audioData), "path": audioPath}, "H1")
	// #endregion

	audioFrame := frames.NewAudioRawFrame(audioData, 16000, 1, 0)
	if err := pl.Push(ctx, audioFrame); err != nil {
		t.Fatalf("pushing AudioRawFrame into pipeline: %v", err)
	}
	// #region agent log
	debugLog("groq_voice_e2e_test.go:push", "AudioRawFrame pushed", map[string]interface{}{"audioLen": len(audioData), "sampleRate": 16000, "channels": 1}, "H3")
	// #endregion

	// Wait for a TTSAudioRawFrame or fail if we only see errors / timeout.
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timeout waiting for TTSAudioRawFrame from Groq voice pipeline")
		case f := <-outCh:
			// #region agent log
			frameType := fmt.Sprintf("%T", f)
			debugLog("groq_voice_e2e_test.go:outCh", "frame received", map[string]interface{}{"frameType": frameType}, "H4")
			// #endregion
			switch v := f.(type) {
			case *frames.TTSAudioRawFrame:
				// #region agent log
				debugLog("groq_voice_e2e_test.go:TTSAudioRawFrame", "success", map[string]interface{}{"audioLen": len(v.Audio), "sampleRate": v.SampleRate}, "H4")
				// #endregion
				if len(v.Audio) == 0 {
					t.Fatal("received TTSAudioRawFrame with empty audio payload")
				}
				if v.SampleRate <= 0 {
					t.Fatalf("invalid sample rate on TTSAudioRawFrame: %d", v.SampleRate)
				}
				outPath := filepath.Join("tests", "testdata", "groq_tts_output.wav")
				if err := saveTTSAudioAsWAV(outPath, v.Audio, v.SampleRate, v.NumChannels); err != nil {
					t.Logf("failed to save TTS output to %s: %v", outPath, err)
				}
				return
			case *frames.ErrorFrame:
				// #region agent log
				debugLog("groq_voice_e2e_test.go:ErrorFrame", "error from processor", map[string]interface{}{"processor": v.Processor, "error": v.Error}, "H4")
				// #endregion
				// If Groq TTS requires terms acceptance or similar account-level
				// setup, skip instead of failing the suite.
				if v.Processor == "tts" && strings.Contains(v.Error, "requires terms acceptance") {
					t.Skipf("Groq TTS model requires terms acceptance or additional setup: %s", v.Error)
				}
				t.Fatalf("received ErrorFrame from processor %q: %s", v.Processor, v.Error)
			default:
				// Ignore other frames (e.g. StartFrame, intermediate text).
			}
		}
	}
}

