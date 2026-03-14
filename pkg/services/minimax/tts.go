// Package minimax provides Minimax text-to-speech.
package minimax

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
)

const defaultMinimaxTTSURL = "https://api.minimax.io/v1/text_to_speech"

var ttsTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// TTSService implements services.TTSService using Minimax TTS API.
type TTSService struct {
	apiKey     string
	baseURL    string
	model      string
	voiceID    string
	httpClient *http.Client
}

// NewTTS creates a Minimax TTS service.
func NewTTS(apiKey, baseURL, model, voiceID string) *TTSService {
	if baseURL == "" {
		baseURL = config.GetEnv("MINIMAX_BASE_URL", "https://api.minimax.io")
	}
	return &TTSService{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		voiceID:    voiceID,
		httpClient: &http.Client{Transport: ttsTransport, Timeout: 60 * time.Second},
	}
}

// Speak synthesizes text and returns TTS audio frames.
func (s *TTSService) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	payload := map[string]any{
		"text":   text,
		"model":  s.model,
		"voice_id": s.voiceID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v1/text_to_speech", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("minimax TTS: %s: %s", resp.Status, string(data))
	}
	// Response may be raw audio or JSON with base64 audio
	var jsonOut struct {
		Audio string `json:"audio"`
		Data  string `json:"data"`
	}
	if json.Unmarshal(data, &jsonOut) == nil && (jsonOut.Audio != "" || jsonOut.Data != "") {
		b64 := jsonOut.Audio
		if b64 == "" {
			b64 = jsonOut.Data
		}
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err == nil {
			f := frames.NewTTSAudioRawFrame(decoded, sampleRate)
			return []*frames.TTSAudioRawFrame{f}, nil
		}
	}
	f := frames.NewTTSAudioRawFrame(data, sampleRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}
