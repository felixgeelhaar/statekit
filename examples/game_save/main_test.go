package main

import (
	"encoding/json"
	"testing"

	"go.klarlabs.de/statekit"
)

func TestGameSave_BasicSnapshot(t *testing.T) {
	machine := buildGameMachine()
	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *GameContext) {
		ctx.PlayerName = "TestPlayer"
	})
	interp.Start()

	// Play a bit
	interp.Send(statekit.Event{Type: "NEW_GAME"})
	interp.Send(statekit.Event{Type: "START_QUEST"})

	// Take snapshot
	snapshot := interp.Snapshot()

	if snapshot.MachineID != "rpg_game" {
		t.Errorf("Expected machine ID 'rpg_game', got %s", snapshot.MachineID)
	}

	if snapshot.Context.PlayerName != "TestPlayer" {
		t.Errorf("Expected player name 'TestPlayer', got %s", snapshot.Context.PlayerName)
	}

	if snapshot.CurrentState != "traveling" {
		t.Errorf("Expected state 'traveling', got %s", snapshot.CurrentState)
	}

	interp.Stop()
}

func TestGameSave_JSONSerialization(t *testing.T) {
	machine := buildGameMachine()
	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *GameContext) {
		ctx.PlayerName = "TestPlayer"
	})
	interp.Start()

	interp.Send(statekit.Event{Type: "NEW_GAME"})
	interp.Send(statekit.Event{Type: "START_QUEST"})
	interp.Send(statekit.Event{Type: "FIGHT"})
	interp.Send(statekit.Event{Type: "WIN"})

	// Serialize snapshot
	snapshot := interp.Snapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot: %v", err)
	}

	// Deserialize
	var restored statekit.Snapshot[GameContext]
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	// Verify restored data
	if restored.Context.Score != snapshot.Context.Score {
		t.Errorf("Score mismatch: got %d, want %d", restored.Context.Score, snapshot.Context.Score)
	}

	if restored.CurrentState != snapshot.CurrentState {
		t.Errorf("State mismatch: got %s, want %s", restored.CurrentState, snapshot.CurrentState)
	}

	interp.Stop()
}

func TestGameSave_RestoreState(t *testing.T) {
	machine := buildGameMachine()

	// Play original game
	interp1 := statekit.NewInterpreter(machine)
	interp1.UpdateContext(func(ctx *GameContext) {
		ctx.PlayerName = "OriginalPlayer"
	})
	interp1.Start()

	interp1.Send(statekit.Event{Type: "NEW_GAME"})
	interp1.Send(statekit.Event{Type: "START_QUEST"})
	interp1.Send(statekit.Event{Type: "FIGHT"})
	interp1.Send(statekit.Event{Type: "WIN"})
	interp1.Send(statekit.Event{Type: "NEXT_LEVEL"})

	// Capture state
	snapshot := interp1.Snapshot()
	originalLevel := interp1.State().Context.Level
	originalScore := interp1.State().Context.Score
	interp1.Stop()

	// Create new interpreter and restore
	interp2 := statekit.NewInterpreter(machine)
	if err := interp2.Restore(snapshot); err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	// Verify restored state
	if interp2.State().Context.Level != originalLevel {
		t.Errorf("Level mismatch: got %d, want %d", interp2.State().Context.Level, originalLevel)
	}

	if interp2.State().Context.Score != originalScore {
		t.Errorf("Score mismatch: got %d, want %d", interp2.State().Context.Score, originalScore)
	}

	if interp2.State().Context.PlayerName != "OriginalPlayer" {
		t.Errorf("Player name mismatch: got %s, want 'OriginalPlayer'", interp2.State().Context.PlayerName)
	}

	// Verify can continue playing
	interp2.Send(statekit.Event{Type: "START_QUEST"})
	if interp2.State().Value != "traveling" {
		t.Errorf("Expected 'traveling' after START_QUEST, got %s", interp2.State().Value)
	}

	interp2.Stop()
}

func TestGameSave_RestoreMachineMismatch(t *testing.T) {
	machine := buildGameMachine()

	// Create snapshot from first interpreter
	interp1 := statekit.NewInterpreter(machine)
	interp1.Start()
	interp1.Send(statekit.Event{Type: "NEW_GAME"})
	snapshot := interp1.Snapshot()
	interp1.Stop()

	// Modify snapshot to simulate different machine
	snapshot.MachineID = "different_game"

	// Try to restore to new interpreter
	interp2 := statekit.NewInterpreter(machine)
	err := interp2.Restore(snapshot)

	if err == nil {
		t.Error("Expected error for machine ID mismatch")
	}
}

func TestGameSave_ContextPreservation(t *testing.T) {
	machine := buildGameMachine()
	interp := statekit.NewInterpreter(machine)
	interp.UpdateContext(func(ctx *GameContext) {
		ctx.PlayerName = "TestPlayer"
	})
	interp.Start()

	// Build up game state
	interp.Send(statekit.Event{Type: "NEW_GAME"})
	interp.Send(statekit.Event{Type: "START_QUEST"})
	interp.Send(statekit.Event{Type: "FIGHT"})
	interp.Send(statekit.Event{Type: "WIN"})
	interp.Send(statekit.Event{Type: "COMPLETE_QUEST"})
	interp.Send(statekit.Event{Type: "NEXT_LEVEL"})
	interp.Send(statekit.Event{Type: "NEXT_LEVEL"})

	// Add inventory item via context update
	interp.UpdateContext(func(ctx *GameContext) {
		ctx.Inventory = append(ctx.Inventory, "magic_ring")
	})

	snapshot := interp.Snapshot()

	// Verify context data in snapshot
	if len(snapshot.Context.Inventory) != 3 {
		t.Errorf("Expected 3 inventory items, got %d", len(snapshot.Context.Inventory))
	}

	if snapshot.Context.Level != 3 {
		t.Errorf("Expected level 3, got %d", snapshot.Context.Level)
	}

	if len(snapshot.Context.QuestLog) != 1 {
		t.Errorf("Expected 1 quest in log, got %d", len(snapshot.Context.QuestLog))
	}

	interp.Stop()
}

func TestGameSave_HistoryPreservation(t *testing.T) {
	machine := buildGameMachine()
	interp := statekit.NewInterpreter(machine)
	interp.Start()

	// Play through some states to build history
	interp.Send(statekit.Event{Type: "NEW_GAME"})
	interp.Send(statekit.Event{Type: "START_QUEST"})
	interp.Send(statekit.Event{Type: "FIGHT"})

	// Take snapshot while in combat
	snapshot := interp.Snapshot()

	// Verify we have history data
	if snapshot.ShallowHistory == nil {
		t.Error("Expected shallow history to be captured")
	}

	interp.Stop()
}
