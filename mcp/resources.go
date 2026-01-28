package mcp

import (
	"context"
	_ "embed"

	"github.com/felixgeelhaar/mcp-go/server"
)

//go:embed ui/visualizer.html
var visualizerHTML string

func registerResources(srv *server.Server) {
	srv.Resource("ui://statekit/visualizer").
		Name("Statekit Visualizer").
		Description("Interactive Cytoscape.js state machine visualizer").
		MimeType("text/html").
		Handler(func(_ context.Context, _ string, _ map[string]string) (*server.ResourceContent, error) {
			return &server.ResourceContent{
				URI:      "ui://statekit/visualizer",
				MimeType: "text/html",
				Text:     visualizerHTML,
			}, nil
		})
}
