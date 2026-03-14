package realtime_test

import (
	"testing"

	"voxray-go/pkg/config"
	"voxray-go/pkg/realtime"
)

// TestNewFromConfig_OpenAI verifies that realtime.NewFromConfig returns a non-nil
// RealtimeService for the "openai" provider (construction only, no connection).
func TestNewFromConfig_OpenAI(t *testing.T) {
	cfg := &config.Config{Model: "gpt-4o-realtime-preview-2024-12-17"}
	svc, err := realtime.NewFromConfig(cfg, "openai")
	if err != nil {
		t.Fatalf("NewFromConfig(openai): %v", err)
	}
	if svc == nil {
		t.Fatal("NewFromConfig(openai) returned nil service")
	}
}

// TestNewFromConfig_HumeAndInworld verifies realtime returns a service for hume and inworld (stub).
func TestNewFromConfig_HumeAndInworld(t *testing.T) {
	cfg := &config.Config{}
	for _, provider := range []string{"hume", "inworld"} {
		svc, err := realtime.NewFromConfig(cfg, provider)
		if err != nil {
			t.Fatalf("NewFromConfig(%q): %v", provider, err)
		}
		if svc == nil {
			t.Fatalf("NewFromConfig(%q) returned nil service", provider)
		}
	}
}

// TestNewFromConfig_UnsupportedProvider returns error for unknown provider.
func TestNewFromConfig_UnsupportedProvider(t *testing.T) {
	cfg := &config.Config{}
	svc, err := realtime.NewFromConfig(cfg, "unknown")
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if svc != nil {
		t.Fatal("expected nil service for unsupported provider")
	}
}
