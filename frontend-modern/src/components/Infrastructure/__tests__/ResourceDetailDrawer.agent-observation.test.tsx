import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '../ResourceDetailDrawer';
import { resetAIRuntimeState } from '@/stores/aiRuntimeState';

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {}, state: { resources: [] } }),
}));
vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getFacetBundle: vi
      .fn()
      .mockResolvedValue({ capabilities: [], relationships: [], recentChanges: [] }),
  },
}));
vi.mock('@/api/ai', () => ({ AIAPI: { getResourceIntelligence: vi.fn().mockResolvedValue({}) } }));
vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: () => <div />,
  GuestDrawerHistoryRangeSelect: () => <div />,
}));
class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverMock);
beforeEach(resetAIRuntimeState);
afterEach(() => {
  cleanup();
  resetAIRuntimeState();
  vi.clearAllMocks();
});
const gib = 1024 ** 3;

describe('canonical native-power agent cards', () => {
  it.each([
    ['vm', 'paused'],
    ['vm', 'suspended'],
    ['vm', 'stopped'],
    ['vm', 'offline'],
    ['system-container', 'stopped'],
    ['system-container', 'offline'],
  ] as const)(
    'qualifies %s reports on %s without changing source or independent inventory',
    async (type, power) => {
      const initial: Resource = {
        id: 'guest-a',
        name: 'Guest A',
        displayName: 'Guest A',
        type,
        platformId: 'site-a',
        platformType: 'proxmox-pve',
        platformScopes: ['proxmox-pve'],
        sourceType: 'hybrid',
        status: 'online',
        lastSeen: 1792497600000,
        proxmox: { runtimeStatus: 'running' },
        agent: {
          stale: false,
          hostname: 'Guest A',
          agentVersion: 'fixture',
          networkInterfaces: [{ name: 'eth0', addresses: ['192.0.2.11/24'] }],
          disks: [{ mountpoint: '/data', total: 10 * gib, used: 9 * gib }],
          raid: [
            {
              device: '/dev/md0',
              name: 'data',
              level: 'raid1',
              state: 'clean, degraded, recovering',
              totalDevices: 2,
              activeDevices: 1,
              workingDevices: 1,
              failedDevices: 1,
              spareDevices: 0,
              rebuildPercent: 37,
              rebuildSpeed: '100M/sec',
            },
          ],
          sensors: {
            temperatureCelsius: { cpu: 42 },
            custom: [
              {
                id: 'service',
                name: 'Service',
                kind: 'boolean',
                value: 1,
                status: 'ok',
                observedAt: '2026-10-20T12:00:00Z',
              },
            ],
            smart: [{ device: '/dev/sda', temperature: 36 }],
          },
        },
      };
      const [value, setValue] = createSignal(initial);
      render(() => (
        <ResourceDetailDrawer resource={value()} presentation="table-row" initialShowHostDetails />
      ));
      const details = screen.getByTestId('resource-technical-details') as HTMLDetailsElement;
      details.open = true;
      fireEvent(details, new Event('toggle'));
      await vi.waitFor(() =>
        expect(screen.getByTestId('disks-card-mount-bar')).toBeInTheDocument(),
      );
      const host = screen.getByTestId('resource-host-details-section');
      expect(within(host).getByTestId('raid-card-array')).toHaveAttribute(
        'data-raid-reading',
        'current',
      );
      const before = structuredClone(value().agent);
      setValue({ ...value(), proxmox: { runtimeStatus: power } });
      const noun = type === 'vm' ? 'VM' : 'container';
      const reason = `Proxmox reports this ${noun} ${power === 'offline' ? 'stopped' : power}`;
      await vi.waitFor(() =>
        expect(within(host).getByTestId('disks-card-total')).toHaveTextContent('Last known 90%'),
      );
      expect(within(host).queryByTestId('disks-card-mount-bar')).toBeNull();
      expect(within(host).getByTestId('raid-card-array')).toHaveAttribute(
        'data-raid-reading',
        'last-known',
      );
      expect(within(host).getByTestId('raid-card-rebuild')).toHaveTextContent('Rebuild was at 37%');
      expect(within(host).getByText('42°C (last known)')).toBeInTheDocument();
      expect(within(host).getByText('36°C (last known)')).toBeInTheDocument();
      expect(within(host).getByText('Online (last known)')).toBeInTheDocument();
      expect(within(host).getByText('192.0.2.11/24')).toHaveClass('text-muted');
      expect(within(host).getAllByRole('status')).toHaveLength(5);
      for (const status of within(host).getAllByRole('status'))
        expect(status).toHaveTextContent(reason);
      expect(value().agent).toEqual(before);
      expect(value().status).toBe('online');
      expect(details.open).toBe(true);
      setValue({ ...value(), proxmox: { runtimeStatus: 'running' } });
      expect(within(host).getByTestId('disks-card-mount-bar')).toBeInTheDocument();
      expect(within(host).getByTestId('raid-card-array')).toHaveAttribute(
        'data-raid-reading',
        'current',
      );
      expect(within(host).getByText('42°C')).toBeInTheDocument();
      expect(within(host).getByText('Online')).toBeInTheDocument();
      expect(within(host).queryAllByRole('status')).toHaveLength(0);
    },
  );
});
