// Package soniox provides Soniox speech-to-text (WebSocket API used for batch Transcribe).
package soniox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"voxray-go/pkg/config"
	"voxray-go/pkg/frames"
)

const sonioxWSURL = "wss://stt-rt.soniox.com/transcribe-websocket"
const sonioxModel = "stt-rt-v4"

// STTService implements services.STTService using Soniox WebSocket API.
// Transcribe opens a short-lived WebSocket session, sends audio, finalizes, and returns the transcript.
type STTService struct {
	apiKey string
	url    string
	model  string
}

// NewSTT creates a Soniox STT service.
func NewSTT(apiKey, url, model string) *STTService {
	if url == "" {
		url = config.GetEnv("SONIOX_WS_URL", sonioxWSURL)
	}
	if model == "" {
		model = config.GetEnv("SONIOX_MODEL", sonioxModel)
	}
	return &STTService{apiKey: apiKey, url: url, model: model}
}

// Transcribe sends audio via Soniox WebSocket and returns the final transcript.
func (s *STTService) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	if len(audio) == 0 {
		return nil, nil
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("soniox dial: %w", err)
	}
	defer conn.Close()

	configMsg := map[string]any{
		"api_key":                 s.apiKey,
		"model":                    s.model,
		"audio_format":             "pcm_s16le",
		"num_channels":             numChannels,
		"sample_rate":              sampleRate,
		"enable_endpoint_detection": false,
	}
	configBytes, err := json.Marshal(configMsg)
	if err != nil {
		return nil, err
	}
	if err := conn.WriteMessage(websocket.TextMessage, configBytes); err != nil {
		return nil, err
	}

	var finalText string
	var mu sync.Mutex
	done := make(chan struct{})

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg struct {
				Tokens  []struct { Text string `json:"text"`; IsFinal bool `json:"is_final"` } `json:"tokens"`
				Finished bool `json:"finished"`
			}
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			for _, t := range msg.Tokens {
				if t.Text != "" && t.Text != " " {
					mu.Lock()
					finalText += t.Text
					mu.Unlock()
				}
			}
			if msg.Finished {
				close(done)
				return
			}
		}
	}()

	// Send audio in one or more chunks
	chunkSize := 4096
	for i := 0; i < len(audio); i += chunkSize {
		end := i + chunkSize
		if end > len(audio) {
			end = len(audio)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, audio[i:end]); err != nil {
			return nil, err
		}
	}
	// Finalize
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type": "finalize"}`)); err != nil {
		return nil, err
	}

	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("soniox: timeout waiting for transcript")
	}
	<-readDone
	mu.Lock()
	text := finalText
	mu.Unlock()
	tf := frames.NewTranscriptionFrame(text, "user", "", true)
	return []*frames.TranscriptionFrame{tf}, nil
}
