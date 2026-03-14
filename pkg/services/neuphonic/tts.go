// Package neuphonic provides Neuphonic text-to-speech (HTTP SSE streaming).
package neuphonic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
)

const defaultNeuphonicBaseURL = "https://api.neuphonic.com"
const defaultSampleRate = 22050

var sharedTransport = &http.Transport{
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// TTSService implements services.TTSService using Neuphonic HTTP SSE API.
type TTSService struct {
	apiKey     string
	baseURL    string
	voiceID    string
	language   string
	speed      float64
	httpClient *http.Client
}

// NewTTS creates a Neuphonic TTS service.
func NewTTS(apiKey, baseURL, voiceID, language string, speed float64) *TTSService {
	if baseURL == "" {
		baseURL = config.GetEnv("NEUPHONIC_BASE_URL", defaultNeuphonicBaseURL)
	}
	if language == "" {
		language = "en"
	}
	if speed <= 0 {
		speed = 1.0
	}
	return &TTSService{
		apiKey:     apiKey,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		voiceID:    voiceID,
		language:   language,
		speed:      speed,
		httpClient: &http.Client{Transport: sharedTransport, Timeout: 60 * time.Second},
	}
}

// Speak synthesizes text and returns TTS audio frames.
func (s *TTSService) Speak(ctx context.Context, text string, sampleRate int) ([]*frames.TTSAudioRawFrame, error) {
	if sampleRate <= 0 {
		sampleRate = defaultSampleRate
	}
	payload := map[string]any{
		"text":          text,
		"lang_code":     s.language,
		"encoding":      "pcm_linear",
		"sampling_rate": sampleRate,
		"speed":         s.speed,
	}
	if s.voiceID != "" {
		payload["voice_id"] = s.voiceID
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := s.baseURL + "/sse/speak/" + s.language
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", s.apiKey)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("neuphonic TTS: %s: %s", resp.Status, string(b))
	}
	var chunks [][]byte
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "" || data == "[DONE]" {
				continue
			}
			var msg struct {
				Data struct {
					Audio string `json:"audio"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(data), &msg); err != nil {
				continue
			}
			if msg.Data.Audio != "" {
				decoded, err := base64.StdEncoding.DecodeString(msg.Data.Audio)
				if err != nil {
					continue
				}
				chunks = append(chunks, decoded)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	// Concatenate and return one frame
	total := 0
	for _, c := range chunks {
		total += len(c)
	}
	out := make([]byte, 0, total)
	for _, c := range chunks {
		out = append(out, c...)
	}
	f := frames.NewTTSAudioRawFrame(out, sampleRate)
	return []*frames.TTSAudioRawFrame{f}, nil
}
