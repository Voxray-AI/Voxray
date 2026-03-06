package observers

import (
	"context"
	"sync"
	"time"
	"strings"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/transcripts"
)

// TranscriptObserver records user and assistant messages for a session.
type TranscriptObserver struct {
	store     transcripts.Store
	sessionID string

	mu     sync.Mutex
	seq    int64
	botBuf strings.Builder
}

// NewTranscriptObserver creates a new TranscriptObserver for a session.
func NewTranscriptObserver(store transcripts.Store, sessionID string) *TranscriptObserver {
	if store == nil || sessionID == "" {
		return nil
	}
	return &TranscriptObserver{
		store:     store,
		sessionID: sessionID,
	}
}

// Ensure TranscriptObserver implements Observer.
var _ Observer = (*TranscriptObserver)(nil)

// OnFrameProcessed implements Observer.
func (o *TranscriptObserver) OnFrameProcessed(processorName string, f frames.Frame, dir processors.Direction) {
	if o == nil || o.store == nil {
		return
	}
	if dir != processors.Downstream {
		return
	}
	if f == nil {
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	switch t := f.(type) {
	case *frames.TranscriptionFrame:
		if !t.Finalized || t.Text == "" {
			return
		}
		o.seq++
		_ = o.store.SaveMessage(context.Background(), o.sessionID, "user", t.Text, time.Now().UTC(), o.seq)
	case *frames.LLMTextFrame:
		if t.Text == "" {
			return
		}
		o.botBuf.WriteString(t.Text)
	case *frames.TTSSpeakFrame:
		o.flushAssistantLocked()
	case *frames.EndFrame, *frames.CancelFrame:
		o.flushAssistantLocked()
	}
}

func (o *TranscriptObserver) flushAssistantLocked() {
	if o.botBuf.Len() == 0 {
		return
	}
	text := o.botBuf.String()
	o.botBuf.Reset()
	if text == "" {
		return
	}
	o.seq++
	_ = o.store.SaveMessage(context.Background(), o.sessionID, "assistant", text, time.Now().UTC(), o.seq)
}

