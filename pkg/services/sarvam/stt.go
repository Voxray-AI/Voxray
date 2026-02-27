package sarvam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"voila-go/pkg/config"
	"voila-go/pkg/frames"
)

// DefaultSarvamSTTModel is the default Sarvam STT model when none is specified.
// It matches the REST API default (saarika:v2.5).
const DefaultSarvamSTTModel = "saarika:v2.5"

// SarvamSTTService implements services.STTService (and STTStreamingService via TranscribeStream)
// using Sarvam AI's speech-to-text REST API.
//
// It uses:
//   POST https://api.sarvam.ai/speech-to-text (multipart/form-data)
// with fields:
//   - file: binary audio (we send raw bytes as provided)
//   - model: e.g. "saarika:v2.5" or "saaras:v3"
//   - input_audio_codec: optional hint when we know we're sending raw PCM.
type SarvamSTTService struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewSTT creates a Sarvam STT service.
// If apiKey is empty, config.GetEnv("SARVAM_API_KEY", "") is used.
// If model is empty, DefaultSarvamSTTModel is used.
func NewSTT(apiKey, model string) *SarvamSTTService {
	if apiKey == "" {
		apiKey = config.GetEnv("SARVAM_API_KEY", "")
	}
	if model == "" {
		model = DefaultSarvamSTTModel
	}
	return &SarvamSTTService{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Transcribe sends audio to Sarvam's REST STT API and returns one TranscriptionFrame (final).
func (s *SarvamSTTService) Transcribe(ctx context.Context, audio []byte, sampleRate, numChannels int) ([]*frames.TranscriptionFrame, error) {
	if len(audio) == 0 {
		return nil, nil
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// File part
	fileWriter, err := writer.CreateFormFile("file", "audio.pcm")
	if err != nil {
		return nil, err
	}
	if _, err := fileWriter.Write(audio); err != nil {
		return nil, err
	}

	// Model part
	if err := writer.WriteField("model", s.model); err != nil {
		return nil, err
	}

	// If we're clearly dealing with 16 kHz raw PCM, hint the codec.
	if sampleRate == 16000 {
		_ = writer.WriteField("input_audio_codec", "pcm_s16le")
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/speech-to-text", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("api-subscription-key", s.apiKey)
	for k, v := range sdkHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		if len(respBody) > 512 {
			respBody = respBody[:512]
		}
		return nil, fmt.Errorf("sarvam STT error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var out struct {
		Transcript    string  `json:"transcript"`
		LanguageCode  *string `json:"language_code"`
		RequestID     string  `json:"request_id"`
		LanguageProb  *float64 `json:"language_probability"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}

	// Even if transcript is empty, return a frame to keep behavior predictable.
	tf := frames.NewTranscriptionFrame(out.Transcript, "user", "", true)
	if out.LanguageCode != nil && *out.LanguageCode != "" {
		tf.Language = *out.LanguageCode
	}
	return []*frames.TranscriptionFrame{tf}, nil
}

// TranscribeStream buffers audio from audioCh and sends final TranscriptionFrame(s) to outCh.
// This mirrors the behavior of the OpenAI STT implementation: it is not truly streaming
// on the wire, but provides a streaming-friendly interface to the pipeline.
func (s *SarvamSTTService) TranscribeStream(ctx context.Context, audioCh <-chan []byte, sampleRate, numChannels int, outCh chan<- frames.Frame) {
	var buf []byte
	for {
		select {
		case <-ctx.Done():
			if len(buf) > 0 && outCh != nil {
				framesOut, _ := s.Transcribe(ctx, buf, sampleRate, numChannels)
				for _, f := range framesOut {
					outCh <- f
				}
			}
			return
		case chunk, ok := <-audioCh:
			if !ok {
				if len(buf) > 0 && outCh != nil {
					framesOut, _ := s.Transcribe(ctx, buf, sampleRate, numChannels)
					for _, f := range framesOut {
						outCh <- f
					}
				}
				return
			}
			buf = append(buf, chunk...)
		}
	}
}

