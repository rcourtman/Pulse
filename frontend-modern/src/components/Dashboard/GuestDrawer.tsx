import { Component, Show, For, createSignal, createEffect } from 'solid-js';
import { VM, Container } from '@/types/api';
import { formatBytes, formatPercent, formatSpeed, formatUptime } from '@/utils/format';
import { DiskList } from './DiskList';
import { aiChatStore } from '@/stores/aiChat';
import { AIAPI } from '@/api/ai';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import { logger } from '@/utils/logger';

type Guest = VM | Container;

interface GuestDrawerProps {
    guest: Guest;
    metricsKey: string;
    onClose: () => void;
}

export const GuestDrawer: Component<GuestDrawerProps> = (props) => {
    const [aiEnabled, setAiEnabled] = createSignal(false);
    const [annotations, setAnnotations] = createSignal<string[]>([]);
    const [newAnnotation, setNewAnnotation] = createSignal('');
    const [saving, setSaving] = createSignal(false);

    // Build guest ID for metadata
    const guestId = () => props.guest.id || `${props.guest.instance}-${props.guest.vmid}`;

    // Check if AI is enabled and load annotations on mount
    createEffect(() => {
        AIAPI.getSettings()
            .then((settings) => setAiEnabled(settings.enabled && settings.configured))
            .catch((err) => logger.debug('[GuestDrawer] AI settings check failed:', err));

        // Load existing annotations
        GuestMetadataAPI.getMetadata(guestId())
            .then((meta) => {
                if (meta.notes && Array.isArray(meta.notes)) setAnnotations(meta.notes);
            })
            .catch((err) => logger.debug('[GuestDrawer] Failed to load annotations:', err));
    });

    // Update AI context whenever guest changes or annotations are loaded
    createEffect(() => {
        const guestType = props.guest.type === 'qemu' ? 'vm' : 'container';
        // Track annotations to re-run when they change
        void annotations();
        aiChatStore.setTargetContext(guestType, guestId(), {
            guestName: props.guest.name,
            ...buildGuestContext(),
        });
    });

    // Note: We no longer clear context on unmount - context accumulates across navigation
    // Users can clear individual items or all context from the AI panel

    const saveAnnotations = async (updated: string[]) => {
        setSaving(true);
        try {
            await GuestMetadataAPI.updateMetadata(guestId(), { notes: updated });
            logger.debug('[GuestDrawer] Annotations saved');
        } catch (err) {
            logger.error('[GuestDrawer] Failed to save annotations:', err);
        } finally {
            setSaving(false);
        }
    };

    const addAnnotation = () => {
        const text = newAnnotation().trim();
        if (!text) return;
        const updated = [...annotations(), text];
        setAnnotations(updated);
        setNewAnnotation('');
        saveAnnotations(updated);
    };

    const removeAnnotation = (index: number) => {
        const updated = annotations().filter((_, i) => i !== index);
        setAnnotations(updated);
        saveAnnotations(updated);
    };

    const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            addAnnotation();
        }
    };

    const isVM = (guest: Guest): guest is VM => {
        return guest.type === 'qemu';
    };

    // Build comprehensive context for AI
    const buildGuestContext = () => {
        const guest = props.guest;
        const context: Record<string, unknown> = {
            name: guest.name,
            type: guest.type === 'qemu' ? 'Virtual Machine' : 'LXC Container',
            vmid: guest.vmid,
            node: guest.node,  // PVE node this guest lives on
            guest_node: guest.node,  // For backend agent routing
            status: guest.status,
            uptime: guest.uptime ? formatUptime(guest.uptime) : 'Not running',
        };

        // CPU info
        if (guest.cpu !== undefined) {
            context.cpu_usage = formatPercent(guest.cpu * 100);
        }
        if (guest.cpus) {
            context.cpu_cores = guest.cpus;
        }

        // Memory info
        if (guest.memory) {
            context.memory_used = formatBytes(guest.memory.used || 0);
            context.memory_total = formatBytes(guest.memory.total || 0);
            context.memory_usage = formatPercent(guest.memory.usage || 0);
            if (guest.memory.balloon && guest.memory.balloon !== guest.memory.total) {
                context.memory_balloon = formatBytes(guest.memory.balloon);
            }
            if (guest.memory.swapTotal && guest.memory.swapTotal > 0) {
                context.swap_used = formatBytes(guest.memory.swapUsed || 0);
                context.swap_total = formatBytes(guest.memory.swapTotal);
            }
        }

        // Disk info
        if (guest.disk && guest.disk.total > 0) {
            context.disk_used = formatBytes(guest.disk.used || 0);
            context.disk_total = formatBytes(guest.disk.total || 0);
            context.disk_usage = formatPercent((guest.disk.used / guest.disk.total) * 100);
        }

        // I/O rates
        if (guest.diskRead !== undefined) {
            context.disk_read_rate = formatSpeed(guest.diskRead);
        }
        if (guest.diskWrite !== undefined) {
            context.disk_write_rate = formatSpeed(guest.diskWrite);
        }
        if (guest.networkIn !== undefined) {
            context.network_in_rate = formatSpeed(guest.networkIn);
        }
        if (guest.networkOut !== undefined) {
            context.network_out_rate = formatSpeed(guest.networkOut);
        }

        // OS info (both VMs and containers can have this)
        if (guest.osName) context.os_name = guest.osName;
        if (guest.osVersion) context.os_version = guest.osVersion;
        if (guest.agentVersion) context.guest_agent = guest.agentVersion;
        if (guest.ipAddresses?.length) context.ip_addresses = guest.ipAddresses;

        // Tags
        if (guest.tags?.length) {
            context.tags = guest.tags;
        }

        // Backup info - Pulse already has this from PBS
        if (guest.lastBackup) {
            const backupDate = new Date(guest.lastBackup);
            const now = new Date();
            const daysSinceBackup = Math.floor((now.getTime() - backupDate.getTime()) / (1000 * 60 * 60 * 24));
            context.last_backup = backupDate.toISOString();
            context.days_since_backup = daysSinceBackup;
            if (daysSinceBackup > 30) {
                context.backup_status = 'CRITICAL - No backup in over 30 days';
            } else if (daysSinceBackup > 7) {
                context.backup_status = 'Warning - Backup is over a week old';
            } else {
                context.backup_status = 'OK';
            }
        } else {
            context.backup_status = 'NEVER - No backup recorded';
        }

        // User annotations (for AI context)
        if (annotations().length > 0) {
            context.user_annotations = annotations();
        }

        return context;
    };

    const handleAskAI = () => {
        const guestType = props.guest.type === 'qemu' ? 'vm' : 'container';
        const guestId = props.guest.id || `${props.guest.instance}-${props.guest.vmid}`;

        aiChatStore.openForTarget(guestType, guestId, {
            guestName: props.guest.name,
            ...buildGuestContext(),
        });
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

    return (
        <div class="space-y-3">
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

            {/* Bottom row: AI Context & Ask AI - only show when AI is enabled */}
            <Show when={aiEnabled()}>
                <div class="mt-3 pt-3 border-t border-gray-200 dark:border-gray-700 space-y-2">
                    <div class="flex items-center gap-1.5">
                        <span class="text-[10px] font-medium text-gray-500 dark:text-gray-400">AI Context</span>
                        <Show when={saving()}>
                            <span class="text-[9px] text-gray-400">saving...</span>
                        </Show>
                    </div>

                    {/* Existing annotations */}
                    <Show when={annotations().length > 0}>
                        <div class="flex flex-wrap gap-1.5">
                            <For each={annotations()}>
                                {(annotation, index) => (
                                    <span class="inline-flex items-center gap-1 px-2 py-1 text-[11px] rounded-md bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-200">
                                        <span class="max-w-[300px] truncate">{annotation}</span>
                                        <button
                                            type="button"
                                            onClick={() => removeAnnotation(index())}
                                            class="ml-0.5 p-0.5 rounded hover:bg-purple-200 dark:hover:bg-purple-800 transition-colors"
                                            title="Remove"
                                        >
                                            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                                            </svg>
                                        </button>
                                    </span>
                                )}
                            </For>
                        </div>
                    </Show>

                    {/* Add new annotation + Ask AI */}
                    <div class="flex items-center gap-2">
                        <input
                            type="text"
                            value={newAnnotation()}
                            onInput={(e) => setNewAnnotation(e.currentTarget.value)}
                            onKeyDown={handleKeyDown}
                            placeholder="Add context for AI (press Enter)..."
                            class="flex-1 px-2 py-1.5 text-[11px] rounded border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-purple-500 focus:border-purple-500"
                        />
                        <button
                            type="button"
                            onClick={addAnnotation}
                            disabled={!newAnnotation().trim()}
                            class="px-2 py-1.5 text-[11px] rounded border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                        >
                            Add
                        </button>
                        <button
                            type="button"
                            onClick={handleAskAI}
                            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-gradient-to-r from-purple-500 to-pink-500 text-white text-[11px] font-medium shadow-sm hover:from-purple-600 hover:to-pink-600 transition-all"
                            title={`Ask AI about ${props.guest.name}`}
                        >
                            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611l-2.576.43a18.003 18.003 0 01-5.118 0l-2.576-.43c-1.717-.293-2.299-2.379-1.067-3.611L5 14.5" />
                            </svg>
                            Ask AI
                        </button>
                    </div>
                </div>
            </Show>
        </div>
    );
};
