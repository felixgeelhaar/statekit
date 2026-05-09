# Choosing an API

Statekit gives you three ways to define a machine. Pick by where the truth lives.

## TL;DR

| Truth lives in... | Use | File |
|---|---|---|
| Go code | Builder API | `statekit.NewMachine[C]("id")...` |
| Go struct tags | Reflection DSL | `statekit.FromStruct[M, C](registry)` |
| External JSON (Stately, hand-written) | `statekit generate` codegen | CLI → generated Go file |

## The three options

### 1. Builder API

Method-chained construction. Most idiomatic, fully type-checked at compile time, IDE auto-complete drives discovery.

```go
machine, _ := statekit.NewMachine[Order]("checkout").
    WithInitial("cart").
    State("cart").On("CHECKOUT").Target("processing").Done().
    State("processing").On("PAID").Target("shipped").Done().
    State("shipped").Final().Done().
    Build()
```

**Pick this when:**
- You're starting fresh.
- The machine lives next to the code that uses it.
- You want compile-time errors on typos in state IDs and event types (use named constants for these).
- You're embedding a small or medium-sized machine in a service.

### 2. Reflection DSL

Define the machine as a Go struct with field tags. The shape reads as data; the registry binds named actions and guards at runtime.

```go
type OrderMachine struct {
    statekit.MachineDef `id:"order" initial:"pending"`
    Pending    statekit.StateNode `on:"SUBMIT->processing:hasItems"`
    Processing statekit.StateNode `on:"COMPLETE->shipped"`
    Shipped    statekit.FinalNode
}

registry := statekit.NewActionRegistry[Order]().
    WithGuard("hasItems", func(c Order, _ statekit.Event) bool { return len(c.Items) > 0 })

machine, _ := statekit.FromStruct[OrderMachine, Order](registry)
```

**Pick this when:**
- The machine shape is more interesting than the runtime logic.
- You want the state graph readable at a glance without reading method chains.
- You're integrating with a system that auto-generates struct definitions (proto, ent, etc.).
- You're OK with runtime parsing instead of compile-time checks for tag syntax.

**Trade-off:** struct-tag DSL has its own micro-syntax (`SUBMIT->processing:hasItems`). Typos surface at `FromStruct` call time, not at compile time.

### 3. Native JSON + `statekit generate`

When the machine is authored elsewhere — Stately Studio, a hand-written JSON file, or a workflow imported from XState.

```bash
statekit generate machine.json -o machine.go -p mypackage
```

The generated Go file gives you typed `BuildXxx()` plus stub functions for every action and guard. Fill in the stubs.

**Pick this when:**
- You're collaborating with non-Go authors using Stately Studio.
- You want the source of truth in JSON (e.g., versioned with the schema, edited by product/design).
- You're migrating from XState and want a one-shot conversion.

**Trade-off:** there's a build step. The generated file should be checked in and re-generated when the JSON changes.

## Mixing approaches

You can mix freely. A common pattern:

- Reflection DSL for the **shape** (so the state graph reads as data).
- Codegen for the **import** (when the source is Stately).
- Builder for **dynamic** machines (driven by config at runtime — e.g., per-tenant workflows).

The Native JSON exporter rebuilds JSON from any of these for visualization.

## What about anonymous closures?

statekit's builder uses *named* actions and guards (`.WithAction("name", fn)` + `.OnEntry("name")`) rather than passing closures inline. This unlocks:

- **`lint`** — `unused-action` and `unused-guard` rules.
- **JSON export** — actions and guards round-trip through the Native format.
- **Visualization** — diagrams reference action names, not function pointers.

If you find the named-action boilerplate intrusive, the reflection DSL keeps the machine shape declarative while the registry stays in one place.
