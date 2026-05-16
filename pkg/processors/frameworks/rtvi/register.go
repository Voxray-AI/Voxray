package rtvi

import (
	"encoding/json"

	"github.com/Voxray-AI/Voxray/pkg/pipeline"
	"github.com/Voxray-AI/Voxray/pkg/processors"
)

func init() {
	pipeline.RegisterProcessor("rtvi", func(name string, opts json.RawMessage) processors.Processor {
		return NewRTVIProcessorFromOptions(name, opts)
	})
}
