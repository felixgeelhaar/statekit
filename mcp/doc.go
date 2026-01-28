// Package mcp provides an MCP (Model Context Protocol) server for creating,
// managing, and visualizing statekit state machines.
//
// It exposes tools for creating machines from Native JSON definitions,
// sending events, querying state, validating definitions, and exporting
// to various visualization formats.
//
// Usage:
//
//	srv := mcp.NewServer()
//	mcpgo.ServeStdio(ctx, srv)
package mcp
