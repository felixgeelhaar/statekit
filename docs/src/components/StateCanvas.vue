<template>
  <div
    ref="containerRef"
    class="canvas-container"
    @mousedown="handleMouseDown"
    @mousemove="handleMouseMove"
    @mouseup="handleMouseUp"
    @mouseleave="handleMouseUp"
    @wheel="handleWheel"
  >
    <canvas ref="canvasRef"></canvas>

    <!-- Empty State -->
    <div v-if="!machine" class="canvas-empty">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
        <path d="M3 9h18"/>
        <path d="M9 21V9"/>
      </svg>
      <h3>No Machine Loaded</h3>
      <p>Import a Statekit JSON file to visualize your state machine</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue';
import type { MachineConfig, StatePosition, CanvasColors } from '../utils/types';
import { defaultColors } from '../utils/types';

const props = defineProps<{
  machine: MachineConfig | null;
  currentState: string | null;
  statePositions: Record<string, StatePosition>;
}>();

const emit = defineEmits<{
  'state-hover': [data: { state: { id: string; type: string } | null; x: number; y: number }];
}>();

// Refs
const containerRef = ref<HTMLDivElement | null>(null);
const canvasRef = ref<HTMLCanvasElement | null>(null);

// Canvas state
const scale = ref(1);
const offsetX = ref(0);
const offsetY = ref(0);
const isPanning = ref(false);
const lastMouseX = ref(0);
const lastMouseY = ref(0);

// Colors
const colors: CanvasColors = defaultColors;

// Exposed methods
function zoomIn() {
  scale.value = Math.min(scale.value * 1.2, 3);
  render();
}

function zoomOut() {
  scale.value = Math.max(scale.value / 1.2, 0.3);
  render();
}

function resetView() {
  scale.value = 1;
  offsetX.value = 0;
  offsetY.value = 0;
  render();
}

defineExpose({ zoomIn, zoomOut, resetView });

// Mouse handlers
function handleMouseDown(e: MouseEvent) {
  if (e.button === 0) {
    isPanning.value = true;
    lastMouseX.value = e.clientX;
    lastMouseY.value = e.clientY;
  }
}

function handleMouseMove(e: MouseEvent) {
  const rect = containerRef.value?.getBoundingClientRect();
  if (!rect) return;

  // Handle panning
  if (isPanning.value) {
    offsetX.value += e.clientX - lastMouseX.value;
    offsetY.value += e.clientY - lastMouseY.value;
    lastMouseX.value = e.clientX;
    lastMouseY.value = e.clientY;
    render();
    return;
  }

  // Handle hover
  const x = (e.clientX - rect.left - offsetX.value) / scale.value;
  const y = (e.clientY - rect.top - offsetY.value) / scale.value;

  let hoveredState: { id: string; type: string } | null = null;

  if (props.machine) {
    for (const [id, pos] of Object.entries(props.statePositions)) {
      if (
        x >= pos.x - pos.width / 2 &&
        x <= pos.x + pos.width / 2 &&
        y >= pos.y - pos.height / 2 &&
        y <= pos.y + pos.height / 2
      ) {
        const state = props.machine.states[id];
        hoveredState = { id, type: state?.type || 'atomic' };
        break;
      }
    }
  }

  emit('state-hover', { state: hoveredState, x: e.clientX, y: e.clientY });
}

function handleMouseUp() {
  isPanning.value = false;
}

function handleWheel(e: WheelEvent) {
  e.preventDefault();
  const delta = e.deltaY > 0 ? 0.9 : 1.1;
  scale.value = Math.max(0.3, Math.min(3, scale.value * delta));
  render();
}

// Rendering
function render() {
  const canvas = canvasRef.value;
  const container = containerRef.value;
  if (!canvas || !container) return;

  const ctx = canvas.getContext('2d');
  if (!ctx) return;

  // Set canvas size
  const rect = container.getBoundingClientRect();
  canvas.width = rect.width * window.devicePixelRatio;
  canvas.height = rect.height * window.devicePixelRatio;
  canvas.style.width = `${rect.width}px`;
  canvas.style.height = `${rect.height}px`;
  ctx.scale(window.devicePixelRatio, window.devicePixelRatio);

  // Clear canvas
  ctx.fillStyle = colors.bg;
  ctx.fillRect(0, 0, rect.width, rect.height);

  if (!props.machine) return;

  // Apply transform
  ctx.save();
  ctx.translate(offsetX.value, offsetY.value);
  ctx.scale(scale.value, scale.value);

  // Draw transitions first (behind states)
  drawTransitions(ctx);

  // Draw states
  for (const [id, pos] of Object.entries(props.statePositions)) {
    const state = props.machine.states[id];
    const isActive = props.currentState === id;
    drawState(ctx, id, pos, state?.type || 'atomic', isActive, !!state?.children);
  }

  ctx.restore();
}

function drawState(
  ctx: CanvasRenderingContext2D,
  id: string,
  pos: StatePosition,
  type: string,
  isActive: boolean,
  hasChildren: boolean
) {
  const x = pos.x - pos.width / 2;
  const y = pos.y - pos.height / 2;
  const radius = type === 'history' ? pos.width / 2 : 8;

  // Determine color
  let stateColor = colors.atomic;
  switch (type) {
    case 'compound': stateColor = colors.compound; break;
    case 'final': stateColor = colors.final; break;
    case 'parallel': stateColor = colors.parallel; break;
    case 'history': stateColor = colors.history; break;
  }

  if (isActive) stateColor = colors.active;

  // Draw shadow for active state
  if (isActive) {
    ctx.shadowColor = stateColor;
    ctx.shadowBlur = 20;
  }

  // Draw state background
  ctx.beginPath();
  if (type === 'history') {
    ctx.arc(pos.x, pos.y, radius, 0, Math.PI * 2);
  } else {
    ctx.roundRect(x, y, pos.width, pos.height, radius);
  }

  // Fill
  ctx.fillStyle = `${stateColor}15`;
  ctx.fill();

  // Border
  ctx.strokeStyle = isActive ? stateColor : colors.border;
  ctx.lineWidth = isActive ? 2 : 1;
  ctx.stroke();

  ctx.shadowColor = 'transparent';
  ctx.shadowBlur = 0;

  // Draw label
  ctx.fillStyle = isActive ? stateColor : colors.text;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';

  if (type === 'history') {
    ctx.font = '600 10px var(--font-mono), monospace';
    ctx.fillText('H', pos.x, pos.y);
  } else if (hasChildren) {
    // For compound states: draw label at the top
    ctx.font = '600 11px var(--font-mono), monospace';
    ctx.fillText(id, pos.x, y + 20);
    // Draw type indicator below the label
    ctx.font = '500 8px var(--font-mono), monospace';
    ctx.fillStyle = colors.textMuted;
    ctx.fillText(type.toUpperCase(), pos.x, y + 32);
  } else {
    // Atomic states: draw label in center
    ctx.font = '600 12px var(--font-mono), monospace';
    ctx.fillText(id, pos.x, pos.y);
  }

  // Draw final state indicator (inner circle)
  if (type === 'final') {
    ctx.beginPath();
    ctx.arc(pos.x, pos.y + 15, 4, 0, Math.PI * 2);
    ctx.fillStyle = colors.final;
    ctx.fill();
  }
}

function drawTransitions(ctx: CanvasRenderingContext2D) {
  if (!props.machine) return;

  for (const [fromId, state] of Object.entries(props.machine.states)) {
    if (!state.transitions) continue;

    const fromPos = props.statePositions[fromId];
    if (!fromPos) continue;

    for (const transition of state.transitions) {
      if (!transition.target) continue;

      const toPos = props.statePositions[transition.target];
      if (!toPos) continue;

      drawArrow(ctx, fromPos, toPos, transition.event);
    }
  }
}

function drawArrow(
  ctx: CanvasRenderingContext2D,
  from: StatePosition,
  to: StatePosition,
  label?: string
) {
  // Calculate direction
  const dx = to.x - from.x;
  const dy = to.y - from.y;
  const angle = Math.atan2(dy, dx);

  // Calculate start and end points at state edges
  const startX = from.x + Math.cos(angle) * (from.width / 2 + 5);
  const startY = from.y + Math.sin(angle) * (from.height / 2 + 5);
  const endX = to.x - Math.cos(angle) * (to.width / 2 + 15);
  const endY = to.y - Math.sin(angle) * (to.height / 2 + 15);

  // Draw line
  ctx.beginPath();
  ctx.moveTo(startX, startY);
  ctx.lineTo(endX, endY);
  ctx.strokeStyle = colors.arrowDim;
  ctx.lineWidth = 1.5;
  ctx.stroke();

  // Draw arrowhead
  const headLength = 10;
  ctx.beginPath();
  ctx.moveTo(endX, endY);
  ctx.lineTo(
    endX - headLength * Math.cos(angle - Math.PI / 6),
    endY - headLength * Math.sin(angle - Math.PI / 6)
  );
  ctx.moveTo(endX, endY);
  ctx.lineTo(
    endX - headLength * Math.cos(angle + Math.PI / 6),
    endY - headLength * Math.sin(angle + Math.PI / 6)
  );
  ctx.strokeStyle = colors.arrow;
  ctx.lineWidth = 2;
  ctx.stroke();

  // Draw label
  if (label) {
    const midX = (startX + endX) / 2;
    const midY = (startY + endY) / 2;

    ctx.font = '500 10px var(--font-mono), monospace';
    const textWidth = ctx.measureText(label).width;

    // Background
    ctx.fillStyle = colors.bg;
    ctx.fillRect(midX - textWidth / 2 - 4, midY - 8, textWidth + 8, 16);

    // Text
    ctx.fillStyle = colors.accent;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(label, midX, midY);
  }
}

// Setup
function setupCanvas() {
  const container = containerRef.value;
  if (!container) return;

  render();
}

// Resize observer
let resizeObserver: ResizeObserver | null = null;

onMounted(() => {
  setupCanvas();

  if (containerRef.value) {
    resizeObserver = new ResizeObserver(() => {
      render();
    });
    resizeObserver.observe(containerRef.value);
  }
});

onUnmounted(() => {
  resizeObserver?.disconnect();
});

// Watch for changes
watch(() => [props.machine, props.currentState, props.statePositions], () => {
  render();
}, { deep: true });
</script>

<style scoped>
.canvas-container {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  cursor: grab;
}

.canvas-container:active {
  cursor: grabbing;
}

.canvas-container canvas {
  display: block;
}

.canvas-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  text-align: center;
  padding: 2rem;
}

.canvas-empty svg {
  width: 4rem;
  height: 4rem;
  margin-bottom: 1rem;
  opacity: 0.3;
}

.canvas-empty h3 {
  font-size: 1.25rem;
  font-weight: 600;
  margin-bottom: 0.5rem;
  color: var(--text);
}

.canvas-empty p {
  font-size: 0.875rem;
  max-width: 280px;
}
</style>
