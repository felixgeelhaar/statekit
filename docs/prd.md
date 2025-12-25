Product Requirements Document (PRD)
Project: Go Statecharts with XState Compatibility

Working title: Statekit (placeholder)
Category: Open-source backend infrastructure library
Status: Draft v1.0
Owner: You
Audience: Backend engineers, platform teams, SREs, workflow/automation builders

1. Problem Statement

Backend engineers increasingly need to model complex, long-lived stateful processes such as:

incident workflows

operational automation

orchestration logic

domain-driven process lifecycles

In frontend and product engineering, XState has emerged as the de-facto standard for modeling such systems using statecharts, offering clarity, correctness, and visualization.

However, Go lacks an equivalent:

Existing Go FSM libraries are too primitive

Workflow engines are too heavyweight

No solution supports statecharts + visualization

No tool bridges Go execution with XState tooling

As a result:

Teams either hand-roll brittle state logic

Or over-adopt workflow engines for simple orchestration

Or lose the ability to reason, visualize, and document state

This is a systemic gap in the Go ecosystem.

2. Product Vision

Enable Go developers to define, execute, and visualize statecharts with the same clarity and rigor that XState provides — without sacrificing Go’s simplicity or control.

The product will:

Execute statecharts natively in Go

Export machine definitions to XState-compatible JSON

Enable visualization, simulation, and documentation using existing XState tooling

Remain small, explicit, and production-safe

This is not an XState clone.
It is a Go-native statechart engine with XState interoperability.

3. Target Users & Personas
   Primary Persona: Platform / Backend Engineer

Builds event-driven systems

Cares about correctness and debuggability

Wants explicit state modeling

Uses Go for services, automation, orchestration

Secondary Persona: SRE / Incident Automation Engineer

Models incident lifecycles

Needs predictable transitions

Wants visual documentation for runbooks

Tertiary Persona: Technical Lead / Architect

Wants shared mental models

Needs diagrams that match runtime behavior

Values OSS primitives over frameworks

4. User Problems & Jobs To Be Done
   Jobs To Be Done

“Model a complex process without if/else sprawl”

“Make state transitions explicit and testable”

“Visualize backend workflows for humans”

“Ensure production behavior matches documentation”

Pain Points Today

FSMs lack hierarchy and expressiveness

Workflow engines add operational overhead

Diagrams drift from code

State logic becomes implicit and fragile

5. Goals & Success Metrics
   Product Goals

Become the default answer to “XState for Go?”

Be adopted as a core primitive, not a framework

Enable diagram-driven understanding of backend systems

Success Metrics (OSS)

GitHub stars & forks

Mentions in blog posts / talks

Adoption in real systems (issues, discussions)

Community contributions

6. Non-Goals (Very Important)

The product will not:

Re-implement the full XState spec

Replace workflow engines (Temporal, Cadence, etc.)

Provide a UI or hosted service (initially)

Abstract away Go semantics

Hide state transitions behind magic

7. Core Product Principles

Go-first execution

Explicit

Deterministic

Testable

Statecharts, not FSMs

Hierarchy is a first-class concept

Visualization as a feature

Exportable, not proprietary

Interop over imitation

XState JSON is a compatibility target

Small surface area

Fewer features, better guarantees

8. Functional Requirements (MVP)
   8.1 State Machine Definition

The system must support:

Machine ID

Initial state

Named states

Nested (hierarchical) states

Final states

8.2 Transitions

Event-driven transitions

Source → target mapping

Guard conditions

Transition actions

8.3 Actions

Entry actions

Exit actions

Transition actions

Executed as Go functions

8.4 Guards

Boolean predicates

Pure functions

Evaluated deterministically

8.5 Execution Engine

Synchronous interpreter

Explicit Send(event) API

Deterministic transition resolution

Current state query

8.6 Export Capability (Critical)

Export machine to XState-compatible JSON

Output must be pasteable into:

Stately Visualizer

XState Inspect tools

Export must reflect actual runtime semantics

9. Configuration & Authoring
   Supported Authoring Styles (MVP)

Builder / Fluent API

Explicit and type-safe

Reflection-based DSL (struct + tags)

Declarative

Config-like

Optional but strongly desired

XState JSON is not the primary authoring format.

10. Out of Scope (v1)

Explicitly deferred:

Parallel / orthogonal states

History states

Delayed / timed transitions

Invoked actors / services

Persistence / durability

Distributed execution

These are intentional constraints.

11. Competitive Landscape
    Direct

Go FSM libraries → too limited

UML/HSM libs → poor ergonomics, no XState interop

Indirect

Workflow engines → too heavy

Custom state logic → unmaintainable

Key Differentiator:

Only Go library that combines executable statecharts with XState-compatible visualization.

12. Risks & Mitigations
    Risk: Over-engineering

Mitigation: ruthless v1 scope control

Risk: XState spec drift

Mitigation: support a stable JSON subset only

Risk: Low adoption

Mitigation: sharp positioning + excellent docs + visual payoff

13. Release Plan (High-Level)
    v0.1

Core IR

Execution engine

Simple builder API

v0.2

Hierarchical states

Guards & actions

XState JSON exporter

v0.3

Reflection/tag DSL

CLI export tool

Docs + examples

14. Open Questions

Naming / branding

How far XState compatibility should go

Whether to support JSON import (round-trip) later

Parallel states timing (v2?)

15. One-Sentence Pitch (for README)

Define and execute statecharts in Go — visualize them with XState tooling.
