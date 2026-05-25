import { describe, expect, it } from 'vitest';
import {
  buildAvailabilitySettingsPath,
  buildAvailabilityTargetAddPath,
  getAvailabilityTargetAddressLabel,
  getAvailabilityTargetKindLabel,
  getAvailabilityTargetMethodLabel,
  getAvailabilityTargetStatusLabel,
  getAvailabilityTargetsSummary,
  shouldOpenAvailabilityTargetAddDialog,
} from '../availabilitySettingsModel';
import type { AvailabilityTarget } from '@/api/availabilityTargets';

const target = (overrides: Partial<AvailabilityTarget> = {}): AvailabilityTarget => ({
  id: 'mqtt-broker',
  name: 'MQTT broker',
  address: 'mqtt.local',
  protocol: 'tcp',
  port: 1883,
  enabled: true,
  ...overrides,
});

describe('availabilitySettingsModel', () => {
  it('owns the canonical monitoring availability settings path', () => {
    expect(buildAvailabilitySettingsPath()).toBe('/settings/monitoring/availability');
    expect(buildAvailabilityTargetAddPath()).toBe('/settings/monitoring/availability?add=target');
    expect(
      shouldOpenAvailabilityTargetAddDialog('/settings/monitoring/availability', '?add=target'),
    ).toBe(true);
    expect(
      shouldOpenAvailabilityTargetAddDialog('/settings/infrastructure', '?add=availability'),
    ).toBe(false);
  });

  it('formats endpoint methods and addresses without infrastructure copy', () => {
    expect(getAvailabilityTargetMethodLabel(target())).toBe('TCP 1883');
    expect(getAvailabilityTargetAddressLabel(target())).toBe('mqtt.local:1883');
    expect(
      getAvailabilityTargetAddressLabel(
        target({ protocol: 'http', address: 'http://sensor.local', path: '/health' }),
      ),
    ).toBe('http://sensor.local/health');
    expect(
      getAvailabilityTargetAddressLabel(
        target({ protocol: 'http', address: 'http://sensor.local/', path: '/health' }),
      ),
    ).toBe('http://sensor.local/health');
    expect(getAvailabilityTargetMethodLabel(target({ protocol: 'icmp', port: undefined }))).toBe(
      'ICMP ping',
    );
    expect(getAvailabilityTargetKindLabel(target({ targetKind: 'machine' }))).toBe('Machine');
    expect(getAvailabilityTargetKindLabel(target())).toBe('Service');
  });

  it('summarizes saved availability targets from their probe state', () => {
    expect(
      getAvailabilityTargetStatusLabel(
        target({
          status: { ...target(), targetId: 'mqtt-broker', available: true, latencyMillis: 12 },
        }),
      ),
    ).toBe('Online · 12 ms');
    expect(
      getAvailabilityTargetsSummary([
        target(),
        target({
          id: 'http-health',
          name: 'HTTP health',
          protocol: 'http',
          address: 'http://service.local',
          status: { ...target(), targetId: 'http-health', available: false },
        }),
      ]),
    ).toBe('1 down · 2 enabled');
  });
});
