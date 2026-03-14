// Package xtts provides Coqui XTTS text-to-speech via local streaming server.
// See https://github.com/coqui-ai/xtts-streaming-server (e.g. docker run -p 8000:80 ...).
package xtts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
)

const defaultXTTSBaseURL = "http://localhost:8000"

var sharedTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// TTSService implements services.TTSService using XTTS streaming server.
type TTSService struct {
	baseURL    string
	voice      string
	language   string
	httpClient *http.Client
	mu         sync.Mutex
	speakers   map[string]struct {
		SpeakerEmbedding []float64 `json:"speaker_embedding"`
		GptCondLatent    []float64 `json:"gpt_cond_latent"`
	}
}

// NewTTS creates an XTTS TTS service. baseURL defaults to XTTS_BASE_URL env or http://localhost:8000.
func NewTTS(apiKey, baseURL, voice, language string) *TTSService {
	if baseURL == "" {
		baseURL = config.GetEnv("XTTS_BASE_URL", defaultXTTSBaseURL)
	}
	if language == "" {
		language = "en"
	}
	return &TTSService{
		baseURL:    baseURL,
		voice:      voice,
		language:   language,
		httpClient: &http.Client{Transport: sharedTransport, Timeout: 60 * time.Second},
	}
}

func (s *TTSService) getStudioSpeakers(ctx context.Context) (map[string]struct {
	SpeakerEmbedding []float64 `json:"speaker_embedding"`
	GptCondLatent    []float64 `json:"gpt_cond_latent"`
}, error) {
	s.mu.Lock()
	sp := s.speakers
	s.mu.Unlock()
	if sp != nil {
		return sp, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/studio_speakers", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("xtts studio_speakers: %s: %s", resp.Status, string(body))
	}
	var out map[string]struct {
		SpeakerEmbedding []float64 `json:"speaker_embedding"`
		GptCondLatent    []float64 `json:"gpt_cond_latent"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.speakers = out
	s.mu.Unlock()
	return out, nil
}

// Speak requests TTS from XTTS and returns TTSAudioRawFrame(s). XTTS outputs 24000 Hz.
func (s *TTSService) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	if sampleRate <= 0 {
		sampleRate = 24000
	}
	speakers, err := s.getStudioSpeakers(ctx)
	if err != nil {
		return nil, err
	}
	voice := s.voice
	if voice == "" {
		for k := range speakers {
			voice = k
			break
		}
	}
	if voice == "" {
		return nil, fmt.Errorf("xtts: no studio speakers available")
	}
	emb, ok := speakers[voice]
	if !ok {
		return nil, fmt.Errorf("xtts: voice %q not in studio_speakers", voice)
	}
	payload := map[string]any{
		"text":               cleanXTTSText(text),
		"language":           s.language,
		"speaker_embedding":  emb.SpeakerEmbedding,
		"gpt_cond_latent":    emb.GptCondLatent,
		"add_wav_header":     false,
		"stream_chunk_size":  20,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/tts_stream", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("xtts tts_stream: %s: %s", resp.Status, string(b))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	f := frames.NewTTSAudioRawFrame(data, sampleRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}

func cleanXTTSText(text string) string {
	b := []byte(text)
	b = bytes.ReplaceAll(b, []byte("."), nil)
	b = bytes.ReplaceAll(b, []byte("*"), nil)
	return string(b)
}
