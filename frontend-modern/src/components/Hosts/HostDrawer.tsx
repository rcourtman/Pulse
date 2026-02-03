import { Component, Show, For, Suspense, createSignal } from 'solid-js';
import { Host } from '@/types/api';
import { formatBytes, formatUptime } from '@/utils/format';
import { HistoryChart } from '../shared/HistoryChart';
import { ResourceType, HistoryTimeRange } from '@/api/charts';
import { hasFeature } from '@/stores/license';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import { StackedDiskBar } from '@/components/Dashboard/StackedDiskBar';

interface HostDrawerProps {
    host: Host;
    onClose: () => void;
    customUrl?: string;
    onCustomUrlChange?: (hostId: string, url: string) => void;
}

export const HostDrawer: Component<HostDrawerProps> = (props) => {
    const [activeTab, setActiveTab] = createSignal<'overview' | 'discovery'>('overview');
    const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>('1h');

    // For unified host agents, the backend stores metrics with resourceType 'host'
    const metricsResource = { type: 'host' as ResourceType, id: props.host.id };

    const switchTab = (tab: 'overview' | 'discovery') => {
        setActiveTab(tab);
    };

    const isHistoryLocked = () => !hasFeature('long_term_metrics') && (historyRange() === '30d' || historyRange() === '90d');

    const diskStats = () => {
        if (!props.host.disks || props.host.disks.length === 0) return { percent: 0, used: 0, total: 0 };
        const totalUsed = props.host.disks.reduce((sum, d) => sum + (d.used ?? 0), 0);
        const totalSize = props.host.disks.reduce((sum, d) => sum + (d.total ?? 0), 0);
        return {
            percent: totalSize > 0 ? (totalUsed / totalSize) * 100 : 0,
            used: totalUsed,
            total: totalSize
        };
    };

    return (
        <div class="space-y-3">
            {/* Tabs */}
            <div class="flex items-center gap-6 border-b border-gray-200 dark:border-gray-700 px-1 mb-1">
                <button
                    onClick={() => switchTab('overview')}
                    class={`pb-2 text-sm font-medium transition-colors relative ${activeTab() === 'overview'
                        ? 'text-blue-600 dark:text-blue-400'
                        : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                        }`}
                >
                    Overview
                    {activeTab() === 'overview' && (
                        <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
                    )}
                </button>
                <button
                    onClick={() => switchTab('discovery')}
                    class={`pb-2 text-sm font-medium transition-colors relative ${activeTab() === 'discovery'
                        ? 'text-blue-600 dark:text-blue-400'
                        : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                        }`}
                >
                    Discovery
                    {activeTab() === 'discovery' && (
                        <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
                    )}
                </button>
            </div>

            {/* Overview Tab */}
            <div class={activeTab() === 'overview' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
                <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full [&>*]:overflow-hidden">
                    {/* System Info */}
                    <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">System</div>
                        <div class="space-y-1.5 text-[11px]">
                            <div class="flex items-center justify-between gap-2 min-w-0">
                                <span class="text-gray-500 dark:text-gray-400 shrink-0">Hostname</span>
                                <span class="font-medium text-gray-700 dark:text-gray-200 select-all truncate" title={props.host.hostname}>{props.host.hostname}</span>
                            </div>
                            <div class="flex items-center justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Platform</span>
                                <span class="font-medium text-gray-700 dark:text-gray-200 capitalize">{props.host.platform || 'Unknown'}</span>
                            </div>
                            <div class="flex items-center justify-between gap-2 min-w-0">
                                <span class="text-gray-500 dark:text-gray-400 shrink-0">OS</span>
                                <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={`${props.host.osName} ${props.host.osVersion}`}>{props.host.osName} {props.host.osVersion}</span>
                            </div>
                            <div class="flex items-center justify-between gap-2 min-w-0">
                                <span class="text-gray-500 dark:text-gray-400 shrink-0">Kernel</span>
                                <span class="font-medium text-gray-700 dark:text-gray-200 truncate" title={props.host.kernelVersion}>{props.host.kernelVersion}</span>
                            </div>
                            <div class="flex items-center justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Architecture</span>
                                <span class="font-medium text-gray-700 dark:text-gray-200">{props.host.architecture}</span>
                            </div>
                            <Show when={props.host.uptimeSeconds}>
                                <div class="flex items-center justify-between">
                                    <span class="text-gray-500 dark:text-gray-400">Uptime</span>
                                    <span class="font-medium text-gray-700 dark:text-gray-200">{formatUptime(props.host.uptimeSeconds!)}</span>
                                </div>
                            </Show>
                        </div>
                    </div>

                    {/* Hardware Info */}
                    <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Hardware</div>
                        <div class="space-y-1.5 text-[11px]">
                            <div class="flex items-center justify-between">
                                <span class="text-gray-500 dark:text-gray-400">CPU</span>
                                <span class="font-medium text-gray-700 dark:text-gray-200">{props.host.cpuCount} Cores</span>
                            </div>
                            <div class="flex items-center justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Memory</span>
                                <span class="font-medium text-gray-700 dark:text-gray-200">
                                    {formatBytes(props.host.memory?.total || 0)}
                                </span>
                            </div>
                            <div class="flex items-center justify-between">
                                <span class="text-gray-500 dark:text-gray-400">Agent</span>
                                <span class="font-medium text-gray-700 dark:text-gray-200">{props.host.agentVersion}</span>
                            </div>
                        </div>
                    </div>

                    {/* Network Interfaces */}
                    <Show when={props.host.networkInterfaces && props.host.networkInterfaces.length > 0}>
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Network</div>
                            <div class="max-h-[140px] overflow-y-auto custom-scrollbar space-y-2">
                                <For each={props.host.networkInterfaces}>
                                    {(iface) => (
                                        <div class="rounded border border-dashed border-gray-200 p-2 dark:border-gray-700 overflow-hidden">
                                            <div class="flex items-center gap-2 text-[11px] font-medium text-gray-700 dark:text-gray-200 min-w-0">
                                                <span class="truncate min-w-0">{iface.name}</span>
                                                <Show when={iface.mac}>
                                                    <span class="text-[9px] text-gray-400 dark:text-gray-500 font-normal truncate shrink-0 max-w-[100px]" title={iface.mac}>{iface.mac}</span>
                                                </Show>
                                            </div>
                                            <Show when={iface.addresses && iface.addresses.length > 0}>
                                                <div class="flex flex-wrap gap-1 mt-1">
                                                    <For each={iface.addresses}>
                                                        {(ip) => (
                                                            <span class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-200 max-w-full truncate" title={ip}>
                                                                {ip}
                                                            </span>
                                                        )}
                                                    </For>
                                                </div>
                                            </Show>
                                        </div>
                                    )}
                                </For>
                            </div>
                        </div>
                    </Show>

                    {/* Disk Usage */}
                    <Show when={props.host.disks && props.host.disks.length > 0}>
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Disks</div>
                            <div class="max-h-[140px] overflow-y-auto custom-scrollbar space-y-2">
                                {/* Summary bar */}
                                <div class="mb-3">
                                    <div class="flex justify-between text-[10px] mb-1">
                                        <span class="text-gray-500 dark:text-gray-400">Total Usage</span>
                                        <span class="text-gray-700 dark:text-gray-200">
                                            {formatBytes(diskStats().used)} / {formatBytes(diskStats().total)}
                                        </span>
                                    </div>
                                    <StackedDiskBar
                                        disks={props.host.disks}
                                        mode="mini"
                                        aggregateDisk={{
                                            total: diskStats().total,
                                            used: diskStats().used,
                                            free: diskStats().total - diskStats().used,
                                            usage: diskStats().percent / 100
                                        }}
                                    />
                                </div>

                                <For each={props.host.disks}>
                                    {(disk) => (
                                        <div class="text-[10px]">
                                            <div class="flex justify-between mb-0.5">
                                                <span class="text-gray-600 dark:text-gray-300 truncate max-w-[120px]" title={disk.mountpoint}>{disk.mountpoint}</span>
                                                <span class="text-gray-500 dark:text-gray-400">{formatBytes(disk.used)} / {formatBytes(disk.total)}</span>
                                            </div>
                                            <div class="h-1.5 w-full rounded-full bg-gray-200 dark:bg-gray-700 overflow-hidden">
                                                <div
                                                    class="h-full rounded-full transition-all duration-500 bg-blue-500"
                                                    style={{ width: `${Math.min(100, Math.max(0, (disk.used / disk.total) * 100))}%` }}
                                                />
                                            </div>
                                        </div>
                                    )}
                                </For>
                            </div>
                        </div>
                    </Show>
                </div>

                {/* Performance Charts */}
                <div class="mt-3 space-y-3">
                    <div class="flex items-center gap-2">
                        <svg class="w-3.5 h-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <circle cx="12" cy="12" r="10" />
                            <path stroke-linecap="round" d="M12 6v6l4 2" />
                        </svg>
                        <select
                            value={historyRange()}
                            onChange={(e) => setHistoryRange(e.currentTarget.value as HistoryTimeRange)}
                            class="text-[11px] font-medium pl-2 pr-6 py-1 rounded-md border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 cursor-pointer focus:ring-1 focus:ring-blue-500 focus:border-blue-500 appearance-none"
                            style={{ "background-image": "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%239ca3af' stroke-width='2'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E\")", "background-repeat": "no-repeat", "background-position": "right 6px center" }}
                        >
                            <option value="1h">Last 1 hour</option>
                            <option value="6h">Last 6 hours</option>
                            <option value="12h">Last 12 hours</option>
                            <option value="24h">Last 24 hours</option>
                            <option value="7d">Last 7 days</option>
                            <option value="30d">Last 30 days</option>
                            <option value="90d">Last 90 days</option>
                        </select>
                    </div>

                    <div class="relative">
                        <div class="space-y-3">
                            <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(33.333%-0.5rem)] [&>*]:min-w-[250px]">
                                <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                    <HistoryChart
                                        resourceType={metricsResource.type}
                                        resourceId={metricsResource.id}
                                        metric="cpu"
                                        height={120}
                                        color="#8b5cf6"
                                        label="CPU"
                                        unit="%"
                                        range={historyRange()}
                                        hideSelector={true}
                                        compact={true}
                                        hideLock={true}
                                    />
                                </div>
                                <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                    <HistoryChart
                                        resourceType={metricsResource.type}
                                        resourceId={metricsResource.id}
                                        metric="memory"
                                        height={120}
                                        color="#f59e0b"
                                        label="Memory"
                                        unit="%"
                                        range={historyRange()}
                                        hideSelector={true}
                                        compact={true}
                                        hideLock={true}
                                    />
                                </div>
                                <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                    <HistoryChart
                                        resourceType={metricsResource.type}
                                        resourceId={metricsResource.id}
                                        metric="disk"
                                        height={120}
                                        color="#10b981"
                                        label="Disk"
                                        unit="%"
                                        range={historyRange()}
                                        hideSelector={true}
                                        compact={true}
                                        hideLock={true}
                                    />
                                </div>
                            </div>
                            <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(50%-0.375rem)] [&>*]:min-w-[250px]">
                                <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                    <HistoryChart
                                        resourceType={metricsResource.type}
                                        resourceId={metricsResource.id}
                                        metric="netin"
                                        height={120}
                                        color="#3b82f6"
                                        label="Net In"
                                        unit="B/s"
                                        range={historyRange()}
                                        hideSelector={true}
                                        compact={true}
                                        hideLock={true}
                                    />
                                </div>
                                <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                                    <HistoryChart
                                        resourceType={metricsResource.type}
                                        resourceId={metricsResource.id}
                                        metric="netout"
                                        height={120}
                                        color="#6366f1"
                                        label="Net Out"
                                        unit="B/s"
                                        range={historyRange()}
                                        hideSelector={true}
                                        compact={true}
                                        hideLock={true}
                                    />
                                </div>
                            </div>
                        </div>
                        {/* Lock Overlay */}
                        <Show when={isHistoryLocked()}>
                            <div class="absolute inset-0 z-10 flex flex-col items-center justify-center backdrop-blur-sm bg-white/60 dark:bg-gray-900/60 rounded-lg">
                                <div class="bg-indigo-500 rounded-full p-3 shadow-lg mb-3">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                        <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
                                        <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
                                    </svg>
                                </div>
                                <h3 class="text-lg font-bold text-gray-900 dark:text-white mb-1">{historyRange() === '30d' ? '30' : '90'}-Day History</h3>
                                <p class="text-sm text-gray-600 dark:text-gray-300 text-center max-w-[260px] mb-4">
                                    Upgrade to Pulse Pro to unlock {historyRange() === '30d' ? '30' : '90'} days of historical data retention.
                                </p>
                                <a
                                    href="https://pulserelay.pro/pricing"
                                    target="_blank"
                                    class="px-4 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium rounded-md shadow-sm transition-colors"
                                >
                                    Unlock Pro Features
                                </a>
                            </div>
                        </Show>
                    </div>
                </div>
            </div>

            {/* Discovery Tab */}
            <div class={activeTab() === 'discovery' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
                <Suspense fallback={
                    <div class="flex items-center justify-center py-8">
                        <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
                        <span class="ml-2 text-sm text-gray-500 dark:text-gray-400">Loading discovery...</span>
                    </div>
                }>
                    <DiscoveryTab
                        resourceType="host"
                        hostId={props.host.id}
                        resourceId={props.host.id} /* For hosts, typically same as hostId */
                        hostname={props.host.hostname}
                        guestId={props.host.id}
                        customUrl={props.customUrl}
                        onCustomUrlChange={(url) => props.onCustomUrlChange?.(props.host.id, url)}
                        commandsEnabled={props.host.commandsEnabled}
                    />
                </Suspense>
            </div>
        </div>
    );
};
