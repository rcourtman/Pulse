import { Component, Show, Suspense, createSignal, createEffect } from 'solid-js';
import { Node } from '@/types/api';
import { HistoryChart } from '../shared/HistoryChart';
import { ResourceType, HistoryTimeRange } from '@/api/charts';
import { isRangeLocked } from '@/stores/license';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import { HostMetadataAPI } from '@/api/hostMetadata';
import { SystemInfoCard } from '@/components/shared/cards/SystemInfoCard';
import { HardwareCard } from '@/components/shared/cards/HardwareCard';
import { RootDiskCard } from '@/components/shared/cards/RootDiskCard';

interface NodeDrawerProps {
    node: Node;
    customUrl?: string; // Nodes don't typically have custom URL in current architecture but we can keep it
    onCustomUrlChange?: (hostId: string, url: string) => void;
}

export const NodeDrawer: Component<NodeDrawerProps> = (props) => {
    const [activeTab, setActiveTab] = createSignal<'overview' | 'discovery'>('overview');
    const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>('1h');

    // For nodes, we treat them as 'host' for charts if consistent, or 'node' if charts API expects it.
    // Charts API (api/charts.ts) shows ResourceType includes 'node' and 'host'.
    // Typically 'node' is used for PVE nodes.
    const metricsResource = { type: 'node' as ResourceType, id: props.node.id || props.node.name };

    // Fetch custom URL from metadata
    const [fetchedCustomUrl, setFetchedCustomUrl] = createSignal<string | undefined>(props.customUrl);

    // Fetch metadata on mount
    createEffect(() => {
        const hostId = props.node.id || props.node.name;
        HostMetadataAPI.getMetadata(hostId).then(meta => {
            if (meta && meta.customUrl) {
                setFetchedCustomUrl(meta.customUrl);
            }
        }).catch(e => console.error("Failed to load node metadata", e));
    });

    const handleCustomUrlChange = (url: string) => {
        setFetchedCustomUrl(url);
        if (props.onCustomUrlChange) {
            props.onCustomUrlChange(props.node.id || props.node.name, url);
        }
    };

    const switchTab = (tab: 'overview' | 'discovery') => {
        setActiveTab(tab);
    };

    const isHistoryLocked = () => isRangeLocked(historyRange());

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
                    <SystemInfoCard variant="node" node={props.node} />
                    <HardwareCard variant="node" node={props.node} />
                    <RootDiskCard node={props.node} />
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
                        resourceType="host" /* Assuming 'host' type works for PVE nodes discovery, or if backend treats them same */
                        hostId={props.node.id || props.node.name}
                        resourceId={props.node.id || props.node.name}
                        hostname={props.node.name}
                        guestId={props.node.id || props.node.name}
                        urlMetadataKind="host"
                        urlMetadataId={props.node.id || props.node.name}
                        urlTargetLabel="host"
                        customUrl={fetchedCustomUrl()}
                        onCustomUrlChange={handleCustomUrlChange}
                    />
                </Suspense>
            </div>
        </div>
    );
};
