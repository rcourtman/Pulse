import { Component, For, Show, createSignal } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { ZFSPool, ZFSDevice } from '@/types/api';

interface ZFSHealthMapProps {
    pool: ZFSPool;
}

export const ZFSHealthMap: Component<ZFSHealthMapProps> = (props) => {
    const [hoveredDevice, setHoveredDevice] = createSignal<ZFSDevice | null>(null);
    const [tooltipPos, setTooltipPos] = createSignal({ x: 0, y: 0 });

    // Filter out non-disk devices if needed, or just show all top-level vdevs/disks
    // Usually 'disk', 'mirror', 'raidz*' are top level.
    // We want to visualize the health of the underlying storage units.
    // If the API returns a flat list of all devices including children, we might want to filter.
    // Assuming 'devices' contains the relevant units to display.
    const devices = () => props.pool.devices || [];

    const getDeviceColor = (device: ZFSDevice) => {
        const state = device.state?.toUpperCase();
        if (state === 'ONLINE') return 'bg-green-500 dark:bg-green-500 hover:bg-green-400';
        if (state === 'DEGRADED') return 'bg-yellow-500 dark:bg-yellow-500 hover:bg-yellow-400';
        if (state === 'FAULTED' || state === 'UNAVAIL' || state === 'OFFLINE') return 'bg-red-500 dark:bg-red-500 hover:bg-red-400';
        return 'bg-slate-400 dark:bg-slate-800 hover:bg-slate-300';
    };

    const isResilvering = (device: ZFSDevice) => {
        // Check pool scan status or device message/state for resilvering
        const scan = props.pool.scan?.toLowerCase() || '';
        return scan.includes('resilver') || (device.message || '').toLowerCase().includes('resilver');
    };

    const handleMouseEnter = (e: MouseEvent, device: ZFSDevice) => {
        const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
        setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
        setHoveredDevice(device);
    };

    const handleMouseLeave = () => {
        setHoveredDevice(null);
    };

    return (
        <div class="flex items-center gap-0.5">
            <For each={devices()}>
                {(device) => (
                    <div
                        class={`w-2.5 h-3 rounded-sm transition-colors duration-200 ${getDeviceColor(device)} ${isResilvering(device) ? 'animate-pulse' : ''}`}
                        onMouseEnter={(e) => handleMouseEnter(e, device)}
                        onMouseLeave={handleMouseLeave}
                    />
                )}
            </For>

            <Show when={hoveredDevice()}>
                <Portal mount={document.body}>
                    <div
                        class="fixed z-[9999] pointer-events-none"
                        style={{
                            left: `${tooltipPos().x}px`,
                            top: `${tooltipPos().y - 8}px`,
                            transform: 'translate(-50%, -100%)',
                        }}
                    >
                        <div class="bg-base text-white text-[10px] rounded-md shadow-sm px-2 py-1.5 min-w-[120px] border border-border">
                            <div class="font-medium mb-0.5 text-base-content">
                                {hoveredDevice()?.name}
                            </div>
                            <div class="text-slate-400 mb-1">
                                {hoveredDevice()?.type}
                            </div>
                            <div class="flex items-center gap-2 border-t border-border pt-1">
                                <span class={`font-semibold ${hoveredDevice()?.state === 'ONLINE' ? 'text-green-400' :
                                    hoveredDevice()?.state === 'DEGRADED' ? 'text-yellow-400' :
                                        'text-red-400'
                                    }`}>
                                    {hoveredDevice()?.state}
                                </span>
                                <Show when={hoveredDevice()?.readErrors || hoveredDevice()?.writeErrors || hoveredDevice()?.checksumErrors}>
                                    <span class="text-red-400">
                                        (E: {hoveredDevice()?.readErrors}/{hoveredDevice()?.writeErrors}/{hoveredDevice()?.checksumErrors})
                                    </span>
                                </Show>
                            </div>
                            <Show when={hoveredDevice()?.message}>
                                <div class="text-slate-400 mt-1 italic max-w-[200px] break-words">
                                    {hoveredDevice()?.message}
                                </div>
                            </Show>
                        </div>
                    </div>
                </Portal>
            </Show>
        </div>
    );
};
