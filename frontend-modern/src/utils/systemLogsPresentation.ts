export type SystemLogStreamPresentation = {
  indicatorClass: string;
  label: string;
  pauseButtonClass: string;
  toggleTitle: string;
};

export type SystemLogConnectionState = 'connecting' | 'streaming' | 'reconnecting' | 'disconnected';

export type SystemLogLevel = 'debug' | 'info' | 'warn' | 'error';

export type SystemLogLevelOption = {
  value: SystemLogLevel;
  label: string;
};

export interface ParsedSystemLogLine {
  raw: string;
  isStructured: boolean;
  level: SystemLogLevel | 'unknown';
  timestamp?: string;
  message: string;
  fields?: string;
}

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
  unavailableState: 'No current log output.',
  connectionError:
    'Log access could not be confirmed. Buffered lines were cleared. Reconnect to try again.',
  readerChanged:
    'The reader or workspace changed. Buffered lines were cleared. Reconnect to read current logs.',
  reconnecting:
    'The log connection was interrupted. Reconnecting while keeping buffered lines visible.',
  levelError:
    'The log level change could not be confirmed. Buffered lines are preserved while you retry.',
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

function normalizeLogLevel(value: unknown): SystemLogLevel | 'unknown' {
  if (typeof value !== 'string') return 'unknown';
  const lowered = value.trim().toLowerCase();
  if (lowered === 'error' || lowered === 'err' || lowered === 'fatal' || lowered === 'panic') {
    return 'error';
  }
  if (lowered === 'warn' || lowered === 'warning' || lowered === 'wrn') return 'warn';
  if (lowered === 'info' || lowered === 'inf') return 'info';
  if (lowered === 'debug' || lowered === 'dbg' || lowered === 'trace') return 'debug';
  return 'unknown';
}

function formatLogTimestamp(value: unknown): string | undefined {
  if (typeof value !== 'string' || !value.trim()) return undefined;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value.trim();
  return parsed.toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
}

export function parseSystemLogLine(raw: string): ParsedSystemLogLine {
  const trimmed = raw.trim();
  if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
    try {
      const parsed = JSON.parse(trimmed) as Record<string, unknown>;
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        const level = normalizeLogLevel(parsed.level ?? parsed.lvl);
        const timestamp = formatLogTimestamp(parsed.time ?? parsed.timestamp ?? parsed.ts);
        const messageCandidate = parsed.message ?? parsed.msg;
        const message =
          typeof messageCandidate === 'string' && messageCandidate.trim() ? messageCandidate : raw;
        const extraPairs: string[] = [];
        for (const [key, value] of Object.entries(parsed)) {
          if (
            key === 'level' ||
            key === 'lvl' ||
            key === 'time' ||
            key === 'timestamp' ||
            key === 'ts' ||
            key === 'message' ||
            key === 'msg' ||
            value === undefined ||
            value === null ||
            value === ''
          ) {
            continue;
          }
          const formattedValue =
            typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean'
              ? String(value)
              : JSON.stringify(value);
          extraPairs.push(`${key}=${formattedValue}`);
        }
        return {
          raw,
          isStructured: true,
          level,
          timestamp,
          message,
          fields: extraPairs.length > 0 ? extraPairs.join(' ') : undefined,
        };
      }
    } catch {
      // Fall back to raw text presentation for non-JSON lines.
    }
  }

  let level: SystemLogLevel | 'unknown' = 'unknown';
  if (includesAny(raw, ERROR_PATTERNS)) level = 'error';
  else if (includesAny(raw, WARNING_PATTERNS)) level = 'warn';
  else if (includesAny(raw, DEBUG_PATTERNS)) level = 'debug';
  else if (raw.includes('INFO') || raw.includes('INF')) level = 'info';

  return {
    raw,
    isStructured: false,
    level,
    message: raw,
  };
}

export function getSystemLogLevelBadgeClass(level: SystemLogLevel | 'unknown'): string {
  switch (level) {
    case 'error':
      return 'bg-red-500/20 text-red-300 border-red-500/40';
    case 'warn':
      return 'bg-amber-500/20 text-amber-300 border-amber-500/40';
    case 'debug':
      return 'bg-blue-500/20 text-blue-300 border-blue-500/40';
    case 'info':
      return 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40';
    default:
      return 'bg-slate-800 text-slate-300 border-slate-700';
  }
}

export function getSystemLogStreamPresentation(
  paused: boolean,
  connection: SystemLogConnectionState = 'streaming',
): SystemLogStreamPresentation {
  const display = paused ? STREAM_PRESENTATION.paused : STREAM_PRESENTATION.live;
  if (connection === 'streaming') return display;
  const labels = {
    connecting: 'Connecting',
    reconnecting: 'Reconnecting',
    disconnected: 'Disconnected',
  };
  return {
    ...display,
    indicatorClass: connection === 'disconnected' ? 'bg-slate-400' : 'bg-amber-400',
    label: `${labels[connection]}${paused ? ' · paused' : ''}`,
  };
}

export function getSystemLogBufferSummary(logCount: number, maxLogs: number): string {
  return `Buffer: ${logCount} / ${maxLogs} lines`;
}
