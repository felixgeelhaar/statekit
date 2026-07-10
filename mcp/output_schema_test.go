package mcp

import (
	"testing"

	"go.klarlabs.de/mcp/schema"
	"go.klarlabs.de/statekit/viz"
)

// TestOutputSchemasGenerate guards every type advertised via OutputSchema.
// OutputSchema runs schema.Generate at registration time and, on error,
// silently drops the tool. This test asserts each advertised output type
// generates a schema without error so a broken type surfaces here rather
// than as a mysteriously missing tool.
func TestOutputSchemasGenerate(t *testing.T) {
	// sampleContext exercises the generic ExposeContextOutput[C] with a
	// concrete struct context, in addition to the map[string]any case.
	type sampleContext struct {
		Count int    `json:"count"`
		Name  string `json:"name"`
	}

	cases := []struct {
		name    string
		example any
	}{
		{"MachineListOutput", MachineListOutput{}},
		{"StateOutput", StateOutput{}},
		{"ValidateOutput", ValidateOutput{}},
		{"Ctx", Ctx{}},
		{"VizMachine", viz.VizMachine{}},
		{"ExposeStateOutput", ExposeStateOutput{}},
		{"ExposeContextOutput[map]", ExposeContextOutput[map[string]any]{}},
		{"ExposeContextOutput[struct]", ExposeContextOutput[sampleContext]{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := schema.Generate(tc.example); err != nil {
				t.Fatalf("schema.Generate(%s) failed: %v", tc.name, err)
			}
		})
	}
}
