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

// TestProtoBinaryRoundTrip verifies binary protobuf frame encode/decode.
func TestProtoBinaryRoundTrip(t *testing.T) {
	text := frames.NewTextFrame("wire test")
	data, err := serialize.ProtoEncode(text)
	if err != nil {
		t.Fatalf("ProtoEncode: %v", err)
	}
	if data == nil {
		t.Fatal("ProtoEncode returned nil")
	}
	decoded, err := serialize.ProtoDecode(data)
	if err != nil {
		t.Fatalf("ProtoDecode: %v", err)
	}
	tf, ok := decoded.(*frames.TextFrame)
	if !ok {
		t.Fatalf("expected *frames.TextFrame, got %T", decoded)
	}
	if tf.Text != text.Text {
		t.Fatalf("expected text %q, got %q", text.Text, tf.Text)
	}
}

// TestProtoEnvelopeRoundTrip verifies binary envelope (wire.FrameEnvelope) encode/decode.
func TestProtoEnvelopeRoundTrip(t *testing.T) {
	text := frames.NewTextFrame("envelope test")
	data, err := serialize.ProtoEncoder(text)
	if err != nil {
		t.Fatalf("ProtoEncoder: %v", err)
	}
	decoded, err := serialize.ProtoDecoder(data)
	if err != nil {
		t.Fatalf("ProtoDecoder: %v", err)
	}
	tf, ok := decoded.(*frames.TextFrame)
	if !ok {
		t.Fatalf("expected *frames.TextFrame, got %T", decoded)
	}
	if tf.Text != text.Text {
		t.Fatalf("expected text %q, got %q", text.Text, tf.Text)
	}
}

// TestTransportMessageFrameRoundTrip verifies TransportMessageFrame in JSON envelope.
func TestTransportMessageFrameRoundTrip(t *testing.T) {
	msg := frames.NewTransportMessageFrame(map[string]any{"event": "media", "payload": "base64data"})
	data, err := serialize.Encoder(msg)
	if err != nil {
		t.Fatalf("Encoder: %v", err)
	}
	decoded, err := serialize.Decoder(data)
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}
	tm, ok := decoded.(*frames.TransportMessageFrame)
	if !ok {
		t.Fatalf("expected *frames.TransportMessageFrame, got %T", decoded)
	}
	if tm.Message["event"] != "media" {
		t.Fatalf("expected event=media, got %v", tm.Message["event"])
	}
}

