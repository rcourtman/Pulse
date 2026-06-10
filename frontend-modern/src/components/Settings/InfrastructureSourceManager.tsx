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
import { AlertTriangle, Cpu, Plus, RotateCw, Search, SlidersHorizontal } from 'lucide-solid';
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
  infrastructureSourcePresentation,
  surfaceLabel,
  type InfrastructureSourcePresentation,
  type InfrastructureSystemMemberRow,
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
// Per-section 'Add' shortcuts are demoted to ghost text buttons so the
// primary '+ Add infrastructure' CTA at the top of the page stays visually
// dominant. They retain the 1-click shortcut for users adding another node
// to an existing platform without competing with the primary CTA.
const addSectionButtonClass =
  'inline-flex items-center justify-center gap-1 rounded px-2 py-0.5 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-50 dark:text-blue-300 dark:hover:bg-blue-950/30';
const workspacePrimaryButtonClass =
  'inline-flex min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60';
const workspaceSecondaryButtonClass =
  'inline-flex min-h-9 items-center justify-center gap-2 rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const discoveryPrimaryButtonClass =
  'inline-flex min-h-9 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60';
const discoverySecondaryButtonClass =
  'inline-flex min-h-9 items-center justify-center gap-2 rounded-md border border-border bg-surface px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const discoveryRowClass =
  'border-b border-border-subtle bg-blue-50/30 hover:bg-blue-50/40 dark:bg-blue-950/10 dark:hover:bg-blue-950/20';
const discoveryScanTargetLabel =
  'Proxmox VE, Proxmox Backup Server, and Proxmox Mail Gateway APIs';
// The system column shows just the name on a single line. The OS / version
// (standalone) or cluster identity (cluster) descriptor that used to sit on a
// second line moves into the name's hover title, so every row stays one line
// like the platform overview tables. Discovery rows already fold version into
// the name title the same way.
const rowSystemTitle = (row: InfrastructureSystemRow): string => {
  const detail = row.isCluster ? row.subtitle : row.identitySubtitle;
  return detail ? `${row.name} · ${detail}` : row.name;
};

const memberSystemTitle = (member: InfrastructureSystemMemberRow): string =>
  member.subtitle ? `${member.name} · ${member.subtitle}` : member.name;
// The connected-systems table sets `min-w-[820px]`; switch to the card
// layout whenever the measured container can't fit that, otherwise the
// table renders but overflows horizontally inside the settings panel.
const CARD_LAYOUT_MAX_WIDTH_PX = 819;

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

const discoverySubnetLabel = (status: DiscoveryScanStatus): string => {
  const subnet = status.subnet?.trim();
  if (!subnet || subnet.toLowerCase() === 'auto') {
    return 'auto-detected networks';
  }
  return subnet;
};

const discoveryErrorSummary = (errors?: readonly string[]): string | undefined => {
  const normalized = (errors ?? []).map((error) => error.trim()).filter(Boolean);
  if (normalized.length === 0) return undefined;
  if (normalized.length === 1) return normalized[0];
  return `${normalized.length} scan issues reported`;
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

// Cluster member rows describe nodes reached through the cluster's single API
// connection. Repeating the connection-level "API" badge on every node reads
// as per-node API credentials (#1493), so member rows only badge sources that
// are actually node-local: an attached Pulse Agent. API-only members render a
// muted dash with an explanatory tooltip instead.
const MEMBER_SHARED_API_TITLE =
  "Monitored through the cluster's API connection. One connection covers every node in the cluster.";

const memberMethodPresentation = (
  member: InfrastructureSystemMemberRow,
): InfrastructureSourcePresentation | null => {
  if (member.source === 'api') return null;
  if (member.source === 'both') return infrastructureSourcePresentation('agent');
  return infrastructureSourcePresentation(member.source);
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
  const infrastructureRows = createMemo(() =>
    props.rows().filter((row) => row.ownerType !== 'availability'),
  );

  const groupedConfiguredRows = createMemo(() => {
    const next = new Map<InfrastructureOnboardingConnectionType, InfrastructureSystemRow[]>();
    for (const product of products()) {
      next.set(product.type, []);
    }

    for (const row of infrastructureRows()) {
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

  const handleInstallAgentShortcut = () => {
    if (props.onAddSourceStep) {
      props.onAddSourceStep('linux-host');
      return;
    }
    props.onAddSource?.('agent');
  };

  const hasAnyConfigured = createMemo(() => infrastructureRows().length > 0);
  const hasAnyDiscovered = createMemo(() => props.discoveredNodes().length > 0);

  const rowInteractive = (row: InfrastructureSystemRow): boolean =>
    !props.readOnly && Boolean(props.onOpenConnection) && (row.canEdit || row.isAgent);

  const actionColumnVisible = () => !props.readOnly;
  const lastDiscoveryResultText = createMemo(() =>
    formatRelativeTimestamp(props.discoveryScanStatus().lastResultAt),
  );
  const connectedSystemCount = createMemo(() => infrastructureRows().length);
  const discoveredCandidateCount = createMemo(() => props.discoveredNodes().length);
  // Scoped to Proxmox VE: PBS/PMG/TrueNAS/etc. are fully covered by their
  // own API and don't benefit from a paired agent in the way PVE hosts do
  // (where the agent adds temps, SMART, host identity). Counting them here
  // would create "Needs agent" pressure for systems that need nothing.
  const apiOnlySystems = createMemo(() =>
    infrastructureRows().filter(
      (row) => row.ownerType === 'pve' && rowHasApiCoverage(row) && !rowHasAgentCoverage(row),
    ),
  );
  const apiOnlySystemCount = createMemo(() => apiOnlySystems().length);
  // Names list keeps the descriptive 'Install agents' hint actionable: when
  // there are 1 or 2 systems missing an agent, surface their names directly
  // so the user knows exactly which boxes the install applies to.
  const apiOnlySystemNamesText = createMemo(() => {
    const names = apiOnlySystems()
      .map((row) => row.name)
      .filter(Boolean);
    if (names.length === 0) return null;
    if (names.length === 1) return names[0];
    if (names.length === 2) return `${names[0]} and ${names[1]}`;
    return null;
  });
  // Counters read directly off the row state the user sees: a row is live
  // when its status is "Active" and needs attention when its derived
  // problem (or any member's) is non-empty. Going via signal predicates
  // duplicated the rollup logic and could disagree with what was rendered.
  const liveFleetSystemCount = createMemo(
    () => infrastructureRows().filter((row) => row.statusLabel === 'Active').length,
  );
  const fleetAttentionSystemCount = createMemo(
    () =>
      infrastructureRows().filter(
        (row) => Boolean(row.problem) || row.members.some((member) => member.problem),
      ).length,
  );
  const discoveryReadinessLabel = createMemo(() => {
    if (props.discoveryScanStatus().scanning) return 'Scanning now';
    if (discoveredCandidateCount() > 0) return `${discoveredCandidateCount()} to review`;
    const lastResult = lastDiscoveryResultText();
    if (lastResult) return `Last scan ${lastResult}`;
    return props.discoveryEnabled ? 'Ready to scan' : 'Discovery off';
  });
  const discoveryMonitorTitle = createMemo(() => {
    const status = props.discoveryScanStatus();
    if (status.scanning) return 'Scanning configured networks';
    if (discoveredCandidateCount() > 0) {
      return `${formatCount(discoveredCandidateCount(), 'candidate')} ready to review`;
    }
    if (!props.discoveryEnabled) return 'Discovery is off';
    if (discoveryErrorSummary(status.errors)) return 'Last discovery scan needs attention';
    if (lastDiscoveryResultText()) return 'No new candidates from the last scan';
    return 'Ready to scan configured networks';
  });
  const discoveryMonitorDetail = createMemo(() => {
    const status = props.discoveryScanStatus();
    const subnet = discoverySubnetLabel(status);
    const errors = discoveryErrorSummary(status.errors);
    if (status.scanning) {
      return `Pulse is scanning ${subnet} for ${discoveryScanTargetLabel}. Candidates appear below for review before anything is added.`;
    }
    if (discoveredCandidateCount() > 0) {
      return `${formatCount(discoveredCandidateCount(), 'candidate')} found on ${subnet}. Review and add credentials before Pulse starts monitoring it.`;
    }
    if (!props.discoveryEnabled) {
      return `Enable discovery to scan configured networks for ${discoveryScanTargetLabel}.`;
    }
    if (errors) {
      return errors;
    }
    if (lastDiscoveryResultText()) {
      return `Last scan checked ${subnet} ${lastDiscoveryResultText()} and found no unattached platform APIs.`;
    }
    return `Run discovery to look for unattached ${discoveryScanTargetLabel} on the configured networks.`;
  });
  const discoveryMonitorMeta = createMemo(() => {
    const status = props.discoveryScanStatus();
    if (status.scanning && status.lastScanStartedAt) {
      return `Started ${formatRelativeTimestamp(status.lastScanStartedAt) ?? 'just now'}`;
    }
    if (lastDiscoveryResultText()) return `Last scan ${lastDiscoveryResultText()}`;
    return props.discoveryEnabled ? 'No scan has run yet' : 'Disabled';
  });
  const setupConfidenceAction = createMemo<SetupConfidenceAction>(() => {
    if (connectedSystemCount() === 0) {
      return {
        kind: 'add',
        label: 'Add infrastructure',
        detail: 'Add a platform, host, NAS, or cluster to start monitoring.',
        onClick: props.onAddInfrastructure,
      };
    }

    if (apiOnlySystemCount() > 0 && (props.onAddSourceStep || props.onAddSource)) {
      const namesText = apiOnlySystemNamesText();
      const target = namesText ?? formatCount(apiOnlySystemCount(), 'API-backed system');
      return {
        kind: 'agent',
        label: 'Install agents',
        detail: `Install Pulse Agent on ${target} when you want node-local telemetry such as temperatures, SMART data, and host identity.`,
        onClick: handleInstallAgentShortcut,
      };
    }

    if (props.onRunDiscovery && props.discoveryEnabled && !lastDiscoveryResultText()) {
      return {
        kind: 'scan',
        label: props.discoveryScanStatus().scanning ? 'Scanning networks' : 'Scan networks',
        detail: `Run discovery to check whether more ${discoveryScanTargetLabel} are waiting on the configured networks.`,
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

  const discoveryMonitorBand = () => (
    <section
      aria-label="Network discovery"
      aria-live="polite"
      class="border-b border-border bg-surface px-4 py-3"
    >
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <div class="flex items-center gap-2 text-sm font-semibold text-base-content">
              <Search class="h-4 w-4 text-blue-600 dark:text-blue-300" aria-hidden="true" />
              Network discovery
            </div>
            <span
              class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${
                props.discoveryScanStatus().scanning
                  ? 'bg-blue-100 text-blue-800 dark:bg-blue-950/40 dark:text-blue-200'
                  : discoveryErrorSummary(props.discoveryScanStatus().errors)
                    ? 'bg-amber-100 text-amber-800 dark:bg-amber-950/40 dark:text-amber-200'
                    : props.discoveryEnabled
                      ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200'
                      : 'bg-surface-alt text-muted'
              }`}
            >
              {discoveryMonitorTitle()}
            </span>
            <span class="text-xs text-muted">{discoveryMonitorMeta()}</span>
          </div>
          <p class="mt-1 max-w-4xl text-xs leading-5 text-muted">{discoveryMonitorDetail()}</p>
          <Show when={discoveryErrorSummary(props.discoveryScanStatus().errors)}>
            {(summary) => (
              <div class="mt-2 flex items-start gap-2 text-xs text-amber-800 dark:text-amber-200">
                <AlertTriangle class="mt-0.5 h-3.5 w-3.5 flex-shrink-0" aria-hidden="true" />
                <span>{summary()}</span>
              </div>
            )}
          </Show>
        </div>

        <Show when={!props.readOnly}>
          <div class="flex flex-wrap gap-2 lg:justify-end">
            <Show when={props.onRunDiscovery}>
              <button
                type="button"
                onClick={props.onRunDiscovery}
                disabled={props.discoveryScanStatus().scanning || !props.discoveryEnabled}
                class={discoveryPrimaryButtonClass}
                title={`Scan configured networks for reachable ${discoveryScanTargetLabel}. Discovered candidates appear here for review before they are added.`}
              >
                <RotateCw
                  class={`h-4 w-4 ${props.discoveryScanStatus().scanning ? 'animate-spin' : ''}`}
                  aria-hidden="true"
                />
                {props.discoveryScanStatus().scanning ? 'Scanning...' : 'Run discovery'}
              </button>
            </Show>
            <Show when={props.onOpenDiscoverySettings}>
              <button
                type="button"
                onClick={props.onOpenDiscoverySettings}
                class={discoverySecondaryButtonClass}
                title={`Configure which networks Pulse scans for ${discoveryScanTargetLabel}.`}
              >
                <SlidersHorizontal class="h-4 w-4" aria-hidden="true" />
                Discovery settings
              </button>
            </Show>
            <Show when={discoveredCandidateCount() > 0 && props.onReviewDiscoveredSource}>
              <button
                type="button"
                onClick={() => props.onReviewDiscoveredSource?.(props.discoveredNodes()[0])}
                class={discoverySecondaryButtonClass}
              >
                <Search class="h-4 w-4" aria-hidden="true" />
                {discoveredCandidateCount() === 1 ? 'Review candidate' : 'Review first candidate'}
              </button>
            </Show>
          </div>
        </Show>
      </div>
    </section>
  );

  const setupSummaryBand = () => (
    <section
      aria-label="Infrastructure setup summary"
      class="border-b border-border bg-surface px-4 py-3"
    >
      <h3 class="sr-only">Setup status</h3>
      <div class="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        {/* Compact summary line replaces the previous 5-cell stats grid.
            The same metrics are surfaced inline so a glance reads the
            shape of the infrastructure ('4 systems, 4 live, 1 needs agent')
            without giving up significant vertical real estate above the
            table that the page is actually about. */}
        <dl class="flex min-w-0 flex-1 flex-wrap items-baseline gap-x-4 gap-y-1 text-sm">
          <For each={setupSummaryMetrics()}>
            {(metric, index) => (
              <div class="flex items-baseline gap-1.5">
                <Show when={index() > 0}>
                  <span aria-hidden="true" class="text-muted">
                    ·
                  </span>
                </Show>
                <dt class="text-[11px] font-medium uppercase tracking-[0.06em] text-muted">
                  {metric.label}
                </dt>
                <dd class="font-semibold text-base-content">{metric.value}</dd>
              </div>
            )}
          </For>
        </dl>
        {/* Recommendation button hides for the apiOnly install-agents case
            because row-level install-agent chips now surface the same
            attention per-system. Other recommendations (discovery scan,
            coverage coherent, add infrastructure) still render here. */}
        <Show
          when={
            !props.readOnly &&
            Boolean(setupConfidenceAction().onClick) &&
            setupConfidenceAction().kind !== 'agent'
          }
        >
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

      <Show when={setupConfidenceAction().kind !== 'agent'}>
        <p class="mt-3 text-xs leading-5 text-muted">{setupConfidenceAction().detail}</p>
      </Show>
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
      <SettingsPanel title="Connected systems" noPadding>
        {headerActions()}
        {discoveryMonitorBand()}
        {setupSummaryBand()}

        <Show when={!useCardLayout()}>
          <Table class="w-full min-w-[820px] table-fixed text-sm">
            <TableHeader class="bg-surface-alt/60">
              <TableRow>
                <TableHead class="w-[20%] py-1.5 pl-3 pr-3 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  System
                </TableHead>
                <TableHead class="w-[10%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  Method
                </TableHead>
                <TableHead class="w-[16%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  Host
                </TableHead>
                <TableHead class="w-[16%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  Coverage
                </TableHead>
                <TableHead class="w-[22%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
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
                                        Add
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
                                      <TableCell class="py-1.5 pl-3 pr-3 align-middle">
                                        <div
                                          class={`truncate text-[13px] ${
                                            row.isCluster
                                              ? 'font-medium text-base-content'
                                              : 'text-base-content/80'
                                          }`}
                                          title={rowSystemTitle(row)}
                                        >
                                          {row.name}
                                        </div>
                                      </TableCell>

                                      <TableCell class="px-3 py-1.5 align-middle">
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

                                      <TableCell class="px-3 py-1.5 align-middle">
                                        <Show
                                          when={row.host}
                                          fallback={
                                            <span class="text-xs text-muted" aria-hidden="true">
                                              —
                                            </span>
                                          }
                                        >
                                          <div
                                            class="truncate text-[12px] text-muted"
                                            title={row.host}
                                          >
                                            {row.host}
                                          </div>
                                        </Show>
                                      </TableCell>

                                      <TableCell class="px-3 py-1.5 align-middle">
                                        <Show
                                          when={row.coverageLabels.length > 0}
                                          fallback={
                                            <span class="text-xs text-muted" aria-hidden="true">
                                              —
                                            </span>
                                          }
                                        >
                                          <div
                                            class="truncate text-[12px] text-muted"
                                            title={row.coverageLabels.join(', ')}
                                          >
                                            {row.coverageLabels.join(', ')}
                                          </div>
                                        </Show>
                                      </TableCell>

                                      <TableCell class="px-3 py-1.5 align-middle">
                                        <div class="flex items-center gap-1.5 overflow-hidden whitespace-nowrap">
                                          <span
                                            class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium whitespace-nowrap ${row.statusClassName}`}
                                          >
                                            {row.statusLabel}
                                          </span>
                                          <Show when={row.agentUpdateCount > 0}>
                                            <span class="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium whitespace-nowrap text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                                              {row.agentUpdateCount === 1
                                                ? 'Agent update'
                                                : `${row.agentUpdateCount} updates`}
                                            </span>
                                          </Show>
                                          <span
                                            class="truncate text-[12px] text-muted/90"
                                            title={
                                              row.isCluster
                                                ? 'Oldest activity across cluster API and member agents'
                                                : undefined
                                            }
                                          >
                                            {row.lastActivityText}
                                          </span>
                                        </div>
                                      </TableCell>

                                      <Show when={actionColumnVisible()}>
                                        <TableCell class="px-3 py-1.5 align-middle text-right">
                                          <Show
                                            when={rowInteractive(row)}
                                            fallback={
                                              <span class="text-xs text-muted">Read only</span>
                                            }
                                          >
                                            <div class="flex items-center justify-end gap-1">
                                              {/* Row-level install-agent shortcut, scoped to
                                                  Proxmox VE: the agent adds node-local
                                                  telemetry (temps, SMART, host identity)
                                                  that the PVE API doesn't expose. PBS and
                                                  other API-only sources are fully covered
                                                  without an agent, so we don't nudge for
                                                  one there. Rendered icon-only so the row
                                                  stays a single line; the full guided flow
                                                  lives behind Manage. */}
                                              <Show
                                                when={
                                                  row.ownerType === 'pve' &&
                                                  rowHasApiCoverage(row) &&
                                                  !rowHasAgentCoverage(row) &&
                                                  Boolean(
                                                    props.onAddSourceStep || props.onAddSource,
                                                  )
                                                }
                                              >
                                                <button
                                                  type="button"
                                                  onClick={handleInstallAgentShortcut}
                                                  class="inline-flex items-center justify-center rounded p-1 text-blue-700 transition-colors hover:bg-blue-50 dark:text-blue-300 dark:hover:bg-blue-950/30"
                                                  aria-label="Install agent"
                                                  title="Install Pulse Agent on this system to add node-local telemetry (temperatures, SMART, host identity)."
                                                >
                                                  <Plus class="h-3.5 w-3.5" />
                                                </button>
                                              </Show>
                                              <button
                                                type="button"
                                                onClick={() => props.onOpenConnection?.(row)}
                                                class={inlineButtonClass}
                                              >
                                                Manage
                                              </button>
                                            </div>
                                          </Show>
                                        </TableCell>
                                      </Show>
                                    </TableRow>

                                    <Show when={row.problem || row.lastErrorMessage}>
                                      <TableRow class="border-b border-border-subtle">
                                        <TableCell
                                          colspan={actionColumnVisible() ? 6 : 5}
                                          class="bg-surface px-3 pb-1.5 pt-0"
                                        >
                                          <Show when={row.problem}>
                                            {(problem) => (
                                              <div
                                                class={`text-[11px] italic ${
                                                  problem().tone === 'critical'
                                                    ? 'text-rose-700 dark:text-rose-300'
                                                    : 'text-amber-700 dark:text-amber-300'
                                                }`}
                                                title={problem().detail}
                                              >
                                                {problem().label}
                                              </div>
                                            )}
                                          </Show>
                                          <Show when={row.lastErrorMessage}>
                                            <div
                                              role="alert"
                                              class="mt-1 rounded-md border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
                                            >
                                              {row.lastErrorMessage}
                                            </div>
                                          </Show>
                                        </TableCell>
                                      </TableRow>
                                    </Show>

                                    <Show when={row.members.length > 0}>
                                      <For each={row.members}>
                                        {(member, memberIndex) => {
                                          const memberPresentation =
                                            memberMethodPresentation(member);
                                          const memberSourceTitle = memberPresentation
                                            ? (memberMethodTitleFor(row, memberIndex()) ??
                                              memberPresentation.title)
                                            : MEMBER_SHARED_API_TITLE;
                                          return (
                                            <>
                                              <TableRow class="border-b border-border-subtle bg-surface-alt/30">
                                                <TableCell class="py-1.5 pl-3 pr-3 align-middle">
                                                  <div class="flex min-w-0 items-center gap-2 pl-4">
                                                    <span class="h-1.5 w-1.5 flex-shrink-0 rounded-full bg-border" />
                                                    <div
                                                      class="truncate text-[13px] text-base-content/85"
                                                      title={memberSystemTitle(member)}
                                                    >
                                                      {member.name}
                                                    </div>
                                                  </div>
                                                </TableCell>

                                                <TableCell class="px-3 py-1.5 align-middle">
                                                  <Show
                                                    when={memberPresentation}
                                                    fallback={
                                                      <span
                                                        class="text-xs text-muted"
                                                        title={memberSourceTitle}
                                                      >
                                                        —
                                                      </span>
                                                    }
                                                  >
                                                    {(presentation) => (
                                                      <span
                                                        class={`${presentation().badgeClassName} whitespace-nowrap`}
                                                        title={memberSourceTitle}
                                                      >
                                                        {presentation().label}
                                                      </span>
                                                    )}
                                                  </Show>
                                                </TableCell>

                                                <TableCell class="px-3 py-1.5 align-middle">
                                                  <Show
                                                    when={member.host}
                                                    fallback={
                                                      <span
                                                        class="text-xs text-muted"
                                                        aria-hidden="true"
                                                      >
                                                        —
                                                      </span>
                                                    }
                                                  >
                                                    <div
                                                      class="truncate text-[12px] text-muted"
                                                      title={member.host}
                                                    >
                                                      {member.host}
                                                    </div>
                                                  </Show>
                                                </TableCell>

                                                <TableCell class="px-3 py-1.5 align-middle">
                                                  <Show
                                                    when={member.coverageLabels.length > 0}
                                                    fallback={
                                                      <span
                                                        class="text-xs text-muted"
                                                        aria-hidden="true"
                                                      >
                                                        —
                                                      </span>
                                                    }
                                                  >
                                                    <div
                                                      class="truncate text-[12px] text-muted"
                                                      title={member.coverageLabels.join(', ')}
                                                    >
                                                      {member.coverageLabels.join(', ')}
                                                    </div>
                                                  </Show>
                                                </TableCell>

                                                <TableCell class="px-3 py-1.5 align-middle">
                                                  <div class="flex items-center gap-1.5 overflow-hidden whitespace-nowrap">
                                                    <span
                                                      class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium whitespace-nowrap ${member.statusClassName}`}
                                                    >
                                                      {member.statusLabel}
                                                    </span>
                                                    <span class="truncate text-[12px] text-muted/90">
                                                      {member.lastActivityText}
                                                    </span>
                                                  </div>
                                                </TableCell>

                                                <Show when={actionColumnVisible()}>
                                                  <TableCell class="px-3 py-1.5 align-middle text-right">
                                                    <span
                                                      class="text-xs text-muted"
                                                      aria-hidden="true"
                                                    >
                                                      —
                                                    </span>
                                                  </TableCell>
                                                </Show>
                                              </TableRow>

                                              <Show when={member.problem}>
                                                {(problem) => (
                                                  <TableRow class="border-b border-border-subtle bg-surface-alt/30">
                                                    <TableCell
                                                      colspan={actionColumnVisible() ? 6 : 5}
                                                      class="pb-1.5 pl-9 pr-3 pt-0"
                                                    >
                                                      <div
                                                        class={`text-[11px] italic ${
                                                          problem().tone === 'critical'
                                                            ? 'text-rose-700 dark:text-rose-300'
                                                            : 'text-amber-700 dark:text-amber-300'
                                                        }`}
                                                        title={problem().detail}
                                                      >
                                                        {problem().label}
                                                      </div>
                                                    </TableCell>
                                                  </TableRow>
                                                )}
                                              </Show>
                                            </>
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
                                      <TableCell class="py-1.5 pl-3 pr-3 align-middle">
                                        <div
                                          class="truncate text-[13px] text-base-content/85"
                                          title={`${discoveredServerName(server)}${server.version ? ` · ${server.version}` : ''}`}
                                        >
                                          {discoveredServerName(server)}
                                        </div>
                                      </TableCell>

                                      <TableCell class="px-3 py-1.5 align-middle">
                                        <span
                                          class="inline-flex items-center rounded-full border border-dashed border-border bg-surface-alt px-2 py-0.5 text-[11px] font-medium text-muted whitespace-nowrap"
                                          title="Discovery candidate — review to attach a source"
                                        >
                                          Candidate
                                        </span>
                                      </TableCell>

                                      <TableCell class="px-3 py-1.5 align-middle">
                                        <div
                                          class="truncate whitespace-nowrap text-[12px] text-muted"
                                          title={discoveredServerEndpoint(server)}
                                        >
                                          {discoveredServerEndpoint(server)}
                                        </div>
                                      </TableCell>

                                      <TableCell class="px-3 py-1.5 align-middle">
                                        <div
                                          class="truncate whitespace-nowrap text-[12px] text-muted"
                                          title={discoveredCoverageText(server)}
                                        >
                                          {discoveredCoverageText(server)}
                                        </div>
                                      </TableCell>

                                      <TableCell class="px-3 py-1.5 align-middle">
                                        <div class="flex items-center gap-1.5 overflow-hidden whitespace-nowrap">
                                          <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium whitespace-nowrap text-blue-800 dark:bg-blue-950/40 dark:text-blue-200">
                                            Discovered
                                          </span>
                                          <span class="truncate text-[12px] text-muted/90">
                                            {lastDiscoveryResultText() ?? 'Waiting for scan'}
                                          </span>
                                        </div>
                                      </TableCell>

                                      <Show when={actionColumnVisible()}>
                                        <TableCell class="px-3 py-1.5 align-middle text-right">
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
                              Add
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
                                            memberMethodPresentation(member);
                                          const memberSourceTitle = memberPresentation
                                            ? (memberMethodTitleFor(row, memberIndex()) ??
                                              memberPresentation.title)
                                            : MEMBER_SHARED_API_TITLE;
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
                                                <Show when={memberPresentation}>
                                                  {(presentation) => (
                                                    <span
                                                      class={`${presentation().badgeClassName} flex-shrink-0`}
                                                      title={memberSourceTitle}
                                                    >
                                                      {presentation().label}
                                                    </span>
                                                  )}
                                                </Show>
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
                                              </div>
                                              <Show when={member.problem}>
                                                {(problem) => (
                                                  <div
                                                    class={`mt-1 text-[11px] italic ${
                                                      problem().tone === 'critical'
                                                        ? 'text-rose-700 dark:text-rose-300'
                                                        : 'text-amber-700 dark:text-amber-300'
                                                    }`}
                                                    title={problem().detail}
                                                  >
                                                    {problem().label}
                                                  </div>
                                                )}
                                              </Show>
                                            </div>
                                          );
                                        }}
                                      </For>
                                    </div>
                                  </div>
                                </Show>

                                <footer class="mt-2 border-t border-border-subtle pt-2">
                                  <div class="flex items-center justify-between gap-2">
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
                                      <span
                                        class="text-[12px] text-muted/90"
                                        title={
                                          row.isCluster
                                            ? 'Oldest activity across cluster API and member agents'
                                            : undefined
                                        }
                                      >
                                        {row.lastActivityText}
                                      </span>
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
                                  </div>
                                  <Show when={row.problem}>
                                    {(problem) => (
                                      <div
                                        class={`mt-1 text-[11px] italic ${
                                          problem().tone === 'critical'
                                            ? 'text-rose-700 dark:text-rose-300'
                                            : 'text-amber-700 dark:text-amber-300'
                                        }`}
                                        title={problem().detail}
                                      >
                                        {problem().label}
                                      </div>
                                    )}
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
