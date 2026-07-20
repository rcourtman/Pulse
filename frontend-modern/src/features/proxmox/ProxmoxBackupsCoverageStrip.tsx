import { For, Show, type Component, type JSX } from 'solid-js';

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
}

export interface CoverageStripProps {
  title: string;
  segments: readonly CoverageStripSegment[];
  // Compact context shown to the right of the title (e.g. "1.2 TB on disk").
  tail?: JSX.Element;
}

export const ProxmoxBackupsCoverageStrip: Component<CoverageStripProps> = (props) => {
  const total = () =>
    props.segments.reduce(
      (sum, seg) => sum + (Number.isFinite(seg.value) ? Math.max(0, seg.value) : 0),
      0,
    );

  return (
    <div class="rounded-lg border border-border-subtle bg-surface-alt/25 px-3 py-2">
      <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
          {props.title}
        </div>
        <Show when={props.tail}>
          <div class="ml-auto text-[11px] text-muted">{props.tail}</div>
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
      <ul class="mt-1.5 flex flex-wrap gap-x-3 gap-y-0.5 text-[11px]">
        <For each={props.segments}>
          {(segment) => (
            <li
              class={`inline-flex items-center gap-1.5 ${segment.muted ? 'text-muted/70' : 'text-base-content'}`}
            >
              <span class={`h-2 w-2 shrink-0 rounded-sm ${segment.toneClass}`} aria-hidden="true" />
              <span class="tabular-nums">
                {segment.display ?? String(Math.max(0, Math.round(segment.value)))}
              </span>
              <span class="text-muted">{segment.label}</span>
            </li>
          )}
        </For>
      </ul>
    </div>
  );
};

export default ProxmoxBackupsCoverageStrip;
