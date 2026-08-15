# Documentation

Organized by [Diataxis](https://diataxis.fr/) — the type of doc you need depends on whether you're learning, doing, looking up, or understanding.

## Tutorials — learning by doing

Step-by-step paths from zero to a working machine.

- [Getting Started](./tutorials/getting-started.md) — your first machine in 10 lines
- [Stripe webhook saga](./tutorials/stripe-webhook-saga.md) — idempotency, Invoke, OnError, outbox
- [Migration from XState](./tutorials/xstate-migration.md) — moving a JS statechart to Go
- [Migration from looplab/fsm](./tutorials/migration-from-looplab-fsm.md) — translating Go's most popular FSM library
- [Migration from qmuntal/stateless](./tutorials/migration-from-qmuntal-stateless.md) — porting from the .NET-style FSM library
- [Test-Driven Development](./tutorials/tdd.md) — building machines TDD-style

## How-to guides — task-oriented recipes

When you know what you want, here's how.

- [Choosing an API](./how-to/choosing-an-api.md) — builder vs reflection DSL vs codegen
- [Builder Terminators](./how-to/builder-terminators.md) — Done vs End vs EndState vs EndRegion
- [Hierarchical States](./how-to/hierarchical-states.md) — nesting, event bubbling, exit/entry order
- [Guards & Actions](./how-to/guards-actions.md) — conditional transitions and side effects
- [Reflection DSL](./how-to/reflection-dsl.md) — define machines with struct tags
- [Code Generation](./how-to/code-generation.md) — Go scaffold from Native JSON
- [Visualization](./how-to/visualization.md) — ASCII / Mermaid / HTML / TUI rendering
- [Testing](./how-to/testing.md) — assertions, recorders, helpers
- [HTTP Integration](./how-to/http-integration.md) — handlers, registry, middleware
- [Observability](./how-to/observability.md) — Prometheus metrics, health checks, structured logging
- [OpenTelemetry](./how-to/opentelemetry.md) — distributed tracing
- [Static Analysis (Lint)](./how-to/lint.md) — catching structural issues at build time
- [Transition budgets](./how-to/transition-budget.md) — halt after N retries with context + guards
- [Plugin System](./how-to/plugin-system.md) — observing and modifying interpreter behavior
- [Machine Composition](./how-to/machine-composition.md) — invoking child machines
- [Actor Persistence](./how-to/actor-persistence.md) — snapshots and restoration
- [Performance Tuning](./how-to/performance-tuning.md) — profiling, hot paths, benchmarks

## Reference — facts and APIs

When you know what to call but need to confirm shape and contracts.

- [API Reference](./reference/api-reference.md) — full type and function reference
- [API Stability Tiers](./reference/stability.md) — what's semver-protected vs experimental

## Explanation — understanding the model

The "why" behind statekit's design choices.

- [Patterns & Recipes](./explanation/patterns-recipes.md) — copy-paste snippets plus the why
- [API Stability Tiers](./reference/stability.md) — explains the v1.x roadmap rationale

## Project planning

- [Backlog](./project/backlog.md) — what's planned next, sourced from the v1.0 expert reviews

---

## Quick decision: which API should I use?

| Use case | API | Why |
|---|---|---|
| Building a machine in code | Builder API | Type-safe, IDE-friendly, compile-time checked |
| Domain models with declarative shape | Reflection DSL | Struct tags make the machine readable as data |
| Importing from XState / Stately | `statekit generate` | Codegen avoids drift between JSON source and Go |
| Driving from a database / config file | Native JSON parser + builder | Runtime construction from external definitions |

Read [Choosing an API](./how-to/choosing-an-api.md) for the full decision tree.
