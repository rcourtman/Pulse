import { Show, For, createSignal, createEffect } from 'solid-js';
import { Dialog } from '../shared/Dialog';
import { ThresholdSlider } from '../Dashboard/ThresholdSlider';

export interface BulkEditDialogProps {
    isOpen: boolean;
    onClose: () => void;
    onSave: (thresholds: Record<string, number | undefined>) => void;
    selectedIds: string[];
    columns: string[];
}

export function normalizeMetricKey(column: string): string {
    const key = column.trim().toLowerCase();
    const mapped = new Map<string, string>([
        ['cpu %', 'cpu'],
        ['memory %', 'memory'],
        ['disk %', 'disk'],
        ['disk r mb/s', 'diskRead'],
        ['disk w mb/s', 'diskWrite'],
        ['net in mb/s', 'networkIn'],
        ['net out mb/s', 'networkOut'],
        ['usage %', 'usage'],
        ['temp °c', 'temperature'],
        ['temperature °c', 'temperature'],
        ['temperature', 'temperature'],
        ['restart count', 'restartCount'],
        ['restart window', 'restartWindow'],
        ['restart window (s)', 'restartWindow'],
        ['memory warn %', 'memoryWarnPct'],
        ['memory critical %', 'memoryCriticalPct'],
        ['warning size (gib)', 'warningSizeGiB'],
        ['critical size (gib)', 'criticalSizeGiB'],
        ['disk temp °c', 'diskTemperature'],
        ['backup', 'backup'],
        ['snapshot', 'snapshot'],
    ]).get(key);
    if (mapped) {
        return mapped;
    }
    return key.replace(/[^a-zA-Z0-9]/g, '');
}

export function BulkEditDialog(props: BulkEditDialogProps) {
    const [thresholds, setThresholds] = createSignal<Record<string, number | undefined>>({});

    createEffect(() => {
        if (props.isOpen) {
            setThresholds({});
        }
    });

    const handleSave = () => {
        props.onSave(thresholds());
    };

    const getMetricBounds = (metric: string) => {
        switch (metric) {
            case 'cpu':
            case 'memory':
            case 'disk':
            case 'memoryWarnPct':
            case 'memoryCriticalPct':
            case 'usage':
                return { min: 0, max: 100, step: 1 };
            case 'temperature':
            case 'diskTemperature':
                return { min: 20, max: 120, step: 1 };
            case 'restartCount':
                return { min: 0, max: 100, step: 1 };
            case 'restartWindow':
                return { min: 0, max: 3600, step: 60 };
            default:
                return { min: 0, max: 1000, step: 1 };
        }
    };

    return (
        <Dialog isOpen={props.isOpen} onClose={props.onClose} ariaLabel="Bulk Edit Settings">
            <div class="fixed inset-0 min-h-screen z-[100] flex items-center justify-center pointer-events-none">
                <div class="bg-surface rounded-xl shadow-2xl ring-1 ring-border max-w-lg w-full p-6 max-h-[90vh] flex flex-col pointer-events-auto">
                    <h2 class="text-xl font-semibold text-base-content mb-2">Bulk Edit Settings</h2>
                    <p class="text-sm text-muted mb-6">
                        Applying changes to {props.selectedIds.length} items. Leave fields empty to keep existing options.
                    </p>

                    <div class="space-y-6 overflow-y-auto px-1 flex-1 min-h-0">
                        <For each={props.columns}>
                            {(column) => {
                                const metric = normalizeMetricKey(column);
                                if (metric === 'backup' || metric === 'snapshot') return null;
                                const bounds = getMetricBounds(metric);
                                const val = () => thresholds()[metric];

                                return (
                                    <div class="space-y-2 pb-4 border-b border-border-subtle last:border-0">
                                        <div class="flex items-center justify-between mb-2">
                                            <label class="text-sm font-medium text-base-content">
                                                {column}
                                            </label>
                                            <span class="text-xs text-slate-500 font-mono">
                                                {val() !== undefined ? val() : 'Unchanged'}
                                            </span>
                                        </div>
                                        <div class="flex items-center justify-between gap-4">
                                            <div class="flex-1">
                                                {['cpu', 'memory', 'disk', 'temperature'].includes(metric) ? (
                                                    <div class="pt-2 px-1">
                                                        <ThresholdSlider
                                                            type={metric as 'cpu' | 'memory' | 'disk' | 'temperature'}
                                                            min={bounds.min}
                                                            max={bounds.max}
                                                            value={val() !== undefined ? val()! : bounds.min}
                                                            onChange={(v) => {
                                                                setThresholds((prev) => ({ ...prev, [metric]: v }));
                                                            }}
                                                        />
                                                    </div>
                                                ) : (
                                                    <input
                                                        type="number"
                                                        class="w-full h-9 rounded-md border border-border bg-surface px-3 py-1 text-sm shadow-sm transition-colors focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500 dark:text-slate-50"
                                                        min={bounds.min}
                                                        max={bounds.max}
                                                        step={bounds.step}
                                                        value={val() ?? ''}
                                                        placeholder="Unchanged"
                                                        onInput={(e) => {
                                                            const v = parseFloat(e.currentTarget.value);
                                                            setThresholds((prev) => ({ ...prev, [metric]: isNaN(v) ? undefined : v }));
                                                        }}
                                                    />
                                                )}
                                            </div>
                                            <div class="w-12 text-right">
                                                <Show when={val() !== undefined}>
                                                    <button
                                                        type="button"
                                                        class="text-xs text-slate-500 hover:text-red-500 transition-colors"
                                                        onClick={() => setThresholds((prev) => ({ ...prev, [metric]: undefined }))}
                                                    >
                                                        Clear
                                                    </button>
                                                </Show>
                                            </div>
                                        </div>
                                    </div>
                                );
                            }}
                        </For>
                    </div>

                    <div class="mt-4 flex justify-end gap-3 pt-4 border-t border-border shrink-0">
                        <button
                            type="button"
                            class="px-5 py-2 text-sm font-medium text-base-content bg-surface border border-border hover:bg-surface-hover rounded-md transition-colors shadow-sm"
                            onClick={props.onClose}
                        >
                            Cancel
                        </button>
                        <button
                            type="button"
                            class="px-5 py-2 text-sm font-medium text-white bg-sky-600 hover:bg-sky-500 rounded-md shadow-sm transition-colors"
                            onClick={handleSave}
                        >
                            Apply to {props.selectedIds.length} items
                        </button>
                    </div>
                </div>
            </div>
        </Dialog>
    );
}
