
## Reposition README — kill "execution engine," lead with concrete pain

Rewrite README hero. Drop "Go-native statechart execution engine" abstract framing. Lead with concrete pain: stop modeling order/payment/incident lifecycles with switch statements + bug-prone enum FSMs. Add 30-sec viz GIF above the fold. Trim 16-bullet feature list (Fitts/Miller violation) — lead with "type-safe statecharts in 10 lines." Demote distributed/event-sourcing/actor sections to "advanced." Source: GTM + Product + UX reviews — top consensus action.

---

## Demote distributed/event-sourcing/actor packages to experimental x/ tier

v1.0 froze API on speculative scope. DistributedInterpreter, PersistentInterpreter, Actor model compete poorly with Temporal/Cadence and overlap each other (Spawn vs InvokeMachine). Move to experimental tier (x/ subpackage or "Experimental — API may change" docs banner). Protects core API stability promise. Source: Product + GTM reviews — feature surface too broad for solo maintainer.

---

## Unify builder terminator API — collapse Done/End/EndState/EndRegion

Four terminators with subtle semantics. README hierarchical example shows .End().End().End().Done() — counting parens is YAML-indentation 2.0. Hick's law violation. Solution: single .End() that walks up to nearest meaningful parent, OR accept End(StateID) for explicit unwind, with compile-time error on mismatch. Update all examples + CLAUDE.md hierarchy snippet. Source: UX review — top friction point. File refs: builder.go:329-349.

---

## Restructure docs/ to Diataxis (tutorials, how-to, reference, explanation)

20 .md files mixed paradigms. getting-started.md is part-tutorial part-reference. patterns-recipes.md (685 lines) mixes how-to + explanation. tdd.md 896 lines unclear category. Restructure to docs/tutorials/, docs/how-to/, docs/reference/, docs/explanation/. Move api-reference.md to godoc-only (single source). Delete prd.md and v1.0-roadmap.md from user-facing docs (project-meta). Add docs/choosing-an-api.md decision tree (builder vs reflection vs codegen). Fix broken xstate-export.md link in README.md:331. Source: UX review.

---

## Adopt go.uber.org/goleak + synthetic clock in tests

Two quality risks killed in one sweep. (1) Add goleak.VerifyNone in TestMain across actor, persist, distributed, invoke packages — currently no goroutine-leak detection despite many spawn paths (lock renewer, services, actors). (2) Inject clock interface (jonboulle/clockwork) into delayed transitions + supervision timers. delayed_test.go uses 50ms wall-clock windows + 30ms sleep → CI flake bomb. Source: Quality review — top 2 risks.

---

## Lift internal/ir coverage to 90%+ and add t.Parallel

internal/ir at 66.9% — foundation package (validation, hierarchy LCA, transition resolution) under-tested while CLAUDE.md claims "90%+ on critical business logic." Worst place for coverage gap given v1.0 API stability commitment. Also: zero t.Parallel() in entire test suite — misses parallel-safety bugs in package-level state. Add t.Parallel() to all leaf unit tests. Coverctl threshold for ir: raise from 80% to 90%. Source: Quality review.

---

## Extend lint package with production-hazard rules

Current 7 rules solid but miss production bugs: (1) missing-OnError-on-Invoke (silent failure path), (2) history-without-children (no-op), (3) actor-id-collision, (4) auto-forward-loop (parent-child event ping-pong), (5) unreachable-via-guard (always-false guard combos), (6) delayed-transition-shorter-than-action-duration warning. Lint is user-facing quality multiplier — biggest leverage. Source: Quality review.

---

## Ship one flagship tutorial — Stripe webhook saga with outbox

11 thin examples = 0 examples for marketing. Promote ONE killer real-world scenario as flagship. Recommended: Stripe webhook saga with DB outbox pattern (touches event sourcing, guards, OnError, durable transitions, real domain). 2000-word tutorial + companion blog post + viz GIF. Distribute aggressively (HN, /r/golang, Go Weekly). Alt candidates: K8s operator state, OAuth flow, multi-step form. Source: GTM + Product reviews.

---

## Show HN launch + Stately.ai partnership outreach

Day-zero traction. Sequence: Week 0 — Show HN "I built XState for Go" (proven hook, drives JS crowd). Week 1 — blog "Why switch statements lie about your workflow" with before/after code. Week 2 — dev.to + Go Weekly newsletter. Month 2 — GopherCon CFP + Stately.ai partnership pitch (XState compat = co-marketing angle). Lead with MCP visualizer rendering live in Claude — novel AI-era hook. Source: GTM review.

---

## Reposition AI story — "deterministic agent runtime for Go"

OTel + event-sourcing replay + property-tested transitions = unique "reproducible agent runs" story underleveraged. README rewrite to position vs LangGraph/Mastra/OpenAI Agents SDK. Determinism + replay = enterprise AI compliance angle no competitor owns. Add examples/llm_agent, examples/rag_pipeline, examples/hitl_approval. Source: AI review — top positioning move.

---

## Invert MCP — expose running machines as MCP tools/resources

Current MCP server lets Claude author/run statekit machines (dev UX). Inverse missing: mcp.ExposeMachine(interp) so any running statekit instance becomes callable MCP tool surface — events become tools, state becomes a resource. Closes loop for agent-driven workflows. Generates tool defs from machine schema. High-leverage AI move. Source: AI review.

---

## Stately.ai bidirectional sync + lightweight VSCode preview

XState refugees + visualization moat compounder. Round-trip: import Stately JSON → Go scaffold (codegen exists), export Go machines → Stately for editing. VSCode extension: live diagram on save (not full editor). Higher leverage than formal verification post-1.0 priority. Source: Product + UX reviews.

---

## AI primitives — LLMTransition, tool-call schema, streaming hooks

Three AI-native features: (1) LLMTransition/LLMGuard — builder API where LLM picks target from declared candidates; deterministic by default (seed/temp=0); span captures prompt+response. (2) Typed ToolEvent w/ JSON-Schema validation in builder — WithTool(name, schema, handler) shorthand; auto-generate MCP tool defs. (3) OnStream hook for token chunks separate from transitions; PartialContext updates via plugin without firing transitions — enables real-time UI without losing determinism. Source: AI review.

---

## aiplugin package — token/cost metrics, prompt-snapshot, retry guards

Dedicated AI plugin pack atop existing plugin system. Components: token + cost counters (Prometheus has counters but no token semantics today), prompt-snapshot in event payloads, guardrail hooks (BeforeAction interception), retry-with-backoff plugin for 429s, deterministic seed propagation. Lowers integration cost for LLM workflows. Source: AI review.

---

## Migration guides FROM looplab/fsm and qmuntal/stateless

Direct rivals = incumbent Go FSM libs. Migration guide = direct conversion path for users already using them. Side-by-side equivalents, "before/after" tables, automated conversion script if feasible. Lowers switching cost dramatically. Source: Product review.

---

## DX polish — defer interp.Stop() pattern, fuzz tests, naming fixes

Misc DX/quality cleanups: (1) Make interpreter implement io.Closer; document defer interp.Stop() in every example for muscle memory (timer cleanup buried in README:132 today). (2) Rename Done() returning *MachineBuilder from nested state to EndMachine() — silently teleports caller out of nested context (builder.go:329-333). (3) Rename InvokeBuilder/MachineInvokeBuilder to InvokeService/InvokeMachine for self-documenting (builder.go:46,57). (4) Add fuzz tests for builder, event payload, JSON exporter (only 1 fuzz file today in internal/parser). Source: UX + Quality reviews.

---

## Validate ICP via 10 Go backend leads — wedge interviews

Critical assumption to validate before further feature work: Go teams currently feel acute pain from ad-hoc FSM code (vs treating it as solved-enough). Conduct 10 user interviews with Go backend leads — ask "show me your messiest stateful service." Outcome decides whether wedge exists or this stays craft project. Source: GTM review — top "validate before committing" callout.

---

## Synthetic clock injection for delayed transitions + supervision timers

Followup to goleak task. Add Clock interface (jonboulle/clockwork or own) injectable via NewInterpreter option, default to wall clock for backward compat. Replace time.AfterFunc in interpreter.go:759 + actor supervision timers. Update delayed_test.go (5 wall-clock sleeps 30-100ms) to use synthetic clock. Eliminates CI flake risk on delayed transitions, parallel state, supervision strategies. Source: Quality review.

---

## Lint rules followup — auto-forward-loop, actor-id-collision, unreachable-via-guard

Followup batch from lint extension. (1) auto-forward-loop — parent auto-forwards event X to child; child sends event X to parent (SendParent); detect ping-pong. (2) actor-id-collision — multiple Spawn calls reuse same ActorID across states. (3) unreachable-via-guard — guard combos that are always-false (requires expression analysis or explicit annotation). (4) delayed-transition-shorter-than-action-duration — declarative annotation needed. Source: Quality review.

---

## t.Parallel sweep across full test suite

Followup to ir coverage task. Add t.Parallel() to all leaf unit tests across actor_test, builder_test, interpreter_test, history_test, parallel_test, persist_test, distributed_test, plugin_test, etc. Touches ~50 test files. Surfaces parallel-safety bugs in package-level state. Do not parallelize benchmark or property tests. Source: Quality review.

---

## Breaking renames followup — Done()→EndMachine, InvokeBuilder→InvokeService

Followup to DX polish. Two name footguns require breaking changes: (1) Done() returning *MachineBuilder from nested state silently teleports caller out of context (builder.go:329-333) — rename to EndMachine(). (2) InvokeBuilder vs MachineInvokeBuilder name collision risk — rename to InvokeServiceBuilder/InvokeMachineBuilder for self-documenting. Also update README hierarchical example showing .End().End().End().Done(). Block on v1.0 API stability decision: do via deprecation alias for one minor, then remove in v2.0; or batch with terminator-unify for one v2.0 release. Source: UX review.

---

## Refactor delayed_test.go to use FakeClock

Followup to synthetic clock injection. Now that Clock interface + FakeClock + WithClock option exist, convert delayed_test.go (5 wall-clock sleeps 30-100ms) and supervision tests in actor_test.go to use FakeClock.Advance(). Eliminates flake risk on shared CI runners. Mechanical change per file. Source: Quality review followup to commit 66ac9cf.

---
