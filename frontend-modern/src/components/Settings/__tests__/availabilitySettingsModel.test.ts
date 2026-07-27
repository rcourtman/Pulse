import { describe, expect, it } from 'vitest';
import {
  buildAvailabilitySettingsPath,
  buildAvailabilityTargetAddPath,
  getAvailabilityTargetAddKind,
  getAvailabilityTargetAddressLabel,
  getAvailabilityTargetKindLabel,
  getAvailabilityTargetMethodLabel,
  getAvailabilityTargetProbeSourceLabel,
  getAvailabilityTargetStatusClass,
  getAvailabilityTargetStatusLabel,
  getAvailabilityTargetsSummary,
  normalizeAvailabilityTargetKind,
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
    expect(buildAvailabilityTargetAddPath('machine')).toBe(
      '/settings/monitoring/availability?add=target&targetKind=machine',
    );
    expect(buildAvailabilityTargetAddPath('service')).toBe(
      '/settings/monitoring/availability?add=target&targetKind=service',
    );
    expect(
      shouldOpenAvailabilityTargetAddDialog('/settings/monitoring/availability', '?add=target'),
    ).toBe(true);
    expect(
      shouldOpenAvailabilityTargetAddDialog(
        '/settings/monitoring/availability',
        '?add=target&targetKind=machine',
      ),
    ).toBe(true);
    expect(
      shouldOpenAvailabilityTargetAddDialog(
        '/settings/monitoring/availability',
        '?add=target&targetKind=vm',
      ),
    ).toBe(false);
    expect(
      getAvailabilityTargetAddKind(
        '/settings/monitoring/availability',
        '?add=target&targetKind=machine',
      ),
    ).toBe('machine');
    expect(getAvailabilityTargetAddKind('/settings/monitoring/availability', '?add=target')).toBe(
      undefined,
    );
    expect(
      shouldOpenAvailabilityTargetAddDialog('/settings/infrastructure', '?add=availability'),
    ).toBe(false);
    expect(normalizeAvailabilityTargetKind('Machine')).toBe('machine');
    expect(normalizeAvailabilityTargetKind('vm')).toBe(undefined);
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
    expect(
      getAvailabilityTargetsSummary([
        target({
          protocol: 'udp',
          port: 27015,
          status: {
            ...target(),
            targetId: 'steam-server',
            protocol: 'udp',
            available: false,
            outcome: 'indeterminate',
          },
        }),
      ]),
    ).toBe('1 open or filtered · 1 enabled');
    expect(
      getAvailabilityTargetsSummary([
        target({
          id: 'closed-port',
          status: { ...target(), targetId: 'closed-port', available: false },
        }),
        target({
          id: 'silent-port',
          protocol: 'udp',
          port: 27015,
          status: {
            ...target(),
            targetId: 'silent-port',
            protocol: 'udp',
            available: false,
            outcome: 'indeterminate',
          },
        }),
      ]),
    ).toBe('1 down · 1 open or filtered · 2 enabled');
    expect(getAvailabilityTargetStatusClass(target({ enabled: false }))).toBe(
      'bg-surface-alt text-muted',
    );
    expect(getAvailabilityTargetStatusClass(target())).toBe(
      'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
    );
    expect(
      getAvailabilityTargetStatusClass(
        target({
          status: { ...target(), targetId: 'mqtt-broker', available: true, latencyMillis: 12 },
        }),
      ),
    ).toBe('bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300');
    expect(
      getAvailabilityTargetStatusClass(
        target({ status: { ...target(), targetId: 'mqtt-broker', available: false } }),
      ),
    ).toBe('bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300');
  });

  it('attributes probe-reported results to the assigned agent host', () => {
    const options = [{ id: 'host-edge-01', label: 'Edge 01' }];
    const probed = target({
      probeAgentId: 'host-edge-01',
      status: {
        ...target(),
        targetId: 'mqtt-broker',
        available: true,
        latencyMillis: 12,
        probeAgentId: 'host-edge-01',
      },
    });

    expect(getAvailabilityTargetProbeSourceLabel(probed, options)).toBe('via Edge 01');
    // A locally executed check carries no attribution chip.
    expect(
      getAvailabilityTargetProbeSourceLabel(
        target({ status: { ...target(), targetId: 'mqtt-broker', available: true } }),
        options,
      ),
    ).toBeNull();
    // Unknown hosts degrade to the raw id rather than disappearing.
    expect(
      getAvailabilityTargetProbeSourceLabel(
        target({
          status: {
            ...target(),
            targetId: 'mqtt-broker',
            available: true,
            probeAgentId: 'host-gone',
          },
        }),
        options,
      ),
    ).toBe('via host-gone');
  });

  it('renders a stale probe assignment with the existing indeterminate treatment', () => {
    const stale = target({
      probeAgentId: 'host-edge-01',
      status: {
        ...target(),
        targetId: 'mqtt-broker',
        available: false,
        outcome: 'indeterminate',
        lastError: 'no recent report from probe agent',
        probeAgentId: 'host-edge-01',
      },
    });

    expect(getAvailabilityTargetStatusLabel(stale)).toBe('No recent probe report');
    // No new visual language: it reuses the amber indeterminate badge.
    expect(getAvailabilityTargetStatusClass(stale)).toBe(
      'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
    );
    // The UDP open-or-filtered case keeps its own copy.
    expect(
      getAvailabilityTargetStatusLabel(
        target({
          status: {
            ...target(),
            targetId: 'mqtt-broker',
            available: true,
            outcome: 'indeterminate',
          },
        }),
      ),
    ).toBe('Open or filtered');
  });
});
