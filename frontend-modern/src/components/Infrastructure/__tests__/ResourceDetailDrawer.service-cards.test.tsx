import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, within } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

const expandPlatformDetails = (getByTestId: (id: string) => HTMLElement): void => {
  const details = getByTestId('resource-platform-details') as HTMLDetailsElement;
  details.open = true;
  fireEvent(details, new Event('toggle'));
};

const wsState = vi.hoisted(() => ({ pmg: [] as any[] }));
const reconnectSpy = vi.hoisted(() => vi.fn());

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    state: wsState,
    connected: () => true,
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: reconnectSpy,
  }),
  useDarkMode: () => () => false,
}));

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab" />,
}));

vi.mock('@/components/Workloads/StackedDiskBar', () => ({
  StackedDiskBar: () => <div data-testid="stacked-disk-bar" />,
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
    getSettings: vi.fn().mockResolvedValue({ discovery_enabled: false }),
    getResourceIntelligence: vi.fn().mockResolvedValue({
      resource_id: 'resource-1',
      health: {
        score: 92,
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

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'host-1',
  displayName: 'host-1',
  platformId: 'host-1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['proxmox'] },
  ...overrides,
});

describe('ResourceDetailDrawer service cards', () => {
  it('renders PBS card with compact summary and job breakdown section', () => {
    const resource = baseResource({
      id: 'pbs-1',
      type: 'pbs',
      name: 'pbs-main',
      displayName: 'PBS Main',
      platformId: '192.168.0.8',
      platformType: 'proxmox-pbs',
      platformData: {
        sources: ['pbs'],
        pbs: {
          hostname: 'pbs-main.local',
          connectionHealth: 'online',
          datastoreCount: 2,
          backupJobCount: 3,
        },
      },
    });

    const { getByText, getByRole, getByTestId, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expandPlatformDetails(getByTestId);
    expect(getByText('Service')).toBeInTheDocument();
    expect(getByText('2 datastores · 3 jobs')).toBeInTheDocument();
    expect(getByText('Platform ID')).toBeInTheDocument();
    expect(queryByText('PBS Service')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show service' }));
    expect(getByTestId('resource-service-details-section').querySelector('.mt-3.grid')).toBeNull();
    const serviceDetails = within(getByTestId('resource-service-details-section'));
    expect(serviceDetails.getByText('PBS')).toBeInTheDocument();
    expect(serviceDetails.queryByText('Connection')).toBeNull();
    expect(serviceDetails.getAllByText('State').length).toBeGreaterThan(0);
    expect(serviceDetails.getAllByText('pbs-main.local').length).toBeGreaterThan(0);
    expect(queryByText('Backup summary')).toBeNull();
    expect(queryByText('Job breakdown')).toBeNull();
    expect(queryByText('Types')).toBeNull();
    expect(queryByText('Show job detail')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show jobs' }));
    expect(getByText('Datastores')).toBeInTheDocument();
    expect(getByText('Jobs')).toBeInTheDocument();
    expect(getByText('Types')).toBeInTheDocument();
    // The legacy "Open Recovery Events" cross-jump was retired with the
    // standalone Recovery surface; PBS detail no longer surfaces it.
    expect(queryByText(/open recovery events/i)).toBeNull();
  });

  it('surfaces active PBS tasks before and inside job detail', () => {
    const resource = baseResource({
      id: 'pbs-2',
      type: 'pbs',
      name: 'pbs-active',
      displayName: 'PBS Active',
      platformId: '192.168.0.9',
      platformType: 'proxmox-pbs',
      platformData: {
        sources: ['pbs'],
        pbs: {
          hostname: 'pbs-active.local',
          connectionHealth: 'online',
          datastoreCount: 2,
          backupJobCount: 2,
          syncJobCount: 1,
          verifyJobCount: 1,
          backupJobs: [
            {
              id: 'backup-nightly',
              store: 'fast',
              type: 'vm',
              vmid: '100',
              lastBackup: '',
              nextRun: '',
              status: 'running',
              error: '',
            },
            {
              id: 'backup-weekly',
              store: 'archive',
              type: 'ct',
              vmid: '200',
              lastBackup: '',
              nextRun: '',
              status: 'ok',
              error: '',
            },
          ],
          syncJobs: [
            {
              id: 'sync-remote',
              store: 'fast',
              remote: 'offsite',
              status: 'queued',
              lastSync: '',
              nextRun: '',
              error: '',
            },
          ],
          verifyJobs: [
            {
              id: 'verify-1',
              store: 'fast',
              status: 'ok',
              lastVerify: '',
              nextRun: '',
              error: '',
            },
          ],
        },
      },
    });

    const { getByText, getByRole, getByTestId } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expandPlatformDetails(getByTestId);
    expect(getByText('2 datastores · 2 active tasks')).toBeInTheDocument();
    fireEvent.click(getByRole('button', { name: 'Show service' }));
    const serviceDetails = within(getByTestId('resource-service-details-section'));
    expect(serviceDetails.getByText('Active tasks')).toBeInTheDocument();
    expect(serviceDetails.getByText('2')).toBeInTheDocument();

    fireEvent.click(getByRole('button', { name: 'Show jobs' }));
    const activeTasks = within(getByTestId('pbs-active-tasks'));
    expect(activeTasks.getByText('Backup backup-nightly')).toBeInTheDocument();
    expect(activeTasks.getByText('fast · VM 100')).toBeInTheDocument();
    expect(activeTasks.getByText('Running')).toBeInTheDocument();
    expect(activeTasks.getByText('Sync sync-remote')).toBeInTheDocument();
    expect(activeTasks.getByText('fast · Remote offsite')).toBeInTheDocument();
    expect(activeTasks.getByText('Queued')).toBeInTheDocument();
  });

  it('renders merged agent hardware for a standalone PBS host', () => {
    const resource = baseResource({
      id: 'pbs-agent-1',
      type: 'pbs',
      name: 'pbs-bare-metal',
      displayName: 'PBS Bare Metal',
      platformId: 'pbs-bare-metal',
      platformType: 'proxmox-pbs',
      sourceType: 'hybrid',
      memory: { current: 50, total: 16_000, used: 8_000, free: 8_000 },
      platformData: {
        sources: ['pbs', 'agent'],
        pbs: {
          hostname: 'pbs-bare-metal',
          connectionHealth: 'online',
          datastoreCount: 1,
        },
        agent: {
          agentId: 'agent-pbs-1',
          agentVersion: '6.4.0',
          hostname: 'pbs-bare-metal',
          osName: 'Debian GNU/Linux',
          osVersion: '13',
          kernelVersion: '6.12.0-pve',
          architecture: 'amd64',
          cpuCount: 8,
          networkInterfaces: [{ name: 'eno1', addresses: ['192.0.2.10'] }],
          disks: [{ mountpoint: '/', total: 10_000, used: 4_000, free: 6_000 }],
          sensors: { temperatureCelsius: { cpu_package: 61 } },
        },
      },
    });

    const { getByTestId } = render(() => (
      <ResourceDetailDrawer resource={resource} initialShowHostDetails />
    ));

    expandPlatformDetails(getByTestId);
    const hostDetails = within(getByTestId('resource-host-details-section'));
    expect(hostDetails.getByText('System')).toBeInTheDocument();
    expect(hostDetails.getByText('Hardware')).toBeInTheDocument();
    expect(hostDetails.getByText('Network')).toBeInTheDocument();
    expect(hostDetails.getByText('Disks')).toBeInTheDocument();
    expect(hostDetails.getByText('Thermals')).toBeInTheDocument();
    expect(hostDetails.getByText('Debian GNU/Linux 13')).toBeInTheDocument();
    expect(hostDetails.getByText('eno1')).toBeInTheDocument();
    expect(hostDetails.getByText('192.0.2.10')).toBeInTheDocument();
  });

  it('renders PMG card with compact summary and queue/mail breakdown sections', () => {
    const resource = baseResource({
      id: 'pmg-1',
      type: 'pmg',
      name: 'pmg-main',
      displayName: 'PMG Main',
      platformId: '192.168.0.25',
      platformType: 'proxmox-pmg',
      platformData: {
        sources: ['pmg'],
        pmg: {
          hostname: 'pmg-main.local',
          connectionHealth: 'online',
          nodeCount: 1,
          lastUpdated: '2026-03-19T23:00:00Z',
          queueTotal: 519,
          queueDeferred: 12,
          queueHold: 4,
          mailCountTotal: 1200,
          spamIn: 32,
          virusIn: 2,
        },
      },
    });

    const { getByText, getByRole, getByTestId, queryByRole, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expandPlatformDetails(getByTestId);
    expect(getByText('Service')).toBeInTheDocument();
    expect(getByText('519 queued messages · 16 delayed messages')).toBeInTheDocument();
    expect(getByText('Platform ID')).toBeInTheDocument();
    expect(queryByText('Mail Gateway')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show service' }));
    expect(getByTestId('resource-service-details-section').querySelector('.mt-3.grid')).toBeNull();
    const serviceDetails = within(getByTestId('resource-service-details-section'));
    expect(serviceDetails.getByText('PMG')).toBeInTheDocument();
    expect(serviceDetails.queryByText('Connection')).toBeNull();
    expect(serviceDetails.getAllByText('State').length).toBeGreaterThan(0);
    expect(serviceDetails.getAllByText('pmg-main.local').length).toBeGreaterThan(0);
    expect(queryByText('Mail flow summary')).toBeNull();
    expect(queryByText('Queue breakdown')).toBeNull();
    expect(queryByText('Mail processing')).toBeNull();
    expect(queryByText('Queue detail')).toBeNull();
    expect(queryByText('Mail detail')).toBeNull();
    expect(queryByText('Show mail flow detail')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show mail flow' }));
    expect(getByText('Queue')).toBeInTheDocument();
    expect(getByText('Backlog')).toBeInTheDocument();
    const pmgSupportContext = within(getByTestId('pmg-support-context'));
    expect(pmgSupportContext.getByText('Nodes')).toBeInTheDocument();
    expect(pmgSupportContext.getByText('Updated')).toBeInTheDocument();
    expect(getByText('Queue detail').closest('summary')?.textContent).toBe('Queue detail');
    expect(getByText('Mail detail').closest('summary')?.textContent).toBe('Mail detail');
    expect(queryByRole('link', { name: /open pmg thresholds/i })).toBeNull();
  });

  it('keeps PMG freshness in support context even without a node count', () => {
    const resource = baseResource({
      id: 'pmg-2',
      type: 'pmg',
      name: 'pmg-edge',
      displayName: 'PMG Edge',
      platformId: 'pmg-edge',
      platformType: 'proxmox-pmg',
      platformData: {
        sources: ['pmg'],
        pmg: {
          hostname: 'pmg-edge.local',
          connectionHealth: 'online',
          lastUpdated: '2026-03-19T23:00:00Z',
          queueTotal: 12,
          mailCountTotal: 320,
        },
      },
    });

    const { getByRole, getByTestId } = render(() => <ResourceDetailDrawer resource={resource} />);

    expandPlatformDetails(getByTestId);
    fireEvent.click(getByRole('button', { name: 'Show service' }));
    fireEvent.click(getByRole('button', { name: 'Show mail flow' }));
    const pmgSupportContext = within(getByTestId('pmg-support-context'));
    expect(pmgSupportContext.queryByText('Nodes')).toBeNull();
    expect(pmgSupportContext.getByText('Updated')).toBeInTheDocument();
  });

  it('keeps docker update controls behind a secondary reveal', () => {
    const resource = baseResource({
      id: 'docker-host-1',
      type: 'docker-host',
      name: 'docker-main',
      displayName: 'Docker Main',
      platformId: 'docker-main',
      platformType: 'docker',
      sourceType: 'agent',
      platformData: {
        sources: ['docker', 'agent'],
        docker: {
          hostSourceId: 'docker-host-1',
          hostname: 'docker-main.local',
          runtime: 'Docker Engine 28.0',
          containerCount: 18,
          updatesAvailableCount: 4,
        },
      },
    });

    const { getByText, getByRole, getByTestId, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expandPlatformDetails(getByTestId);
    expect(getByText('Service')).toBeInTheDocument();
    expect(getByText('18 containers · 4 updates')).toBeInTheDocument();
    fireEvent.click(getByRole('button', { name: 'Show service' }));
    expect(getByText('Docker runtime')).toBeInTheDocument();
    expect(queryByText('Container Updates')).toBeNull();
    expect(queryByText('Check now')).toBeNull();
    expect(queryByText('Show update controls')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show actions' }));
    expect(getByText('Check now')).toBeInTheDocument();
    expect(getByText('Update all (4)')).toBeInTheDocument();
    expect(queryByText('Updates Available')).toBeNull();
    expect(queryByText('Last Check')).toBeNull();
  });

  it('renders linked HTTPS certificate posture in the resource overview', () => {
    const resource = baseResource({
      availabilityChecks: [
        {
          targetId: 'pulse-ui',
          address: 'pulse.example.test',
          protocol: 'https',
          enabled: true,
          available: true,
          lastChecked: new Date().toISOString(),
          certificateExpiryWarningDays: 30,
          certificate: {
            subject: 'pulse.example.test',
            issuer: 'Example CA',
            dnsNames: ['pulse.example.test'],
            notBefore: '2026-01-01T00:00:00Z',
            notAfter: '2027-01-01T00:00:00Z',
            fingerprintSha256: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
            chainValid: true,
            hostnameValid: true,
            selfSigned: false,
            trustStatus: 'trusted',
            observedAt: '2026-08-06T12:00:00Z',
          },
        },
      ],
    });

    const { getByTestId, getByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getByTestId('availability-probe-status')).toBeInTheDocument();
    expect(getByText('Trusted')).toBeInTheDocument();
    expect(getByText('Example CA')).toBeInTheDocument();
  });
});
