import { Component, For, Show, createMemo, createSignal, createEffect, onCleanup, onMount } from 'solid-js';
import { Portal } from 'solid-js/web';
import { useWebSocket } from '@/App';
import { useResources } from '@/hooks/useResources';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import type { CephCluster, CephPool, CephServiceStatus } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { isKioskMode, subscribeToKioskMode } from '@/utils/url';

// Service type icon component with proper styling
const ServiceIcon: Component<{ type: string; class?: string }> = (props) => {
    const iconClass = () => `${props.class || 'w-4 h-4'}`;

    switch (props.type.toLowerCase()) {
        case 'mon':
            return (
                <svg class={iconClass()} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" />
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
            );
        case 'mgr':
            return (
                <svg class={iconClass()} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z" />
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
            );
        case 'osd':
            return (
                <svg class={iconClass()} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125" />
                </svg>
            );
        case 'mds':
            return (
                <svg class={iconClass()} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z" />
                </svg>
            );
        case 'rgw':
            return (
                <svg class={iconClass()} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" />
                </svg>
            );
        default:
            return (
                <svg class={iconClass()} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z" />
                </svg>
            );
    }
};

// Get health color classes
const getHealthInfo = (health: string) => {
    const h = health?.toUpperCase() || '';
    if (h === 'OK' || h === 'HEALTH_OK') {
        return {
            bgClass: 'bg-green-100 dark:bg-green-900/30',
            textClass: 'text-green-700 dark:text-green-400',
            borderClass: 'border-green-200 dark:border-green-700',
            dotClass: 'bg-green-500',
            label: 'OK',
        };
    }
    if (h === 'WARN' || h === 'HEALTH_WARN' || h === 'WARNING') {
        return {
            bgClass: 'bg-yellow-100 dark:bg-yellow-900/30',
            textClass: 'text-yellow-700 dark:text-yellow-400',
            borderClass: 'border-yellow-200 dark:border-yellow-700',
            dotClass: 'bg-yellow-500',
            label: 'WARN',
        };
    }
    if (h === 'ERR' || h === 'HEALTH_ERR' || h === 'ERROR' || h === 'CRITICAL') {
        return {
            bgClass: 'bg-red-100 dark:bg-red-900/30',
            textClass: 'text-red-700 dark:text-red-400',
            borderClass: 'border-red-200 dark:border-red-700',
            dotClass: 'bg-red-500',
            label: 'ERROR',
        };
    }
    return {
        bgClass: 'bg-slate-100 dark:bg-slate-800',
        textClass: 'text-slate-600 dark:text-slate-400',
        borderClass: 'border-slate-200 dark:border-slate-700',
        dotClass: 'bg-slate-400',
        label: 'UNKNOWN',
    };
};

// Cluster health status badge with tooltip
const HealthBadge: Component<{ health: string; message?: string }> = (props) => {
    const [showTooltip, setShowTooltip] = createSignal(false);
    const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

    const healthInfo = createMemo(() => getHealthInfo(props.health));

    const handleMouseEnter = (e: MouseEvent) => {
        if (!props.message) return;
        const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
        setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
        setShowTooltip(true);
    };

    return (
        <>
            <span
                class={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide ${healthInfo().bgClass} ${healthInfo().textClass} border ${healthInfo().borderClass}`}
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
                        <div class="bg-slate-900 dark:bg-slate-800 text-white text-[10px] rounded-md shadow-sm px-2.5 py-1.5 max-w-[280px] border border-slate-700">
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

    const getServiceStatus = (svc: CephServiceStatus) => {
        if (svc.running === svc.total) return { color: 'text-green-600 dark:text-green-400', status: 'healthy' };
        if (svc.running > 0) return { color: 'text-yellow-600 dark:text-yellow-400', status: 'degraded' };
        return { color: 'text-red-600 dark:text-red-400', status: 'down' };
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
                        const status = getServiceStatus(svc);
                        return (
                            <span class={`inline-flex items-center gap-0.5 text-xs ${status.color}`}>
                                <ServiceIcon type={svc.type} class="w-3.5 h-3.5" />
                                <span class="font-mono text-[10px]">{svc.running}/{svc.total}</span>
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
                        <div class="bg-slate-900 dark:bg-slate-800 text-white text-[10px] rounded-md shadow-sm px-2.5 py-2 min-w-[180px] border border-slate-700">
                            <div class="font-medium mb-1.5 text-slate-300 border-b border-slate-700 pb-1">
                                Ceph Services
                            </div>
                            <div class="space-y-1">
                                <For each={props.services}>
                                    {(svc) => {
                                        const status = getServiceStatus(svc);
                                        return (
                                            <div class="flex items-center justify-between gap-3">
                                                <span class="flex items-center gap-1.5 text-slate-400">
                                                    <ServiceIcon type={svc.type} class="w-3.5 h-3.5" />
                                                    <span class="uppercase">{svc.type}</span>
                                                </span>
                                                <span class={`font-mono ${status.color}`}>
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
    const barHeight = () => props.size === 'sm' ? 'h-1.5' : 'h-2';
    const barColor = () => {
        const p = props.percent || 0;
        if (p > 90) return 'bg-red-500';
        if (p > 75) return 'bg-yellow-500';
        return 'bg-blue-500';
    };

    return (
        <div class={`w-full bg-slate-200 dark:bg-slate-700 rounded-full ${barHeight()} overflow-hidden`}>
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

    const [kioskMode, setKioskMode] = createSignal(isKioskMode());
    onMount(() => {
        const unsubscribe = subscribeToKioskMode((enabled) => {
            setKioskMode(enabled);
        });
        return unsubscribe;
    });

    const [searchTerm, setSearchTerm] = createSignal('');
    let searchInputRef: HTMLInputElement | undefined;

    // Keyboard handler for type-
    createEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            const target = e.target as HTMLElement;
            const isInputField =
                target.tagName === 'INPUT' ||
                target.tagName === 'TEXTAREA' ||
                target.tagName === 'SELECT' ||
                target.contentEditable === 'true';

            if (e.key === 'Escape') {
                if (searchTerm().trim()) {
                    setSearchTerm('');
                    searchInputRef?.blur();
                }
            } else if (!isInputField && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
                if (searchInputRef) {
                    searchInputRef.focus();
                }
            }
        };

        document.addEventListener('keydown', handleKeyDown);
        onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
    });

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
            pool =>
                pool.name.toLowerCase().includes(term) ||
                pool.clusterName.toLowerCase().includes(term)
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

    const thClass = "px-2 py-1.5 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-slate-100 dark:hover:bg-slate-600/50 whitespace-nowrap transition-colors";

    return (
        <div class="space-y-4">
            {/* Navigation */}

            {/* Loading State */}
            <Show when={isLoading()}>
                <Card padding="lg">
                    <EmptyState
                        icon={
                            <svg class="h-12 w-12 animate-spin text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                            </svg>
                        }
                        title="Loading Ceph data..."
                        description="Connecting to the monitoring service."
                    />
                </Card>
            </Show>

            {/* Disconnected State */}
            <Show when={!connected() && !isLoading()}>
                <Card padding="lg" tone="danger">
                    <EmptyState
                        icon={
                            <svg class="h-12 w-12 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                        }
                        title="Connection lost"
                        description={reconnecting() ? 'Attempting to reconnect…' : 'Unable to connect to the backend server'}
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
                                <svg class="h-12 w-12 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                                </svg>
                            }
                            title="No Ceph Clusters Detected"
                            description="Ceph cluster data will appear here when detected via the Pulse agent on your Proxmox nodes. Install the agent on a node with Ceph configured."
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
                                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">Total Storage</span>
                                <svg class="w-4 h-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125" />
                                </svg>
                            </div>
                            <div class="text-xl sm:text-2xl font-bold text-slate-900 dark:text-slate-100">
                                {formatBytes(totalStats().totalBytes)}
                            </div>
                            <div class="mt-1.5">
                                <UsageBar percent={totalStats().usagePercent} size="sm" />
                                <div class="flex justify-between text-[10px] text-slate-500 dark:text-slate-400 mt-1">
                                    <span>{formatBytes(totalStats().usedBytes)} used</span>
                                    <span>{totalStats().usagePercent.toFixed(1)}%</span>
                                </div>
                            </div>
                        </Card>

                        {/* Clusters Card */}
                        <Card padding="sm" tone="card">
                            <div class="flex items-center justify-between mb-2">
                                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">Clusters</span>
                                <svg class="w-4 h-4 text-purple-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M21 7.5l-2.25-1.313M21 7.5v2.25m0-2.25l-2.25 1.313M3 7.5l2.25-1.313M3 7.5l2.25 1.313M3 7.5v2.25m9 3l2.25-1.313M12 12.75l-2.25-1.313M12 12.75V15m0 6.75l2.25-1.313M12 21.75V19.5m0 2.25l-2.25-1.313m0-16.875L12 2.25l2.25 1.313M21 14.25v2.25l-2.25 1.313m-13.5 0L3 16.5v-2.25" />
                                </svg>
                            </div>
                            <div class="text-xl sm:text-2xl font-bold text-slate-900 dark:text-slate-100">
                                {clusters().length}
                            </div>
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
                                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">Services</span>
                                <svg class="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M5.25 14.25h13.5m-13.5 0a3 3 0 01-3-3m3 3a3 3 0 100 6h13.5a3 3 0 100-6m-16.5-3a3 3 0 013-3h13.5a3 3 0 013 3m-19.5 0a4.5 4.5 0 01.9-2.7L5.737 5.1a3.375 3.375 0 012.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 01.9 2.7m0 0a3 3 0 01-3 3m0 3h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008zm-3 6h.008v.008h-.008v-.008zm0-6h.008v.008h-.008v-.008z" />
                                </svg>
                            </div>
                            <div class="text-xl sm:text-2xl font-bold text-slate-900 dark:text-slate-100">
                                {allServices().reduce((acc, svc) => acc + svc.running, 0)}
                                <span class="text-sm font-normal text-slate-500 dark:text-slate-400">
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
                                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">Pools</span>
                                <svg class="w-4 h-4 text-cyan-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M6 6.878V6a2.25 2.25 0 012.25-2.25h7.5A2.25 2.25 0 0118 6v.878m-12 0c.235-.083.487-.128.75-.128h10.5c.263 0 .515.045.75.128m-12 0A2.25 2.25 0 004.5 9v.878m13.5-3A2.25 2.25 0 0119.5 9v.878m0 0a2.246 2.246 0 00-.75-.128H5.25c-.263 0-.515.045-.75.128m15 0A2.25 2.25 0 0121 12v6a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 18v-6c0-.98.626-1.813 1.5-2.122" />
                                </svg>
                            </div>
                            <div class="text-xl sm:text-2xl font-bold text-slate-900 dark:text-slate-100">
                                {allPools().length}
                            </div>
                            <div class="text-xs text-slate-500 dark:text-slate-400 mt-2">
                                {allPools().reduce((acc, pool) => acc + (pool.objects || 0), 0).toLocaleString()} objects
                            </div>
                        </Card>
                    </div>

                    {/* Cluster Details Table */}
                    <Show when={clusters().length > 0}>
                        <Card padding="none" tone="card" class="overflow-hidden">
                            <div class="px-4 py-3 border-b border-slate-200 dark:border-slate-700 bg-slate-50/50 dark:bg-slate-800">
                                <h3 class="text-sm font-semibold text-slate-900 dark:text-slate-100">
                                    Cluster Overview
                                </h3>
                            </div>
                            <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
                                <style>{`.overflow-x-auto::-webkit-scrollbar { display: none; }`}</style>
                                <table class="w-full border-collapse whitespace-nowrap" style={{ "min-width": "700px" }}>
                                    <thead>
                                        <tr class="bg-slate-50 dark:bg-slate-800 text-slate-600 dark:text-slate-300 border-b border-slate-200 dark:border-slate-700">
                                            <th class={`${thClass} pl-4`}>Cluster</th>
                                            <th class={thClass}>Health</th>
                                            <th class={thClass}>Monitors</th>
                                            <th class={thClass}>Managers</th>
                                            <th class={thClass}>OSDs</th>
                                            <th class={thClass}>PGs</th>
                                            <th class={`${thClass} min-w-[160px]`}>Capacity</th>
                                        </tr>
                                    </thead>
                                    <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
                                        <For each={clusters()}>
                                            {(cluster) => (
                                                <tr class="hover:bg-slate-50/80 dark:hover:bg-slate-700/30 transition-colors">
                                                    <td class="px-4 py-2.5">
                                                        <div class="font-medium text-sm text-slate-900 dark:text-slate-100">
                                                            {cluster.name || 'Ceph Cluster'}
                                                        </div>
                                                        <Show when={cluster.fsid}>
                                                            <div class="text-[10px] text-slate-500 dark:text-slate-400 font-mono truncate max-w-[180px]">
                                                                {cluster.fsid}
                                                            </div>
                                                        </Show>
                                                    </td>
                                                    <td class="px-2 py-2.5">
                                                        <HealthBadge health={cluster.health || ''} message={cluster.healthMessage} />
                                                    </td>
                                                    <td class="px-2 py-2.5">
                                                        <span class="inline-flex items-center gap-1 text-xs text-blue-600 dark:text-blue-400">
                                                            <ServiceIcon type="mon" class="w-3.5 h-3.5" />
                                                            <span class="font-semibold">{cluster.numMons || 0}</span>
                                                        </span>
                                                    </td>
                                                    <td class="px-2 py-2.5">
                                                        <span class="inline-flex items-center gap-1 text-xs text-purple-600 dark:text-purple-400">
                                                            <ServiceIcon type="mgr" class="w-3.5 h-3.5" />
                                                            <span class="font-semibold">{cluster.numMgrs || 0}</span>
                                                        </span>
                                                    </td>
                                                    <td class="px-2 py-2.5">
                                                        <span class="inline-flex items-center gap-1 text-xs">
                                                            <ServiceIcon type="osd" class="w-3.5 h-3.5 text-green-600 dark:text-green-400" />
                                                            <span class={`font-semibold ${(cluster.numOsdsUp || 0) < (cluster.numOsds || 0) ? 'text-yellow-600 dark:text-yellow-400' : 'text-green-600 dark:text-green-400'}`}>
                                                                {cluster.numOsdsUp || 0}
                                                            </span>
                                                            <span class="text-slate-400 dark:text-slate-500">
                                                                /{cluster.numOsds || 0}
                                                            </span>
                                                        </span>
                                                    </td>
                                                    <td class="px-2 py-2.5">
                                                        <span class="text-xs text-slate-700 dark:text-slate-300 font-medium">
                                                            {(cluster.numPGs || 0).toLocaleString()}
                                                        </span>
                                                    </td>
                                                    <td class="px-2 py-2.5">
                                                        <div class="w-full max-w-[160px]">
                                                            <UsageBar percent={cluster.usagePercent || 0} size="sm" />
                                                            <div class="flex justify-between text-[10px] text-slate-500 dark:text-slate-400 mt-0.5">
                                                                <span>{formatBytes(cluster.usedBytes || 0)}</span>
                                                                <span>{(cluster.usagePercent || 0).toFixed(1)}%</span>
                                                            </div>
                                                        </div>
                                                    </td>
                                                </tr>
                                            )}
                                        </For>
                                    </tbody>
                                </table>
                            </div>
                        </Card>
                    </Show>

                    {/* Pools Table */}
                    <Show when={allPools().length > 0}>
                        <Card padding="none" tone="card" class="overflow-hidden">
                            <div class="px-4 py-3 border-b border-slate-200 dark:border-slate-700 bg-slate-50/50 dark:bg-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3 sm:gap-4">
                                <h3 class="text-sm font-semibold text-slate-900 dark:text-slate-100">
                                    Storage Pools ({filteredPools().length})
                                </h3>
                                {/* Search Input */}
                                <Show when={!kioskMode()}>
                                    <div class="relative w-full sm:max-w-xs flex-1 sm:flex-none">
                                        <input
                                            ref={(el) => (searchInputRef = el)}
                                            type="text"
                                            placeholder="Search pools..."
                                            aria-label="Search storage pools"
                                            value={searchTerm()}
                                            onInput={(e) => setSearchTerm(e.currentTarget.value)}
                                            class="w-full pl-8 pr-3 py-1.5 text-sm border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-900 text-slate-800 dark:text-slate-200 placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 dark:focus:border-blue-400 outline-none transition-all"
                                        />
                                        <svg
                                            class="absolute left-2.5 top-2 h-4 w-4 text-slate-400 dark:text-slate-500"
                                            fill="none"
                                            viewBox="0 0 24 24"
                                            stroke="currentColor"
                                        >
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                                        </svg>
                                        <Show when={searchTerm()}>
                                            <button
                                                type="button"
                                                aria-label="Clear pool search"
                                                onClick={() => setSearchTerm('')}
                                                class="absolute right-2.5 top-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
                                            >
                                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                                    <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                                                </svg>
                                            </button>
                                        </Show>
                                    </div>
                                </Show>
                            </div>

                            <Show
                                when={filteredPools().length > 0}
                                fallback={
                                    <div class="p-8 text-center text-slate-500 dark:text-slate-400">
                                        No pools match "{searchTerm()}"
                                    </div>
                                }
                            >
                                <div class="overflow-x-auto" style="scrollbar-width: none; -ms-overflow-style: none;">
                                    <style>{`.overflow-x-auto::-webkit-scrollbar { display: none; }`}</style>
                                    <table class="w-full border-collapse whitespace-nowrap" style={{ "min-width": "650px" }}>
                                        <thead>
                                            <tr class="bg-slate-50 dark:bg-slate-800 text-slate-600 dark:text-slate-300 border-b border-slate-200 dark:border-slate-700">
                                                <th class={`${thClass} pl-4`}>Pool</th>
                                                <th class={thClass}>Cluster</th>
                                                <th class={thClass}>Used</th>
                                                <th class={thClass}>Available</th>
                                                <th class={thClass}>Objects</th>
                                                <th class={`${thClass} min-w-[120px]`}>Usage</th>
                                            </tr>
                                        </thead>
                                        <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
                                            <For each={filteredPools()}>
                                                {(pool) => (
                                                    <tr class="hover:bg-slate-50/80 dark:hover:bg-slate-700/30 transition-colors">
                                                        <td class="px-4 py-2.5 font-medium text-sm text-slate-900 dark:text-slate-100">
                                                            {pool.name}
                                                        </td>
                                                        <td class="px-2 py-2.5 text-xs text-slate-600 dark:text-slate-400">
                                                            {pool.clusterName}
                                                        </td>
                                                        <td class="px-2 py-2.5 text-xs text-slate-700 dark:text-slate-300 font-mono">
                                                            {formatBytes(pool.storedBytes || 0)}
                                                        </td>
                                                        <td class="px-2 py-2.5 text-xs text-slate-700 dark:text-slate-300 font-mono">
                                                            {formatBytes(pool.availableBytes || 0)}
                                                        </td>
                                                        <td class="px-2 py-2.5 text-xs text-slate-700 dark:text-slate-300 font-mono">
                                                            {(pool.objects || 0).toLocaleString()}
                                                        </td>
                                                        <td class="px-2 py-2.5">
                                                            <div class="flex items-center gap-2">
                                                                <div class="w-16">
                                                                    <UsageBar percent={pool.percentUsed || 0} size="sm" />
                                                                </div>
                                                                <span class="text-xs text-slate-600 dark:text-slate-400 font-mono w-12 text-right">
                                                                    {(pool.percentUsed || 0).toFixed(1)}%
                                                                </span>
                                                            </div>
                                                        </td>
                                                    </tr>
                                                )}
                                            </For>
                                        </tbody>
                                    </table>
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
