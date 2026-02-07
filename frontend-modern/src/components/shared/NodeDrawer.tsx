import { Component, Suspense, createSignal, createEffect } from 'solid-js';
import type { Host, Node } from '@/types/api';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import { HostMetadataAPI } from '@/api/hostMetadata';
import { SystemInfoCard } from '@/components/shared/cards/SystemInfoCard';
import { HardwareCard } from '@/components/shared/cards/HardwareCard';
import { RootDiskCard } from '@/components/shared/cards/RootDiskCard';
import { NetworkInterfacesCard } from '@/components/shared/cards/NetworkInterfacesCard';
import { DisksCard } from '@/components/shared/cards/DisksCard';

interface NodeDrawerProps {
    node: Node;
    host?: Host;
    customUrl?: string; // Nodes don't typically have custom URL in current architecture but we can keep it
    onCustomUrlChange?: (hostId: string, url: string) => void;
}

export const NodeDrawer: Component<NodeDrawerProps> = (props) => {
    const [activeTab, setActiveTab] = createSignal<'overview' | 'discovery'>('overview');

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
                    <NetworkInterfacesCard interfaces={props.host?.networkInterfaces} />
                    <DisksCard disks={props.host?.disks} />
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
