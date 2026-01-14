package export

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/felixgeelhaar/statekit/viz"
)

// mockExporter implements MachineExporter for testing
type mockExporter struct {
	id string
}

func (m *mockExporter) Export() *viz.VizMachine {
	return &viz.VizMachine{
		ID: m.id,
		States: map[string]*viz.VizState{
			"idle": {
				ID: "idle",
				Transitions: []viz.VizTransition{
					{Event: "NEXT", Target: "next"},
				},
			},
		},
	}
}

func TestExportAll(t *testing.T) {
	exporters := map[string]MachineExporter{
		"machine1": &mockExporter{id: "machine1"},
		"machine2": &mockExporter{id: "machine2"},
	}

	// Test case 1: Export all
	var buf bytes.Buffer
	opts := ExportOptions{
		Output: &buf,
	}

	if err := ExportAll(exporters, opts); err != nil {
		t.Fatalf("ExportAll failed: %v", err)
	}

	var result map[string]*viz.VizMachine
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 machines, got %d", len(result))
	}
	if result["machine1"].ID != "machine1" {
		t.Errorf("expected machine1 ID, got %s", result["machine1"].ID)
	}

	// Test case 2: Export specific machine
	buf.Reset()
	opts.MachineID = "machine1"

	if err := ExportAll(exporters, opts); err != nil {
		t.Fatalf("ExportAll(machine1) failed: %v", err)
	}

	// When exporting single machine, it exports the machine object directly, not a map
	var machine viz.VizMachine
	if err := json.Unmarshal(buf.Bytes(), &machine); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if machine.ID != "machine1" {
		t.Errorf("expected machine1 ID, got %s", machine.ID)
	}

	// Test case 3: Machine not found
	buf.Reset()
	opts.MachineID = "unknown"
	if err := ExportAll(exporters, opts); err == nil {
		t.Error("expected error for unknown machine")
	}
}

func TestRunCLI_List(t *testing.T) {
	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exporters := map[string]MachineExporter{
		"machine1": &mockExporter{id: "machine1"},
	}

	err := RunCLI(exporters, []string{"-list"})
	
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("RunCLI failed: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if output == "" {
		t.Error("expected output")
	}
	// Output should contain machine ID
	// Note: output order is map order (random), but we only have 1
}

func TestRunCLI_Export(t *testing.T) {
	exporters := map[string]MachineExporter{
		"machine1": &mockExporter{id: "machine1"},
	}

	// Create temp file
	tmpfile, err := os.CreateTemp("", "export_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// Run CLI with -o
	err = RunCLI(exporters, []string{"-o", tmpfile.Name(), "-machine", "machine1"})
	if err != nil {
		t.Fatalf("RunCLI failed: %v", err)
	}

	// Verify file content
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	var machine viz.VizMachine
	if err := json.Unmarshal(content, &machine); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if machine.ID != "machine1" {
		t.Errorf("expected machine1, got %s", machine.ID)
	}
}