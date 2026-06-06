// Package main provides the statekit CLI tool for visualizing state machines.
package main

import (
	"os"

	"go.klarlabs.de/statekit/cmd/statekit/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
