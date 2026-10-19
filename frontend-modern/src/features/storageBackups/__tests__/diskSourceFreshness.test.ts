import { renderHook } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { useDiskDetailModel } from '@/components/Storage/useDiskDetailModel';
import {
  extractPhysicalDiskPresentationData,
  getPhysicalDiskHealthStatus,
  getPhysicalDiskLifeTextClass,
  getPhysicalDiskNormalizedHealth,
  getPhysicalDiskTemperaturePresentation,
} from '../diskPresentation';
import {
  getDiskAttributeCardValueTextClass,
  getDiskDetailAttributeCards,
} from '../diskDetailPresentation';

const disk = (health?: Resource['health']): Resource => ({
  id: 'disk-101',
  type: 'physical_disk',
  name: 'Guest SATA',
  displayName: 'Guest SATA',
  parentId: 'guest-101',
  platformId: 'fixture-a',
  platformType: 'proxmox-pve',
  sourceType: 'agent',
  status: 'online',
  lastSeen: 1,
  health,
  metricsTarget: { resourceType: 'disk', resourceId: 'agent-101:sda' },
  physicalDisk: {
    model: 'Guest SATA',
    devPath: '/dev/sda',
    diskType: 'ssd',
    health: 'PASSED',
    temperature: 72,
    wearout: 99,
    collection: {
      temperature: { state: 'available', source: 'smartctl' },
      io: { state: 'available' },
    },
    smart: { powerOnHours: 100, reallocatedSectors: 0, pendingSectors: 0, udmaCrcErrors: 2 },
  },
});
const stale: Resource['health'] = {
  verdict: 'stale',
  reasons: [{ code: 'telemetry_stale', detail: '9m' }],
};

describe('canonical disk source freshness', () => {
  it.each([
    stale,
    { verdict: 'stale', reasons: [] },
    {
      verdict: 'critical',
      reasons: [{ code: 'critical_alert' }, { code: 'telemetry_stale', detail: '9m' }],
    },
  ] as Resource['health'][])(
    'does not turn a successful inventory read with %j into current SMART health',
    (health) => {
      const data = extractPhysicalDiskPresentationData(disk(health));
      const status = getPhysicalDiskHealthStatus(data);
      expect(status.label).toBe('Last known');
      expect(status.summary).toContain('current disk health is unconfirmed');
      expect(status.tone).toBe('text-muted');
      expect(getPhysicalDiskNormalizedHealth(disk(health), data)).toBe('unknown');
      expect(getPhysicalDiskTemperaturePresentation(data)).toEqual({
        label: '72°C',
        current: false,
        title: 'Last known reading, not current: disk source stopped reporting',
      });
    },
  );

  it('retains recorded SMART failure and its attention filter without claiming a current failure', () => {
    const resource = disk(stale);
    resource.physicalDisk!.health = 'FAILED';
    const data = extractPhysicalDiskPresentationData(resource);
    expect(getPhysicalDiskHealthStatus(data)).toMatchObject({ label: 'Last known risk' });
    expect(getPhysicalDiskHealthStatus(data).summary).toContain('Last reported: Replace Now.');
    expect(getPhysicalDiskHealthStatus(data).summary).toContain(
      'current disk health is unconfirmed',
    );
    expect(getPhysicalDiskNormalizedHealth(resource, data)).toBe('critical');
  });

  it('keeps recorded non-zero SMART counters visible as a qualified warning', () => {
    const resource = disk(stale);
    resource.physicalDisk!.smart!.pendingSectors = 3;
    const data = extractPhysicalDiskPresentationData(resource);
    expect(getPhysicalDiskHealthStatus(data).label).toBe('Last known risk');
    expect(getPhysicalDiskHealthStatus(data).summary).toContain(
      'SMART counters indicate elevated risk',
    );
    expect(getPhysicalDiskNormalizedHealth(resource, data)).toBe('warning');
  });

  it('qualifies every retained SMART card and endurance colour, not just temperature', () => {
    const data = extractPhysicalDiskPresentationData(disk(stale));
    const cards = getDiskDetailAttributeCards(data);
    expect(cards.length).toBeGreaterThan(3);
    expect(cards.every((card) => card.lastKnown)).toBe(true);
    expect(new Set(cards.map(getDiskAttributeCardValueTextClass))).toEqual(new Set(['text-muted']));
    expect(cards.find((card) => card.label === 'Last known temperature')?.value).toBe('72°C');
    expect(cards.find((card) => card.label === 'CRC Errors')?.value).toBe('2');
    expect(getPhysicalDiskLifeTextClass(data)).toBe('text-muted');
  });

  it('keeps historical identity but withdraws live I/O until the canonical source recovers', () => {
    const [resource, setResource] = createSignal(disk(stale));
    const { result } = renderHook(() => useDiskDetailModel({ disk: resource }));
    expect(result.liveIOAvailable()).toBe(false);
    expect(result.historyResourceId()).toBe('agent-101:sda');
    expect(result.healthPresentation().label).toBe('Last known');
    setResource(disk({ verdict: 'ok', reasons: [] }));
    expect(result.liveIOAvailable()).toBe(true);
    expect(result.healthPresentation().label).toBe('Running Hot');
    expect(result.attributeCards().some((card) => card.lastKnown)).toBe(false);
  });

  it('does not manufacture a browser-side expiry for legacy or fresh source metadata', () => {
    for (const health of [
      undefined,
      { verdict: 'ok', reasons: [] },
      { verdict: 'critical', reasons: [{ code: 'critical_alert' }] },
    ] as Resource['health'][]) {
      const data = extractPhysicalDiskPresentationData(disk(health));
      expect(getPhysicalDiskHealthStatus(data).label).toBe('Running Hot');
      expect(getPhysicalDiskTemperaturePresentation(data)?.current).toBe(true);
    }
  });

  it('does not treat one unavailable field as expiry of all disk evidence', () => {
    const resource = disk();
    resource.physicalDisk!.collection!.temperature = {
      state: 'unavailable',
      reason: 'disk is in standby',
    };
    const data = extractPhysicalDiskPresentationData(resource);
    expect(getPhysicalDiskHealthStatus(data).label).toBe('Healthy');
    expect(getPhysicalDiskTemperaturePresentation(data)?.current).toBe(false);
    expect(
      getDiskDetailAttributeCards(data).find((card) => card.label === 'CRC Errors')?.lastKnown,
    ).not.toBe(true);
  });

  it('leaves the registry payload and per-field collection provenance unchanged', () => {
    const resource = disk(stale);
    const before = JSON.stringify(resource);
    const data = extractPhysicalDiskPresentationData(resource);
    getPhysicalDiskHealthStatus(data);
    getDiskDetailAttributeCards(data);
    expect(JSON.stringify(resource)).toBe(before);
    expect(data.collection).toBe(resource.physicalDisk!.collection);
  });
});
