package echo

import (
	"context"

	"github.com/Voxray-AI/Voxray/pkg/frames"
	"github.com/Voxray-AI/Voxray/pkg/processors"
)

// Processor forwards all frames unchanged (for tests).
type Processor struct {
	*processors.BaseProcessor
}

// New returns a new echo processor.
func New(name string) *Processor {
	if name == "" {
		name = "Echo"
	}
	return &Processor{BaseProcessor: processors.NewBaseProcessor(name)}
}

// ProcessFrame forwards the frame unchanged.
func (p *Processor) ProcessFrame(ctx context.Context, f frames.Frame, dir processors.Direction) error {
	return p.BaseProcessor.ProcessFrame(ctx, f, dir)
}
