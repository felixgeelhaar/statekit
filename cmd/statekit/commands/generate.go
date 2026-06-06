package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.klarlabs.de/statekit/generate"
)

// GenerateCmd creates the generate command.
func GenerateCmd() *cobra.Command {
	var (
		outputFile  string
		packageName string
		typeName    string
		contextType string
	)

	cmd := &cobra.Command{
		Use:   "generate [file.json]",
		Short: "Generate Go code from XState JSON",
		Long: `Generate Go code from an XState JSON state machine definition.

The generated code includes:
- A Build<TypeName>() function that constructs the state machine
- Stub functions for all actions and guards
- Type alias for the context type

Examples:
  # Generate from file
  statekit generate machine.json -o machine.go

  # Generate with custom package and type name
  statekit generate machine.json -p mypackage -t OrderWorkflow -c OrderContext

  # Read from stdin
  cat machine.json | statekit generate -o machine.go`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var r io.Reader

			if len(args) == 0 || args[0] == "-" {
				r = os.Stdin
			} else {
				f, err := os.Open(args[0])
				if err != nil {
					return fmt.Errorf("open file: %w", err)
				}
				defer func() { _ = f.Close() }()
				r = f

				// Infer defaults from filename if not specified
				if typeName == "" {
					base := filepath.Base(args[0])
					name := strings.TrimSuffix(base, filepath.Ext(base))
					typeName = toPascalCase(name)
				}
			}

			if typeName == "" {
				typeName = "Machine"
			}

			gen := generate.NewGenerator(packageName, typeName, contextType)
			code, err := gen.Generate(r)
			if err != nil {
				return err
			}

			if outputFile == "" || outputFile == "-" {
				fmt.Print(string(code))
				return nil
			}

			if err := os.WriteFile(outputFile, code, 0o600); err != nil {
				return fmt.Errorf("write file: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Generated %s\n", outputFile)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringVarP(&packageName, "package", "p", "main", "Go package name")
	cmd.Flags().StringVarP(&typeName, "type", "t", "", "Type name for the machine (default: inferred from filename)")
	cmd.Flags().StringVarP(&contextType, "context", "c", "struct{}", "Context type name")

	return cmd
}

// toPascalCase converts a string to PascalCase.
func toPascalCase(s string) string {
	var result strings.Builder
	capitalize := true

	for _, r := range s {
		if r == '_' || r == '-' || r == ' ' {
			capitalize = true
			continue
		}
		if capitalize {
			result.WriteRune(toUpper(r))
			capitalize = false
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	return r
}
