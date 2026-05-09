import {
  For,
  Show,
  createMemo,
  createSignal,
  onCleanup,
  onMount,
  type Accessor,
  type Component,
} from 'solid-js';
import { Cpu, Plus, RotateCw, Search, SlidersHorizontal } from 'lucide-solid';
import SettingsPanel from '@/components/shared/SettingsPanel';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import {
  getGroupedTableRowCellClass,
  getGroupedTableRowClass,
} from '@/components/shared/groupedTableRowPresentation';
import {
  connectionAgentVersionPresentation,
  fleetSignalClassName,
  infrastructureSourcePresentation,
  surfaceLabel,
  type FleetGovernanceSignal,
  type InfrastructureSystemRow,
} from './connectionsTableModel';
import type { DiscoveredServer, DiscoveryScanStatus } from './infrastructureSettingsModel';
import {
  getInfrastructureCoverageCompleteActionPresentation,
  getInfrastructureEmptyStateDetail,
  getInfrastructureEmptyStateSummary,
  getInfrastructureOnboardingProductPresentation,
  getInfrastructureSourceManagerProducts,
  getInfrastructureSourcePickerItemForRouteStep,
  type InfrastructureOnboardingConnectionType,
  type InfrastructureSourcePickerRouteStep,
} from '@/utils/infrastructureOnboardingPresentation';
import { getAgentHostProfileFamily } from '@/utils/platformSupportManifest';

interface InfrastructureSourceManagerProps {
  rows: Accessor<readonly InfrastructureSystemRow[]>;
  discoveredNodes: Accessor<readonly DiscoveredServer[]>;
  discoveryEnabled: boolean;
  discoveryScanStatus: Accessor<DiscoveryScanStatus>;
  readOnly: boolean;
  onAddSource?: (type: InfrastructureOnboardingConnectionType) => void;
  onAddSourceStep?: (step: InfrastructureSourcePickerRouteStep) => void;
  onAddInfrastructure?: () => void;
  onRunDiscovery?: () => void;
  onOpenDiscoverySettings?: () => void;
  onOpenConnection?: (row: InfrastructureSystemRow) => void;
  onReviewDiscoveredSource?: (server: DiscoveredServer) => void;
}

const inlineButtonClass =
  'inline-flex min-w-[4.5rem] items-center justify-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const addSectionButtonClass =
  'inline-flex min-w-[4.5rem] items-center justify-center gap-1.5 rounded-md border border-blue-200 bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-200 dark:hover:bg-blue-900/40';
const utilityToolbarButtonClass =
  'inline-flex min-h-9 items-center justify-center gap-1.5 rounded-md border border-transparent px-2.5 py-2 text-sm font-medium text-muted transition-colors hover:border-border hover:bg-surface hover:text-base-content disabled:cursor-not-allowed disabled:opacity-60';
const workspacePrimaryButtonClass =
  'inline-flex min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60';
const workspaceSecondaryButtonClass =
  'inline-flex min-h-9 items-center justify-center gap-2 rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const discoveryRowClass =
  'border-b border-border-subtle bg-blue-50/30 hover:bg-blue-50/40 dark:bg-blue-950/10 dark:hover:bg-blue-950/20';
const wrappedFieldClass = 'whitespace-normal break-words leading-4';
const CARD_LAYOUT_MAX_WIDTH_PX = 767;

const sortRows = (rows: readonly InfrastructureSystemRow[]): InfrastructureSystemRow[] =>
  [...rows].sort((left, right) => left.name.localeCompare(right.name));

const sortDiscoveredNodes = (nodes: readonly DiscoveredServer[]): DiscoveredServer[] =>
  [...nodes].sort((left, right) => {
    const leftLabel = (left.hostname || left.ip).toLowerCase();
    const rightLabel = (right.hostname || right.ip).toLowerCase();
    return leftLabel.localeCompare(rightLabel);
  });

const normalizeDiscoveryTimestamp = (value?: number): number | undefined => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return undefined;
  }

  return value < 1_000_000_000_000 ? value * 1000 : value;
};

const formatRelativeTimestamp = (value?: number): string | undefined => {
  const timestamp = normalizeDiscoveryTimestamp(value);
  if (!timestamp) return undefined;

  const diff = Math.max(0, Date.now() - timestamp);
  const sec = Math.floor(diff / 1000);
  if (sec < 60) return `${sec}s ago`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const days = Math.floor(hr / 24);
  return `${days}d ago`;
};

const discoveredServerName = (server: DiscoveredServer): string =>
  server.hostname?.trim() || server.ip;

const discoveredServerEndpoint = (server: DiscoveredServer): string =>
  `https://${server.hostname?.trim() || server.ip}:${server.port}`;

const discoveredCoverageText = (server: DiscoveredServer): string => {
  const keys = getInfrastructureOnboardingProductPresentation(server.type).defaultSurfaceKeys;
  if (keys.length === 0) return '';
  return keys.map(surfaceLabel).join(', ');
};

const agentMethodTitleFor = (row: InfrastructureSystemRow): string | undefined => {
  const agentConnections = row.attachedConnections.filter(
    (connection) => connection.type === 'agent',
  );
  if (row.connection.type === 'agent') {
    agentConnections.unshift(row.connection);
  }
  if (agentConnections.length === 0) return undefined;
  if (agentConnections.length === 1) {
    return connectionAgentVersionPresentation(agentConnections[0])?.title ?? 'Pulse Agent attached';
  }
  return `${agentConnections.length} Pulse Agent attachments`;
};

const memberMethodTitleFor = (
  row: InfrastructureSystemRow,
  memberIndex: number,
): string | undefined => {
  const member = row.members[memberIndex];
  if (!member?.agentConnection) return undefined;
  return (
    connectionAgentVersionPresentation(member.agentConnection)?.title ??
    'Pulse Agent attached to cluster member'
  );
};

type SetupConfidenceActionKind = 'add' | 'agent' | 'detect' | 'review' | 'scan';

interface SetupConfidenceAction {
  kind: SetupConfidenceActionKind;
  label: string;
  detail: string;
  onClick?: () => void;
  disabled?: boolean;
}

interface ConfiguredSourceGroup {
  id: string;
  label: string;
  actionLabel: string;
  actionType: InfrastructureOnboardingConnectionType;
  actionStep?: InfrastructureSourcePickerRouteStep;
  rows: InfrastructureSystemRow[];
}

const formatCount = (count: number, noun: string): string =>
  `${count} ${noun}${count === 1 ? '' : 's'}`;

const rowHasApiCoverage = (row: InfrastructureSystemRow): boolean =>
  row.source === 'api' ||
  row.source === 'both' ||
  row.members.some((member) => member.source === 'api' || member.source === 'both');

const rowHasAgentCoverage = (row: InfrastructureSystemRow): boolean =>
  row.source === 'agent' ||
  row.source === 'both' ||
  row.attachedConnections.some((connection) => connection.type === 'agent') ||
  row.members.some(
    (member) => member.source === 'agent' || member.source === 'both' || member.agentConnection,
  );

const rowFleetSignals = (row: InfrastructureSystemRow): FleetGovernanceSignal[] => [
  ...row.fleetSignals,
  ...row.members.flatMap((member) => member.fleetSignals),
];

const rowHasFleetTone = (
  row: InfrastructureSystemRow,
  predicate: (signal: FleetGovernanceSignal) => boolean,
): boolean => rowFleetSignals(row).some(predicate);

const agentHostProfileRouteStep = (
  row: InfrastructureSystemRow,
): InfrastructureSourcePickerRouteStep | null => {
  if (row.connection.type !== 'agent') return null;
  const hostProfile = row.connection.agentIdentity?.hostProfile?.trim().toLowerCase();
  if (!hostProfile) return null;
  return getInfrastructureSourcePickerItemForRouteStep(hostProfile)?.routeStep ?? null;
};

const agentHostProfileGroupLabel = (row: InfrastructureSystemRow): string | null => {
  if (row.connection.type !== 'agent') return null;
  return getAgentHostProfileFamily(row.connection.agentIdentity?.hostProfile) ?? null;
};

export const InfrastructureSourceManager: Component<InfrastructureSourceManagerProps> = (props) => {
  let layoutContainerRef: HTMLDivElement | undefined;
  const products = createMemo(() => getInfrastructureSourceManagerProducts());

  const groupedConfiguredRows = createMemo(() => {
    const next = new Map<InfrastructureOnboardingConnectionType, InfrastructureSystemRow[]>();
    for (const product of products()) {
      next.set(product.type, []);
    }

    for (const row of props.rows()) {
      const productRows = next.get(row.ownerType as InfrastructureOnboardingConnectionType);
      if (!productRows) continue;
      productRows.push(row);
    }

    for (const [type, rows] of next.entries()) {
      next.set(type, sortRows(rows));
    }

    return next;
  });

  const groupedDiscoveredRows = createMemo(() => {
    const next = new Map<InfrastructureOnboardingConnectionType, DiscoveredServer[]>();
    for (const product of products()) {
      next.set(product.type, []);
    }

    for (const server of props.discoveredNodes()) {
      const productRows = next.get(server.type as InfrastructureOnboardingConnectionType);
      if (!productRows) continue;
      productRows.push(server);
    }

    for (const [type, rows] of next.entries()) {
      next.set(type, sortDiscoveredNodes(rows));
    }

    return next;
  });

  const sortedProducts = createMemo(() =>
    products().filter((product) => {
      const configuredCount = groupedConfiguredRows().get(product.type)?.length ?? 0;
      const discoveredCount = groupedDiscoveredRows().get(product.type)?.length ?? 0;
      return configuredCount + discoveredCount > 0;
    }),
  );

  const configuredSourceGroupsForProduct = (
    product: ReturnType<typeof getInfrastructureSourceManagerProducts>[number],
    rows: readonly InfrastructureSystemRow[],
  ): ConfiguredSourceGroup[] => {
    if (product.type !== 'agent') {
      return [
        {
          id: product.type,
          label: product.label,
          actionLabel: product.actionLabel,
          actionType: product.type,
          rows: [...rows],
        },
      ];
    }

    const profileGroups = new Map<string, ConfiguredSourceGroup>();
    const genericRows: InfrastructureSystemRow[] = [];

    for (const row of rows) {
      const profileLabel = agentHostProfileGroupLabel(row);
      const profileStep = agentHostProfileRouteStep(row);
      if (!profileLabel || !profileStep) {
        genericRows.push(row);
        continue;
      }

      const existing = profileGroups.get(profileStep);
      if (existing) {
        existing.rows.push(row);
        continue;
      }

      profileGroups.set(profileStep, {
        id: `agent-profile:${profileStep}`,
        label: profileLabel,
        actionLabel: `Add ${profileLabel}`,
        actionType: product.type,
        actionStep: profileStep,
        rows: [row],
      });
    }

    return [
      ...Array.from(profileGroups.values()).sort((left, right) =>
        left.label.localeCompare(right.label),
      ),
      ...(genericRows.length > 0
        ? [
            {
              id: product.type,
              label: product.label,
              actionLabel: product.actionLabel,
              actionType: product.type,
              rows: genericRows,
            },
          ]
        : []),
    ];
  };

  const fallbackConfiguredGroupForProduct = (
    product: ReturnType<typeof getInfrastructureSourceManagerProducts>[number],
  ): ConfiguredSourceGroup => ({
    id: product.type,
    label: product.label,
    actionLabel: product.actionLabel,
    actionType: product.type,
    rows: [],
  });

  const visibleSourceGroupsForProduct = (
    product: ReturnType<typeof getInfrastructureSourceManagerProducts>[number],
    configuredGroups: readonly ConfiguredSourceGroup[],
    hasDiscoveredRows: boolean,
  ): ConfiguredSourceGroup[] => {
    const groups = [...configuredGroups];
    const hasFallbackGroup = groups.some((group) => group.id === product.type);
    if (groups.length === 0 || (hasDiscoveredRows && !hasFallbackGroup)) {
      groups.push(fallbackConfiguredGroupForProduct(product));
    }
    return groups;
  };

  const handleGroupAddSource = (group: ConfiguredSourceGroup) => {
    if (group.actionStep && props.onAddSourceStep) {
      props.onAddSourceStep(group.actionStep);
      return;
    }
    props.onAddSource?.(group.actionType);
  };

  const hasAnyConfigured = createMemo(() => props.rows().length > 0);
  const hasAnyDiscovered = createMemo(() => props.discoveredNodes().length > 0);

  const rowInteractive = (row: InfrastructureSystemRow): boolean =>
    !props.readOnly && Boolean(props.onOpenConnection) && (row.canEdit || row.isAgent);

  const actionColumnVisible = () => !props.readOnly;
  const lastDiscoveryResultText = createMemo(() =>
    formatRelativeTimestamp(props.discoveryScanStatus().lastResultAt),
  );
  const connectedSystemCount = createMemo(() => props.rows().length);
  const discoveredCandidateCount = createMemo(() => props.discoveredNodes().length);
  const apiOnlySystems = createMemo(() =>
    props.rows().filter((row) => rowHasApiCoverage(row) && !rowHasAgentCoverage(row)),
  );
  const apiOnlySystemCount = createMemo(() => apiOnlySystems().length);
  // Names list keeps the descriptive 'Install agents' hint actionable: when
  // there are 1 or 2 systems missing an agent, surface their names directly
  // so the user knows exactly which boxes the install applies to.
  const apiOnlySystemNamesText = createMemo(() => {
    const names = apiOnlySystems().map((row) => row.name).filter(Boolean);
    if (names.length === 0) return null;
    if (names.length === 1) return names[0];
    if (names.length === 2) return `${names[0]} and ${names[1]}`;
    return null;
  });
  const liveFleetSystemCount = createMemo(
    () =>
      props
        .rows()
        .filter((row) =>
          rowHasFleetTone(row, (signal) => signal.key === 'liveness' && signal.tone === 'ok'),
        ).length,
  );
  const fleetAttentionSystemCount = createMemo(
    () =>
      props
        .rows()
        .filter((row) =>
          rowHasFleetTone(row, (signal) => signal.tone === 'warning' || signal.tone === 'critical'),
        ).length,
  );
  const discoveryReadinessLabel = createMemo(() => {
    if (props.discoveryScanStatus().scanning) return 'Scanning now';
    if (discoveredCandidateCount() > 0) return `${discoveredCandidateCount()} to review`;
    const lastResult = lastDiscoveryResultText();
    if (lastResult) return `Last scan ${lastResult}`;
    return props.discoveryEnabled ? 'Ready to scan' : 'Discovery off';
  });
  const setupConfidenceAction = createMemo<SetupConfidenceAction>(() => {
    const discoveredCandidates = props.discoveredNodes();
    if (discoveredCandidates.length > 0 && props.onReviewDiscoveredSource) {
      return {
        kind: 'review',
        label: discoveredCandidates.length === 1 ? 'Review candidate' : 'Review first candidate',
        detail: `${formatCount(discoveredCandidates.length, 'candidate')} discovered and waiting to be attached to the infrastructure model.`,
        onClick: () => props.onReviewDiscoveredSource?.(discoveredCandidates[0]),
      };
    }

    if (connectedSystemCount() === 0) {
      return {
        kind: 'add',
        label: 'Add infrastructure',
        detail: 'Add a platform, host, NAS, cluster, or endpoint to start monitoring.',
        onClick: props.onAddInfrastructure,
      };
    }

    if (apiOnlySystemCount() > 0 && props.onAddSource) {
      const namesText = apiOnlySystemNamesText();
      const target = namesText ?? formatCount(apiOnlySystemCount(), 'API-backed system');
      return {
        kind: 'agent',
        label: 'Install agents',
        detail: `Install Pulse Agent on ${target} when you want node-local telemetry such as temperatures, SMART data, and host identity.`,
        onClick: () => props.onAddSource?.('agent'),
      };
    }

    if (props.onRunDiscovery && props.discoveryEnabled && !lastDiscoveryResultText()) {
      return {
        kind: 'scan',
        label: props.discoveryScanStatus().scanning ? 'Scanning networks' : 'Scan networks',
        detail:
          'Run discovery to check whether more platform APIs are waiting on the configured networks.',
        onClick: props.onRunDiscovery,
        disabled: props.discoveryScanStatus().scanning,
      };
    }

    const completeAction = getInfrastructureCoverageCompleteActionPresentation();
    return {
      kind: 'add',
      label: completeAction.label,
      detail: completeAction.detail,
    };
  });
  const setupSummaryMetrics = createMemo(() => [
    {
      label: 'Systems',
      value: formatCount(connectedSystemCount(), 'system'),
    },
    {
      label: 'Live',
      value: formatCount(liveFleetSystemCount(), 'system'),
    },
    {
      label: 'Needs attention',
      value: formatCount(fleetAttentionSystemCount(), 'system'),
    },
    {
      label: 'Needs agent',
      value: formatCount(apiOnlySystemCount(), 'system'),
    },
    {
      label: 'Discovery',
      value: discoveryReadinessLabel(),
    },
  ]);

  const [layoutWidth, setLayoutWidth] = createSignal(
    typeof window !== 'undefined' ? window.innerWidth : 1024,
  );

  const updateLayoutWidth = (width: number) => {
    if (!Number.isFinite(width) || width <= 0) return;
    setLayoutWidth(Math.round(width));
  };

  const measureLayoutWidth = () => {
    const width = layoutContainerRef?.getBoundingClientRect().width;
    if (typeof width === 'number') {
      updateLayoutWidth(width);
    }
  };

  onMount(() => {
    measureLayoutWidth();

    if (typeof ResizeObserver === 'undefined' || !layoutContainerRef) {
      const handleResize = () => measureLayoutWidth();
      window.addEventListener('resize', handleResize);
      onCleanup(() => window.removeEventListener('resize', handleResize));
      return;
    }

    const observer = new ResizeObserver((entries) => {
      const entryWidth = entries[0]?.contentRect.width;
      if (typeof entryWidth === 'number') {
        updateLayoutWidth(entryWidth);
        return;
      }
      measureLayoutWidth();
    });
    observer.observe(layoutContainerRef);
    onCleanup(() => observer.disconnect());
  });
  const useCardLayout = createMemo(() => layoutWidth() <= CARD_LAYOUT_MAX_WIDTH_PX);

  const headerActions = () => (
    <Show when={!props.readOnly}>
      <div class="border-b border-border bg-surface px-4 py-3">
        <div class="flex flex-wrap items-center gap-2">
          <Show when={props.onAddInfrastructure}>
            <button
              type="button"
              onClick={props.onAddInfrastructure}
              class={workspacePrimaryButtonClass}
            >
              <Plus class="h-4 w-4" />
              Add infrastructure
            </button>
          </Show>

          <div class="ml-auto flex flex-wrap items-center gap-2">
            <Show when={props.onRunDiscovery}>
              <button
                type="button"
                onClick={props.onRunDiscovery}
                disabled={props.discoveryScanStatus().scanning}
                class={utilityToolbarButtonClass}
                aria-label="Run discovery"
                title="Run discovery"
              >
                <RotateCw
                  class={`h-4 w-4 ${props.discoveryScanStatus().scanning ? 'animate-spin' : ''}`}
                />
                {props.discoveryScanStatus().scanning ? 'Scanning…' : 'Run discovery'}
              </button>
            </Show>

            <Show when={props.onOpenDiscoverySettings}>
              <button
                type="button"
                onClick={props.onOpenDiscoverySettings}
                class={utilityToolbarButtonClass}
                aria-label="Discovery settings"
                title="Discovery settings"
              >
                <SlidersHorizontal class="h-4 w-4" />
                Discovery settings
              </button>
            </Show>
          </div>
        </div>
      </div>
    </Show>
  );

  const setupConfidenceActionIcon = (kind: SetupConfidenceActionKind) => (
    <>
      <Show when={kind === 'agent'}>
        <Cpu class="h-4 w-4" />
      </Show>
      <Show when={kind === 'detect' || kind === 'review'}>
        <Search class="h-4 w-4" />
      </Show>
      <Show when={kind === 'scan'}>
        <RotateCw class={`h-4 w-4 ${props.discoveryScanStatus().scanning ? 'animate-spin' : ''}`} />
      </Show>
      <Show when={kind === 'add'}>
        <Plus class="h-4 w-4" />
      </Show>
    </>
  );

  const setupSummaryBand = () => (
    <section
      aria-label="Infrastructure setup summary"
      class="border-b border-border bg-surface px-4 py-3"
    >
      <h3 class="sr-only">Setup status</h3>
      <div class="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <dl class="grid min-w-0 flex-1 grid-cols-2 overflow-hidden rounded-md border border-border-subtle bg-border-subtle sm:grid-cols-3 xl:grid-cols-5">
          <For each={setupSummaryMetrics()}>
            {(metric, index) => (
              <div
                class={`bg-surface px-3 py-3 ${
                  index() === setupSummaryMetrics().length - 1 ? 'col-span-2 sm:col-span-1' : ''
                }`}
              >
                <dt class="text-[11px] font-medium uppercase tracking-[0.08em] text-muted">
                  {metric.label}
                </dt>
                <dd class="mt-1 text-sm font-semibold text-base-content">{metric.value}</dd>
              </div>
            )}
          </For>
        </dl>
        <Show when={!props.readOnly && Boolean(setupConfidenceAction().onClick)}>
          <button
            type="button"
            onClick={() => setupConfidenceAction().onClick?.()}
            disabled={setupConfidenceAction().disabled}
            class={`${workspaceSecondaryButtonClass} self-start xl:self-center`}
          >
            {setupConfidenceActionIcon(setupConfidenceAction().kind)}
            {setupConfidenceAction().label}
          </button>
        </Show>
      </div>

      <p class="mt-3 text-xs leading-5 text-muted">{setupConfidenceAction().detail}</p>
    </section>
  );

  const emptyStateContent = () => (
    <div class="mx-auto max-w-3xl space-y-2">
      <div class="text-sm font-semibold text-base-content">Start monitoring infrastructure</div>
      <p>{getInfrastructureEmptyStateSummary()}</p>
      <p class="text-xs leading-5 text-muted">{getInfrastructureEmptyStateDetail()}</p>
    </div>
  );

  return (
    <div ref={layoutContainerRef} class="min-w-0">
      <SettingsPanel
        title="Infrastructure systems"
        description="Add, discover, and verify the platform APIs plus Pulse Agent telemetry that make up Pulse's infrastructure model."
        noPadding
      >
        {headerActions()}
        {setupSummaryBand()}

        <Show when={!useCardLayout()}>
          <Table class="w-full min-w-[820px] table-fixed text-sm">
            <TableHeader class="bg-surface-alt/60">
              <TableRow>
                <TableHead class="w-[22%] py-1.5 pl-3 pr-3 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[20%]">
                  System
                </TableHead>
                <TableHead class="w-[7.5rem] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  Source
                </TableHead>
                <TableHead class="w-[21%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[20%]">
                  Endpoint
                </TableHead>
                <TableHead class="w-[18%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[22%]">
                  Coverage
                </TableHead>
                <TableHead class="w-[16%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[15%]">
                  Status
                </TableHead>
                <Show when={actionColumnVisible()}>
                  <TableHead class="w-[7rem] px-3 py-1.5 text-right text-[11px] font-medium text-muted whitespace-nowrap">
                    Actions
                  </TableHead>
                </Show>
              </TableRow>
            </TableHeader>

            <TableBody class="bg-surface">
              <For each={sortedProducts()}>
                {(product) => {
                  const configuredRows = () => groupedConfiguredRows().get(product.type) ?? [];
                  const configuredGroups = () =>
                    configuredSourceGroupsForProduct(product, configuredRows());
                  const discoveredRows = () => groupedDiscoveredRows().get(product.type) ?? [];
                  const visibleConfiguredGroups = () =>
                    visibleSourceGroupsForProduct(
                      product,
                      configuredGroups(),
                      discoveredRows().length > 0,
                    );
                  const groupRowClass = () =>
                    getGroupedTableRowClass('border-b border-border-subtle');

                  return (
                    <>
                      <For each={visibleConfiguredGroups()}>
                        {(group) => (
                          <>
                            <Show
                              when={actionColumnVisible()}
                              fallback={
                                <TableRow class={groupRowClass()}>
                                  <TableCell colspan={5} class={getGroupedTableRowCellClass()}>
                                    <div class="flex min-w-0 items-center gap-2">
                                      <span>{group.label}</span>
                                    </div>
                                  </TableCell>
                                </TableRow>
                              }
                            >
                              <TableRow class={groupRowClass()}>
                                <TableCell colspan={6} class={getGroupedTableRowCellClass()}>
                                  <div class="flex items-center justify-between gap-2 whitespace-nowrap">
                                    <span>{group.label}</span>
                                    <Show
                                      when={
                                        !props.readOnly &&
                                        Boolean(props.onAddSource || props.onAddSourceStep)
                                      }
                                    >
                                      <button
                                        type="button"
                                        onClick={() => handleGroupAddSource(group)}
                                        class={`${addSectionButtonClass} whitespace-nowrap`}
                                        aria-label={group.actionLabel}
                                        title={group.actionLabel}
                                      >
                                        <Plus class="h-3.5 w-3.5" />
                                        {group.actionLabel}
                                      </button>
                                    </Show>
                                  </div>
                                </TableCell>
                              </TableRow>
                            </Show>

                            <Show when={group.rows.length > 0}>
                              <For each={group.rows}>
                                {(row) => (
                                  <>
                                    <TableRow
                                      class={`border-b border-border-subtle ${
                                        row.isCluster ? 'bg-surface-alt/40' : ''
                                      }`}
                                    >
                                      <TableCell class="py-1 pl-3 pr-3 align-top">
                                        <div
                                          class={`text-[13px] ${
                                            row.isCluster
                                              ? 'font-medium text-base-content'
                                              : 'text-base-content/80'
                                          } ${wrappedFieldClass}`}
                                          title={row.name}
                                        >
                                          {row.name}
                                        </div>
                                        <Show when={row.isCluster && row.subtitle}>
                                          <div class="mt-0.5 text-[11px] text-muted">
                                            {row.subtitle}
                                          </div>
                                        </Show>
                                        <Show when={!row.isCluster && row.identitySubtitle}>
                                          <div class="mt-0.5 text-[11px] text-muted">
                                            {row.identitySubtitle}
                                          </div>
                                        </Show>
                                      </TableCell>

                                      <TableCell class="px-3 py-1 align-top">
                                        {(() => {
                                          const presentation = infrastructureSourcePresentation(
                                            row.source,
                                          );
                                          const title =
                                            agentMethodTitleFor(row) ?? presentation.title;
                                          return (
                                            <span
                                              class={`${presentation.badgeClassName} whitespace-nowrap`}
                                              title={title}
                                            >
                                              {presentation.label}
                                            </span>
                                          );
                                        })()}
                                      </TableCell>

                                      <TableCell class="px-3 py-1 align-top">
                                        <Show
                                          when={row.host}
                                          fallback={<span class="text-xs text-muted">-</span>}
                                        >
                                          <div
                                            class="truncate whitespace-nowrap text-[12px] text-muted"
                                            title={row.host}
                                          >
                                            {row.host}
                                          </div>
                                        </Show>
                                      </TableCell>

                                      <TableCell class="px-3 py-1 align-top">
                                        <Show
                                          when={row.coverageLabels.length > 0}
                                          fallback={<span class="text-xs text-muted">-</span>}
                                        >
                                          <div
                                            class="whitespace-normal break-words text-[12px] leading-4 text-muted"
                                            title={row.coverageLabels.join(', ')}
                                          >
                                            {row.coverageLabels.join(', ')}
                                          </div>
                                        </Show>
                                      </TableCell>

                                      <TableCell class="px-3 py-1 align-top">
                                        <div class="flex flex-wrap items-center gap-x-1.5 gap-y-1">
                                          <span
                                            class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium whitespace-nowrap ${row.statusClassName}`}
                                          >
                                            {row.statusLabel}
                                          </span>
                                          <Show when={row.agentUpdateCount > 0}>
                                            <span class="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium whitespace-nowrap text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                                              {row.agentUpdateCount === 1
                                                ? 'Agent update'
                                                : `${row.agentUpdateCount} agent updates`}
                                            </span>
                                          </Show>
                                          <span class="whitespace-nowrap text-[12px] text-muted/90">
                                            {row.lastActivityText}
                                          </span>
                                        </div>
                                        <div class="mt-1 flex flex-wrap items-center gap-1">
                                          <For each={row.fleetHighlights}>
                                            {(signal) => (
                                              <span
                                                class={fleetSignalClassName(signal.tone)}
                                                title={signal.detail}
                                              >
                                                {signal.label}
                                              </span>
                                            )}
                                          </For>
                                        </div>
                                      </TableCell>

                                      <Show when={actionColumnVisible()}>
                                        <TableCell class="px-3 py-1 align-top text-right">
                                          <Show
                                            when={rowInteractive(row)}
                                            fallback={
                                              <span class="text-xs text-muted">Read only</span>
                                            }
                                          >
                                            <button
                                              type="button"
                                              onClick={() => props.onOpenConnection?.(row)}
                                              class={inlineButtonClass}
                                            >
                                              Manage
                                            </button>
                                          </Show>
                                        </TableCell>
                                      </Show>
                                    </TableRow>

                                    <Show when={row.lastErrorMessage}>
                                      <TableRow class="border-b border-border-subtle">
                                        <TableCell
                                          colspan={actionColumnVisible() ? 6 : 5}
                                          class="bg-surface px-3 pb-1.5 pt-0"
                                        >
                                          <div
                                            role="alert"
                                            class="rounded-md border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
                                          >
                                            {row.lastErrorMessage}
                                          </div>
                                        </TableCell>
                                      </TableRow>
                                    </Show>

                                    <Show when={row.members.length > 0}>
                                      <For each={row.members}>
                                        {(member, memberIndex) => {
                                          const memberPresentation =
                                            infrastructureSourcePresentation(member.source);
                                          const memberSourceTitle =
                                            memberMethodTitleFor(row, memberIndex()) ??
                                            memberPresentation.title;
                                          return (
                                            <TableRow class="border-b border-border-subtle bg-surface-alt/30">
                                              <TableCell class="py-1 pl-3 pr-3 align-top">
                                                <div class="flex min-w-0 items-start gap-2 pl-4">
                                                  <span class="mt-1.5 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-border" />
                                                  <div class="min-w-0">
                                                    <div
                                                      class={`text-[13px] text-base-content/85 ${wrappedFieldClass}`}
                                                      title={member.name}
                                                    >
                                                      {member.name}
                                                    </div>
                                                    <div class="mt-0.5 text-[11px] text-muted">
                                                      {member.subtitle}
                                                    </div>
                                                  </div>
                                                </div>
                                              </TableCell>

                                              <TableCell class="px-3 py-1 align-top">
                                                <span
                                                  class={`${memberPresentation.badgeClassName} whitespace-nowrap`}
                                                  title={memberSourceTitle}
                                                >
                                                  {memberPresentation.label}
                                                </span>
                                              </TableCell>

                                              <TableCell class="px-3 py-1 align-top">
                                                <Show
                                                  when={member.host}
                                                  fallback={
                                                    <span class="text-xs text-muted">-</span>
                                                  }
                                                >
                                                  <div
                                                    class="truncate whitespace-nowrap text-[12px] text-muted"
                                                    title={member.host}
                                                  >
                                                    {member.host}
                                                  </div>
                                                </Show>
                                              </TableCell>

                                              <TableCell class="px-3 py-1 align-top">
                                                <Show
                                                  when={member.coverageLabels.length > 0}
                                                  fallback={
                                                    <span class="text-xs text-muted">-</span>
                                                  }
                                                >
                                                  <div
                                                    class="whitespace-normal break-words text-[12px] leading-4 text-muted"
                                                    title={member.coverageLabels.join(', ')}
                                                  >
                                                    {member.coverageLabels.join(', ')}
                                                  </div>
                                                </Show>
                                              </TableCell>

                                              <TableCell class="px-3 py-1 align-top">
                                                <div class="flex flex-wrap items-center gap-x-1.5 gap-y-1">
                                                  <span
                                                    class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium whitespace-nowrap ${member.statusClassName}`}
                                                  >
                                                    {member.statusLabel}
                                                  </span>
                                                  <span class="whitespace-nowrap text-[12px] text-muted/90">
                                                    {member.lastActivityText}
                                                  </span>
                                                </div>
                                                <div class="mt-1 flex flex-wrap items-center gap-1">
                                                  <For each={member.fleetHighlights}>
                                                    {(signal) => (
                                                      <span
                                                        class={fleetSignalClassName(signal.tone)}
                                                        title={signal.detail}
                                                      >
                                                        {signal.label}
                                                      </span>
                                                    )}
                                                  </For>
                                                </div>
                                              </TableCell>

                                              <Show when={actionColumnVisible()}>
                                                <TableCell class="px-3 py-1 align-top text-right">
                                                  <span
                                                    class="text-xs text-muted"
                                                    aria-hidden="true"
                                                  >
                                                    —
                                                  </span>
                                                </TableCell>
                                              </Show>
                                            </TableRow>
                                          );
                                        }}
                                      </For>
                                    </Show>
                                  </>
                                )}
                              </For>
                            </Show>

                            <Show when={group.id === product.type && discoveredRows().length > 0}>
                              <For each={discoveredRows()}>
                                {(server) => {
                                  return (
                                    <TableRow class={discoveryRowClass}>
                                      <TableCell class="py-1 pl-3 pr-3 align-top">
                                        <div
                                          class={`text-[13px] text-base-content/85 ${wrappedFieldClass}`}
                                          title={`${discoveredServerName(server)}${server.version ? ` · ${server.version}` : ''}`}
                                        >
                                          {discoveredServerName(server)}
                                        </div>
                                      </TableCell>

                                      <TableCell class="px-3 py-1 align-top">
                                        <span
                                          class="inline-flex items-center rounded-full border border-dashed border-border bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-muted whitespace-nowrap"
                                          title="Discovery candidate — review to attach a source"
                                        >
                                          Candidate
                                        </span>
                                      </TableCell>

                                      <TableCell class="px-3 py-1 align-top">
                                        <div
                                          class="truncate whitespace-nowrap text-[12px] text-muted"
                                          title={discoveredServerEndpoint(server)}
                                        >
                                          {discoveredServerEndpoint(server)}
                                        </div>
                                      </TableCell>

                                      <TableCell class="px-3 py-1 align-top">
                                        <div
                                          class="whitespace-normal break-words text-[12px] leading-4 text-muted"
                                          title={discoveredCoverageText(server)}
                                        >
                                          {discoveredCoverageText(server)}
                                        </div>
                                      </TableCell>

                                      <TableCell class="px-3 py-1 align-top">
                                        <div class="flex flex-wrap items-center gap-x-1.5 gap-y-1">
                                          <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium whitespace-nowrap text-blue-800 dark:bg-blue-950/40 dark:text-blue-200">
                                            Discovered
                                          </span>
                                          <span class="whitespace-nowrap text-[12px] text-muted/90">
                                            {lastDiscoveryResultText() ?? 'Waiting for scan'}
                                          </span>
                                        </div>
                                      </TableCell>

                                      <Show when={actionColumnVisible()}>
                                        <TableCell class="px-3 py-1 align-top text-right">
                                          <Show
                                            when={props.onReviewDiscoveredSource}
                                            fallback={
                                              <span class="text-xs text-muted">Read only</span>
                                            }
                                          >
                                            <button
                                              type="button"
                                              onClick={() =>
                                                props.onReviewDiscoveredSource?.(server)
                                              }
                                              class={inlineButtonClass}
                                            >
                                              Review
                                            </button>
                                          </Show>
                                        </TableCell>
                                      </Show>
                                    </TableRow>
                                  );
                                }}
                              </For>
                            </Show>
                          </>
                        )}
                      </For>
                    </>
                  );
                }}
              </For>

              <Show when={!hasAnyConfigured() && !hasAnyDiscovered()}>
                <TableRow>
                  <TableCell
                    colspan={actionColumnVisible() ? 6 : 5}
                    class="px-4 py-8 text-center text-sm text-muted"
                  >
                    {emptyStateContent()}
                  </TableCell>
                </TableRow>
              </Show>
            </TableBody>
          </Table>
        </Show>

        <Show when={useCardLayout()}>
          <div class="space-y-3 p-3">
            <For each={sortedProducts()}>
              {(product) => {
                const configuredRows = () => groupedConfiguredRows().get(product.type) ?? [];
                const configuredGroups = () =>
                  configuredSourceGroupsForProduct(product, configuredRows());
                const discoveredRows = () => groupedDiscoveredRows().get(product.type) ?? [];
                const visibleConfiguredGroups = () =>
                  visibleSourceGroupsForProduct(
                    product,
                    configuredGroups(),
                    discoveredRows().length > 0,
                  );

                return (
                  <For each={visibleConfiguredGroups()}>
                    {(group) => (
                      <section class="space-y-2">
                        <header class="flex items-center justify-between gap-2">
                          <h3 class="text-[14px] font-semibold text-base-content">{group.label}</h3>
                          <Show
                            when={
                              !props.readOnly && Boolean(props.onAddSource || props.onAddSourceStep)
                            }
                          >
                            <button
                              type="button"
                              onClick={() => handleGroupAddSource(group)}
                              class={`${addSectionButtonClass} whitespace-nowrap`}
                              aria-label={group.actionLabel}
                              title={group.actionLabel}
                            >
                              <Plus class="h-3.5 w-3.5" />
                              {group.actionLabel}
                            </button>
                          </Show>
                        </header>

                        <For each={group.rows}>
                          {(row) => {
                            const presentation = infrastructureSourcePresentation(row.source);
                            const sourceTitle = agentMethodTitleFor(row) ?? presentation.title;
                            return (
                              <article class="rounded-md border border-border-subtle bg-surface p-3 shadow-sm">
                                <header class="flex items-start justify-between gap-2">
                                  <div class="min-w-0 flex-1">
                                    <div
                                      class="break-words text-[13px] font-medium text-base-content"
                                      title={row.name}
                                    >
                                      {row.name}
                                    </div>
                                    <Show when={row.isCluster && row.subtitle}>
                                      <div class="mt-0.5 text-[11px] text-muted">
                                        {row.subtitle}
                                      </div>
                                    </Show>
                                    <Show when={!row.isCluster && row.identitySubtitle}>
                                      <div class="mt-0.5 text-[11px] text-muted">
                                        {row.identitySubtitle}
                                      </div>
                                    </Show>
                                  </div>
                                  <span
                                    class={`${presentation.badgeClassName} flex-shrink-0`}
                                    title={sourceTitle}
                                  >
                                    {presentation.label}
                                  </span>
                                </header>

                                <Show when={row.host}>
                                  <div
                                    class="mt-1 truncate text-[12px] text-muted"
                                    title={row.host}
                                  >
                                    {row.host}
                                  </div>
                                </Show>

                                <Show when={row.coverageLabels.length > 0}>
                                  <div class="mt-1 text-[12px] leading-4 text-muted">
                                    {row.coverageLabels.join(', ')}
                                  </div>
                                </Show>

                                <Show when={row.members.length > 0}>
                                  <div class="mt-3 border-t border-border-subtle pt-2">
                                    <div class="text-[11px] font-medium uppercase tracking-[0.08em] text-muted">
                                      Cluster nodes
                                    </div>
                                    <div class="mt-2 space-y-2">
                                      <For each={row.members}>
                                        {(member, memberIndex) => {
                                          const memberPresentation =
                                            infrastructureSourcePresentation(member.source);
                                          const memberSourceTitle =
                                            memberMethodTitleFor(row, memberIndex()) ??
                                            memberPresentation.title;
                                          return (
                                            <div class="rounded-md border border-border-subtle bg-surface-alt/30 px-2.5 py-2">
                                              <div class="flex items-start justify-between gap-2">
                                                <div class="min-w-0 flex-1">
                                                  <div
                                                    class="break-words text-[13px] font-medium text-base-content"
                                                    title={member.name}
                                                  >
                                                    {member.name}
                                                  </div>
                                                  <div class="mt-0.5 text-[11px] text-muted">
                                                    {member.subtitle}
                                                  </div>
                                                </div>
                                                <span
                                                  class={`${memberPresentation.badgeClassName} flex-shrink-0`}
                                                  title={memberSourceTitle}
                                                >
                                                  {memberPresentation.label}
                                                </span>
                                              </div>

                                              <Show when={member.host}>
                                                <div
                                                  class="mt-1 truncate text-[12px] text-muted"
                                                  title={member.host}
                                                >
                                                  {member.host}
                                                </div>
                                              </Show>

                                              <Show when={member.coverageLabels.length > 0}>
                                                <div class="mt-1 text-[12px] leading-4 text-muted">
                                                  {member.coverageLabels.join(', ')}
                                                </div>
                                              </Show>

                                              <div class="mt-2 flex flex-wrap items-center gap-1.5">
                                                <span
                                                  class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium whitespace-nowrap ${member.statusClassName}`}
                                                >
                                                  {member.statusLabel}
                                                </span>
                                                <span class="text-[12px] text-muted/90">
                                                  {member.lastActivityText}
                                                </span>
                                                <For each={member.fleetHighlights}>
                                                  {(signal) => (
                                                    <span
                                                      class={fleetSignalClassName(signal.tone)}
                                                      title={signal.detail}
                                                    >
                                                      {signal.label}
                                                    </span>
                                                  )}
                                                </For>
                                              </div>
                                            </div>
                                          );
                                        }}
                                      </For>
                                    </div>
                                  </div>
                                </Show>

                                <footer class="mt-2 flex items-center justify-between gap-2 border-t border-border-subtle pt-2">
                                  <div class="flex flex-wrap items-center gap-1.5">
                                    <span
                                      class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium whitespace-nowrap ${row.statusClassName}`}
                                    >
                                      {row.statusLabel}
                                    </span>
                                    <Show when={row.agentUpdateCount > 0}>
                                      <span class="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                                        {row.agentUpdateCount === 1
                                          ? 'Agent update'
                                          : `${row.agentUpdateCount} agent updates`}
                                      </span>
                                    </Show>
                                    <span class="text-[12px] text-muted/90">
                                      {row.lastActivityText}
                                    </span>
                                    <For each={row.fleetHighlights}>
                                      {(signal) => (
                                        <span
                                          class={fleetSignalClassName(signal.tone)}
                                          title={signal.detail}
                                        >
                                          {signal.label}
                                        </span>
                                      )}
                                    </For>
                                  </div>
                                  <Show when={!props.readOnly && rowInteractive(row)}>
                                    <button
                                      type="button"
                                      onClick={() => props.onOpenConnection?.(row)}
                                      class={`${inlineButtonClass} flex-shrink-0`}
                                    >
                                      Manage
                                    </button>
                                  </Show>
                                </footer>

                                <Show when={row.lastErrorMessage}>
                                  <div
                                    role="alert"
                                    class="mt-2 rounded-md border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
                                  >
                                    {row.lastErrorMessage}
                                  </div>
                                </Show>
                              </article>
                            );
                          }}
                        </For>

                        <Show when={group.id === product.type && discoveredRows().length > 0}>
                          <For each={discoveredRows()}>
                            {(server) => (
                              <article class="rounded-md border border-blue-200 bg-blue-50/50 p-3 shadow-sm dark:border-blue-900 dark:bg-blue-950/20">
                                <header class="flex items-start justify-between gap-2">
                                  <div
                                    class="min-w-0 flex-1 break-words text-[13px] font-medium text-base-content"
                                    title={`${discoveredServerName(server)}${server.version ? ` · ${server.version}` : ''}`}
                                  >
                                    {discoveredServerName(server)}
                                  </div>
                                  <span
                                    class="inline-flex flex-shrink-0 items-center rounded-full border border-dashed border-border bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-muted whitespace-nowrap"
                                    title="Discovery candidate — review to attach a source"
                                  >
                                    Candidate
                                  </span>
                                </header>

                                <div
                                  class="mt-1 truncate text-[12px] text-muted"
                                  title={discoveredServerEndpoint(server)}
                                >
                                  {discoveredServerEndpoint(server)}
                                </div>

                                <div class="mt-1 text-[12px] leading-4 text-muted">
                                  {discoveredCoverageText(server)}
                                </div>

                                <footer class="mt-2 flex items-center justify-between gap-2 border-t border-border-subtle pt-2">
                                  <div class="flex flex-wrap items-center gap-1.5">
                                    <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium text-blue-800 dark:bg-blue-950/40 dark:text-blue-200">
                                      Discovered
                                    </span>
                                    <span class="text-[12px] text-muted/90">
                                      {lastDiscoveryResultText() ?? 'Waiting for scan'}
                                    </span>
                                  </div>
                                  <Show when={!props.readOnly && props.onReviewDiscoveredSource}>
                                    <button
                                      type="button"
                                      onClick={() => props.onReviewDiscoveredSource?.(server)}
                                      class={`${inlineButtonClass} flex-shrink-0`}
                                    >
                                      Review
                                    </button>
                                  </Show>
                                </footer>
                              </article>
                            )}
                          </For>
                        </Show>
                      </section>
                    )}
                  </For>
                );
              }}
            </For>

            <Show when={!hasAnyConfigured() && !hasAnyDiscovered()}>
              <div class="rounded-md border border-dashed border-border px-3 py-6 text-center text-sm text-muted">
                {emptyStateContent()}
              </div>
            </Show>
          </div>
        </Show>
      </SettingsPanel>
    </div>
  );
};

export default InfrastructureSourceManager;
