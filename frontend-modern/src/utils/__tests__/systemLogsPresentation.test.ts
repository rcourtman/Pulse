import { describe, expect, it } from 'vitest';
import {
  getSystemLogLineClass,
  getSystemLogStreamPresentation,
} from '@/utils/systemLogsPresentation';

describe('systemLogsPresentation', () => {
  it('maps log severity text to canonical classes', () => {
    expect(getSystemLogLineClass('{"level":"error","msg":"boom"}')).toBe('text-red-400');
    expect(getSystemLogLineClass('[WARN] something odd')).toBe('text-amber-400');
    expect(getSystemLogLineClass('DBG refresh loop')).toBe('text-blue-400');
    expect(getSystemLogLineClass('plain info log')).toBe('text-slate-300');
  });

  it('maps live and paused stream state to canonical presentation', () => {
    expect(getSystemLogStreamPresentation(false)).toEqual({
      indicatorClass: 'bg-emerald-400 animate-pulse',
      label: 'Live',
      pauseButtonClass: 'hover:bg-surface-hover text-muted',
    });
    expect(getSystemLogStreamPresentation(true)).toEqual({
      indicatorClass: 'bg-amber-400',
      label: 'Stream Paused',
      pauseButtonClass: 'bg-amber-100 text-amber-600 dark:bg-amber-900 dark:text-amber-400',
    });
  });
});
