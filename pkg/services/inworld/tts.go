// Package inworld provides Inworld text-to-speech (and LLM).
package inworld

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

const inworldTTSBaseURL = "https://api.inworld.ai/v1"

var ttsTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// TTSService implements services.TTSService using Inworld TTS API.
type TTSService struct {
	apiKey     string
	voiceID    string
	httpClient *http.Client
}

// NewTTS creates an Inworld TTS service.
func NewTTS(apiKey, voiceID string) *TTSService {
	if apiKey == "" {
		apiKey = config.GetEnv("INWORLD_API_KEY", "")
	}
	return &TTSService{
		apiKey:     apiKey,
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
		"text":     text,
		"voice_id": s.voiceID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, inworldTTSBaseURL+"/tts", bytes.NewReader(body))
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
		return nil, fmt.Errorf("inworld TTS: %s: %s", resp.Status, string(data))
	}
	var jsonOut struct {
		Audio string `json:"audio"`
	}
	if json.Unmarshal(data, &jsonOut) == nil && jsonOut.Audio != "" {
		decoded, err := base64.StdEncoding.DecodeString(jsonOut.Audio)
		if err != nil {
			return nil, err
		}
		f := frames.NewTTSAudioRawFrame(decoded, sampleRate)
		return []*frames.TTSAudioRawFrame{f}, nil
	}
	f := frames.NewTTSAudioRawFrame(data, sampleRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}
