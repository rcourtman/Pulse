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
        if (state === 'ONLINE') return 'bg-green-500/80 dark:bg-green-500/70 hover:bg-green-400';
        if (state === 'DEGRADED') return 'bg-yellow-500/80 dark:bg-yellow-500/70 hover:bg-yellow-400';
        if (state === 'FAULTED' || state === 'UNAVAIL' || state === 'OFFLINE') return 'bg-red-500/80 dark:bg-red-500/70 hover:bg-red-400';
        return 'bg-gray-400/80 dark:bg-gray-500/70 hover:bg-gray-300';
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
                        <div class="bg-gray-900 dark:bg-gray-800 text-white text-[10px] rounded-md shadow-lg px-2 py-1.5 min-w-[120px] border border-gray-700">
                            <div class="font-medium mb-0.5 text-gray-200">
                                {hoveredDevice()?.name}
                            </div>
                            <div class="text-gray-400 mb-1">
                                {hoveredDevice()?.type}
                            </div>
                            <div class="flex items-center gap-2 border-t border-gray-700/50 pt-1">
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
                                <div class="text-gray-400 mt-1 italic max-w-[200px] break-words">
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
