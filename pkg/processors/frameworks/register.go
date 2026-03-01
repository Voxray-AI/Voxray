package frameworks

import (
	"encoding/json"

	"voila-go/pkg/pipeline"
	"voila-go/pkg/processors"
)

func init() {
	pipeline.RegisterProcessor("external_chain", func(name string, opts json.RawMessage) processors.Processor {
		return NewExternalChainProcessorFromOptions(name, opts)
	})
}
