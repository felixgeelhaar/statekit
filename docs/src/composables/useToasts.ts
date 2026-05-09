import { ref } from 'vue';

export interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error';
}

/**
 * Composable for showing transient notifications.
 *
 * Success toasts auto-dismiss after `successDurationMs` (default 3s).
 * Error toasts persist until the user calls `dismiss(id)` so a
 * malformed JSON message doesn't disappear before the user reads it.
 */
export function useToasts(successDurationMs = 3000) {
  const toasts = ref<Toast[]>([]);
  let nextId = 0;

  function show(message: string, type: 'success' | 'error' = 'success'): number {
    const id = ++nextId;
    toasts.value = [...toasts.value, { id, message, type }];
    if (type === 'success') {
      setTimeout(() => dismiss(id), successDurationMs);
    }
    return id;
  }

  function dismiss(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id);
  }

  function clear() {
    toasts.value = [];
  }

  return { toasts, show, dismiss, clear };
}
