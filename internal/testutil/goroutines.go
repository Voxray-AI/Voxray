package testutil

import (
	"runtime"
	"testing"
	"time"
)

// TrackGoroutines records the goroutine count at the start of a test and
// returns a function that asserts no significant leaks when called at the end.
// Typical usage:
//
//	func TestSomething(t *testing.T) {
//	    done := TrackGoroutines(t)
//	    defer done()
//	    // ... test body ...
//	}
//
// We allow a small slack (+2) for background runtime goroutines that may start
// or stop between measurements.
func TrackGoroutines(t *testing.T) func() {
	t.Helper()
	before := runtime.NumGoroutine()
	return func() {
		// Allow settling time for goroutines to exit.
		time.Sleep(100 * time.Millisecond)
		after := runtime.NumGoroutine()
		if after > before+2 {
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Errorf("goroutine leak: started with %d, ended with %d\n%s",
				before, after, buf[:n])
		}
	}
}

