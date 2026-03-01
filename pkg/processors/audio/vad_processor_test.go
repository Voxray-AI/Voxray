package audio

import (
	"context"
	"testing"

	"voila-go/pkg/audio/vad"
	"voila-go/pkg/frames"
	"voila-go/pkg/processors"
)

// fakeVADAnalyzer returns a fixed sequence of states for testing.
type fakeVADAnalyzer struct {
	states  []vad.State
	index   int
	params  vad.Params
	sr      int
}

func (f *fakeVADAnalyzer) SetSampleRate(sr int)   { f.sr = sr }
func (f *fakeVADAnalyzer) SetParams(p vad.Params) { f.params = p }
func (f *fakeVADAnalyzer) Params() vad.Params    { return f.params }

func (f *fakeVADAnalyzer) Analyze(_ []byte) (vad.State, float64, float64, error) {
	if f.index >= len(f.states) {
		return vad.StateQuiet, 0, 0, nil
	}
	s := f.states[f.index]
	f.index++
	return s, 0, 0, nil
}

type collectProcessor struct {
	received []frames.Frame
}

func (c *collectProcessor) ProcessFrame(_ context.Context, f frames.Frame, dir processors.Direction) error {
	if dir == processors.Downstream {
		c.received = append(c.received, f)
	}
	return nil
}

func (c *collectProcessor) SetNext(processors.Processor)  {}
func (c *collectProcessor) SetPrev(processors.Processor)  {}
func (c *collectProcessor) Setup(context.Context) error   { return nil }
func (c *collectProcessor) Cleanup(context.Context) error { return nil }
func (c *collectProcessor) Name() string                  { return "collector" }

func TestVADProcessor_EmitsStartAndStopFrames(t *testing.T) {
	ctx := context.Background()
	// Sequence: Quiet -> Speaking -> Speaking -> Quiet to get one start and one stop
	analyzer := &fakeVADAnalyzer{
		states: []vad.State{vad.StateQuiet, vad.StateSpeaking, vad.StateSpeaking, vad.StateQuiet},
		params: vad.Params{StartSecs: 0.2, StopSecs: 0.3},
	}
	p := NewVADProcessor("vad", analyzer, 0)
	col := &collectProcessor{}
	p.SetNext(col)

	start := frames.NewStartFrame()
	start.AudioInSampleRate = 16000
	if err := p.ProcessFrame(ctx, start, processors.Downstream); err != nil {
		t.Fatalf("StartFrame: %v", err)
	}

	chunk := make([]byte, 320) // 10ms at 16kHz
	for i := 0; i < 4; i++ {
		ar := frames.NewAudioRawFrame(chunk, 16000, 1, 0)
		if err := p.ProcessFrame(ctx, ar, processors.Downstream); err != nil {
			t.Fatalf("AudioRawFrame %d: %v", i, err)
		}
	}

	var started, stopped bool
	for _, f := range col.received {
		switch f.(type) {
		case *frames.VADUserStartedSpeakingFrame:
			started = true
		case *frames.VADUserStoppedSpeakingFrame:
			stopped = true
		}
	}
	if !started {
		t.Error("expected VADUserStartedSpeakingFrame downstream")
	}
	if !stopped {
		t.Error("expected VADUserStoppedSpeakingFrame downstream")
	}
}

func TestVADProcessor_ForwardsFrames(t *testing.T) {
	ctx := context.Background()
	analyzer := &fakeVADAnalyzer{states: []vad.State{vad.StateQuiet}}
	p := NewVADProcessor("vad", analyzer, 0)
	col := &collectProcessor{}
	p.SetNext(col)

	start := frames.NewStartFrame()
	if err := p.ProcessFrame(ctx, start, processors.Downstream); err != nil {
		t.Fatal(err)
	}
	if len(col.received) != 1 {
		t.Fatalf("expected 1 frame (StartFrame), got %d", len(col.received))
	}
	if _, ok := col.received[0].(*frames.StartFrame); !ok {
		t.Fatalf("expected StartFrame, got %T", col.received[0])
	}
}
