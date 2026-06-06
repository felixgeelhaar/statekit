import type { MachineConfig } from './types';
import { parseMachineJSON } from './json-validator';

// URL hash key. Updating this is a breaking change for existing
// permalinks — keep stable.
const HASH_KEY = 'm';

// Synchronous gzip-via-CompressionStream is not possible because
// CompressionStream is async. We base64-encode the raw JSON instead;
// modern URL fragments routinely carry hundreds of KB without issue.
// If size becomes a concern we'll swap to async encode/decode.

function toBase64Url(input: string): string {
  return btoa(unescape(encodeURIComponent(input)))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
}

function fromBase64Url(input: string): string {
  const padded = input
    .replace(/-/g, '+')
    .replace(/_/g, '/')
    + '='.repeat((4 - (input.length % 4)) % 4);
  return decodeURIComponent(escape(atob(padded)));
}

export function encodeMachine(machine: MachineConfig): string {
  return toBase64Url(JSON.stringify(machine));
}

export function decodeMachine(encoded: string): MachineConfig | null {
  if (!encoded) return null;
  try {
    const json = fromBase64Url(encoded);
    return parseMachineJSON(json);
  } catch {
    return null;
  }
}

/** Reads the current location hash and extracts a machine, if present. */
export function readMachineFromHash(): MachineConfig | null {
  if (typeof window === 'undefined') return null;
  const hash = window.location.hash.replace(/^#/, '');
  if (!hash) return null;
  const params = new URLSearchParams(hash);
  const encoded = params.get(HASH_KEY);
  return encoded ? decodeMachine(encoded) : null;
}

/** Builds a shareable permalink that round-trips through this site. */
export function buildPermalink(machine: MachineConfig): string {
  if (typeof window === 'undefined') {
    return `https://klarlabs-studio.github.io/statekit/play#${HASH_KEY}=${encodeMachine(machine)}`;
  }
  const url = new URL(window.location.href);
  url.hash = `${HASH_KEY}=${encodeMachine(machine)}`;
  return url.toString();
}

/**
 * Render a Statekit Native machine config as a Mermaid stateDiagram-v2
 * snippet. Mirrors the Go viz/mermaid package format on the basics —
 * states, transitions, finals.
 */
export function machineToMermaid(machine: MachineConfig): string {
  const lines = ['stateDiagram-v2'];
  if (machine.initial) {
    lines.push(`    [*] --> ${machine.initial}`);
  }
  for (const [id, state] of Object.entries(machine.states)) {
    if (state.transitions) {
      for (const t of state.transitions) {
        if (!t.target) continue;
        const label = t.event || (t as { isDelayed?: boolean }).isDelayed ? `${t.event ?? 'after'}${t.guard ? ` [${t.guard}]` : ''}` : '';
        lines.push(`    ${id} --> ${t.target}${label ? ` : ${label}` : ''}`);
      }
    }
    if (state.type === 'final') {
      lines.push(`    ${id} --> [*]`);
    }
  }
  return lines.join('\n');
}

/**
 * Synthesize a Go builder snippet equivalent to the given machine.
 * Output uses statekit.NewMachine[struct{}] and skips actions/guards
 * since those don't round-trip through Native JSON values — but the
 * topology, hierarchy, finals, and delayed transitions translate.
 */
export function machineToGoBuilder(machine: MachineConfig): string {
  const lines: string[] = [
    `// Generated from the Statekit visualizer. Drop this into a Go file`,
    `// and wire actions/guards via .WithAction()/.WithGuard().`,
    ``,
    `machine, err := statekit.NewMachine[struct{}](${q(machine.id)}).`,
    `    WithInitial(${q(machine.initial)}).`,
  ];
  const roots = Object.entries(machine.states).filter(([, s]) => !s.parent);
  for (const [id, state] of roots) {
    appendState(lines, machine, id, state, '    ');
  }
  // Trim trailing dot of last state, append Build.
  const last = lines[lines.length - 1];
  if (last.endsWith('.')) {
    lines[lines.length - 1] = last.slice(0, -1);
  }
  lines.push(`    Build()`);
  lines.push(``);
  lines.push(`if err != nil {`);
  lines.push(`    panic(err)`);
  lines.push(`}`);
  return lines.join('\n');
}

function appendState(
  lines: string[],
  machine: MachineConfig,
  id: string,
  state: MachineConfig['states'][string],
  indent: string,
) {
  lines.push(`${indent}State(${q(id)}).`);
  if (state.type === 'parallel') {
    lines.push(`${indent}    Parallel().`);
  }
  if (state.type === 'final') {
    lines.push(`${indent}    Final().`);
  }
  if (state.initial) {
    lines.push(`${indent}    WithInitial(${q(state.initial)}).`);
  }
  if (state.transitions) {
    for (const t of state.transitions) {
      if (!t.target) continue;
      const after = (t as { isDelayed?: boolean; delayMs?: number }).isDelayed;
      if (after) {
        const ms = (t as { delayMs?: number }).delayMs ?? 0;
        lines.push(`${indent}    After(${ms} * time.Millisecond).Target(${q(t.target)}).`);
      } else {
        lines.push(`${indent}    On(${q(t.event ?? '')}).Target(${q(t.target)}).`);
      }
    }
  }
  // Recurse into children
  if (state.children) {
    for (const childId of state.children) {
      const child = machine.states[childId];
      if (child) appendState(lines, machine, childId, child, `${indent}    `);
    }
  }
  lines.push(`${indent}Done().`);
}

function q(s: string): string {
  return `"${s.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}
