import { Component, For, Show, createMemo } from 'solid-js';
import { useWebSocket } from '@/App';
import { ProxmoxSectionNav } from '@/components/Proxmox/ProxmoxSectionNav';
import type { CephCluster, CephPool, CephServiceStatus } from '@/types/api';

// Format bytes to human readable
const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
};

// Get health color classes
const getHealthClasses = (health: string) => {
    const h = health?.toUpperCase() || '';
    if (h === 'OK' || h === 'HEALTH_OK') {
        return {
            bg: 'bg-green-100 dark:bg-green-900/30',
            text: 'text-green-700 dark:text-green-400',
            border: 'border-green-200 dark:border-green-800',
            dot: 'bg-green-500',
        };
    }
    if (h === 'WARN' || h === 'HEALTH_WARN' || h === 'WARNING') {
        return {
            bg: 'bg-yellow-100 dark:bg-yellow-900/30',
            text: 'text-yellow-700 dark:text-yellow-400',
            border: 'border-yellow-200 dark:border-yellow-800',
            dot: 'bg-yellow-500',
        };
    }
    if (h === 'ERR' || h === 'HEALTH_ERR' || h === 'ERROR' || h === 'CRITICAL') {
        return {
            bg: 'bg-red-100 dark:bg-red-900/30',
            text: 'text-red-700 dark:text-red-400',
            border: 'border-red-200 dark:border-red-800',
            dot: 'bg-red-500',
        };
    }
    return {
        bg: 'bg-gray-100 dark:bg-gray-800',
        text: 'text-gray-700 dark:text-gray-400',
        border: 'border-gray-200 dark:border-gray-700',
        dot: 'bg-gray-500',
    };
};

// Cluster Overview Card
const ClusterCard: Component<{ cluster: CephCluster }> = (props) => {
    const healthClasses = createMemo(() => getHealthClasses(props.cluster.health));

    const displayHealth = createMemo(() => {
        const h = props.cluster.health?.toUpperCase() || 'UNKNOWN';
        return h.replace('HEALTH_', '');
    });

    return (
        <div class={`rounded-xl border ${healthClasses().border} ${healthClasses().bg} p-5 shadow-sm`}>
            <div class="flex items-start justify-between mb-4">
                <div>
                    <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                        {props.cluster.name || 'Ceph Cluster'}
                    </h3>
                    <Show when={props.cluster.fsid}>
                        <p class="text-xs text-gray-500 dark:text-gray-400 font-mono mt-0.5">
                            {props.cluster.fsid}
                        </p>
                    </Show>
                </div>
                <div class={`flex items-center gap-2 px-3 py-1.5 rounded-full ${healthClasses().bg} ${healthClasses().text} font-semibold text-sm`}>
                    <span class={`w-2 h-2 rounded-full ${healthClasses().dot} animate-pulse`} />
                    {displayHealth()}
                </div>
            </div>

            <Show when={props.cluster.healthMessage}>
                <p class={`text-sm ${healthClasses().text} mb-4 italic`}>
                    {props.cluster.healthMessage}
                </p>
            </Show>

            {/* Capacity Bar */}
            <div class="mb-4">
                <div class="flex justify-between text-sm mb-1">
                    <span class="text-gray-600 dark:text-gray-400">Capacity</span>
                    <span class="font-medium text-gray-900 dark:text-gray-100">
                        {props.cluster.usagePercent?.toFixed(1) || 0}% used
                    </span>
                </div>
                <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5 overflow-hidden">
                    <div
                        class={`h-full rounded-full transition-all duration-500 ${(props.cluster.usagePercent || 0) > 85
                                ? 'bg-red-500'
                                : (props.cluster.usagePercent || 0) > 70
                                    ? 'bg-yellow-500'
                                    : 'bg-blue-500'
                            }`}
                        style={{ width: `${Math.min(props.cluster.usagePercent || 0, 100)}%` }}
                    />
                </div>
                <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400 mt-1">
                    <span>{formatBytes(props.cluster.usedBytes || 0)} used</span>
                    <span>{formatBytes(props.cluster.totalBytes || 0)} total</span>
                </div>
            </div>

            {/* Service Stats Grid */}
            <div class="grid grid-cols-3 gap-3">
                <div class="text-center p-2 bg-white/50 dark:bg-gray-800/50 rounded-lg">
                    <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">
                        {props.cluster.numMons || 0}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">Monitors</div>
                </div>
                <div class="text-center p-2 bg-white/50 dark:bg-gray-800/50 rounded-lg">
                    <div class="text-2xl font-bold text-purple-600 dark:text-purple-400">
                        {props.cluster.numMgrs || 0}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">Managers</div>
                </div>
                <div class="text-center p-2 bg-white/50 dark:bg-gray-800/50 rounded-lg">
                    <div class="text-2xl font-bold text-green-600 dark:text-green-400">
                        <span class={props.cluster.numOsdsUp !== props.cluster.numOsds ? 'text-yellow-600 dark:text-yellow-400' : ''}>
                            {props.cluster.numOsdsUp || 0}
                        </span>
                        <span class="text-gray-400 dark:text-gray-500 text-lg">/{props.cluster.numOsds || 0}</span>
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">OSDs Up</div>
                </div>
            </div>

            {/* PG Count */}
            <Show when={props.cluster.numPGs}>
                <div class="mt-3 text-center text-sm text-gray-500 dark:text-gray-400">
                    <span class="font-medium text-gray-700 dark:text-gray-300">{props.cluster.numPGs?.toLocaleString()}</span>
                    {' '}Placement Groups
                </div>
            </Show>
        </div>
    );
};

// Pool Table
const PoolsTable: Component<{ pools: CephPool[] }> = (props) => {
    return (
        <div class="overflow-hidden rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm">
            <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
                <h3 class="font-semibold text-gray-900 dark:text-gray-100">
                    Pools ({props.pools.length})
                </h3>
            </div>
            <div class="overflow-x-auto">
                <table class="w-full text-sm">
                    <thead class="bg-gray-50 dark:bg-gray-900/50">
                        <tr>
                            <th class="px-4 py-2 text-left text-gray-600 dark:text-gray-400 font-medium">Name</th>
                            <th class="px-4 py-2 text-right text-gray-600 dark:text-gray-400 font-medium">Used</th>
                            <th class="px-4 py-2 text-right text-gray-600 dark:text-gray-400 font-medium">Available</th>
                            <th class="px-4 py-2 text-right text-gray-600 dark:text-gray-400 font-medium">Objects</th>
                            <th class="px-4 py-2 text-center text-gray-600 dark:text-gray-400 font-medium">Usage</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
                        <For each={props.pools}>
                            {(pool) => (
                                <tr class="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                                    <td class="px-4 py-3 font-medium text-gray-900 dark:text-gray-100">
                                        {pool.name}
                                    </td>
                                    <td class="px-4 py-3 text-right text-gray-700 dark:text-gray-300">
                                        {formatBytes(pool.storedBytes || 0)}
                                    </td>
                                    <td class="px-4 py-3 text-right text-gray-700 dark:text-gray-300">
                                        {formatBytes(pool.availableBytes || 0)}
                                    </td>
                                    <td class="px-4 py-3 text-right text-gray-700 dark:text-gray-300">
                                        {(pool.objects || 0).toLocaleString()}
                                    </td>
                                    <td class="px-4 py-3">
                                        <div class="flex items-center gap-2 justify-center">
                                            <div class="w-16 h-2 bg-gray-200 dark:bg-gray-600 rounded-full overflow-hidden">
                                                <div
                                                    class={`h-full rounded-full ${(pool.percentUsed || 0) > 85
                                                            ? 'bg-red-500'
                                                            : (pool.percentUsed || 0) > 70
                                                                ? 'bg-yellow-500'
                                                                : 'bg-blue-500'
                                                        }`}
                                                    style={{ width: `${Math.min(pool.percentUsed || 0, 100)}%` }}
                                                />
                                            </div>
                                            <span class="text-xs text-gray-600 dark:text-gray-400 w-12 text-right">
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
        </div>
    );
};

// Services Status
const ServicesStatus: Component<{ services: CephServiceStatus[] }> = (props) => {
    const getServiceIcon = (type: string) => {
        switch (type.toLowerCase()) {
            case 'mon':
                return '🔵';
            case 'mgr':
                return '🟣';
            case 'osd':
                return '🟢';
            case 'mds':
                return '🟡';
            case 'rgw':
                return '🟠';
            default:
                return '⚪';
        }
    };

    return (
        <div class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-sm p-4">
            <h3 class="font-semibold text-gray-900 dark:text-gray-100 mb-3">Services</h3>
            <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
                <For each={props.services}>
                    {(service) => {
                        const allRunning = service.running === service.total;
                        return (
                            <div
                                class={`p-3 rounded-lg border ${allRunning
                                        ? 'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800'
                                        : service.running > 0
                                            ? 'bg-yellow-50 dark:bg-yellow-900/20 border-yellow-200 dark:border-yellow-800'
                                            : 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800'
                                    }`}
                            >
                                <div class="flex items-center gap-2 mb-1">
                                    <span class="text-lg">{getServiceIcon(service.type)}</span>
                                    <span class="font-medium text-gray-900 dark:text-gray-100 uppercase text-sm">
                                        {service.type}
                                    </span>
                                </div>
                                <div class={`text-lg font-bold ${allRunning
                                        ? 'text-green-700 dark:text-green-400'
                                        : service.running > 0
                                            ? 'text-yellow-700 dark:text-yellow-400'
                                            : 'text-red-700 dark:text-red-400'
                                    }`}>
                                    {service.running}/{service.total}
                                </div>
                                <Show when={service.message}>
                                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-1 truncate" title={service.message}>
                                        {service.message}
                                    </div>
                                </Show>
                            </div>
                        );
                    }}
                </For>
            </div>
        </div>
    );
};

// Main Ceph Page
const Ceph: Component = () => {
    const { state } = useWebSocket();

    const clusters = createMemo(() => state.cephClusters || []);
    const hasClusters = createMemo(() => clusters().length > 0);

    // Aggregate all pools from all clusters
    const allPools = createMemo(() => {
        const pools: CephPool[] = [];
        for (const cluster of clusters()) {
            if (cluster.pools) {
                pools.push(...cluster.pools);
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

    return (
        <div class="flex flex-col gap-4 sm:gap-5">
            {/* Header */}
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div>
                    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-gray-100">
                        Ceph Storage
                    </h1>
                    <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
                        Distributed storage cluster status and pools
                    </p>
                </div>
                <ProxmoxSectionNav current="ceph" />
            </div>

            <Show
                when={hasClusters()}
                fallback={
                    <div class="flex flex-col items-center justify-center py-12 text-center">
                        <div class="w-16 h-16 mb-4 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center">
                            <svg class="w-8 h-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                            </svg>
                        </div>
                        <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100 mb-1">
                            No Ceph Clusters Detected
                        </h3>
                        <p class="text-sm text-gray-500 dark:text-gray-400 max-w-md">
                            Ceph cluster data will appear here when detected via the Pulse agent on your Proxmox nodes.
                            Install the agent on a node with Ceph configured.
                        </p>
                    </div>
                }
            >
                {/* Cluster Cards */}
                <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                    <For each={clusters()}>
                        {(cluster) => <ClusterCard cluster={cluster} />}
                    </For>
                </div>

                {/* Services Overview */}
                <Show when={allServices().length > 0}>
                    <ServicesStatus services={allServices() as CephServiceStatus[]} />
                </Show>

                {/* Pools Table */}
                <Show when={allPools().length > 0}>
                    <PoolsTable pools={allPools()} />
                </Show>
            </Show>
        </div>
    );
};

export default Ceph;
