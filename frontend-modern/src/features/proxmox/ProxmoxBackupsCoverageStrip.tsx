import { For, Index, Show, type Component, type JSX } from 'solid-js';

// A single horizontal bar broken into proportional segments, followed by
// a one-line legend of labels + counts. Visually integrated with the page
// chrome (no tile border per metric) so it reads as a status line, not a
// stat-card grid. Used at the top of Snapshots and Backup files to answer
// the "is my backup posture healthy?" question on sight.

export interface CoverageStripSegment {
  key: string;
  // Value used to compute the segment's proportional width.
  value: number;
  label: string;
  // Tailwind class applied to the segment bar (e.g. `bg-emerald-500`).
  toneClass: string;
  // Optional formatted display value; defaults to the raw `value`.
  display?: string;
  // When true the segment is rendered with a neutral tone for "absent".
  muted?: boolean;
  // A segment that names a list filter is a toggle: selecting "11
  // unprotected" shows those workloads instead of leaving the count as text.
  onSelect?: () => void;
  pressed?: boolean;
}

export interface CoverageStripProps {
  title: string;
  segments: readonly CoverageStripSegment[];
  // Compact context shown to the right of the title (e.g. "1.2 TB on disk").
  tail?: JSX.Element;
  // The rule behind the segments, so a count such as "17 attention" says what
  // it is counting.
  footnote?: JSX.Element;
}

export const ProxmoxBackupsCoverageStrip: Component<CoverageStripProps> = (props) => {
  const total = () =>
    props.segments.reduce(
      (sum, seg) => sum + (Number.isFinite(seg.value) ? Math.max(0, seg.value) : 0),
      0,
    );

  return (
    <div class="rounded-lg border border-border-subtle px-3 py-2">
      <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
          {props.title}
        </div>
        <Show when={props.tail}>
          <div class="w-full text-[11px] text-muted sm:ml-auto sm:w-auto">{props.tail}</div>
        </Show>
      </div>
      <Show
        when={total() > 0}
        fallback={<div class="mt-2 h-2 w-full rounded-full bg-surface" aria-hidden="true" />}
      >
        <div class="mt-2 flex h-2 w-full overflow-hidden rounded-full bg-surface">
          <For each={props.segments}>
            {(segment) => {
              const widthPct = () =>
                total() > 0 ? Math.max(0, (segment.value / total()) * 100) : 0;
              return (
                <Show when={widthPct() > 0}>
                  <div
                    class={segment.toneClass}
                    style={{ width: `${widthPct()}%` }}
                    role="presentation"
                  />
                </Show>
              );
            }}
          </For>
        </div>
      </Show>
      <ul class="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px]">
        {/* Index keeps each legend item's node while the caller rebuilds the
            segment objects, so a focused count keeps focus after it toggles. */}
        <Index each={props.segments}>
          {(segment) => {
            const content = () => (
              <>
                <span
                  class={`h-2 w-2 shrink-0 rounded-xs ${segment().toneClass}`}
                  aria-hidden="true"
                />
                <span class="tabular-nums">
                  {segment().display ?? String(Math.max(0, Math.round(segment().value)))}
                </span>
                <span class="text-muted">{segment().label}</span>
              </>
            );
            return (
              <li class={`inline-flex items-center ${segment().muted ? '' : 'text-base-content'}`}>
                <Show
                  when={segment().onSelect}
                  fallback={<span class="inline-flex items-center gap-1.5">{content()}</span>}
                >
                  {(onSelect) => (
                    <button
                      type="button"
                      class={`inline-flex min-h-7 items-center gap-1.5 rounded-md border px-1.5 hover:bg-surface-hover focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 sm:min-h-6 ${
                        segment().pressed
                          ? 'border-blue-400 bg-blue-50 dark:border-blue-500 dark:bg-blue-900/30'
                          : 'border-border'
                      }`}
                      aria-pressed={segment().pressed ?? false}
                      onClick={() => onSelect()()}
                    >
                      {content()}
                    </button>
                  )}
                </Show>
              </li>
            );
          }}
        </Index>
      </ul>
      <Show when={props.footnote}>
        <p class="mt-1.5 text-[11px] leading-4 text-muted" data-coverage-strip-footnote>
          {props.footnote}
        </p>
      </Show>
    </div>
  );
};

export default ProxmoxBackupsCoverageStrip;
