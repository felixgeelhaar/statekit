package export

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockExporter implements MachineExporter for testing
type mockExporter struct {
	id      string
	initial string
}

func (m *mockExporter) Export() (*XStateMachine, error) {
	return &XStateMachine{
		ID:      m.id,
		Initial: m.initial,
		States: map[string]XStateNode{
			m.initial: {
				On: map[string]XStateTransition{
					"NEXT": {Target: "done"},
				},
			},
			"done": {Type: "final"},
		},
	}, nil
}

func TestExportMachine(t *testing.T) {
	exporter := &mockExporter{id: "test", initial: "idle"}

	var buf bytes.Buffer
	opts := ExportOptions{
		PrettyPrint: false,
		Output:      &buf,
	}

	err := ExportMachine(exporter, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse output
	var machine XStateMachine
	if err := json.Unmarshal(buf.Bytes(), &machine); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if machine.ID != "test" {
		t.Errorf("expected ID 'test', got %q", machine.ID)
	}
	if machine.Initial != "idle" {
		t.Errorf("expected Initial 'idle', got %q", machine.Initial)
	}
}

func TestExportMachine_PrettyPrint(t *testing.T) {
	exporter := &mockExporter{id: "test", initial: "idle"}

	var buf bytes.Buffer
	opts := ExportOptions{
		PrettyPrint: true,
		Indent:      "    ",
		Output:      &buf,
	}

	err := ExportMachine(exporter, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check for indentation
	if !strings.Contains(output, "    ") {
		t.Error("expected indented output")
	}

	// Verify it's still valid JSON
	var machine XStateMachine
	if err := json.Unmarshal([]byte(output), &machine); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
}

func TestExportAll(t *testing.T) {
	machines := map[string]MachineExporter{
		"machine1": &mockExporter{id: "machine1", initial: "start"},
		"machine2": &mockExporter{id: "machine2", initial: "begin"},
	}

	var buf bytes.Buffer
	opts := ExportOptions{
		Output: &buf,
	}

	err := ExportAll(machines, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse output as map
	var result map[string]XStateMachine
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 machines, got %d", len(result))
	}

	if result["machine1"].Initial != "start" {
		t.Errorf("machine1 initial mismatch")
	}
	if result["machine2"].Initial != "begin" {
		t.Errorf("machine2 initial mismatch")
	}
}

func TestExportAll_SingleMachine(t *testing.T) {
	machines := map[string]MachineExporter{
		"machine1": &mockExporter{id: "machine1", initial: "start"},
		"machine2": &mockExporter{id: "machine2", initial: "begin"},
	}

	var buf bytes.Buffer
	opts := ExportOptions{
		MachineID: "machine1",
		Output:    &buf,
	}

	err := ExportAll(machines, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse output as single machine (not a map)
	var machine XStateMachine
	if err := json.Unmarshal(buf.Bytes(), &machine); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if machine.ID != "machine1" {
		t.Errorf("expected ID 'machine1', got %q", machine.ID)
	}
}

func TestExportAll_MachineNotFound(t *testing.T) {
	machines := map[string]MachineExporter{
		"machine1": &mockExporter{id: "machine1", initial: "start"},
	}

	var buf bytes.Buffer
	opts := ExportOptions{
		MachineID: "nonexistent",
		Output:    &buf,
	}

	err := ExportAll(machines, opts)
	if err == nil {
		t.Fatal("expected error for nonexistent machine")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestRunCLI_List(t *testing.T) {
	machines := map[string]MachineExporter{
		"alpha": &mockExporter{id: "alpha", initial: "a"},
		"beta":  &mockExporter{id: "beta", initial: "b"},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunCLI(machines, []string{"-list"})

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "alpha") {
		t.Error("expected 'alpha' in list output")
	}
	if !strings.Contains(output, "beta") {
		t.Error("expected 'beta' in list output")
	}
}

func TestRunCLI_Pretty(t *testing.T) {
	machines := map[string]MachineExporter{
		"test": &mockExporter{id: "test", initial: "idle"},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunCLI(machines, []string{"-pretty", "-machine=test"})

	_ = w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Check for default indentation
	if !strings.Contains(output, "  ") {
		t.Error("expected indented output")
	}
}

func TestRunCLI_OutputFile(t *testing.T) {
	machines := map[string]MachineExporter{
		"test": &mockExporter{id: "test", initial: "idle"},
	}

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "output.json")

	err := RunCLI(machines, []string{"-o", outFile, "-machine=test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read and verify output file
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var machine XStateMachine
	if err := json.Unmarshal(data, &machine); err != nil {
		t.Fatalf("invalid JSON in output file: %v", err)
	}

	if machine.ID != "test" {
		t.Errorf("expected ID 'test', got %q", machine.ID)
	}
}

func TestRunCLI_InvalidFlag(t *testing.T) {
	machines := map[string]MachineExporter{}

	err := RunCLI(machines, []string{"-invalid-flag"})
	if err == nil {
		t.Error("expected error for invalid flag")
	}
}

func TestDefaultExportOptions(t *testing.T) {
	opts := DefaultExportOptions()

	if opts.PrettyPrint {
		t.Error("expected PrettyPrint to be false by default")
	}
	if opts.Indent != "  " {
		t.Errorf("expected Indent '  ', got %q", opts.Indent)
	}
	if opts.Output != os.Stdout {
		t.Error("expected Output to be os.Stdout")
	}
	if opts.MachineID != "" {
		t.Errorf("expected empty MachineID, got %q", opts.MachineID)
	}
}
