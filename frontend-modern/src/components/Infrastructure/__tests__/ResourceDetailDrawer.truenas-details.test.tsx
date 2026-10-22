import { describe, expect, it, vi } from 'vitest';
import { render, within } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    activeAlerts: {},
    state: { pmg: [] as any[] },
    connected: () => true,
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: vi.fn(),
  }),
  useDarkMode: () => () => false,
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getFacetBundle: vi.fn().mockResolvedValue({
      capabilities: [],
      relationships: [],
      recentChanges: [],
    }),
  },
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getResourceIntelligence: vi.fn().mockResolvedValue({
      resource_id: 'resource-1',
      health: {
        score: 100,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Stable',
      },
      recent_changes: [],
      dependencies: [],
      dependents: [],
      correlations: [],
      note_count: 0,
    }),
  },
}));

const baseResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: 'resource-1',
    type: 'vm',
    name: 'resource-1',
    displayName: 'resource-1',
    platformId: 'truenas-main',
    platformType: 'truenas',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    platformData: { sources: ['truenas'] },
    ...overrides,
  }) as Resource;

const details = (getByTestId: (id: string) => HTMLElement) =>
  within(getByTestId('resource-technical-details'));

const worthChecking = (queryByTestId: (id: string) => HTMLElement | null) => {
  const list = queryByTestId('resource-drawer-exceptions');
  return list ? within(list) : null;
};

// TrueNAS sections sit in the Overview's one Details disclosure as plain
// tables (no second "Show TrueNAS" toggle), their problems are lifted to
// "Worth checking", and a pool's or disk's health stays in view as a card.
describe('ResourceDetailDrawer TrueNAS details', () => {
  it("lifts a TrueNAS system's storage problems and keeps the rest under Details", () => {
    const resource = baseResource({
      id: 'truenas-system-1',
      type: 'agent',
      displayName: 'truenas-main',
      uptime: 42 * 86_400,
      truenas: {
        hostname: 'truenas-main',
        version: 'TrueNAS-SCALE-24.10.2',
        storageRisk: { level: 'warning' },
        storageRiskSummary: 'ZFS pool archive is DEGRADED',
        storagePostureSummary: 'One pool needs attention',
        protectionReduced: true,
        protectionSummary: 'Snapshots are current but replication is degraded',
        services: [
          { id: '1', service: 'smb', enabled: true, state: 'RUNNING', pids: [2418, 2420] },
          { id: '2', service: 'nfs', enabled: true, state: 'RUNNING', pids: [2501] },
          { id: '3', service: 'ssh', enabled: false, state: 'STOPPED' },
          { id: '4', service: 'smartd', enabled: true, state: 'STOPPED' },
        ],
      },
    });

    const { getByTestId, queryByRole, queryByTestId } = render(() => (
      <ResourceDetailDrawer resource={resource} presentation="table-row" />
    ));

    expect(queryByRole('button', { name: 'Show TrueNAS' })).toBeNull();
    const lifted = worthChecking(queryByTestId);
    expect(lifted?.getByText('ZFS pool archive is DEGRADED')).toBeInTheDocument();
    expect(
      lifted?.getByText('Snapshots are current but replication is degraded'),
    ).toBeInTheDocument();
    // An enabled service that is stopped is worth a look.
    expect(lifted?.getByText('Stopped')).toBeInTheDocument();
    expect(lifted?.getByText('1 service')).toBeInTheDocument();

    const inventory = details(getByTestId);
    expect(inventory.getByText('System')).toBeInTheDocument();
    expect(inventory.getByText('TrueNAS-SCALE-24.10.2')).toBeInTheDocument();
    expect(inventory.getByText('SMB, NFS, SSH, SMART')).toBeInTheDocument();
    // Lifted rows move rather than repeat.
    expect(inventory.queryByText('ZFS pool archive is DEGRADED')).toBeNull();
  });

  it('keeps native TrueNAS VM detail under Details', () => {
    const resource = baseResource({
      id: 'truenas-vm-1',
      type: 'vm',
      displayName: 'ubuntu-build',
      truenas: {
        vm: {
          name: 'ubuntu-build',
          state: 'RUNNING',
          vcpus: 4,
          memoryBytes: 8 * 1024 ** 3,
          deviceCount: 3,
          diskCount: 1,
          nicCount: 1,
          bootloader: 'UEFI',
          secureBoot: true,
          trustedPlatformModule: true,
        },
      },
    });

    const { getByTestId, queryByTestId } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(queryByTestId('resource-drawer-exceptions')).toBeNull();
    expect((getByTestId('resource-technical-details') as HTMLDetailsElement).open).toBe(false);
    const inventory = details(getByTestId);
    expect(inventory.getByText('Compute')).toBeInTheDocument();
    expect(inventory.getByText('Runtime')).toBeInTheDocument();
    expect(inventory.getByText('Devices')).toBeInTheDocument();
    expect(inventory.getByText('Flags')).toBeInTheDocument();
    expect(inventory.getByText('Bootloader')).toBeInTheDocument();
    expect(inventory.getByText('UEFI')).toBeInTheDocument();
    expect(inventory.getAllByText('Enabled').length).toBeGreaterThan(1);
  });

  it('lifts an available TrueNAS app update without Docker runtime controls', () => {
    const resource = baseResource({
      id: 'truenas-app-1',
      type: 'app-container',
      displayName: 'nextcloud',
      truenas: {
        app: {
          name: 'nextcloud',
          state: 'RUNNING',
          humanVersion: '29.0.0',
          containerCount: 2,
          upgradeAvailable: true,
          imageUpdatesAvailable: false,
          usedHostIps: ['0.0.0.0'],
          usedPorts: [
            {
              containerPort: 443,
              protocol: 'tcp',
              hostPorts: [{ hostIp: '0.0.0.0', hostPort: 30443 }],
            },
          ],
          networks: [{ name: 'ix-nextcloud_default' }],
          volumes: [{ source: '/mnt/tank/apps/nextcloud', destination: '/data' }],
          images: ['nextcloud:29'],
        },
      },
    });

    const { getByTestId, queryByTestId, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(queryByText('Docker runtime')).toBeNull();
    const lifted = worthChecking(queryByTestId);
    expect(lifted?.getByText('App updates')).toBeInTheDocument();
    expect(lifted?.getByText('Available')).toBeInTheDocument();

    const inventory = details(getByTestId);
    expect(inventory.getByText('App')).toBeInTheDocument();
    expect(inventory.getByText('Networking')).toBeInTheDocument();
    expect(inventory.getByText('0.0.0.0:30443 -> 443/tcp')).toBeInTheDocument();
    expect(inventory.getByText('/mnt/tank/apps/nextcloud -> /data')).toBeInTheDocument();
  });

  it('keeps native TrueNAS share detail under Details', () => {
    const resource = baseResource({
      id: 'truenas-share-1',
      type: 'network-share',
      displayName: 'Media',
      truenas: {
        share: {
          name: 'Media',
          protocol: 'SMB',
          dataset: 'tank/media',
          path: '/mnt/tank/media',
          enabled: true,
          readOnly: false,
          browsable: true,
          accessBasedEnumeration: true,
          auditEnabled: true,
          aliases: ['media'],
        },
      },
    });

    const { getByTestId } = render(() => <ResourceDetailDrawer resource={resource} />);

    const inventory = details(getByTestId);
    expect(inventory.getByText('Share')).toBeInTheDocument();
    expect(inventory.getByText('Access')).toBeInTheDocument();
    expect(inventory.getByText('Clients')).toBeInTheDocument();
    expect(inventory.getByText('/mnt/tank/media')).toBeInTheDocument();
    expect(inventory.getByText('Read/write')).toBeInTheDocument();
    expect(inventory.getByText('media')).toBeInTheDocument();
  });

  it("shows a degraded pool's health in front and its ZFS errors under Worth checking", () => {
    const resource = baseResource({
      id: 'truenas-pool-1',
      type: 'storage',
      displayName: 'archive',
      status: 'degraded',
      platformScopes: ['truenas'],
      disk: {
        current: 35.2,
        used: 25.3 * 1024 ** 4,
        total: 72 * 1024 ** 4,
      },
      childCount: 4,
      storage: {
        type: 'zfs-pool',
        topology: 'pool',
        platform: 'truenas',
        protection: 'zfs',
        zfsPoolState: 'DEGRADED',
        zfsReadErrors: 1,
        zfsChecksumErrors: 2,
        risk: {
          level: 'warning',
          reasons: [
            {
              code: 'zfs_pool_state',
              severity: 'warning',
              summary: 'ZFS pool archive is DEGRADED',
            },
          ],
        },
        riskSummary: 'ZFS pool archive is DEGRADED',
        protectionReduced: true,
        protectionSummary: 'ZFS pool archive is DEGRADED',
      },
    });

    const { getByTestId, queryByTestId } = render(() => (
      <ResourceDetailDrawer resource={resource} presentation="table-row" />
    ));

    // The pool's own summary is stated once, by the alerts block.
    expect(
      within(getByTestId('drawer-attention-section')).getByText('ZFS pool archive is DEGRADED'),
    ).toBeInTheDocument();
    const lifted = worthChecking(queryByTestId);
    expect(lifted?.getByText('ZFS errors')).toBeInTheDocument();
    expect(lifted?.getByText('Read 1 · Write 0 · Checksum 2')).toBeInTheDocument();
    expect(lifted?.queryByText('ZFS pool archive is DEGRADED')).toBeNull();

    // Pool health is a content card outside the closed Details.
    const cards = getByTestId('resource-drawer-cards');
    expect(cards.closest('[data-testid="resource-technical-details"]')).toBeNull();
    expect(within(cards).getByText('Health')).toBeInTheDocument();
    expect(within(cards).getByText('DEGRADED')).toBeInTheDocument();

    const inventory = details(getByTestId);
    expect(inventory.getByText('Storage')).toBeInTheDocument();
    expect(inventory.getByText('Capacity')).toBeInTheDocument();
    expect(inventory.getByText('ZFS')).toBeInTheDocument();
  });

  it("shows a disk's health and SMART readings in front", () => {
    const resource = baseResource({
      id: 'truenas-disk-1',
      type: 'physical_disk',
      displayName: 'sdc',
      platformScopes: ['truenas'],
      physicalDisk: {
        devPath: '/dev/sdc',
        model: 'WD Red Pro',
        serial: 'WD-WX12A3456',
        diskType: 'sata',
        sizeBytes: 24 * 1024 ** 4,
        health: 'PASSED',
        smart: { powerOnHours: 10_240, reallocatedSectors: 4 },
      },
    });

    const { getByTestId, queryByTestId } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    const cards = within(getByTestId('resource-drawer-cards'));
    // The Health card and its Health row.
    expect(cards.getAllByText('Health')).toHaveLength(2);
    expect(cards.getByText('SMART')).toBeInTheDocument();
    expect(cards.getByText('Passed')).toBeInTheDocument();
    expect(worthChecking(queryByTestId)?.getByText('Reallocated')).toBeInTheDocument();
    expect(details(getByTestId).getByText('WD-WX12A3456')).toBeInTheDocument();
  });
});
