<template>
  <div class="panel">
    <div class="panel-header">
      <h3 class="panel-title">History</h3>
      <span v-if="history.length > 0" class="history-count">{{ history.length }}</span>
    </div>
    <div class="panel-content">
      <div v-if="history.length > 0" class="history-list">
        <div
          v-for="(item, index) in history"
          :key="index"
          class="history-item"
        >
          <span class="history-event">{{ item.event }}</span>
          <div class="history-transition">
            <span v-if="item.from" class="history-from">{{ item.from }}</span>
            <svg v-if="item.from" class="history-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="5" y1="12" x2="19" y2="12"/>
              <polyline points="12 5 19 12 12 19"/>
            </svg>
            <span class="history-to">{{ item.to }}</span>
          </div>
        </div>
      </div>
      <div v-else class="empty-history">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M12 8v4l3 3"/>
          <circle cx="12" cy="12" r="10"/>
        </svg>
        <p>No transitions yet</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { HistoryItem } from '../utils/types';

defineProps<{
  history: HistoryItem[];
}>();
</script>

<style scoped>
.history-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.25rem;
  height: 1.25rem;
  padding: 0 0.375rem;
  background: var(--accent);
  border-radius: 9999px;
  font-size: 0.6875rem;
  font-weight: 600;
  color: var(--bg);
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  max-height: 200px;
  overflow-y: auto;
}

.history-item {
  padding: 0.5rem 0.75rem;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}

.history-event {
  display: block;
  font-family: var(--font-mono);
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--accent);
  margin-bottom: 0.25rem;
}

.history-transition {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-family: var(--font-mono);
  font-size: 0.6875rem;
  color: var(--text-muted);
}

.history-arrow {
  width: 1rem;
  height: 1rem;
  color: var(--text-muted);
  opacity: 0.5;
}

.history-from {
  color: var(--text-muted);
}

.history-to {
  color: var(--state-active);
}

.empty-history {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
  color: var(--text-muted);
  text-align: center;
}

.empty-history svg {
  width: 2rem;
  height: 2rem;
  margin-bottom: 0.5rem;
  opacity: 0.5;
}

.empty-history p {
  font-size: 0.8125rem;
}
</style>
