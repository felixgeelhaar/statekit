<template>
  <div class="visualizer-content">
    <!-- Sidebar -->
    <aside class="sidebar">
      <JsonImporter
        @load="handleMachineLoad"
        @error="showToast($event, 'error')"
      />

      <SimulationPanel
        :machine="machine"
        :current-state="currentState"
        :is-simulating="isSimulating"
        :history="history"
        @start="startSimulation"
        @reset="resetSimulation"
        @send-event="sendEvent"
      />

      <StateHistory :history="history" />

      <MachineJson :machine="machine" />
    </aside>

    <!-- Canvas Area -->
    <main class="canvas-area">
      <StateCanvas
        ref="canvasRef"
        :machine="machine"
        :current-state="currentState"
        :state-positions="statePositions"
        @state-hover="handleStateHover"
      />

      <!-- Toolbar -->
      <div v-if="machine" class="toolbar">
        <button class="btn btn-icon" @click="zoomIn" title="Zoom In">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8"/>
            <path d="M21 21l-4.35-4.35"/>
            <path d="M11 8v6M8 11h6"/>
          </svg>
        </button>
        <button class="btn btn-icon" @click="zoomOut" title="Zoom Out">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8"/>
            <path d="M21 21l-4.35-4.35"/>
            <path d="M8 11h6"/>
          </svg>
        </button>
        <button class="btn btn-icon" @click="resetView" title="Reset View">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M15 3h6v6"/>
            <path d="M9 21H3v-6"/>
            <path d="M21 3l-7 7"/>
            <path d="M3 21l7-7"/>
          </svg>
        </button>
      </div>

      <!-- Legend -->
      <div v-if="machine" class="legend">
        <div class="legend-item">
          <div class="legend-dot atomic"></div>
          <span>Atomic</span>
        </div>
        <div class="legend-item">
          <div class="legend-dot compound"></div>
          <span>Compound</span>
        </div>
        <div class="legend-item">
          <div class="legend-dot final"></div>
          <span>Final</span>
        </div>
        <div class="legend-item">
          <div class="legend-dot parallel"></div>
          <span>Parallel</span>
        </div>
        <div class="legend-item">
          <div class="legend-dot history"></div>
          <span>History</span>
        </div>
      </div>

      <!-- State Tooltip -->
      <div
        class="state-tooltip"
        :class="{ visible: hoveredState }"
        :style="tooltipStyle"
      >
        <div class="state-tooltip-name">{{ hoveredState?.id }}</div>
        <div class="state-tooltip-type">{{ hoveredState?.type || 'atomic' }}</div>
      </div>
    </main>
  </div>

  <!-- Keyboard Shortcuts Modal -->
  <KeyboardShortcuts
    @close="shortcutsVisible = false"
    :visible="shortcutsVisible"
  />

  <!-- Toast Container -->
  <div class="toast-container" role="status" aria-live="polite" aria-atomic="true">
    <div
      v-for="toast in toasts"
      :key="toast.id"
      class="toast"
      :class="toast.type"
      :role="toast.type === 'error' ? 'alert' : undefined"
    >
      <svg class="toast-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
        <template v-if="toast.type === 'success'">
          <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
          <polyline points="22 4 12 14.01 9 11.01"/>
        </template>
        <template v-else>
          <circle cx="12" cy="12" r="10"/>
          <path d="M15 9l-6 6"/>
          <path d="M9 9l6 6"/>
        </template>
      </svg>
      <span class="toast-message">{{ toast.message }}</span>
      <button
        v-if="toast.type === 'error'"
        class="toast-dismiss"
        aria-label="Dismiss error"
        @click="dismissToast(toast.id)"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <line x1="18" y1="6" x2="6" y2="18"/>
          <line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, onErrorCaptured } from 'vue';
import type { MachineConfig, StatePosition } from '../utils/types';
import JsonImporter from './JsonImporter.vue';
import SimulationPanel from './SimulationPanel.vue';
import StateHistory from './StateHistory.vue';
import StateCanvas from './StateCanvas.vue';
import KeyboardShortcuts from './KeyboardShortcuts.vue';
import MachineJson from './MachineJson.vue';
import { useToasts } from '../composables/useToasts';
import { useSimulation } from '../composables/useSimulation';

// State
const machine = ref<MachineConfig | null>(null);
const statePositions = ref<Record<string, StatePosition>>({});
const shortcutsVisible = ref(false);

// Canvas refs
const canvasRef = ref<InstanceType<typeof StateCanvas> | null>(null);

// Tooltip
const hoveredState = ref<{ id: string; type: string } | null>(null);
const tooltipStyle = ref({ left: '0px', top: '0px' });

// Toasts (extracted to composable)
const { toasts, show: showToast, dismiss: dismissToast } = useToasts();

// Simulation (extracted to composable)
const sim = useSimulation(machine);
const currentState = sim.currentState;
const history = sim.history;
const isSimulating = sim.isSimulating;

// Catch descendant Vue errors so a malformed render doesn't blank
// the page; surface them as an error toast instead.
onErrorCaptured((err) => {
  console.error('statekit: visualizer error', err);
  showToast(
    err instanceof Error ? err.message : 'Something went wrong rendering the machine',
    'error',
  );
  return false; // suppress further propagation
});

// Machine loading
function handleMachineLoad(loadedMachine: MachineConfig) {
  machine.value = loadedMachine;
  calculatePositions();
  resetSimulation();
  showToast('Machine loaded successfully', 'success');
}

// Position calculation
function calculatePositions() {
  if (!machine.value || !canvasRef.value) return;

  const states = Object.keys(machine.value.states);
  const width = 700;
  const height = 500;
  const paddingLeft = 80;
  const paddingTop = 80;
  const paddingRight = 80;
  const paddingBottom = 80;

  // Child state dimensions
  const childStateWidth = 90;
  const childStateHeight = 32;
  const childGap = 50;  // Space between children for arrow labels
  const compoundPadding = 20;
  const compoundLabelHeight = 40;  // Space for compound state label

  // Group states by parent
  const groups: Record<string, string[]> = {};
  const rootStates: string[] = [];

  states.forEach(id => {
    const state = machine.value!.states[id];
    if (state.parent) {
      if (!groups[state.parent]) groups[state.parent] = [];
      groups[state.parent].push(id);
    } else {
      rootStates.push(id);
    }
  });

  // Calculate compound state sizes based on children
  function getCompoundSize(childCount: number): { width: number; height: number } {
    // Arrange children in a single column with space for arrow labels between them
    const innerWidth = childStateWidth + compoundPadding * 2;
    const innerHeight = compoundLabelHeight + childCount * childStateHeight + (childCount - 1) * childGap + compoundPadding * 2;
    return { width: innerWidth, height: innerHeight };
  }

  // Position root states in a grid
  const cols = Math.ceil(Math.sqrt(rootStates.length));
  const rows = Math.ceil(rootStates.length / cols);
  const availableWidth = width - paddingLeft - paddingRight;
  const availableHeight = height - paddingTop - paddingBottom;
  const cellWidth = availableWidth / cols;
  const cellHeight = availableHeight / rows;

  const positions: Record<string, StatePosition> = {};

  rootStates.forEach((id, i) => {
    const row = Math.floor(i / cols);
    const col = i % cols;
    const state = machine.value!.states[id];
    const childCount = groups[id]?.length || 0;

    // Calculate size based on whether it has children
    let stateWidth = 140;
    let stateHeight = 60;

    if (childCount > 0) {
      const size = getCompoundSize(childCount);
      stateWidth = size.width;
      stateHeight = size.height;
    }

    positions[id] = {
      x: paddingLeft + col * cellWidth + cellWidth / 2,
      y: paddingTop + row * cellHeight + cellHeight / 2,
      width: stateWidth,
      height: stateHeight
    };

    // Position children inside parent (single column layout with gaps for labels)
    if (groups[id]) {
      const children = groups[id];
      const parentPos = positions[id];
      const startY = parentPos.y - parentPos.height/2 + compoundLabelHeight + compoundPadding;

      children.forEach((childId, j) => {
        positions[childId] = {
          x: parentPos.x,
          y: startY + j * (childStateHeight + childGap) + childStateHeight / 2,
          width: childStateWidth,
          height: childStateHeight
        };
      });
    }
  });

  statePositions.value = positions;
}

// Simulation thin wrappers — the heavy lifting lives in
// composables/useSimulation. These exist so the component can also
// announce final-state toasts when an event reaches a terminal node.

function startSimulation() {
  sim.start();
}

function resetSimulation() {
  sim.reset();
}

function sendEvent(eventType: string) {
  const next = sim.send(eventType);
  if (next && machine.value?.states[next]?.type === 'final') {
    showToast('Reached final state: ' + next, 'success');
  }
}

// Canvas controls
function zoomIn() {
  canvasRef.value?.zoomIn();
}

function zoomOut() {
  canvasRef.value?.zoomOut();
}

function resetView() {
  canvasRef.value?.resetView();
}

// Tooltip
function handleStateHover(data: { state: { id: string; type: string } | null; x: number; y: number }) {
  hoveredState.value = data.state;

  // Calculate tooltip position, keeping it within viewport
  const tooltipWidth = 120;
  const tooltipHeight = 50;
  const padding = 10;

  let left = data.x + padding;
  let top = data.y + padding;

  // Constrain to viewport
  if (left + tooltipWidth > window.innerWidth - padding) {
    left = data.x - tooltipWidth - padding;
  }
  if (top + tooltipHeight > window.innerHeight - padding) {
    top = data.y - tooltipHeight - padding;
  }

  tooltipStyle.value = { left: `${left}px`, top: `${top}px` };
}

// Keyboard shortcuts
function handleKeyDown(e: KeyboardEvent) {
  // Ignore if typing in textarea
  if ((e.target as HTMLElement).tagName === 'TEXTAREA') return;

  if (e.key === '?') {
    shortcutsVisible.value = !shortcutsVisible.value;
  } else if (e.key === '+' || e.key === '=') {
    zoomIn();
  } else if (e.key === '-') {
    zoomOut();
  } else if (e.key === '0') {
    resetView();
  } else if (e.key === ' ') {
    e.preventDefault();
    if (isSimulating.value) {
      resetSimulation();
    } else if (machine.value) {
      startSimulation();
    }
  } else if (e.key === 'Escape') {
    shortcutsVisible.value = false;
  } else if (e.key >= '1' && e.key <= '9') {
    const events = getAvailableEvents();
    const idx = parseInt(e.key) - 1;
    if (idx < events.length) {
      sendEvent(events[idx]);
    }
  }
}

function getAvailableEvents(): string[] {
  return sim.availableEvents();
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown);

  // Listen for shortcuts button click from Header
  document.getElementById('shortcuts-btn')?.addEventListener('click', () => {
    shortcutsVisible.value = true;
  });
});

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown);
});
</script>
