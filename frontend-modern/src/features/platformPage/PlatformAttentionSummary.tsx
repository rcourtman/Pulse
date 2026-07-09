import { For, Show, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';

export type PlatformAttentionSummaryTone = 'danger' | 'warning' | 'info';

export type PlatformAttentionSummaryMetric = {
  label: string;
  value: string | number;
};

const toneClasses: Record<PlatformAttentionSummaryTone, string> = {
  danger: 'border-red-300 bg-red-50/70 dark:border-red-900/70 dark:bg-red-950/20',
  warning: 'border-amber-300 bg-amber-50/70 dark:border-amber-900/70 dark:bg-amber-950/20',
  info: 'border-blue-300 bg-blue-50/70 dark:border-blue-900/70 dark:bg-blue-950/20',
};

const toneVariant = (tone: PlatformAttentionSummaryTone) =>
  tone === 'danger' ? 'danger' : tone === 'warning' ? 'warning' : 'info';

export function PlatformAttentionSummary(props: {
  title: string;
  headline: string;
  description: string;
  tone: PlatformAttentionSummaryTone;
  metrics?: readonly PlatformAttentionSummaryMetric[];
  actions?: JSX.Element;
}) {
  return (
    <TableCard
      class={`border ${toneClasses[props.tone]}`}
      role="region"
      aria-label={props.title}
      data-platform-attention-summary={props.tone}
    >
      <div class="flex flex-col gap-3 px-3 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <StatusDot
              size="sm"
              variant={toneVariant(props.tone)}
              title={props.headline}
              ariaHidden
            />
            <span class="text-[11px] font-semibold uppercase tracking-wide text-muted">
              {props.title}
            </span>
            <span class="text-sm font-semibold text-base-content">{props.headline}</span>
          </div>
          <p class="mt-1 text-xs leading-5 text-muted">{props.description}</p>
        </div>

        <div class="flex flex-wrap items-center gap-3 sm:justify-end">
          <For each={props.metrics ?? []}>
            {(metric) => (
              <div class="min-w-14 text-right">
                <div class="text-sm font-semibold tabular-nums text-base-content">
                  {metric.value}
                </div>
                <div class="text-[10px] uppercase tracking-wide text-muted">{metric.label}</div>
              </div>
            )}
          </For>
          <Show when={props.actions}>
            <div class="flex flex-wrap items-center gap-2">{props.actions}</div>
          </Show>
        </div>
      </div>
    </TableCard>
  );
}

export default PlatformAttentionSummary;
