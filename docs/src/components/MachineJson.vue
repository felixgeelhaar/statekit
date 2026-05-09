<template>
  <div class="panel" v-if="machine">
    <div class="panel-header" @click="expanded = !expanded" style="cursor: pointer;">
      <h3 class="panel-title">Machine JSON</h3>
      <button
        class="btn btn-icon btn-sm"
        :aria-label="expanded ? 'Collapse panel' : 'Expand panel'"
        :aria-expanded="expanded"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <polyline v-if="expanded" points="18 15 12 9 6 15"/>
          <polyline v-else points="6 9 12 15 18 9"/>
        </svg>
      </button>
    </div>
    <div class="panel-content json-viewer-content" v-show="expanded">
      <pre class="json-viewer">{{ formattedJson }}</pre>
      <div class="share-actions">
        <button class="btn btn-sm" @click="copyJson" :aria-label="copied === 'json' ? 'Copied' : 'Copy JSON'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
          </svg>
          {{ copied === 'json' ? 'Copied!' : 'Copy JSON' }}
        </button>
        <button class="btn btn-sm" @click="copyPermalink" :aria-label="copied === 'link' ? 'Permalink copied' : 'Copy permalink'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
            <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
          </svg>
          {{ copied === 'link' ? 'Copied!' : 'Permalink' }}
        </button>
        <button class="btn btn-sm" @click="copyMermaid" :aria-label="copied === 'mermaid' ? 'Mermaid copied' : 'Copy as Mermaid'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <circle cx="6" cy="6" r="3"/>
            <circle cx="18" cy="6" r="3"/>
            <circle cx="12" cy="18" r="3"/>
            <path d="M9 6h6"/>
            <path d="M7.5 8.5l3 7"/>
            <path d="M16.5 8.5l-3 7"/>
          </svg>
          {{ copied === 'mermaid' ? 'Copied!' : 'Mermaid' }}
        </button>
        <button class="btn btn-sm" @click="copyGo" :aria-label="copied === 'go' ? 'Go builder copied' : 'Copy as Go builder'">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <polyline points="16 18 22 12 16 6"/>
            <polyline points="8 6 2 12 8 18"/>
          </svg>
          {{ copied === 'go' ? 'Copied!' : 'Go' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { MachineConfig } from '../utils/types';
import { buildPermalink, machineToMermaid, machineToGoBuilder } from '../utils/share-url';

const props = defineProps<{
  machine: MachineConfig | null;
}>();

const expanded = ref(false);
type CopyKind = 'json' | 'link' | 'mermaid' | 'go' | null;
const copied = ref<CopyKind>(null);

const formattedJson = computed(() => {
  if (!props.machine) return '';
  return JSON.stringify(props.machine, null, 2);
});

async function copyText(text: string, kind: CopyKind) {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const ta = document.createElement('textarea');
      ta.value = text;
      ta.setAttribute('readonly', '');
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    }
    copied.value = kind;
    setTimeout(() => {
      if (copied.value === kind) copied.value = null;
    }, 2000);
  } catch (err) {
    console.warn('statekit: clipboard copy failed', err);
  }
}

function copyJson() {
  if (!props.machine) return;
  copyText(formattedJson.value, 'json');
}

function copyPermalink() {
  if (!props.machine) return;
  copyText(buildPermalink(props.machine), 'link');
}

function copyMermaid() {
  if (!props.machine) return;
  copyText(machineToMermaid(props.machine), 'mermaid');
}

function copyGo() {
  if (!props.machine) return;
  copyText(machineToGoBuilder(props.machine), 'go');
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

.share-actions {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 0.4rem;
  margin-top: 0.5rem;
}

.share-actions .btn {
  width: 100%;
}
</style>
