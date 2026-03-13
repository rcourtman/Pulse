import { Component, For, Show } from 'solid-js';
import type { HostRAIDArray } from '@/types/api';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  getRaidDeviceBadgeClass,
  getRaidStateTextClass,
  getRaidStateVariant,
} from '@/utils/raidPresentation';

interface RaidCardProps {
  arrays?: HostRAIDArray[];
  title?: string;
}

export const RaidCard: Component<RaidCardProps> = (props) => {
  if (!props.arrays || props.arrays.length === 0) return null;

  return (
    <div class="rounded border border-border bg-surface p-3 shadow-sm">
      <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
        {props.title || 'RAID'}
      </div>

      <div class="max-h-[160px] overflow-y-auto custom-scrollbar space-y-2">
        <For each={props.arrays}>
          {(array) => {
            const label = () =>
              (array.name || '').trim() || (array.device || '').trim() || 'RAID array';
            const variant = () => getRaidStateVariant(array.state);
            const stateText = () => (array.state || '').trim() || 'unknown';
            const levelText = () => (array.level || '').trim() || 'unknown';
            const rebuilding = () =>
              Number.isFinite(array.rebuildPercent) &&
              array.rebuildPercent > 0 &&
              array.rebuildPercent < 100;

            return (
              <div class="rounded border border-dashed border-border p-2 overflow-hidden">
                <div class="flex items-start justify-between gap-2 min-w-0">
                  <div class="min-w-0">
                    <div
                      class="text-[11px] font-semibold text-base-content truncate"
                      title={label()}
                    >
                      {label()}
                    </div>
                    <div class="mt-0.5 text-[10px] text-muted truncate" title={levelText()}>
                      {levelText()}
                    </div>
                  </div>

                  <div class="flex items-center gap-1.5 shrink-0" title={stateText()}>
                    <StatusDot
                      variant={variant()}
                      size="xs"
                      ariaLabel={`RAID state: ${stateText()}`}
                    />
                    <span class={`text-[10px] font-medium ${getRaidStateTextClass(array.state)}`}>
                      {stateText()}
                    </span>
                  </div>
                </div>

                <Show when={rebuilding()}>
                  <div class="mt-2 text-[10px] text-muted">
                    Rebuild:{' '}
                    <span class="font-medium text-base-content">
                      {Math.round(array.rebuildPercent)}%
                    </span>
                    <Show when={array.rebuildSpeed}>
                      <span class="text-muted"> · </span>
                      <span class="font-medium text-base-content">{array.rebuildSpeed}</span>
                    </Show>
                  </div>
                </Show>

                <Show when={array.devices && array.devices.length > 0}>
                  <div class="mt-2 flex flex-wrap gap-1">
                    <For each={array.devices}>
                      {(device) => (
                        <span
                          class={`inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-medium ${getRaidDeviceBadgeClass(device)}`}
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
