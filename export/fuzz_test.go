package export

import (
	"testing"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/viz"
)

// FuzzExportJSONRoundTrip builds a tiny machine, exports Native JSON, and
// ensures ParseNativeJSON never panics on the exporter output (and that
// garbage inputs still fail cleanly).
func FuzzExportJSONRoundTrip(f *testing.F) {
	machine, err := statekit.NewMachine[struct{}]("fuzz").
		WithInitial("a").
		State("a").On("GO").Target("b").Done().
		State("b").Final().Done().
		Build()
	if err != nil {
		f.Fatal(err)
	}
	raw, err := NewNativeExporter(machine).ExportJSON()
	if err != nil {
		f.Fatal(err)
	}

	f.Add([]byte(raw))
	f.Add([]byte(`{"id":"fuzz","initial":"a","states":{"a":{},"b":{"type":"final"}}}`))
	f.Add([]byte(`{`))
	f.Add([]byte(``))
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = viz.ParseNativeJSON(data)
	})
}
