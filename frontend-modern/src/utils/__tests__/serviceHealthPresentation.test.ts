import { describe, expect, it } from 'vitest';
import {
  getServiceHealthPresentation,
  getServiceHealthSummaryPresentation,
} from '@/utils/serviceHealthPresentation';

describe('getServiceHealthPresentation', () => {
  it('treats healthy and online states as healthy', () => {
    expect(getServiceHealthPresentation('online')).toMatchObject({
      label: 'Healthy',
      bg: expect.stringContaining('green'),
      text: expect.stringContaining('green'),
      dot: 'bg-green-500',
    });

    expect(getServiceHealthPresentation(undefined, 'healthy')).toMatchObject({
      label: 'Healthy',
    });
  });

  it('treats degraded and warning states as degraded', () => {
    expect(getServiceHealthPresentation('warning')).toMatchObject({
      label: 'Degraded',
      bg: expect.stringContaining('yellow'),
      text: expect.stringContaining('yellow'),
      dot: 'bg-yellow-500',
    });

    expect(getServiceHealthPresentation(undefined, 'degraded')).toMatchObject({
      label: 'Degraded',
    });
  });

  it('treats error and offline states as offline', () => {
    expect(getServiceHealthPresentation('offline')).toMatchObject({
      label: 'Offline',
      bg: expect.stringContaining('red'),
      text: expect.stringContaining('red'),
      dot: 'bg-red-500',
    });

    expect(getServiceHealthPresentation(undefined, 'error')).toMatchObject({
      label: 'Offline',
    });
  });

  it('falls back to unknown styling for missing values', () => {
    expect(getServiceHealthPresentation()).toEqual({
      bg: 'bg-surface-alt',
      text: 'text-muted',
      dot: 'bg-slate-400',
      label: 'Unknown',
    });
  });

  it('preserves unknown normalized labels', () => {
    expect(getServiceHealthPresentation('syncing')).toMatchObject({
      label: 'syncing',
      bg: 'bg-surface-alt',
      text: 'text-muted',
      dot: 'bg-slate-400',
    });
  });

  it('returns shared summary tone classes for compact service tables', () => {
    expect(getServiceHealthSummaryPresentation('online')).toEqual({
      tone: 'ok',
      textClass: 'text-emerald-600 dark:text-emerald-400',
    });
    expect(getServiceHealthSummaryPresentation(undefined, 'warning')).toEqual({
      tone: 'warning',
      textClass: 'text-amber-600 dark:text-amber-400',
    });
    expect(getServiceHealthSummaryPresentation('syncing')).toEqual({
      tone: 'muted',
      textClass: 'text-muted',
    });
  });
});
