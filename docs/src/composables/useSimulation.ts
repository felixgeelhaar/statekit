import { ref, type Ref } from 'vue';
import type { MachineConfig, HistoryItem, StateConfig, Transition } from '../utils/types';

/**
 * Composable for simulating a state machine in-browser.
 *
 * Mirrors the Go interpreter on the basics: hierarchical event
 * bubbling, initial-leaf resolution, final-state detection. Diverges
 * intentionally — this runtime ignores guards, actions, parallel
 * regions, history, and delayed transitions because the visualizer
 * cannot execute Go code. The viz UI displays a "preview" badge for
 * exactly this reason.
 */
export function useSimulation(machine: Ref<MachineConfig | null>) {
  const currentState = ref<string | null>(null);
  const history = ref<HistoryItem[]>([]);
  const isSimulating = ref(false);

  /**
   * Resolve a state to its initial leaf, recursing into compound
   * children. Caps depth via a visited set so malformed JSON with
   * cyclic `initial` chains does not infinite-loop.
   */
  function resolveInitialLeaf(start: string): string {
    if (!machine.value) return start;
    let current = start;
    const seen = new Set<string>();
    while (machine.value.states[current]?.initial) {
      if (seen.has(current)) {
        console.warn('statekit: cycle detected in initial chain at', current);
        break;
      }
      seen.add(current);
      current = machine.value.states[current].initial!;
    }
    return current;
  }

  function start() {
    if (!machine.value) return;
    isSimulating.value = true;
    currentState.value = resolveInitialLeaf(machine.value.initial);
    history.value = [{ event: 'START', to: currentState.value }];
  }

  function reset() {
    isSimulating.value = false;
    currentState.value = null;
    history.value = [];
  }

  /**
   * Send an event. Walks parents to bubble unhandled events up the
   * hierarchy. Returns the resolved final state (or null if no
   * transition fired) — useful so the caller can announce a
   * final-state toast without re-walking.
   */
  function send(eventType: string): string | null {
    if (!isSimulating.value || !currentState.value || !machine.value) return null;

    let transition: Transition | undefined;
    const visited = new Set<string>();
    let searchState: string | undefined = currentState.value;

    while (searchState && !transition && !visited.has(searchState)) {
      visited.add(searchState);
      const s: StateConfig | undefined = machine.value.states[searchState];
      if (s?.transitions) {
        transition = s.transitions.find((t: Transition) => t.event === eventType);
      }
      searchState = s?.parent;
    }

    if (!transition?.target) return null;

    const prevState = currentState.value;
    currentState.value = resolveInitialLeaf(transition.target);
    history.value.unshift({
      event: eventType,
      from: prevState,
      to: currentState.value,
    });
    return currentState.value;
  }

  /**
   * Inspect the current state and its ancestors and return all event
   * types that would fire from the current configuration.
   */
  function availableEvents(): string[] {
    if (!isSimulating.value || !currentState.value || !machine.value) return [];

    const events = new Set<string>();
    const visited = new Set<string>();
    let searchState: string | undefined = currentState.value;

    while (searchState && !visited.has(searchState)) {
      visited.add(searchState);
      const state: StateConfig | undefined = machine.value.states[searchState];
      if (state?.transitions) {
        state.transitions.forEach((t: Transition) => {
          if (t.event) events.add(t.event);
        });
      }
      searchState = state?.parent;
    }
    return Array.from(events).sort();
  }

  return {
    currentState,
    history,
    isSimulating,
    start,
    reset,
    send,
    availableEvents,
    resolveInitialLeaf,
  };
}
