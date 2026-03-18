import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import { Portal } from 'solid-js/web';
import { useWebSocket } from '@/App';
import { useResources } from '@/hooks/useResources';
import { Card } from '@/components/shared/Card';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import { EmptyState } from '@/components/shared/EmptyState';
import { PageHeader } from '@/components/shared/PageHeader';
import { SearchInput } from '@/components/shared/SearchInput';
import CephServiceIcon from '@/components/Ceph/CephServiceIcon';
import type { CephCluster, CephPool, CephServiceStatus } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { useKioskMode } from '@/hooks/useKioskMode';
import {
  getCephHealthPresentation,
  getCephLoadingStatePresentation,
  getCephDisconnectedStatePresentation,
  getCephNoClustersStatePresentation,
  getCephPoolsSearchEmptyStatePresentation,
  getCephServiceStatusPresentation,
} from '@/features/storageBackups/storageDomain';

// Cluster health status badge with tooltip
const HealthBadge: Component<{ health: string; message?: string }> = (props) => {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

  const healthInfo = createMemo(() => getCephHealthPresentation(props.health));

  const handleMouseEnter = (e: MouseEvent) => {
    if (!props.message) return;
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
    setShowTooltip(true);
  };

  return (
    <>
      <span
        class={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide ${healthInfo().badgeClass}`}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={() => setShowTooltip(false)}
      >
        <span class={`w-1.5 h-1.5 rounded-full ${healthInfo().dotClass} animate-pulse`} />
        {healthInfo().label}
      </span>

      <Show when={showTooltip() && props.message}>
        <Portal mount={document.body}>
          <div
            class="fixed z-[9999] pointer-events-none"
            style={{
              left: `${tooltipPos().x}px`,
              top: `${tooltipPos().y - 8}px`,
              transform: 'translate(-50%, -100%)',
            }}
          >
            <div class="bg-surface text-base-content text-[10px] rounded-md shadow-sm px-2.5 py-1.5 max-w-[280px] border border-border">
              {props.message}
            </div>
          </div>
        </Portal>
      </Show>
    </>
  );
};

// Service status cell with tooltip
const ServiceStatusCell: Component<{ services: CephServiceStatus[] }> = (props) => {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

  const handleMouseEnter = (e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
    setShowTooltip(true);
  };

  return (
    <>
      <div
        class="flex items-center gap-1.5 cursor-help"
        onMouseEnter={handleMouseEnter}
        onMouseLeave={() => setShowTooltip(false)}
      >
        <For each={props.services.slice(0, 4)}>
          {(svc) => {
            const status = getCephServiceStatusPresentation(svc);
            return (
              <span class={`inline-flex items-center gap-0.5 text-xs ${status.textClass}`}>
                <CephServiceIcon type={svc.type} class="w-3.5 h-3.5" />
                <span class="font-mono text-[10px]">
                  {svc.running}/{svc.total}
                </span>
              </span>
            );
          }}
        </For>
        <Show when={props.services.length > 4}>
          <span class="text-[10px] text-slate-500">+{props.services.length - 4}</span>
        </Show>
      </div>

      <Show when={showTooltip() && props.services.length > 0}>
        <Portal mount={document.body}>
          <div
            class="fixed z-[9999] pointer-events-none"
            style={{
              left: `${tooltipPos().x}px`,
              top: `${tooltipPos().y - 8}px`,
              transform: 'translate(-50%, -100%)',
            }}
          >
            <div class="bg-surface text-base-content text-[10px] rounded-md shadow-sm px-2.5 py-2 min-w-[180px] border border-border">
              <div class="font-medium mb-1.5 text-base-content border-b border-border pb-1">
                Ceph Services
              </div>
              <div class="space-y-1">
                <For each={props.services}>
                  {(svc) => {
                    const status = getCephServiceStatusPresentation(svc);
                    return (
                      <div class="flex items-center justify-between gap-3">
                        <span class="flex items-center gap-1.5 text-muted">
                          <CephServiceIcon type={svc.type} class="w-3.5 h-3.5" />
                          <span class="uppercase">{svc.type}</span>
                        </span>
                        <span class={`font-mono ${status.textClass}`}>
                          {svc.running}/{svc.total}
                        </span>
                      </div>
                    );
                  }}
                </For>
              </div>
            </div>
          </div>
        </Portal>
      </Show>
    </>
  );
};

// Usage bar component
const UsageBar: Component<{ percent: number; size?: 'sm' | 'md' }> = (props) => {
  const barHeight = () => (props.size === 'sm' ? 'h-1.5' : 'h-2');
  const barColor = () => {
    const p = props.percent || 0;
    if (p > 90) return 'bg-red-500';
    if (p > 75) return 'bg-yellow-500';
    return 'bg-blue-500';
  };

  return (
    <div class={`w-full bg-surface-hover rounded-full ${barHeight()} overflow-hidden`}>
      <div
        class={`${barHeight()} rounded-full transition-all duration-500 ${barColor()}`}
        style={{ width: `${Math.min(props.percent || 0, 100)}%` }}
      />
    </div>
  );
};

// Main Ceph Page Component
const Ceph: Component = () => {
  const { connected, initialDataReceived, reconnecting, reconnect } = useWebSocket();
  const { byType } = useResources();

  const kioskMode = useKioskMode();

  const [searchTerm, setSearchTerm] = createSignal('');

  const cephResources = createMemo(() => byType('ceph'));

  const clusters = createMemo<CephCluster[]>(() => {
    return cephResources().map((r) => {
      const cephMeta = (r.platformData as any)?.ceph || {};
      return {
        id: r.id,
        instance: (r.platformData as any)?.proxmox?.instance || r.platformId || '',
        name: r.name,
        fsid: cephMeta.fsid,
        health: cephMeta.healthStatus || 'HEALTH_UNKNOWN',
        healthMessage: cephMeta.healthMessage || '',
        totalBytes: r.disk?.total || 0,
        usedBytes: r.disk?.used || 0,
        availableBytes: r.disk?.free || 0,
        usagePercent: r.disk?.current || 0,
        numMons: cephMeta.numMons || 0,
        numMgrs: cephMeta.numMgrs || 0,
        numOsds: cephMeta.numOsds || 0,
        numOsdsUp: cephMeta.numOsdsUp || 0,
        numOsdsIn: cephMeta.numOsdsIn || 0,
        numPGs: cephMeta.numPGs || 0,
        pools: cephMeta.pools?.map((p: any) => ({
          id: 0,
          name: p.name || '',
          storedBytes: p.storedBytes || 0,
          availableBytes: p.availableBytes || 0,
          objects: p.objects || 0,
          percentUsed: p.percentUsed || 0,
        })),
        services: cephMeta.services?.map((s: any) => ({
          type: s.type || '',
          running: s.running || 0,
          total: s.total || 0,
        })),
        lastUpdated: r.lastSeen || Date.now(),
      } as CephCluster;
    });
  });
  const hasClusters = createMemo(() => clusters().length > 0);

  // Aggregate all pools from all clusters
  const allPools = createMemo(() => {
    const pools: (CephPool & { clusterName: string })[] = [];
    for (const cluster of clusters()) {
      if (cluster.pools) {
        for (const pool of cluster.pools) {
          pools.push({
            ...pool,
            clusterName: cluster.name || 'Ceph Cluster',
          });
        }
      }
    }
    return pools;
  });

  // Aggregate services from all clusters
  const allServices = createMemo(() => {
    const serviceMap = new Map<string, { running: number; total: number }>();
    for (const cluster of clusters()) {
      if (cluster.services) {
        for (const svc of cluster.services) {
          const existing = serviceMap.get(svc.type) || { running: 0, total: 0 };
          serviceMap.set(svc.type, {
            running: existing.running + svc.running,
            total: existing.total + svc.total,
          });
        }
      }
    }
    return Array.from(serviceMap.entries()).map(([type, counts]) => ({
      type,
      running: counts.running,
      total: counts.total,
    }));
  });

  // Filter pools by search term
  const filteredPools = createMemo(() => {
    const term = searchTerm().toLowerCase().trim();
    if (!term) return allPools();
    return allPools().filter(
      (pool) =>
        pool.name.toLowerCase().includes(term) || pool.clusterName.toLowerCase().includes(term),
    );
  });

  // Calculate total storage stats
  const totalStats = createMemo(() => {
    let totalBytes = 0;
    let usedBytes = 0;
    for (const cluster of clusters()) {
      totalBytes += cluster.totalBytes || 0;
      usedBytes += cluster.usedBytes || 0;
    }
    const usagePercent = totalBytes > 0 ? (usedBytes / totalBytes) * 100 : 0;
    return { totalBytes, usedBytes, usagePercent };
  });

  const isLoading = createMemo(() => connected() && !initialDataReceived());
  const cephLoadingState = createMemo(() => getCephLoadingStatePresentation());
  const cephDisconnectedState = createMemo(() =>
    getCephDisconnectedStatePresentation(reconnecting()),
  );
  const cephNoClustersState = createMemo(() => getCephNoClustersStatePresentation());

  const thClass =
    'px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-surface-hover whitespace-nowrap transition-colors';

  return (
    <div class="space-y-4">
      <PageHeader
        id="ceph-title"
        title="Ceph"
        description="Cluster health, services, pools, and capacity across connected storage nodes."
      />
      {/* Navigation */}

      {/* Loading State */}
      <Show when={isLoading()}>
        <Card padding="lg">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 animate-spin text-blue-500"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
            }
            title={cephLoadingState().title}
            description={cephLoadingState().description}
          />
        </Card>
      </Show>

      {/* Disconnected State */}
      <Show when={!connected() && !isLoading()}>
        <Card padding="lg" tone="danger">
          <EmptyState
            icon={
              <svg
                class="h-12 w-12 text-red-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            }
            title={cephDisconnectedState().title}
            description={cephDisconnectedState().description}
            tone="danger"
            actions={
              !reconnecting() ? (
                <button
                  type="button"
                  onClick={() => reconnect()}
                  class="mt-2 inline-flex items-center px-4 py-2 text-xs font-medium rounded bg-red-600 text-white hover:bg-red-700 transition-colors"
                >
                  Reconnect now
                </button>
              ) : undefined
            }
          />
        </Card>
      </Show>

      <Show when={connected() && initialDataReceived()}>
        {/* No Clusters Empty State */}
        <Show when={!hasClusters()}>
          <Card padding="lg">
            <EmptyState
              icon={
                <svg
                  class="h-12 w-12 text-slate-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
                  />
                </svg>
              }
              title={cephNoClustersState().title}
              description={cephNoClustersState().description}
            />
          </Card>
        </Show>

        {/* Clusters Found - Show Content */}
        <Show when={hasClusters()}>
          {/* Summary Cards */}
          <div class="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-2 xl:grid-cols-4">
            {/* Total Storage Card */}
            <Card padding="sm" tone="card">
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-medium text-muted uppercase tracking-wide">
                  Total Storage
                </span>
                <svg
                  class="w-4 h-4 text-blue-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="1.5"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125"
                  />
                </svg>
              </div>
              <div class="text-xl sm:text-2xl font-bold text-base-content">
                {formatBytes(totalStats().totalBytes)}
              </div>
              <div class="mt-1.5">
                <UsageBar percent={totalStats().usagePercent} size="sm" />
                <div class="flex justify-between text-[10px] text-muted mt-1">
                  <span>{formatBytes(totalStats().usedBytes)} used</span>
                  <span>{totalStats().usagePercent.toFixed(1)}%</span>
                </div>
              </div>
            </Card>

            {/* Clusters Card */}
            <Card padding="sm" tone="card">
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-medium text-muted uppercase tracking-wide">Clusters</span>
                <svg
                  class="w-4 h-4 text-purple-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="1.5"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M21 7.5l-2.25-1.313M21 7.5v2.25m0-2.25l-2.25 1.313M3 7.5l2.25-1.313M3 7.5l2.25 1.313M3 7.5v2.25m9 3l2.25-1.313M12 12.75l-2.25-1.313M12 12.75V15m0 6.75l2.25-1.313M12 21.75V19.5m0 2.25l-2.25-1.313m0-16.875L12 2.25l2.25 1.313M21 14.25v2.25l-2.25 1.313m-13.5 0L3 16.5v-2.25"
                  />
                </svg>
              </div>
              <div class="text-xl sm:text-2xl font-bold text-base-content">{clusters().length}</div>
              <div class="flex flex-wrap gap-1 mt-2">
                <For each={clusters()}>
                  {(cluster) => (
                    <HealthBadge health={cluster.health || ''} message={cluster.healthMessage} />
                  )}
                </For>
              </div>
            </Card>

            {/* Services Card */}
            <Card padding="sm" tone="card">
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-medium text-muted uppercase tracking-wide">Services</span>
                <svg
                  class="w-4 h-4 text-green-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="1.5"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z"
                  />
                </svg>
              </div>
              <div class="text-xl sm:text-2xl font-bold text-base-content">
                {allServices().reduce((acc, svc) => acc + svc.running, 0)}
                <span class="text-sm font-normal text-muted">
                  /{allServices().reduce((acc, svc) => acc + svc.total, 0)}
                </span>
              </div>
              <div class="mt-2">
                <ServiceStatusCell services={allServices() as CephServiceStatus[]} />
              </div>
            </Card>

            {/* Pools Card */}
            <Card padding="sm" tone="card">
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-medium text-muted uppercase tracking-wide">Pools</span>
                <svg
                  class="w-4 h-4 text-cyan-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="1.5"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M6 6.878V6a2.25 2.25 0 012.25-2.25h7.5A2.25 2.25 0 0118 6v.878m-12 0c.235-.083.487-.128.75-.128h10.5c.263 0 .515.045.75.128m-12 0A2.25 2.25 0 004.5 9v.878m13.5-3A2.25 2.25 0 0119.5 9v.878m0 0a2.246 2.246 0 00-.75-.128H5.25c-.263 0-.515.045-.75.128m15 0A2.25 2.25 0 0121 12v6a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 18v-6c0-.98.626-1.813 1.5-2.122"
                  />
                </svg>
              </div>
              <div class="text-xl sm:text-2xl font-bold text-base-content">{allPools().length}</div>
              <div class="text-xs text-muted mt-2">
                {allPools()
                  .reduce((acc, pool) => acc + (pool.objects || 0), 0)
                  .toLocaleString()}{' '}
                objects
              </div>
            </Card>
          </div>

          {/* Cluster Details Table */}
          <Show when={clusters().length > 0}>
            <Card padding="none" tone="card" class="overflow-hidden">
              <div class="px-4 py-3 border-b border-border bg-surface-alt">
                <h3 class="text-sm font-semibold text-base-content">Cluster Overview</h3>
              </div>
              <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
                <style>{`.overflow-x-auto::-webkit-scrollbar { display: none; }`}</style>
                <Table
                  class="w-full border-collapse whitespace-nowrap"
                  style={{ 'min-width': '700px' }}
                >
                  <TableHeader>
                    <TableRow class="bg-surface-alt text-muted border-b border-border">
                      <TableHead class={`${thClass} pl-4`}>Cluster</TableHead>
                      <TableHead class={thClass}>Health</TableHead>
                      <TableHead class={thClass}>Monitors</TableHead>
                      <TableHead class={thClass}>Managers</TableHead>
                      <TableHead class={thClass}>OSDs</TableHead>
                      <TableHead class={thClass}>PGs</TableHead>
                      <TableHead class={`${thClass} min-w-[160px]`}>Capacity</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody class="divide-y divide-border-subtle">
                    <For each={clusters()}>
                      {(cluster) => (
                        <TableRow class="hover:bg-surface-hover transition-colors">
                          <TableCell class="px-4 py-2.5">
                            <div class="font-medium text-sm text-base-content">
                              {cluster.name || 'Ceph Cluster'}
                            </div>
                            <Show when={cluster.fsid}>
                              <div class="text-[10px] text-muted font-mono truncate max-w-[180px]">
                                {cluster.fsid}
                              </div>
                            </Show>
                          </TableCell>
                          <TableCell class="px-2 py-2.5">
                            <HealthBadge
                              health={cluster.health || ''}
                              message={cluster.healthMessage}
                            />
                          </TableCell>
                          <TableCell class="px-2 py-2.5">
                            <span class="inline-flex items-center gap-1 text-xs text-blue-600 dark:text-blue-400">
                              <CephServiceIcon type="mon" class="w-3.5 h-3.5" />
                              <span class="font-semibold">{cluster.numMons || 0}</span>
                            </span>
                          </TableCell>
                          <TableCell class="px-2 py-2.5">
                            <span class="inline-flex items-center gap-1 text-xs text-purple-600 dark:text-purple-400">
                              <CephServiceIcon type="mgr" class="w-3.5 h-3.5" />
                              <span class="font-semibold">{cluster.numMgrs || 0}</span>
                            </span>
                          </TableCell>
                          <TableCell class="px-2 py-2.5">
                            <span class="inline-flex items-center gap-1 text-xs">
                              <CephServiceIcon
                                type="osd"
                                class="w-3.5 h-3.5 text-green-600 dark:text-green-400"
                              />
                              <span
                                class={`font-semibold ${(cluster.numOsdsUp || 0) < (cluster.numOsds || 0) ? 'text-yellow-600 dark:text-yellow-400' : 'text-green-600 dark:text-green-400'}`}
                              >
                                {cluster.numOsdsUp || 0}
                              </span>
                              <span class="text-muted">/{cluster.numOsds || 0}</span>
                            </span>
                          </TableCell>
                          <TableCell class="px-2 py-2.5">
                            <span class="text-xs text-base-content font-medium">
                              {(cluster.numPGs || 0).toLocaleString()}
                            </span>
                          </TableCell>
                          <TableCell class="px-2 py-2.5">
                            <div class="w-full max-w-[160px]">
                              <UsageBar percent={cluster.usagePercent || 0} size="sm" />
                              <div class="flex justify-between text-[10px] text-muted mt-0.5">
                                <span>{formatBytes(cluster.usedBytes || 0)}</span>
                                <span>{(cluster.usagePercent || 0).toFixed(1)}%</span>
                              </div>
                            </div>
                          </TableCell>
                        </TableRow>
                      )}
                    </For>
                  </TableBody>
                </Table>
              </div>
            </Card>
          </Show>

          {/* Pools Table */}
          <Show when={allPools().length > 0}>
            <Card padding="none" tone="card" class="overflow-hidden">
              <div class="px-4 py-3 border-b border-border bg-surface-alt flex flex-col sm:flex-row sm:items-center justify-between gap-3 sm:gap-4">
                <h3 class="text-sm font-semibold text-base-content">
                  Storage Pools ({filteredPools().length})
                </h3>
                {/* Search Input */}
                <Show when={!kioskMode()}>
                  <SearchInput
                    value={searchTerm}
                    onChange={setSearchTerm}
                    placeholder="Search pools..."
                    title="Search storage pools"
                    class="w-full sm:max-w-xs flex-1 sm:flex-none"
                    clearOnEscape
                  />
                </Show>
              </div>

              <Show
                when={filteredPools().length > 0}
                fallback={
                  <div class="p-8 text-center text-muted">
                    {getCephPoolsSearchEmptyStatePresentation(searchTerm()).text}
                  </div>
                }
              >
                <div
                  class="overflow-x-auto"
                  style="scrollbar-width: none; -ms-overflow-style: none;"
                >
                  <style>{`.overflow-x-auto::-webkit-scrollbar { display: none; }`}</style>
                  <Table
                    class="w-full border-collapse whitespace-nowrap"
                    style={{ 'min-width': '650px' }}
                  >
                    <TableHeader>
                      <TableRow class="bg-surface-alt text-muted border-b border-border">
                        <TableHead class={`${thClass} pl-4`}>Pool</TableHead>
                        <TableHead class={thClass}>Cluster</TableHead>
                        <TableHead class={thClass}>Used</TableHead>
                        <TableHead class={thClass}>Available</TableHead>
                        <TableHead class={thClass}>Objects</TableHead>
                        <TableHead class={`${thClass} min-w-[120px]`}>Usage</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody class="divide-y divide-border-subtle">
                      <For each={filteredPools()}>
                        {(pool) => (
                          <TableRow class="hover:bg-surface-hover transition-colors">
                            <TableCell class="px-4 py-2.5 font-medium text-sm text-base-content">
                              {pool.name}
                            </TableCell>
                            <TableCell class="px-2 py-2.5 text-xs text-muted">
                              {pool.clusterName}
                            </TableCell>
                            <TableCell class="px-2 py-2.5 text-xs text-base-content font-mono">
                              {formatBytes(pool.storedBytes || 0)}
                            </TableCell>
                            <TableCell class="px-2 py-2.5 text-xs text-base-content font-mono">
                              {formatBytes(pool.availableBytes || 0)}
                            </TableCell>
                            <TableCell class="px-2 py-2.5 text-xs text-base-content font-mono">
                              {(pool.objects || 0).toLocaleString()}
                            </TableCell>
                            <TableCell class="px-2 py-2.5">
                              <div class="flex items-center gap-2">
                                <div class="w-16">
                                  <UsageBar percent={pool.percentUsed || 0} size="sm" />
                                </div>
                                <span class="text-xs text-muted font-mono w-12 text-right">
                                  {(pool.percentUsed || 0).toFixed(1)}%
                                </span>
                              </div>
                            </TableCell>
                          </TableRow>
                        )}
                      </For>
                    </TableBody>
                  </Table>
                </div>
              </Show>
            </Card>
          </Show>
        </Show>
      </Show>
    </div>
  );
};

export default Ceph;
