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

        // OS info (VMs only)
        if (isVM(guest)) {
            if (guest.osName) context.os_name = guest.osName;
            if (guest.osVersion) context.os_version = guest.osVersion;
            if (guest.agentVersion) context.guest_agent = guest.agentVersion;
            if (guest.ipAddresses?.length) context.ip_addresses = guest.ipAddresses;
        }

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
        if (!isVM(props.guest)) return false;
        return (props.guest.osName?.length ?? 0) > 0 || (props.guest.osVersion?.length ?? 0) > 0;
    };

    const osName = () => {
        if (!isVM(props.guest)) return '';
        return props.guest.osName || '';
    };

    const osVersion = () => {
        if (!isVM(props.guest)) return '';
        return props.guest.osVersion || '';
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
        <div class="space-y-3">
            {/* Top row: metrics columns */}
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
