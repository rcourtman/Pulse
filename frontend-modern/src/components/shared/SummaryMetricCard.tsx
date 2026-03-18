import { Show, type JSX } from 'solid-js';
import { Card } from './Card';
import { SparklineSkeleton } from './SparklineSkeleton';

export interface SummaryMetricCardProps {
  /** Uppercase label at top-left of the card. */
  label: string;
  /** Optional secondary content to the right of the label. */
  secondaryLabel?: JSX.Element;
  /** Whether data has loaded at least once. Drives skeleton vs empty state. */
  loaded: boolean;
  /** Whether there is renderable data. When false, shows the fallback. */
  hasData: boolean;
  /** Message to show when loaded but no data. Defaults to "No history yet". */
  emptyMessage?: string;
  /** Chart or visualization content. */
  children: JSX.Element;
}

export function SummaryMetricCard(props: SummaryMetricCardProps) {
  return (
    <Card padding="sm" class="h-full">
      <div class="flex flex-col h-full">
        <div class="flex items-center mb-1.5 min-w-0">
          <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">
            {props.label}
          </span>
          {props.secondaryLabel}
        </div>
        <Show
          when={props.hasData}
          fallback={
            props.loaded ? (
              <div class="flex h-[56px] items-center text-sm text-muted">
                {props.emptyMessage ?? 'No history yet'}
              </div>
            ) : (
              <SparklineSkeleton />
            )
          }
        >
          <div class="flex-1 min-h-0">{props.children}</div>
        </Show>
      </div>
    </Card>
  );
}
