package mcp

import (
	mcpgo "go.klarlabs.de/mcp"
	"go.klarlabs.de/mcp/server"
	"go.klarlabs.de/statekit/viz"
)

// NewServer creates a new MCP server with all statekit tools and resources registered.
func NewServer() *server.Server {
	srv := mcpgo.NewServer(server.Info{
		Name:    "statekit",
		Version: "1.0.0",
		Capabilities: server.Capabilities{
			Tools:     true,
			Resources: true,
		},
	})

	reg := NewRegistry()
	registerTools(srv, reg)
	registerResources(srv)

	return srv
}

func registerTools(srv *server.Server, reg *Registry) {
	srv.Tool("create_machine").
		Description("Create a state machine from a Statekit Native JSON definition").
		Handler(handleCreateMachine(reg))

	srv.Tool("list_machines").
		Description("List all running state machine instances").
		ReadOnly().
		OutputSchema(MachineListOutput{}).
		Handler(handleListMachines(reg))

	srv.Tool("get_state").
		Description("Get the current state of a machine instance").
		ReadOnly().
		OutputSchema(StateOutput{}).
		Handler(handleGetState(reg))

	srv.Tool("send_event").
		Description("Send an event to a machine instance, triggering a state transition").
		Handler(handleSendEvent(reg))

	srv.Tool("get_context").
		Description("Get the current context data of a machine instance").
		ReadOnly().
		OutputSchema(Ctx{}).
		Handler(handleGetContext(reg))

	srv.Tool("get_machine_data").
		Description("Get the full machine definition data as JSON for a running instance").
		ReadOnly().
		UIResource("ui://statekit/visualizer").
		OutputSchema(viz.VizMachine{}).
		Handler(handleGetMachineData(reg))

	srv.Tool("validate_machine").
		Description("Validate a Statekit Native JSON machine definition for structural issues").
		ReadOnly().
		OutputSchema(ValidateOutput{}).
		Handler(handleValidateMachine(reg))

	srv.Tool("export_machine").
		Description("Export a machine in JSON, Mermaid, or ASCII format").
		ReadOnly().
		Handler(handleExportMachine(reg))

	srv.Tool("reset_machine").
		Description("Reset a machine instance back to its initial state").
		Handler(handleResetMachine(reg))

	srv.Tool("delete_machine").
		Description("Delete a machine instance and stop its interpreter").
		Handler(handleDeleteMachine(reg))
}
