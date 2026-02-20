import { Component, Show, For, Suspense, createSignal } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import type { WorkloadGuest } from '@/types/workloads';
import { formatBytes, formatUptime } from '@/utils/format';
import { DiskList } from './DiskList';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import type { ResourceType as DiscoveryResourceType } from '@/types/discovery';
import { resolveWorkloadType } from '@/utils/workloads';
import { buildInfrastructureHrefForWorkload } from './infrastructureLink';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';

type Guest = WorkloadGuest;

interface GuestDrawerProps {
    guest: Guest;
    onClose: () => void;
    customUrl?: string;
    onCustomUrlChange?: (guestId: string, url: string) => void;
}

export const GuestDrawer: Component<GuestDrawerProps> = (props) => {
    const navigate = useNavigate();
    const guestId = () => {
        if (props.guest.id) return props.guest.id;
        return `${props.guest.instance}:${props.guest.node}:${props.guest.vmid}`;
    };
    const infrastructureHref = () => buildInfrastructureHrefForWorkload(props.guest);

    const isVM = (guest: Guest): boolean => {
        return resolveWorkloadType(guest) === 'vm';
    };

    const hasOsInfo = () => {
        // Both VMs and containers can have OS info
        return (props.guest.osName?.length ?? 0) > 0 || (props.guest.osVersion?.length ?? 0) > 0;
    };

    const osName = () => {
        return props.guest.osName || '';
    };

    const osVersion = () => {
        return props.guest.osVersion || '';
    };

    const hasAgentInfo = () => {
        return !!props.guest.agentVersion;
    };

    const agentVersion = () => {
        return props.guest.agentVersion || '';
    };

    const ipAddresses = () => {
        return props.guest.ipAddresses || [];
    };

    const memoryExtraLines = () => {
        if (!props.guest.memory) return undefined;
        const lines: string[] = [];
        const total = props.guest.memory.total ?? 0;
        if (
            props.guest.memory.balloon &&
            props.guest.memory.balloon > 0 &&
            props.guest.memory.balloon !== total
        ) {
            lines.push(`Balloon: ${formatBytes(props.guest.memory.balloon)}`);
        }
        if (props.guest.memory.swapTotal && props.guest.memory.swapTotal > 0) {
            const swapUsed = props.guest.memory.swapUsed ?? 0;
            lines.push(`Swap: ${formatBytes(swapUsed)} / ${formatBytes(props.guest.memory.swapTotal)}`);
        }
        return lines.length > 0 ? lines : undefined;
    };

    const hasFilesystemDetails = () => {
        if (!props.guest.disks) return false;
        return props.guest.disks.length > 0;
    };

    const networkInterfaces = () => {
        // Both VMs and containers can have network interfaces
        return props.guest.networkInterfaces || [];
    };

    const hasNetworkInterfaces = () => {
        return networkInterfaces().length > 0;
    };

    const [activeTab, setActiveTab] = createSignal<'overview' | 'discovery'>('overview');

    // All tabs are always rendered (hidden via CSS) to avoid any DOM
    // mount/unmount during tab switches. Mounting new components inside
    // a <For>-rendered table row causes SolidJS to recreate the row,
    // which detaches the element and resets the scroll container.
    const switchTab = (tab: 'overview' | 'discovery') => {
        setActiveTab(tab);
    };

    // Get discovery resource type for the guest
    const discoveryResourceType = (): DiscoveryResourceType => {
        const type = resolveWorkloadType(props.guest);
        if (type === 'vm') return 'vm';
        if (type === 'docker') return 'docker';
        if (type === 'k8s') return 'k8s';
        return 'lxc';
    };

    const urlTargetLabel = () => {
        const type = resolveWorkloadType(props.guest);
        if (type === 'docker') return 'container';
        if (type === 'k8s') return 'workload';
        return 'guest';
    };
    return (
        <div class="space-y-3">
            {/* Tabs */}
            <div class="flex items-center gap-6 border-b border-slate-200 dark:border-slate-700 px-1 mb-1">
                <button
                    onClick={() => switchTab('overview')}
                    class={`pb-2 text-sm font-medium transition-colors relative ${activeTab() === 'overview'
                        ? 'text-blue-600 dark:text-blue-400'
                        : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200'
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
                        : 'text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200'
                        }`}
                >
                    Discovery
                    {activeTab() === 'discovery' && (
                        <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
                    )}
                </button>
            </div>
            <div class="flex justify-end">
                <button
                    type="button"
                    onClick={() => navigate(infrastructureHref())}
                    class="inline-flex items-center rounded border border-slate-300 bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700 transition-colors hover:bg-slate-200 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
                >
                    Open related infrastructure
                </button>
            </div>

            {/* Use CSS hidden instead of Show to avoid mount/unmount which causes scroll jumps.
                 overflow-anchor: none prevents browser scroll anchoring from jumping when display toggles. */}
            <div class={activeTab() === 'overview' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
                {/* Flex layout - items grow to fill space, max ~4 per row */}
                <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full [&>*]:overflow-hidden">
                    {/* System Info - always show */}
                    <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
                        <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">System</div>
                        <div class="space-y-1.5 text-[11px]">
                            <Show when={props.guest.cpus}>
                                <div class="flex items-center justify-between">
                                    <span class="text-slate-500 dark:text-slate-400">CPUs</span>
                                    <span class="font-medium text-slate-700 dark:text-slate-200">{props.guest.cpus}</span>
                                </div>
                            </Show>
                            <Show when={props.guest.uptime > 0}>
                                <div class="flex items-center justify-between">
                                    <span class="text-slate-500 dark:text-slate-400">Uptime</span>
                                    <span class="font-medium text-slate-700 dark:text-slate-200">{formatUptime(props.guest.uptime)}</span>
                                </div>
                            </Show>
                            <Show when={props.guest.node}>
                                <div class="flex items-center justify-between">
                                    <span class="text-slate-500 dark:text-slate-400">Node</span>
                                    <span class="font-medium text-slate-700 dark:text-slate-200">{props.guest.node}</span>
                                </div>
                            </Show>
                            <Show when={hasAgentInfo()}>
                                <div class="flex items-center justify-between">
                                    <span class="text-slate-500 dark:text-slate-400">Agent</span>
                                    <span class="font-medium text-slate-700 dark:text-slate-200 truncate ml-2" title={isVM(props.guest) ? `QEMU guest agent ${agentVersion()}` : agentVersion()}>
                                        {isVM(props.guest) ? `QEMU ${agentVersion()}` : agentVersion()}
                                    </span>
                                </div>
                            </Show>
                        </div>
                    </div>

                    {/* Guest Info - OS and IPs */}
                    <Show when={hasOsInfo() || ipAddresses().length > 0}>
                        <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Guest Info</div>
                            <div class="space-y-2">
                                <Show when={hasOsInfo()}>
                                    <div class="text-[11px] text-slate-600 dark:text-slate-300 truncate" title={`${osName()} ${osVersion()}`.trim()}>
                                        <Show when={osName().length > 0}>
                                            <span class="font-medium">{osName()}</span>
                                        </Show>
                                        <Show when={osName().length > 0 && osVersion().length > 0}>
                                            <span class="text-slate-400 dark:text-slate-500 mx-1">•</span>
                                        </Show>
                                        <Show when={osVersion().length > 0}>
                                            <span>{osVersion()}</span>
                                        </Show>
                                    </div>
                                </Show>
                                <Show when={ipAddresses().length > 0}>
                                    <div class="flex flex-wrap gap-1">
                                        <For each={ipAddresses()}>
                                            {(ip) => (
                                                <span class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900 dark:text-blue-200 max-w-full truncate" title={ip}>
                                                    {ip}
                                                </span>
                                            )}
                                        </For>
                                    </div>
                                </Show>
                            </div>
                        </div>
                    </Show>

                    {/* Memory Details */}
                    <Show when={memoryExtraLines() && memoryExtraLines()!.length > 0}>
                        <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Memory</div>
                            <div class="space-y-1 text-[11px] text-slate-600 dark:text-slate-300">
                                <For each={memoryExtraLines()!}>{(line) => <div>{line}</div>}</For>
                            </div>
                        </div>
                    </Show>

                    {/* Backup Info */}
                    <Show when={props.guest.lastBackup}>
                        <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Backup</div>
                            <div class="space-y-1 text-[11px]">
                                {(() => {
                                    const backupDate = new Date(props.guest.lastBackup);
                                    const now = new Date();
                                    const daysSince = Math.floor((now.getTime() - backupDate.getTime()) / (1000 * 60 * 60 * 24));
                                    const isOld = daysSince > 7;
                                    const isCritical = daysSince > 30;
                                    return (
                                        <>
                                            <div class="flex items-center justify-between">
                                                <span class="text-slate-500 dark:text-slate-400">Last Backup</span>
                                                <span class={`font-medium ${isCritical ? 'text-red-600 dark:text-red-400' : isOld ? 'text-amber-600 dark:text-amber-400' : 'text-green-600 dark:text-green-400'}`}>
                                                    {daysSince === 0 ? 'Today' : daysSince === 1 ? 'Yesterday' : `${daysSince}d ago`}
                                                </span>
                                            </div>
                                            <div class="text-[10px] text-slate-400 dark:text-slate-500">
                                                {backupDate.toLocaleDateString()}
                                            </div>
                                        </>
                                    );
                                })()}
                            </div>
                        </div>
                    </Show>

                    {/* Tags */}
                    <Show when={props.guest.tags && (Array.isArray(props.guest.tags) ? props.guest.tags.length > 0 : props.guest.tags.length > 0)}>
                        <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Tags</div>
                            <div class="flex flex-wrap gap-1">
                                <For each={Array.isArray(props.guest.tags) ? props.guest.tags : (props.guest.tags?.split(',') || [])}>
                                    {(tag) => (
                                        <span class="inline-block rounded bg-slate-100 px-1.5 py-0.5 text-[10px] text-slate-700 dark:bg-slate-700 dark:text-slate-200">
                                            {tag.trim()}
                                        </span>
                                    )}
                                </For>
                            </div>
                        </div>
                    </Show>

                    {/* Filesystems */}
                    <Show when={hasFilesystemDetails() && props.guest.disks && props.guest.disks.length > 0}>
                        <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Filesystems</div>
                            <div class="text-[11px] text-slate-600 dark:text-slate-300">
                                <DiskList
                                    disks={props.guest.disks || []}
                                    diskStatusReason={isVM(props.guest) ? (props.guest as any).diskStatusReason : undefined}
                                />
                            </div>
                        </div>
                    </Show>

                    {/* Network Interfaces */}
                    <Show when={hasNetworkInterfaces()}>
                        <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Network</div>
                            <div class="space-y-2">
                                <For each={networkInterfaces().slice(0, 4)}>
                                    {(iface) => {
                                        const addresses = iface.addresses ?? [];
                                        const hasTraffic = (iface.rxBytes ?? 0) > 0 || (iface.txBytes ?? 0) > 0;
                                        return (
                                            <div class="rounded border border-dashed border-slate-200 p-2 dark:border-slate-700 overflow-hidden">
                                                <div class="flex items-center gap-2 text-[11px] font-medium text-slate-700 dark:text-slate-200 min-w-0">
                                                    <span class="truncate min-w-0">{iface.name || 'interface'}</span>
                                                    <Show when={iface.mac}>
                                                        <span class="text-[9px] text-slate-400 dark:text-slate-500 font-normal truncate shrink-0 max-w-[100px]" title={iface.mac}>{iface.mac}</span>
                                                    </Show>
                                                </div>
                                                <Show when={addresses.length > 0}>
                                                    <div class="flex flex-wrap gap-1 mt-1">
                                                        <For each={addresses}>
                                                            {(ip) => (
                                                                <span class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900 dark:text-blue-200 max-w-full truncate" title={ip}>
                                                                    {ip}
                                                                </span>
                                                            )}
                                                        </For>
                                                    </div>
                                                </Show>
                                                <Show when={hasTraffic}>
                                                    <div class="flex gap-3 mt-1 text-[10px] text-slate-500 dark:text-slate-400">
                                                        <span>RX {formatBytes(iface.rxBytes ?? 0)}</span>
                                                        <span>TX {formatBytes(iface.txBytes ?? 0)}</span>
                                                    </div>
                                                </Show>
                                            </div>
                                        );
                                    }}
                                </For>
                            </div>
                        </div>
                    </Show>
                </div>

                <div class="mt-3">
                    <WebInterfaceUrlField
                        metadataKind="guest"
                        metadataId={guestId()}
                        targetLabel={urlTargetLabel()}
                        customUrl={props.customUrl}
                        onCustomUrlChange={(url) => props.onCustomUrlChange?.(guestId(), url)}
                    />
                </div>

            </div>

            {/* Always rendered, hidden via CSS. Wrapped in a local Suspense
                     so DiscoveryTab's createResource loading state doesn't bubble
                     up to the app-level Suspense and replace the entire page. */}
            <div class={activeTab() === 'discovery' ? '' : 'hidden'} style={{ "overflow-anchor": "none" }}>
                <Suspense fallback={
                    <div class="flex items-center justify-center py-8">
                        <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
                        <span class="ml-2 text-sm text-slate-500 dark:text-slate-400">Loading discovery...</span>
                    </div>
                }>
                    <DiscoveryTab
                        resourceType={discoveryResourceType()}
                        hostId={props.guest.node}
                        resourceId={String(props.guest.vmid)}
                        hostname={props.guest.name}
                    />
                </Suspense>
            </div>
        </div>
    );
};
