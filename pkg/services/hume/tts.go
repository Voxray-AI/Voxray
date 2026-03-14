// Package hume provides Hume (Hume AI) text-to-speech.
package hume

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

const humeTTSBaseURL = "https://api.hume.ai/v0/evi/tts"
const humeSampleRate = 48000

var sharedTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// TTSService implements services.TTSService using Hume's TTS API.
type TTSService struct {
	apiKey     string
	voiceID    string
	httpClient *http.Client
}

// NewTTS creates a Hume TTS service.
func NewTTS(apiKey, voiceID string) *TTSService {
	if apiKey == "" {
		apiKey = config.GetEnv("HUME_API_KEY", "")
	}
	return &TTSService{
		apiKey:     apiKey,
		voiceID:    voiceID,
		httpClient: &http.Client{Transport: sharedTransport, Timeout: 60 * time.Second},
	}
}

// Speak synthesizes text and returns TTS audio frames (48 kHz PCM).
func (s *TTSService) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	if sampleRate <= 0 {
		sampleRate = humeSampleRate
	}
	payload := map[string]any{
		"text":  text,
		"voice": map[string]string{"id": s.voiceID},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, humeTTSBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hume-API-Key", s.apiKey)
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
		return nil, fmt.Errorf("hume TTS: %s: %s", resp.Status, string(data))
	}
	// Hume may return JSON with base64 audio or raw PCM
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
	// Raw PCM
	f := frames.NewTTSAudioRawFrame(data, sampleRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}
