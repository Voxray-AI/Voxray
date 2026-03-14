// Package camb provides Camb AI speech-to-text.
package camb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
)

const defaultCambBaseURL = "https://api.camb.ai"

var sharedTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// STTService implements services.STTService using Camb AI's transcription API.
type STTService struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewSTT creates a Camb STT service.
func NewSTT(apiKey, baseURL, model string) *STTService {
	if baseURL == "" {
		baseURL = config.GetEnv("CAMB_BASE_URL", defaultCambBaseURL)
	}
	return &STTService{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Transport: sharedTransport, Timeout: 60 * time.Second},
	}
}

// Transcribe sends audio to Camb and returns transcription frames.
func (s *STTService) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	if len(audio) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("audio", "audio.wav")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(audio); err != nil {
		return nil, err
	}
	if s.model != "" {
		_ = w.WriteField("model", s.model)
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v1/transcribe", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("camb STT: %s: %s", resp.Status, string(body))
	}
	var out struct {
		Text     string `json:"text"`
		Transcript string `json:"transcript"`
	}
	_ = json.Unmarshal(body, &out)
	text := out.Text
	if text == "" {
		text = out.Transcript
	}
	tf := frames.NewTranscriptionFrame(text, "user", "", true)
	return []*frames.TranscriptionFrame{tf}, nil
}
