import { describe, it, expect } from 'vitest';
import { validateMachineJSON, parseMachineJSON } from './json-validator';

describe('validateMachineJSON', () => {
  it('accepts a minimal valid machine', () => {
    const result = validateMachineJSON({
      id: 'order',
      initial: 'cart',
      states: {
        cart: { type: 'atomic', transitions: [{ event: 'PAY', target: 'paid' }] },
        paid: { type: 'final' },
      },
    });
    expect(result.valid).toBe(true);
    expect(result.errors).toEqual([]);
  });

  it('rejects cyclic initial chains', () => {
    const result = validateMachineJSON({
      id: 'cycle',
      initial: 'a',
      states: {
        a: { type: 'compound', initial: 'b' },
        b: { type: 'compound', initial: 'a' },
      },
    });
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes('Cyclic initial'))).toBe(true);
  });

  it('rejects children/parent mismatch', () => {
    const result = validateMachineJSON({
      id: 'bad',
      initial: 'root',
      states: {
        root: { type: 'compound', initial: 'child', children: ['child'] },
        child: { type: 'atomic', parent: 'other' },
        other: { type: 'atomic' },
      },
    });
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes('lists child'))).toBe(true);
  });

  it('rejects parent claim missing from children[]', () => {
    const result = validateMachineJSON({
      id: 'bad',
      initial: 'root',
      states: {
        root: { type: 'compound', initial: 'child', children: [] },
        child: { type: 'atomic', parent: 'root' },
      },
    });
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes('not listed in that state\'s children'))).toBe(true);
  });

  it('parseMachineJSON throws on invalid input', () => {
    expect(() => parseMachineJSON('{"id":"x"}')).toThrow();
  });
});
