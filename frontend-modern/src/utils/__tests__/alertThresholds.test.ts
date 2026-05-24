import { describe, expect, it } from 'vitest';

import {
  getDefaultMetricDisplayThresholds,
  getMetricSeverity,
  getMetricVisualSeverity,
  resolveMetricDisplayThresholds,
} from '../alertThresholds';
import type { AlertConfig } from '@/types/alerts';

describe('alertThresholds', () => {
  it('uses alert defaults for display bands', () => {
    expect(getDefaultMetricDisplayThresholds('cpu')).toEqual({ warning: 75, critical: 80 });
    expect(getDefaultMetricDisplayThresholds('memory')).toEqual({ warning: 80, critical: 85 });
    expect(getDefaultMetricDisplayThresholds('disk')).toEqual({ warning: 85, critical: 90 });
    expect(getDefaultMetricDisplayThresholds('generic')).toEqual({ warning: 75, critical: 90 });
  });

  it('resolves scope defaults when no config is loaded', () => {
    expect(resolveMetricDisplayThresholds(null, 'node', 'memory')).toEqual({
      warning: 80,
      critical: 85,
    });
    expect(resolveMetricDisplayThresholds(null, 'docker', 'disk')).toEqual({
      warning: 80,
      critical: 85,
    });
  });

  it('prefers resource overrides and uses explicit clear values', () => {
    const config = {
      enabled: true,
      guestDefaults: {},
      nodeDefaults: {
        cpu: { trigger: 80, clear: 75 },
      },
      storageDefault: { trigger: 85, clear: 80 },
      overrides: {
        'node-1': {
          cpu: { trigger: 85, clear: 80 },
        },
      },
    } as AlertConfig;

    expect(resolveMetricDisplayThresholds(config, 'node', 'cpu', 'node-1')).toEqual({
      warning: 80,
      critical: 85,
    });
    expect(resolveMetricDisplayThresholds(config, 'node', 'cpu', 'node-2')).toEqual({
      warning: 75,
      critical: 80,
    });
  });

  it('treats disabled thresholds as absent', () => {
    const config = {
      enabled: true,
      guestDefaults: {},
      nodeDefaults: {
        cpu: { trigger: 80, clear: 75 },
      },
      storageDefault: { trigger: 85, clear: 80 },
      overrides: {
        'node-1': {
          cpu: { trigger: -1, clear: 0 },
        },
      },
    } as AlertConfig;

    expect(resolveMetricDisplayThresholds(config, 'node', 'cpu', 'node-1')).toBeNull();
  });

  it('maps usage values to severity bands', () => {
    const thresholds = { warning: 80, critical: 85 };
    expect(getMetricSeverity(79, thresholds)).toBe('green');
    expect(getMetricSeverity(80, thresholds)).toBe('yellow');
    expect(getMetricSeverity(85, thresholds)).toBe('red');
  });

  it('keeps visual severity when notification threshold is disabled', () => {
    expect(getMetricSeverity(100, null)).toBe('green');
    expect(getMetricVisualSeverity(100, 'memory', null)).toBe('red');
  });
});
