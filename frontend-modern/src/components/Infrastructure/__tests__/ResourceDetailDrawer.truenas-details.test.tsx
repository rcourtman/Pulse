import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, within } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    state: { pmg: [] as any[] },
    connected: () => true,
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: vi.fn(),
  }),
  useDarkMode: () => () => false,
}));

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab" />,
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

describe('ResourceDetailDrawer TrueNAS details', () => {
  it('opens native TrueNAS system detail immediately for inline platform rows', () => {
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

    const { getByRole, getByTestId } = render(() => (
      <ResourceDetailDrawer
        resource={resource}
        presentation="table-row"
        initialShowTrueNASDetails
      />
    ));

    expect(getByRole('button', { name: 'Hide TrueNAS' })).toBeInTheDocument();
    const section = within(getByTestId('resource-truenas-details-section'));
    expect(section.getByText('System')).toBeInTheDocument();
    expect(section.getByText('Storage Health')).toBeInTheDocument();
    expect(section.getAllByText('Services').length).toBeGreaterThan(1);
    expect(section.getByText('TrueNAS-SCALE-24.10.2')).toBeInTheDocument();
    expect(section.getByText('SMB, NFS, SSH, SMART')).toBeInTheDocument();
    expect(
      section.getByText('Snapshots are current but replication is degraded'),
    ).toBeInTheDocument();
  });

  it('renders native TrueNAS VM detail only after the TrueNAS section is expanded', () => {
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

    const { getByText, getByRole, getByTestId, getAllByText, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getAllByText('TrueNAS').length).toBeGreaterThan(0);
    expect(getByText('Running, 4 vCPU, 8.00 GB, 3 devices')).toBeInTheDocument();
    expect(queryByText('Bootloader')).toBeNull();

    fireEvent.click(getByRole('button', { name: 'Show TrueNAS' }));

    const section = within(getByTestId('resource-truenas-details-section'));
    expect(section.getByText('Compute')).toBeInTheDocument();
    expect(section.getByText('Runtime')).toBeInTheDocument();
    expect(section.getByText('Devices')).toBeInTheDocument();
    expect(section.getByText('Flags')).toBeInTheDocument();
    expect(section.getByText('Bootloader')).toBeInTheDocument();
    expect(section.getByText('UEFI')).toBeInTheDocument();
    expect(section.getByText('Secure boot')).toBeInTheDocument();
    expect(section.getAllByText('Enabled').length).toBeGreaterThan(1);
  });

  it('renders native TrueNAS app detail without Docker runtime controls', () => {
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

    const { getByText, getByRole, getByTestId, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Running, 2 containers, 1 port, 1 update')).toBeInTheDocument();
    expect(queryByText('Docker runtime')).toBeNull();

    fireEvent.click(getByRole('button', { name: 'Show TrueNAS' }));

    const section = within(getByTestId('resource-truenas-details-section'));
    expect(section.getByText('App')).toBeInTheDocument();
    expect(section.getByText('Networking')).toBeInTheDocument();
    expect(section.getByText('Storage')).toBeInTheDocument();
    expect(section.getByText('0.0.0.0:30443 -> 443/tcp')).toBeInTheDocument();
    expect(section.getByText('/mnt/tank/apps/nextcloud -> /data')).toBeInTheDocument();
  });

  it('can open native TrueNAS detail immediately for inline platform rows', () => {
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
          usedPorts: [
            {
              containerPort: 443,
              protocol: 'tcp',
              hostPorts: [{ hostIp: '0.0.0.0', hostPort: 30443 }],
            },
          ],
          volumes: [{ source: '/mnt/tank/apps/nextcloud', destination: '/data' }],
        },
      },
    });

    const { getByRole, getByTestId } = render(() => (
      <ResourceDetailDrawer
        resource={resource}
        presentation="table-row"
        initialShowTrueNASDetails
      />
    ));

    const summary = getByTestId('resource-summary-section');
    expect(summary.querySelector('table')).toBeTruthy();
    expect(getByTestId('resource-current-state-section').tagName).toBe('TBODY');
    expect(getByTestId('resource-identity-section').tagName).toBe('TBODY');
    expect(summary.querySelector('[class*="shadow-sm"]')).toBeNull();
    expect(getByRole('button', { name: 'Hide TrueNAS' })).toBeInTheDocument();
    const section = within(getByTestId('resource-truenas-details-section'));
    expect(getByTestId('resource-truenas-details-section').querySelector('table')).toBeTruthy();
    expect(
      getByTestId('resource-truenas-details-section').querySelector('[class*="cyan"]'),
    ).toBeNull();
    expect(section.getByText('App')).toBeInTheDocument();
    expect(section.getByText('0.0.0.0:30443 -> 443/tcp')).toBeInTheDocument();
    expect(section.getByText('/mnt/tank/apps/nextcloud -> /data')).toBeInTheDocument();
  });

  it('renders native TrueNAS share detail from SMB and NFS inventory', () => {
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

    const { getByText, getByRole, getByTestId } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('SMB, Enabled, tank/media, Read/write')).toBeInTheDocument();

    fireEvent.click(getByRole('button', { name: 'Show TrueNAS' }));

    const section = within(getByTestId('resource-truenas-details-section'));
    expect(section.getByText('Share')).toBeInTheDocument();
    expect(section.getByText('Access')).toBeInTheDocument();
    expect(section.getByText('Clients')).toBeInTheDocument();
    expect(section.getByText('/mnt/tank/media')).toBeInTheDocument();
    expect(section.getByText('Read/write')).toBeInTheDocument();
    expect(section.getByText('media')).toBeInTheDocument();
  });

  it('opens native TrueNAS storage detail immediately for inline storage rows', () => {
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

    const { getByRole, getByTestId } = render(() => (
      <ResourceDetailDrawer
        resource={resource}
        presentation="table-row"
        initialShowTrueNASDetails
      />
    ));

    expect(getByRole('button', { name: 'Hide TrueNAS' })).toBeInTheDocument();
    const section = within(getByTestId('resource-truenas-details-section'));
    expect(getByTestId('resource-truenas-details-section').querySelector('table')).toBeTruthy();
    expect(section.getByText('Storage')).toBeInTheDocument();
    expect(section.getByText('Capacity')).toBeInTheDocument();
    expect(section.getByText('Health')).toBeInTheDocument();
    expect(section.getByText('ZFS')).toBeInTheDocument();
    expect(section.getAllByText('ZFS pool archive is DEGRADED')).toHaveLength(3);
  });
});
