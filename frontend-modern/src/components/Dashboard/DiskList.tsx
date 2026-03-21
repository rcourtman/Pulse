import { For, Show } from 'solid-js';
import type { DiskListProps } from './diskListModel';
import { useDiskListState } from './useDiskListState';

export function DiskList(props: DiskListProps) {
  const { diskPresentation, diskStatusTooltip, hasDisks } = useDiskListState(props);

  return (
    <Show
      when={hasDisks()}
      fallback={
        <span class="text-slate-400 text-sm" title={diskStatusTooltip()}>
          -
        </span>
      }
    >
      <div class="flex flex-col gap-1.5">
        <For each={diskPresentation()}>
          {(disk) => {
            return (
              <div class="rounded border border-border bg-surface-hover px-1.5 py-1 text-[10px] leading-tight shadow-sm">
                <div class="truncate text-base-content" title={disk.labelTitle}>
                  {disk.label}
                </div>
                <div class="mt-0.5 text-[9px] text-muted">{disk.usageText}</div>
                <div class="relative mt-1 h-1.5 w-full overflow-hidden rounded bg-surface-hover">
                  <div
                    class={`absolute inset-y-0 left-0 ${disk.progressClass}`}
                    style={{ width: disk.progressWidth }}
                  />
                </div>
                <div class="mt-0.5 flex items-center justify-between text-[9px] font-medium text-muted">
                  <span>{disk.usagePercentLabel}</span>
                  <span>{disk.typeLabel}</span>
                </div>
              </div>
            );
          }}
        </For>
      </div>
    </Show>
  );
}
