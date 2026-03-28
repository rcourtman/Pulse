import { For, Show, type JSX } from 'solid-js';
import {
  SUMMARY_TIME_RANGES,
  SUMMARY_TIME_RANGE_LABEL,
  type SummaryTimeRange,
} from './summaryTimeRange';

export interface SummaryPanelProps {
  /** Left-side header content (resource counts, badges). */
  headerLeft: JSX.Element;
  /** Currently selected time range (highlights the active button). */
  timeRange?: string;
  /** Called when the user clicks a time range button. Omit to hide the buttons. */
  onTimeRangeChange?: (range: SummaryTimeRange) => void;
  /** Custom time range options. Defaults to SUMMARY_TIME_RANGES. */
  timeRanges?: readonly string[];
  /** Custom labels for time range buttons. Defaults to SUMMARY_TIME_RANGE_LABEL. */
  timeRangeLabels?: Record<string, string>;
  /** Card grid content. */
  children: JSX.Element;
  /** data-testid for the outer wrapper. */
  testId?: string;
  /** Extra CSS classes on the outer container. */
  class?: string;
  /** Optional shared density override for compact monitoring surfaces. */
  density?: 'default' | 'compact';
}

export function SummaryPanel(props: SummaryPanelProps) {
  const ranges = () => (props.timeRanges ?? SUMMARY_TIME_RANGES) as readonly string[];
  const labels = () =>
    props.timeRangeLabels ?? (SUMMARY_TIME_RANGE_LABEL as Record<string, string>);
  const isCompact = () => props.density === 'compact';

  return (
    <div
      data-testid={props.testId}
      class={`rounded-md border border-border bg-surface shadow-sm ${
        isCompact() ? 'p-1.5 sm:p-2.5' : 'p-2 sm:p-3'
      } ${props.class ?? ''}`}
    >
      <div
        class={`flex flex-wrap items-center justify-between border-b border-border-subtle text-[11px] ${
          isCompact()
            ? 'mb-1.5 gap-1.5 px-0.5 pb-1.5'
            : 'mb-2 gap-2 px-1 pb-2'
        }`}
      >
        <div class="flex items-center gap-3">{props.headerLeft}</div>
        <Show when={props.onTimeRangeChange}>
          <div class="inline-flex shrink-0 rounded border border-border bg-surface p-0.5 text-xs">
            <For each={[...ranges()]}>
              {(range) => (
                <button
                  type="button"
                  onClick={() => props.onTimeRangeChange?.(range as SummaryTimeRange)}
                  class={`rounded px-2 py-1 ${
                    props.timeRange === range
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                      : 'text-muted hover:bg-surface-hover'
                  }`}
                >
                  {labels()[range] ?? range}
                </button>
              )}
            </For>
          </div>
        </Show>
      </div>

      <div class={`grid grid-cols-2 lg:grid-cols-4 ${isCompact() ? 'gap-2' : 'gap-2 sm:gap-3'}`}>
        {props.children}
      </div>
    </div>
  );
}
