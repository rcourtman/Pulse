import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import {
  ResourceDetailDrawer,
  getSpecializedTabAvailabilityMessage,
} from '@/components/Infrastructure/ResourceDetailDrawer';

const wsState = vi.hoisted(() => ({ pmg: [] as any[] }));
const reconnectSpy = vi.hoisted(() => vi.fn());

vi.mock('@/App', () => ({
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
        score: 92,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Stable',
      },
      recent_changes: [
        {
          id: 'change-1',
          observedAt: '2026-03-01T00:00:00Z',
          resourceId: 'resource-1',
          kind: 'config_update',
          sourceType: 'pulse_diff',
          confidence: 'high',
          reason: 'Updated canonical config',
        },
      ],
      dependencies: ['storage-1'],
      dependents: ['vm-child'],
      correlations: [
        {
          source_id: 'storage-1',
          source_name: 'Storage 1',
          source_type: 'storage',
          target_id: 'resource-1',
          target_name: 'Host 1',
          target_type: 'vm',
          event_pattern: 'disk_full -> restart',
          occurrences: 2,
          avg_delay: 125000000000,
          confidence: 0.875,
          last_seen: '2026-03-01T00:15:00Z',
          description: 'Disk pressure often precedes restarts',
        },
      ],
      note_count: 3,
    }),
  },
}));

class ResizeObserverMock {
  constructor(_callback: ResizeObserverCallback) {}
  observe() {}
  unobserve() {}
  disconnect() {}
}

if (typeof globalThis.ResizeObserver === 'undefined') {
  vi.stubGlobal('ResizeObserver', ResizeObserverMock);
}

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'host-1',
  displayName: 'host-1',
  platformId: 'host-1',
  platformType: 'agent',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['agent', 'proxmox'] },
  ...overrides,
});

describe('ResourceDetailDrawer runtime and identity cards', () => {
  it('renders a unified summary shell with current-state source and mode details', () => {
    const resource = baseResource({
      platformData: {
        sources: ['agent', 'proxmox'],
        sourceStatus: {
          agent: { status: 'online', lastSeen: new Date().toISOString() },
          proxmox: { status: 'online', lastSeen: new Date().toISOString() },
        },
      },
    });

    const { getByRole, getByText, queryByRole } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Summary')).toBeInTheDocument();
    expect(getByText('Current state')).toBeInTheDocument();
    expect(getByText('Sources')).toBeInTheDocument();
    expect(getByText('2/2 healthy')).toBeInTheDocument();
    expect(getByText('Mode')).toBeInTheDocument();
    expect(getByText('Hybrid')).toBeInTheDocument();
    expect(() => getByText('Platform ID')).toThrow();
    expect(getByText('Quick links')).toBeInTheDocument();
    expect(queryByRole('button', { name: 'Show details' })).toBeNull();
    expect(() => getByText('Runtime')).toThrow();
    expect(getByRole('link', { name: 'Open related workloads for host-1' })).toHaveTextContent(
      'Workloads',
    );
  });

  it('keeps discovery as secondary overview context instead of a peer tab', async () => {
    const { getByRole, getByText, getByTestId, queryByRole, queryByTestId, queryByText } = render(
      () => <ResourceDetailDrawer resource={baseResource({})} />,
    );

    expect(queryByRole('button', { name: 'Discovery' })).toBeNull();
    expect(getByText('Discovery context')).toBeInTheDocument();
    expect(getByText('Host discovery via host-1')).toBeInTheDocument();
    expect(
      queryByText('Supporting metadata only. The web interface path above stays primary.'),
    ).toBeNull();
    expect(queryByTestId('discovery-tab')).toBeNull();
    expect(
      getByTestId('resource-discovery-context').querySelector(
        '.mt-3.rounded.border.border-border.bg-surface.p-2\\.5',
      ),
    ).toBeNull();

    expect(getByRole('button', { name: 'Show metadata' })).toBeInTheDocument();

    fireEvent.click(getByRole('button', { name: 'Show metadata' }));

    await waitFor(() => {
      expect(queryByTestId('discovery-tab')).toBeInTheDocument();
    });
  });

  it('uses terse availability notices for specialized tabs', () => {
    expect(getSpecializedTabAvailabilityMessage('mail')).toBe('PMG resources only.');
    expect(getSpecializedTabAvailabilityMessage('namespaces')).toBe('Kubernetes clusters only.');
    expect(getSpecializedTabAvailabilityMessage('deployments')).toBe(
      'Kubernetes clusters only.',
    );
    expect(getSpecializedTabAvailabilityMessage('swarm')).toBe(
      'Docker runtimes with Swarm only.',
    );
  });

  it('keeps host detail cards behind a secondary overview disclosure', async () => {
    const resource = baseResource({
      platformData: {
        sources: ['agent'],
        agent: {
          agentId: 'agent-1',
          hostname: 'host-1',
          platform: 'linux',
          osName: 'Ubuntu',
          osVersion: '24.04',
          kernelVersion: '6.8.0',
          architecture: 'x86_64',
          uptimeSeconds: 7200,
          cpuCount: 8,
          agentVersion: '1.2.3',
          memory: { total: 16 * 1024 * 1024 * 1024 },
          networkInterfaces: [
            {
              name: 'eth0',
              mac: '00:11:22:33:44:55',
              addresses: ['192.0.2.10'],
            },
          ],
          disks: [
            {
              mountpoint: '/',
              total: 100 * 1024 * 1024 * 1024,
              used: 50 * 1024 * 1024 * 1024,
            },
          ],
        },
      },
    });

    const { getByRole, getByText, getByTestId, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Host details')).toBeInTheDocument();
    expect(
      getByText('4 detail cards covering system, hardware, network, and disks.'),
    ).toBeInTheDocument();
    expect(queryByText('Hardware')).toBeNull();
    expect(queryByText('Network')).toBeNull();

    fireEvent.click(getByRole('button', { name: 'Show host details' }));

    await waitFor(() => {
      expect(getByText('Hardware')).toBeInTheDocument();
    });
    expect(getByTestId('resource-secondary-sections').classList.contains('flex')).toBe(true);
    expect(
      getByTestId('resource-secondary-sections').classList.contains('flex-wrap'),
    ).toBe(true);
    expect(getByTestId('resource-host-details-section').querySelector('.mt-3.flex.flex-wrap')).toBeTruthy();
    expect(
      getByTestId('resource-host-details-section').querySelector('.mt-3.flex.flex-wrap')?.classList.contains('[&>*]:min-w-[220px]'),
    ).toBe(true);
    expect(getByText('Network')).toBeInTheDocument();
    expect(getByText('Disks')).toBeInTheDocument();
    expect(getByText('eth0')).toBeInTheDocument();
  });

  it('falls back to source list summary when per-source health is unavailable', () => {
    const resource = baseResource({
      sourceType: 'api',
      platformData: {
        sources: ['pmg'],
      },
    });

    const { getByText, getAllByText, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Sources')).toBeInTheDocument();
    expect(getAllByText('PMG').length).toBeGreaterThan(0);
    expect(getByText('Mode')).toBeInTheDocument();
    expect(getByText('API')).toBeInTheDocument();
    expect(queryByText('Platform ID')).toBeNull();
    expect(queryByText('Details')).toBeNull();
    expect(queryByText('Show details')).toBeNull();
  });

  it('shows identity aliases and fallback message when identity metadata is sparse', () => {
    const resource = baseResource({
      id: 'pmg-main',
      type: 'pmg',
      name: 'pmg-main',
      displayName: 'PMG Main',
      platformId: 'pmg-main',
      platformType: 'proxmox-pmg',
      sourceType: 'api',
      identity: {
        hostname: 'pmg.local',
      },
      platformData: {
        sources: ['pmg'],
        pmg: {
          instanceId: 'pmg-main',
        },
      },
    });

    const { getByText, getAllByText, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Identity')).toBeInTheDocument();
    expect(getByText('Aliases')).toBeInTheDocument();
    expect(getAllByText('pmg-main').length).toBeGreaterThan(0);
    expect(queryByText('No identity metadata yet.')).toBeNull();

    const sparse = baseResource({
      id: 'host-min',
      name: 'host-min',
      displayName: 'Host Min',
      platformId: 'host-min',
      sourceType: 'agent',
      identity: undefined,
      parentId: undefined,
      clusterId: undefined,
      discoveryTarget: undefined,
      tags: [],
      platformData: { sources: ['agent'] },
    });

    const sparseRender = render(() => <ResourceDetailDrawer resource={sparse} />);
    expect(sparseRender.getByText('No identity metadata yet.')).toBeInTheDocument();
  });

  it('shows canonical metrics target identity for docker-backed host resources', () => {
    const resource = baseResource({
      id: 'hash-docker-resource',
      type: 'docker-host',
      name: 'Tower',
      displayName: 'Tower',
      platformId: 'tower',
      platformType: 'docker',
      sourceType: 'agent',
      identity: {
        hostname: 'tower.local',
      },
      metricsTarget: {
        resourceType: 'docker-host',
        resourceId: 'docker-host-1',
      },
      platformData: {
        sources: ['docker', 'agent'],
        docker: {
          hostSourceId: 'docker-host-1',
          hostname: 'tower.local',
        },
      },
    });

    const { container, getByText, getAllByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Metrics Target')).toBeInTheDocument();
    expect(getAllByText('docker-host:docker-host-1').length).toBeGreaterThan(1);
    expect(getByText('Aliases')).toBeInTheDocument();
    expect(getAllByText('docker-host-1').length).toBeGreaterThan(0);
    expect(container.querySelector('.text-\\[11px\\].text-muted.truncate')?.textContent).toBe(
      'docker-host:docker-host-1',
    );
  });

  it('renders aliases inline when few exist and collapses when many exist', () => {
    const inlineResource = baseResource({
      type: 'agent',
      platformData: {
        sources: ['agent'],
        agent: {
          agentId: 'agent-inline-1',
          hostname: 'inline-host.local',
        },
      },
      identity: undefined,
      parentId: undefined,
      clusterId: undefined,
      discoveryTarget: undefined,
      tags: [],
    });

    const inlineRender = render(() => <ResourceDetailDrawer resource={inlineResource} />);
    expect(inlineRender.getByText('Aliases')).toBeInTheDocument();
    expect(inlineRender.container.querySelector('details')).toBeNull();
    expect(inlineRender.getByText('agent-inline-1')).toBeInTheDocument();
    expect(inlineRender.getAllByText('inline-host.local').length).toBeGreaterThan(0);

    const overflowResource = baseResource({
      type: 'agent',
      platformData: {
        sources: ['agent', 'proxmox'],
        agent: {
          agentId: 'agent-overflow-1',
          hostname: 'overflow-host.local',
        },
        proxmox: {
          nodeName: 'overflow-node-1',
        },
      },
      discoveryTarget: {
        resourceType: 'agent',
        agentId: 'overflow-host-id',
        resourceId: 'overflow-resource-id',
      },
      identity: {
        hostname: 'overflow-identity-host',
      },
      tags: [],
    });

    const overflowRender = render(() => <ResourceDetailDrawer resource={overflowResource} />);
    expect(overflowRender.getByText('Aliases')).toBeInTheDocument();
    expect(overflowRender.container.querySelector('details')).toBeTruthy();
  });

  it('surfaces policy governance details and AI-safe summary', () => {
    const resource = baseResource({
      id: 'sensitive-resource-1',
      name: 'sensitive-host',
      displayName: 'Sensitive Host',
      platformId: 'platform-1',
      policy: {
        sensitivity: 'restricted',
        routing: {
          scope: 'local-only',
          redact: ['hostname', 'ip-address', 'alias'],
        },
      },
      aiSafeSummary: 'restricted host summary safe for remote AI consumption',
    });

    const { getAllByText, getByRole, getByText, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Investigation context')).toBeInTheDocument();
    expect(queryByText('Data Governance')).toBeNull();
    expect(queryByText('AI-Safe Summary')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show context' }));
    expect(getByText('Data Governance')).toBeInTheDocument();
    expect(getByText('Redactions')).toBeInTheDocument();
    expect(getByText('AI-Safe Summary')).toBeInTheDocument();
    expect(getAllByText('Restricted').length).toBeGreaterThan(0);
    expect(getAllByText('Local Only').length).toBeGreaterThan(0);
    expect(getByText('Hostname')).toBeInTheDocument();
    expect(getByText('IP Address')).toBeInTheDocument();
    expect(
      getAllByText('restricted host summary safe for remote AI consumption').length,
    ).toBeGreaterThan(0);
    expect(queryByText('Sensitive Host')).toBeNull();
    expect(queryByText('sensitive-host')).toBeNull();
  });

  it('uses the governed policy helper when aiSafeSummary is absent', () => {
    const resource = baseResource({
      id: 'governed-resource-1',
      name: 'governed-host',
      displayName: 'Governed Host',
      policy: {
        sensitivity: 'restricted',
        routing: {
          scope: 'local-only',
          redact: ['hostname'],
        },
      },
    });

    const { getAllByText, getByRole, getByText, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Investigation context')).toBeInTheDocument();
    expect(queryByText('AI-Safe Summary')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show context' }));
    expect(getByText('AI-Safe Summary')).toBeInTheDocument();
    expect(getAllByText('redacted by policy').length).toBeGreaterThan(1);
  });

  it('surfaces canonical AI intelligence for the resource overview', async () => {
    const resource = baseResource({});
    const { getByRole, getByText, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    await waitFor(() => {
      expect(getByText('Investigation context')).toBeInTheDocument();
    });

    expect(queryByText('AI Intelligence')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show context' }));
    await waitFor(() => {
      expect(getByText('AI Intelligence')).toBeInTheDocument();
    });
    expect(getByText('Health')).toBeInTheDocument();
    expect(getByText('A · 92/100')).toBeInTheDocument();
    expect(getByText('Trend')).toBeInTheDocument();
    expect(getByText('stable')).toBeInTheDocument();
    expect(getByText('Notes')).toBeInTheDocument();
    expect(getByText('3')).toBeInTheDocument();
    expect(queryByText('Storage 1')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show correlations' }));
    await waitFor(() => {
      expect(getByText('Storage 1')).toBeInTheDocument();
    });
    expect(
      getByRole('button', { name: 'Hide correlations' })
        .parentElement?.querySelector('.mt-0\\.5.text-\\[10px\\].text-muted'),
    ).toBeNull();
    expect(
      getByRole('link', {
        name: 'Open dependency resource storage-1 in Infrastructure',
      }),
    ).toHaveAttribute('href', expect.stringContaining('/infrastructure?resource=storage-1'));
    expect(
      getByRole('link', {
        name: 'Open dependent resource vm-child in Infrastructure',
      }),
    ).toHaveAttribute('href', expect.stringContaining('/infrastructure?resource=vm-child'));
    expect(getByText('Storage 1')).toBeInTheDocument();
    expect(getByText('Host 1')).toBeInTheDocument();
    expect(getByText('Disk Full → Restart')).toBeInTheDocument();
    expect(getByText(/2 occurrences · avg delay 2m · 88% confidence/)).toBeInTheDocument();
    expect(getByText('Disk pressure often precedes restarts')).toBeInTheDocument();
    expect(getByText('Latest canonical change')).toBeInTheDocument();
    const latestChangeItem = getByText('Config update: Updated canonical config').closest('li');
    expect(latestChangeItem).not.toBeNull();
    expect(getByText('Updated canonical config')).toBeInTheDocument();
    expect(latestChangeItem).toHaveTextContent(/just now|m ago|h ago|d ago/);
  });
});
