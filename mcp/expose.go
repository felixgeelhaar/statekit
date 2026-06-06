package mcp

import (
	"fmt"

	"go.klarlabs.de/mcp/server"
	"go.klarlabs.de/statekit"
)

// ExposeInput identifies an event to send to an exposed interpreter.
type ExposeInput struct {
	Event   string         `json:"event" jsonschema:"description=Event type to send"`
	Payload map[string]any `json:"payload,omitempty" jsonschema:"description=Optional event payload"`
}

// ExposeStateOutput describes the current state of an exposed interpreter.
type ExposeStateOutput struct {
	CurrentState string `json:"currentState"`
	Done         bool   `json:"done"`
}

// ExposeSendOutput describes the result of sending an event to an exposed interpreter.
type ExposeSendOutput struct {
	PreviousState string `json:"previousState"`
	CurrentState  string `json:"currentState"`
	Transitioned  bool   `json:"transitioned"`
	Done          bool   `json:"done"`
}

// ExposeContextOutput[C] wraps the typed machine context for JSON return.
type ExposeContextOutput[C any] struct {
	Context C `json:"context"`
}

// ExposeMatchInput asks whether the interpreter is in or under a state.
type ExposeMatchInput struct {
	StateID string `json:"state_id" jsonschema:"description=State ID to check (matches current state or any ancestor)"`
}

// ExposeMatchOutput is the result of a Matches check.
type ExposeMatchOutput struct {
	Matches bool `json:"matches"`
}

// ExposeSendEvent fires an event on the interpreter and reports the
// transition result.
func ExposeSendEvent[C any](interp *statekit.Interpreter[C], input ExposeInput) ExposeSendOutput {
	prev := string(interp.State().Value)
	evt := statekit.Event{Type: statekit.EventType(input.Event)}
	if input.Payload != nil {
		evt.Payload = input.Payload
	}
	interp.Send(evt)
	cur := string(interp.State().Value)
	return ExposeSendOutput{
		PreviousState: prev,
		CurrentState:  cur,
		Transitioned:  prev != cur,
		Done:          interp.Done(),
	}
}

// ExposeGetState returns the current state and done flag.
func ExposeGetState[C any](interp *statekit.Interpreter[C]) ExposeStateOutput {
	return ExposeStateOutput{
		CurrentState: string(interp.State().Value),
		Done:         interp.Done(),
	}
}

// ExposeGetContext returns the typed machine context wrapped for JSON.
func ExposeGetContext[C any](interp *statekit.Interpreter[C]) ExposeContextOutput[C] {
	return ExposeContextOutput[C]{Context: interp.State().Context}
}

// ExposeMatches returns whether the interpreter is in or under the given state.
func ExposeMatches[C any](interp *statekit.Interpreter[C], input ExposeMatchInput) ExposeMatchOutput {
	return ExposeMatchOutput{
		Matches: interp.Matches(statekit.StateID(input.StateID)),
	}
}

// ExposeInterpreter registers MCP tools that expose a running statekit
// interpreter as an agent-callable surface. Inverts the existing
// authoring-direction MCP server: instead of letting Claude create
// machines from JSON, this lets a running machine be driven by an
// MCP-speaking agent.
//
// Registers four tools under the given prefix:
//
//   - <prefix>.send_event  — fire an event, returning the transition result
//   - <prefix>.get_state   — read current state and done flag
//   - <prefix>.get_context — read the typed machine context as JSON
//   - <prefix>.matches     — test whether the interpreter is in or under a state
//
// The interpreter must already be Started. Caller is responsible for
// the interpreter's lifecycle (Stop/Close).
func ExposeInterpreter[C any](srv *server.Server, prefix string, interp *statekit.Interpreter[C]) {
	srv.Tool(prefix + ".send_event").
		Description(fmt.Sprintf("Send an event to the %q machine and report the transition result", prefix)).
		Handler(func(input ExposeInput) (ExposeSendOutput, error) {
			return ExposeSendEvent(interp, input), nil
		})

	srv.Tool(prefix + ".get_state").
		Description(fmt.Sprintf("Get the current state of the %q machine", prefix)).
		ReadOnly().
		Handler(func(_ struct{}) (ExposeStateOutput, error) {
			return ExposeGetState(interp), nil
		})

	srv.Tool(prefix + ".get_context").
		Description(fmt.Sprintf("Get the typed machine context of the %q machine as JSON", prefix)).
		ReadOnly().
		Handler(func(_ struct{}) (ExposeContextOutput[C], error) {
			return ExposeGetContext(interp), nil
		})

	srv.Tool(prefix + ".matches").
		Description(fmt.Sprintf("Check whether the %q machine is in or under a given state", prefix)).
		ReadOnly().
		Handler(func(input ExposeMatchInput) (ExposeMatchOutput, error) {
			return ExposeMatches(interp, input), nil
		})
}
