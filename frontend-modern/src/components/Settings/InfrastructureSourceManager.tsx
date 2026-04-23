import { For, Show, createMemo, type Accessor, type Component } from 'solid-js';
import { Plus, RotateCw, Server, SlidersHorizontal } from 'lucide-solid';
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
  onRunDiscovery?: () => void;
  onOpenDiscoverySettings?: () => void;
  onOpenConnection?: (row: InfrastructureSystemRow) => void;
  onReviewDiscoveredSource?: (server: DiscoveredServer) => void;
}

const inlineButtonClass =
  'inline-flex min-w-[4.5rem] items-center justify-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const addSectionButtonClass =
  'inline-flex min-w-[4.5rem] items-center justify-center gap-1.5 rounded-md border border-blue-200 bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-200 dark:hover:bg-blue-900/40';
const primaryToolbarButtonClass =
  'inline-flex items-center justify-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-medium text-blue-700 transition-colors hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-200 dark:hover:bg-blue-900/40';
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

const discoveredCoverageText = (server: DiscoveredServer): string =>
  getInfrastructureOnboardingProductPresentation(server.type).coverage;

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
    [...products()].sort((left, right) => {
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

  const rowInteractive = (row: InfrastructureSystemRow): boolean =>
    !props.readOnly && Boolean(props.onOpenConnection) && (row.canEdit || row.isAgent);

  const actionColumnVisible = () => !props.readOnly;
  const discoveredCount = createMemo(() => props.discoveredNodes().length);
  const discoveryErrors = createMemo(() => props.discoveryScanStatus().errors ?? []);
  const lastDiscoveryResultText = createMemo(() =>
    formatRelativeTimestamp(props.discoveryScanStatus().lastResultAt),
  );
  const discoverySummary = createMemo(() => {
    const parts: string[] = [];
    parts.push(props.discoveryEnabled ? 'Automatic discovery on' : 'Automatic discovery off');
    if (props.discoveryScanStatus().scanning) {
      parts.push('Scanning now');
    }
    if (discoveredCount() > 0) {
      parts.push(`${discoveredCount()} candidate${discoveredCount() === 1 ? '' : 's'}`);
    }
    if (lastDiscoveryResultText()) {
      parts.push(`Updated ${lastDiscoveryResultText()}`);
    }
    if (discoveryErrors().length > 0) {
      parts.push(`${discoveryErrors().length} issue${discoveryErrors().length === 1 ? '' : 's'}`);
    }
    return parts;
  });

  return (
    <SettingsPanel
      title="Infrastructure systems"
      description="Configured systems and discovered candidates grouped by platform or host type. Install Pulse Agent on each machine where you want full node-local telemetry."
      noPadding
      icon={<Server class="h-5 w-5" strokeWidth={2} />}
    >
      <div class="border-b border-border bg-surface-alt/40 px-3 py-2.5 sm:px-4">
        <div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
              <span class="font-medium text-base-content">Discovery</span>
              <span class="text-muted">{discoverySummary().join(' · ')}</span>
            </div>
          </div>

          <Show when={!props.readOnly}>
            <div class="flex flex-wrap items-center gap-3 lg:justify-end">
              <Show when={props.onRunDiscovery}>
                <button
                  type="button"
                  onClick={props.onRunDiscovery}
                  disabled={props.discoveryScanStatus().scanning}
                  class={primaryToolbarButtonClass}
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
        </div>
      </div>

      <Table class="w-full table-fixed text-sm">
        <TableHeader class="bg-surface-alt/60">
          <TableRow>
            <TableHead class="w-[30%] py-1.5 pl-3 pr-3 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[26%]">
              System
            </TableHead>
            <TableHead class="w-[22%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[22%]">
              Endpoint
            </TableHead>
            <TableHead class="w-[22%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[24%]">
              Coverage
            </TableHead>
            <TableHead class="w-[16%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap xl:w-[16%]">
              Status
            </TableHead>
            <Show when={actionColumnVisible()}>
              <TableHead class="w-[12%] px-3 py-1.5 text-right text-[11px] font-medium text-muted whitespace-nowrap xl:w-[12%]">
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
                        <TableCell colspan={4} class="px-3 py-1.5">
                          <div class="flex min-w-0 items-center gap-2">
                            <span class={groupLabelClass()}>{product.label}</span>
                          </div>
                        </TableCell>
                      </TableRow>
                    }
                  >
                    <TableRow class={groupRowClass()}>
                      <TableCell colspan={4} class="px-3 py-1.5">
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
                            <TableRow class="border-b border-border-subtle">
                              <TableCell class="py-1 pl-3 pr-3 align-top">
                                <div class="min-w-0 space-y-0.5">
                                  <div class="min-w-0 flex-1">
                                    <div
                                      class={`text-[13px] text-base-content/80 ${wrappedFieldClass}`}
                                      title={row.name}
                                    >
                                      {row.name}
                                    </div>
                                  </div>
                                  <Show when={row.subtitle}>
                                    <div
                                      class="text-[11px] leading-4 text-muted"
                                      title={agentMethodTitleFor(row) ?? row.subtitle}
                                    >
                                      {row.subtitle}
                                    </div>
                                  </Show>
                                </div>
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
                                <div class="flex items-center gap-1.5 whitespace-nowrap">
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
                                  colspan={actionColumnVisible() ? 5 : 4}
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
                              <div class="min-w-0 flex-1">
                                <div
                                  class={`text-[13px] text-base-content/85 ${wrappedFieldClass}`}
                                  title={`${discoveredServerName(server)}${server.version ? ` · ${server.version}` : ''}`}
                                >
                                  {discoveredServerName(server)}
                                </div>
                              </div>
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
                              <div class="flex items-center gap-1.5 whitespace-nowrap">
                                <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium text-blue-800 dark:bg-blue-950/40 dark:text-blue-200">
                                  Discovered
                                </span>
                                <span class="text-[12px] text-muted/90">
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
        </TableBody>
      </Table>
    </SettingsPanel>
  );
};

export default InfrastructureSourceManager;
