package voice

import (
	"context"
	"sync"

	"voila-go/pkg/frames"
	"voila-go/pkg/logger"
	"voila-go/pkg/processors"
	"voila-go/pkg/services"
)

// LLMProcessor runs the LLM on transcription/context and streams LLMTextFrame downstream.
type LLMProcessor struct {
	*processors.BaseProcessor
	LLM          services.LLMService
	SystemPrompt string // optional; when set, sent as system message so the LLM replies as assistant
	mu           sync.Mutex
	msgs         []map[string]any
}

// NewLLMProcessor returns a processor that runs the LLM and streams text downstream.
func NewLLMProcessor(name string, llm services.LLMService) *LLMProcessor {
	return NewLLMProcessorWithSystemPrompt(name, llm, "")
}

// NewLLMProcessorWithSystemPrompt returns a processor that runs the LLM with an optional system prompt (e.g. "You are a helpful voice assistant. Reply briefly.").
func NewLLMProcessorWithSystemPrompt(name string, llm services.LLMService, systemPrompt string) *LLMProcessor {
	if name == "" {
		name = "LLM"
	}
	return &LLMProcessor{
		BaseProcessor: processors.NewBaseProcessor(name),
		LLM:           llm,
		SystemPrompt:  systemPrompt,
		msgs:          make([]map[string]any, 0),
	}
}

// ProcessFrame runs LLM on TranscriptionFrame (appends user message) or LLMRunFrame; streams LLMTextFrame downstream. Forwards other frames.
func (p *LLMProcessor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	if dir != processors.Downstream {
		if p.Prev() != nil {
			return p.Prev().ProcessFrame(ctx, f, dir)
		}
		return nil
	}

	switch t := f.(type) {
	case *frames.TranscriptionFrame:
		preview := t.Text
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		logger.Info("LLM: transcript received from STT: %d chars, preview=%q", len(t.Text), preview)
		p.mu.Lock()
		p.msgs = append(p.msgs, map[string]any{"role": "user", "content": t.Text})
		msgs := make([]map[string]any, len(p.msgs))
		copy(msgs, p.msgs)
		p.mu.Unlock()
		return p.runLLM(ctx, msgs)
	case *frames.LLMRunFrame:
		p.mu.Lock()
		msgs := make([]map[string]any, len(p.msgs))
		copy(msgs, p.msgs)
		p.mu.Unlock()
		return p.runLLM(ctx, msgs)
	default:
		return p.PushDownstream(ctx, f)
	}
}

func (p *LLMProcessor) runLLM(ctx context.Context, messages []map[string]any) error {
	msgsToSend := make([]map[string]any, 0, len(messages)+1)
	if p.SystemPrompt != "" {
		msgsToSend = append(msgsToSend, map[string]any{"role": "system", "content": p.SystemPrompt})
	}
	msgsToSend = append(msgsToSend, messages...)

	var fullContent string
	err := p.LLM.Chat(ctx, msgsToSend, func(tf *frames.LLMTextFrame) {
		fullContent += tf.Text
		_ = p.PushDownstream(ctx, tf)
	})
	if err != nil {
		return err
	}
	if fullContent != "" {
		preview := fullContent
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		logger.Info("LLM: response complete, sending to TTS: %d chars, preview=%q", len(fullContent), preview)
		p.mu.Lock()
		p.msgs = append(p.msgs, map[string]any{"role": "assistant", "content": fullContent})
		p.mu.Unlock()
		// Signal TTS to flush any buffered text (sentence batching)
		_ = p.PushDownstream(ctx, frames.NewTTSSpeakFrame(""))
	}
	return nil
}
