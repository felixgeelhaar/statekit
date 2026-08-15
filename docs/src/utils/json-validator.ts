import type { MachineConfig } from './types';

export interface ValidationResult {
  valid: boolean;
  errors: string[];
}

export function validateMachineJSON(data: unknown): ValidationResult {
  const errors: string[] = [];

  if (!data || typeof data !== 'object') {
    return { valid: false, errors: ['Input must be an object'] };
  }

  const machine = data as Record<string, unknown>;

  // Check required fields
  if (!machine.id || typeof machine.id !== 'string') {
    errors.push('Missing or invalid "id" field (must be a string)');
  }

  if (!machine.initial || typeof machine.initial !== 'string') {
    errors.push('Missing or invalid "initial" field (must be a string)');
  }

  if (!machine.states || typeof machine.states !== 'object') {
    errors.push('Missing or invalid "states" field (must be an object)');
    return { valid: false, errors };
  }

  const states = machine.states as Record<string, unknown>;

  // Check initial state exists
  if (machine.initial && !states[machine.initial as string]) {
    errors.push(`Initial state "${machine.initial}" not found in states`);
  }

  // Validate each state
  for (const [stateId, stateConfig] of Object.entries(states)) {
    if (!stateConfig || typeof stateConfig !== 'object') {
      errors.push(`State "${stateId}" must be an object`);
      continue;
    }

    const state = stateConfig as Record<string, unknown>;

    // Validate type if present
    if (state.type && !['atomic', 'compound', 'final', 'parallel', 'history'].includes(state.type as string)) {
      errors.push(`State "${stateId}" has invalid type "${state.type}"`);
    }

    // Validate transitions if present
    if (state.transitions) {
      if (!Array.isArray(state.transitions)) {
        errors.push(`State "${stateId}" transitions must be an array`);
      } else {
        for (const transition of state.transitions) {
          if (!transition || typeof transition !== 'object') {
            errors.push(`State "${stateId}" has invalid transition`);
            continue;
          }

          const t = transition as Record<string, unknown>;
          if (t.target && typeof t.target === 'string' && !states[t.target]) {
            errors.push(`State "${stateId}" has transition to unknown state "${t.target}"`);
          }
        }
      }
    }

    // Validate parent if present
    if (state.parent && typeof state.parent === 'string' && !states[state.parent]) {
      errors.push(`State "${stateId}" has unknown parent "${state.parent}"`);
    }

    // Validate children if present
    if (state.children && Array.isArray(state.children)) {
      for (const child of state.children) {
        if (typeof child === 'string' && !states[child]) {
          errors.push(`State "${stateId}" has unknown child "${child}"`);
        }
      }
    }

    // children[] ↔ parent consistency
    if (state.children && Array.isArray(state.children)) {
      for (const child of state.children) {
        if (typeof child !== 'string') continue;
        const childCfg = states[child];
        if (!childCfg || typeof childCfg !== 'object') continue;
        const childParent = (childCfg as Record<string, unknown>).parent;
        if (childParent !== undefined && childParent !== stateId) {
          errors.push(
            `State "${stateId}" lists child "${child}" but that state's parent is "${String(childParent)}"`,
          );
        }
      }
    }

    if (state.parent && typeof state.parent === 'string') {
      const parentCfg = states[state.parent];
      if (parentCfg && typeof parentCfg === 'object') {
        const siblings = (parentCfg as Record<string, unknown>).children;
        if (Array.isArray(siblings) && !siblings.includes(stateId)) {
          errors.push(
            `State "${stateId}" claims parent "${state.parent}" but is not listed in that state's children`,
          );
        }
      }
    }

    // Nested initial must exist when declared
    if (state.initial && typeof state.initial === 'string' && !states[state.initial]) {
      errors.push(`State "${stateId}" initial "${state.initial}" not found in states`);
    }
  }

  // Cyclic initial chains (machine + nested compound initials)
  if (typeof machine.initial === 'string') {
    const cycle = findInitialCycle(states, machine.initial);
    if (cycle) {
      errors.push(`Cyclic initial chain detected: ${cycle.join(' → ')}`);
    }
  }

  return { valid: errors.length === 0, errors };
}

/** Walk initial pointers from `start`; return the cycle path if one exists. */
function findInitialCycle(
  states: Record<string, unknown>,
  start: string,
): string[] | null {
  const seen = new Map<string, number>();
  const path: string[] = [];
  let current: string | undefined = start;

  while (current) {
    if (seen.has(current)) {
      const idx = seen.get(current)!;
      return [...path.slice(idx), current];
    }
    seen.set(current, path.length);
    path.push(current);

    const cfg = states[current];
    if (!cfg || typeof cfg !== 'object') break;
    const next = (cfg as Record<string, unknown>).initial;
    if (typeof next !== 'string' || next === '') break;
    current = next;
  }

  return null;
}

export function parseMachineJSON(json: string): MachineConfig {
  const data = JSON.parse(json);
  const result = validateMachineJSON(data);

  if (!result.valid) {
    throw new Error(result.errors.join('; '));
  }

  return data as MachineConfig;
}
