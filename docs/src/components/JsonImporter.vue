<template>
  <div class="panel">
    <div class="panel-header">
      <h3 class="panel-title">Import Machine</h3>
    </div>
    <div class="panel-content">
      <!-- File Upload -->
      <div
        class="file-upload"
        :class="{ dragging: isDragging }"
        @dragover.prevent="isDragging = true"
        @dragleave="isDragging = false"
        @drop.prevent="handleDrop"
        @click="fileInputRef?.click()"
      >
        <svg class="upload-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
          <polyline points="17 8 12 3 7 8"/>
          <line x1="12" y1="3" x2="12" y2="15"/>
        </svg>
        <span>Drop JSON file or click to browse</span>
        <input
          ref="fileInputRef"
          type="file"
          accept=".json"
          @change="handleFileSelect"
          style="display: none"
        />
      </div>

      <!-- JSON Textarea -->
      <textarea
        v-model="jsonInput"
        class="json-input"
        placeholder="Or paste Statekit JSON here..."
        spellcheck="false"
      ></textarea>

      <!-- Actions -->
      <div class="import-actions">
        <button class="btn btn-primary" @click="importJson" :disabled="!jsonInput.trim()">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="16 16 12 12 8 16"/>
            <line x1="12" y1="12" x2="12" y2="21"/>
            <path d="M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3"/>
          </svg>
          Import
        </button>
        <button class="btn btn-secondary" @click="loadSample">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
            <line x1="3" y1="9" x2="21" y2="9"/>
            <line x1="9" y1="21" x2="9" y2="9"/>
          </svg>
          Load Sample
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { MachineConfig } from '../utils/types';
import { parseMachineJSON } from '../utils/json-validator';

const emit = defineEmits<{
  load: [machine: MachineConfig];
  error: [message: string];
}>();

const jsonInput = ref('');
const isDragging = ref(false);
const fileInputRef = ref<HTMLInputElement | null>(null);

function handleDrop(e: DragEvent) {
  isDragging.value = false;
  const file = e.dataTransfer?.files[0];
  if (file && file.type === 'application/json') {
    readFile(file);
  }
}

function handleFileSelect(e: Event) {
  const target = e.target as HTMLInputElement;
  const file = target.files?.[0];
  if (file) {
    readFile(file);
  }
}

function readFile(file: File) {
  const reader = new FileReader();
  reader.onload = (e) => {
    jsonInput.value = e.target?.result as string;
    importJson();
  };
  reader.onerror = () => {
    emit('error', 'Failed to read file');
  };
  reader.readAsText(file);
}

function importJson() {
  if (!jsonInput.value.trim()) return;

  try {
    const machine = parseMachineJSON(jsonInput.value);
    emit('load', machine);
  } catch (err) {
    emit('error', err instanceof Error ? err.message : 'Invalid JSON format');
  }
}

function loadSample() {
  const sample: MachineConfig = {
    id: 'trafficLight',
    initial: 'green',
    states: {
      green: {
        type: 'atomic',
        transitions: [{ event: 'TIMER', target: 'yellow' }]
      },
      yellow: {
        type: 'atomic',
        transitions: [{ event: 'TIMER', target: 'red' }]
      },
      red: {
        type: 'compound',
        initial: 'walk',
        children: ['walk', 'wait', 'stop'],
        transitions: [{ event: 'TIMER', target: 'green' }]
      },
      walk: {
        type: 'atomic',
        parent: 'red',
        transitions: [{ event: 'PED_TIMER', target: 'wait' }]
      },
      wait: {
        type: 'atomic',
        parent: 'red',
        transitions: [{ event: 'PED_TIMER', target: 'stop' }]
      },
      stop: {
        type: 'atomic',
        parent: 'red'
      }
    }
  };

  jsonInput.value = JSON.stringify(sample, null, 2);
  emit('load', sample);
}
</script>
