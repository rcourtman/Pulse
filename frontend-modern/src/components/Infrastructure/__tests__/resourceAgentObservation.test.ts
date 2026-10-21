import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getResourceAgentReadingsLastKnownReason,
  buildTemperatureRows,
} from '../resourceDetailMappers';

const resource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'guest-a',
  name: 'Guest A',
  type: 'vm',
  platformId: 'site-a',
  platformType: 'proxmox-pve',
  platformScopes: ['proxmox-pve'],
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: 1792497600000,
  proxmox: { runtimeStatus: 'running' },
  agent: { stale: false },
  ...overrides,
});

describe('canonical agent observations', () => {
  it.each(['paused', 'suspended', 'stopped', 'offline', ' PAUSED '])(
    'does not let a healthy row or unexpired lease renew a VM report at %s',
    (runtimeStatus) => {
      const value = resource({ proxmox: { runtimeStatus } });
      const before = structuredClone(value);
      const power = runtimeStatus.trim().toLowerCase();
      expect(getResourceAgentReadingsLastKnownReason(value)).toBe(
        `Proxmox reports this VM ${power === 'offline' ? 'stopped' : power}`,
      );
      expect(value).toEqual(before);
    },
  );
  it.each(['stopped', 'offline', ' STOPPED '])(
    'qualifies stopped in-LXC reports at %s',
    (runtimeStatus) => {
      expect(
        getResourceAgentReadingsLastKnownReason(
          resource({ type: 'system-container', proxmox: { runtimeStatus } }),
        ),
      ).toBe('Proxmox reports this container stopped');
    },
  );
  it.each([
    'running',
    'online',
    'healthy',
    'unknown',
    'starting',
    'stopping',
    'private-state',
    '',
    undefined,
  ])('does not invent native power semantics at %s', (runtimeStatus) => {
    expect(
      getResourceAgentReadingsLastKnownReason(resource({ proxmox: { runtimeStatus } })),
    ).toBeUndefined();
  });
  it.each(['agent', 'host', 'node', 'docker-host', 'app-container', 'pod'] as Resource['type'][])(
    'does not turn a %s host or workload into a guest power claim',
    (type) => {
      expect(
        getResourceAgentReadingsLastKnownReason(
          resource({ type, proxmox: { runtimeStatus: 'stopped' } }),
        ),
      ).toBeUndefined();
    },
  );
  it.each(['vmware-vsphere', 'incus', 'agent'])(
    'keeps an explicit %s scope separate from the Proxmox label',
    (scope) => {
      expect(
        getResourceAgentReadingsLastKnownReason(
          resource({ platformScopes: [scope], proxmox: { runtimeStatus: 'stopped' } }),
        ),
      ).toBeUndefined();
    },
  );
  it('uses canonical platformType when scopes are absent, but never the row status as native power', () => {
    expect(
      getResourceAgentReadingsLastKnownReason(
        resource({ platformScopes: undefined, proxmox: { runtimeStatus: 'stopped' } }),
      ),
    ).toBe('Proxmox reports this VM stopped');
    expect(
      getResourceAgentReadingsLastKnownReason(resource({ status: 'offline', proxmox: undefined })),
    ).toBeUndefined();
  });
  it('retains expired-agent qualification outside guest power, including physical hosts', () => {
    expect(
      getResourceAgentReadingsLastKnownReason(resource({ type: 'agent', agent: { stale: true } })),
    ).toBe('host agent stopped reporting');
    expect(
      getResourceAgentReadingsLastKnownReason(
        resource({ proxmox: { runtimeStatus: 'paused' }, agent: { stale: true } }),
      ),
    ).toBe('Proxmox reports this VM paused');
  });
  it('does not invent paused/suspended LXC semantics', () => {
    for (const runtimeStatus of ['paused', 'suspended'])
      expect(
        getResourceAgentReadingsLastKnownReason(
          resource({ type: 'system-container', proxmox: { runtimeStatus } }),
        ),
      ).toBeUndefined();
  });
  it('qualifies in-guest SMART without changing independent host/disk collection', () => {
    const sensors = {
      temperatureCelsius: { cpu: 42 },
      smart: [
        {
          device: '/dev/sda',
          temperature: 36,
          collection: { temperature: { state: 'available' as const } },
        },
      ],
    };
    const before = structuredClone(sensors);
    expect(
      buildTemperatureRows(sensors, { lastKnownReason: 'host agent stopped reporting' }).at(-1)
        ?.value,
    ).toBe('36°C');
    const rows = buildTemperatureRows(sensors, {
      lastKnownReason: 'Proxmox reports this VM paused',
      inGuestLastKnownReason: 'Proxmox reports this VM paused',
    });
    expect(rows.map((row) => row.value)).toEqual(['42°C (last known)', '36°C (last known)']);
    expect(rows.at(-1)?.valueTitle).toContain('Proxmox reports this VM paused');
    expect(sensors).toEqual(before);
  });
});
