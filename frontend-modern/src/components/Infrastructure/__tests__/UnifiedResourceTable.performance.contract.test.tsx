import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, waitFor, within } from '@solidjs/testing-library';
import type { Resource } from '@/types/resource';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import { ResourceFacetSummary } from '@/components/Infrastructure/ResourceFacetSummary';
import { formatSensorName } from '@/components/Infrastructure/resourceDetailMappers';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
import unifiedResourceTableSource from '@/components/Infrastructure/UnifiedResourceTable.tsx?raw';
import unifiedResourceTableStateSource from '@/components/Infrastructure/useUnifiedResourceTableState.ts?raw';
import unifiedResourceTableViewportSyncSource from '@/components/Infrastructure/useUnifiedResourceTableViewportSync.ts?raw';
import unifiedResourceTableModelSource from '@/components/Infrastructure/unifiedResourceTableModel.ts?raw';
import unifiedResourceTableStateModelSource from '@/components/Infrastructure/unifiedResourceTableStateModel.ts?raw';
import infrastructureSummarySource from '@/components/Infrastructure/InfrastructureSummary.tsx?raw';
import infrastructureSummaryStateSource from '@/components/Infrastructure/useInfrastructureSummaryState.ts?raw';
import infrastructureSummaryModelSource from '@/components/Infrastructure/infrastructureSummaryModel.ts?raw';
import unifiedResourceHostTableCardSource from '@/components/Infrastructure/UnifiedResourceHostTableCard.tsx?raw';
import unifiedResourcePBSTableSectionSource from '@/components/Infrastructure/UnifiedResourcePBSTableSection.tsx?raw';
import unifiedResourcePMGTableSectionSource from '@/components/Infrastructure/UnifiedResourcePMGTableSection.tsx?raw';
import resourceDetailMappersSource from '@/components/Infrastructure/resourceDetailMappers.ts?raw';
import resourceDetailDiscoveryModelSource from '@/components/Infrastructure/resourceDetailDiscoveryModel.ts?raw';
import {
  buildStatusOptions,
  filterResources,
  matchesSearch,
  sortResources,
  groupResources,
  splitPrimaryAndServiceResources,
  computeIOScale,
} from '@/components/Infrastructure/infrastructureSelectors';
const resourceDetailDrawerMock = vi.hoisted(() => vi.fn());
const frontendIndexCssSource = readFileSync(join(process.cwd(), 'src/index.css'), 'utf8');
// Stub ResizeObserver for jsdom
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}
if (typeof Element.prototype.scrollIntoView !== 'function') {
  Element.prototype.scrollIntoView = vi.fn();
}

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: () => false,
    isVisible: () => true,
  }),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="metric-cell">metric</div>,
}));

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: Record<string, unknown>) => {
    resourceDetailDrawerMock(props);
    return <div data-testid="resource-drawer">drawer</div>;
  },
}));

const makeResource = (i: number, overrides?: Partial<Resource>): Resource => ({
  id: `resource-${i}`,
  type: i % 3 === 0 ? 'docker-host' : 'agent',
  name: `host-${i}`,
  displayName: `Host ${i}`,
  platformId: `platform-${i}`,
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: i % 10 === 0 ? 'offline' : 'online',
  cpu: { current: (i % 100) / 100 },
  memory: {
    current: (i % 80) / 100,
    total: 16 * 1024 * 1024 * 1024,
    used: ((i % 80) / 100) * 16 * 1024 * 1024 * 1024,
  },
  disk: { current: 0.3 },
  lastSeen: Date.now(),
  platformData: { sources: ['proxmox'] },
  ...overrides,
});

const PROFILES = {
  S: 250,
  M: 1000,
  L: 3000,
} as const;

const makeResources = (count: number, overrides?: (i: number) => Partial<Resource>): Resource[] =>
  Array.from({ length: count }, (_, i) => makeResource(i, overrides?.(i)));

const getTypeDistribution = (resources: Resource[]) =>
  resources.reduce(
    (acc, resource) => {
      if (resource.type === 'agent') acc.agent += 1;
      if (resource.type === 'docker-host') acc.dockerHost += 1;
      return acc;
    },
    { agent: 0, dockerHost: 0 },
  );

const expectedTypeDistribution = (count: number) => {
  const dockerHost = Math.floor((count - 1) / 3) + 1;
  const agent = count - dockerHost;
  return { agent, dockerHost };
};

const getBodyRowCount = (container: HTMLElement) => container.querySelectorAll('tbody tr').length;

describe('UnifiedResourceTable performance contract', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  describe('Fixture profile validation', () => {
    for (const [profile, count] of Object.entries(PROFILES)) {
      it(`builds profile ${profile} with stable size and type distribution`, () => {
        const resources = makeResources(count);
        const distribution = getTypeDistribution(resources);
        const expectedDistribution = expectedTypeDistribution(count);

        expect(resources).toHaveLength(count);
        expect(distribution).toEqual(expectedDistribution);
      });
    }
  });

  describe('Baseline structural contracts', () => {
    it('renders Profile S table structure and all rows', async () => {
      const resources = makeResources(PROFILES.S);
      const { container } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBeGreaterThan(0);
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBe(PROFILES.S);
      });
    });

    it('renders the shared facet summary component in timeline-only mode for canonical resource counts', async () => {
      const { getByText, queryByText } = render(() => (
      <ResourceFacetSummary
        counts={{
          recentChanges: 3,
          recentChangeKinds: {
            restart: 2,
              config_update: 1,
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
          }}
          recentChanges={[]}
        />
      ));

      expect(getByText('Timeline 3')).toBeInTheDocument();
      expect(getByText('Restart 2')).toBeInTheDocument();
      expect(getByText('Config update 1')).toBeInTheDocument();
      expect(getByText('Anomaly 1')).toBeInTheDocument();
      expect(getByText('Platform event 1')).toBeInTheDocument();
      expect(getByText('Pulse diff 2')).toBeInTheDocument();
      expect(getByText('Docker adapter 2')).toBeInTheDocument();
      expect(getByText('Proxmox adapter 1')).toBeInTheDocument();
      expect(queryByText('Capabilities 1')).toBeNull();
      expect(queryByText('Relationships 1')).toBeNull();
    });

    it('formats sensor labels through the shared resource detail mapper helper', () => {
      expect(formatSensorName('fan1_cpu_temp')).toBe('Cpu Temp');
      expect(formatSensorName('psu_temp')).toBe('Temp');
      expect(resourceDetailMappersSource).toContain('titleCaseDelimitedLabel');
      expect(resourceDetailMappersSource).not.toContain('export const toDiscoveryConfig');
      expect(resourceDetailDiscoveryModelSource).toContain('export const toDiscoveryConfig');
    });

    it('keeps hot-path table state and windowing in the shared table state owner', () => {
      expect(unifiedResourceTableSource).toContain('useUnifiedResourceTableState');
      expect(unifiedResourceTableSource).toContain('UnifiedResourceHostTableCard');
      expect(unifiedResourceTableSource).toContain('UnifiedResourceServiceInfrastructureCard');
      expect(unifiedResourceTableSource).not.toContain('const sortedPBSResources = createMemo(() =>');
      expect(unifiedResourceTableSource).not.toContain('const getOutlierEmphasis =');
      expect(unifiedResourceTableSource).not.toContain('const getPBSTableRow =');
      expect(unifiedResourceTableStateSource).toContain("from './unifiedResourceTableStateModel'");
      expect(unifiedResourceTableStateSource).toContain('buildHostTableItems');
      expect(unifiedResourceTableStateSource).toContain('getUnifiedResourceTableColumnStyles');
      expect(unifiedResourceTableStateSource).toContain('useTableWindowing');
      expect(unifiedResourceTableStateSource).toContain('useUnifiedResourceTableViewportSync');
      expect(unifiedResourceTableStateSource).not.toContain('const resourceColumnStyle = createMemo(() =>');
      expect(unifiedResourceTableStateSource).not.toContain("const showGroupHeaders = props.groupingMode === 'grouped'");
      expect(unifiedResourceTableStateSource).not.toContain('const items: HostTableItem[] = [];');
      expect(unifiedResourceTableStateSource).not.toContain('window.addEventListener');
      expect(unifiedResourceTableStateSource).not.toContain('getBoundingClientRect');
      expect(unifiedResourceTableViewportSyncSource).toContain('window.addEventListener');
      expect(unifiedResourceTableViewportSyncSource).toContain('getBoundingClientRect');
      expect(unifiedResourceTableViewportSyncSource).toContain('scrollIntoView');
      expect(unifiedResourceTableViewportSyncSource).toContain('hostWindowing.onScroll');
      expect(unifiedResourceTableModelSource).toContain('export const getPBSTableRow');
      expect(unifiedResourceTableModelSource).toContain('export const getPMGTableRow');
      expect(unifiedResourceTableModelSource).toContain('export const getOutlierEmphasis');
      expect(unifiedResourceTableStateModelSource).toContain('export const buildHostTableItems');
      expect(unifiedResourceTableStateModelSource).toContain(
        'export const getUnifiedResourceTableColumnStyles',
      );
      expect(unifiedResourceTableStateModelSource).toContain(
        'export const getNextUnifiedResourceTableSortState',
      );
    });

    it('keeps infrastructure summary fetch runtime out of the render shell', () => {
      expect(infrastructureSummarySource).toContain('useInfrastructureSummaryState');
      expect(infrastructureSummarySource).not.toContain('fetchInfrastructureSummaryAndCache');
      expect(infrastructureSummarySource).not.toContain('readInfrastructureSummaryCache');
      expect(infrastructureSummarySource).not.toContain('setInterval(');
      expect(infrastructureSummarySource).not.toContain('AbortController');
      expect(infrastructureSummaryStateSource).toContain('fetchInfrastructureSummaryAndCache');
      expect(infrastructureSummaryStateSource).toContain('readInfrastructureSummaryCache');
      expect(infrastructureSummaryStateSource).toContain('setInterval(');
      expect(infrastructureSummaryStateSource).toContain('AbortController');
      expect(infrastructureSummaryStateSource).toContain("eventBus.on('org_switched'");
      expect(infrastructureSummaryStateSource).toContain('buildInfrastructureDisplaySeries');
      expect(infrastructureSummaryStateSource).toContain('buildInfrastructureMetricSeries');
      expect(infrastructureSummaryStateSource).toContain('buildInfrastructureEmptyMessage');
      expect(infrastructureSummaryStateSource).not.toContain(
        'const match = allSeries.find((series) => series.id === focused);',
      );
      expect(infrastructureSummaryStateSource).not.toContain(
        "displaySeries().map((series) => ({",
      );
      expect(infrastructureSummaryStateSource).not.toContain(
        "isAwaitingFirstSample() ? 'Waiting for first sample' : 'No history yet'",
      );
      expect(infrastructureSummaryStateSource).not.toContain("fetchFailed() ? 'Trend data unavailable' : emptyHistoryLabel()");
      expect(infrastructureSummaryModelSource).toContain(
        'export function buildInfrastructureSummarySeries',
      );
      expect(infrastructureSummaryModelSource).toContain(
        'export function combineResourceThroughputSeries',
      );
      expect(infrastructureSummaryModelSource).toContain(
        'export function buildInfrastructureDisplaySeries',
      );
      expect(infrastructureSummaryModelSource).toContain(
        'export function buildInfrastructureMetricSeries',
      );
      expect(infrastructureSummaryModelSource).toContain(
        'export function shouldShowInfrastructureNetworkCard',
      );
    });

    it('keeps contextual resource-row emphasis on the shared active-row presentation contract', () => {
      expect(frontendIndexCssSource).toContain("tr[data-summary-row-active='true'] > td");
      expect(frontendIndexCssSource).toContain('--color-summary-row-bg');
      expect(frontendIndexCssSource).toContain('--color-summary-row-accent');

      for (const source of [
        unifiedResourceHostTableCardSource,
        unifiedResourcePBSTableSectionSource,
        unifiedResourcePMGTableSectionSource,
      ]) {
        expect(source).toContain('data-summary-row-active');
        expect(source).not.toContain('bg-blue-50 dark:bg-blue-900 ring-1 ring-blue-300');
        expect(source).not.toContain('bg-blue-100 dark:bg-blue-800');
      }
    });

    it('keeps source filtering on the shared canonical source-platform helper', () => {
      const resources = [
        makeResource(0, { platformData: { sources: ['proxmox'] } }),
        makeResource(1, { platformData: { sources: ['docker'] } }),
      ];

      const filtered = filterResources(resources, new Set(['proxmox-pve']), new Set(), []);

      expect(filtered).toHaveLength(1);
      expect(filtered[0]?.platformData?.sources).toEqual(['proxmox']);
    });

    it('keeps governed resource search aligned with the infrastructure display label', () => {
      const governedResource = makeResource(9, {
        name: 'secret-host-9',
        displayName: 'secret-host-9',
        policy: {
          sensitivity: 'restricted',
          routing: { scope: 'local-only', redact: ['hostname'] },
        },
        aiSafeSummary: 'Production Host',
      });

      expect(matchesSearch(governedResource, 'Production')).toBe(false);
      expect(matchesSearch(governedResource, 'secret-host-9')).toBe(true);
    });

    it('suppresses the default policy posture badges in host-table rows while preserving exceptional policy badges', async () => {
      const resources = [
        makeResource(0, {
          name: 'default-policy-host',
          displayName: 'Default Policy Host',
          policy: {
            sensitivity: 'internal',
            routing: { scope: 'cloud-summary' },
          },
        }),
        makeResource(1, {
          name: 'governed-host',
          displayName: 'Governed Host',
          policy: {
            sensitivity: 'sensitive',
            routing: { scope: 'local-first', redact: ['hostname'] },
          },
        }),
      ];

      const { container } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });

      const body = container.querySelector('tbody');
      expect(body).not.toBeNull();
      const bodyQueries = within(body as HTMLElement);

      expect(bodyQueries.queryByText('Internal')).not.toBeInTheDocument();
      expect(bodyQueries.queryByText('Cloud Summary')).not.toBeInTheDocument();
      expect(bodyQueries.getByText('Sensitive')).toBeInTheDocument();
      expect(bodyQueries.getByText('Local First')).toBeInTheDocument();
    });

    it('renders facet summary badges without changing the Profile S row budget', async () => {
      const resources = makeResources(PROFILES.S, (i) =>
        i === 0
          ? {
              capabilities: [
                {
                  name: 'restart',
                  type: 'native',
                  description: 'Restart the host',
                  minimumApprovalLevel: 'admin',
                },
              ],
              relationships: [
                {
                  sourceId: 'resource-0',
                  targetId: 'storage-1',
                  type: 'depends_on',
                  confidence: 0.91,
                  active: true,
                  discoverer: 'proxmox_adapter',
                  observedAt: new Date().toISOString(),
                  lastSeenAt: new Date().toISOString(),
                },
              ],
              facetCounts: {
                recentChanges: 3,
                recentChangeKinds: {
                  restart: 2,
                  config_update: 1,
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
              recentChanges: [],
            }
          : {},
      );
      const { container, getByText } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getByText('Timeline 3')).toBeInTheDocument();
        expect(getByText('Config update 1')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBe(PROFILES.S);
      });
    });

    it('renders service facet summary badges without changing the dedicated service row budget', async () => {
      const resources: Resource[] = [
        makeResource(0, {
          id: 'pbs-service',
          type: 'pbs',
          displayName: 'pbs-service',
          name: 'pbs-service',
          platformType: 'proxmox-pbs',
          sourceType: 'api',
          platformData: {
            sources: ['pbs'],
            pbs: {
              datastoreCount: 1,
              backupJobCount: 1,
            },
          },
          recentChanges: [
            {
              id: 'pbs-change-1',
              observedAt: new Date().toISOString(),
              resourceId: 'pbs-service',
              kind: 'state_transition',
              sourceType: 'pulse_diff',
              confidence: 'high',
            },
          ],
        }),
        makeResource(1, {
          id: 'pmg-service',
          type: 'pmg',
          displayName: 'pmg-service',
          name: 'pmg-service',
          platformType: 'proxmox-pmg',
          sourceType: 'api',
          platformData: {
            sources: ['pmg'],
            pmg: {
              queueTotal: 12,
              nodeCount: 1,
            },
          },
          recentChanges: [
            {
              id: 'pmg-change-1',
              observedAt: new Date().toISOString(),
              resourceId: 'pmg-service',
              kind: 'metric_anomaly',
              sourceType: 'heuristic',
              confidence: 'medium',
            },
          ],
        }),
      ];
      const { container, getByText } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBe(2);
      });

      const pbsRow = getByText('pbs-service').closest('tr');
      expect(pbsRow).toBeTruthy();
      if (pbsRow) {
        const row = within(pbsRow);
        expect(row.getByText('Timeline 1')).toBeInTheDocument();
        expect(row.queryByText('Capabilities 1')).toBeNull();
        expect(row.queryByText('Relationships 1')).toBeNull();
      }

      const pmgRow = getByText('pmg-service').closest('tr');
      expect(pmgRow).toBeTruthy();
      if (pmgRow) {
        const row = within(pmgRow);
        expect(row.getByText('Timeline 1')).toBeInTheDocument();
        expect(row.queryByText('Capabilities 1')).toBeNull();
        expect(row.queryByText('Relationships 1')).toBeNull();
      }
    });

    it('passes the canonical resource-label resolver into the drawer for related-resource chips', async () => {
      resourceDetailDrawerMock.mockClear();
      const resources = [
        makeResource(0, {
          id: 'resource-0',
          displayName: 'Host 0',
          name: 'host-0',
        }),
      ];
      const { container } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId="resource-0"
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(resourceDetailDrawerMock).toHaveBeenCalled();
      });

      const drawerProps = resourceDetailDrawerMock.mock.calls.at(-1)?.[0] as {
        resolveResourceLabel?: (resourceId: string) => string | undefined;
      };

      expect(drawerProps.resolveResourceLabel?.('resource-0')).toBe('Host 0');
    });
  });

  describe('Row windowing contracts', () => {
    it('Profile M: preserves row windowing with policy badge rendering enabled', async () => {
      const resources = makeResources(PROFILES.M, (i) =>
        i === 0
          ? {
              displayName: 'Sensitive Host',
              name: 'sensitive-host',
              aiSafeSummary: 'restricted host summary safe for remote AI consumption',
              policy: {
                sensitivity: 'restricted',
                routing: {
                  scope: 'local-only',
                  redact: ['hostname', 'alias'],
                },
              },
            }
          : {
              policy: {
                sensitivity: i % 2 === 0 ? 'restricted' : 'internal',
                routing: {
                  scope: i % 3 === 0 ? 'local-only' : 'local-first',
                  redact: ['hostname', 'alias'],
                },
              },
            },
      );
      const { container, getAllByText, getByTitle, queryByText } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={resources[0]?.id ?? null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        const rowCount = getBodyRowCount(container);
        expect(rowCount).toBeGreaterThan(0);
        expect(rowCount).toBeLessThanOrEqual(140);
      });
      await waitFor(() => {
        expect(getAllByText('Restricted').length).toBeGreaterThan(0);
      });
      expect(getPreferredInfrastructureDisplayName(resources[0]!)).toBe('Sensitive Host');
      expect(getByTitle('Sensitive Host')).toBeInTheDocument();
      expect(queryByText('Sensitive Host')).toBeInTheDocument();
    });

    it('Profile M: caps mounted rows when windowing is active', async () => {
      const resources = makeResources(PROFILES.M);
      const { container } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        const rowCount = getBodyRowCount(container);
        expect(rowCount).toBeGreaterThan(0);
        expect(rowCount).toBeLessThanOrEqual(140);
      });
    });

    it('Profile L: keeps mounted rows capped under large load', async () => {
      const resources = makeResources(PROFILES.L);
      const { container } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(
        () => {
          const rowCount = getBodyRowCount(container);
          expect(rowCount).toBeGreaterThan(0);
          expect(rowCount).toBeLessThanOrEqual(140);
        },
        { timeout: 15000 },
      );
    });
  });

  describe('Grouped vs flat mode contract', () => {
    it('renders grouped mode with profile S resources', async () => {
      const resources = makeResources(PROFILES.S, (i) => ({
        clusterId: i % 2 === 0 ? 'cluster-a' : 'cluster-b',
      }));
      const { container, getByText } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="grouped"
        />
      ));

      await waitFor(() => {
        expect(getByText('cluster-a')).toBeInTheDocument();
        expect(getByText('cluster-b')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBe(PROFILES.S + 2);
      });
    });

    it('renders flat mode with profile S resources', async () => {
      const resources = makeResources(PROFILES.S, (i) => ({
        clusterId: i % 2 === 0 ? 'cluster-a' : 'cluster-b',
      }));
      const { container, queryByText } = render(() => (
        <UnifiedResourceTable
          resources={resources}
          expandedResourceId={null}
          onExpandedResourceChange={vi.fn()}
          groupingMode="flat"
        />
      ));

      await waitFor(() => {
        expect(container.querySelector('table')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(getBodyRowCount(container)).toBe(PROFILES.S);
      });
      expect(queryByText('cluster-a')).not.toBeInTheDocument();
      expect(queryByText('cluster-b')).not.toBeInTheDocument();
    });
  });

  describe('Infrastructure derivation contracts', () => {
    it('filterResources returns correct count after source filter on Profile S', () => {
      const resources = makeResources(PROFILES.S);
      const proxmoxOnly = filterResources(resources, new Set(['proxmox-pve']), new Set(), []);
      expect(proxmoxOnly.length).toBe(PROFILES.S);
    });

    it('sortResources preserves array length for all sort keys', () => {
      const resources = makeResources(PROFILES.M);
      for (const key of ['default', 'name', 'cpu', 'memory', 'disk']) {
        const sorted = sortResources(resources, key, 'asc');
        expect(sorted).toHaveLength(PROFILES.M);
      }
    });

    it('buildStatusOptions keeps canonical statuses ahead of unknown filters', () => {
      expect(buildStatusOptions(new Set(['custom-state', 'paused', 'degraded']))).toEqual([
        { key: 'degraded', label: 'Degraded' },
        { key: 'paused', label: 'Paused' },
        { key: 'custom-state', label: 'custom-state' },
      ]);
    });

    it('groupResources produces correct total resource count in grouped mode', () => {
      const resources = makeResources(PROFILES.S, (i) => ({
        clusterId: `cluster-${i % 3}`,
      }));
      const grouped = groupResources(resources, 'grouped');
      const total = grouped.reduce((sum, group) => sum + group.resources.length, 0);
      expect(total).toBe(PROFILES.S);
      expect(grouped.length).toBe(3);
    });

    it('splitPrimaryAndServiceResources partitions without losing resources', () => {
      const resources = makeResources(PROFILES.S);
      const { primaryResources, services } = splitPrimaryAndServiceResources(resources);
      expect(primaryResources.length + services.length).toBe(PROFILES.S);
    });

    it('computeIOScale produces valid stats for Profile M', () => {
      const resources = makeResources(PROFILES.M, (i) => ({
        network: { rxBytes: i * 100, txBytes: i * 50 },
        diskIO: { readRate: i * 10, writeRate: i * 5 },
      }));
      const scale = computeIOScale(resources);
      expect(scale.network.count).toBeGreaterThan(0);
      expect(scale.diskIO.count).toBeGreaterThan(0);
      expect(scale.network.median).toBeGreaterThan(0);
    });
  });
});
