// Package main demonstrates the actor model features of statekit.
// This example shows a supervisor pattern where a parent machine spawns
// child worker machines that process tasks independently.
package main

import (
	"fmt"
	"time"

	"go.klarlabs.de/statekit"
)

// WorkerContext tracks the worker's current task
type WorkerContext struct {
	WorkerID string
	TaskID   int
	Result   string
}

// SupervisorContext tracks the supervisor's state
type SupervisorContext struct {
	TotalTasks     int
	CompletedTasks int
	ActiveWorkers  int
}

func main() {
	fmt.Println("=== Actor Supervisor Example ===")
	fmt.Println()

	// Create the worker machine
	workerMachine := buildWorkerMachine()

	// Create and start the supervisor
	supervisor := buildSupervisor()
	interp := statekit.NewInterpreter(supervisor)
	defer func() { _ = interp.Close() }()
	interp.Start()

	fmt.Println("Supervisor started in 'idle' state")
	fmt.Printf("Current state: %s\n\n", interp.State().Value)

	// Transition to supervising state
	interp.Send(statekit.Event{Type: "BEGIN"})
	fmt.Println("Supervisor now in 'supervising' state")

	// Spawn some worker actors
	fmt.Println("\nSpawning workers...")
	for i := 1; i <= 3; i++ {
		workerID := statekit.ActorID(fmt.Sprintf("worker-%d", i))
		ref, err := statekit.Spawn(interp, workerID, workerMachine,
			statekit.WithAutoForward("TASK"),
			statekit.WithSupervision(statekit.SupervisionRecover),
		)
		if err != nil {
			fmt.Printf("Failed to spawn %s: %v\n", workerID, err)
			continue
		}
		fmt.Printf("  Spawned %s\n", ref.ID())
	}

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// Send tasks to workers
	fmt.Println("\nSending tasks to workers...")
	for i := 1; i <= 3; i++ {
		workerID := statekit.ActorID(fmt.Sprintf("worker-%d", i))
		err := interp.SendTo(workerID, statekit.Event{
			Type:    "TASK",
			Payload: fmt.Sprintf("Process item %d", i),
		})
		if err != nil {
			fmt.Printf("Failed to send to %s: %v\n", workerID, err)
		} else {
			fmt.Printf("  Sent task to %s\n", workerID)
		}
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Complete the workers
	fmt.Println("\nCompleting workers...")
	for i := 1; i <= 3; i++ {
		workerID := statekit.ActorID(fmt.Sprintf("worker-%d", i))
		err := interp.SendTo(workerID, statekit.Event{Type: "COMPLETE"})
		if err != nil {
			fmt.Printf("Failed to complete %s: %v\n", workerID, err)
		}
	}

	// Wait for completion
	time.Sleep(100 * time.Millisecond)

	// Stop supervision
	fmt.Println("\nStopping supervisor...")
	interp.Send(statekit.Event{Type: "STOP"})
	fmt.Printf("Final state: %s\n", interp.State().Value)

	// Clean shutdown
	fmt.Println("\n=== Example Complete ===")
}

func buildWorkerMachine() *statekit.MachineConfig[WorkerContext] {
	machine, _ := statekit.NewMachine[WorkerContext]("worker").
		WithInitial("idle").
		WithAction("logReceived", func(ctx *WorkerContext, e statekit.Event) {
			if task, ok := e.Payload.(string); ok {
				ctx.Result = task
				fmt.Printf("    [Worker] Received: %s\n", task)
			}
		}).
		WithAction("logComplete", func(ctx *WorkerContext, e statekit.Event) {
			fmt.Printf("    [Worker] Completed task\n")
		}).
		State("idle").
		On("TASK").Target("working").Do("logReceived").
		Done().
		State("working").
		On("COMPLETE").Target("done").Do("logComplete").
		Done().
		State("done").Final().
		Done().
		Build()
	return machine
}

func buildSupervisor() *statekit.MachineConfig[SupervisorContext] {
	machine, _ := statekit.NewMachine[SupervisorContext]("supervisor").
		WithInitial("idle").
		WithContext(SupervisorContext{}).
		State("idle").
		On("BEGIN").Target("supervising").
		Done().
		State("supervising").
		On("STOP").Target("stopped").
		On("statekit.done.actor.worker-1").Target("supervising").
		On("statekit.done.actor.worker-2").Target("supervising").
		On("statekit.done.actor.worker-3").Target("supervising").
		Done().
		State("stopped").Final().
		Done().
		Build()
	return machine
}
