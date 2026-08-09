# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **`viz.ParseNativeJSON` dropped every delayed (`after`) transition.**
  `rawState` had no `After` field, so a state whose only edge is a timeout
  parsed as terminal — zero transitions, no error. A machine rendered from it
  showed two disconnected states, which reads as a modelling mistake rather
  than a parser gap. `VizTransition` has carried `IsDelayed` and `DelayMs`
  since 0.2.0; nothing ever set them.

  This is the same defect as `always` before 1.13.0, in the same file, and it
  survived that fix: 1.13.1 taught the parser every *shape* the exporter
  writes (object, array, raise descriptors) without noticing there was a
  *field* it never read at all.

  A non-numeric delay key — an XState delay reference such as
  `{"after": {"TIMEOUT": …}}` — keeps its edge and carries the name, rather
  than being discarded. It cannot be drawn as a duration, but dropping it
  would misreport the state as terminal, which is the bug being fixed.

  The `export` round-trip test now covers a delayed transition, so the seam is
  checked rather than each half in isolation.

## [1.13.2] - 2026-08-08

### Fixed

- **A 12 MB compiled binary is no longer shipped inside the module.** `go build ./cmd/statekit` run from the repo root writes `./statekit`, which is extensionless and so matched none of `.gitignore`'s binary patterns (`*.exe`, `*.dll`, `*.so`, `*.dylib`). It was committed on 2026-06-21 and has ridden along in every release since — a darwin/arm64 Mach-O executable, useless on any other platform, downloaded by every consumer through the module proxy and copied into the working tree of every consumer that vendors. It accounted for very nearly the whole repository size. Removed, and `.gitignore` now covers the paths `go build` actually writes.

  The published v1.9.0–v1.13.1 zips still contain it; module versions are immutable. Upgrading to v1.13.2 is what removes it from a consumer's tree.

## [1.13.1] - 2026-08-08

### Fixed

- **`viz.ParseNativeJSON` now reads every transition shape `export.NewXStateExporter` writes.** The two are halves of one round-trip and disagreed about the shape of a transition group: `export.collapseTransitionGroup` writes an object when a group holds exactly one transition and an array otherwise, while the parser read exactly one of those shapes per field.

  Three consequences, none of which reported an error:

  - **A single eventless transition broke the whole parse.** `rawState.Always` was `[]VizTransition`, so `"always": {"target": "b"}` — what the exporter emits for a group of one — failed `ParseNativeJSON` outright with `cannot unmarshal object into Go struct field rawState.states.always`. This is a **regression introduced in 1.13.0**: 1.12.0 and earlier accepted the same input because they silently dropped `always` altogether, which is the bug 1.13.0 set out to fix. Any machine with exactly one eventless transition exported JSON that statekit's own parser, and therefore `statekit viz`, could not read.
  - **Guarded alternatives on one event vanished.** A second transition on the same event makes the exporter emit an array; the parser tried a string, then a single object, and returned nothing. Zero transitions, no error — a diagram missing an edge reads exactly like a machine that has no such edge.
  - **A transition raising an internal event vanished.** `transitionEntry` widens `actions` from `[]string` to `[]any` to embed `xstate.raise` descriptors; the parser's `Actions []string` could not hold that, so the unmarshal failed and took the entire transition with it. Raised events now populate `VizTransition.Raise`.

  Internal transitions (`{"internal": true}`, no target) are also kept now rather than discarded, since the actions they run are the only reason they exist.

  Covered by a round-trip test in `export` that marshals a machine and parses it straight back — the seam neither package's suite was testing. Both packages were well covered on their own sides of it.

## [1.13.0] - 2026-08-08

### Added

- **`statetest.InterpreterAt`** — returns a started interpreter positioned at any state of a machine, including a final one, so a property that belongs to one state can be asserted against the machine that ships rather than a variant rebuilt for the test. Built on `Interpreter.Restore`; entry actions do not run and the context is the machine's configured initial context.
- **`statetest.AssertTerminal`** — asserts an interpreter is in a final state and that none of the given events move it: the "a final state accepts nothing" property.
- **`viz.FromMachine`** — builds a visualization model directly from a compiled `*statekit.MachineConfig[C]`, with no JSON serialization and no Go source parsing in between. Machines assembled at runtime (from a transition table, a config file, a database) can now be rendered to Mermaid, ASCII, HTML, or the TUI, which makes a published diagram testable against the machine the runtime executes. `export.NewNativeExporter(...).Export()` now delegates to it and is unchanged for callers.

### Changed

- **`StateBuilder.End` on a top-level state and `StateBuilder.EndState` outside a parallel region now panic with a message naming the terminator to use instead.** Both previously returned nil, so the mistake surfaced as an unexplained nil-pointer dereference at whatever call came next. No working program is affected: a nil builder could only ever panic.
- Doc comments on all builder terminators (`Done`, `End`, `EndState`, `EndRegion`, `EndMachine`) rewritten to name the level each returns to, with an example of the shape it belongs to.

### Deprecated

- **`StateBuilder.EndMachine` and `TransitionBuilder.EndMachine`** — use `Done`, which is identical in signature, behaviour, and return value. Two spellings of one terminator was the confusion; nothing at a call site distinguished them. `EndMachine` keeps working and is not scheduled for removal.

### Fixed

- **`viz.ParseNativeJSON` dropped hierarchy, `always` transitions, and tags from flat native JSON.** The `parent` field was ignored for top-level entries, so every state in a machine exported by `export.NewNativeExporter` came back as a root — compound, parallel, and history machines rendered flattened. `always` and `tags` had no parser fields at all. Round-tripping a machine through native JSON is now lossless. (Only for the array form of `always`; the object form regressed and is fixed in 1.13.1 above.)

### Documentation

- A "Closing the builder" section in the package documentation and the README, showing the flat, nested, and parallel shapes side by side plus the build-in-a-loop case, with a table of which terminator returns where.
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
