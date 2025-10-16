import { describe, expect, it } from 'vitest';

import {
  ALERT_TAB_SEGMENTS,
  createDefaultCooldown,
  createDefaultEscalation,
  createDefaultGrouping,
  createDefaultQuietHours,
  extractTriggerValues,
  getTriggerValue,
  normalizeMetricDelayMap,
  pathForTab,
  tabFromPath,
} from '../Alerts';
import type { RawOverrideConfig } from '@/types/alerts';

describe('normalizeMetricDelayMap', () => {
  it('returns empty object when input is nullish', () => {
    expect(normalizeMetricDelayMap(undefined)).toEqual({});
    expect(normalizeMetricDelayMap(null)).toEqual({});
  });

  it('normalizes resource and metric keys while discarding invalid values', () => {
    const input = {
      Guest: {
        CPU: 10,
        ' ': 5,
        memory: -1,
        disk: Number.NaN,
      },
      node: {
        Temperature: 30,
        disk: 15.6,
      },
      ' ': {
        metric: 5,
      },
    };

    const result = normalizeMetricDelayMap(input);

    expect(result).toEqual({
      guest: {
        cpu: 10,
      },
      node: {
        temperature: 30,
        disk: 16,
      },
    });
  });

  it('drops metric groups that normalize to empty', () => {
    const result = normalizeMetricDelayMap({
      guest: {
        cpu: -1,
        mem: Number.NaN,
      },
    });

    expect(result).toEqual({});
  });
});

describe('tab path helpers', () => {
  it('maps tab to path', () => {
    expect(pathForTab('overview')).toBe('/alerts/overview');
    expect(pathForTab('schedule')).toBe('/alerts/schedule');
  });

  it('resolves tab from path', () => {
    expect(tabFromPath('/alerts')).toBe('overview');
    expect(tabFromPath('/alerts/thresholds')).toBe('thresholds');
    expect(tabFromPath('/alerts/thresholds/proxmox')).toBe('thresholds');
    expect(tabFromPath('/alerts/custom-rules')).toBe('thresholds');
    expect(tabFromPath('/foo/bar')).toBe('overview');
  });

  it('allows custom segments map', () => {
    const custom = { ...ALERT_TAB_SEGMENTS, overview: 'summary' as const };
    expect(pathForTab('overview', custom)).toBe('/alerts/summary');
    expect(tabFromPath('/alerts/summary', custom)).toBe('overview');
  });
});

describe('default schedule helpers', () => {
  it('creates quiet hours defaults', () => {
    const quiet = createDefaultQuietHours();
    const expectedTz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

    expect(quiet).toMatchObject({
      enabled: false,
      start: '22:00',
      end: '08:00',
      suppress: {
        performance: false,
        storage: false,
        offline: false,
      },
    });
    expect(quiet.timezone).toBe(expectedTz);
    expect(quiet.days).toEqual({
      monday: true,
      tuesday: true,
      wednesday: true,
      thursday: true,
      friday: true,
      saturday: false,
      sunday: false,
    });
  });

  it('creates cooldown defaults', () => {
    expect(createDefaultCooldown()).toEqual({
      enabled: true,
      minutes: 30,
      maxAlerts: 3,
    });
  });

  it('creates grouping defaults', () => {
    expect(createDefaultGrouping()).toEqual({
      enabled: true,
      window: 5,
      byNode: true,
      byGuest: false,
    });
  });

  it('creates escalation defaults', () => {
    expect(createDefaultEscalation()).toEqual({
      enabled: false,
      levels: [],
    });
  });
});

describe('threshold helper utilities', () => {
  it('extracts trigger values and ignores non-threshold keys', () => {
    const result = extractTriggerValues({
      cpu: { trigger: 80, clear: 70 },
      memory: { trigger: 85, clear: 75 },
      disabled: true,
      poweredOffSeverity: 'warning',
      customFlag: true,
      customLegacy: 42,
      label: 'ignored',
    } as RawOverrideConfig);

    expect(result).toEqual({
      cpu: 80,
      memory: 85,
      customFlag: 0,
      customLegacy: 42,
    });
  });

  it('getTriggerValue handles multiple input shapes', () => {
    expect(getTriggerValue(75)).toBe(75);
    expect(getTriggerValue({ trigger: 90, clear: 80 })).toBe(90);
    expect(getTriggerValue(true)).toBe(0);
    expect(getTriggerValue(undefined)).toBe(0);
  });
});
