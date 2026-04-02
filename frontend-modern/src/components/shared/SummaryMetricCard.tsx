import { Show, type JSX } from 'solid-js';
import { Card } from './Card';
import { SparklineSkeleton } from './SparklineSkeleton';
import type { SummaryCardInteractionState } from './summaryCardInteraction';

export interface SummaryMetricCardProps {
  /** Uppercase label at top-left of the card. */
  label: string;
  /** Optional secondary content to the right of the label. */
  secondaryLabel?: JSX.Element;
  /** Optional compact readout aligned to the header right edge. */
  headerValue?: JSX.Element;
  /** Shared summary cards default to chart-backed bodies unless explicitly auto-sized. */
  bodyLayout?: 'chart' | 'auto';
  /** Whether data has loaded at least once. Drives skeleton vs empty state. */
  loaded: boolean;
  /** Whether there is renderable data. When false, shows the fallback. */
  hasData: boolean;
  /** Message to show when loaded but no data. Defaults to "No history yet". */
  emptyMessage?: string;
  /** Chart or visualization content. */
  children: JSX.Element;
  /** Optional shared density override for compact monitoring surfaces. */
  density?: 'default' | 'compact';
  /** Optional interaction state for shared hover/focus summary behavior. */
  interactionState?: SummaryCardInteractionState;
}

export function SummaryMetricCard(props: SummaryMetricCardProps) {
  const isCompact = () => props.density === 'compact';
  const bodyLayout = () => props.bodyLayout ?? 'chart';
  const interactionState = () => props.interactionState ?? 'default';
  const chartSlotClass = () =>
    isCompact() ? 'h-[108px] sm:h-[120px]' : 'h-[136px] sm:h-[150px]';
  const interactionClass = () => {
    switch (interactionState()) {
      case 'active':
        return 'border-sky-500/45 ring-2 ring-inset ring-sky-500/25 shadow-sm';
      case 'inactive':
        return 'opacity-50';
      default:
        return '';
    }
  };

  return (
    <div
      data-summary-card-state={interactionState()}
      class={`h-full rounded-md transition-all duration-150 ease-out ${interactionClass()}`.trim()}
    >
      <Card
        padding="sm"
        class={`h-full ${isCompact() ? '!p-1.5 sm:!p-2' : ''}`.trim()}
      >
        <div class="flex flex-col h-full">
          <div
            class={`flex min-w-0 items-center gap-2 ${isCompact() ? 'mb-1 min-h-[20px]' : 'mb-1.5 min-h-[24px]'}`.trim()}
          >
            <div class="flex min-w-0 flex-1 items-center">
              <span class="text-xs font-medium text-muted uppercase tracking-wide shrink-0">
                {props.label}
              </span>
              {props.secondaryLabel}
            </div>
            <Show when={props.headerValue}>
              <div class="ml-auto shrink-0">{props.headerValue}</div>
            </Show>
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
            <div
              class={`min-h-0 ${
                bodyLayout() === 'chart' ? `${chartSlotClass()} shrink-0` : 'flex-1'
              }`.trim()}
            >
              {props.children}
            </div>
          </Show>
        </div>
      </Card>
    </div>
  );
}
