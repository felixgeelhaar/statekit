export interface Transition {
  event: string;
  target: string;
  guard?: string;
  actions?: string[];
}

export interface StateConfig {
  type?: 'atomic' | 'compound' | 'final' | 'parallel' | 'history';
  initial?: string;
  parent?: string;
  children?: string[];
  transitions?: Transition[];
  entry?: string[];
  exit?: string[];
}

export interface MachineConfig {
  id: string;
  initial: string;
  states: Record<string, StateConfig>;
  context?: Record<string, unknown>;
}

export interface StatePosition {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface HistoryItem {
  event: string;
  from?: string;
  to: string;
}

export interface CanvasColors {
  atomic: string;
  compound: string;
  final: string;
  parallel: string;
  history: string;
  active: string;
  bg: string;
  border: string;
  text: string;
  textMuted: string;
  arrow: string;
  arrowDim: string;
}

export const defaultColors: CanvasColors = {
  atomic: '#00d4ff',
  compound: '#a855f7',
  final: '#00ff88',
  parallel: '#ff9500',
  history: '#f472b6',
  active: '#00ff88',
  bg: '#0a0e14',
  border: '#21262d',
  text: '#e6edf3',
  textMuted: '#8b949e',
  arrow: '#00d4ff',
  arrowDim: '#00d4ff60'
};
