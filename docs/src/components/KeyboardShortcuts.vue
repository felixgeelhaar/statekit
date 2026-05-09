<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="modal-overlay"
      role="presentation"
      @click.self="$emit('close')"
    >
      <div
        class="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="shortcuts-modal-title"
        ref="dialogRef"
        tabindex="-1"
        @keydown.esc.stop="$emit('close')"
        @keydown.tab="trapFocus"
      >
        <div class="modal-header">
          <h2 id="shortcuts-modal-title" class="modal-title">Keyboard Shortcuts</h2>
          <button
            class="modal-close"
            aria-label="Close keyboard shortcuts"
            @click="$emit('close')"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>
        <div class="modal-content">
          <div class="shortcuts-grid">
            <div v-for="shortcut in shortcuts" :key="shortcut.key" class="shortcut">
              <kbd class="shortcut-key">{{ shortcut.key }}</kbd>
              <span class="shortcut-desc">{{ shortcut.description }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue';

const props = defineProps<{
  visible: boolean;
}>();

defineEmits<{
  close: [];
}>();

const dialogRef = ref<HTMLElement | null>(null);
let triggerEl: HTMLElement | null = null;

watch(
  () => props.visible,
  async (open) => {
    if (open) {
      triggerEl = document.activeElement as HTMLElement | null;
      await nextTick();
      dialogRef.value?.focus();
    } else if (triggerEl) {
      triggerEl.focus();
      triggerEl = null;
    }
  },
);

function trapFocus(e: KeyboardEvent) {
  const root = dialogRef.value;
  if (!root) return;
  const focusables = root.querySelectorAll<HTMLElement>(
    'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
  );
  if (focusables.length === 0) return;
  const first = focusables[0];
  const last = focusables[focusables.length - 1];
  const active = document.activeElement;
  if (e.shiftKey && active === first) {
    e.preventDefault();
    last.focus();
  } else if (!e.shiftKey && active === last) {
    e.preventDefault();
    first.focus();
  }
}

const shortcuts = [
  { key: '?', description: 'Toggle this help' },
  { key: 'Space', description: 'Start/Reset simulation' },
  { key: '1-9', description: 'Send event (by number)' },
  { key: '+', description: 'Zoom in' },
  { key: '-', description: 'Zoom out' },
  { key: '0', description: 'Reset view' },
  { key: 'Esc', description: 'Close modal' },
];
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.8);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  animation: fadeIn 0.15s ease-out;
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

.modal {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  width: 100%;
  max-width: 400px;
  max-height: 90vh;
  overflow: hidden;
  animation: slideIn 0.2s ease-out;
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: scale(0.95) translateY(-10px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border);
}

.modal-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text);
}

.modal-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  background: transparent;
  border: none;
  border-radius: var(--radius);
  color: var(--text-muted);
  cursor: pointer;
  transition: all var(--transition);
}

.modal-close:hover {
  background: var(--surface-hover);
  color: var(--text);
}

.modal-close svg {
  width: 1.25rem;
  height: 1.25rem;
}

.modal-content {
  padding: 1.25rem;
}

.shortcuts-grid {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.shortcut {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.shortcut-key {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 2.5rem;
  padding: 0.375rem 0.625rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  font-family: var(--font-mono);
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--accent);
}

.shortcut-desc {
  font-size: 0.875rem;
  color: var(--text-muted);
}
</style>
