# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2026-06-20

### Removed

- **BREAKING: `ai` package** — LLM-driven transitions (`Drive`, `Tool` schema) have been removed. statekit core no longer knows about LLMs. This capability has been rehomed in agent-go as `contrib/ai`.
- **BREAKING: `aiplugin` package** — AI plugins (`TokenCounter`, `PromptRecorder`, `TransitionBudget`) have been removed. They have been rehomed in agent-go as `contrib/aiplugin`. No deprecated shims are left behind.
- **`examples/llm_agent`** — the deterministic RAG agent example has been dropped.

### Changed

- README, documentation site, and migration guides no longer position statekit as an "AI agent runtime." statekit is a typed statechart library for backend domain workflows. The interpreter, builder, types, snapshot, lint, export, statetest, and all other core packages are unchanged and agent-free.

### Migration

- Replace imports of `go.klarlabs.de/statekit/ai` and `go.klarlabs.de/statekit/aiplugin` with their agent-go `contrib/ai` and `contrib/aiplugin` equivalents.

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

[2.0.0]: https://github.com/klarlabs-studio/statekit/releases/tag/v2.0.0
[0.2.0]: https://github.com/klarlabs-studio/statekit/releases/tag/v0.2.0
[0.1.0]: https://github.com/klarlabs-studio/statekit/releases/tag/v0.1.0
