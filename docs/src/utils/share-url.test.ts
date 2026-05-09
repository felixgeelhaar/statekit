import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  encodeMachine,
  decodeMachine,
  readMachineFromHash,
  buildPermalink,
  machineToMermaid,
  machineToGoBuilder,
} from './share-url';
import type { MachineConfig } from './types';

const sample: MachineConfig = {
  id: 'order',
  initial: 'cart',
  states: {
    cart: {
      type: 'atomic',
      transitions: [{ event: 'CHECKOUT', target: 'paid' }],
    },
    paid: {
      type: 'final',
    },
  },
};

describe('encode/decode', () => {
  it('round-trips a machine through base64url', () => {
    const encoded = encodeMachine(sample);
    expect(typeof encoded).toBe('string');
    expect(encoded).not.toContain('+');
    expect(encoded).not.toContain('/');
    expect(encoded).not.toContain('=');

    const decoded = decodeMachine(encoded);
    expect(decoded).toEqual(sample);
  });

  it('decodeMachine returns null for empty input', () => {
    expect(decodeMachine('')).toBeNull();
  });

  it('decodeMachine returns null for malformed base64', () => {
    expect(decodeMachine('!@#$%^&*()')).toBeNull();
  });
});

describe('readMachineFromHash', () => {
  beforeEach(() => {
    window.location.hash = '';
  });

  it('returns null when hash is empty', () => {
    expect(readMachineFromHash()).toBeNull();
  });

  it('returns null when hash has no m= parameter', () => {
    window.location.hash = 'foo=bar';
    expect(readMachineFromHash()).toBeNull();
  });

  it('decodes a machine when m= is present', () => {
    window.location.hash = `m=${encodeMachine(sample)}`;
    expect(readMachineFromHash()).toEqual(sample);
  });
});

describe('buildPermalink', () => {
  it('embeds the encoded machine in the URL hash under m=', () => {
    const url = buildPermalink(sample);
    expect(url).toContain('#m=');
    const encoded = url.split('#m=')[1];
    expect(decodeMachine(encoded)).toEqual(sample);
  });
});

describe('machineToMermaid', () => {
  it('emits stateDiagram-v2 with initial pseudo-arrow', () => {
    const out = machineToMermaid(sample);
    expect(out.startsWith('stateDiagram-v2')).toBe(true);
    expect(out).toContain('[*] --> cart');
    expect(out).toContain('cart --> paid : CHECKOUT');
  });

  it('emits final pseudo-arrow for final states', () => {
    const out = machineToMermaid(sample);
    expect(out).toContain('paid --> [*]');
  });
});

describe('machineToGoBuilder', () => {
  it('emits a NewMachine[struct{}] header with the right id', () => {
    const out = machineToGoBuilder(sample);
    expect(out).toContain('statekit.NewMachine[struct{}]("order")');
  });

  it('emits WithInitial for the initial state', () => {
    const out = machineToGoBuilder(sample);
    expect(out).toContain('WithInitial("cart")');
  });

  it('emits On.Target chains for transitions', () => {
    const out = machineToGoBuilder(sample);
    expect(out).toContain('On("CHECKOUT").Target("paid")');
  });

  it('emits Final() for final states', () => {
    const out = machineToGoBuilder(sample);
    expect(out).toContain('Final()');
  });

  it('terminates with Build()', () => {
    const out = machineToGoBuilder(sample);
    expect(out).toContain('Build()');
  });
});

describe('compatibility shim', () => {
  it('falls back when localStorage is throwing', () => {
    // The share-url module shouldn't depend on localStorage directly.
    // This test pins that contract — adding such a dep would break
    // the auto-load-sample flow we wired up.
    const spy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked');
    });
    expect(() => encodeMachine(sample)).not.toThrow();
    expect(() => decodeMachine(encodeMachine(sample))).not.toThrow();
    spy.mockRestore();
  });
});
