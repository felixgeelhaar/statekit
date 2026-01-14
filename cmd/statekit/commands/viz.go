package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/felixgeelhaar/statekit/viz"
	"github.com/felixgeelhaar/statekit/viz/ascii"
	"github.com/felixgeelhaar/statekit/viz/goparser"
	"github.com/felixgeelhaar/statekit/viz/html"
	"github.com/felixgeelhaar/statekit/viz/mermaid"
	"github.com/felixgeelhaar/statekit/viz/tui"
)

var (
	vizFormat    string
	vizOutput    string
	vizNoUnicode bool
	vizDirection string
	vizGoPackage string
	vizGoType    string
)

var vizCmd = &cobra.Command{
	Use:   "viz [file]",
	Short: "Visualize a state machine",
	Long: `Visualize a state machine from Statekit JSON input or Go source code.

Input can be provided as:
  - A file path argument (Statekit JSON)
  - Piped via stdin (Statekit JSON)
  - A Go package path (--go-package)

Output formats:
  - ascii   : Terminal-friendly box diagrams (default)
  - mermaid : Mermaid stateDiagram-v2 markdown
  - html    : Standalone HTML visualizer with interactive simulation
  - tui     : Interactive terminal UI with navigation

Examples:
  statekit viz machine.json
  statekit viz machine.json --format mermaid
  statekit viz machine.json --format html -o machine.html
  statekit viz machine.json --format tui
  cat machine.json | statekit viz
  statekit viz machine.json --no-unicode
  statekit viz --go-package ./examples/order_workflow
  statekit viz --go-package ./... --go-type OrderMachine`,
	Args: cobra.MaximumNArgs(1),
	RunE: runViz,
}

func init() {
	vizCmd.Flags().StringVarP(&vizFormat, "format", "f", "ascii",
		"Output format: ascii, mermaid, html, tui")
	vizCmd.Flags().StringVarP(&vizOutput, "output", "o", "",
		"Output file (default: stdout)")
	vizCmd.Flags().BoolVar(&vizNoUnicode, "no-unicode", false,
		"Use ASCII-only characters (for ascii format)")
	vizCmd.Flags().StringVarP(&vizDirection, "direction", "d", "TB",
		"Diagram direction: TB (top-bottom), LR (left-right)")
	vizCmd.Flags().StringVar(&vizGoPackage, "go-package", "",
		"Parse Go package for state machine definitions")
	vizCmd.Flags().StringVar(&vizGoType, "go-type", "",
		"Filter to specific type name when using --go-package")
}

func runViz(cmd *cobra.Command, args []string) error {
	var machines []*viz.VizMachine

	// Determine input source
	if vizGoPackage != "" {
		// Parse Go package
		parser := goparser.NewParser()
		if vizGoType != "" {
			parser = parser.WithTypeFilter(vizGoType)
		}

		var err error
		machines, err = parser.ParsePackage(vizGoPackage)
		if err != nil {
			return fmt.Errorf("parse Go package: %w", err)
		}

		if len(machines) == 0 {
			return fmt.Errorf("no state machines found in %s", vizGoPackage)
		}
	} else {
		// Parse JSON input
		var input []byte
		var err error

		if len(args) > 0 {
			input, err = os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
		} else {
			// Check if stdin has data
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) != 0 {
				return fmt.Errorf("no input provided: specify a file, pipe JSON via stdin, or use --go-package")
			}
			input, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
		}

		if len(input) == 0 {
			return fmt.Errorf("empty input")
		}

		// Parse Statekit JSON
		machine, err := viz.ParseNativeJSON(input)
		if err != nil {
			return fmt.Errorf("parse JSON: %w", err)
		}
		machines = []*viz.VizMachine{machine}
	}

	// Handle TUI separately (interactive, single machine only)
	if vizFormat == "tui" {
		if len(machines) > 1 {
			fmt.Fprintf(os.Stderr, "TUI mode: showing first of %d machines\n", len(machines))
		}
		return tui.Run(machines[0])
	}

	// Render all machines
	var outputs []string
	for _, machine := range machines {
		output, err := renderMachine(machine)
		if err != nil {
			return err
		}
		outputs = append(outputs, output)
	}

	// Combine outputs with separator
	var result string
	for i, output := range outputs {
		if i > 0 {
			result += "\n---\n\n"
		}
		result += output
	}

	// Write output
	if vizOutput != "" {
		if err := os.WriteFile(vizOutput, []byte(result), 0o600); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Written to %s\n", vizOutput)
	} else {
		fmt.Print(result)
	}

	return nil
}

func renderMachine(machine *viz.VizMachine) (string, error) {
	switch vizFormat {
	case "ascii":
		r := ascii.NewRenderer()
		r.UseUnicode = !vizNoUnicode
		return r.Render(machine), nil

	case "mermaid":
		r := mermaid.NewRenderer()
		r.Direction = vizDirection
		return r.Render(machine), nil

	case "html":
		r := html.NewRenderer()
		return r.Render(machine)

	default:
		return "", fmt.Errorf("unknown format: %s (supported: ascii, mermaid, html, tui)", vizFormat)
	}
}
