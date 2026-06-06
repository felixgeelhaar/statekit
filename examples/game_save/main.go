// Package main demonstrates persistence with snapshots for a game save system.
//
// This example shows:
// - Capturing interpreter state with Snapshot()
// - Restoring interpreter state with Restore()
// - JSON serialization of snapshots
// - History state preservation
// - Context data persistence
package main

import (
	"encoding/json"
	"fmt"
	"time"

	"go.klarlabs.de/statekit"
)

// GameContext holds the game state
type GameContext struct {
	PlayerName   string    `json:"player_name"`
	Level        int       `json:"level"`
	Health       int       `json:"health"`
	Score        int       `json:"score"`
	Inventory    []string  `json:"inventory"`
	QuestLog     []string  `json:"quest_log"`
	PlayTime     int       `json:"play_time_seconds"`
	LastSaveTime time.Time `json:"last_save_time"`
}

func main() {
	machine := buildGameMachine()

	fmt.Println("=== Game Save/Load Demo ===")
	fmt.Println()

	// Start a new game
	fmt.Println("--- Starting New Game ---")
	interp := statekit.NewInterpreter(machine)
	initializePlayer(interp)
	interp.Start()

	printGameState(interp, "Initial")

	// Play through some of the game
	fmt.Println("\n--- Playing Game ---")
	interp.Send(statekit.Event{Type: "START_QUEST"})
	printGameState(interp, "After START_QUEST")

	interp.Send(statekit.Event{Type: "FIGHT"})
	printGameState(interp, "After FIGHT")

	interp.Send(statekit.Event{Type: "WIN"})
	printGameState(interp, "After WIN")

	interp.Send(statekit.Event{Type: "NEXT_LEVEL"})
	printGameState(interp, "After NEXT_LEVEL")

	// Save the game
	fmt.Println("\n--- Saving Game ---")
	interp.UpdateContext(func(ctx *GameContext) {
		ctx.LastSaveTime = time.Now()
		ctx.PlayTime += 300 // Simulate 5 minutes of play
	})

	snapshot := interp.Snapshot()
	saveData, _ := json.MarshalIndent(snapshot, "", "  ")
	fmt.Printf("Saved game state:\n%s\n", saveData)

	// Simulate closing the game
	interp.Stop()
	fmt.Println("\n--- Game Closed ---")

	// Later: Load the game
	fmt.Println("\n--- Loading Saved Game ---")
	newInterp := statekit.NewInterpreter(machine)

	// Restore from snapshot
	var loadedSnapshot statekit.Snapshot[GameContext]
	if err := json.Unmarshal(saveData, &loadedSnapshot); err != nil {
		panic(fmt.Sprintf("Failed to unmarshal: %v", err))
	}

	if err := newInterp.Restore(loadedSnapshot); err != nil {
		panic(fmt.Sprintf("Failed to restore: %v", err))
	}

	printGameState(newInterp, "After Restore")

	// Continue playing from saved state
	fmt.Println("\n--- Continuing Game ---")
	newInterp.Send(statekit.Event{Type: "START_QUEST"})
	printGameState(newInterp, "After continuing quest")

	newInterp.Send(statekit.Event{Type: "FIGHT"})
	newInterp.Send(statekit.Event{Type: "WIN"})
	newInterp.Send(statekit.Event{Type: "COMPLETE_QUEST"})
	printGameState(newInterp, "After completing quest")

	newInterp.Stop()
	fmt.Println("\n=== Demo Complete ===")
}

func buildGameMachine() *statekit.MachineConfig[GameContext] {
	machine, err := statekit.NewMachine[GameContext]("rpg_game").
		WithInitial("main_menu").
		// Actions
		WithAction("initGame", func(ctx *GameContext, e statekit.Event) {
			ctx.Level = 1
			ctx.Health = 100
			ctx.Score = 0
			ctx.Inventory = []string{"sword", "shield"}
			ctx.QuestLog = []string{}
			fmt.Printf("[Game] New game started for %s\n", ctx.PlayerName)
		}).
		WithAction("startQuest", func(ctx *GameContext, e statekit.Event) {
			quest := fmt.Sprintf("Quest %d: Defeat the boss", ctx.Level)
			ctx.QuestLog = append(ctx.QuestLog, quest)
			fmt.Printf("[Game] Started: %s\n", quest)
		}).
		WithAction("enterCombat", func(ctx *GameContext, e statekit.Event) {
			fmt.Println("[Game] Entering combat...")
		}).
		WithAction("resolveCombat", func(ctx *GameContext, e statekit.Event) {
			ctx.Score += 100 * ctx.Level
			fmt.Printf("[Game] Victory! Score: %d\n", ctx.Score)
		}).
		WithAction("levelUp", func(ctx *GameContext, e statekit.Event) {
			ctx.Level++
			ctx.Health = 100 // Restore health
			fmt.Printf("[Game] Level up! Now level %d\n", ctx.Level)
		}).
		WithAction("completeQuest", func(ctx *GameContext, e statekit.Event) {
			ctx.Score += 500 * ctx.Level
			fmt.Printf("[Game] Quest complete! Score: %d\n", ctx.Score)
		}).
		WithAction("takeDamage", func(ctx *GameContext, e statekit.Event) {
			ctx.Health -= 25
			fmt.Printf("[Game] Took damage! Health: %d\n", ctx.Health)
		}).
		// Guards
		WithGuard("isAlive", func(ctx GameContext, e statekit.Event) bool {
			return ctx.Health > 0
		}).
		// Main Menu
		State("main_menu").
		On("NEW_GAME").Target("playing").Do("initGame").
		On("LOAD_GAME").Target("playing"). // Restore handles the state
		Done().
		// Playing state (compound with history for save/load)
		State("playing").
		WithInitial("exploring").
		On("QUIT").Target("main_menu").End().
		On("GAME_OVER").Target("game_over").End().
		History("resume").Shallow().Default("exploring").End().
		// Exploring sub-state
		State("exploring").
		On("START_QUEST").Target("on_quest").Do("startQuest").
		On("NEXT_LEVEL").Target("exploring").Do("levelUp").
		End().
		End().
		// On Quest sub-state (compound)
		State("on_quest").
		WithInitial("traveling").
		On("ABANDON").Target("exploring").End().
		On("COMPLETE_QUEST").Target("exploring").Do("completeQuest").End().
		State("traveling").
		On("FIGHT").Target("combat").Do("enterCombat").
		End().
		End().
		State("combat").
		On("WIN").Target("traveling").Do("resolveCombat").
		On("LOSE").Target("traveling").Do("takeDamage").Guard("isAlive").
		On("LOSE").Target("game_over").
		End().
		End().
		End().  // End on_quest
		Done(). // End playing
		// Game Over
		State("game_over").
		Final().
		Done().
		Build()

	if err != nil {
		panic(fmt.Sprintf("Failed to build machine: %v", err))
	}

	return machine
}

func initializePlayer(interp *statekit.Interpreter[GameContext]) {
	interp.UpdateContext(func(ctx *GameContext) {
		ctx.PlayerName = "Hero"
	})
}

func printGameState(interp *statekit.Interpreter[GameContext], label string) {
	state := interp.State()
	ctx := state.Context

	fmt.Printf("\n[%s]\n", label)
	fmt.Printf("  State: %s\n", state.Value)
	fmt.Printf("  Player: %s (Level %d)\n", ctx.PlayerName, ctx.Level)
	fmt.Printf("  Health: %d | Score: %d\n", ctx.Health, ctx.Score)
	fmt.Printf("  Inventory: %v\n", ctx.Inventory)
	if len(ctx.QuestLog) > 0 {
		fmt.Printf("  Quest Log: %v\n", ctx.QuestLog)
	}
}
