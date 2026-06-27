import { Show } from 'solid-js';
import { Activity, AlertCircle, Check } from 'lucide-solid';

import type { ResourceAvailabilityMeta } from '@/types/resource';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import { getAvailabilityProbeMethodLabel } from '@/utils/availabilityProbePresentation';
import { formatRelativeTime } from '@/utils/format';

interface AvailabilityProbeStatusCardProps {
  availability: ResourceAvailabilityMeta;
}

export function AvailabilityProbeStatusCard(props: AvailabilityProbeStatusCardProps) {
  const isUp = () => props.availability.available === true;
  const latency = () => {
    const ms = props.availability.latencyMillis;
    return typeof ms === 'number' && Number.isFinite(ms) && ms > 0 ? `${Math.round(ms)}ms` : null;
  };
  const lastChecked = () => formatRelativeTime(props.availability.lastChecked);
  const method = () => getAvailabilityProbeMethodLabel(props.availability);
  const targetAddr = () => {
    const addr = props.availability.address ?? '';
    const port = props.availability.port;
    return port ? `${addr}:${port}` : addr;
  };
  const failureLabel = () => {
    const err = (props.availability.lastError ?? '').trim();
    if (!err) return null;
    if (/timed?\s*out/i.test(err)) return 'Timed out';
    const httpMatch = err.match(/\b([45]\d{2})\b/);
    if (httpMatch) return `HTTP ${httpMatch[1]}`;
    if (/refused|unreachable|no route/i.test(err)) return 'Unreachable';
    return err.length > 40 ? `${err.slice(0, 40)}…` : err;
  };

  return (
    <InfoCardFrame data-testid="availability-probe-status">
      <div class="flex items-center justify-between gap-2 mb-2">
        <div class="flex min-w-0 items-center gap-1.5">
          <Activity class="h-3.5 w-3.5 text-base-content/60" aria-hidden="true" />
          <h3 class="truncate text-[11px] font-medium uppercase tracking-wide text-base-content">
            Availability
          </h3>
        </div>
        <span
          class="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold"
          classList={{
            'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300': isUp(),
            'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300': !isUp(),
          }}
        >
          {isUp() ? 'Up' : 'Down'}
        </span>
      </div>
      <div class="space-y-1.5 text-[11px]">
        <div class="flex items-center justify-between gap-2">
          <span class="text-muted">Latency</span>
          <Show when={isUp() && latency()} fallback={<span class="text-red-600 dark:text-red-400 font-medium">—</span>}>
            <span class="font-medium text-emerald-600 dark:text-emerald-400">{latency()}</span>
          </Show>
        </div>
        <div class="flex items-center justify-between gap-2">
          <span class="text-muted">Method</span>
          <span class="font-medium text-base-content" title={targetAddr()}>{method()}</span>
        </div>
        <div class="flex items-center justify-between gap-2">
          <span class="text-muted">Target</span>
          <span class="font-medium text-base-content truncate ml-2" title={targetAddr()}>{targetAddr()}</span>
        </div>
        <Show when={lastChecked()}>
          <div class="flex items-center justify-between gap-2">
            <span class="text-muted">Checked</span>
            <span class="text-base-content/70">{lastChecked()}</span>
          </div>
        </Show>
        <Show when={!isUp() && failureLabel()}>
          <div class="flex items-start gap-1.5 mt-1.5 pt-1.5 border-t border-base-200">
            <AlertCircle class="h-3 w-3 text-red-500 shrink-0 mt-0.5" aria-hidden="true" />
            <span class="text-[10px] text-red-600 dark:text-red-400">{failureLabel()}</span>
          </div>
        </Show>
        <Show when={isUp()}>
          <div class="flex items-center gap-1 mt-1.5 pt-1.5 border-t border-base-200">
            <Check class="h-3 w-3 text-emerald-500 shrink-0" aria-hidden="true" />
            <span class="text-[10px] text-muted">Responding normally</span>
          </div>
        </Show>
      </div>
    </InfoCardFrame>
  );
}
