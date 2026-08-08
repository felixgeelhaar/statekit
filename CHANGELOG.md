# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **`statetest.InterpreterAt`** — returns a started interpreter positioned at any state of a machine, including a final one, so a property that belongs to one state can be asserted against the machine that ships rather than a variant rebuilt for the test. Built on `Interpreter.Restore`; entry actions do not run and the context is the machine's configured initial context.
- **`statetest.AssertTerminal`** — asserts an interpreter is in a final state and that none of the given events move it: the "a final state accepts nothing" property.
- **`viz.FromMachine`** — builds a visualization model directly from a compiled `*statekit.MachineConfig[C]`, with no JSON serialization and no Go source parsing in between. Machines assembled at runtime (from a transition table, a config file, a database) can now be rendered to Mermaid, ASCII, HTML, or the TUI, which makes a published diagram testable against the machine the runtime executes. `export.NewNativeExporter(...).Export()` now delegates to it and is unchanged for callers.

### Fixed

- **`viz.ParseNativeJSON` dropped hierarchy, `always` transitions, and tags from flat native JSON.** The `parent` field was ignored for top-level entries, so every state in a machine exported by `export.NewNativeExporter` came back as a root — compound, parallel, and history machines rendered flattened. `always` and `tags` had no parser fields at all. Round-tripping a machine through native JSON is now lossless.

### Documentation

- `docs/testing.md` gains a "Starting a Test at a Specific State" section covering `InterpreterAt`, `AssertTerminal`, and restoring a snapshot as the general route to an arbitrary starting state — previously documented only as persistence and recovery.

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
