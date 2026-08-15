# Static Analysis (Lint)

The `lint` package provides static analysis for statekit state machines, detecting potential issues like unreachable states, dead ends, non-deterministic transitions, and other structural problems.

## Installation

The lint package is included with statekit:

```bash
go get go.klarlabs.de/statekit
```

Import the package:

```go
import "go.klarlabs.de/statekit/lint"
```

## Quick Start

```go
import (
    "fmt"
    "go.klarlabs.de/statekit"
    "go.klarlabs.de/statekit/lint"
)

func main() {
    machine, _ := statekit.NewMachine[Context]("order").
        WithInitial("pending").
        // ... state definitions
        Build()

    // Run lint analysis
    result := lint.Lint(machine)

    // Check for issues
    if result.HasErrors() {
        fmt.Println("Errors found:")
        for _, d := range result.Errors() {
            fmt.Printf("  %s\n", d)
        }
    }

    if result.HasWarnings() {
        fmt.Println("Warnings found:")
        for _, d := range result.Warnings() {
            fmt.Printf("  %s\n", d)
        }
    }
}
```

## Available Rules

| Rule | Severity | Description |
|------|----------|-------------|
| `unreachable` | Warning | State cannot be reached from the initial state |
| `dead-end` | Warning | Non-final state has no outgoing transitions |
| `non-determinism` | Error | Multiple unguarded transitions for the same event |
| `compound-initial` | Error | Compound state has children but no initial state |
| `self-transition` | Info | Unguarded self-transition will re-run entry actions |
| `unused-action` | Info | Registered action is never referenced |
| `unused-guard` | Info | Registered guard is never referenced |
| `invoke-missing-onerror` | Warning | Invoke / InvokeMachine has no OnError handler |
| `invoke-id-collision` | Error | Duplicate invocation IDs within a state |
| `auto-forward-redundancy` | Warning | AutoForward event is also handled by the parent |
| `deep-nesting` | Info | State nests five or more levels deep |
| `history-without-siblings` | Warning | History state has no non-history siblings to remember |
| `guarded-only-entry` | Warning | State is only entered via guarded transitions |
| `auto-forward-loop` | Warning | State auto-forwards and raises the same event (ping-pong risk) |
| `actor-id-collision` | Warning | Same MachineInvocation ID reused across states |

### Rule Details

#### `unreachable`
Detects states that cannot be reached from the initial state through any transition path.

```go
machine, _ := statekit.NewMachine[struct{}]("example").
    WithInitial("idle").
    State("idle").On("START").Target("running").Done().
    State("running").Done().
    State("orphan").Done(). // Warning: unreachable
    Build()
```

#### `dead-end`
Detects non-final states with no outgoing transitions (and no parent transitions that could handle events via bubbling).

```go
machine, _ := statekit.NewMachine[struct{}]("example").
    WithInitial("idle").
    State("idle").On("GO").Target("stuck").Done().
    State("stuck").Done(). // Warning: dead-end (not final, no transitions)
    Build()
```

Note: This check considers event bubbling - if a parent state has transitions, child states are not flagged as dead ends.

#### `non-determinism`
Detects states where the same event triggers multiple unguarded transitions.

```go
machine, _ := statekit.NewMachine[struct{}]("example").
    WithInitial("idle").
    State("idle").
        On("GO").Target("a"). // No guard
        On("GO").Target("b"). // No guard - Error: non-determinism!
    Done().
    State("a").Final().Done().
    State("b").Final().Done().
    Build()
```

To fix, add guards to make transitions mutually exclusive:

```go
State("idle").
    On("GO").Target("a").Guard("checkA").
    On("GO").Target("b").Guard("checkB").
Done()
```

#### `compound-initial`
Detects compound states that have children but no defined initial child state.

```go
// Error: compound-initial
// Compound state "parent" has children but no initial state
```

#### `self-transition`
Warns when a state has an unguarded self-transition and also has entry actions, since the entry actions will re-run on each self-transition.

```go
machine, _ := statekit.NewMachine[struct{}]("example").
    WithInitial("counting").
    WithAction("increment", func(ctx *struct{}, e statekit.Event) {}).
    State("counting").
        OnEntry("increment").
        On("COUNT").Target("counting"). // Info: will re-run "increment"
    Done().
    Build()
```

#### `unused-action` and `unused-guard`
Detects registered actions or guards that are never referenced in any state or transition.

## Severity Levels

| Level | Description |
|-------|-------------|
| `Error` | Definite problem that will cause issues at runtime |
| `Warning` | Likely problem that should be reviewed |
| `Info` | Suggestion or observation |

## Ignoring Rules

Configure the linter to skip specific rules:

```go
linter := lint.NewLinter().
    Ignore(lint.RuleUnreachable).
    Ignore(lint.RuleDeadEnd)

result := lint.CheckTyped(linter, machine)
```

## Result API

```go
result := lint.Lint(machine)

// Check overall status
result.HasErrors()   // true if any Error-level diagnostics
result.HasWarnings() // true if any Warning or Error level diagnostics

// Get filtered diagnostics
result.Errors()      // []Diagnostic with SeverityError
result.Warnings()    // []Diagnostic with SeverityWarning

// All diagnostics
result.Diagnostics   // []Diagnostic (sorted by severity, then state)

// Human-readable summary
fmt.Println(result.String())
// Output:
// Machine "example": 1 error(s), 2 warning(s), 0 info(s)
//   [error] idle: event "GO" has 2 unguarded transitions (non-determinism)
//   [warning] orphan: state is unreachable from initial state (unreachable)
//   [warning] stuck: non-final state has no outgoing transitions (dead-end)
```

## Diagnostic Structure

```go
type Diagnostic struct {
    Severity Severity         // SeverityError, SeverityWarning, SeverityInfo
    Rule     string           // Rule identifier (e.g., "unreachable")
    State    statekit.StateID // State where issue was found (empty for machine-level)
    Event    statekit.EventType // Event involved (for transition issues)
    Message  string           // Human-readable description
}
```

## Integration with CI/CD

Use the linter in your test suite:

```go
func TestMachine_Lint(t *testing.T) {
    machine := buildMyMachine()
    result := lint.Lint(machine)

    if result.HasErrors() {
        t.Fatalf("machine has lint errors: %v", result.Errors())
    }

    // Optionally fail on warnings too
    if result.HasWarnings() {
        t.Logf("machine has lint warnings: %v", result.Warnings())
    }
}
```

## Best Practices

1. **Run lint in tests**: Add lint checks to your test suite to catch issues early.

2. **Fix non-determinism errors**: These indicate undefined behavior at runtime.

3. **Review dead ends**: Intentional dead ends should be marked as final states.

4. **Remove unused actions/guards**: Clean up registered but unused handlers.

5. **Use guards for multi-target events**: When the same event can go to different states, use guards to make the choice explicit.

## Rule Constants

Use these constants when configuring ignored rules:

```go
lint.RuleUnreachable     // "unreachable"
lint.RuleDeadEnd         // "dead-end"
lint.RuleNonDeterminism  // "non-determinism"
lint.RuleCompoundInitial // "compound-initial"
lint.RuleSelfTransition  // "self-transition"
lint.RuleUnusedAction    // "unused-action"
lint.RuleUnusedGuard     // "unused-guard"
```

Get all rule names:

```go
rules := lint.AllRules() // []string
```
