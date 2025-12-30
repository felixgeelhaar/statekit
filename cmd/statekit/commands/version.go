package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("statekit version %s\n", Version)
	},
}
