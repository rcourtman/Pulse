import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

const facetBundleMock = vi.hoisted(() => ({
  getFacetBundle: vi.fn(),
}));

const aiIntelligenceMock = vi.hoisted(() => ({
  getResourceIntelligence: vi.fn().mockResolvedValue({
    resource_id: 'resource-1',
    health: {
      score: 92,
      grade: 'A',
      trend: 'stable',
      factors: [],
      prediction: 'Stable',
    },
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
    recent_changes: [],
    note_count: 3,
  }),
}));

vi.mock('@/App', () => ({
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
    getFacetBundle: facetBundleMock.getFacetBundle,
  },
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getResourceIntelligence: aiIntelligenceMock.getResourceIntelligence,
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
  platformData: { sources: ['agent'] },
  ...overrides,
});

describe('ResourceDetailDrawer change history section', () => {
  it('keeps compact timeline summary chips in overview while showing one embedded change history section', async () => {
    facetBundleMock.getFacetBundle.mockResolvedValueOnce({
      capabilities: [
        {
          name: 'restart',
          type: 'common',
          description: 'Restart the resource safely.',
          minimumApprovalLevel: 'admin',
        },
      ],
      relationships: [
        {
          sourceId: 'node:pve-1',
          targetId: 'vm:42',
          type: 'runs_on',
          confidence: 1,
          active: true,
          discoverer: 'proxmox_adapter',
          observedAt: '2026-03-18T12:00:00Z',
          lastSeenAt: '2026-03-18T12:05:00Z',
          metadata: {
            cluster: 'pve-prod',
            hypervisor: 'pve-1',
          },
        },
      ],
      recentChanges: [
        {
          id: 'change-1',
          observedAt: '2026-03-18T12:06:00Z',
          occurredAt: '2026-03-18T12:04:00Z',
          resourceId: 'vm:42',
          kind: 'restart',
          from: 'running',
          to: 'restarting',
          sourceType: 'platform_event',
          sourceAdapter: 'proxmox_adapter',
          confidence: 'high',
          actor: 'agent:oncall-helper',
          relatedResources: ['node:pve-1'],
          reason: 'Routine restart requested',
          metadata: {
            ticket: 'INC-1234',
            policy: 'routine-maintenance',
          },
        },
      ],
      counts: {
        recentChanges: 3,
        recentChangeKinds: {
          restart: 2,
          metric_anomaly: 1,
        },
        recentChangeSourceTypes: {
          platform_event: 1,
          pulse_diff: 2,
        },
        recentChangeSourceAdapters: {
          docker_adapter: 2,
          proxmox_adapter: 1,
        },
      },
    });

    const resource = baseResource({
      id: 'vm:42',
      type: 'vm',
      name: 'vm-42',
      displayName: 'VM 42',
      platformId: 'vm-42',
      platformType: 'proxmox-pve',
      platformData: { sources: ['proxmox'] },
    });

    render(() => (
      <ResourceDetailDrawer
        resource={resource}
        resolveResourceLabel={(resourceId) =>
          resourceId === 'node:pve-1'
            ? 'PVE Node 1'
            : resourceId === 'storage-1'
              ? 'Storage 1 alias'
              : resourceId === 'vm-child'
                ? 'VM Child'
                : resourceId
        }
      />
    ));

    await screen.findByText('Changes loaded');
    const changeHistorySection = screen.getByTestId('resource-change-history-section');
    expect(screen.queryByRole('button', { name: 'Discovery' })).toBeNull();
    expect(screen.getByText('Change history')).toBeInTheDocument();
    expect(screen.queryByText('Host details')).toBeNull();
    expect(screen.queryByText('Service details')).toBeNull();
    expect(screen.queryByText('Supporting context')).toBeNull();
    expect(screen.getByText('Discovery context')).toBeInTheDocument();
    expect(
      screen.queryByText('Supporting metadata only. The web interface path above stays primary.'),
    ).toBeNull();
    expect(screen.getByRole('button', { name: 'Show metadata' })).toBeInTheDocument();
    expect(screen.queryByText('Operational context')).toBeNull();
    expect(
      within(changeHistorySection).queryByText('Filterable event history for this resource.'),
    ).toBeNull();
    expect(within(changeHistorySection).queryByText('Recent activity')).toBeNull();
    expect(screen.queryByText('Events')).toBeNull();
    expect(screen.getAllByText('Timeline 3')).toHaveLength(1);
    expect(screen.getAllByText('Restart 2')).toHaveLength(1);
    expect(screen.getAllByText('Anomaly 1')).toHaveLength(1);
    expect(screen.getAllByText('Platform event 1')).toHaveLength(1);
    expect(screen.getAllByText('Pulse diff 2')).toHaveLength(1);
    expect(screen.getAllByText('Docker adapter 2')).toHaveLength(1);
    expect(screen.getAllByText('Proxmox adapter 1')).toHaveLength(1);
    expect(screen.queryByText('Quick links')).toBeNull();
    expect(screen.getByText('Investigation context')).toBeInTheDocument();
    expect(screen.queryByText('Correlation context')).toBeNull();
    expect(screen.queryByText('Storage 1 alias')).toBeNull();
    expect(screen.queryByText('VM Child')).toBeNull();
    expect(screen.queryByText('Capabilities 1')).toBeNull();
    expect(screen.queryByText('Relationships 1')).toBeNull();
  });

  it('renders timeline history without surfacing unsupported capability or relationship facets', async () => {
    facetBundleMock.getFacetBundle.mockResolvedValueOnce({
      capabilities: [
        {
          name: 'restart',
          type: 'common',
          description: 'Restart the resource safely.',
          minimumApprovalLevel: 'admin',
          platform: 'proxmox',
          params: [
            {
              name: 'force',
              type: 'boolean',
              required: false,
              isSensitive: false,
              description: 'Force the restart when needed.',
            },
          ],
        },
      ],
      relationships: [
        {
          sourceId: 'node:pve-1',
          targetId: 'vm:42',
          type: 'runs_on',
          confidence: 1,
          active: true,
          discoverer: 'proxmox_adapter',
          observedAt: '2026-03-18T12:00:00Z',
          lastSeenAt: '2026-03-18T12:05:00Z',
          metadata: {
            cluster: 'pve-prod',
            hypervisor: 'pve-1',
          },
        },
      ],
      recentChanges: [
        {
          id: 'change-1',
          observedAt: '2026-03-18T12:06:00Z',
          occurredAt: '2026-03-18T12:04:00Z',
          resourceId: 'vm:42',
          kind: 'restart',
          from: 'running',
          to: 'restarting',
          sourceType: 'platform_event',
          sourceAdapter: 'proxmox_adapter',
          confidence: 'high',
          actor: 'agent:oncall-helper',
          relatedResources: ['node:pve-1'],
          reason: 'Routine restart requested',
          metadata: {
            ticket: 'INC-1234',
            policy: 'routine-maintenance',
          },
        },
      ],
      counts: {
        recentChanges: 3,
        recentChangeKinds: {
          restart: 2,
          metric_anomaly: 1,
        },
        recentChangeSourceAdapters: {
          docker_adapter: 1,
          proxmox_adapter: 2,
        },
      },
    });

    const resource = baseResource({
      id: 'vm:42',
      type: 'vm',
      name: 'vm-42',
      displayName: 'VM 42',
      platformId: 'vm-42',
      platformType: 'proxmox-pve',
      platformData: { sources: ['proxmox'] },
    });

    render(() => (
      <ResourceDetailDrawer
        resource={resource}
        resolveResourceLabel={(resourceId) =>
          resourceId === 'node:pve-1' ? 'PVE Node 1' : resourceId
        }
      />
    ));

    await screen.findByText('Change history');
    const historyPanel = screen.getByTestId('resource-change-history-section');
    const panel = within(historyPanel);
    expect(await panel.findByText('Changes loaded')).toBeInTheDocument();
    expect(panel.getByText('Routine restart requested')).toBeInTheDocument();
    expect(panel.queryByText('Recent activity')).toBeNull();
    expect(panel.queryByText('Filterable event history for this resource.')).toBeNull();
    expect(panel.getByText('Event log')).toBeInTheDocument();
    expect(
      panel.getByRole('link', { name: 'Open related resource PVE Node 1 in Infrastructure' }),
    ).toHaveAttribute('href', '/infrastructure?resource=node%3Apve-1');
    expect(panel.getByText('PVE Node 1')).toBeInTheDocument();
    expect(panel.getByText('Routine restart requested')).toBeInTheDocument();
    expect(panel.getByText('Confidence')).toBeInTheDocument();
    expect(panel.getByText('Adapter')).toBeInTheDocument();
    expect(panel.getByText('Metadata')).toBeInTheDocument();
    expect(panel.getByText(/"ticket": "INC-1234"/)).toBeInTheDocument();
    expect(panel.queryByText('Capabilities')).toBeNull();
    expect(panel.queryByText('Relationships')).toBeNull();
    expect(panel.queryByText('Runs on')).toBeNull();
  });

  it('keeps service details summary-first until the service-local reveal is opened', () => {
    facetBundleMock.getFacetBundle.mockResolvedValueOnce({
      capabilities: [],
      relationships: [],
      recentChanges: [],
    });

    const resource = baseResource({
      id: 'pbs-1',
      type: 'pbs',
      name: 'pbs-main',
      displayName: 'PBS Main',
      platformId: 'pbs-main',
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

    render(() => <ResourceDetailDrawer resource={resource} />);

    expect(screen.getByText('Service details')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Show service details' }));
    expect(screen.getByText('PBS Service')).toBeInTheDocument();
    expect(screen.getByText('Backup summary')).toBeInTheDocument();
    expect(screen.queryByText('Job breakdown')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Show job detail' }));
    expect(screen.getByText('Job breakdown')).toBeInTheDocument();
  });

  it('filters timeline entries by kind and source type', async () => {
    const unfilteredFacetBundle = {
      capabilities: [
        {
          name: 'restart',
          type: 'common',
          description: 'Restart the resource safely.',
          minimumApprovalLevel: 'admin',
        },
      ],
      relationships: [
        {
          sourceId: 'node:pve-1',
          targetId: 'vm:42',
          type: 'runs_on',
          confidence: 1,
          active: true,
          discoverer: 'proxmox_adapter',
          observedAt: '2026-03-18T12:00:00Z',
          lastSeenAt: '2026-03-18T12:05:00Z',
          metadata: {
            cluster: 'pve-prod',
            source: 'live',
          },
        },
      ],
      recentChanges: [
        {
          id: 'change-1',
          observedAt: '2026-03-18T12:06:00Z',
          occurredAt: '2026-03-18T12:04:00Z',
          resourceId: 'vm:42',
          kind: 'restart',
          sourceType: 'platform_event',
          sourceAdapter: 'proxmox_adapter',
          confidence: 'high',
          actor: 'agent:oncall-helper',
          relatedResources: ['node:pve-1'],
          reason: 'Routine restart requested',
          metadata: {
            ticket: 'INC-1234',
          },
        },
        {
          id: 'change-2',
          observedAt: '2026-03-18T12:02:00Z',
          resourceId: 'vm:42',
          kind: 'metric_anomaly',
          sourceType: 'pulse_diff',
          sourceAdapter: 'proxmox_adapter',
          confidence: 'medium',
          reason: 'CPU spike detected',
        },
      ],
      counts: {
        recentChanges: 2,
        recentChangeKinds: {
          restart: 1,
          metric_anomaly: 1,
        },
        recentChangeSourceAdapters: {
          docker_adapter: 1,
          proxmox_adapter: 1,
        },
      },
    };
    const filteredFacetBundle = {
      capabilities: [
        {
          name: 'restart',
          type: 'common',
          description: 'Restart the resource safely.',
          minimumApprovalLevel: 'admin',
        },
      ],
      relationships: unfilteredFacetBundle.relationships,
      recentChanges: [
        {
          id: 'change-1',
          observedAt: '2026-03-18T12:06:00Z',
          occurredAt: '2026-03-18T12:04:00Z',
          resourceId: 'vm:42',
          kind: 'restart',
          sourceType: 'platform_event',
          sourceAdapter: 'proxmox_adapter',
          confidence: 'high',
          actor: 'agent:oncall-helper',
          relatedResources: ['node:pve-1'],
          reason: 'Routine restart requested',
          metadata: {
            ticket: 'INC-1234',
          },
        },
      ],
      counts: {
        recentChanges: 1,
      },
    };
    facetBundleMock.getFacetBundle
      .mockResolvedValueOnce(unfilteredFacetBundle)
      .mockResolvedValueOnce(filteredFacetBundle)
      .mockResolvedValueOnce(filteredFacetBundle);

    const resource = baseResource({
      id: 'vm:42',
      type: 'vm',
      name: 'vm-42',
      displayName: 'VM 42',
      platformId: 'vm-42',
      platformType: 'proxmox-pve',
      platformData: { sources: ['proxmox'] },
    });

    render(() => <ResourceDetailDrawer resource={resource} />);

    await screen.findByText('Change history');
    const historyPanel = screen.getByTestId('resource-change-history-section');
    const panel = within(historyPanel);
    expect(await panel.findByText('Changes loaded')).toBeInTheDocument();
    expect(panel.getByText('CPU spike detected')).toBeInTheDocument();

    fireEvent.change(panel.getByLabelText('Change kind'), {
      target: { value: 'restart' },
    });
    fireEvent.change(panel.getByLabelText('Source type'), {
      target: { value: 'platform_event' },
    });

    expect(await panel.findByText('Filtered changes loaded')).toBeInTheDocument();
    expect(await panel.findByText('Routine restart requested')).toBeInTheDocument();
    expect(panel.queryByText('CPU spike detected')).toBeNull();
  });

  it('filters timeline entries by source adapter', async () => {
    facetBundleMock.getFacetBundle
      .mockResolvedValueOnce({
        capabilities: [
          {
            name: 'restart',
            type: 'common',
            description: 'Restart the resource safely.',
            minimumApprovalLevel: 'admin',
          },
        ],
        relationships: [
          {
            sourceId: 'node:pve-1',
            targetId: 'vm:42',
            type: 'runs_on',
            confidence: 1,
            active: true,
            discoverer: 'proxmox_adapter',
            observedAt: '2026-03-18T12:00:00Z',
            lastSeenAt: '2026-03-18T12:05:00Z',
            metadata: {
              cluster: 'pve-prod',
              source: 'live',
            },
          },
        ],
        recentChanges: [
          {
            id: 'change-1',
            observedAt: '2026-03-18T12:06:00Z',
            occurredAt: '2026-03-18T12:04:00Z',
            resourceId: 'vm:42',
            kind: 'restart',
            sourceType: 'platform_event',
            sourceAdapter: 'proxmox_adapter',
            confidence: 'high',
            actor: 'agent:oncall-helper',
            relatedResources: ['node:pve-1'],
            reason: 'Routine restart requested',
            metadata: {
              ticket: 'INC-1234',
            },
          },
          {
            id: 'change-2',
            observedAt: '2026-03-18T12:02:00Z',
            resourceId: 'vm:42',
            kind: 'metric_anomaly',
            sourceType: 'pulse_diff',
            sourceAdapter: 'docker_adapter',
            confidence: 'medium',
            reason: 'CPU spike detected',
          },
        ],
        counts: {
          recentChanges: 2,
          recentChangeKinds: {
            restart: 1,
            metric_anomaly: 1,
          },
          recentChangeSourceAdapters: {
            docker_adapter: 1,
            proxmox_adapter: 1,
          },
        },
      })
      .mockResolvedValueOnce({
        capabilities: [
          {
            name: 'restart',
            type: 'common',
            description: 'Restart the resource safely.',
            minimumApprovalLevel: 'admin',
          },
        ],
        relationships: [
          {
            sourceId: 'node:pve-1',
            targetId: 'vm:42',
            type: 'runs_on',
            confidence: 1,
            active: true,
            discoverer: 'proxmox_adapter',
            observedAt: '2026-03-18T12:00:00Z',
            lastSeenAt: '2026-03-18T12:05:00Z',
            metadata: {
              cluster: 'pve-prod',
              source: 'live',
            },
          },
        ],
        recentChanges: [
          {
            id: 'change-2',
            observedAt: '2026-03-18T12:02:00Z',
            resourceId: 'vm:42',
            kind: 'metric_anomaly',
            sourceType: 'pulse_diff',
            sourceAdapter: 'docker_adapter',
            confidence: 'medium',
            reason: 'CPU spike detected',
          },
        ],
        counts: {
          recentChanges: 1,
          recentChangeKinds: {
            metric_anomaly: 1,
          },
          recentChangeSourceAdapters: {
            docker_adapter: 1,
          },
        },
      });

    const resource = baseResource({
      id: 'vm:42',
      type: 'vm',
      name: 'vm-42',
      displayName: 'VM 42',
      platformId: 'vm-42',
      platformType: 'proxmox-pve',
      platformData: { sources: ['proxmox'] },
    });

    render(() => <ResourceDetailDrawer resource={resource} />);

    await screen.findByText('Change history');
    const historyPanel = screen.getByTestId('resource-change-history-section');
    const panel = within(historyPanel);
    expect(await panel.findByText('Changes loaded')).toBeInTheDocument();
    expect(panel.getByText('CPU spike detected')).toBeInTheDocument();

    fireEvent.change(panel.getByLabelText('Source adapter'), {
      target: { value: 'docker_adapter' },
    });

    expect(await panel.findByText('Filtered changes loaded')).toBeInTheDocument();
    expect(await panel.findByText('CPU spike detected')).toBeInTheDocument();
    expect(panel.queryByText('Routine restart requested')).toBeNull();
  });
});
