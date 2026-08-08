# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`viz.FromMachine`** — builds a visualization model directly from a compiled `*statekit.MachineConfig[C]`, with no JSON serialization and no Go source parsing in between. Machines assembled at runtime (from a transition table, a config file, a database) can now be rendered to Mermaid, ASCII, HTML, or the TUI, which makes a published diagram testable against the machine the runtime executes. `export.NewNativeExporter(...).Export()` now delegates to it and is unchanged for callers.

### Changed

- **`StateBuilder.End` on a top-level state and `StateBuilder.EndState` outside a parallel region now panic with a message naming the terminator to use instead.** Both previously returned nil, so the mistake surfaced as an unexplained nil-pointer dereference at whatever call came next. No working program is affected: a nil builder could only ever panic.
- Doc comments on all builder terminators (`Done`, `End`, `EndState`, `EndRegion`, `EndMachine`) rewritten to name the level each returns to, with an example of the shape it belongs to.

### Deprecated

- **`StateBuilder.EndMachine` and `TransitionBuilder.EndMachine`** — use `Done`, which is identical in signature, behaviour, and return value. Two spellings of one terminator was the confusion; nothing at a call site distinguished them. `EndMachine` keeps working and is not scheduled for removal.

### Fixed

- **`viz.ParseNativeJSON` dropped hierarchy, `always` transitions, and tags from flat native JSON.** The `parent` field was ignored for top-level entries, so every state in a machine exported by `export.NewNativeExporter` came back as a root — compound, parallel, and history machines rendered flattened. `always` and `tags` had no parser fields at all. Round-tripping a machine through native JSON is now lossless.

### Documentation

- A "Closing the builder" section in the package documentation and the README, showing the flat, nested, and parallel shapes side by side plus the build-in-a-loop case, with a table of which terminator returns where.

## [1.9.0] - 2026-06-20

### Removed

- **`ai` package** — LLM-driven transitions (`Drive`, `Tool` schema) removed; statekit core no longer knows about LLMs. Rehomed in agent-go as `contrib/ai`. The only in-stack consumer was `examples/llm_agent` (dropped below), so no remaining package imports it.
- **`aiplugin` package** — `TokenCounter`, `PromptRecorder`, `TransitionBudget` removed. Rehomed in agent-go as `contrib/aiplugin`. No deprecated shims are left behind.
- **`examples/llm_agent`** — the deterministic RAG agent example has been dropped.

### Changed

- README, documentation site, and migration guides no longer position statekit as an "AI agent runtime." statekit is a typed statechart library for backend domain workflows. The interpreter, builder, types, snapshot, lint, export, statetest, and all other core packages are unchanged and agent-free.

### Migration

- Anyone importing `go.klarlabs.de/statekit/ai` or `go.klarlabs.de/statekit/aiplugin` should switch to the agent-go `contrib/ai` / `contrib/aiplugin` equivalents. The statekit module path is unchanged — this stays on the v1 line.

## [0.2.0] - 2025-12-25

### Added

- **Hierarchical States**: Compound/nested states with parent-child relationships
- **Event Bubbling**: Events unhandled by child states bubble up to ancestors
- **Proper Entry/Exit Ordering**: Leaf-to-root exit, root-to-leaf entry
- **Self-Transition Support**: External transition semantics (exit and re-enter)
- **XState JSON Exporter**: Export machines for visualization with Stately.ai
- **Pedestrian Light Example**: Demonstrates hierarchical states and event bubbling
- `Matches()` now returns true for ancestor states
- `UpdateContext()` method for modifying interpreter context

### Changed

- `Start()` now recursively enters nested initial states to reach leaf state
- Transition resolution now uses Lowest Common Ancestor (LCA) algorithm

## [0.1.0] - 2025-12-25

### Added

- Initial release
- Fluent builder API with Go generics for type-safe context
- Synchronous interpreter with deterministic execution
- Guards for conditional transitions
- Actions for entry, exit, and transition side effects
- Build-time validation for machine configuration
- Final states support
- Traffic light example

[1.9.0]: https://github.com/klarlabs-studio/statekit/releases/tag/v1.9.0
[0.2.0]: https://github.com/klarlabs-studio/statekit/releases/tag/v0.2.0
[0.1.0]: https://github.com/klarlabs-studio/statekit/releases/tag/v0.1.0
