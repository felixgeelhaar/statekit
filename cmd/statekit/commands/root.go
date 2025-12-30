// Package commands provides CLI commands for the statekit tool.
package commands

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "statekit",
	Short: "State machine toolkit for Go",
	Long: `Statekit is a Go-native statechart execution engine with XState JSON
compatibility for visualization. This CLI provides tools for visualizing
and inspecting state machines.

Examples:
  statekit viz machine.json              # Visualize as ASCII diagram
  statekit viz -f mermaid machine.json   # Output Mermaid diagram
  statekit viz -f tui machine.json       # Interactive TUI explorer
  cat machine.json | statekit viz        # Read from stdin`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(vizCmd)
	rootCmd.AddCommand(versionCmd)
}
