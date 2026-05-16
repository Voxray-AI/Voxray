package logger_test

import (
	"context"
	"testing"

	"github.com/Voxray-AI/Voxray/pkg/frames"
	"github.com/Voxray-AI/Voxray/pkg/processors"
	"github.com/Voxray-AI/Voxray/pkg/processors/logger"
)

func TestLogger_ProcessFrame(t *testing.T) {
	p := logger.New("log")
	ctx := context.Background()
	f := frames.NewTextFrame("test")
	if err := p.ProcessFrame(ctx, f, processors.Downstream); err != nil {
		t.Fatal(err)
	}
}

func TestLogger_NewDefaultName(t *testing.T) {
	p := logger.New("")
	if p.Name() != "Logger" {
		t.Errorf("default name: got %q", p.Name())
	}
}

func TestLogger_New(t *testing.T) {
	p := logger.New("log")
	if p == nil || p.Name() != "log" {
		t.Errorf("New: got %v", p)
	}
}

