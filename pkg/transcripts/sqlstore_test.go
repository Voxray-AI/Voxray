package transcripts

import (
	"context"
	"testing"
	"time"
)

// TestSaveMessage_NilStore verifies that calling SaveMessage on a nil *SQLStore
// returns a non-nil error instead of silently succeeding.
func TestSaveMessage_NilStore(t *testing.T) {
	var s *SQLStore
	err := s.SaveMessage(context.Background(), "sess", "user", "hi", time.Now(), 1)
	if err == nil {
		t.Fatalf("expected error when calling SaveMessage on nil store, got nil")
	}
}

// TestSaveMessage_UninitializedDB verifies that SaveMessage detects an
// uninitialized *sql.DB and returns an error.
func TestSaveMessage_UninitializedDB(t *testing.T) {
	s := &SQLStore{}
	err := s.SaveMessage(context.Background(), "sess", "user", "hi", time.Now(), 1)
	if err == nil {
		t.Fatalf("expected error when calling SaveMessage with nil DB, got nil")
	}
}

