package export

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

// MachineExporter is implemented by types that can export to XState JSON format.
// XStateExporter[C] implements this interface.
type MachineExporter interface {
	Export() (*XStateMachine, error)
}

// ExportOptions configures the export behavior.
type ExportOptions struct {
	// PrettyPrint enables indented JSON output
	PrettyPrint bool

	// Indent is the string used for indentation (default: "  ")
	Indent string

	// Output is where JSON will be written (default: os.Stdout)
	Output io.Writer

	// MachineID filters to a specific machine ID (empty = export all)
	MachineID string
}

// DefaultExportOptions returns options with sensible defaults.
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		PrettyPrint: false,
		Indent:      "  ",
		Output:      os.Stdout,
		MachineID:   "",
	}
}

// ExportMachine exports a single machine to JSON.
func ExportMachine(exporter MachineExporter, opts ExportOptions) error {
	machine, err := exporter.Export()
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	return writeJSON(machine, opts)
}

// ExportAll exports multiple machines to JSON.
// The output is a JSON object with machine IDs as keys.
func ExportAll(machines map[string]MachineExporter, opts ExportOptions) error {
	// Filter to specific machine if requested
	if opts.MachineID != "" {
		exporter, ok := machines[opts.MachineID]
		if !ok {
			return fmt.Errorf("machine %q not found", opts.MachineID)
		}
		return ExportMachine(exporter, opts)
	}

	// Export all machines
	result := make(map[string]*XStateMachine)
	for id, exporter := range machines {
		machine, err := exporter.Export()
		if err != nil {
			return fmt.Errorf("export %q failed: %w", id, err)
		}
		result[id] = machine
	}

	return writeJSON(result, opts)
}

// writeJSON writes a value as JSON to the configured output.
func writeJSON(v any, opts ExportOptions) error {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	var data []byte
	var err error

	if opts.PrettyPrint {
		indent := opts.Indent
		if indent == "" {
			indent = "  "
		}
		data, err = json.MarshalIndent(v, "", indent)
	} else {
		data, err = json.Marshal(v)
	}

	if err != nil {
		return fmt.Errorf("JSON marshal failed: %w", err)
	}

	_, err = out.Write(data)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	// Add trailing newline for terminal output
	if _, err := out.Write([]byte("\n")); err != nil {
		return fmt.Errorf("write newline failed: %w", err)
	}

	return nil
}

// RunCLI provides a simple CLI for exporting machines.
// Usage: go run export_tool.go [-pretty] [-indent=STR] [-machine=ID] [-o=FILE]
func RunCLI(machines map[string]MachineExporter, args []string) error {
	fs := flag.NewFlagSet("statekit-export", flag.ContinueOnError)

	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	indent := fs.String("indent", "  ", "Indentation string (used with -pretty)")
	machineID := fs.String("machine", "", "Export only this machine ID")
	output := fs.String("o", "", "Output file (default: stdout)")
	list := fs.Bool("list", false, "List available machine IDs")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// List mode
	if *list {
		fmt.Println("Available machines:")
		for id := range machines {
			fmt.Printf("  - %s\n", id)
		}
		return nil
	}

	// Build options
	opts := ExportOptions{
		PrettyPrint: *pretty,
		Indent:      *indent,
		MachineID:   *machineID,
		Output:      os.Stdout,
	}

	// Handle output file
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer func() { _ = f.Close() }()
		opts.Output = f
	}

	return ExportAll(machines, opts)
}
