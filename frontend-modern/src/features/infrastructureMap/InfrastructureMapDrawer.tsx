import { For, Match, Show, Switch, createEffect, createMemo, createSignal } from 'solid-js';
import ArrowLeftIcon from 'lucide-solid/icons/arrow-left';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import GlobeIcon from 'lucide-solid/icons/globe';
import NetworkIcon from 'lucide-solid/icons/network';
import SearchIcon from 'lucide-solid/icons/search';
import XIcon from 'lucide-solid/icons/x';
import { Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { StatusDot } from '@/components/shared/StatusDot';
import { formatSpeed } from '@/utils/format';
import type { StatusIndicatorVariant } from '@/utils/status';
import type { MapInspectTarget } from './infrastructureMapLayout';
import { MAP_LINK_SATURATED_PERCENT, type MapHealth } from './infrastructureMapModel';
import {
  collectActiveTrafficTiles,
  collectExposedEntries,
  collectNetworkTiles,
  compareDiagramTilesByAttention,
  compareDiagramTilesByTraffic,
  findDiagramGroup,
  formatMapLatency,
  formatMapLinkSpeed,
  isDiagramTileAttention,
  matchesDiagramTileQuery,
  type DiagramExposedEntry,
  type DiagramNetwork,
  type DiagramTile,
  type ServiceDiagram,
} from './infrastructureMapServices';

type DrawerHealthFilter = 'all' | 'attention' | 'ok' | 'off';
type DrawerExposedFilter = 'all' | 'proxied' | 'ports';
type DrawerSortMode = 'attention' | 'traffic';

interface DrawerEntry {
  tile: DiagramTile;
  groupLabel?: string;
}

export interface InfrastructureMapDrawerProps {
  target: MapInspectTarget | null;
  diagram: ServiceDiagram | null;
  activeNetworkId: string | null;
  activeTrafficOnly?: boolean;
  hrefForResourceId: (id: string) => string | null;
  onSelectTarget: (target: MapInspectTarget) => void;
  onFilterByNetwork: (networkId: string | null) => void;
  onFilterByActiveTraffic?: (active: boolean) => void;
  onClose: () => void;
}

const healthVariant = (health: MapHealth): StatusIndicatorVariant => {
  switch (health) {
    case 'ok':
      return 'success';
    case 'attention':
      return 'warning';
    case 'critical':
      return 'danger';
    default:
      return 'muted';
  }
};

const healthBadge = (
  health: MapHealth,
): { label: string; tone: 'danger' | 'warning' | 'neutral' } | null => {
  switch (health) {
    case 'critical':
      return { label: 'Critical', tone: 'danger' };
    case 'attention':
      return { label: 'Needs attention', tone: 'warning' };
    case 'stale':
      return { label: 'Not reporting', tone: 'neutral' };
    case 'off':
      return { label: 'Off', tone: 'neutral' };
    default:
      return null;
  }
};

const formatDrawerNet = (tile: DiagramTile): string => {
  if (tile.netRxBytes === undefined && tile.netTxBytes === undefined) return '';
  const rx = tile.netRxBytes ?? 0;
  const tx = tile.netTxBytes ?? 0;
  const total = tile.netTotalBytes ?? rx + tx;
  if (total <= 0) return '';
  if (total < 1024) return 'Net <1 KB/s';
  return `Net ↓ ${formatSpeed(rx)} ↑ ${formatSpeed(tx)}`;
};

const networkSubtitle = (network: DiagramNetwork): string => {
  const count =
    network.kind === 'bridge'
      ? `${network.memberCount.toLocaleString()} connected`
      : `${network.memberCount.toLocaleString()} ${network.memberCount === 1 ? 'address' : 'addresses'}`;
  const throughput =
    network.netTotalBytes !== undefined && network.netTotalBytes >= 1024
      ? formatSpeed(network.netTotalBytes)
      : '';
  const linkSpeed = network.linkSpeedMbps ? formatMapLinkSpeed(network.linkSpeedMbps) : '';
  return [network.detail, count, throughput, linkSpeed].filter(Boolean).join(' · ');
};

const formatDockerNetworks = (networks?: string[]): string => {
  if (!networks?.length) return '';
  const shown = networks.slice(0, 2).join(', ');
  const extra = networks.length > 2 ? ` +${networks.length - 2}` : '';
  return `Docker net: ${shown}${extra}`;
};

const formatProbeAndLatency = (probeSummary?: string, latencyMs?: number): string => {
  const latency = latencyMs !== undefined ? formatMapLatency(latencyMs) : '';
  if (probeSummary && latency) return `${probeSummary} · ${latency}`;
  if (probeSummary) return probeSummary;
  return latency ? `Latency ${latency}` : '';
};

const certWarningTone = (warning: string): 'danger' | 'warning' =>
  warning === 'Cert expired' || warning === 'Cert invalid' ? 'danger' : 'warning';

const tileSubtitle = (entry: DrawerEntry): string =>
  [
    entry.groupLabel,
    entry.tile.typeLabel,
    entry.tile.caption,
    entry.tile.address && !entry.tile.caption.includes(entry.tile.address)
      ? entry.tile.address
      : '',
    entry.tile.detail ? `Published ${entry.tile.detail}` : '',
    formatDockerNetworks(entry.tile.dockerNetworks),
  ]
    .filter(Boolean)
    .join(' · ');

const formatDrawerLinkSpeed = (tile: DiagramTile): string => {
  if (!tile.linkSpeedMbps) return '';
  return tile.linkUtilizationPercent !== undefined
    ? `${formatMapLinkSpeed(tile.linkSpeedMbps)} link (${tile.linkUtilizationPercent}%)`
    : `${formatMapLinkSpeed(tile.linkSpeedMbps)} link`;
};

const tileMetrics = (tile: DiagramTile): string =>
  [
    tile.cpu !== undefined ? `CPU ${Math.round(tile.cpu)}%` : '',
    tile.memory !== undefined ? `Mem ${Math.round(tile.memory)}%` : '',
    formatDrawerNet(tile),
    formatProbeAndLatency(tile.probeSummary, tile.latencyMs),
    formatDrawerLinkSpeed(tile),
  ]
    .filter(Boolean)
    .join(' · ');

const exposedEntrySubtitle = (entry: DiagramExposedEntry): string =>
  [
    entry.groupLabel,
    entry.tile.hostName && entry.tile.hostName !== entry.groupLabel ? entry.tile.hostName : '',
    entry.tile.typeLabel,
  ]
    .filter(Boolean)
    .join(' · ');

export function InfrastructureMapDrawer(props: InfrastructureMapDrawerProps) {
  const [query, setQuery] = createSignal('');
  const [healthFilter, setHealthFilter] = createSignal<DrawerHealthFilter>('all');
  const [exposedFilter, setExposedFilter] = createSignal<DrawerExposedFilter>('all');
  const [sortMode, setSortMode] = createSignal<DrawerSortMode>('attention');

  createEffect(() => {
    const current = props.target;
    setQuery('');
    setExposedFilter('all');
    setSortMode(current?.kind === 'traffic' ? 'traffic' : 'attention');
    if (current?.kind === 'group' && current.initialFilter) {
      setHealthFilter(current.initialFilter);
    } else {
      setHealthFilter('all');
    }
  });

  const activeGroup = createMemo(() => {
    const current = props.target;
    const diagram = props.diagram;
    if (!current || current.kind !== 'group' || !diagram) return null;
    return findDiagramGroup(diagram, current.groupKey) ?? null;
  });

  const activeNetwork = createMemo(() => {
    const current = props.target;
    const diagram = props.diagram;
    if (!current || current.kind !== 'network' || !diagram) return null;
    return diagram.networks.find((network) => network.id === current.networkId) ?? null;
  });

  const allEntries = createMemo<DrawerEntry[]>(() => {
    const current = props.target;
    const diagram = props.diagram;
    if (!current || !diagram) return [];
    if (current.kind === 'group') {
      const group = activeGroup();
      if (!group) return [];
      return [...group.allTiles].sort(compareDiagramTilesByAttention).map((tile) => ({ tile }));
    }
    if (current.kind === 'network') {
      return collectNetworkTiles(diagram, current.networkId);
    }
    if (current.kind === 'traffic') {
      return collectActiveTrafficTiles(diagram);
    }
    return [];
  });

  const counts = createMemo(() => {
    const entries = allEntries();
    let attention = 0;
    let ok = 0;
    let off = 0;
    for (const { tile } of entries) {
      if (isDiagramTileAttention(tile.health)) attention += 1;
      else if (tile.health === 'ok') ok += 1;
      else if (tile.health === 'off') off += 1;
    }
    return { all: entries.length, attention, ok, off };
  });

  const hasTrafficEntries = createMemo(() =>
    allEntries().some(
      (entry) => entry.tile.netTotalBytes !== undefined && entry.tile.netTotalBytes > 0,
    ),
  );

  const filteredEntries = createMemo(() => {
    const filter = healthFilter();
    const search = query().trim();
    const filtered = allEntries().filter((entry) => {
      if (filter === 'attention' && !isDiagramTileAttention(entry.tile.health)) return false;
      if (filter === 'ok' && entry.tile.health !== 'ok') return false;
      if (filter === 'off' && entry.tile.health !== 'off') return false;
      if (!search) return true;
      return matchesDiagramTileQuery(entry.tile, search, entry.groupLabel ?? '');
    });
    if (sortMode() === 'traffic' && hasTrafficEntries()) {
      return [...filtered].sort((a, b) => compareDiagramTilesByTraffic(a.tile, b.tile));
    }
    if (props.target?.kind === 'traffic') {
      return [...filtered].sort((a, b) => compareDiagramTilesByAttention(a.tile, b.tile));
    }
    return filtered;
  });

  const exposedEntries = createMemo<DiagramExposedEntry[]>(() => {
    const current = props.target;
    const diagram = props.diagram;
    if (!current || current.kind !== 'exposed' || !diagram) return [];
    return collectExposedEntries(diagram);
  });

  const exposedCounts = createMemo(() => {
    const entries = exposedEntries();
    let proxied = 0;
    let ports = 0;
    let attention = 0;
    for (const entry of entries) {
      if (entry.hostnames.length > 0) proxied += 1;
      if (entry.ports) ports += 1;
      if (isDiagramTileAttention(entry.tile.health)) attention += 1;
    }
    return { all: entries.length, proxied, ports, attention };
  });

  const filteredExposedEntries = createMemo(() => {
    const filter = exposedFilter();
    const search = query().trim();
    return exposedEntries().filter((entry) => {
      if (filter === 'proxied' && entry.hostnames.length === 0) return false;
      if (filter === 'ports' && !entry.ports) return false;
      if (!search) return true;
      const extra = [
        entry.groupLabel ?? '',
        entry.tile.hostName ?? '',
        ...entry.hostnames,
        entry.ports ?? '',
        entry.proxyName ?? '',
      ]
        .filter(Boolean)
        .join(' ');
      return matchesDiagramTileQuery(entry.tile, search, extra);
    });
  });

  const exposedFilterOptions = createMemo<
    Array<{ value: DrawerExposedFilter; label: string; count: number }>
  >(() => {
    const c = exposedCounts();
    if (c.proxied === 0 || c.ports === 0) return [];
    return [
      { value: 'all', label: 'All', count: c.all },
      { value: 'proxied', label: 'Proxied', count: c.proxied },
      { value: 'ports', label: 'Published ports', count: c.ports },
    ];
  });

  const headerAttentionCount = createMemo(() =>
    props.target?.kind === 'exposed' ? exposedCounts().attention : counts().attention,
  );

  const filteredNetworks = createMemo(() => {
    const diagram = props.diagram;
    if (!diagram) return [];
    const tokens = query().trim().toLowerCase().split(/\s+/).filter(Boolean);
    if (tokens.length === 0) return diagram.networks;
    return diagram.networks.filter((network) => {
      const text = [network.label, network.detail, network.kind]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return tokens.every((token) => text.includes(token));
    });
  });

  const title = createMemo(() => {
    const current = props.target;
    if (!current) return '';
    if (current.kind === 'group') return activeGroup()?.label ?? 'Group';
    if (current.kind === 'network') return activeNetwork()?.label ?? 'Network';
    if (current.kind === 'traffic') return 'Live traffic';
    if (current.kind === 'exposed') return 'Published & routed';
    return 'Networks';
  });

  const subtitle = createMemo(() => {
    const current = props.target;
    if (!current) return '';
    if (current.kind === 'group') {
      const total = counts().all;
      return `${total.toLocaleString()} ${total === 1 ? 'resource' : 'resources'}`;
    }
    if (current.kind === 'network') {
      const network = activeNetwork();
      return network ? networkSubtitle(network) : '';
    }
    if (current.kind === 'traffic') {
      const total = allEntries().length;
      if (total === 0) return 'No active network traffic reported';
      const estateBytes = props.diagram?.estateTrafficBytes;
      const totalRate =
        estateBytes && estateBytes > 0 ? ` · ${formatSpeed(estateBytes)} total` : '';
      return `${total.toLocaleString()} active ${total === 1 ? 'resource' : 'resources'}${totalRate}`;
    }
    if (current.kind === 'exposed') {
      const c = exposedCounts();
      if (c.all === 0) return 'No proxied hostnames or published ports reported';
      const base = `${c.all.toLocaleString()} exposed ${c.all === 1 ? 'service' : 'services'}`;
      if (c.proxied > 0 && c.ports > 0) {
        return `${base} · ${c.proxied.toLocaleString()} proxied · ${c.ports.toLocaleString()} with published ports`;
      }
      return base;
    }
    const total = props.diagram?.networks.length ?? 0;
    return `${total.toLocaleString()} ${total === 1 ? 'network' : 'networks'} across the estate`;
  });

  const isFilteredToCurrentNetwork = createMemo(() => {
    const current = props.target;
    return current?.kind === 'network' && props.activeNetworkId === current.networkId;
  });

  const filterOptions = createMemo<
    Array<{ value: DrawerHealthFilter; label: string; count: number }>
  >(() => {
    const c = counts();
    const opts: Array<{ value: DrawerHealthFilter; label: string; count: number }> = [
      { value: 'all', label: 'All', count: c.all },
    ];
    if (c.attention > 0) {
      opts.push({ value: 'attention', label: 'Needs attention', count: c.attention });
    }
    if (c.ok > 0) {
      opts.push({ value: 'ok', label: 'Healthy', count: c.ok });
    }
    if (c.off > 0) {
      opts.push({ value: 'off', label: 'Off', count: c.off });
    }
    return opts;
  });

  return (
    <Dialog
      isOpen={props.target !== null}
      onClose={props.onClose}
      layout="drawer-right"
      ariaLabelledBy="infrastructure-map-drawer-title"
      panelClass="max-w-xl"
    >
      <div
        class="flex h-full min-h-0 flex-col bg-surface text-base-content"
        data-testid="infrastructure-map-drawer"
      >
        <div class="flex items-start justify-between gap-3 border-b border-border px-4 py-3.5 sm:px-5">
          <div class="min-w-0 flex-1">
            <Show
              when={props.target?.kind === 'network' && (props.diagram?.networks.length ?? 0) > 1}
            >
              <button
                type="button"
                class="mb-1 inline-flex items-center gap-1 text-xs font-medium text-muted hover:text-base-content"
                onClick={() => props.onSelectTarget({ kind: 'all-networks' })}
              >
                <ArrowLeftIcon class="h-3.5 w-3.5" aria-hidden="true" />
                All networks
              </button>
            </Show>
            <div class="flex flex-wrap items-center gap-2">
              <h2
                id="infrastructure-map-drawer-title"
                class="truncate text-base font-semibold text-base-content"
              >
                {title()}
              </h2>
              <Show when={props.target?.kind !== 'all-networks' && headerAttentionCount() > 0}>
                <MetadataBadge tone="warning" size="xs" shape="pill">
                  {headerAttentionCount().toLocaleString()} need attention
                </MetadataBadge>
              </Show>
              <Show
                when={props.target?.kind === 'network' && (activeNetwork()?.linkDownCount ?? 0) > 0}
              >
                <MetadataBadge tone="warning" size="xs" shape="pill">
                  {activeNetwork()!.linkDownCount === 1
                    ? 'Link down'
                    : `Link down (${activeNetwork()!.linkDownCount})`}
                </MetadataBadge>
              </Show>
            </div>
            <p class="mt-0.5 text-xs text-muted">{subtitle()}</p>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <Show when={props.target?.kind === 'network' && activeNetwork()}>
              {(network) => (
                <Button
                  variant={isFilteredToCurrentNetwork() ? 'primaryFlat' : 'secondary'}
                  size="sm"
                  onClick={() => {
                    props.onFilterByNetwork(isFilteredToCurrentNetwork() ? null : network().id);
                    props.onClose();
                  }}
                >
                  {isFilteredToCurrentNetwork() ? 'Show all' : 'Filter map'}
                </Button>
              )}
            </Show>
            <Show
              when={
                props.target?.kind === 'traffic' &&
                props.onFilterByActiveTraffic &&
                (allEntries().length > 0 || props.activeTrafficOnly)
              }
            >
              <Button
                variant={props.activeTrafficOnly ? 'primaryFlat' : 'secondary'}
                size="sm"
                onClick={() => {
                  props.onFilterByActiveTraffic?.(!props.activeTrafficOnly);
                  props.onClose();
                }}
              >
                {props.activeTrafficOnly ? 'Show all' : 'Filter map'}
              </Button>
            </Show>
            <button
              type="button"
              class="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-blue-500"
              aria-label="Close drawer"
              onClick={() => props.onClose()}
            >
              <XIcon class="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
        </div>

        <div class="flex flex-col gap-2.5 border-b border-border bg-surface-alt/50 px-4 py-3 sm:px-5">
          <div class="relative">
            <SearchIcon
              class="pointer-events-none absolute top-1/2 left-2.5 h-4 w-4 -translate-y-1/2 text-muted"
              aria-hidden="true"
            />
            <input
              type="text"
              inputMode="search"
              value={query()}
              onInput={(event) => setQuery(event.currentTarget.value)}
              placeholder={
                props.target?.kind === 'all-networks'
                  ? 'Search networks, VLANs, subnets...'
                  : props.target?.kind === 'exposed'
                    ? 'Search hostnames, ports, services...'
                    : 'Search resources by name, type, host, IP...'
              }
              aria-label={
                props.target?.kind === 'all-networks'
                  ? 'Search networks'
                  : props.target?.kind === 'exposed'
                    ? 'Search exposed services'
                    : 'Search resources in drawer'
              }
              data-testid="infrastructure-map-drawer-search"
              class="h-9 w-full rounded-md border border-border bg-surface pr-8 pl-8 text-sm text-base-content placeholder:text-muted focus:border-blue-500 focus:ring-2 focus:ring-blue-500/30 focus:outline-none"
            />
            <Show when={query().length > 0}>
              <button
                type="button"
                class="absolute top-1/2 right-2 inline-flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-full text-muted hover:bg-surface-hover hover:text-base-content"
                aria-label="Clear drawer search"
                onClick={() => setQuery('')}
              >
                <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            </Show>
          </div>

          <Show when={props.target?.kind === 'exposed' && exposedFilterOptions().length > 1}>
            <div
              class="flex flex-wrap items-center gap-1.5"
              role="group"
              aria-label="Exposure filter"
              data-testid="infrastructure-map-exposed-filter"
            >
              <For each={exposedFilterOptions()}>
                {(option) => (
                  <button
                    type="button"
                    aria-pressed={exposedFilter() === option.value}
                    class={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium transition-colors ${
                      exposedFilter() === option.value
                        ? 'border-blue-400 bg-blue-500/15 text-blue-700 dark:border-blue-500 dark:text-blue-300'
                        : 'border-border bg-surface text-muted hover:bg-surface-hover hover:text-base-content'
                    }`}
                    onClick={() => setExposedFilter(option.value)}
                  >
                    <span>{option.label}</span>
                    <span class="tabular-nums">{option.count.toLocaleString()}</span>
                  </button>
                )}
              </For>
            </div>
          </Show>

          <Show
            when={
              props.target?.kind !== 'all-networks' &&
              props.target?.kind !== 'exposed' &&
              (filterOptions().length > 1 || hasTrafficEntries())
            }
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <Show when={filterOptions().length > 1}>
                <div
                  class="flex flex-wrap items-center gap-1.5"
                  role="group"
                  aria-label="Status filter"
                >
                  <For each={filterOptions()}>
                    {(option) => (
                      <button
                        type="button"
                        aria-pressed={healthFilter() === option.value}
                        class={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium transition-colors ${
                          healthFilter() === option.value
                            ? 'border-blue-400 bg-blue-500/15 text-blue-700 dark:border-blue-500 dark:text-blue-300'
                            : 'border-border bg-surface text-muted hover:bg-surface-hover hover:text-base-content'
                        }`}
                        onClick={() => setHealthFilter(option.value)}
                      >
                        <span>{option.label}</span>
                        <span class="tabular-nums">{option.count.toLocaleString()}</span>
                      </button>
                    )}
                  </For>
                </div>
              </Show>
              <Show when={hasTrafficEntries()}>
                <div
                  class="ml-auto inline-flex items-center rounded-md border border-border bg-surface p-0.5 text-xs"
                  role="group"
                  aria-label="Sort resources"
                  data-testid="infrastructure-map-drawer-sort"
                >
                  <button
                    type="button"
                    aria-pressed={sortMode() === 'attention'}
                    class={`rounded px-2 py-0.5 font-medium transition-colors ${
                      sortMode() === 'attention'
                        ? 'bg-blue-500/15 text-blue-700 dark:text-blue-300'
                        : 'text-muted hover:text-base-content'
                    }`}
                    onClick={() => setSortMode('attention')}
                  >
                    Attention
                  </button>
                  <button
                    type="button"
                    aria-pressed={sortMode() === 'traffic'}
                    class={`rounded px-2 py-0.5 font-medium transition-colors ${
                      sortMode() === 'traffic'
                        ? 'bg-blue-500/15 text-blue-700 dark:text-blue-300'
                        : 'text-muted hover:text-base-content'
                    }`}
                    onClick={() => setSortMode('traffic')}
                  >
                    Traffic
                  </button>
                </div>
              </Show>
            </div>
          </Show>
        </div>

        <div class="min-h-0 flex-1 overflow-y-auto px-2 py-2 sm:px-3">
          <Switch
            fallback={
              <Show
                when={filteredEntries().length > 0}
                fallback={
                  <div class="flex flex-col items-center justify-center gap-2 px-4 py-12 text-center">
                    <p class="text-sm text-muted">No matching resources in this view.</p>
                    <Show when={query().length > 0 || healthFilter() !== 'all'}>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => {
                          setQuery('');
                          setHealthFilter('all');
                        }}
                      >
                        Reset drawer filters
                      </Button>
                    </Show>
                  </div>
                }
              >
                <ul class="divide-y divide-border/60" data-testid="infrastructure-map-drawer-list">
                  <For each={filteredEntries()}>
                    {(entry) => {
                      const href = () => props.hrefForResourceId(entry.tile.id);
                      const badge = () => healthBadge(entry.tile.health);
                      const metrics = () => tileMetrics(entry.tile);
                      const rowContent = () => (
                        <>
                          <span class="mt-1 shrink-0">
                            <Show
                              when={entry.tile.health !== 'off'}
                              fallback={
                                <span
                                  class="inline-block h-2 w-2 rounded-full border-[1.5px] border-slate-400"
                                  aria-hidden="true"
                                />
                              }
                            >
                              <StatusDot variant={healthVariant(entry.tile.health)} size="sm" />
                            </Show>
                          </span>
                          <div class="min-w-0 flex-1">
                            <div class="flex flex-wrap items-center gap-2">
                              <span class="truncate text-sm font-medium text-base-content">
                                {entry.tile.name}
                              </span>
                              <Show when={badge()}>
                                {(item) => (
                                  <MetadataBadge tone={item().tone} size="xs" shape="pill">
                                    {item().label}
                                  </MetadataBadge>
                                )}
                              </Show>
                              <Show when={(entry.tile.linkDownCount ?? 0) > 0}>
                                <MetadataBadge tone="warning" size="xs" shape="pill">
                                  {entry.tile.linkDownCount === 1
                                    ? 'Link down'
                                    : `Link down (${entry.tile.linkDownCount})`}
                                </MetadataBadge>
                              </Show>
                              <Show
                                when={
                                  entry.tile.linkUtilizationPercent !== undefined &&
                                  entry.tile.linkUtilizationPercent >= MAP_LINK_SATURATED_PERCENT
                                }
                              >
                                <MetadataBadge tone="warning" size="xs" shape="pill">
                                  Link busy ({entry.tile.linkUtilizationPercent}%)
                                </MetadataBadge>
                              </Show>
                              <Show when={entry.tile.firewallEnabled}>
                                <MetadataBadge
                                  tone="neutral"
                                  size="xs"
                                  shape="pill"
                                  title="Proxmox guest interface firewall flag is enabled"
                                >
                                  NIC firewall
                                </MetadataBadge>
                              </Show>
                              <Show when={entry.tile.certWarning}>
                                {(warning) => (
                                  <MetadataBadge
                                    tone={certWarningTone(warning())}
                                    size="xs"
                                    shape="pill"
                                  >
                                    {warning()}
                                  </MetadataBadge>
                                )}
                              </Show>
                            </div>
                            <p class="mt-0.5 truncate text-xs text-muted">{tileSubtitle(entry)}</p>
                            <Show when={metrics()}>
                              <p class="mt-0.5 text-[11px] text-muted">{metrics()}</p>
                            </Show>
                          </div>
                          <Show when={href()}>
                            <ChevronRightIcon
                              class="mt-1 h-4 w-4 shrink-0 text-muted"
                              aria-hidden="true"
                            />
                          </Show>
                        </>
                      );
                      return (
                        <li>
                          <Show
                            when={href()}
                            fallback={
                              <div class="flex items-start gap-2.5 rounded-md px-2.5 py-2.5">
                                {rowContent()}
                              </div>
                            }
                          >
                            {(url) => (
                              <a
                                href={url()}
                                class="flex items-start gap-2.5 rounded-md px-2.5 py-2.5 transition-colors hover:bg-surface-hover focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-blue-500"
                                onClick={() => props.onClose()}
                              >
                                {rowContent()}
                              </a>
                            )}
                          </Show>
                        </li>
                      );
                    }}
                  </For>
                </ul>
              </Show>
            }
          >
            <Match when={props.target?.kind === 'exposed'}>
              <Show
                when={filteredExposedEntries().length > 0}
                fallback={
                  <div class="flex flex-col items-center justify-center gap-2 px-4 py-12 text-center">
                    <p class="text-sm text-muted">No matching exposed services in this view.</p>
                    <Show when={query().length > 0 || exposedFilter() !== 'all'}>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => {
                          setQuery('');
                          setExposedFilter('all');
                        }}
                      >
                        Reset drawer filters
                      </Button>
                    </Show>
                  </div>
                }
              >
                <ul class="divide-y divide-border/60" data-testid="infrastructure-map-exposed-list">
                  <For each={filteredExposedEntries()}>
                    {(entry) => {
                      const href = () => props.hrefForResourceId(entry.tile.id);
                      const badge = () => healthBadge(entry.tile.health);
                      const meta = () => exposedEntrySubtitle(entry);
                      const rowContent = () => (
                        <>
                          <span class="mt-1 shrink-0">
                            <Show
                              when={entry.tile.health !== 'off'}
                              fallback={
                                <span
                                  class="inline-block h-2 w-2 rounded-full border-[1.5px] border-slate-400"
                                  aria-hidden="true"
                                />
                              }
                            >
                              <StatusDot variant={healthVariant(entry.tile.health)} size="sm" />
                            </Show>
                          </span>
                          <div class="min-w-0 flex-1">
                            <div class="flex flex-wrap items-center gap-2">
                              <span class="truncate text-sm font-medium text-base-content">
                                {entry.tile.name}
                              </span>
                              <Show when={badge()}>
                                {(item) => (
                                  <MetadataBadge tone={item().tone} size="xs" shape="pill">
                                    {item().label}
                                  </MetadataBadge>
                                )}
                              </Show>
                            </div>
                            <Show when={meta()}>
                              <p class="mt-0.5 truncate text-xs text-muted">{meta()}</p>
                            </Show>
                            <div class="mt-1.5 flex flex-wrap items-center gap-1.5">
                              <For each={entry.hostnames}>
                                {(hostname) => (
                                  <MetadataBadge
                                    tone="indigo"
                                    appearance="outline"
                                    size="xs"
                                    shape="pill"
                                    wrap
                                  >
                                    <GlobeIcon class="h-3 w-3 shrink-0" aria-hidden="true" />
                                    <span>{hostname}</span>
                                  </MetadataBadge>
                                )}
                              </For>
                              <Show when={entry.proxyName}>
                                {(proxy) => (
                                  <span class="text-[11px] text-muted">via {proxy()}</span>
                                )}
                              </Show>
                              <Show when={entry.ports}>
                                {(ports) => (
                                  <MetadataBadge
                                    tone="neutral"
                                    appearance="outline"
                                    size="xs"
                                    shape="rounded"
                                    class="font-mono"
                                  >
                                    {ports()}
                                  </MetadataBadge>
                                )}
                              </Show>
                            </div>
                          </div>
                          <Show when={href()}>
                            <ChevronRightIcon
                              class="mt-1 h-4 w-4 shrink-0 text-muted"
                              aria-hidden="true"
                            />
                          </Show>
                        </>
                      );
                      return (
                        <li>
                          <Show
                            when={href()}
                            fallback={
                              <div class="flex items-start gap-2.5 rounded-md px-2.5 py-2.5">
                                {rowContent()}
                              </div>
                            }
                          >
                            {(url) => (
                              <a
                                href={url()}
                                class="flex items-start gap-2.5 rounded-md px-2.5 py-2.5 transition-colors hover:bg-surface-hover focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-blue-500"
                                onClick={() => props.onClose()}
                              >
                                {rowContent()}
                              </a>
                            )}
                          </Show>
                        </li>
                      );
                    }}
                  </For>
                </ul>
              </Show>
            </Match>
            <Match when={props.target?.kind === 'all-networks'}>
              <Show
                when={filteredNetworks().length > 0}
                fallback={
                  <div class="flex flex-col items-center justify-center gap-2 px-4 py-12 text-center">
                    <p class="text-sm text-muted">No networks match your search.</p>
                    <Button variant="secondary" size="sm" onClick={() => setQuery('')}>
                      Clear search
                    </Button>
                  </div>
                }
              >
                <ul class="divide-y divide-border/60" data-testid="infrastructure-map-network-list">
                  <For each={filteredNetworks()}>
                    {(network) => {
                      const isActive = () => props.activeNetworkId === network.id;
                      return (
                        <li class="flex items-center gap-2 px-2 py-2">
                          <button
                            type="button"
                            class="flex min-w-0 flex-1 items-center gap-2.5 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-surface-hover"
                            onClick={() =>
                              props.onSelectTarget({ kind: 'network', networkId: network.id })
                            }
                          >
                            <NetworkIcon class="h-4 w-4 shrink-0 text-muted" aria-hidden="true" />
                            <div class="min-w-0 flex-1">
                              <div class="flex flex-wrap items-center gap-1.5">
                                <span class="truncate text-sm font-medium text-base-content">
                                  {network.label}
                                </span>
                                <Show when={(network.linkDownCount ?? 0) > 0}>
                                  <MetadataBadge tone="warning" size="xs" shape="pill">
                                    {network.linkDownCount === 1
                                      ? 'Link down'
                                      : `Link down (${network.linkDownCount})`}
                                  </MetadataBadge>
                                </Show>
                              </div>
                              <div class="truncate text-xs text-muted">
                                {networkSubtitle(network)}
                              </div>
                            </div>
                            <ChevronRightIcon
                              class="h-4 w-4 shrink-0 text-muted"
                              aria-hidden="true"
                            />
                          </button>
                          <Button
                            variant={isActive() ? 'primaryFlat' : 'secondary'}
                            size="xs"
                            onClick={() => {
                              props.onFilterByNetwork(isActive() ? null : network.id);
                              props.onClose();
                            }}
                          >
                            {isActive() ? 'Filtered' : 'Filter'}
                          </Button>
                        </li>
                      );
                    }}
                  </For>
                </ul>
              </Show>
            </Match>
          </Switch>
        </div>
      </div>
    </Dialog>
  );
}
