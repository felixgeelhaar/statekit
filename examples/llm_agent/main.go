// Package main demonstrates a deterministic LLM agent runtime
// using statekit + aiplugin.
//
// Pattern: a 4-state RAG-style pipeline where each stage is a
// statekit transition and LLM observability lives in plugins. The
// machine is deterministic (transitions fire on declared events,
// guards are pure functions); the LLM nondeterminism is contained
// inside actions that produce events with cost/token payloads.
//
// Why this matters:
//
//   - Replay: the event log replays the exact agent run without
//     re-calling the model.
//   - Observability: TokenCounter aggregates spend; PromptRecorder
//     captures every prompt/response for audit.
//   - HITL: the awaiting_review state waits on an APPROVE/REJECT
//     event from a human — no special primitive needed.
//   - Audit-grade compliance: paired with event sourcing, every
//     decision is reproducible.
package main

import (
	"fmt"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/aiplugin"
)

// AgentContext holds the agent's working state across transitions.
type AgentContext struct {
	Question  string
	Documents []string
	Answer    string
	Approved  bool
}

// Event types for the RAG pipeline.
const (
	EventStart       statekit.EventType = "START"
	EventRetrieved   statekit.EventType = "RETRIEVED"
	EventGenerated   statekit.EventType = "GENERATED"
	EventApprove     statekit.EventType = "APPROVE"
	EventReject      statekit.EventType = "REJECT"
	EventRetrieveErr statekit.EventType = "RETRIEVE_ERROR"
	EventGenerateErr statekit.EventType = "GENERATE_ERROR"
)

func buildAgent() *statekit.MachineConfig[AgentContext] {
	machine, err := statekit.NewMachine[AgentContext]("rag-agent").
		WithInitial("idle").
		WithAction("recordRetrieval", func(c *AgentContext, e statekit.Event) {
			if payload, ok := e.Payload.(map[string]any); ok {
				if docs, ok := payload["documents"].([]string); ok {
					c.Documents = docs
				}
			}
		}).
		WithAction("recordAnswer", func(c *AgentContext, e statekit.Event) {
			if payload, ok := e.Payload.(map[string]any); ok {
				if a, ok := payload[aiplugin.KeyResponse].(string); ok {
					c.Answer = a
				}
			}
		}).
		WithAction("approve", func(c *AgentContext, _ statekit.Event) {
			c.Approved = true
		}).
		WithGuard("hasDocuments", func(_ AgentContext, e statekit.Event) bool {
			// Guard checks payload directly because actions run AFTER
			// guard evaluation — context-state isn't yet populated.
			payload, ok := e.Payload.(map[string]any)
			if !ok {
				return false
			}
			docs, ok := payload["documents"].([]string)
			return ok && len(docs) > 0
		}).
		State("idle").
		On(EventStart).Target("retrieving").
		Done().
		State("retrieving").
		On(EventRetrieved).Target("generating").Do("recordRetrieval").Guard("hasDocuments").
		On(EventRetrieveErr).Target("failed").
		Done().
		State("generating").
		On(EventGenerated).Target("awaiting_review").Do("recordAnswer").
		On(EventGenerateErr).Target("failed").
		Done().
		State("awaiting_review").
		On(EventApprove).Target("done").Do("approve").
		On(EventReject).Target("retrieving").
		Done().
		State("done").Final().Done().
		State("failed").Final().Done().
		Build()
	if err != nil {
		panic(err)
	}
	return machine
}

func main() {
	machine := buildAgent()
	interp := statekit.NewInterpreter(machine)

	// Wire AI observability plugins.
	tokens := aiplugin.NewTokenCounter[AgentContext]()
	prompts := aiplugin.NewPromptRecorder[AgentContext]()
	interp.Use(tokens)
	interp.Use(prompts)

	defer func() { _ = interp.Close() }()
	interp.Start()

	interp.UpdateContext(func(c *AgentContext) {
		c.Question = "What is statekit?"
	})

	// Stage 1: kick off retrieval.
	interp.Send(statekit.Event{Type: EventStart})

	// Stage 2: pretend retrieval returned 3 documents + token usage.
	interp.Send(statekit.Event{
		Type: EventRetrieved,
		Payload: map[string]any{
			"documents":             []string{"doc1", "doc2", "doc3"},
			aiplugin.KeyModel:       "embedding-model-v1",
			aiplugin.KeyInputTokens: 42,
			aiplugin.KeyInputCost:   0.0001,
		},
	})

	// Stage 3: pretend generation produced an answer with full
	// prompt + response captured for replay.
	interp.Send(statekit.Event{
		Type: EventGenerated,
		Payload: map[string]any{
			aiplugin.KeyPrompt:       "Q: What is statekit?\nDocs: doc1, doc2, doc3",
			aiplugin.KeyResponse:     "Statekit is a Go-native statechart execution engine.",
			aiplugin.KeyModel:        "claude-opus-4-7",
			aiplugin.KeyInputTokens:  150,
			aiplugin.KeyOutputTokens: 28,
			aiplugin.KeyInputCost:    0.0030,
			aiplugin.KeyOutputCost:   0.0140,
		},
	})

	// Stage 4: HITL approval.
	interp.Send(statekit.Event{Type: EventApprove})

	state := interp.State()
	fmt.Printf("Final state:    %s\n", state.Value)
	fmt.Printf("Done:           %v\n", interp.Done())
	fmt.Printf("Approved:       %v\n", state.Context.Approved)
	fmt.Printf("Answer:         %q\n", state.Context.Answer)
	fmt.Printf("Total tokens:   %d\n", tokens.TotalTokens())
	fmt.Printf("Cost USD:       $%.4f\n", tokens.CostUSD())
	fmt.Printf("Prompt records: %d\n", prompts.Len())
}
