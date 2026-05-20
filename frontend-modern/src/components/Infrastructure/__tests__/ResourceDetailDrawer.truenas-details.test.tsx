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
});
