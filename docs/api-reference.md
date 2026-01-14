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

### NativeExporter

Exports machine definitions to Statekit Native JSON.

```go
func NewNativeExporter[C any](machine *ir.MachineConfig[C]) *NativeExporter[C]
```

```go
type NativeExporter[C any] struct { ... }
```

```go
func (e *NativeExporter[C]) Export() *viz.VizMachine
func (e *NativeExporter[C]) ExportJSON() (string, error)
func (e *NativeExporter[C]) ExportJSONIndent(prefix, indent string) (string, error)
```

### JSON Types

```go
type VizMachine struct {
    ID      string               `json:"id"`
    Initial string               `json:"initial"`
    States  map[string]*VizState `json:"states"`
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

---

## Package generate

Go code generation from Statekit JSON.

### Generator

```go
func NewGenerator(packageName, typeName, contextType string) *Generator

type Generator struct {
    PackageName string
    TypeName    string
    ContextType string
}

func (g *Generator) Generate(r io.Reader) ([]byte, error)
func (g *Generator) GenerateMachine(machine *viz.VizMachine) ([]byte, error)
```

### XState JSON Types

```go
type XStateMachine struct {
    ID      string            `json:"id"`
    Initial string            `json:"initial"`
    Context json.RawMessage   `json:"context,omitempty"`
    States  map[string]XState `json:"states"`
}

type XState struct {
    Initial string                `json:"initial,omitempty"`
    Type    string                `json:"type,omitempty"`
    States  map[string]XState     `json:"states,omitempty"`
    On      map[string]XTransSpec `json:"on,omitempty"`
    Entry   XActionSpec           `json:"entry,omitempty"`
    Exit    XActionSpec           `json:"exit,omitempty"`
}

type XTransSpec struct { ... }   // string, object, or array
type XActionSpec struct { ... }  // string or array
```

### CLI Command

```bash
statekit generate [file.json] [flags]

Flags:
  -o, --output string    Output file (default: stdout)
  -p, --package string   Go package name (default: "main")
  -t, --type string      Type name for the machine
  -c, --context string   Context type name (default: "struct{}")
```

---

## Package http

HTTP handlers and middleware for web frameworks.

### MachineHandler

```go
func NewMachineHandler[C any](interp *statekit.Interpreter[C]) *MachineHandler[C]

type MachineHandler[C any] struct { ... }

func (h *MachineHandler[C]) HandleGetState(w http.ResponseWriter, r *http.Request)
func (h *MachineHandler[C]) HandleSendEvent(w http.ResponseWriter, r *http.Request)
func (h *MachineHandler[C]) HandleGetContext(w http.ResponseWriter, r *http.Request)
func (h *MachineHandler[C]) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

### Request/Response Types

```go
type StateResponse struct {
    CurrentState string `json:"currentState"`
    Done         bool   `json:"done"`
    MachineID    string `json:"machineId,omitempty"`
}

type EventRequest struct {
    Type    string         `json:"type"`
    Payload map[string]any `json:"payload,omitempty"`
}

type EventResponse struct {
    PreviousState string `json:"previousState"`
    CurrentState  string `json:"currentState"`
    Transitioned  bool   `json:"transitioned"`
    Done          bool   `json:"done"`
}
```

### MachineRegistry

```go
func NewMachineRegistry[C any](
    factory func(id string) (*statekit.Interpreter[C], error),
) *MachineRegistry[C]

type MachineRegistry[C any] struct { ... }

func (r *MachineRegistry[C]) Get(id string) (*statekit.Interpreter[C], error)
func (r *MachineRegistry[C]) Remove(id string)
func (r *MachineRegistry[C]) List() []string
```

### Middleware

```go
type Middleware func(http.Handler) http.Handler

func MachineMiddleware[C any](interp *statekit.Interpreter[C]) Middleware
func RegistryMiddleware[C any](
    registry *MachineRegistry[C],
    idExtractor func(*http.Request) string,
) Middleware
```

### Context Helpers

```go
const MachineContextKey contextKey = "statekit.machine"

func WithMachine[C any](ctx context.Context, interp *statekit.Interpreter[C]) context.Context
func MachineFromContext[C any](ctx context.Context) (*statekit.Interpreter[C], bool)
```

### ServeMux Helper

```go
func NewServeMux[C any](interp *statekit.Interpreter[C], prefix string) *http.ServeMux
```

---

## Package otel

OpenTelemetry tracing integration.

### TracingInterpreter

```go
func NewTracingInterpreter[C any](
    interp *statekit.Interpreter[C],
    machineID string,
    opts ...TracingOption[C],
) *TracingInterpreter[C]

type TracingInterpreter[C any] struct { ... }

func (ti *TracingInterpreter[C]) Start(ctx context.Context) context.Context
func (ti *TracingInterpreter[C]) Send(ctx context.Context, event statekit.Event)
func (ti *TracingInterpreter[C]) SendAll(ctx context.Context, events []statekit.Event)
func (ti *TracingInterpreter[C]) State() statekit.State[C]
func (ti *TracingInterpreter[C]) Context() C
func (ti *TracingInterpreter[C]) Done() bool
func (ti *TracingInterpreter[C]) Matches(stateID statekit.StateID) bool
func (ti *TracingInterpreter[C]) Stop()
func (ti *TracingInterpreter[C]) Interpreter() *statekit.Interpreter[C]
```

### Options

```go
type TracingOption[C any] func(*TracingInterpreter[C])

func WithTracer[C any](tracer trace.Tracer) TracingOption[C]
```

### TracingHook

```go
type TransitionHook func(
    ctx context.Context,
    machineID string,
    event statekit.Event,
    stateBefore, stateAfter string,
)

func TracingHook(tracer trace.Tracer) TransitionHook
```

### Attribute Helpers

```go
func StateAttributes(machineID string, state statekit.StateID) []attribute.KeyValue
func EventAttributes(machineID string, event statekit.Event) []attribute.KeyValue
func TransitionAttributes(
    machineID string,
    event statekit.Event,
    from, to statekit.StateID,
) []attribute.KeyValue
```

### Constants

```go
const TracerName = "github.com/felixgeelhaar/statekit/otel"
```

### Span Attributes

| Attribute | Description |
|-----------|-------------|
| `statekit.machine.id` | Machine identifier |
| `statekit.event.type` | Event type |
| `statekit.state.id` | Current state ID |
| `statekit.state.before` | State before transition |
| `statekit.state.after` | State after transition |
| `statekit.state.from` | Source state (in events) |
| `statekit.state.to` | Target state (in events) |
| `statekit.transitioned` | Whether state changed |
| `statekit.completed` | Machine reached final state |
