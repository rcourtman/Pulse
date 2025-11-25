import { Component, Show, For } from 'solid-js';
import { VM, Container } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { DiskList } from './DiskList';
import { IOMetric } from './IOMetric';

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
        if (!isVM(props.guest)) return false;
        return (props.guest.osInfo?.name?.length ?? 0) > 0 || (props.guest.osInfo?.version?.length ?? 0) > 0;
    };

    const osName = () => {
        if (!isVM(props.guest)) return '';
        return props.guest.osInfo?.name || '';
    };

    const osVersion = () => {
        if (!isVM(props.guest)) return '';
        return props.guest.osInfo?.version || '';
    };

    const hasAgentInfo = () => {
        if (!isVM(props.guest)) return false;
        return !!props.guest.agentVersion;
    };

    const agentVersion = () => {
        if (!isVM(props.guest)) return '';
        return props.guest.agentVersion || '';
    };

    const ipAddresses = () => {
        if (!isVM(props.guest)) return [];
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
        if (!isVM(props.guest)) return [];
        return props.guest.networkInterfaces || [];
    };

    const hasNetworkInterfaces = () => {
        return networkInterfaces().length > 0;
    };

    return (
        <div class="flex items-start gap-4">
            {/* Left Column: Guest Overview */}
            <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Guest Overview</div>
                <div class="mt-1 space-y-1">
                    <Show when={hasOsInfo()}>
                        <div class="flex flex-wrap items-center gap-1 text-gray-600 dark:text-gray-300">
                            <Show when={osName().length > 0}>
                                <span class="font-medium" title={osName()}>{osName()}</span>
                            </Show>
                            <Show when={osName().length > 0 && osVersion().length > 0}>
                                <span class="text-gray-400 dark:text-gray-500">•</span>
                            </Show>
                            <Show when={osVersion().length > 0}>
                                <span title={osVersion()}>{osVersion()}</span>
                            </Show>
                        </div>
                    </Show>
                    <Show when={hasAgentInfo()}>
                        <div class="flex flex-wrap items-center gap-1 text-[11px] text-gray-500 dark:text-gray-400">
                            <span class="uppercase tracking-wide text-[10px] text-gray-400 dark:text-gray-500">
                                Agent
                            </span>
                            <span title={`QEMU guest agent ${agentVersion()}`}>
                                QEMU guest agent {agentVersion()}
                            </span>
                        </div>
                    </Show>
                    <Show when={ipAddresses().length > 0}>
                        <div class="flex flex-wrap gap-1">
                            <For each={ipAddresses()}>
                                {(ip) => (
                                    <span
                                        class="max-w-full truncate rounded bg-blue-100 px-1.5 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
                                        title={ip}
                                    >
                                        {ip}
                                    </span>
                                )}
                            </For>
                        </div>
                    </Show>
                </div>
            </div>

            {/* Middle Column: Memory Details */}
            <Show when={memoryExtraLines() && memoryExtraLines()!.length > 0}>
                <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                    <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Memory</div>
                    <div class="mt-1 space-y-1 text-gray-600 dark:text-gray-300">
                        <For each={memoryExtraLines()!}>{(line) => <div>{line}</div>}</For>
                    </div>
                </div>
            </Show>

            {/* Right Column: Filesystems */}
            <Show when={hasFilesystemDetails() && props.guest.disks && props.guest.disks.length > 0}>
                <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                    <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Filesystems</div>
                    <div class="mt-1 text-gray-600 dark:text-gray-300">
                        <DiskList
                            disks={props.guest.disks || []}
                            diskStatusReason={isVM(props.guest) ? props.guest.diskStatusReason : undefined}
                        />
                    </div>
                </div>
            </Show>

            {/* Far Right Column: Network Interfaces */}
            <Show when={hasNetworkInterfaces()}>
                <div class="min-w-[220px] flex-1 rounded border border-gray-200 bg-white/70 p-2 shadow-sm dark:border-gray-600/70 dark:bg-gray-900/30">
                    <div class="text-[11px] font-medium text-gray-700 dark:text-gray-200">Network Interfaces</div>
                    <div class="mt-1 text-[10px] text-gray-400 dark:text-gray-500">Row charts show current rate; totals below are cumulative since boot.</div>
                    <div class="mt-1 space-y-1 text-gray-600 dark:text-gray-300">
                        <For each={networkInterfaces()}>
                            {(iface) => {
                                const addresses = iface.addresses ?? [];
                                const hasTraffic = (iface.rxBytes ?? 0) > 0 || (iface.txBytes ?? 0) > 0;
                                return (
                                    <div class="space-y-1 rounded border border-dashed border-gray-200 p-2 last:mb-0 dark:border-gray-700">
                                        <div class="flex items-center gap-2 font-medium text-gray-700 dark:text-gray-200">
                                            <span class="truncate" title={iface.name}>{iface.name || 'interface'}</span>
                                            <Show when={iface.mac}>
                                                <span class="text-[10px] text-gray-400 dark:text-gray-500" title={iface.mac}>
                                                    {iface.mac}
                                                </span>
                                            </Show>
                                        </div>
                                        <Show when={addresses.length > 0}>
                                            <div class="flex flex-wrap gap-1">
                                                <For each={addresses}>
                                                    {(ip) => (
                                                        <span
                                                            class="max-w-full truncate rounded bg-blue-100 px-1.5 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
                                                            title={ip}
                                                        >
                                                            {ip}
                                                        </span>
                                                    )}
                                                </For>
                                            </div>
                                        </Show>
                                        <Show when={hasTraffic}>
                                            <div class="flex items-center gap-3 text-[10px] text-gray-500 dark:text-gray-400">
                                                <span>Total RX {formatBytes(iface.rxBytes ?? 0)}</span>
                                                <span>Total TX {formatBytes(iface.txBytes ?? 0)}</span>
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
    );
};
