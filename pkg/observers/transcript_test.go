package observers

import (
	"context"
	"sync"
	"testing"
	"time"

	"voxray-go/pkg/frames"
	"voxray-go/pkg/processors"
	"voxray-go/pkg/transcripts"
)

type fakeStore struct {
	mu    sync.Mutex
	msgs  []storedMsg
}

type storedMsg struct {
	sessionID string
	role      string
	text      string
	seq       int64
}

func (f *fakeStore) SaveMessage(ctx context.Context, sessionID, role, text string, at time.Time, seq int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, storedMsg{
		sessionID: sessionID,
		role:      role,
		text:      text,
		seq:       seq,
	})
	return nil
}

func (f *fakeStore) Close() error { return nil }

var _ transcripts.Store = (*fakeStore)(nil)

func TestTranscriptObserver_UserAndAssistant(t *testing.T) {
	store := &fakeStore{}
	obs := NewTranscriptObserver(store, "session-1")
	if obs == nil {
		t.Fatalf("expected non-nil observer")
	}

	// Final user transcription.
	tf := frames.NewTranscriptionFrame("hello world", "", "", true)
	obs.OnFrameProcessed("stt", tf, processors.Downstream)

	// Assistant response in two chunks, then TTSSpeakFrame to flush.
	llmChunk1 := &frames.LLMTextFrame{
		TextFrame: frames.TextFrame{
			DataFrame:        frames.DataFrame{Base: frames.NewBase()},
			Text:             "hi ",
			AppendToContext:  true,
		},
	}
	llmChunk2 := &frames.LLMTextFrame{
		TextFrame: frames.TextFrame{
			DataFrame:        frames.DataFrame{Base: frames.NewBase()},
			Text:             "there",
			AppendToContext:  true,
		},
	}
	obs.OnFrameProcessed("llm", llmChunk1, processors.Downstream)
	obs.OnFrameProcessed("llm", llmChunk2, processors.Downstream)
	obs.OnFrameProcessed("tts", frames.NewTTSSpeakFrame(""), processors.Downstream)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(store.msgs))
	}
	if store.msgs[0].role != "user" || store.msgs[0].text != "hello world" {
		t.Errorf("unexpected first message: %+v", store.msgs[0])
	}
	if store.msgs[1].role != "assistant" || store.msgs[1].text != "hi there" {
		t.Errorf("unexpected second message: %+v", store.msgs[1])
	}
	if store.msgs[0].seq != 1 || store.msgs[1].seq != 2 {
		t.Errorf("unexpected seq values: %+v", store.msgs)
	}
}

