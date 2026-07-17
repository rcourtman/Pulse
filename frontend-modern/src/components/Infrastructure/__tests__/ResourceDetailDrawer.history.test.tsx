import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@solidjs/testing-library';
import { Suspense } from 'solid-js';

import discoveryTabSource from '@/components/Discovery/DiscoveryTab.tsx?raw';
import discoveryTabStateSource from '@/components/Discovery/useDiscoveryTabState.ts?raw';
import resourceDetailDrawerShellSource from '@/components/Infrastructure/ResourceDetailDrawer.tsx?raw';
import resourceDetailDrawerOverviewSource from '@/components/Infrastructure/ResourceDetailDrawerOverviewTab.tsx?raw';
import resourceDetailSummarySource from '@/components/Infrastructure/ResourceDetailSummary.tsx?raw';
import resourceInvestigationContextSource from '@/components/Infrastructure/ResourceInvestigationContextTables.tsx?raw';
import resourceActionHistorySource from '@/components/Infrastructure/ResourceActionHistory.tsx?raw';
import resourceDetailDrawerHistoryStateSource from '@/components/Infrastructure/useResourceDetailDrawerHistoryState.ts?raw';
import createNonSuspendingQuerySource from '@/hooks/createNonSuspendingQuery.ts?raw';
import resourceDetailDrawerDerivedStateSource from '@/components/Infrastructure/useResourceDetailDrawerDerivedState.ts?raw';
import resourceDetailDrawerDiscoveryModelSource from '@/components/Infrastructure/resourceDetailDiscoveryModel.ts?raw';
import resourceDetailDrawerIdentityModelSource from '@/components/Infrastructure/resourceDetailDrawerIdentityModel.ts?raw';
import resourceDetailDrawerOperationalModelSource from '@/components/Infrastructure/resourceDetailDrawerOperationalModel.ts?raw';
import resourceDetailDrawerServiceModelSource from '@/components/Infrastructure/resourceDetailDrawerServiceModel.ts?raw';
import resourceDetailDrawerVmwareModelSource from '@/components/Infrastructure/resourceDetailDrawerVmwareModel.ts?raw';
import resourceDetailDrawerTrueNASModelSource from '@/components/Infrastructure/resourceDetailDrawerTrueNASModel.ts?raw';
import resourceDetailDrawerDockerActionsStateSource from '@/components/Infrastructure/useResourceDetailDrawerDockerActionsState.ts?raw';
import resourceDetailDrawerStateSource from '@/components/Infrastructure/useResourceDetailDrawerState.ts?raw';
import actionAuditApiSource from '@/api/actionAudit.ts?raw';
import actionAuditPresentationSource from '@/utils/actionAuditPresentation.ts?raw';
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

const actionAuditMock = vi.hoisted(() => ({
  listActionAudits: vi.fn().mockResolvedValue({
    audits: [],
    count: 0,
    available: false,
  }),
}));

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
    getFacetBundle: facetBundleMock.getFacetBundle,
  },
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getResourceIntelligence: aiIntelligenceMock.getResourceIntelligence,
  },
}));

vi.mock('@/api/actionAudit', () => ({
  ActionAuditAPI: {
    listActionAudits: actionAuditMock.listActionAudits,
  },
}));

// Stub the operator-state client so the drawer's
// ResourceOperatorStateSection does not fan out a real network call
// during this test. Returns no-state (null) by default — the resource
// has no operator overrides, which is the default posture.
vi.mock('@/api/resourceOperatorState', () => ({
  getResourceOperatorState: vi.fn().mockResolvedValue(null),
  setResourceOperatorState: vi.fn(),
  clearResourceOperatorState: vi.fn(),
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
    expect(resourceDetailDrawerOverviewSource).toContain("from './ResourceDetailSummary'");
    expect(resourceDetailDrawerOverviewSource).toContain(
      "from './ResourceInvestigationContextTables'",
    );
    expect(resourceDetailSummarySource).toContain("from '@/components/shared/TagBadges'");
    expect(resourceInvestigationContextSource).toContain('<table');
    expect(resourceInvestigationContextSource).toContain('RESOURCE_SAFE_SUMMARY_LABEL');
    expect(resourceDetailDrawerOverviewSource).toContain('getAllFilterOptionLabel');
    expect(resourceDetailDrawerOverviewSource).not.toContain("'All kinds'");
    expect(resourceDetailDrawerOverviewSource).not.toContain("'All sources'");
    expect(resourceDetailDrawerOverviewSource).not.toContain("'All adapters'");
    expect(resourceDetailSummarySource).not.toContain("from '@/components/Workloads/TagBadges'");
    expect(resourceDetailDrawerShellSource).toContain("from './ResourceDetailDrawerOverviewTab'");
    expect(resourceDetailDrawerShellSource).toContain("from './ResourceDetailDrawerDebugTab'");
    expect(resourceDetailDrawerShellSource).toContain('data-testid="resource-header-badges"');
    expect(resourceDetailDrawerShellSource).toContain('drawer.headerBadges()');
    expect(resourceDetailDrawerShellSource).toContain('DrawerHeaderActionGroup');
    expect(resourceDetailDrawerShellSource).toContain('DrawerHeaderActionButton');
    expect(resourceDetailDrawerShellSource).toContain('DrawerHeaderIconButton');
    expect(resourceDetailDrawerShellSource).not.toContain('drawer.headerIdentity()');
    expect(resourceDetailDrawerShellSource).not.toContain('drawer.unifiedSourceBadges()');
    expect(resourceDetailDrawerShellSource).not.toContain('Change history');
    expect(resourceDetailDrawerStateSource).toContain(
      "from './useResourceDetailDrawerHistoryState'",
    );
    expect(resourceDetailDrawerStateSource).toContain(
      "from './useResourceDetailDrawerDerivedState'",
    );
    expect(resourceDetailDrawerStateSource).toContain(
      "from './useResourceDetailDrawerDockerActionsState'",
    );
    expect(resourceDetailDrawerStateSource).toContain(
      'const [showHistoryFilters, setShowHistoryFilters]',
    );
    expect(resourceDetailDrawerStateSource).not.toContain('createResource(');
    expect(resourceDetailDrawerStateSource).not.toContain('MonitoringAPI.');
    expect(resourceDetailDrawerHistoryStateSource).toContain(
      "from '@/hooks/createNonSuspendingQuery'",
    );
    expect(resourceDetailDrawerHistoryStateSource).toContain('createNonSuspendingQuery');
    expect(resourceDetailDrawerHistoryStateSource).not.toContain('createResource(');
    expect(resourceDetailDrawerHistoryStateSource).toContain('ResourceAPI.getFacetBundle');
    expect(resourceDetailDrawerHistoryStateSource).toContain('AIAPI.getResourceIntelligence');
    expect(resourceDetailDrawerHistoryStateSource).toContain('ActionAuditAPI.listActionAudits');
    expect(resourceDetailDrawerOverviewSource).toContain("from './ResourceActionHistory'");
    expect(resourceDetailDrawerOverviewSource).toContain(
      'dataTestId="resource-relationship-map-section"',
    );
    expect(resourceDetailDrawerHistoryStateSource).toContain('resourceFacetRelationships');
    expect(resourceDetailDrawerDerivedStateSource).toContain('options.resourceRelationships?.()');
    expect(resourceDetailDrawerDerivedStateSource).toContain('resource.relationships ?? []');
    expect(resourceActionHistorySource).toContain('getActionAuditRecordStatePresentation');
    expect(resourceActionHistorySource).toContain('getActionAuditResultPresentation');
    expect(resourceActionHistorySource).toContain('getActionAuditVerificationOutcomePresentation');
    expect(resourceActionHistorySource).toContain('formatActionApprovalPolicyLabel');
    expect(resourceActionHistorySource).toContain('getAPTActionPresentation');
    expect(resourceActionHistorySource).toContain('resource-apt-action-facts');
    expect(resourceActionHistorySource).toContain('resource-action-recovery-truth');
    expect(resourceActionHistorySource).toContain('resource-apt-action-next-step');
    // The audit history must surface the broker's read-after-write
    // verification outcome alongside the dispatch result, not silently
    // drop it. Pin the wiring so future refactors cannot regress to an
    // output-only render.
    expect(resourceActionHistorySource).toContain(
      'shouldRenderActionAuditVerification(props.audit)',
    );
    expect(resourceActionHistorySource).toContain('Legacy check passed (source unclassified)');
    expect(resourceActionHistorySource).toContain('Legacy check failed (source unclassified)');
    expect(actionAuditPresentationSource).toContain('resource_remediation_locked:');
    expect(actionAuditPresentationSource).toContain('Confirmed by executing agent');
    expect(actionAuditPresentationSource).toContain('Confirmed by independent observer');
    expect(actionAuditApiSource).toContain('/api/audit/actions');
    expect(actionAuditApiSource).toContain('ACTION_AUDIT_UNAVAILABLE_STATUSES');
    expect(actionAuditPresentationSource).toContain('pending_approval');
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
    expect(resourceDetailDrawerDerivedStateSource).toContain(
      "from './resourceDetailDrawerVmwareModel'",
    );
    expect(resourceDetailDrawerDerivedStateSource).toContain(
      "from './resourceDetailDrawerTrueNASModel'",
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
    expect(resourceDetailDrawerVmwareModelSource).toContain(
      'export const buildVMwareDetailSections',
    );
    expect(resourceDetailDrawerVmwareModelSource).toContain(
      'export const buildVMwareDetailsSummary',
    );
    expect(resourceDetailDrawerTrueNASModelSource).toContain(
      'export const buildTrueNASDetailSections',
    );
    expect(resourceDetailDrawerTrueNASModelSource).toContain(
      'export const buildTrueNASDetailsSummary',
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
    expect(resourceDetailDrawerOperationalModelSource).toContain(
      'export const buildHostDetailCards',
    );
    expect(resourceDetailDrawerOperationalModelSource).toContain('export const buildRelatedLinks');
    expect(resourceDetailDrawerServiceModelSource).toContain(
      'export const getServiceDetailsSummary',
    );
    expect(resourceDetailDrawerServiceModelSource).toContain(
      'export const buildPbsVisibleJobBreakdown',
    );
    expect(resourceDetailDrawerServiceModelSource).toContain(
      'export const buildPmgVisibleQueueBreakdown',
    );
    expect(resourceDetailDrawerServiceModelSource).toContain(
      'export const buildPmgVisibleMailBreakdown',
    );
    expect(resourceDetailDrawerServiceModelSource).toContain(
      "formatCount(pmg.queueTotal || 0, 'queued message')",
    );
    expect(resourceDetailDrawerServiceModelSource).toContain("'delayed message'");
    expect(resourceDetailDrawerOverviewSource).not.toContain('MonitoringAPI.');
    expect(resourceDetailDrawerOverviewSource).toContain('drawer.queueDockerUpdateCheck');
    expect(resourceDetailDrawerOverviewSource).toContain('drawer.queueDockerUpdateAll');
    expect(resourceDetailDrawerOverviewSource).toContain('Filter history');
    expect(resourceDetailDrawerOverviewSource).not.toContain(
      'const modeLabel = formatSourceType(resource.sourceType);',
    );
    expect(resourceDetailDrawerOverviewSource).not.toContain(
      '<span class="text-muted">Mode</span>',
    );
    expect(createNonSuspendingQuerySource).toContain('const retainedQueryCache = new Map<');
    expect(createNonSuspendingQuerySource).toContain(
      'export function resetCreateNonSuspendingQueryCacheForTest()',
    );
    expect(createNonSuspendingQuerySource).toContain('setResolvedOnce(true);');
    expect(createNonSuspendingQuerySource).toContain('setResolvedOnce(false);');
    expect(resourceDetailDrawerDockerActionsStateSource).toContain(
      'MonitoringAPI.checkDockerUpdates',
    );
    expect(resourceDetailDrawerDockerActionsStateSource).not.toContain(
      'MonitoringAPI.updateAllDockerContainers',
    );
  });

  it('keeps drawer history fetches out of the page-level suspense fallback', async () => {
    const pendingFacetBundle = new Promise(() => {});
    const pendingIntelligence = new Promise(() => {});
    facetBundleMock.getFacetBundle.mockImplementationOnce(() => pendingFacetBundle as never);
    aiIntelligenceMock.getResourceIntelligence.mockImplementationOnce(
      () => pendingIntelligence as never,
    );

    render(() => (
      <Suspense fallback={<div>Loading view...</div>}>
        <ResourceDetailDrawer resource={baseResource({ recentChanges: [] })} />
      </Suspense>
    ));

    await Promise.resolve();

    expect(screen.queryByText('Loading view...')).not.toBeInTheDocument();
    expect(screen.getByText('Current state')).toBeInTheDocument();
  });

  it('keeps table-row presentation local until the history disclosure is opened', async () => {
    facetBundleMock.getFacetBundle.mockClear();
    aiIntelligenceMock.getResourceIntelligence.mockClear();
    actionAuditMock.listActionAudits.mockClear();
    facetBundleMock.getFacetBundle.mockResolvedValue({
      capabilities: [],
      relationships: [],
      recentChanges: [
        {
          id: 'change-row-1',
          observedAt: '2026-03-18T12:06:00Z',
          resourceId: 'agent:truenas-main',
          kind: 'restart',
          sourceType: 'platform_event',
          sourceAdapter: 'truenas_adapter',
          confidence: 'high',
          reason: 'Service restart observed',
        },
      ],
      counts: {
        recentChanges: 1,
        recentChangeKinds: { restart: 1 },
        recentChangeSourceTypes: { platform_event: 1 },
        recentChangeSourceAdapters: { truenas_adapter: 1 },
      },
    });
    actionAuditMock.listActionAudits.mockResolvedValue({
      audits: [],
      count: 0,
      available: true,
    });

    render(() => (
      <ResourceDetailDrawer
        resource={baseResource({
          id: 'agent:truenas-main',
          name: 'truenas-main',
          displayName: 'truenas-main',
          platformId: 'truenas-main',
          platformType: 'truenas',
          recentChanges: [],
        })}
        presentation="table-row"
      />
    ));

    await Promise.resolve();

    expect(screen.getByText('Current state')).toBeInTheDocument();
    expect(screen.getByText('Identity')).toBeInTheDocument();
    // Collapsed disclosure is present, but nothing fetches and no history
    // panels render until the user opens it.
    expect(screen.getByTestId('resource-row-history-disclosure')).toBeInTheDocument();
    expect(screen.queryByText('Change history')).not.toBeInTheDocument();
    expect(screen.queryByText('Operator overrides')).not.toBeInTheDocument();
    expect(screen.queryByText('Maintenance verification')).not.toBeInTheDocument();
    expect(screen.queryByText('Action history')).not.toBeInTheDocument();
    expect(screen.queryByText('Refreshing changes...')).not.toBeInTheDocument();
    expect(screen.queryByText('No events yet.')).not.toBeInTheDocument();
    expect(facetBundleMock.getFacetBundle).not.toHaveBeenCalled();
    expect(aiIntelligenceMock.getResourceIntelligence).not.toHaveBeenCalled();
    expect(actionAuditMock.listActionAudits).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Show history' }));
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();

    expect(facetBundleMock.getFacetBundle).toHaveBeenCalledWith('agent:truenas-main', {
      limit: 25,
    });
    expect(actionAuditMock.listActionAudits).toHaveBeenCalledWith({
      resourceId: 'agent:truenas-main',
      limit: 5,
    });
    expect(screen.getByText('Change history')).toBeInTheDocument();
    expect(await screen.findByText('Service restart observed')).toBeInTheDocument();
    expect(screen.getByText('Action history')).toBeInTheDocument();
    // Collapsing keeps the loaded data without refetching.
    const fetchCount = facetBundleMock.getFacetBundle.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: 'Hide history' }));
    fireEvent.click(screen.getByRole('button', { name: 'Show history' }));
    await Promise.resolve();
    expect(facetBundleMock.getFacetBundle.mock.calls.length).toBe(fetchCount);
    facetBundleMock.getFacetBundle.mockReset();
    actionAuditMock.listActionAudits.mockReset();
    actionAuditMock.listActionAudits.mockResolvedValue({
      audits: [],
      count: 0,
      available: false,
    });
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
        },
      ],
    });

    render(() => (
      <ResourceDetailDrawer
        resource={resource}
        resolveResourceLabel={(resourceId) =>
          resourceId === 'node:pve-1'
            ? 'PVE Node 1'
            : resourceId === 'vm:42'
              ? 'VM 42'
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
    ).toEqual(['resource-access-section', 'resource-investigation-context']);
    expect(screen.getAllByText('Restart 2')).toHaveLength(1);
    expect(screen.getAllByText('Anomaly 1')).toHaveLength(1);
    expect(screen.getAllByText('Platform event 1')).toHaveLength(1);
    expect(screen.getAllByText('Pulse diff 2')).toHaveLength(1);
    expect(screen.getAllByText('Docker adapter 2')).toHaveLength(1);
    expect(screen.getAllByText('Proxmox adapter 1')).toHaveLength(1);
    expect(changeHistorySection.querySelectorAll('.mt-1.grid').length).toBe(0);
    expect(screen.queryByText('Quick links')).toBeNull();
    const relationshipMap = within(screen.getByTestId('resource-relationship-map-section'));
    expect(screen.getByText('Relationship map')).toBeInTheDocument();
    expect(relationshipMap.getByText('Canonical relationships')).toBeInTheDocument();
    // Legacy /infrastructure?resource=... cross-jumps were retired; the
    // relationship map renders resource labels as plain text now.
    expect(relationshipMap.queryAllByRole('link')).toHaveLength(0);
    expect(relationshipMap.getByText('PVE Node 1')).toBeInTheDocument();
    expect(relationshipMap.getByText('Runs On')).toBeInTheDocument();
    expect(screen.getByText('Depends on')).toBeInTheDocument();
    expect(screen.getByText('Used by')).toBeInTheDocument();
    expect(screen.getByText('Correlations')).toBeInTheDocument();
    expect(screen.getByText('Storage 1 alias')).toBeInTheDocument();
    expect(screen.getByText('VM Child')).toBeInTheDocument();
    expect(screen.getByText('Context')).toBeInTheDocument();
    expect(screen.queryByText('Capabilities 1')).toBeNull();
    expect(screen.queryByText('Relationships 1')).toBeNull();
    expect(screen.queryByText('Analysis')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Show access' }));
    expect(screen.getByText('Analysis')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open analysis' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Show context' }));
    const contextSection = screen.getByTestId('resource-investigation-context');
    await within(contextSection).findByText('Analysis');
    expect(contextSection.querySelector('table')).toBeTruthy();
    expect(contextSection.querySelector('tbody')).toBeTruthy();
    expect(
      contextSection.querySelectorAll('.rounded.border.border-border.bg-surface.p-3').length,
    ).toBe(0);
    expect(screen.getByText('Health')).toBeInTheDocument();
    expect(screen.getByText('A · 92/100')).toBeInTheDocument();
    expect(screen.getByText('Trend')).toBeInTheDocument();
    expect(screen.getByText('stable')).toBeInTheDocument();
    expect(screen.getByText('Notes')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.queryByTestId('resource-correlation-context')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Show correlations' })).toBeNull();
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
    expect(screen.queryByText('Safe Summary')).toBeNull();
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

    expect(
      within(screen.getByTestId('resource-identity-section')).getByText('Aliases'),
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId('resource-current-state-section')).queryByText('Aliases'),
    ).toBeNull();
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
    // Cross-jump to /infrastructure?resource=... retired; related resources
    // render as plain text now.
    expect(panel.queryAllByRole('link')).toHaveLength(0);
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

  it('surfaces resource-scoped action history from the canonical action audit API', async () => {
    actionAuditMock.listActionAudits.mockResolvedValueOnce({
      available: true,
      count: 2,
      resourceId: 'vm:action-42',
      audits: [
        {
          id: 'action-1',
          createdAt: '2026-04-29T12:00:00Z',
          updatedAt: '2026-04-29T12:05:00Z',
          state: 'completed',
          request: {
            requestId: 'req-1',
            resourceId: 'vm:action-42',
            capabilityName: 'restart_service',
            reason: 'Restart nginx after patching',
            requestedBy: 'agent:oncall-helper',
          },
          plan: {
            actionId: 'action-1',
            requestId: 'req-1',
            allowed: true,
            requiresApproval: true,
            approvalPolicy: 'admin',
            rollbackAvailable: true,
            preflight: {
              target: 'agent:node-1',
              currentState: 'nginx active',
              intendedChange: 'Restart nginx',
              dryRunAvailable: false,
              dryRunSummary: 'No provider-supported dry run is available for this action.',
              safetyChecks: ['Approval scoped to this resource.'],
              verificationSteps: ['Read back service state after execution.'],
              generatedAt: '2026-04-29T12:01:00Z',
            },
          },
          result: {
            success: true,
            output: 'nginx restarted',
          },
          verification: {
            ran: true,
            command: "systemctl is-active 'nginx'",
            output: 'active',
            success: true,
            ranAt: '2026-04-29T12:05:20Z',
          },
          verificationOutcome: {
            status: 'verified',
            evidenceSummary: 'Readback reported nginx active.',
          },
        },
        {
          id: 'action-refused',
          createdAt: '2026-04-29T12:10:00Z',
          updatedAt: '2026-04-29T12:10:30Z',
          state: 'failed',
          request: {
            requestId: 'req-refused',
            resourceId: 'vm:action-42',
            capabilityName: 'lock_remediation',
            reason: 'Patrol proposed remediation while remediation was locked',
            requestedBy: 'pulse_patrol',
          },
          plan: {
            actionId: 'action-refused',
            requestId: 'req-refused',
            allowed: true,
            requiresApproval: true,
            approvalPolicy: 'mfa',
            rollbackAvailable: false,
          },
          result: {
            success: false,
            errorMessage: 'resource_remediation_locked: operator lock is active',
          },
          verificationOutcome: {
            status: 'unverified',
            evidenceSummary: 'No dispatch occurred, so no verification probe ran.',
          },
        },
      ],
    });

    facetBundleMock.getFacetBundle.mockResolvedValueOnce({
      capabilities: [],
      relationships: [],
      recentChanges: [],
      counts: { recentChanges: 0 },
    });

    render(() => (
      <ResourceDetailDrawer
        resource={baseResource({
          id: 'vm:action-42',
          type: 'vm',
          name: 'action-vm',
          displayName: 'Action VM',
          platformType: 'proxmox-pve',
        })}
      />
    ));

    const actionHistoryNode = await screen.findByTestId('resource-action-history-section');
    const actionHistory = within(actionHistoryNode);

    expect(actionAuditMock.listActionAudits).toHaveBeenCalledWith({
      resourceId: 'vm:action-42',
      limit: 5,
    });
    expect(actionHistory.getByText('Action history')).toBeInTheDocument();
    expect(actionHistory.getByText('Actions 2')).toBeInTheDocument();
    expect(actionHistory.getByText('Actions loaded')).toBeInTheDocument();
    expect(actionHistory.getByText('Restart Service')).toBeInTheDocument();
    expect(actionHistory.getByText('Completed')).toBeInTheDocument();
    expect(actionHistory.getByText('agent:oncall-helper')).toBeInTheDocument();
    expect(actionHistory.getByText('Restart nginx after patching')).toBeInTheDocument();
    expect(actionHistory.getByText('Admin approval')).toBeInTheDocument();
    expect(actionHistory.getByText('Not available')).toBeInTheDocument();
    expect(actionHistory.getByText('Restart nginx')).toBeInTheDocument();
    expect(actionHistory.getByText('Approval scoped to this resource.')).toBeInTheDocument();
    expect(actionHistory.getByText('nginx restarted')).toBeInTheDocument();
    expect(actionHistory.getByText('Readback reported nginx active.')).toBeInTheDocument();
    expect(actionHistory.getAllByText('Legacy check passed (source unclassified)')).toHaveLength(2);
    expect(actionHistory.queryByText('Verified')).toBeNull();
    expect(actionHistory.getByText("systemctl is-active 'nginx'")).toBeInTheDocument();
    expect(actionHistory.getByText('active')).toBeInTheDocument();
    expect(actionHistory.getByText('Refused')).toBeInTheDocument();
    expect(actionHistory.getByText('Execution refused')).toBeInTheDocument();
    expect(actionHistory.getByText('Refused before dispatch')).toBeInTheDocument();
    expect(actionHistory.getByText('Resource remediation locked')).toBeInTheDocument();
    expect(
      actionHistory.getByText(
        'Pulse refused the action before dispatch because this resource is locked against automatic remediation.',
      ),
    ).toBeInTheDocument();
    expect(actionHistory.getByText('Recorded detail:')).toBeInTheDocument();
    expect(actionHistory.getByText('operator lock is active')).toBeInTheDocument();
    expect(actionHistory.getByText('Verification not confirmed')).toBeInTheDocument();
    expect(
      actionHistory.getByText(
        'Pulse did not receive verification evidence that confirmed the intended state.',
      ),
    ).toBeInTheDocument();
    expect(
      actionHistory.getByText('No dispatch occurred, so no verification probe ran.'),
    ).toBeInTheDocument();
    expect(actionHistoryNode.textContent).not.toContain('resource_remediation_locked:');
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
          jobHealthEvidenceCount: 2,
          jobHealthEvidence: [
            {
              id: 'backup:task-history:fast:vm/100',
              family: 'backup',
              store: 'fast',
              confidence: 'direct-task-match',
              evidenceSource: 'pbs-task-history',
              evidenceScope: 'task-history',
              'last-run-state': 'OK',
              'last-run-upid': 'UPID:backup:1',
              'last-run-endtime': 1776717000,
              freshness: {
                observedAt: '2026-04-20T21:30:00Z',
                state: 'observed',
              },
              posture: 'healthy',
            },
            {
              id: 'prune:partial',
              family: 'prune',
              store: 'archive',
              confidence: 'partial-permission',
              evidenceSource: 'pbs-partial-read',
              evidenceScope: 'partial-read',
              freshness: {
                observedAt: '2026-04-20T21:30:00Z',
                state: 'partial',
              },
              posture: 'unknown',
              postureReason: 'PBS token cannot read prune job configuration.',
              error: 'permission denied',
            },
          ],
        },
      },
    });

    render(() => <ResourceDetailDrawer resource={resource} />);

    expect(screen.getByText('Service')).toBeInTheDocument();
    expect(
      Array.from(screen.getByTestId('resource-support-sections').children).map((node) =>
        node.getAttribute('data-testid'),
      ),
    ).toEqual(['resource-access-section', 'resource-service-details-section']);
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
    expect(screen.queryByText('Job health evidence')).toBeNull();
    expect(screen.queryByText('Show job detail')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Show jobs' }));
    expect(screen.getByText('Types')).toBeInTheDocument();
    const evidence = within(screen.getByTestId('pbs-job-health-evidence'));
    expect(evidence.getByText('2 evidence records')).toBeInTheDocument();
    expect(evidence.getByText('Observed backup task history')).toBeInTheDocument();
    expect(evidence.getByText('Partial PBS read')).toBeInTheDocument();
    expect(evidence.getByText('Partial read')).toBeInTheDocument();
    expect(evidence.getByText('Permission gap')).toBeInTheDocument();
    expect(evidence.queryByText(/scheduled backup/i)).toBeNull();
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
