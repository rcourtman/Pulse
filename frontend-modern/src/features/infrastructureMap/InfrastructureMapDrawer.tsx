import { For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import ArrowLeftIcon from 'lucide-solid/icons/arrow-left';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import NetworkIcon from 'lucide-solid/icons/network';
import SearchIcon from 'lucide-solid/icons/search';
import XIcon from 'lucide-solid/icons/x';
import { Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { StatusDot } from '@/components/shared/StatusDot';
import type { StatusIndicatorVariant } from '@/utils/status';
import type { MapInspectTarget } from './infrastructureMapLayout';
import type { MapHealth } from './infrastructureMapModel';
import {
  collectDiagramGroups,
  collectNetworkTiles,
  compareDiagramTilesByAttention,
  isDiagramTileAttention,
  matchesDiagramTileQuery,
  type DiagramNetwork,
  type DiagramTile,
  type ServiceDiagram,
} from './infrastructureMapServices';

type DrawerHealthFilter = 'all' | 'attention' | 'ok' | 'off';

interface DrawerEntry {
  tile: DiagramTile;
  groupLabel?: string;
}

export interface InfrastructureMapDrawerProps {
  target: MapInspectTarget | null;
  diagram: ServiceDiagram | null;
  activeNetworkId: string | null;
  hrefForResourceId: (id: string) => string | null;
  onSelectTarget: (target: MapInspectTarget) => void;
  onFilterByNetwork: (networkId: string | null) => void;
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

const networkSubtitle = (network: DiagramNetwork): string => {
  const count =
    network.kind === 'bridge'
      ? `${network.memberCount.toLocaleString()} connected`
      : `${network.memberCount.toLocaleString()} ${network.memberCount === 1 ? 'address' : 'addresses'}`;
  return [network.detail, count].filter(Boolean).join(' · ');
};

const tileSubtitle = (entry: DrawerEntry): string =>
  [
    entry.groupLabel,
    entry.tile.typeLabel,
    entry.tile.caption,
    entry.tile.address && !entry.tile.caption.includes(entry.tile.address)
      ? entry.tile.address
      : '',
    entry.tile.detail ? `Published ${entry.tile.detail}` : '',
  ]
    .filter(Boolean)
    .join(' · ');

const tileMetrics = (tile: DiagramTile): string =>
  [
    tile.cpu !== undefined ? `CPU ${Math.round(tile.cpu)}%` : '',
    tile.memory !== undefined ? `Mem ${Math.round(tile.memory)}%` : '',
  ]
    .filter(Boolean)
    .join(' · ');

export function InfrastructureMapDrawer(props: InfrastructureMapDrawerProps) {
  const [query, setQuery] = createSignal('');
  const [healthFilter, setHealthFilter] = createSignal<DrawerHealthFilter>('all');

  createEffect(() => {
    const current = props.target;
    setQuery('');
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
    return collectDiagramGroups(diagram).find((group) => group.key === current.groupKey) ?? null;
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

  const filteredEntries = createMemo(() => {
    const filter = healthFilter();
    const search = query().trim();
    return allEntries().filter((entry) => {
      if (filter === 'attention' && !isDiagramTileAttention(entry.tile.health)) return false;
      if (filter === 'ok' && entry.tile.health !== 'ok') return false;
      if (filter === 'off' && entry.tile.health !== 'off') return false;
      if (!search) return true;
      return matchesDiagramTileQuery(entry.tile, search, entry.groupLabel ?? '');
    });
  });

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
              <Show when={props.target?.kind !== 'all-networks' && counts().attention > 0}>
                <MetadataBadge tone="warning" size="xs" shape="pill">
                  {counts().attention.toLocaleString()} need attention
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
                  : 'Search resources by name, type, host, IP...'
              }
              aria-label={
                props.target?.kind === 'all-networks'
                  ? 'Search networks'
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

          <Show when={props.target?.kind !== 'all-networks' && filterOptions().length > 1}>
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
                    <span class="opacity-75">{option.count.toLocaleString()}</span>
                  </button>
                )}
              </For>
            </div>
          </Show>
        </div>

        <div class="min-h-0 flex-1 overflow-y-auto px-2 py-2 sm:px-3">
          <Show
            when={props.target?.kind === 'all-networks'}
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
                            <div class="truncate text-sm font-medium text-base-content">
                              {network.label}
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
          </Show>
        </div>
      </div>
    </Dialog>
  );
}
