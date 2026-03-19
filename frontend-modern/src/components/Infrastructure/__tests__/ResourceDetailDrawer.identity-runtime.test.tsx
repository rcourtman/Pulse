import { describe, expect, it, vi } from 'vitest';
import { render, waitFor } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

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
      policy_posture: {
        total_resources: 2,
        sensitivity_counts: {
          public: 1,
          internal: 1,
        },
        routing_counts: {
          'cloud-summary': 1,
          'local-first': 1,
        },
        redaction_counts: {
          hostname: 1,
        },
      },
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
  it('renders runtime source summary and mode details', () => {
    const resource = baseResource({
      platformData: {
        sources: ['agent', 'proxmox'],
        sourceStatus: {
          agent: { status: 'online', lastSeen: new Date().toISOString() },
          proxmox: { status: 'online', lastSeen: new Date().toISOString() },
        },
      },
    });

    const { getByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getByText('Runtime')).toBeInTheDocument();
    expect(getByText('Sources')).toBeInTheDocument();
    expect(getByText('2/2 healthy')).toBeInTheDocument();
    expect(getByText('Mode')).toBeInTheDocument();
    expect(getByText('Hybrid')).toBeInTheDocument();
  });

  it('falls back to source list summary when per-source health is unavailable', () => {
    const resource = baseResource({
      sourceType: 'api',
      platformData: {
        sources: ['pmg'],
      },
    });

    const { getByText, getAllByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getByText('Sources')).toBeInTheDocument();
    expect(getAllByText('PMG').length).toBeGreaterThan(0);
    expect(getByText('Mode')).toBeInTheDocument();
    expect(getByText('API')).toBeInTheDocument();
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
    expect(queryByText('No enriched identity metadata yet.')).toBeNull();

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
    expect(sparseRender.getByText('No enriched identity metadata yet.')).toBeInTheDocument();
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
      policy: {
        sensitivity: 'restricted',
        routing: {
          scope: 'local-only',
          redact: ['hostname', 'ip-address', 'alias'],
        },
      },
      aiSafeSummary: 'restricted host summary safe for remote AI consumption',
    });

    const { getAllByText, getByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getAllByText('Restricted').length).toBeGreaterThan(0);
    expect(getAllByText('Local Only').length).toBeGreaterThan(0);
    expect(getByText('Data Governance')).toBeInTheDocument();
    expect(getByText('Redactions')).toBeInTheDocument();
    expect(getByText('Hostname')).toBeInTheDocument();
    expect(getByText('IP Address')).toBeInTheDocument();
    expect(getByText('AI-Safe Summary')).toBeInTheDocument();
    expect(
      getByText('restricted host summary safe for remote AI consumption'),
    ).toBeInTheDocument();
  });

  it('surfaces canonical AI intelligence for the resource overview', async () => {
    const resource = baseResource({});
    const { getByRole, getByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    await waitFor(() => {
      expect(getByText('AI Intelligence')).toBeInTheDocument();
    });

    expect(getByText(/A · 92\/100/)).toBeInTheDocument();
    expect(getByText('Recent Changes')).toBeInTheDocument();
    expect(getByText('Recent Changes').parentElement).toHaveTextContent('1');
    expect(getByText('Policy posture')).toBeInTheDocument();
    expect(getByText('2 governed resources')).toBeInTheDocument();
    expect(getByText('Public')).toBeInTheDocument();
    expect(getByText('Cloud Summary')).toBeInTheDocument();
    await waitFor(() => {
      expect(getByText('Correlation context')).toBeInTheDocument();
    });
    expect(getByText('1 dependency · 1 dependent · 1 correlation')).toBeInTheDocument();
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
