import { Component, Show, For, createSignal } from 'solid-js';
import { VM, Container } from '@/types/api';
import { formatBytes, formatUptime } from '@/utils/format';
import { DiskList } from './DiskList';
import { HistoryChart } from '../shared/HistoryChart';
import { UnifiedHistoryChart } from '../shared/UnifiedHistoryChart';
import { HistoryTimeRange, ResourceType } from '@/api/charts';

type Guest = VM | Container;

interface GuestDrawerProps {
    guest: Guest;
    metricsKey: string;
    onClose: () => void;
}

export const GuestDrawer: Component<GuestDrawerProps> = (props) => {
    const isVM = (guest: Guest): guest is VM => {
        return guest.type === 'qemu';
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
            lines.push(`Balloon: ${formatBytes(props.guest.memory.balloon, 0)}`);
        }
        if (props.guest.memory.swapTotal && props.guest.memory.swapTotal > 0) {
            const swapUsed = props.guest.memory.swapUsed ?? 0;
            lines.push(`Swap: ${formatBytes(swapUsed, 0)} / ${formatBytes(props.guest.memory.swapTotal, 0)}`);
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

    const fallbackGuestId = () => {
        return props.guest.id || `${props.guest.instance}:${props.guest.node}:${props.guest.vmid}`;
    };

    const metricsResource = (): { type: ResourceType; id: string } => {
        const key = props.metricsKey || '';
        const separatorIndex = key.indexOf(':');
        const fallbackType: ResourceType = isVM(props.guest) ? 'vm' : 'container';

        if (separatorIndex === -1) {
            return { type: fallbackType, id: fallbackGuestId() };
        }

        const kind = key.slice(0, separatorIndex);
        const id = key.slice(separatorIndex + 1) || fallbackGuestId();
        const type: ResourceType = kind === 'vm' || kind === 'container' ? kind : fallbackType;

        return { type, id };
    };

    const [activeTab, setActiveTab] = createSignal<'overview' | 'history'>('overview');
    const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>('24h');
    const [viewMode, setViewMode] = createSignal<'unified' | 'split'>('unified');

    return (
        <div class="space-y-3">
            {/* Tabs */}
            <div class="flex items-center gap-6 border-b border-gray-200 dark:border-gray-700 px-1 mb-1">
                <button
                    onClick={() => setActiveTab('overview')}
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
                    onClick={() => setActiveTab('history')}
                    class={`pb-2 text-sm font-medium transition-colors relative ${activeTab() === 'history'
                        ? 'text-blue-600 dark:text-blue-400'
                        : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                        }`}
                >
                    History
                    {activeTab() === 'history' && (
                        <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
                    )}
                </button>
            </div>

            <Show when={activeTab() === 'overview'}>
                {/* Flex layout - items grow to fill space, max ~4 per row */}
                <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full">
                    {/* System Info - always show */}
                    <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                        <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">System</div>
                        <div class="space-y-1.5 text-[11px]">
                            <Show when={props.guest.cpus}>
                                <div class="flex items-center justify-between">
                                    <span class="text-gray-500 dark:text-gray-400">CPUs</span>
                                    <span class="font-medium text-gray-700 dark:text-gray-200">{props.guest.cpus}</span>
                                </div>
                            </Show>
                            <Show when={props.guest.uptime > 0}>
                                <div class="flex items-center justify-between">
                                    <span class="text-gray-500 dark:text-gray-400">Uptime</span>
                                    <span class="font-medium text-gray-700 dark:text-gray-200">{formatUptime(props.guest.uptime)}</span>
                                </div>
                            </Show>
                            <Show when={props.guest.node}>
                                <div class="flex items-center justify-between">
                                    <span class="text-gray-500 dark:text-gray-400">Node</span>
                                    <span class="font-medium text-gray-700 dark:text-gray-200">{props.guest.node}</span>
                                </div>
                            </Show>
                            <Show when={hasAgentInfo()}>
                                <div class="flex items-center justify-between">
                                    <span class="text-gray-500 dark:text-gray-400">Agent</span>
                                    <span class="font-medium text-gray-700 dark:text-gray-200 truncate ml-2" title={isVM(props.guest) ? `QEMU guest agent ${agentVersion()}` : agentVersion()}>
                                        {isVM(props.guest) ? `QEMU ${agentVersion()}` : agentVersion()}
                                    </span>
                                </div>
                            </Show>
                        </div>
                    </div>

                    {/* Guest Info - OS and IPs */}
                    <Show when={hasOsInfo() || ipAddresses().length > 0}>
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Guest Info</div>
                            <div class="space-y-2">
                                <Show when={hasOsInfo()}>
                                    <div class="text-[11px] text-gray-600 dark:text-gray-300">
                                        <Show when={osName().length > 0}>
                                            <span class="font-medium">{osName()}</span>
                                        </Show>
                                        <Show when={osName().length > 0 && osVersion().length > 0}>
                                            <span class="text-gray-400 dark:text-gray-500 mx-1">•</span>
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
                                                <span class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-200" title={ip}>
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
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Memory</div>
                            <div class="space-y-1 text-[11px] text-gray-600 dark:text-gray-300">
                                <For each={memoryExtraLines()!}>{(line) => <div>{line}</div>}</For>
                            </div>
                        </div>
                    </Show>

                    {/* Backup Info */}
                    <Show when={props.guest.lastBackup}>
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Backup</div>
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
                                                <span class="text-gray-500 dark:text-gray-400">Last Backup</span>
                                                <span class={`font-medium ${isCritical ? 'text-red-600 dark:text-red-400' : isOld ? 'text-amber-600 dark:text-amber-400' : 'text-green-600 dark:text-green-400'}`}>
                                                    {daysSince === 0 ? 'Today' : daysSince === 1 ? 'Yesterday' : `${daysSince}d ago`}
                                                </span>
                                            </div>
                                            <div class="text-[10px] text-gray-400 dark:text-gray-500">
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
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Tags</div>
                            <div class="flex flex-wrap gap-1">
                                <For each={Array.isArray(props.guest.tags) ? props.guest.tags : (props.guest.tags?.split(',') || [])}>
                                    {(tag) => (
                                        <span class="inline-block rounded bg-gray-100 px-1.5 py-0.5 text-[10px] text-gray-700 dark:bg-gray-700 dark:text-gray-200">
                                            {tag.trim()}
                                        </span>
                                    )}
                                </For>
                            </div>
                        </div>
                    </Show>

                    {/* Filesystems */}
                    <Show when={hasFilesystemDetails() && props.guest.disks && props.guest.disks.length > 0}>
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Filesystems</div>
                            <div class="text-[11px] text-gray-600 dark:text-gray-300">
                                <DiskList
                                    disks={props.guest.disks || []}
                                    diskStatusReason={isVM(props.guest) ? props.guest.diskStatusReason : undefined}
                                />
                            </div>
                        </div>
                    </Show>

                    {/* Network Interfaces */}
                    <Show when={hasNetworkInterfaces()}>
                        <div class="rounded border border-gray-200 bg-white/70 p-3 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                            <div class="text-[11px] font-medium uppercase tracking-wide text-gray-700 dark:text-gray-200 mb-2">Network</div>
                            <div class="space-y-2">
                                <For each={networkInterfaces().slice(0, 4)}>
                                    {(iface) => {
                                        const addresses = iface.addresses ?? [];
                                        const hasTraffic = (iface.rxBytes ?? 0) > 0 || (iface.txBytes ?? 0) > 0;
                                        return (
                                            <div class="rounded border border-dashed border-gray-200 p-2 dark:border-gray-700">
                                                <div class="flex items-center gap-2 text-[11px] font-medium text-gray-700 dark:text-gray-200">
                                                    <span class="truncate">{iface.name || 'interface'}</span>
                                                    <Show when={iface.mac}>
                                                        <span class="text-[9px] text-gray-400 dark:text-gray-500 font-normal">{iface.mac}</span>
                                                    </Show>
                                                </div>
                                                <Show when={addresses.length > 0}>
                                                    <div class="flex flex-wrap gap-1 mt-1">
                                                        <For each={addresses}>
                                                            {(ip) => (
                                                                <span class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                                                                    {ip}
                                                                </span>
                                                            )}
                                                        </For>
                                                    </div>
                                                </Show>
                                                <Show when={hasTraffic}>
                                                    <div class="flex gap-3 mt-1 text-[10px] text-gray-500 dark:text-gray-400">
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
            </Show>

            <Show when={activeTab() === 'history'}>
                <div class="space-y-6">
                    {/* Toolbar: Range and View Toggle */}
                    <div class="flex flex-col gap-4 bg-gray-50 dark:bg-gray-800/50 p-3 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
                        <div class="flex items-center justify-between">
                            <span class="text-xs font-bold text-gray-400 uppercase tracking-widest">Controls</span>
                            <div class="flex bg-gray-100 dark:bg-gray-700 rounded-lg p-0.5">
                                <button
                                    onClick={() => setViewMode('unified')}
                                    class={`px-3 py-1 text-[10px] font-bold rounded-md transition-all ${viewMode() === 'unified'
                                        ? 'bg-white dark:bg-gray-600 text-blue-600 dark:text-blue-400 shadow-sm'
                                        : 'text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                                        }`}
                                >
                                    Unified
                                </button>
                                <button
                                    onClick={() => setViewMode('split')}
                                    class={`px-3 py-1 text-[10px] font-bold rounded-md transition-all ${viewMode() === 'split'
                                        ? 'bg-white dark:bg-gray-600 text-blue-600 dark:text-blue-400 shadow-sm'
                                        : 'text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                                        }`}
                                >
                                    Split
                                </button>
                            </div>
                        </div>

                        <div class="flex items-center justify-between pt-3 border-t border-gray-100 dark:border-gray-700/30">
                            <span class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">Range</span>
                            <div class="flex bg-gray-100 dark:bg-gray-700 rounded-lg p-0.5">
                                {(['24h', '7d', '30d', '90d'] as HistoryTimeRange[]).map(r => (
                                    <button
                                        onClick={() => setHistoryRange(r)}
                                        class={`px-4 py-1.5 text-xs font-medium rounded-md transition-all ${historyRange() === r
                                            ? 'bg-white dark:bg-gray-600 text-blue-600 dark:text-blue-400 shadow-sm'
                                            : 'text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
                                            }`}
                                    >
                                        {r}
                                    </button>
                                ))}
                            </div>
                        </div>
                    </div>

                    <Show when={viewMode() === 'unified'}>
                        <UnifiedHistoryChart
                            resourceType={metricsResource().type}
                            resourceId={metricsResource().id}
                            label="Resource Performance"
                            height={280}
                            range={historyRange()}
                            hideSelector={true}
                        />
                    </Show>

                    <Show when={viewMode() === 'split'}>
                        <div class="grid grid-cols-1 gap-4">
                            <HistoryChart
                                resourceType={metricsResource().type}
                                resourceId={metricsResource().id}
                                metric="cpu"
                                label="CPU Usage"
                                unit="%"
                                height={160}
                                range={historyRange()}
                                hideSelector={true}
                            />
                            <HistoryChart
                                resourceType={metricsResource().type}
                                resourceId={metricsResource().id}
                                metric="memory"
                                label="Memory Usage"
                                unit="%"
                                height={160}
                                range={historyRange()}
                                hideSelector={true}
                            />
                            <HistoryChart
                                resourceType={metricsResource().type}
                                resourceId={metricsResource().id}
                                metric="disk"
                                label="Disk Usage"
                                unit="%"
                                height={160}
                                range={historyRange()}
                                hideSelector={true}
                            />
                        </div>
                    </Show>
                </div>
            </Show>
        </div>
    );
};
