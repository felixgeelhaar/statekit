<template>
  <div class="panel">
    <div class="panel-header">
      <h3 class="panel-title">Simulation</h3>
      <span v-if="isSimulating" class="badge badge-active">
        <span class="badge-dot"></span>
        Active
      </span>
    </div>
    <div class="panel-content">
      <!-- Current State Display -->
      <div v-if="currentState" class="current-state">
        <span class="current-state-label">Current State</span>
        <span class="current-state-value">{{ currentState }}</span>
      </div>

      <!-- Controls -->
      <div class="simulation-controls">
        <button
          v-if="!isSimulating"
          class="btn btn-primary"
          :disabled="!machine"
          @click="$emit('start')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polygon points="5 3 19 12 5 21 5 3"/>
          </svg>
          Start Simulation
        </button>
        <button
          v-else
          class="btn btn-danger"
          @click="$emit('reset')"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
          </svg>
          Reset
        </button>
      </div>

      <!-- Available Events -->
      <div v-if="isSimulating && availableEvents.length > 0" class="events-section">
        <h4 class="events-title">Available Events</h4>
        <div class="events-list">
          <button
            v-for="(event, index) in availableEvents"
            :key="event"
            class="event-btn"
            @click="$emit('send-event', event)"
          >
            <span class="event-key">{{ index + 1 }}</span>
            {{ event }}
          </button>
        </div>
      </div>

      <!-- Empty State -->
      <div v-if="!machine" class="empty-state">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/>
          <path d="M12 6v6l4 2"/>
        </svg>
        <p>Import a machine to start simulation</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { MachineConfig, HistoryItem } from '../utils/types';

const props = defineProps<{
  machine: MachineConfig | null;
  currentState: string | null;
  isSimulating: boolean;
  history: HistoryItem[];
}>();

defineEmits<{
  start: [];
  reset: [];
  'send-event': [event: string];
}>();

const availableEvents = computed(() => {
  if (!props.currentState || !props.machine) return [];

  const events = new Set<string>();
  let searchState: string | undefined = props.currentState;

  while (searchState) {
    const state = props.machine.states[searchState];
    if (state?.transitions) {
      state.transitions.forEach(t => {
        if (t.event) events.add(t.event);
      });
    }
    searchState = state?.parent;
  }

  return Array.from(events).sort();
});
</script>

<style scoped>
.events-section {
  margin-top: 1rem;
}

.events-title {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 0.5rem;
}

.events-list {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.event-btn {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 0.8125rem;
  cursor: pointer;
  transition: all var(--transition);
  text-align: left;
}

.event-btn:hover {
  background: var(--surface-hover);
  border-color: var(--accent);
}

.event-key {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.25rem;
  height: 1.25rem;
  background: var(--border);
  border-radius: 0.25rem;
  font-size: 0.6875rem;
  font-weight: 600;
  color: var(--text-muted);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2rem 1rem;
  color: var(--text-muted);
  text-align: center;
}

.empty-state svg {
  width: 2.5rem;
  height: 2.5rem;
  margin-bottom: 0.75rem;
  opacity: 0.5;
}

.empty-state p {
  font-size: 0.875rem;
}
</style>
