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
import { Plus, RotateCw, SlidersHorizontal } from 'lucide-solid';
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
  connectionAgentVersionPresentation,
  infrastructureSourcePresentation,
  surfaceLabel,
  type InfrastructureSystemRow,
} from './connectionsTableModel';
import type { DiscoveredServer, DiscoveryScanStatus } from './infrastructureSettingsModel';
import {
  getInfrastructureOnboardingProductPresentation,
  getInfrastructureSourceManagerProducts,
  type InfrastructureOnboardingConnectionType,
} from '@/utils/infrastructureOnboardingPresentation';

interface InfrastructureSourceManagerProps {
  rows: Accessor<readonly InfrastructureSystemRow[]>;
  discoveredNodes: Accessor<readonly DiscoveredServer[]>;
  discoveryEnabled: boolean;
  discoveryScanStatus: Accessor<DiscoveryScanStatus>;
  readOnly: boolean;
  onAddSource?: (type: InfrastructureOnboardingConnectionType) => void;
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
  'inline-flex items-center gap-1.5 rounded-md px-1 py-1 text-sm font-medium text-muted transition-colors hover:text-base-content disabled:cursor-not-allowed disabled:opacity-60';
const discoveryRowClass =
  'border-b border-border-subtle bg-blue-50/30 hover:bg-blue-50/40 dark:bg-blue-950/10 dark:hover:bg-blue-950/20';
const wrappedFieldClass = 'whitespace-normal break-words leading-4';

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

const memberMethodTitleFor = (row: InfrastructureSystemRow, memberIndex: number): string | undefined => {
  const member = row.members[memberIndex];
  if (!member?.agentConnection) return undefined;
  return (
    connectionAgentVersionPresentation(member.agentConnection)?.title ??
    'Pulse Agent attached to cluster member'
  );
};

export const InfrastructureSourceManager: Component<InfrastructureSourceManagerProps> = (props) => {
  const products = createMemo(() => getInfrastructureSourceManagerProducts());
  const productRank = createMemo(() => {
    const next = new Map<InfrastructureOnboardingConnectionType, number>();
    products().forEach((product, index) => {
      next.set(product.type, index);
    });
    return next;
  });

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
    [...products()]
      .sort((left, right) => {
        const configuredDifference =
          (groupedConfiguredRows().get(right.type)?.length ?? 0) -
          (groupedConfiguredRows().get(left.type)?.length ?? 0);
        if (configuredDifference !== 0) return configuredDifference;

        const discoveredDifference =
          (groupedDiscoveredRows().get(right.type)?.length ?? 0) -
          (groupedDiscoveredRows().get(left.type)?.length ?? 0);
        if (discoveredDifference !== 0) return discoveredDifference;

        return (productRank().get(left.type) ?? 0) - (productRank().get(right.type) ?? 0);
      }),
  );

  const hasAnyConfigured = createMemo(() => props.rows().length > 0);
  const hasAnyDiscovered = createMemo(() => props.discoveredNodes().length > 0);

  const rowInteractive = (row: InfrastructureSystemRow): boolean =>
    !props.readOnly && Boolean(props.onOpenConnection) && (row.canEdit || row.isAgent);

  const actionColumnVisible = () => !props.readOnly;
  const lastDiscoveryResultText = createMemo(() =>
    formatRelativeTimestamp(props.discoveryScanStatus().lastResultAt),
  );

  const [viewportWidth, setViewportWidth] = createSignal(
    typeof window !== 'undefined' ? window.innerWidth : 1024,
  );
  onMount(() => {
    const handler = () => setViewportWidth(window.innerWidth);
    window.addEventListener('resize', handler);
    onCleanup(() => window.removeEventListener('resize', handler));
  });
  const useCardLayout = createMemo(() => viewportWidth() < 768);

  const headerActions = () => (
    <Show when={!props.readOnly}>
      <div class="flex flex-wrap items-center gap-2 sm:justify-end">
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
            Settings
          </button>
        </Show>
      </div>
    </Show>
  );

  return (
    <SettingsPanel
      title="Infrastructure systems"
      description="Configured systems and discovered candidates grouped by platform or host type. Install Pulse Agent on each machine where you want full node-local telemetry."
      noPadding
      action={headerActions()}
    >
      <Show when={!useCardLayout()}>
      <Table class="w-full table-fixed text-sm">
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
              const discoveredRows = () => groupedDiscoveredRows().get(product.type) ?? [];
              const groupRowClass = () =>
                'border-b border-border-subtle bg-surface-alt hover:bg-surface-alt dark:bg-base dark:hover:bg-base';
              const groupLabelClass = () => 'text-[15px] font-semibold text-base-content';

              return (
                <>
                  <Show
                    when={actionColumnVisible()}
                    fallback={
                      <TableRow class={groupRowClass()}>
                        <TableCell colspan={5} class="px-3 py-1.5">
                          <div class="flex min-w-0 items-center gap-2">
                            <span class={groupLabelClass()}>{product.label}</span>
                          </div>
                        </TableCell>
                      </TableRow>
                    }
                  >
                    <TableRow class={groupRowClass()}>
                      <TableCell colspan={5} class="px-3 py-1.5">
                        <div class="flex items-center gap-2 whitespace-nowrap">
                          <span class={groupLabelClass()}>{product.label}</span>
                        </div>
                      </TableCell>
                      <TableCell class="px-3 py-1.5 text-right">
                        <Show when={!props.readOnly && props.onAddSource}>
                          <button
                            type="button"
                            onClick={() => props.onAddSource?.(product.type)}
                            class={`${addSectionButtonClass} whitespace-nowrap`}
                            aria-label={product.actionLabel}
                            title={product.actionLabel}
                          >
                            <Plus class="h-3.5 w-3.5" />
                            Add
                          </button>
                        </Show>
                      </TableCell>
                    </TableRow>
                  </Show>

                  <Show when={configuredRows().length > 0}>
                    <For each={configuredRows()}>
                      {(row) => {
                        return (
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
                                  <div class="mt-0.5 text-[11px] text-muted">{row.subtitle}</div>
                                </Show>
                                <Show when={!row.isCluster && row.identitySubtitle}>
                                  <div class="mt-0.5 text-[11px] text-muted">
                                    {row.identitySubtitle}
                                  </div>
                                </Show>
                              </TableCell>

                              <TableCell class="px-3 py-1 align-top">
                                {(() => {
                                  const presentation = infrastructureSourcePresentation(row.source);
                                  const title = agentMethodTitleFor(row) ?? presentation.title;
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
                              </TableCell>

                              <Show when={actionColumnVisible()}>
                                <TableCell class="px-3 py-1 align-top text-right">
                                  <Show
                                    when={rowInteractive(row)}
                                    fallback={<span class="text-xs text-muted">Read only</span>}
                                  >
                                    <button
                                      type="button"
                                      onClick={() => props.onOpenConnection?.(row)}
                                      class={inlineButtonClass}
                                    >
                                      Edit
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
                                  const memberPresentation = infrastructureSourcePresentation(
                                    member.source,
                                  );
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
                                          fallback={<span class="text-xs text-muted">-</span>}
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
                                          fallback={<span class="text-xs text-muted">-</span>}
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
                                      </TableCell>

                                      <Show when={actionColumnVisible()}>
                                        <TableCell class="px-3 py-1 align-top text-right">
                                          <span class="text-xs text-muted" aria-hidden="true">
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
                        );
                      }}
                    </For>
                  </Show>

                  <Show when={discoveredRows().length > 0}>
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
                                  fallback={<span class="text-xs text-muted">Read only</span>}
                                >
                                  <button
                                    type="button"
                                    onClick={() => props.onReviewDiscoveredSource?.(server)}
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
              );
            }}
          </For>

          <Show when={!hasAnyConfigured() && !hasAnyDiscovered()}>
            <TableRow>
              <TableCell
                colspan={actionColumnVisible() ? 6 : 5}
                class="px-3 py-6 text-center text-sm text-muted"
              >
                No infrastructure configured yet.
              </TableCell>
            </TableRow>
          </Show>

          <Show when={!props.readOnly && props.onAddInfrastructure}>
            <TableRow class="border-t border-border-subtle bg-surface-alt/40">
              <TableCell
                colspan={actionColumnVisible() ? 6 : 5}
                class="px-3 py-2 text-center"
              >
                <button
                  type="button"
                  onClick={props.onAddInfrastructure}
                  class={`${addSectionButtonClass} whitespace-nowrap`}
                >
                  <Plus class="h-3.5 w-3.5" />
                  Add infrastructure
                </button>
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
            const discoveredRows = () => groupedDiscoveredRows().get(product.type) ?? [];

            return (
              <section class="space-y-2">
                <header class="flex items-center justify-between gap-2">
                  <h3 class="text-[14px] font-semibold text-base-content">{product.label}</h3>
                  <Show when={!props.readOnly && props.onAddSource}>
                    <button
                      type="button"
                      onClick={() => props.onAddSource?.(product.type)}
                      class={`${addSectionButtonClass} whitespace-nowrap`}
                      aria-label={product.actionLabel}
                      title={product.actionLabel}
                    >
                      <Plus class="h-3.5 w-3.5" />
                      Add
                    </button>
                  </Show>
                </header>

                <For each={configuredRows()}>
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
                              <div class="mt-0.5 text-[11px] text-muted">{row.subtitle}</div>
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
                                  const memberPresentation = infrastructureSourcePresentation(
                                    member.source,
                                  );
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
                                        <div class="mt-1 truncate text-[12px] text-muted" title={member.host}>
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
                          </div>
                          <Show when={!props.readOnly && rowInteractive(row)}>
                            <button
                              type="button"
                              onClick={() => props.onOpenConnection?.(row)}
                              class={`${inlineButtonClass} flex-shrink-0`}
                            >
                              Edit
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
              </section>
            );
          }}
        </For>

        <Show when={!hasAnyConfigured() && !hasAnyDiscovered()}>
          <div class="rounded-md border border-dashed border-border px-3 py-6 text-center text-sm text-muted">
            No infrastructure configured yet.
          </div>
        </Show>

        <Show when={!props.readOnly && props.onAddInfrastructure}>
          <div class="border-t border-border-subtle pt-3 text-center">
            <button
              type="button"
              onClick={props.onAddInfrastructure}
              class={`${addSectionButtonClass} whitespace-nowrap`}
            >
              <Plus class="h-3.5 w-3.5" />
              Add infrastructure
            </button>
          </div>
        </Show>
      </div>
      </Show>
    </SettingsPanel>
  );
};

export default InfrastructureSourceManager;
