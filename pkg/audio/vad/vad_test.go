package vad

import "testing"

func TestParamsNormalizeDefaults(t *testing.T) {
	p := Params{}
	p2 := p.normalize()
	if p2.Confidence <= 0 || p2.StartSecs <= 0 || p2.StopSecs <= 0 || p2.MinVolume <= 0 {
		t.Fatalf("expected normalized params to have positive values, got %+v", p2)
	}
}

func TestBaseAnalyzerStateTransitions(t *testing.T) {
	backend := &EnergyAnalyzerBackend{Threshold: 0.02}
	a := newBaseAnalyzer(backend)
	a.SetSampleRate(16000)
	a.SetParams(Params{
		Confidence: 0.1,
		StartSecs:  0.0,
		StopSecs:   0.0,
		MinVolume:  0.0,
	})

	// Generate a small non-zero buffer to simulate speech.
	speech := []byte{0x10, 0x00, 0x20, 0x00}
	state, _, _, err := a.Analyze(speech)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if state == StateQuiet {
		t.Fatalf("expected state to move away from Quiet with speech-like buffer")
	}
}

