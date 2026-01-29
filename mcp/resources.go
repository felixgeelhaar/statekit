package mcp

import (
	"context"
	_ "embed"
	"strings"

	"github.com/felixgeelhaar/mcp-go/server"
)

//go:embed ui/visualizer.html
var visualizerHTML string

//go:embed ui/vendor/vue.global.prod.js
var vendorVue string

//go:embed ui/vendor/cytoscape.min.js
var vendorCytoscape string

//go:embed ui/vendor/dagre.min.js
var vendorDagre string

//go:embed ui/vendor/cytoscape-dagre.min.js
var vendorCytoscapeDagre string

func assembledVisualizerHTML() string {
	scripts := "<script>" + vendorVue + "</script>\n" +
		"    <script>" + vendorCytoscape + "</script>\n" +
		"    <script>" + vendorDagre + "</script>\n" +
		"    <script>" + vendorCytoscapeDagre + "</script>"
	return strings.Replace(visualizerHTML, "<!-- VENDOR_SCRIPTS -->", scripts, 1)
}

func registerResources(srv *server.Server) {
	srv.Resource("ui://statekit/visualizer").
		Name("Statekit Visualizer").
		Description("Interactive Cytoscape.js state machine visualizer").
		MimeType("text/html").
		Handler(func(_ context.Context, _ string, _ map[string]string) (*server.ResourceContent, error) {
			return &server.ResourceContent{
				URI:      "ui://statekit/visualizer",
				MimeType: "text/html",
				Text:     assembledVisualizerHTML(),
			}, nil
		})
}
