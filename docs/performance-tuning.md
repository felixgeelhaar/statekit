# Performance Tuning Guide

This guide covers performance characteristics, optimization techniques, and best practices for building high-performance state machines with Statekit.

## Performance Baseline

Benchmarks run on Apple M1 (arm64):

| Operation | Time | Allocations | Notes |
|-----------|------|-------------|-------|
| Simple transition | ~364ns | 7 allocs | Basic event → state change |
| Hierarchical bubble | ~669ns | 10 allocs | Event bubbles to parent |
| No matching transition | ~36ns | 0 allocs | Fast rejection path |
| State query (`State()`) | ~14ns | 0 allocs | Zero-cost access |
| `Matches()` direct | ~14ns | 0 allocs | Current state check |
| `Matches()` ancestor | ~21ns | 0 allocs | Parent state check |
| `UpdateContext()` | ~15ns | 0 allocs | Context mutation |
| `Snapshot()` | ~196ns | 3 allocs | State serialization |
| Parallel state entry | ~1μs | 16 allocs | Multiple regions |
| Plugin overhead | ~16ns | 0 allocs | Per-plugin per-hook |

### Machine Construction

| Method | Time | Allocations |
|--------|------|-------------|
| Builder (3 states) | ~1.3μs | 40 allocs |
| Builder (5 states, nested) | ~1.5μs | 46 allocs |
| Reflection DSL (2 states) | ~2.4μs | 42 allocs |
| Reflection DSL (nested) | ~2.8μs | 49 allocs |

**Key insight**: Machine construction is a one-time cost. Build once, reuse the `MachineConfig` across all interpreters.

---

## Optimization Strategies

### 1. Reuse Machine Configurations

**Don't** create a new machine for each request:

```go
// Bad: Rebuilds machine every time
func HandleRequest(req Request) {
    machine, _ := statekit.NewMachine[Ctx]("order").
        WithInitial("pending").
        // ... build machine ...
        Build()

    interp := statekit.NewInterpreter(machine)
    // ...
}
```

**Do** build once and reuse:

```go
// Good: Build once at startup
var orderMachine *statekit.MachineConfig[OrderContext]

func init() {
    var err error
    orderMachine, err = statekit.NewMachine[OrderContext]("order").
        WithInitial("pending").
        // ... build machine ...
        Build()
    if err != nil {
        panic(err)
    }
}

func HandleRequest(req Request) {
    interp := statekit.NewInterpreter(orderMachine)
    // ...
}
```

**Impact**: Saves ~1-3μs per request plus allocations.

### 2. Minimize Context Size

Smaller contexts are faster to copy (for guards) and snapshot:

```go
// Expensive: Large embedded structs
type HeavyContext struct {
    Order       Order       // Large struct copied to guards
    Customer    Customer
    Items       []Item
    AuditLog    []LogEntry
}

// Better: Use pointers for large data
type LightContext struct {
    OrderID    string      // Small value types
    Status     OrderStatus
    Order      *Order      // Pointer, not copied
    Customer   *Customer
}

// Best: Separate concerns
type MinimalContext struct {
    OrderID    string
    Status     OrderStatus
    // Store large data elsewhere, fetch when needed
}
```

**Impact**: Guards receive context by value. Large contexts = slower guard evaluation.

### 3. Optimize Guard Functions

Guards run on every matching event. Keep them fast:

```go
// Slow: Database lookup in guard
WithGuard("canApprove", func(ctx OrderContext, e statekit.Event) bool {
    user, _ := db.GetUser(ctx.ApproverID)  // Network call!
    return user.HasPermission("approve")
})

// Fast: Pre-computed in context
WithGuard("canApprove", func(ctx OrderContext, e statekit.Event) bool {
    return ctx.ApproverHasPermission  // Boolean already computed
})

// Alternative: Move expensive checks to actions
WithGuard("hasApprover", func(ctx OrderContext, e statekit.Event) bool {
    return ctx.ApproverID != ""  // Fast check
})
WithAction("validateApprover", func(ctx *OrderContext, e statekit.Event) {
    // Do expensive validation here, set ctx.Approved = true/false
})
```

### 4. Use Appropriate State Depth

Deeper hierarchies = more ancestor traversal:

```go
// Deep nesting (slower event bubbling)
machine := State("l1").
    State("l2").
        State("l3").
            State("l4").
                State("l5").  // 5 levels deep
                    On("EVENT").Target("target").
                End().End().End().End().
    Done()

// Flatter structure (faster)
machine := State("workflow").
    WithInitial("step1").
    State("step1").On("NEXT").Target("step2").End().End().
    State("step2").On("NEXT").Target("step3").End().End().
    // 2 levels only
Done()
```

**Guideline**: Each hierarchy level adds ~100-200ns to event processing when bubbling.

### 5. Prefer Builder over Reflection DSL for Hot Paths

The builder API is slightly faster for machine construction:

| Approach | Build Time |
|----------|------------|
| Builder API | ~1.7μs |
| Reflection DSL | ~2.4μs |

For applications creating machines dynamically, prefer the builder. For most applications where machines are built once at startup, either approach is fine.

### 6. Batch State Checks

```go
// Inefficient: Multiple Matches() calls
if interp.Matches("pending") || interp.Matches("processing") || interp.Matches("reviewing") {
    // ...
}

// Better: Check state value directly
state := interp.State().Value
switch state {
case "pending", "processing", "reviewing":
    // ...
}

// Or use a parent state for grouping
if interp.Matches("in_progress") {  // Parent of pending/processing/reviewing
    // ...
}
```

### 7. Avoid Unnecessary Snapshots

`Snapshot()` allocates memory. Don't call it in tight loops:

```go
// Bad: Snapshot on every event
for _, event := range events {
    interp.Send(event)
    snapshot := interp.Snapshot()  // Unnecessary allocation
    log.Printf("State: %s", snapshot.CurrentState)
}

// Good: Use State() for reads
for _, event := range events {
    interp.Send(event)
    state := interp.State()  // Zero allocation
    log.Printf("State: %s", state.Value)
}

// Snapshot only when persisting
interp.Send(event)
if shouldPersist() {
    snapshot := interp.Snapshot()
    saveToDatabase(snapshot)
}
```

---

## Memory Optimization

### Allocation Sources

| Source | Allocations | Mitigation |
|--------|-------------|------------|
| Event creation | 1-2 per event | Pool events if needed |
| Transition resolution | 5-7 per transition | Normal, unavoidable |
| History tracking | 1-2 per compound exit | Use shallow history when possible |
| Parallel regions | 2-4 per region | Minimize region count |
| Snapshots | 3-4 per snapshot | Snapshot infrequently |

### Reducing Allocations

**1. Pre-allocate event payloads:**

```go
// Allocates new map each time
interp.Send(statekit.Event{
    Type: "UPDATE",
    Payload: map[string]any{"key": "value"},  // New allocation
})

// Pre-allocate and reuse
type UpdatePayload struct {
    Key   string
    Value string
}
payload := UpdatePayload{Key: "key", Value: "value"}
interp.Send(statekit.Event{Type: "UPDATE", Payload: payload})
```

**2. Use sync.Pool for interpreters in high-throughput scenarios:**

```go
var interpPool = sync.Pool{
    New: func() any {
        return statekit.NewInterpreter(sharedMachine)
    },
}

func ProcessRequest(req Request) {
    interp := interpPool.Get().(*statekit.Interpreter[Context])
    defer interpPool.Put(interp)

    interp.Reset()  // Reset to initial state
    interp.Start()
    // Process...
    interp.Stop()
}
```

---

## Concurrency Considerations

### Interpreter Thread Safety

**Interpreters are NOT thread-safe.** Each goroutine needs its own interpreter:

```go
// Wrong: Shared interpreter
var sharedInterp *statekit.Interpreter[Ctx]

func HandleConcurrently(event statekit.Event) {
    sharedInterp.Send(event)  // Race condition!
}

// Correct: Interpreter per goroutine
func HandleConcurrently(event statekit.Event) {
    interp := statekit.NewInterpreter(sharedMachine)
    interp.Start()
    interp.Send(event)
}

// Correct: Mutex protection (if sharing is required)
var (
    sharedInterp *statekit.Interpreter[Ctx]
    mu           sync.Mutex
)

func HandleConcurrently(event statekit.Event) {
    mu.Lock()
    defer mu.Unlock()
    sharedInterp.Send(event)
}
```

### Machine Config Thread Safety

`MachineConfig` is immutable and safe to share across goroutines:

```go
// Safe: Shared machine config
var machine *statekit.MachineConfig[Ctx]

func Worker(id int) {
    interp := statekit.NewInterpreter(machine)  // Safe
    interp.Start()
    // ...
}
```

---

## Profiling Your State Machines

### Using Go's Built-in Profiler

```go
import (
    "os"
    "runtime/pprof"
)

func main() {
    // CPU profiling
    f, _ := os.Create("cpu.prof")
    pprof.StartCPUProfile(f)
    defer pprof.StopCPUProfile()

    // Run your state machine workload
    runWorkload()

    // Memory profiling
    mf, _ := os.Create("mem.prof")
    defer mf.Close()
    pprof.WriteHeapProfile(mf)
}
```

```bash
# Analyze profiles
go tool pprof cpu.prof
go tool pprof mem.prof
```

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem

# Run specific benchmark
go test -bench=BenchmarkInterpreterSend -benchmem

# Compare before/after optimization
go test -bench=. -benchmem > before.txt
# Make changes
go test -bench=. -benchmem > after.txt
benchcmp before.txt after.txt
```

### Adding Custom Benchmarks

```go
func BenchmarkMyWorkflow(b *testing.B) {
    machine, _ := buildMyMachine()
    events := []statekit.Event{
        {Type: "START"},
        {Type: "PROCESS"},
        {Type: "COMPLETE"},
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        interp := statekit.NewInterpreter(machine)
        interp.Start()
        for _, e := range events {
            interp.Send(e)
        }
    }
}
```

---

## Performance Checklist

Before deploying to production:

- [ ] Machine configs are built once and reused
- [ ] Context contains only necessary data (pointers for large objects)
- [ ] Guards are fast (no I/O, minimal computation)
- [ ] State hierarchy depth is reasonable (< 5 levels typical)
- [ ] Snapshots are taken only when persisting
- [ ] Each goroutine has its own interpreter (or mutex protection)
- [ ] Benchmark results are acceptable for your use case
- [ ] Memory profile shows no unexpected allocations

---

## Common Performance Anti-Patterns

### 1. Building Machines per Request

```go
// Anti-pattern
func HandleOrder(order Order) {
    machine, _ := buildOrderMachine()  // Don't do this
    interp := statekit.NewInterpreter(machine)
    // ...
}
```

### 2. Heavy Context Objects

```go
// Anti-pattern: Entire domain model in context
type Context struct {
    Order     Order
    Customer  Customer
    Products  []Product
    Inventory []InventoryItem
    // ... everything
}
```

### 3. I/O in Guards

```go
// Anti-pattern: Network call in guard
WithGuard("canProcess", func(ctx Ctx, e statekit.Event) bool {
    resp, _ := http.Get("http://api/check")  // Never do this
    return resp.StatusCode == 200
})
```

### 4. Excessive Hierarchy Depth

```go
// Anti-pattern: 10+ levels of nesting
State("l1").State("l2").State("l3").State("l4").State("l5").
    State("l6").State("l7").State("l8").State("l9").State("l10").
    // Event bubbles through ALL 10 levels
```

### 5. Snapshot Spam

```go
// Anti-pattern: Snapshot after every operation
for _, item := range items {
    interp.Send(statekit.Event{Type: "PROCESS", Payload: item})
    db.Save(interp.Snapshot())  // Unnecessary I/O + allocations
}
```

---

## See Also

- [Getting Started](getting-started.md)
- [Patterns & Recipes](patterns-recipes.md)
- [API Reference](api-reference.md)
