# API Stability Tiers

statekit is **v1.0** with a stability commitment, but not every package has the same maturity. This page makes the tiers explicit so you can pick the right tools and know what to pin.

## Tier 1 — Stable (semver-protected)

These are the load-bearing parts of the library. We follow [semver](https://semver.org/): no breaking changes within v1.x.

- **Builder API** — `statekit.NewMachine`, `MachineBuilder`, `StateBuilder`, `TransitionBuilder`, `RegionBuilder`
- **Core types** — `Event`, `State`, `StateID`, `EventType`, `Action`, `Guard`, `MachineConfig`
- **Interpreter** — `NewInterpreter`, `Start`, `Send`, `State`, `Matches`, `Done`, `Stop`, `Close`, `UpdateContext`
- **Reflection DSL** — `MachineDef`, `StateNode`, `CompoundNode`, `FinalNode`, `ActionRegistry`, `FromStruct`
- **Snapshots** — `Snapshot`, `Restore`, `InterpreterSnapshot`
- **Hierarchy + parallel + history + delayed transitions** — all builder methods that produce these
- **Plugin system** — `plugin.Plugin`, all hook interfaces, `Composite`
- **Static analysis** — `lint.Lint`, `lint.Linter`, `Diagnostic`, all rule constants
- **Visualization** — `viz.ParseNativeJSON`, `export.NativeExporter`, ASCII / Mermaid / HTML / TUI renderers
- **Testing utilities** — `statetest`
- **HTTP integration** — `http.MachineHandler`, `http.MachineRegistry`, middleware
- **OpenTelemetry tracing** — `otel.TracingInterpreter`, `otel.TracingHook`
- **Prometheus metrics** — `metrics.MetricsInterpreter`
- **Health checks** — `health` package
- **Code generation** — `generate` package + `statekit generate` CLI
- **Clock injection** — `Clock`, `Timer`, `SystemClock`, `FakeClock`, `WithClock`

## Tier 2 — Experimental (may change in v1.x)

> **Banner:** These APIs ship today but are not covered by the same “no breaking changes in v1.x” promise as Tier 1. Prefer Tier 1 primitives when they solve the problem. If you need Tier 2 in production, pin an exact minor (`v1.13.x`) and read release notes before bumping.

These features explore problem spaces where the right shape isn't fully settled. We may iterate on the API in a future minor release based on real-world usage.

- **Actor model** — `Spawn`, `SpawnWithContext`, `ActorRef`, `SendTo`, `SendParent`, supervision strategies (`SupervisionEscalate`, `SupervisionRecover`, `SupervisionRestart`, `SupervisionStop`)
- **Persistent interpreter** — `PersistentInterpreter`, `EventStore`, `SnapshotStore`, `MemoryEventStore`, `MemorySnapshotStore`
- **Distributed execution** — `DistributedInterpreter`, `StreamLock`, `ClusterMembership`, `StreamRouter`, `MemoryStreamLock`, `ConsistentHashRouter`
- **Machine composition** — `InvokeMachine`, `MachineInvokeBuilder`, `WithChildMachine`

### Why experimental

- **Actor model** overlaps with the `InvokeMachine` builder helper for child machine composition. The right shape (one mechanism vs two) is an open question.
- **Distributed + Persistent** features compete with mature workflow engines (Temporal, Cadence). Real-world use will tell us whether to deepen these or position statekit as in-process-only.

### What "experimental" means concretely

- Bug fixes: yes, always.
- API additions: yes.
- API breaking changes: only in a clearly-numbered minor (v1.1, v1.2, ...) with deprecation warnings for at least one release.
- Removal: only after a deprecation cycle, never inside a patch release.

If you depend on a Tier 2 feature in production, pin to a specific minor version (e.g., `v1.2.x`) and read release notes before bumping.

## Tier 3 — Internal

The `internal/` packages (`internal/ir`, `internal/parser`) are not subject to semver. They can change in any release.

## See also

- [v1.0 release notes](https://github.com/klarlabs-studio/statekit/releases)
- [Backlog](./backlog.md) — what's planned next
