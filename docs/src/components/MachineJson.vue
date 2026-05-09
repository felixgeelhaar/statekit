<template>
  <div class="panel" v-if="machine">
    <div class="panel-header" @click="expanded = !expanded" style="cursor: pointer;">
      <h3 class="panel-title">Machine JSON</h3>
      <button class="btn btn-icon btn-sm" :title="expanded ? 'Collapse' : 'Expand'">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline v-if="expanded" points="18 15 12 9 6 15"/>
          <polyline v-else points="6 9 12 15 18 9"/>
        </svg>
      </button>
    </div>
    <div class="panel-content json-viewer-content" v-show="expanded">
      <pre class="json-viewer">{{ formattedJson }}</pre>
      <button class="btn btn-sm" @click="copyJson" style="width: 100%; margin-top: 0.5rem;">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
        </svg>
        {{ copied ? 'Copied!' : 'Copy JSON' }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { MachineConfig } from '../utils/types';

const props = defineProps<{
  machine: MachineConfig | null;
}>();

const expanded = ref(false);
const copied = ref(false);

const formattedJson = computed(() => {
  if (!props.machine) return '';
  return JSON.stringify(props.machine, null, 2);
});

async function copyJson() {
  if (!props.machine) return;
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(formattedJson.value);
    } else {
      // Older browsers / insecure context: fall back to a temporary
      // textarea + execCommand. Last resort, but better than a
      // silent failure.
      const ta = document.createElement('textarea');
      ta.value = formattedJson.value;
      ta.setAttribute('readonly', '');
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    }
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch (err) {
    console.warn('statekit: clipboard copy failed', err);
  }
}
</script>

<style scoped>
.json-viewer-content {
  max-height: 300px;
  overflow-y: auto;
}

.json-viewer {
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.65rem;
  line-height: 1.5;
  color: var(--text-secondary);
  background: var(--bg-secondary);
  padding: 0.75rem;
  border-radius: 4px;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
