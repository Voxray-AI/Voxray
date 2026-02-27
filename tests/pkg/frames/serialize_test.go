package frames_test

import (
	"testing"

	"voila-go/pkg/frames"
	"voila-go/pkg/frames/serialize"
)

// TestEncoderDecoderRoundTrip verifies that frames can be encoded to the JSON
// envelope format and decoded back while preserving their type and key fields.
func TestEncoderDecoderRoundTrip(t *testing.T) {
	text := frames.NewTextFrame("hello world")
	data, err := serialize.Encoder(text)
	if err != nil {
		t.Fatalf("Encoder returned error: %v", err)
	}

	decoded, err := serialize.Decoder(data)
	if err != nil {
		t.Fatalf("Decoder returned error: %v", err)
	}

	tf, ok := decoded.(*frames.TextFrame)
	if !ok {
		t.Fatalf("expected *frames.TextFrame, got %T", decoded)
	}
	if tf.Text != text.Text {
		t.Fatalf("expected text %q, got %q", text.Text, tf.Text)
	}
	if !tf.AppendToContext {
		t.Fatalf("expected AppendToContext to be true")
	}
}

