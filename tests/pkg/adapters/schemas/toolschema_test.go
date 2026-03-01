package schemas_test

import (
	"encoding/json"
	"testing"

	"voila-go/pkg/adapters/schemas"
)

func TestAdapterTypeConstants(t *testing.T) {
	if schemas.AdapterTypeGemini != "gemini" || schemas.AdapterTypeShim != "shim" {
		t.Errorf("AdapterType constants: gemini=%q shim=%q", schemas.AdapterTypeGemini, schemas.AdapterTypeShim)
	}
}

func TestFunctionSchema_JSONRoundTrip(t *testing.T) {
	s := schemas.FunctionSchema{
		Name:        "get_weather",
		Description: "Get weather",
		Parameters:  map[string]any{"type": "object"},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out schemas.FunctionSchema
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Name != s.Name || out.Description != s.Description {
		t.Errorf("round-trip: got %+v", out)
	}
}

func TestToolsSchema_ZeroValue(t *testing.T) {
	var ts schemas.ToolsSchema
	if ts.StandardTools != nil {
		t.Error("zero ToolsSchema StandardTools should be nil")
	}
	ts.StandardTools = []schemas.FunctionSchema{{Name: "x"}}
	if len(ts.StandardTools) != 1 {
		t.Errorf("StandardTools len = %d", len(ts.StandardTools))
	}
}
