import { Show } from 'solid-js';

import type { DashboardState } from './useDashboardState';

type DashboardStatsStripProps = Pick<
  DashboardState,
  'connected' | 'initialDataReceived' | 'totalStats'
>;

export function DashboardStatsStrip(props: DashboardStatsStripProps) {
  return (
    <Show when={props.connected() && props.initialDataReceived()}>
      <div class="mb-4">
        <div class="flex items-center gap-2 p-2 bg-surface-alt border border-border rounded">
          <span class="flex items-center gap-1 text-xs text-muted">
            <span class="h-2 w-2 bg-green-500 rounded-full"></span>
            {props.totalStats().running} running
          </span>
          <Show when={props.totalStats().degraded > 0}>
            <span class="text-slate-400">|</span>
            <span class="flex items-center gap-1 text-xs text-muted">
              <span class="h-2 w-2 bg-orange-500 rounded-full"></span>
              {props.totalStats().degraded} degraded
            </span>
          </Show>
          <span class="text-slate-400">|</span>
          <span class="flex items-center gap-1 text-xs text-muted">
            <span class="h-2 w-2 bg-red-500 rounded-full"></span>
            {props.totalStats().stopped} stopped
          </span>
        </div>
      </div>
    </Show>
  );
}
