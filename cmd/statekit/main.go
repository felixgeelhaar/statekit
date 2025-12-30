// Package main provides the statekit CLI tool for visualizing state machines.
package main

import (
	"os"

	"github.com/felixgeelhaar/statekit/cmd/statekit/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
