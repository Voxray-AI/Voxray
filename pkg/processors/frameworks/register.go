package frameworks

import (
	"encoding/json"

	"github.com/Voxray-AI/Voxray/pkg/pipeline"
	"github.com/Voxray-AI/Voxray/pkg/processors"
)

func init() {
	pipeline.RegisterProcessor("external_chain", func(name string, opts json.RawMessage) processors.Processor {
		return NewExternalChainProcessorFromOptions(name, opts)
	})
}
