package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	mcpgo "github.com/felixgeelhaar/mcp-go"
	statekmcp "github.com/felixgeelhaar/statekit/mcp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	srv := statekmcp.NewServer()

	if err := mcpgo.ServeStdio(ctx, srv); err != nil {
		log.Fatal(err)
	}
}
