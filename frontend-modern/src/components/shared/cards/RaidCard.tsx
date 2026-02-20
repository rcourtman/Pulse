import { Component, For, Show } from 'solid-js';
import type { HostRAIDArray, HostRAIDDevice } from '@/types/api';
import { StatusDot } from '@/components/shared/StatusDot';

interface RaidCardProps {
  arrays?: HostRAIDArray[];
  title?: string;
}

const normalize = (value?: string): string => (value || '').trim().toLowerCase();

const raidStateVariant = (state?: string) => {
  const s = normalize(state);
  if (s === 'active' || s === 'clean') return 'success';
  if (s.includes('fail') || s.includes('inactive') || s.includes('offline') || s.includes('stopped')) return 'danger';
  return 'warning';
};

const raidStateTextClass = (state?: string) => {
  const variant = raidStateVariant(state);
  if (variant === 'success') return 'text-emerald-600 dark:text-emerald-400';
  if (variant === 'danger') return 'text-red-600 dark:text-red-400';
  return 'text-amber-600 dark:text-amber-400';
};

const deviceToneClass = (device: HostRAIDDevice) => {
  const s = normalize(device.state);
  if (s === 'active' || s === 'in_sync' || s === 'online') {
    return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-900 dark:text-emerald-200 dark:border-emerald-800';
  }
  if (s.includes('fail') || s.includes('fault') || s.includes('offline') || s.includes('removed')) {
    return 'bg-red-50 text-red-700 border-red-200 dark:bg-red-900 dark:text-red-200 dark:border-red-800';
  }
  return 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-900 dark:text-amber-200 dark:border-amber-800';
};

export const RaidCard: Component<RaidCardProps> = (props) => {
  if (!props.arrays || props.arrays.length === 0) return null;

  return (
    <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
      <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">
        {props.title || 'RAID'}
      </div>

      <div class="max-h-[160px] overflow-y-auto custom-scrollbar space-y-2">
        <For each={props.arrays}>
          {(array) => {
            const label = () => (array.name || '').trim() || (array.device || '').trim() || 'RAID array';
            const variant = () => raidStateVariant(array.state);
            const stateText = () => (array.state || '').trim() || 'unknown';
            const levelText = () => (array.level || '').trim() || 'unknown';
            const rebuilding = () => Number.isFinite(array.rebuildPercent) && array.rebuildPercent > 0 && array.rebuildPercent < 100;

            return (
              <div class="rounded border border-dashed border-slate-200 p-2 dark:border-slate-700 overflow-hidden">
                <div class="flex items-start justify-between gap-2 min-w-0">
                  <div class="min-w-0">
                    <div class="text-[11px] font-semibold text-slate-700 dark:text-slate-200 truncate" title={label()}>
                      {label()}
                    </div>
                    <div class="mt-0.5 text-[10px] text-slate-500 dark:text-slate-400 truncate" title={levelText()}>
                      {levelText()}
                    </div>
                  </div>

                  <div class="flex items-center gap-1.5 shrink-0" title={stateText()}>
                    <StatusDot variant={variant()} size="xs" ariaLabel={`RAID state: ${stateText()}`} />
                    <span class={`text-[10px] font-medium ${raidStateTextClass(array.state)}`}>
                      {stateText()}
                    </span>
                  </div>
                </div>

                <Show when={rebuilding()}>
                  <div class="mt-2 text-[10px] text-slate-500 dark:text-slate-400">
                    Rebuild: <span class="font-medium text-slate-700 dark:text-slate-200">{Math.round(array.rebuildPercent)}%</span>
                    <Show when={array.rebuildSpeed}>
                      <span class="text-slate-400 dark:text-slate-500"> · </span>
                      <span class="font-medium text-slate-700 dark:text-slate-200">{array.rebuildSpeed}</span>
                    </Show>
                  </div>
                </Show>

                <Show when={array.devices && array.devices.length > 0}>
                  <div class="mt-2 flex flex-wrap gap-1">
                    <For each={array.devices}>
                      {(device) => (
                        <span
                          class={`inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-medium ${deviceToneClass(device)}`}
                          title={`slot ${device.slot} • ${device.state}`}
                        >
                          {device.device}
                        </span>
                      )}
                    </For>
                  </div>
                </Show>
              </div>
            );
          }}
        </For>
      </div>
    </div>
  );
};

export default RaidCard;

