import { Component, Suspense, createSignal } from 'solid-js';
import type { Host, Node } from '@/types/api';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import { SystemInfoCard } from '@/components/shared/cards/SystemInfoCard';
import { HardwareCard } from '@/components/shared/cards/HardwareCard';
import { RootDiskCard } from '@/components/shared/cards/RootDiskCard';
import { NetworkInterfacesCard } from '@/components/shared/cards/NetworkInterfacesCard';
import { DisksCard } from '@/components/shared/cards/DisksCard';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';

interface NodeDrawerProps {
    node: Node;
    host?: Host;
    customUrl?: string; // Nodes don't typically have custom URL in current architecture but we can keep it
    onCustomUrlChange?: (hostId: string, url: string) => void;
}

export const NodeDrawer: Component<NodeDrawerProps> = (props) => {
    const [activeTab, setActiveTab] = createSignal<'overview' | 'discovery'>('overview');
    const metadataId = () => props.node.id || props.node.name;

    const switchTab = (tab: 'overview' | 'discovery') => {
        setActiveTab(tab);
    };

    return (
        <div class="space-y-3">
            {/* Tabs */}
            <div class="flex items-center gap-6 border-b border-border px-1 mb-1">
                <button
                    onClick={() => switchTab('overview')}
                    class={`pb-2 text-sm font-medium transition-colors relative ${activeTab() === 'overview'
 ? 'text-blue-600 dark:text-blue-400'
 : ' hover:text-muted'
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
 : ' hover:text-muted'
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
                <div class="mt-3">
                    <WebInterfaceUrlField
                        metadataKind="host"
                        metadataId={metadataId()}
                        targetLabel="host"
                        customUrl={props.customUrl}
                        onCustomUrlChange={(url) => props.onCustomUrlChange?.(metadataId(), url)}
                    />
                </div>
            </div>

            {/* Discovery Tab */}
            <div class={activeTab() === 'discovery' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
                <Suspense fallback={
                    <div class="flex items-center justify-center py-8">
                        <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
                        <span class="ml-2 text-sm text-muted">Loading discovery...</span>
                    </div>
                }>
                    <DiscoveryTab
                        resourceType="host" /* Assuming 'host' type works for PVE nodes discovery, or if backend treats them same */
                        hostId={metadataId()}
                        resourceId={metadataId()}
                        hostname={props.node.name}
                    />
                </Suspense>
            </div>
        </div>
    );
};
