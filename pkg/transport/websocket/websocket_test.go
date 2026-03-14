package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"voxray-go/pkg/frames"
)

// TestWriteCoalescing_Disabled verifies that when WriteCoalesceMs is 0, each frame results in exactly one write.
func TestWriteCoalescing_Disabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		_ = conn
		// Server holds conn open; readLoop will block until client closes.
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	u.Path = "/"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	tr := NewConnTransport(conn, 64, 64, nil)
	var writeCount int32
	tr.WriteMessageFunc = func(messageType int, data []byte) error {
		atomic.AddInt32(&writeCount, 1)
		return nil
	}
	tr.WriteCoalesceMs = 0

	if err := tr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer tr.Close()

	const N = 5
	for i := 0; i < N; i++ {
		tr.Output() <- frames.NewTextFrame("x")
	}
	time.Sleep(100 * time.Millisecond)
	if n := atomic.LoadInt32(&writeCount); n != N {
		t.Errorf("with coalescing disabled: expected %d writes, got %d", N, n)
	}
}

// TestWriteCoalescing_Enabled verifies that when WriteCoalesceMs > 0, the coalescing path runs and all frames are written.
// Current implementation still does one write per frame (batched drain only); we assert all N frames result in N writes and no panic.
func TestWriteCoalescing_Enabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		_ = conn
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	u.Scheme = "ws"
	u.Path = "/"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	tr := NewConnTransport(conn, 64, 64, nil)
	var writeCount int32
	tr.WriteMessageFunc = func(messageType int, data []byte) error {
		atomic.AddInt32(&writeCount, 1)
		return nil
	}
	tr.WriteCoalesceMs = 10
	tr.WriteCoalesceMaxFrames = 10

	if err := tr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer tr.Close()

	const N = 8
	for i := 0; i < N; i++ {
		tr.Output() <- frames.NewTextFrame("y")
	}
	time.Sleep(50 * time.Millisecond)
	if n := atomic.LoadInt32(&writeCount); n != N {
		t.Errorf("with coalescing enabled: expected %d writes (one per frame), got %d", N, n)
	}
}
