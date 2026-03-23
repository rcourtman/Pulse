import { describe, expect, it } from 'vitest';
import {
  getSystemLogBufferSummary,
  getSystemLogLineClass,
  getSystemLogStreamPresentation,
  SYSTEM_LOG_LEVEL_OPTIONS,
  SYSTEM_LOGS_PANEL_COPY,
} from '@/utils/systemLogsPresentation';

describe('systemLogsPresentation', () => {
  it('returns canonical system logs panel copy and level options', () => {
    expect(SYSTEM_LOGS_PANEL_COPY).toEqual({
      title: 'System Logs',
      description: 'Stream live system logs and download support bundles.',
      levelLabel: 'Log Level:',
      clearTitle: 'Clear Log Output',
      downloadLabel: 'Support Bundle',
      emptyState: 'Waiting for log output.',
    });
    expect(SYSTEM_LOG_LEVEL_OPTIONS).toEqual([
      { value: 'debug', label: 'Debug' },
      { value: 'info', label: 'Info' },
      { value: 'warn', label: 'Warn' },
      { value: 'error', label: 'Error' },
    ]);
  });

  it('maps log severity text to canonical classes', () => {
    expect(getSystemLogLineClass('{"level":"error","msg":"boom"}')).toBe('text-red-400');
    expect(getSystemLogLineClass('[WARN] something odd')).toBe('text-amber-400');
    expect(getSystemLogLineClass('DBG refresh loop')).toBe('text-blue-400');
    expect(getSystemLogLineClass('plain info log')).toBe('text-slate-300');
  });

  it('maps live and paused stream state to canonical presentation', () => {
    expect(getSystemLogStreamPresentation(false)).toEqual({
      indicatorClass: 'bg-emerald-400 animate-pulse',
      label: 'Streaming',
      pauseButtonClass: 'hover:bg-surface-hover text-muted',
      toggleTitle: 'Pause Stream',
    });
    expect(getSystemLogStreamPresentation(true)).toEqual({
      indicatorClass: 'bg-amber-400',
      label: 'Paused',
      pauseButtonClass: 'bg-amber-100 text-amber-600 dark:bg-amber-900 dark:text-amber-400',
      toggleTitle: 'Resume Stream',
    });
  });

  it('formats the canonical log buffer summary', () => {
    expect(getSystemLogBufferSummary(24, 500)).toBe('Buffer: 24 / 500 lines');
  });
});
