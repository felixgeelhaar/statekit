import { describe, it, expect } from 'vitest';
import { ref } from 'vue';
import { useSimulation } from './useSimulation';
import type { MachineConfig } from '../utils/types';

const traffic: MachineConfig = {
  id: 'traffic',
  initial: 'green',
  states: {
    green: { type: 'atomic', transitions: [{ event: 'TIMER', target: 'yellow' }] },
    yellow: { type: 'atomic', transitions: [{ event: 'TIMER', target: 'red' }] },
    red: { type: 'atomic', transitions: [{ event: 'TIMER', target: 'green' }] },
  },
};

const hierarchical: MachineConfig = {
  id: 'editor',
  initial: 'active',
  states: {
    active: {
      type: 'compound',
      initial: 'idle',
      children: ['idle', 'dirty'],
      transitions: [{ event: 'SAVE', target: 'saved' }],
    },
    idle: { type: 'atomic', parent: 'active', transitions: [{ event: 'TYPE', target: 'dirty' }] },
    dirty: { type: 'atomic', parent: 'active', transitions: [{ event: 'CLEAR', target: 'idle' }] },
    saved: { type: 'final' },
  },
};

const cyclic: MachineConfig = {
  id: 'cycle',
  initial: 'a',
  states: {
    a: { type: 'compound', initial: 'b' },
    b: { type: 'compound', initial: 'a' },
  },
};

describe('useSimulation', () => {
  it('start enters the initial state', () => {
    const machine = ref(traffic);
    const sim = useSimulation(machine);
    sim.start();
    expect(sim.currentState.value).toBe('green');
    expect(sim.isSimulating.value).toBe(true);
  });

  it('send walks transitions on the current state', () => {
    const machine = ref(traffic);
    const sim = useSimulation(machine);
    sim.start();
    expect(sim.send('TIMER')).toBe('yellow');
    expect(sim.send('TIMER')).toBe('red');
    expect(sim.history.value[0]).toMatchObject({ event: 'TIMER', from: 'yellow', to: 'red' });
  });

  it('send returns null when no matching transition fires', () => {
    const machine = ref(traffic);
    const sim = useSimulation(machine);
    sim.start();
    expect(sim.send('UNKNOWN')).toBeNull();
    expect(sim.currentState.value).toBe('green');
  });

  it('start resolves to the initial leaf of a compound', () => {
    const machine = ref(hierarchical);
    const sim = useSimulation(machine);
    sim.start();
    expect(sim.currentState.value).toBe('idle');
  });

  it('send bubbles up to ancestors when the leaf does not handle the event', () => {
    const machine = ref(hierarchical);
    const sim = useSimulation(machine);
    sim.start();
    expect(sim.send('SAVE')).toBe('saved');
  });

  it('availableEvents enumerates leaf + ancestor events', () => {
    const machine = ref(hierarchical);
    const sim = useSimulation(machine);
    sim.start();
    expect(sim.availableEvents()).toEqual(['SAVE', 'TYPE']);
  });

  it('reset clears state', () => {
    const machine = ref(traffic);
    const sim = useSimulation(machine);
    sim.start();
    sim.reset();
    expect(sim.currentState.value).toBeNull();
    expect(sim.isSimulating.value).toBe(false);
    expect(sim.history.value).toHaveLength(0);
  });

  it('resolveInitialLeaf does not infinite-loop on cyclic chains', () => {
    const machine = ref(cyclic);
    const sim = useSimulation(machine);
    expect(() => sim.start()).not.toThrow();
  });

  it('send with no machine is a no-op', () => {
    const machine = ref<MachineConfig | null>(null);
    const sim = useSimulation(machine);
    sim.start();
    expect(sim.send('TIMER')).toBeNull();
  });
});
