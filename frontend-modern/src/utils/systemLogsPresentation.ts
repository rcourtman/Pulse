export type SystemLogStreamPresentation = {
  indicatorClass: string;
  label: string;
  pauseButtonClass: string;
  toggleTitle: string;
};

export type SystemLogLevelOption = {
  value: 'debug' | 'info' | 'warn' | 'error';
  label: string;
};

export const SYSTEM_LOGS_PANEL_COPY = {
  title: 'System Logs',
  description: 'Stream live system logs and download support bundles.',
  levelLabel: 'Server Log Level:',
  levelHelp:
    'Changes logging for the whole Pulse server. Debug can include private guest names and paths. Keep the current level unless needed.',
  displayHelp:
    'Pause and Clear affect this display only. Monitoring and server logging continue. This is not a backup safety pause. Messages received while paused are not added to the display.',
  bundleHelp:
    'Support bundles include server logs, configuration and environment details, not just the lines shown here. Pausing or clearing this display does not remove them. Keep the archive private. Share only manually reviewed, redacted excerpts.',
  clearTitle: 'Clear Log Output',
  downloadLabel: 'Support Bundle',
  emptyState: 'Waiting for log output.',
  bufferHelp:
    'This view keeps only the latest 1,000 received lines. Paused lines are not collected. Clearing the view does not delete server logs.',
} as const;

export const SYSTEM_LOG_LEVEL_OPTIONS: readonly SystemLogLevelOption[] = [
  { value: 'debug', label: 'Debug' },
  { value: 'info', label: 'Info' },
  { value: 'warn', label: 'Warn' },
  { value: 'error', label: 'Error' },
] as const;

const ERROR_PATTERNS = ['"level":"error"', 'ERR', '[ERROR]'];
const WARNING_PATTERNS = ['"level":"warn"', 'WRN', '[WARN]'];
const DEBUG_PATTERNS = ['"level":"debug"', 'DBG', '[DEBUG]'];

const STREAM_PRESENTATION: Record<'live' | 'paused', SystemLogStreamPresentation> = {
  live: {
    indicatorClass: 'bg-emerald-400 animate-pulse',
    label: 'Streaming',
    pauseButtonClass: 'hover:bg-surface-hover text-muted',
    toggleTitle: 'Pause Stream',
  },
  paused: {
    indicatorClass: 'bg-amber-400',
    label: 'Paused',
    pauseButtonClass: 'bg-amber-100 text-amber-600 dark:bg-amber-900/25 dark:text-amber-400',
    toggleTitle: 'Resume Stream',
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

export function getSystemLogBufferSummary(logCount: number, maxLogs: number): string {
  return `Buffer: ${logCount} / ${maxLogs} lines`;
}
