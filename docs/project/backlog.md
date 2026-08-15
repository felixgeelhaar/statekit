
## Reposition README — kill "execution engine," lead with concrete pain

**Done (v1-safe):** README hero rewritten around concrete workflow pain; Why/feature list trimmed; Tier 2 demoted; XState framed as optional export. Viz GIF added (`docs/public/viz-demo.gif`).

Rewrite README hero. Drop "Go-native statechart execution engine" abstract framing. Lead with concrete pain: stop modeling order/payment/incident lifecycles with switch statements + bug-prone enum FSMs. Add 30-sec viz GIF above the fold. Trim 16-bullet feature list (Fitts/Miller violation) — lead with "type-safe statecharts in 10 lines." Demote distributed/event-sourcing/actor sections to "advanced." Source: GTM + Product + UX reviews — top consensus action.

---

## Demote distributed/event-sourcing/actor packages to experimental x/ tier

**Done (docs banner, stay on v1):** Tier 2 marked experimental in godoc + `docs/reference/stability.md` + README. Physical `x/` package move deferred (breaking).

v1.0 froze API on speculative scope. DistributedInterpreter, PersistentInterpreter, Actor model compete poorly with Temporal/Cadence and overlap each other (Spawn vs InvokeMachine). Move to experimental tier (x/ subpackage or "Experimental — API may change" docs banner). Protects core API stability promise. Source: Product + GTM reviews — feature surface too broad for solo maintainer.

---

## Unify builder terminator API — collapse Done/End/EndState/EndRegion

**Done (additive):** `TransitionBuilder.Up()` (≡ `End().End()`), `EndTo(parent)` unwind, docs updated. Existing Done/End/EndState/EndRegion kept for compatibility.

Four terminators with subtle semantics. README hierarchical example shows .End().End().End().Done() — counting parens is YAML-indentation 2.0. Hick's law violation. Solution: single .End() that walks up to nearest meaningful parent, OR accept End(StateID) for explicit unwind, with compile-time error on mismatch. Update all examples + CLAUDE.md hierarchy snippet. Source: UX review — top friction point. File refs: builder.go:329-349.

---

## Restructure docs/ to Diataxis (tutorials, how-to, reference, explanation)

**Done** (layout live under `docs/{tutorials,how-to,reference,explanation,project}/`; Astro glob updated; site slugs unchanged).

20 .md files mixed paradigms. getting-started.md is part-tutorial part-reference. patterns-recipes.md (685 lines) mixes how-to + explanation. tdd.md 896 lines unclear category. Restructure to docs/tutorials/, docs/how-to/, docs/reference/, docs/explanation/. Move api-reference.md to godoc-only (single source). Delete prd.md and v1.0-roadmap.md from user-facing docs (project-meta). Add docs/choosing-an-api.md decision tree (builder vs reflection vs codegen). Fix broken xstate-export.md link in README.md:331. Source: UX review.

---

## Adopt go.uber.org/goleak + synthetic clock in tests

**Done:** `goleak.VerifyTestMain` in root + `distributed`; `FakeClock` / `WithClock` for delayed transitions; async tests prefer `waitUntil` over fixed Sleeps.

Two quality risks killed in one sweep. (1) Add goleak.VerifyNone in TestMain across actor, persist, distributed, invoke packages — currently no goroutine-leak detection despite many spawn paths (lock renewer, services, actors). (2) Inject clock interface (jonboulle/clockwork) into delayed transitions + supervision timers. delayed_test.go uses 50ms wall-clock windows + 30ms sleep → CI flake bomb. Source: Quality review — top 2 risks.

---

## Lift internal/ir coverage to 90%+ and add t.Parallel

**Done:** `internal/ir` at 100% statement coverage; leaf tests use `t.Parallel()` widely.

internal/ir at 66.9% — foundation package (validation, hierarchy LCA, transition resolution) under-tested while CLAUDE.md claims "90%+ on critical business logic." Worst place for coverage gap given v1.0 API stability commitment. Also: zero t.Parallel() in entire test suite — misses parallel-safety bugs in package-level state. Add t.Parallel() to all leaf unit tests. Coverctl threshold for ir: raise from 80% to 90%. Source: Quality review.

---

## Extend lint package with production-hazard rules

**Done:** invoke-missing-onerror, invoke-id-collision, auto-forward-redundancy, deep-nesting, history-without-siblings, guarded-only-entry, guarded-event-without-fallback, auto-forward-loop, actor-id-collision. Full always-false guard expression analysis deferred (heuristic coverage via guarded-only-entry + guarded-event-without-fallback).

Current 7 rules solid but miss production bugs: (1) missing-OnError-on-Invoke (silent failure path), (2) history-without-children (no-op), (3) actor-id-collision, (4) auto-forward-loop (parent-child event ping-pong), (5) unreachable-via-guard (always-false guard combos), (6) delayed-transition-shorter-than-action-duration warning. Lint is user-facing quality multiplier — biggest leverage. Source: Quality review.

---

## Ship one flagship tutorial — Stripe webhook saga with outbox

**Done** — see `docs/tutorials/stripe-webhook-saga.md` + `examples/stripe_webhook`.

11 thin examples = 0 examples for marketing. Promote ONE killer real-world scenario as flagship. Recommended: Stripe webhook saga with DB outbox pattern (touches event sourcing, guards, OnError, durable transitions, real domain). 2000-word tutorial + companion blog post + viz GIF. Distribute aggressively (HN, /r/golang, Go Weekly). Alt candidates: K8s operator state, OAuth flow, multi-step form. Source: GTM + Product reviews.

---

## Show HN launch + Stately.ai partnership outreach

Day-zero traction. Sequence: Week 0 — Show HN "I built XState for Go" (proven hook, drives JS crowd). Week 1 — blog "Why switch statements lie about your workflow" with before/after code. Week 2 — dev.to + Go Weekly newsletter. Month 2 — GopherCon CFP + Stately.ai partnership pitch (XState compat = co-marketing angle). Lead with MCP visualizer rendering live in Claude — novel AI-era hook. Source: GTM review.

---

## Reposition AI story — "deterministic agent runtime for Go"

OTel + event-sourcing replay + property-tested transitions = unique "reproducible agent runs" story underleveraged. README rewrite to position vs LangGraph/Mastra/OpenAI Agents SDK. Determinism + replay = enterprise AI compliance angle no competitor owns. Add examples/llm_agent, examples/rag_pipeline, examples/hitl_approval. Source: AI review — top positioning move.

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

**Done** — see `docs/tutorials/migration-from-looplab-fsm.md` and `docs/tutorials/migration-from-qmuntal-stateless.md` (+ README "Coming from another FSM library?").

Direct rivals = incumbent Go FSM libs. Migration guide = direct conversion path for users already using them. Side-by-side equivalents, "before/after" tables, automated conversion script if feasible. Lowers switching cost dramatically. Source: Product review.

---

## DX polish — defer interp.Stop() pattern, fuzz tests, naming fixes

**Done (partial):** `Interpreter.Close()` + README/`defer interp.Close()` in examples; InvokeMachine/InvokeService aliases; export Native JSON fuzz; builder/interpreter fuzz (`fuzz_test.go`). Breaking Done→EndMachine rename deferred (v1).

Misc DX/quality cleanups: (1) Make interpreter implement io.Closer; document defer interp.Stop() in every example for muscle memory (timer cleanup buried in README:132 today). (2) Rename Done() returning *MachineBuilder from nested state to EndMachine() — silently teleports caller out of nested context (builder.go:329-333). (3) Rename InvokeBuilder/MachineInvokeBuilder to InvokeService/InvokeMachine for self-documenting (builder.go:46,57). (4) Add fuzz tests for builder, event payload, JSON exporter (only 1 fuzz file today in internal/parser). Source: UX + Quality reviews.

---

## Validate ICP via 10 Go backend leads — wedge interviews

Critical assumption to validate before further feature work: Go teams currently feel acute pain from ad-hoc FSM code (vs treating it as solved-enough). Conduct 10 user interviews with Go backend leads — ask "show me your messiest stateful service." Outcome decides whether wedge exists or this stays craft project. Source: GTM review — top "validate before committing" callout.

---

## Synthetic clock injection for delayed transitions + supervision timers

**Done:** `Clock` / `FakeClock` / `WithClock`; `delayed_test.go` uses FakeClock.

Followup to goleak task. Add Clock interface (jonboulle/clockwork or own) injectable via NewInterpreter option, default to wall clock for backward compat. Replace time.AfterFunc in interpreter.go:759 + actor supervision timers. Update delayed_test.go (5 wall-clock sleeps 30-100ms) to use synthetic clock. Eliminates CI flake risk on delayed transitions, parallel state, supervision strategies. Source: Quality review.

---

## Lint rules followup — auto-forward-loop, actor-id-collision, unreachable-via-guard

**Done:** `auto-forward-loop`, `actor-id-collision`, `guarded-only-entry` (inbound), `guarded-event-without-fallback` (outbound). Full expression-level always-false analysis still deferred.

Followup batch from lint extension. (1) auto-forward-loop — parent auto-forwards event X to child; child sends event X to parent (SendParent); detect ping-pong. (2) actor-id-collision — multiple Spawn calls reuse same ActorID across states. (3) unreachable-via-guard — guard combos that are always-false (requires expression analysis or explicit annotation). (4) delayed-transition-shorter-than-action-duration — declarative annotation needed. Source: Quality review.

---

## t.Parallel sweep across full test suite

**Done:** leaf unit tests across core, export, lint, internal, http/otel/metrics/health, viz, and examples (except wall-clock `session_timeout`).

Followup to ir coverage task. Add t.Parallel() to all leaf unit tests across actor_test, builder_test, interpreter_test, history_test, parallel_test, persist_test, distributed_test, plugin_test, etc. Touches ~50 test files. Surfaces parallel-safety bugs in package-level state. Do not parallelize benchmark or property tests. Source: Quality review.

---

## Breaking renames followup — Done()→EndMachine, InvokeBuilder→InvokeService

Followup to DX polish. Two name footguns require breaking changes: (1) Done() returning *MachineBuilder from nested state silently teleports caller out of context (builder.go:329-333) — rename to EndMachine(). (2) InvokeBuilder vs MachineInvokeBuilder name collision risk — rename to InvokeServiceBuilder/InvokeMachineBuilder for self-documenting. Also update README hierarchical example showing .End().End().End().Done(). Block on v1.0 API stability decision: do via deprecation alias for one minor, then remove in v2.0; or batch with terminator-unify for one v2.0 release. Source: UX review.

---

## Refactor delayed_test.go to use FakeClock

**Done.**

Followup to synthetic clock injection. Now that Clock interface + FakeClock + WithClock option exist, convert delayed_test.go (5 wall-clock sleeps 30-100ms) and supervision tests in actor_test.go to use FakeClock.Advance(). Eliminates flake risk on shared CI runners. Mechanical change per file. Source: Quality review followup to commit 66ac9cf.

---

## Reposition away from "XState for Go" — two ports already exist

**Done (README):** hero leads with Go workflow pain; XState/Stately mentioned as optional export, not identity.

ICP signal: dstotijn/go-xstate and CorrectRoadH/XState-For-Golang already occupy the "XState port" framing. Update README hero from "XState compatibility" lead to "statecharts for Go" lead. Frame XState/Stately compat as a feature (round-trip via XStateExporter), not as the identity. Show HN headline candidate: "Statekit — Go statecharts with hierarchy, parallel states, visual debugger" not "XState for Go". Source: ICP signal sweep — competing ports already exist with the same framing.

---

## README "Coming from looplab/fsm or qmuntal/stateless?" callout

**Done** — README section + migration tutorials.

ICP signal: looplab #40 (persistence pain), looplab #86 (generics request), looplab #115 (context cancellation bug), qmuntal #77 (halt FSM pain), qmuntal #94 (Mermaid export missing), qmuntal #98/#99 (OnEntry hierarchy bugs). Add a 5-bullet README section: "Tired of FSM that can't persist? Tired of InTransitionError after context cancellation? Tired of broken self-transitions?" — directly addressing the verbatim pain. Link to existing migration guides. Source: ICP signal sweep — top open issues across both incumbents.

---

## Verify context-cancellation correctness — counter-test for looplab #115

**Done** — `cancellation_recovery_test.go`.

looplab/fsm #115 reports a bug where ctx.Err leaves FSM in InTransitionError state with no recovery path. Add an explicit test in interpreter_test.go that exercises this scenario in statekit and confirms our interpreter recovers correctly: send event with cancelled context, verify subsequent events still process. Test doubles as a public differentiator and a regression guard. Source: ICP signal sweep — direct quote from looplab #115.

---

## Verify Snapshot is gob/JSON-encodable — counter-test for looplab #40

**Done** — `snapshot_serialization_test.go`.

looplab/fsm #40 reports that the FSM type has no exported fields, blocking gob encoding for production persistence. Add an explicit test that takes interp.Snapshot(), serializes it via json.Marshal AND encoding/gob, deserializes, and Restore()s into a fresh interpreter. Demonstrates that statekit's snapshot is publicly serializable. Lifts the result to a docs/snapshot-serialization.md or a section in api-reference.md. Source: ICP signal sweep — direct quote from looplab #40.

---

## Add transition-budget lint rule — directly addresses qmuntal #77

**Done (docs):** recommended pattern in [`docs/how-to/transition-budget.md`](../how-to/transition-budget.md) + Stripe webhook example. Dedicated lint/plugin deferred.

qmuntal #77 author needs to halt workflow execution after N transitions (runaway prevention). Add a lint rule "missing-transition-budget" or document the recommended pattern: use Action+Guard pair with attempt counter (the stripe_webhook example pattern). Could also ship as a new aiplugin.RetryBudget[C] plugin that wraps OnEvent and emits a HALT event when count exceeds threshold. Source: ICP signal sweep — direct quote from qmuntal #77.

---

## DM 4 candidate interviewees from competitor issues

User-driven action. Reach out via GitHub to four authors of public pain issues: looplab/fsm #40 (persistence pain, multi-FSM use case), looplab/fsm #109 (external state storage + instance reuse), looplab/fsm #115 (context cancellation bug), qmuntal/stateless #77 (workflow execution + retry budget). 30-min interview each. Real signal at low cost since they self-identified the pain. Source: ICP signal sweep.

---

## README two-narrative thread — backend wedge + AI wedge

ICP signal: Go AI agent runtime is open territory (only tRPC-Agent-Go exists; LangGraph/AutoGen/CrewAI are Python). Pair the Stripe webhook example (proves backend wedge) with llm_agent example (proves AI wedge) in a coherent README arc: "Statekit fits two jobs that look similar — backend workflow logic and deterministic agent runtime." Cite the existing examples explicitly. Source: ICP signal sweep — Vellum 2026 industry articulation matches statekit's narrative verbatim.

---

## Build landing page at / — move visualizer to /play

Top consensus action across UX/GTM/Product/Frontend reviews. Replace visualizer-as-homepage with real landing: hero ("Statecharts for Go"), 10-line code sample lifted from README, `go get` install block, dual-job section (backend workflows + AI agents), comparison table vs looplab/qmuntal/Temporal, CTAs (GitHub star + Quickstart + Try Visualizer). Move existing Visualizer.vue to /play route. Source: 4-expert website review.

---

## Publish 30+ docs via Astro content collection

30+ markdown docs (getting-started, hierarchical-states, guards-actions, migration-from-*, stability, etc.) exist in repo but only the visualizer is rendered as site. Wire Astro content collection or @astrojs/starlight so /docs/* renders. Migration pages = highest intent capture for SEO ("looplab fsm alternative", "go statechart library"). Source: 4-expert website review — content asset wasted.

---

## OG card, Twitter meta, sitemap, per-page titles

Site is share-invisible. Title is "Statekit Visualizer" — kills SEO for "go state machine", "go statechart library", "looplab fsm alternative". Add: OG image with "Statecharts for Go" + code snippet, Twitter card meta, sitemap.xml, structured data (SoftwareSourceCode JSON-LD), per-page <title> + <meta description>. Source: GTM review — distribution leak.

---

## Visualizer accessibility sweep — WCAG 2.2 AA

Multiple WCAG hits identified by UX review: 2.4.7 outline:none with no :focus-visible replacement; 4.1.2 icon-only buttons missing aria-label, modal missing role=dialog/aria-modal/focus trap, toast container missing aria-live=polite; 1.4.3 --text-muted #484f58 on #0a0e14 ≈ 3.5:1 (fails 4.5:1); 2.1.1 canvas pan/zoom mouse-only; 2.3.3 no @media (prefers-reduced-motion: reduce). Fix: global :focus-visible rule, prefers-reduced-motion guard, aria-labels on toolbar/modal-close/header buttons, role="dialog" + aria-modal + focus-trap on KeyboardShortcuts modal, role="status" + aria-live="polite" on toast container, bump --text-muted to #7d8590 (≥4.5:1), add pointer/touch handlers on StateCanvas. Source: UX review.

---

## Convert Visualizer to Astro island + auto-load sample

Frontend review: index.astro:13-19 boots Vue manually via createApp in inline script, bypassing Astro islands. Vue runtime + entire visualizer hydrate eagerly even though canvas not in viewport on mobile. Convert <Visualizer /> to Astro Vue island with client:idle (or client:visible). Defer 76KB Vue runtime past LCP, ~30% TTI improvement. Also: auto-load sample machine on first visit (localStorage flag) — kills empty-state bounce per UX review (Jakob's law violation). Source: Frontend + UX reviews.

---

## Visualizer robustness — infinite-loop guard + roundRect fallback + error boundary

**Done:** cycle detection in `useSimulation`; `roundRectCompat` in StateCanvas; `onErrorCaptured` + toasts; clipboard try/catch + execCommand fallback; json-validator cyclic `initial` + children↔parent consistency (+ Vitest).

Frontend review: (1) Visualizer.vue:280-282 + :313-315 use `while (states[currentState].initial)` — infinite loop if initial chain cycles. Add cycle detection. (2) StateCanvas.vue:209 ctx.roundRect not in older Safari (<16) — silent throw breaks render. Add fillRect fallback or Path2D polyfill. (3) No app.config.errorHandler — Vue throws yield blank page. Add global error handler that surfaces to toast + console. (4) MachineJson.vue:43 navigator.clipboard.writeText unhandled rejection in older browsers / insecure context. Add try/catch with fallback. (5) json-validator.ts doesn't validate cyclic initial chains or that children[] matches actual parent refs. Source: Frontend review.

---

## Share URL + copy-as-Mermaid + copy-as-Go-builder

Product review: viral primitive. Add URL hash with LZ-compressed JSON (?m=...) so users can share machines via permalink. Add three copy buttons: copy permalink, copy as Mermaid (uses existing viz/mermaid renderer), copy as Go builder (synthesize statekit.NewMachine[...] code from Native JSON). Unlocks HN/Twitter/blog embeds. Source: Product review.

---

## Visualizer mobile + persistent JSON errors

UX review: mobile dead. components.css:10-15 only swaps grid columns at <1024px, sidebar stacks above canvas pushing it below fold. Visualizer.vue:182-188 hardcoded width=700/height=500 ignores viewport. StateCanvas.vue:127 wheel/drag — no touch handlers. Fix: pointer events for pan/zoom, drawer or tabs for sidebar on mobile, viewport-relative layout. Also: JSON error UX is 3s toast that disappears. Visualizer.vue:165-167. Add persistent inline error block under textarea with line/column hint + link to JSON schema; sticky toasts for errors. Source: UX review.

---

## Refactor Visualizer.vue + StateCanvas.vue god components

Frontend review: Visualizer.vue (424 LOC) mixes state ownership, layout calc (calculatePositions 92 LOC), simulation engine, tooltip positioning, toast manager, keyboard handler. StateCanvas.vue (424 LOC) has render logic that should move to pure utils. Extract: composables/useSimulation.ts, composables/useLayout.ts, composables/useToasts.ts, composables/useKeyboard.ts; utils/canvas-renderer.ts (pure drawState/drawArrow/drawTransitions). DRY transition-bubble logic duplicated between Visualizer.vue:298-309 and SimulationPanel.vue:91-103. Source: Frontend review.

---

## Frontend test infrastructure — Vitest + Playwright + astro check in CI

Frontend review: zero CI safety net for visualizer. Add Vitest unit tests for json-validator.ts + new composables, Playwright e2e for paste-JSON → simulate → verify state badge, @astrojs/check + typescript in devDeps so CI can type-check. Pin caret deps to exact versions for reproducible builds. Source: Frontend review.

---
