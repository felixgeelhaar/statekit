package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	mcpgo "go.klarlabs.de/mcp"
	statekmcp "go.klarlabs.de/statekit/mcp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	srv := statekmcp.NewServer()
	return mcpgo.ServeStdio(ctx, srv)
}
