package main

import (
	"testing"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/aiplugin"
)

func TestAgent_HappyPath(t *testing.T) {
	t.Parallel()
	machine := buildAgent()
	interp := statekit.NewInterpreter(machine)
	tokens := aiplugin.NewTokenCounter[AgentContext]()
	prompts := aiplugin.NewPromptRecorder[AgentContext]()
	interp.Use(tokens)
	interp.Use(prompts)
	defer func() { _ = interp.Close() }()
	interp.Start()

	interp.Send(statekit.Event{Type: EventStart})
	interp.Send(statekit.Event{
		Type: EventRetrieved,
		Payload: map[string]any{
			"documents":             []string{"a", "b"},
			aiplugin.KeyInputTokens: 10,
			aiplugin.KeyInputCost:   0.01,
		},
	})
	interp.Send(statekit.Event{
		Type: EventGenerated,
		Payload: map[string]any{
			aiplugin.KeyPrompt:       "p",
			aiplugin.KeyResponse:     "r",
			aiplugin.KeyOutputTokens: 5,
			aiplugin.KeyOutputCost:   0.05,
		},
	})
	interp.Send(statekit.Event{Type: EventApprove})

	if got := string(interp.State().Value); got != "done" {
		t.Errorf("final state = %q, want done", got)
	}
	if !interp.Done() {
		t.Error("expected Done")
	}
	if !interp.State().Context.Approved {
		t.Error("expected Approved")
	}
	if got := tokens.TotalTokens(); got != 15 {
		t.Errorf("TotalTokens = %d, want 15", got)
	}
	if got := tokens.CostUSD(); got < 0.059 || got > 0.061 {
		t.Errorf("CostUSD = %v, want ~0.06", got)
	}
	if got := prompts.Len(); got != 1 {
		t.Errorf("prompt records = %d, want 1", got)
	}
}

func TestAgent_GuardRejectsEmptyDocs(t *testing.T) {
	t.Parallel()
	machine := buildAgent()
	interp := statekit.NewInterpreter(machine)
	defer func() { _ = interp.Close() }()
	interp.Start()

	interp.Send(statekit.Event{Type: EventStart})
	// Send retrieved with no documents — guard should reject.
	interp.Send(statekit.Event{
		Type:    EventRetrieved,
		Payload: map[string]any{"documents": []string{}},
	})

	if got := string(interp.State().Value); got != "retrieving" {
		t.Errorf("state = %q, want still retrieving (guard blocked)", got)
	}
}

func TestAgent_RejectLoopsBack(t *testing.T) {
	t.Parallel()
	machine := buildAgent()
	interp := statekit.NewInterpreter(machine)
	defer func() { _ = interp.Close() }()
	interp.Start()

	interp.Send(statekit.Event{Type: EventStart})
	interp.Send(statekit.Event{
		Type:    EventRetrieved,
		Payload: map[string]any{"documents": []string{"x"}},
	})
	interp.Send(statekit.Event{
		Type: EventGenerated,
		Payload: map[string]any{
			aiplugin.KeyResponse: "draft",
		},
	})
	interp.Send(statekit.Event{Type: EventReject})

	if got := string(interp.State().Value); got != "retrieving" {
		t.Errorf("after REJECT state = %q, want retrieving", got)
	}
}

func TestAgent_ErrorPath(t *testing.T) {
	t.Parallel()
	machine := buildAgent()
	interp := statekit.NewInterpreter(machine)
	defer func() { _ = interp.Close() }()
	interp.Start()

	interp.Send(statekit.Event{Type: EventStart})
	interp.Send(statekit.Event{Type: EventRetrieveErr})

	if got := string(interp.State().Value); got != "failed" {
		t.Errorf("state = %q, want failed", got)
	}
	if !interp.Done() {
		t.Error("expected Done in failed final state")
	}
}
