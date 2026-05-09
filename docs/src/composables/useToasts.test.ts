import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { useToasts } from './useToasts';

describe('useToasts', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('show enqueues a toast and returns its id', () => {
    const { toasts, show } = useToasts();
    const id = show('hi');
    expect(toasts.value).toEqual([{ id, message: 'hi', type: 'success' }]);
  });

  it('success toasts auto-dismiss after the configured duration', () => {
    const { toasts, show } = useToasts(1000);
    show('temporary');
    expect(toasts.value).toHaveLength(1);
    vi.advanceTimersByTime(1001);
    expect(toasts.value).toHaveLength(0);
  });

  it('error toasts persist until dismissed', () => {
    const { toasts, show, dismiss } = useToasts(1000);
    const id = show('boom', 'error');
    vi.advanceTimersByTime(60_000);
    expect(toasts.value).toHaveLength(1);
    dismiss(id);
    expect(toasts.value).toHaveLength(0);
  });

  it('clear empties all toasts at once', () => {
    const { toasts, show, clear } = useToasts();
    show('a');
    show('b', 'error');
    expect(toasts.value).toHaveLength(2);
    clear();
    expect(toasts.value).toHaveLength(0);
  });

  it('dismiss is a no-op for unknown ids', () => {
    const { toasts, show, dismiss } = useToasts();
    show('keep me');
    dismiss(99_999);
    expect(toasts.value).toHaveLength(1);
  });
});
