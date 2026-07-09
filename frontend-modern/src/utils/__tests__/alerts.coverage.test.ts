import { describe, expect, it } from 'vitest';
import { isMetricAlertType, isStateAlertType } from '@/utils/alerts';

const STATE_ALERT_TYPES = [
  'powered-off',
  'unreachable',
  'offline',
  'host-offline',
  'connectivity',
  'docker-host-offline',
  'docker-container-state',
  'docker-container-health',
] as const;

const METRIC_ALERT_TYPES = [
  'high-cpu',
  'cpu',
  'memory',
  'disk-usage',
  'network-in',
  'latency',
] as const;

describe('isStateAlertType', () => {
  it.each(STATE_ALERT_TYPES)('returns true for the state alert type %s', (alertType) => {
    expect(isStateAlertType(alertType)).toBe(true);
  });

  it.each(METRIC_ALERT_TYPES)('returns false for the metric alert type %s', (alertType) => {
    expect(isStateAlertType(alertType)).toBe(false);
  });

  it('returns false for an arbitrary unknown alert type', () => {
    expect(isStateAlertType('some-new-threshold-rule')).toBe(false);
  });

  it('returns false for undefined', () => {
    expect(isStateAlertType(undefined)).toBe(false);
  });

  it('returns false for the empty string via the falsy guard', () => {
    expect(isStateAlertType('')).toBe(false);
  });

  it.each([
    'Powered-Off',
    'UNREACHABLE',
    'Offline',
    'Host-Offline',
  ])('is case-sensitive: %s is not a state alert type', (alertType) => {
    expect(isStateAlertType(alertType)).toBe(false);
  });

  it.each([' powered-off', 'powered-off ', 'unreachable\n', '\toffline'])(
    'does not match state types with surrounding whitespace: %j',
    (alertType) => {
      expect(isStateAlertType(alertType)).toBe(false);
    },
  );

  it.each(['PoweredOff', 'powered_off', 'poweredOff', 'hostOffline'])(
    'does not match separator/camel variants of state types: %j',
    (alertType) => {
      expect(isStateAlertType(alertType)).toBe(false);
    },
  );
});

describe('isMetricAlertType', () => {
  it.each(METRIC_ALERT_TYPES)('returns true for the metric alert type %s', (alertType) => {
    expect(isMetricAlertType(alertType)).toBe(true);
  });

  it.each(STATE_ALERT_TYPES)('returns false for the state alert type %s', (alertType) => {
    expect(isMetricAlertType(alertType)).toBe(false);
  });

  it('returns true for an arbitrary unknown alert type', () => {
    expect(isMetricAlertType('some-new-threshold-rule')).toBe(true);
  });

  it('returns true for undefined', () => {
    expect(isMetricAlertType(undefined)).toBe(true);
  });

  it('returns true for the empty string', () => {
    expect(isMetricAlertType('')).toBe(true);
  });

  it.each(['Powered-Off', ' powered-off ', 'powered_off'])(
    'treats non-canonical variants of state types as metric: %j',
    (alertType) => {
      expect(isMetricAlertType(alertType)).toBe(true);
    },
  );

  it('is the exact logical inverse of isStateAlertType across a mixed input set', () => {
    const inputs = [
      ...STATE_ALERT_TYPES,
      ...METRIC_ALERT_TYPES,
      '',
      undefined,
      'unknown-type',
      'Powered-Off',
      ' powered-off ',
    ];
    for (const input of inputs) {
      expect(isMetricAlertType(input)).toBe(!isStateAlertType(input));
    }
  });
});
