package rtvi

import (
	"encoding/json"

	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors"
)

func init() {
	pipeline.RegisterProcessor("rtvi", func(name string, opts json.RawMessage) processors.Processor {
		return NewRTVIProcessorFromOptions(name, opts)
	})
}
