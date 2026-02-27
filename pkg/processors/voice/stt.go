// Package voice provides processors that wire STT, LLM, and TTS into a pipeline.
package voice

import (
	"context"

	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
	"voila-go/pkg/processors"
	"voila-go/pkg/services"
)

// STTProcessor turns AudioRawFrame into TranscriptionFrame using an STTService.
type STTProcessor struct {
	*processors.BaseProcessor
	STT        services.STTService
	SampleRate int
	Channels   int
}

// NewSTTProcessor returns a processor that transcribes audio and pushes TranscriptionFrame(s) downstream.
func NewSTTProcessor(name string, stt services.STTService, sampleRate, channels int) *STTProcessor {
	if name == "" {
		name = "STT"
	}
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	if channels <= 0 {
		channels = 1
	}
	return &STTProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		STT:           stt,
		SampleRate:    sampleRate,
		Channels:      channels,
	}
}

// ProcessFrame transcribes AudioRawFrame and forwards other frames.
func (p *STTProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}
	audio, ok := f.(*frames.AudioRawFrame)
	if !ok {
		return p.PushDownstream(ctx, f)
	}
	tfs, err := p.STT.Transcribe(ctx, audio.Audio, p.SampleRate, p.Channels)
	if err != nil {
		_ = p.PushDownstream(ctx, frames.NewErrorFrame(err.Error(), false, p.Name()))
		return nil
	}
	for _, tf := range tfs {
		// Log that STT output is in expected format (TranscriptionFrame with Text, Finalized, optional Language).
		preview := tf.Text
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		logger.Info("STT output (expected format): processor=%s textLen=%d finalized=%v language=%q preview=%q\n",
			p.Name(), len(tf.Text), tf.Finalized, tf.Language, preview)
		_ = p.PushDownstream(ctx, tf)
	}
	return nil
}
