export type SystemLogStreamPresentation = {
  indicatorClass: string;
  label: string;
  pauseButtonClass: string;
};

const ERROR_PATTERNS = ['"level":"error"', 'ERR', '[ERROR]'];
const WARNING_PATTERNS = ['"level":"warn"', 'WRN', '[WARN]'];
const DEBUG_PATTERNS = ['"level":"debug"', 'DBG', '[DEBUG]'];

const STREAM_PRESENTATION: Record<'live' | 'paused', SystemLogStreamPresentation> = {
  live: {
    indicatorClass: 'bg-emerald-400 animate-pulse',
    label: 'Live',
    pauseButtonClass: 'hover:bg-surface-hover text-muted',
  },
  paused: {
    indicatorClass: 'bg-amber-400',
    label: 'Stream Paused',
    pauseButtonClass: 'bg-amber-100 text-amber-600 dark:bg-amber-900 dark:text-amber-400',
  },
};

function includesAny(value: string, patterns: string[]): boolean {
  return patterns.some((pattern) => value.includes(pattern));
}

export function getSystemLogLineClass(log: string): string {
  if (includesAny(log, ERROR_PATTERNS)) return 'text-red-400';
  if (includesAny(log, WARNING_PATTERNS)) return 'text-amber-400';
  if (includesAny(log, DEBUG_PATTERNS)) return 'text-blue-400';
  return 'text-slate-300';
}

export function getSystemLogStreamPresentation(paused: boolean): SystemLogStreamPresentation {
  return paused ? STREAM_PRESENTATION.paused : STREAM_PRESENTATION.live;
}
