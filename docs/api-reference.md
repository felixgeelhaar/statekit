# API Reference

Complete API documentation for Statekit.

## Package statekit

### Types

#### StateID

```go
type StateID = ir.StateID // string
```

Uniquely identifies a state within a machine.

#### EventType

```go
type EventType = ir.EventType // string
```

Named event identifier.

#### ActionType

```go
type ActionType = ir.ActionType // string
```

Identifies a named action.

#### GuardType

```go
type GuardType = ir.GuardType // string
```

Identifies a named guard.

#### Event

```go
type Event struct {
    Type    EventType
    Payload any
}
```

Runtime event with optional payload.

#### State

```go
type State[C any] struct {
    Value   StateID
    Context C
}

func (s State[C]) Matches(id StateID) bool
```

Current runtime state of an interpreter.

#### Action

```go
type Action[C any] func(ctx *C, e Event)
```

Side-effect function executed during transitions. Receives mutable context.

#### Guard

```go
type Guard[C any] func(ctx C, e Event) bool
```

Predicate determining if transition should occur. Receives immutable context.

#### MachineConfig

```go
type MachineConfig[C any] = ir.MachineConfig[C]
```

Complete machine definition. Built by `MachineBuilder.Build()` or `FromStruct()`.

---

### Builder API

#### NewMachine

```go
func NewMachine[C any](id string) *MachineBuilder[C]
```

Creates a new MachineBuilder with the given ID.

#### MachineBuilder

```go
type MachineBuilder[C any] struct { ... }

func (b *MachineBuilder[C]) WithInitial(initial StateID) *MachineBuilder[C]
func (b *MachineBuilder[C]) WithContext(ctx C) *MachineBuilder[C]
func (b *MachineBuilder[C]) WithAction(name ActionType, action Action[C]) *MachineBuilder[C]
func (b *MachineBuilder[C]) WithGuard(name GuardType, guard Guard[C]) *MachineBuilder[C]
func (b *MachineBuilder[C]) State(id StateID) *StateBuilder[C]
func (b *MachineBuilder[C]) Build() (*MachineConfig[C], error)
```

#### StateBuilder

```go
type StateBuilder[C any] struct { ... }

func (b *StateBuilder[C]) Final() *StateBuilder[C]
func (b *StateBuilder[C]) OnEntry(action ActionType) *StateBuilder[C]
func (b *StateBuilder[C]) OnExit(action ActionType) *StateBuilder[C]
func (b *StateBuilder[C]) WithInitial(initial StateID) *StateBuilder[C]
func (b *StateBuilder[C]) State(id StateID) *StateBuilder[C]
func (b *StateBuilder[C]) On(event EventType) *TransitionBuilder[C]
func (b *StateBuilder[C]) Done() *MachineBuilder[C]
func (b *StateBuilder[C]) End() *StateBuilder[C]
```

#### TransitionBuilder

```go
type TransitionBuilder[C any] struct { ... }

func (b *TransitionBuilder[C]) Target(target StateID) *TransitionBuilder[C]
func (b *TransitionBuilder[C]) Guard(guard GuardType) *TransitionBuilder[C]
func (b *TransitionBuilder[C]) Do(action ActionType) *TransitionBuilder[C]
func (b *TransitionBuilder[C]) On(event EventType) *TransitionBuilder[C]
func (b *TransitionBuilder[C]) Done() *MachineBuilder[C]
func (b *TransitionBuilder[C]) End() *StateBuilder[C]
```

---

### Interpreter

#### NewInterpreter

```go
func NewInterpreter[C any](machine *MachineConfig[C]) *Interpreter[C]
```

Creates a new interpreter for the machine.

#### Interpreter Methods

```go
type Interpreter[C any] struct { ... }

func (i *Interpreter[C]) Start()
func (i *Interpreter[C]) Send(e Event)
func (i *Interpreter[C]) State() State[C]
func (i *Interpreter[C]) Matches(id StateID) bool
func (i *Interpreter[C]) Done() bool
func (i *Interpreter[C]) UpdateContext(fn func(*C))
```

| Method | Description |
|--------|-------------|
| `Start()` | Enter initial state, execute entry actions |
| `Send(e)` | Process event, may trigger transition |
| `State()` | Get current state and context |
| `Matches(id)` | Check if in state or any ancestor |
| `Done()` | Check if in final state |
| `UpdateContext(fn)` | Modify context with function |

---

### Reflection DSL

#### Marker Types

```go
type MachineDef struct{}      // `id:"..." initial:"..."`
type StateNode struct{}       // `on:"..." entry:"..." exit:"..."`
type CompoundNode struct{}    // `initial:"..." on:"..."`
type FinalNode struct{}       // final state marker
```

#### ActionRegistry

```go
func NewActionRegistry[C any]() *ActionRegistry[C]

type ActionRegistry[C any] struct { ... }

func (r *ActionRegistry[C]) WithAction(name ActionType, action Action[C]) *ActionRegistry[C]
func (r *ActionRegistry[C]) WithGuard(name GuardType, guard Guard[C]) *ActionRegistry[C]
```

#### FromStruct

```go
func FromStruct[M any, C any](registry *ActionRegistry[C]) (*MachineConfig[C], error)
```

Build machine from struct definition.

#### FromStructWithContext

```go
func FromStructWithContext[M any, C any](registry *ActionRegistry[C], ctx C) (*MachineConfig[C], error)
```

Build machine with initial context value.

---

## Package export

### XStateExporter

```go
func NewXStateExporter[C any](machine *ir.MachineConfig[C]) *XStateExporter[C]

type XStateExporter[C any] struct { ... }

func (e *XStateExporter[C]) Export() (*XStateMachine, error)
func (e *XStateExporter[C]) ExportJSON() (string, error)
func (e *XStateExporter[C]) ExportJSONIndent(prefix, indent string) (string, error)
```

### XState Types

```go
type XStateMachine struct {
    ID      string                `json:"id"`
    Initial string                `json:"initial,omitempty"`
    States  map[string]XStateNode `json:"states"`
}

type XStateNode struct {
    Type    string                      `json:"type,omitempty"`
    Initial string                      `json:"initial,omitempty"`
    States  map[string]XStateNode       `json:"states,omitempty"`
    Entry   []string                    `json:"entry,omitempty"`
    Exit    []string                    `json:"exit,omitempty"`
    On      map[string]XStateTransition `json:"on,omitempty"`
}

type XStateTransition struct {
    Target  string   `json:"target,omitempty"`
    Actions []string `json:"actions,omitempty"`
    Guard   string   `json:"guard,omitempty"`
}
```

### CLI Helper

```go
type MachineExporter interface {
    Export() (*XStateMachine, error)
}

type ExportOptions struct {
    PrettyPrint bool
    Indent      string
    Output      io.Writer
    MachineID   string
}

func DefaultExportOptions() ExportOptions
func ExportMachine(exporter MachineExporter, opts ExportOptions) error
func ExportAll(machines map[string]MachineExporter, opts ExportOptions) error
func RunCLI(machines map[string]MachineExporter, args []string) error
```

---

## Tag Reference

### Machine Tags

| Tag | Required | Description |
|-----|----------|-------------|
| `id:"..."` | Yes | Machine identifier |
| `initial:"..."` | Yes | Initial state name |

### State Tags

| Tag | Description |
|-----|-------------|
| `on:"EVENT->target"` | Transition definition |
| `on:"EVENT->target:guard"` | With guard |
| `on:"EVENT->target/action"` | With action |
| `on:"EVENT->target/a1;a2:guard"` | Multiple actions + guard |
| `on:"E1->t1,E2->t2"` | Multiple transitions |
| `entry:"action1,action2"` | Entry actions |
| `exit:"action1,action2"` | Exit actions |
| `initial:"child"` | Initial child (CompoundNode) |

---

## Constants

```go
const (
    StateTypeAtomic   StateType = iota
    StateTypeCompound
    StateTypeFinal
)
```

---

## Error Handling

### Validation Errors

Returned by `Build()` and `FromStruct()`:

- `INITIAL_STATE_NOT_FOUND` - Initial state doesn't exist
- `TRANSITION_TARGET_NOT_FOUND` - Target state doesn't exist
- `ACTION_NOT_REGISTERED` - Action name not in registry
- `GUARD_NOT_REGISTERED` - Guard name not in registry
- `COMPOUND_MISSING_INITIAL` - Compound state needs initial child
- `CIRCULAR_HIERARCHY` - State is its own ancestor

### Parsing Errors (Reflection)

- Missing `id` or `initial` tag on MachineDef
- Invalid transition syntax in `on` tag
- Struct doesn't embed MachineDef

---

## Usage Patterns

### Fluent Builder

```go
machine, err := statekit.NewMachine[Context]("id").
    WithInitial("start").
    WithAction("act", func(ctx *Context, e Event) { ... }).
    WithGuard("check", func(ctx Context, e Event) bool { ... }).
    State("start").
        OnEntry("act").
        On("GO").Target("end").Guard("check").
    Done().
    State("end").Final().Done().
    Build()
```

### Reflection DSL

```go
type Machine struct {
    statekit.MachineDef `id:"id" initial:"start"`
    Start statekit.StateNode `on:"GO->end:check" entry:"act"`
    End   statekit.FinalNode
}

registry := statekit.NewActionRegistry[Context]().
    WithAction("act", func(ctx *Context, e Event) { ... }).
    WithGuard("check", func(ctx Context, e Event) bool { ... })

machine, err := statekit.FromStruct[Machine, Context](registry)
```

### Export

```go
exporter := export.NewXStateExporter(machine)
json, _ := exporter.ExportJSONIndent("", "  ")
```
