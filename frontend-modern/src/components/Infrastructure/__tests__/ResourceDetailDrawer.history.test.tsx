import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@solidjs/testing-library';

import discoveryTabSource from '@/components/Discovery/DiscoveryTab.tsx?raw';
import discoveryTabStateSource from '@/components/Discovery/useDiscoveryTabState.ts?raw';
import resourceDetailDrawerShellSource from '@/components/Infrastructure/ResourceDetailDrawer.tsx?raw';
import resourceDetailDrawerOverviewSource from '@/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx?raw';
import resourceDetailDrawerHistoryStateSource from '@/components/Infrastructure/useResourceDetailDrawerHistoryState.ts?raw';
import resourceDetailDrawerDerivedStateSource from '@/components/Infrastructure/useResourceDetailDrawerDerivedState.ts?raw';
import resourceDetailDrawerDiscoveryModelSource from '@/components/Infrastructure/resourceDetailDiscoveryModel.ts?raw';
import resourceDetailDrawerIdentityModelSource from '@/components/Infrastructure/resourceDetailDrawerIdentityModel.ts?raw';
import resourceDetailDrawerOperationalModelSource from '@/components/Infrastructure/resourceDetailDrawerOperationalModel.ts?raw';
import resourceDetailDrawerServiceModelSource from '@/components/Infrastructure/resourceDetailDrawerServiceModel.ts?raw';
import resourceDetailDrawerDockerActionsStateSource from '@/components/Infrastructure/useResourceDetailDrawerDockerActionsState.ts?raw';
import resourceDetailDrawerStateSource from '@/components/Infrastructure/useResourceDetailDrawerState.ts?raw';
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
  it('keeps discovery context presentation separate from discovery runtime ownership', () => {
    expect(discoveryTabSource).toContain('useDiscoveryTabState');
    expect(discoveryTabStateSource).toContain('export function useDiscoveryTabState');
    expect(discoveryTabStateSource).toContain('createResource');
    expect(discoveryTabStateSource).toContain("eventBus.on('ai_discovery_progress'");
    expect(discoveryTabStateSource).toContain('triggerDiscovery(');
    expect(discoveryTabStateSource).toContain('updateDiscoveryNotes(');
    expect(discoveryTabSource).not.toContain("eventBus.on('ai_discovery_progress'");
    expect(discoveryTabSource).not.toContain('createResource(');
    expect(discoveryTabSource).not.toContain('getConnectedAgents(');
    expect(discoveryTabSource).not.toContain('triggerDiscovery(');
    expect(discoveryTabSource).not.toContain('updateDiscoveryNotes(');
    expect(resourceDetailDrawerOverviewSource).toContain("from '@/components/shared/TagBadges'");
    expect(resourceDetailDrawerOverviewSource).not.toContain(
      "from '@/components/Dashboard/TagBadges'",
    );
    expect(resourceDetailDrawerShellSource).toContain(
      "from './ResourceDetailDrawerOverviewTab'",
    );
    expect(resourceDetailDrawerShellSource).toContain("from './ResourceDetailDrawerDebugTab'");
    expect(resourceDetailDrawerShellSource).toContain('data-testid="resource-header-badges"');
    expect(resourceDetailDrawerShellSource).toContain('drawer.headerBadges()');
    expect(resourceDetailDrawerShellSource).not.toContain('drawer.headerIdentity()');
    expect(resourceDetailDrawerShellSource).not.toContain('drawer.unifiedSourceBadges()');
    expect(resourceDetailDrawerShellSource).not.toContain('Change history');
    expect(resourceDetailDrawerStateSource).toContain("from './useResourceDetailDrawerHistoryState'");
    expect(resourceDetailDrawerStateSource).toContain("from './useResourceDetailDrawerDerivedState'");
    expect(resourceDetailDrawerStateSource).toContain(
      "from './useResourceDetailDrawerDockerActionsState'",
    );
    expect(resourceDetailDrawerStateSource).toContain('const [showHistoryFilters, setShowHistoryFilters]');
    expect(resourceDetailDrawerStateSource).not.toContain('createResource(');
    expect(resourceDetailDrawerStateSource).not.toContain('MonitoringAPI.');
    expect(resourceDetailDrawerHistoryStateSource).toContain('createResource(');
    expect(resourceDetailDrawerHistoryStateSource).toContain('ResourceAPI.getFacetBundle');
    expect(resourceDetailDrawerHistoryStateSource).toContain('AIAPI.getResourceIntelligence');
    expect(resourceDetailDrawerDerivedStateSource).toContain('toDiscoveryConfig');
    expect(resourceDetailDrawerDerivedStateSource).toContain(
      "from '@/components/Infrastructure/resourceDetailDiscoveryModel'",
    );
    expect(resourceDetailDrawerDerivedStateSource).toContain(
      "from './resourceDetailDrawerOperationalModel'",
    );
    expect(resourceDetailDrawerDerivedStateSource).toContain(
      "from './resourceDetailDrawerServiceModel'",
    );
    expect(resourceDetailDrawerDerivedStateSource).toContain(
      "from './resourceDetailDrawerIdentityModel'",
    );
    expect(resourceDetailDrawerDiscoveryModelSource).toContain('export const toDiscoveryConfig');
    expect(resourceDetailDrawerIdentityModelSource).toContain(
      'export const buildResourceIdentityView',
    );
    expect(resourceDetailDrawerIdentityModelSource).toContain(
      'export const buildDiscoveryContextSummary',
    );
    expect(resourceDetailDrawerIdentityModelSource).toContain(
      'export const buildResourceDebugBundle',
    );
    expect(resourceDetailDrawerDerivedStateSource).not.toContain('buildWorkloadsHref');
    expect(resourceDetailDrawerDerivedStateSource).not.toContain('buildServiceDetailLinks');
    expect(resourceDetailDrawerDerivedStateSource).not.toContain('const supportedBadge =');
    expect(resourceDetailDrawerDerivedStateSource).not.toContain(
      'const links: Array<{ href: string;',
    );
    expect(resourceDetailDrawerDerivedStateSource).not.toContain('ALIAS_COLLAPSE_THRESHOLD');
    expect(resourceDetailDrawerDerivedStateSource).not.toContain('formatIdentifierLabel');
    expect(resourceDetailDrawerOperationalModelSource).toContain(
      'export const buildKubernetesCapabilityBadges',
    );
    expect(resourceDetailDrawerOperationalModelSource).toContain('export const buildSourceSummary');
    expect(resourceDetailDrawerOperationalModelSource).toContain('export const buildHostDetailCards');
    expect(resourceDetailDrawerOperationalModelSource).toContain('export const buildRelatedLinks');
    expect(resourceDetailDrawerServiceModelSource).toContain('export const getServiceDetailsSummary');
    expect(resourceDetailDrawerServiceModelSource).toContain(
      'export const buildPbsVisibleJobBreakdown',
    );
    expect(resourceDetailDrawerServiceModelSource).toContain(
      'export const buildPmgVisibleQueueBreakdown',
    );
    expect(resourceDetailDrawerServiceModelSource).toContain(
      'export const buildPmgVisibleMailBreakdown',
    );
    expect(resourceDetailDrawerServiceModelSource).toContain("formatCount(pmg.queueTotal || 0, 'queued message')");
    expect(resourceDetailDrawerServiceModelSource).toContain("'delayed message'");
    expect(resourceDetailDrawerOverviewSource).not.toContain('MonitoringAPI.');
    expect(resourceDetailDrawerOverviewSource).toContain('drawer.queueDockerUpdateCheck');
    expect(resourceDetailDrawerOverviewSource).toContain('drawer.queueDockerUpdateAll');
    expect(resourceDetailDrawerOverviewSource).toContain('Filter history');
    expect(resourceDetailDrawerOverviewSource).not.toContain(
      'const modeLabel = formatSourceType(resource.sourceType);',
    );
    expect(resourceDetailDrawerOverviewSource).not.toContain('<span class="text-muted">Mode</span>');
    expect(resourceDetailDrawerDockerActionsStateSource).toContain('MonitoringAPI.checkDockerUpdates');
    expect(resourceDetailDrawerDockerActionsStateSource).toContain(
      'MonitoringAPI.updateAllDockerContainers',
    );
  });

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
      tags: ['timeline-tag'],
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
    expect(screen.queryByText('Summary')).toBeNull();
    expect(screen.getByText('Current state')).toBeInTheDocument();
    expect(screen.queryByText('Runtime')).toBeNull();
    expect(screen.getByText('Change history')).toBeInTheDocument();
    expect(screen.getByTestId('resource-secondary-sections').classList.contains('space-y-3')).toBe(
      true,
    );
    expect(screen.getByTestId('resource-support-sections').classList.contains('flex')).toBe(true);
    expect(screen.getByTestId('resource-support-sections').classList.contains('flex-wrap')).toBe(
      true,
    );
    expect(
      screen.getByTestId('resource-summary-section').querySelectorAll('.bg-surface-hover.px-2.py-2')
        .length,
    ).toBe(0);
    const summarySection = screen.getByTestId('resource-summary-section');
    expect(summarySection.classList.contains('grid')).toBe(true);
    expect(summarySection.classList.contains('gap-3')).toBe(true);
    expect(summarySection.classList.contains('sm:grid-cols-2')).toBe(true);
    expect(
      screen.getByTestId('resource-current-state-section').classList.contains('rounded-md'),
    ).toBe(true);
    expect(
      screen.getByTestId('resource-current-state-section').classList.contains('bg-surface'),
    ).toBe(true);
    expect(
      screen.getByTestId('resource-current-state-section').classList.contains('shadow-sm'),
    ).toBe(true);
    expect(screen.getByTestId('resource-identity-section').classList.contains('rounded-md')).toBe(
      true,
    );
    expect(screen.getByTestId('resource-identity-section').classList.contains('bg-surface')).toBe(
      true,
    );
    expect(screen.getByTestId('resource-identity-section').classList.contains('shadow-sm')).toBe(
      true,
    );
    expect(
      screen.getByTestId('resource-change-history-section').querySelector('.mt-3.grid.gap-2'),
    ).toBeNull();
    const currentStateSection = screen.getByTestId('resource-current-state-section');
    const identitySection = screen.getByTestId('resource-identity-section');
    expect(screen.queryByText('Host')).toBeNull();
    expect(screen.queryByText('Service')).toBeNull();
    expect(screen.queryByText('Supporting context')).toBeNull();
    expect(screen.queryByText('Mail details are only available for PMG resources.')).toBeNull();
    expect(
      screen.queryByText('Namespaces are only available for Kubernetes cluster resources.'),
    ).toBeNull();
    expect(
      screen.queryByText('Deployments are only available for Kubernetes cluster resources.'),
    ).toBeNull();
    expect(
      screen.queryByText(
        'Swarm details are only available for Docker runtimes reporting Swarm metadata.',
      ),
    ).toBeNull();
    expect(screen.queryByText('Container Updates')).toBeNull();
    expect(screen.queryByText('Check Updates')).toBeNull();
    expect(screen.queryByText('Show update controls')).toBeNull();
    expect(screen.getByText('Access')).toBeInTheDocument();
    expect(screen.queryByText('Analysis')).toBeNull();
    expect(
      screen.queryByText('Supporting metadata only. The web interface path above stays primary.'),
    ).toBeNull();
    expect(screen.getByRole('button', { name: 'Show access' })).toBeInTheDocument();
    expect(
      screen
        .getByTestId('resource-access-section')
        .querySelector('.mt-3.rounded.border.border-border.bg-surface.p-2\\.5'),
    ).toBeNull();
    expect(screen.queryByText('Details')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Show details' })).toBeNull();
    expect(screen.queryByText('Platform ID')).toBeNull();
    expect(currentStateSection.querySelector('.border-dashed')).toBeNull();
    expect(within(identitySection).getByText('Tags')).toBeInTheDocument();
    expect(within(currentStateSection).queryByText('Tags')).toBeNull();
    expect(
      within(changeHistorySection).queryByText('Filterable event history for this resource.'),
    ).toBeNull();
    expect(within(changeHistorySection).queryByText('Recent activity')).toBeNull();
    expect(screen.queryByText('Events')).toBeNull();
    expect(screen.getAllByText('Timeline 3')).toHaveLength(1);
    expect(
      Array.from(screen.getByTestId('resource-support-sections').children).map((node) =>
        node.getAttribute('data-testid'),
      ),
    ).toEqual([
      'resource-access-section',
      'resource-investigation-context',
    ]);
    expect(screen.getAllByText('Restart 2')).toHaveLength(1);
    expect(screen.getAllByText('Anomaly 1')).toHaveLength(1);
    expect(screen.getAllByText('Platform event 1')).toHaveLength(1);
    expect(screen.getAllByText('Pulse diff 2')).toHaveLength(1);
    expect(screen.getAllByText('Docker adapter 2')).toHaveLength(1);
    expect(screen.getAllByText('Proxmox adapter 1')).toHaveLength(1);
    expect(changeHistorySection.querySelectorAll('.mt-1.grid').length).toBe(0);
    expect(screen.queryByText('Quick links')).toBeNull();
    expect(screen.getByText('Context')).toBeInTheDocument();
    expect(screen.queryByText('Correlations')).toBeNull();
    expect(screen.queryByText('Storage 1 alias')).toBeNull();
    expect(screen.queryByText('VM Child')).toBeNull();
    expect(screen.queryByText('Capabilities 1')).toBeNull();
    expect(screen.queryByText('Relationships 1')).toBeNull();
    expect(screen.queryByText('AI')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Show access' }));
    expect(screen.getByText('Analysis')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open analysis' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Show context' }));
    await screen.findByText('AI');
    expect(
      screen
        .getByTestId('resource-investigation-context')
        .querySelectorAll('.rounded.border.border-border.bg-surface.p-3').length,
    ).toBe(0);
    expect(screen.getByText('Health')).toBeInTheDocument();
    expect(screen.getByText('A · 92/100')).toBeInTheDocument();
    expect(screen.getByText('Trend')).toBeInTheDocument();
    expect(screen.getByText('stable')).toBeInTheDocument();
    expect(screen.getByText('Notes')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(
      screen.getByTestId('resource-correlation-context').querySelector('.rounded.border'),
    ).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Show correlations' }));
    expect(
      screen
        .getByRole('button', { name: 'Hide correlations' })
        .parentElement?.querySelector('.mt-0\\.5.text-\\[10px\\].text-muted'),
    ).toBeNull();
  });

  it('keeps default internal cloud-summary posture out of the investigation context drawer block', async () => {
    aiIntelligenceMock.getResourceIntelligence.mockResolvedValueOnce({
      resource_id: 'agent-default-policy',
      health: {
        score: 100,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: '',
      },
      dependencies: [],
      dependents: [],
      correlations: [],
      recent_changes: [],
      note_count: 0,
    });

    const resource = baseResource({
      id: 'agent-default-policy',
      name: 'default-policy-host',
      displayName: 'Default Policy Host',
      policy: {
        sensitivity: 'internal',
        routing: {
          scope: 'cloud-summary',
        },
      },
      aiSafeSummary: 'agent resource; status online; sources agent',
    });

    render(() => <ResourceDetailDrawer resource={resource} />);

    await screen.findByText('Current state');
    expect(screen.queryByText('Context')).toBeNull();
    expect(screen.queryByText('Governance')).toBeNull();
    expect(screen.queryByText('AI-Safe Summary')).toBeNull();
    expect(screen.queryByText('Routing Cloud Summary')).toBeNull();
  });

  it('keeps details label-first without a summary sentence', () => {
    facetBundleMock.getFacetBundle.mockResolvedValueOnce({
      capabilities: [],
      relationships: [],
      recentChanges: [],
    });

    const resource = baseResource({
      id: 'agent-with-support',
      name: 'agent-with-support',
      displayName: 'Agent With Support',
      platformId: 'agent-with-support',
      sourceType: 'agent',
      identity: {
        hostname: 'agent-with-support.local',
      },
      tags: ['support-tag'],
      platformData: {
        sources: ['agent'],
        agent: {
          agentId: 'agent-support-1',
          hostname: 'agent-with-support.local',
        },
      },
    });

    render(() => <ResourceDetailDrawer resource={resource} />);

    expect(within(screen.getByTestId('resource-identity-section')).getByText('Aliases')).toBeInTheDocument();
    expect(within(screen.getByTestId('resource-current-state-section')).queryByText('Aliases')).toBeNull();
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
    expect(panel.queryByText('Event log')).toBeNull();
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

    expect(screen.getByText('Service')).toBeInTheDocument();
    expect(
      Array.from(screen.getByTestId('resource-support-sections').children).map((node) =>
        node.getAttribute('data-testid'),
      ),
    ).toEqual([
      'resource-access-section',
      'resource-service-details-section',
    ]);
    fireEvent.click(screen.getByRole('button', { name: 'Show service' }));
    const serviceDetails = within(screen.getByTestId('resource-service-details-section'));
    expect(
      screen.getByTestId('resource-service-details-section').querySelector('.mt-3.space-y-3'),
    ).toBeTruthy();
    expect(
      screen.getByTestId('resource-service-details-section').querySelector('.mt-3.grid'),
    ).toBeNull();
    expect(serviceDetails.getByText('PBS')).toBeInTheDocument();
    expect(serviceDetails.queryByText('PBS Service')).toBeNull();
    expect(serviceDetails.queryByText('Connection')).toBeNull();
    expect(screen.queryByText('Backup summary')).toBeNull();
    expect(screen.queryByText('Job breakdown')).toBeNull();
    expect(screen.queryByText('Types')).toBeNull();
    expect(screen.queryByText('Show job detail')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Show jobs' }));
    expect(screen.getByText('Types')).toBeInTheDocument();
  });

  it('keeps PMG node count out of the primary mail-flow metric grid', () => {
    facetBundleMock.getFacetBundle.mockResolvedValueOnce({
      capabilities: [],
      relationships: [],
      recentChanges: [],
    });

    const resource = baseResource({
      id: 'pmg-1',
      type: 'pmg',
      name: 'pmg-main',
      displayName: 'PMG Main',
      platformId: 'pmg-main',
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

    render(() => <ResourceDetailDrawer resource={resource} />);

    fireEvent.click(screen.getByRole('button', { name: 'Show service' }));
    fireEvent.click(screen.getByRole('button', { name: 'Show mail flow' }));
    expect(screen.getByText('Queue')).toBeInTheDocument();
    expect(screen.getByText('Backlog')).toBeInTheDocument();
    const supportContext = within(screen.getByTestId('pmg-support-context'));
    expect(supportContext.getByText('Nodes')).toBeInTheDocument();
    expect(supportContext.getByText('Updated')).toBeInTheDocument();
    expect(screen.getByText('Queue detail').closest('summary')?.textContent).toBe('Queue detail');
    expect(screen.getByText('Mail detail').closest('summary')?.textContent).toBe('Mail detail');
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
    expect(panel.queryByLabelText('Change kind')).toBeNull();
    expect(panel.getByRole('button', { name: 'Filter history' })).toBeInTheDocument();

    fireEvent.click(panel.getByRole('button', { name: 'Filter history' }));

    fireEvent.change(panel.getByLabelText('Change kind'), {
      target: { value: 'restart' },
    });
    fireEvent.change(panel.getByLabelText('Source type'), {
      target: { value: 'platform_event' },
    });

    expect(await panel.findByText('Filtered changes loaded')).toBeInTheDocument();
    expect(await panel.findByText('Timeline 1')).toBeInTheDocument();
    expect(panel.queryByText('Timeline 2')).toBeNull();
    expect(await panel.findByText('Routine restart requested')).toBeInTheDocument();
    expect(panel.queryByText('CPU spike detected')).toBeNull();

    fireEvent.click(panel.getByRole('button', { name: 'Clear filters' }));

    expect(await panel.findByText('Changes loaded')).toBeInTheDocument();
    expect(await panel.findByText('Timeline 2')).toBeInTheDocument();
    expect(panel.queryByText('Filtered changes loaded')).toBeNull();
    expect(panel.getByText('CPU spike detected')).toBeInTheDocument();
    expect(panel.queryByLabelText('Change kind')).toBeNull();
    expect(panel.getByRole('button', { name: 'Filter history' })).toBeInTheDocument();
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
    expect(panel.queryByLabelText('Source adapter')).toBeNull();

    fireEvent.click(panel.getByRole('button', { name: 'Filter history' }));

    fireEvent.change(panel.getByLabelText('Source adapter'), {
      target: { value: 'docker_adapter' },
    });

    expect(await panel.findByText('Filtered changes loaded')).toBeInTheDocument();
    expect(await panel.findByText('Timeline 1')).toBeInTheDocument();
    expect(panel.queryByText('Timeline 2')).toBeNull();
    expect(await panel.findByText('CPU spike detected')).toBeInTheDocument();
    expect(panel.queryByText('Routine restart requested')).toBeNull();
  });
});
