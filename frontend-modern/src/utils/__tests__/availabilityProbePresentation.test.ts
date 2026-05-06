import { describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getAvailabilityProbeMethodLabel,
  getAvailabilityProbePresentation,
} from '@/utils/availabilityProbePresentation';

const makeAvailabilityResource = (overrides?: Partial<Resource>): Resource => ({
  id: 'availability:mock-availability-mqtt-meter',
  type: 'network-endpoint',
  name: 'MQTT power meter',
  displayName: 'MQTT power meter',
  platformId: 'mock-availability-mqtt-meter',
  platformType: 'availability',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  availability: {
    protocol: 'tcp',
    port: 1883,
    available: true,
    latencyMillis: 7,
    lastChecked: '2026-05-06T13:00:06Z',
  },
  platformData: { sources: ['availability'] },
  ...overrides,
});

describe('availabilityProbePresentation', () => {
  it('labels ICMP as ping so agentless rows explain the probe method plainly', () => {
    expect(getAvailabilityProbeMethodLabel({ protocol: 'icmp' })).toBe('Ping');
  });

  it('builds compact row evidence for a successful TCP availability probe', () => {
    vi.spyOn(Date, 'now').mockReturnValue(new Date('2026-05-06T13:00:20Z').getTime());

    const presentation = getAvailabilityProbePresentation(makeAvailabilityResource());

    expect(presentation).toMatchObject({
      methodLabel: 'TCP 1883',
      resultLabel: '7 ms',
      rowLabel: 'TCP 1883 - 7 ms - checked 14s ago',
    });
    expect(presentation?.detailLabel).toContain('TCP 1883 - 7 ms');
    expect(presentation?.toneClassName).toContain('emerald');
  });

  it('also reads availability evidence from platform data for live state rows', () => {
    vi.spyOn(Date, 'now').mockReturnValue(new Date('2026-05-06T13:00:20Z').getTime());

    const presentation = getAvailabilityProbePresentation(
      makeAvailabilityResource({
        availability: undefined,
        platformData: {
          sources: ['availability'],
          availability: {
            protocol: 'tcp',
            port: 6053,
            available: true,
            latencyMillis: 11,
            lastChecked: '2026-05-06T13:00:15Z',
          },
        },
      }),
    );

    expect(presentation).toMatchObject({
      methodLabel: 'TCP 6053',
      resultLabel: '11 ms',
      rowLabel: 'TCP 6053 - 11 ms - checked 5s ago',
    });
  });

  it('keeps failure evidence visible without exposing a full error as the row label', () => {
    vi.spyOn(Date, 'now').mockReturnValue(new Date('2026-05-06T13:00:20Z').getTime());

    const presentation = getAvailabilityProbePresentation(
      makeAvailabilityResource({
        status: 'offline',
        availability: {
          protocol: 'icmp',
          available: false,
          lastChecked: '2026-05-06T13:00:09Z',
          lastSuccess: '2026-05-06T12:51:20Z',
          consecutiveFailures: 3,
          failureThreshold: 2,
          lastError: 'icmp probe timed out',
        },
      }),
    );

    expect(presentation).toMatchObject({
      methodLabel: 'Ping',
      resultLabel: 'timed out',
      rowLabel: 'Ping - timed out - checked 11s ago',
    });
    expect(presentation?.detailLabel).toContain('3/2 failures');
    expect(presentation?.detailLabel).toContain('icmp probe timed out');
    expect(presentation?.toneClassName).toContain('red');
  });
});
