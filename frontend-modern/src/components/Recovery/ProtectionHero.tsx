import { Component, createMemo, Show } from 'solid-js';

import { Card } from '@/components/shared/Card';
import { MiniDonut } from '@/pages/DashboardPanels/Visualizations';

export interface ProtectionHeroProps {
  summary: {
    total: number;
    counts: { success: number; warning: number; failed: number; running: number; unknown: number };
    stale: number;
    neverSucceeded: number;
  };
}

export const ProtectionHero: Component<ProtectionHeroProps> = (props) => {
  const issues = createMemo(() => props.summary.counts.warning + props.summary.counts.failed);
  const issueParts = createMemo(() => {
    const parts: string[] = [];
    if (props.summary.counts.warning > 0) parts.push(`${props.summary.counts.warning} warn`);
    if (props.summary.counts.failed > 0) parts.push(`${props.summary.counts.failed} failed`);
    return parts.join(' · ');
  });

  const donutData = createMemo(() => [
    { value: props.summary.counts.success, color: 'text-emerald-500 dark:text-emerald-400' },
    { value: props.summary.counts.warning, color: 'text-amber-500 dark:text-amber-400' },
    { value: props.summary.counts.failed, color: 'text-red-500 dark:text-red-400' },
    { value: props.summary.counts.running, color: 'text-blue-500 dark:text-blue-400' },
    { value: props.summary.counts.unknown, color: 'text-slate-300 dark:text-slate-600' },
  ]);

  const segmentPercentages = createMemo(() => {
    const total = props.summary.total;
    if (total <= 0) return [];
    return [
      { count: props.summary.counts.success, color: 'bg-emerald-500' },
      { count: props.summary.counts.running, color: 'bg-blue-500' },
      { count: props.summary.counts.warning, color: 'bg-amber-400' },
      { count: props.summary.counts.failed, color: 'bg-red-500' },
    ]
      .filter((segment) => segment.count > 0)
      .map((segment) => ({ color: segment.color, width: (segment.count / total) * 100 }));
  });

  return (
    <Show when={props.summary.total > 0}>
      <Card padding="sm">
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div class="flex items-center gap-3 rounded-md border border-border bg-slate-50 dark:bg-slate-800 px-3 py-2.5">
            <MiniDonut size={32} strokeWidth={4} data={donutData()} centerText={String(props.summary.total)} />
            <div class="min-w-0">
              <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">Protected</div>
              <div class="text-sm font-bold text-base-content">{props.summary.total}</div>
              <div class="text-[10px] text-muted">{props.summary.counts.success} healthy</div>
            </div>
          </div>

          <div class="flex items-center gap-3 rounded-md border border-border bg-slate-50 dark:bg-slate-800 px-3 py-2.5">
            <div class="w-8 h-8 rounded-full bg-emerald-100 dark:bg-emerald-900 flex items-center justify-center flex-shrink-0">
              <svg class="w-4 h-4 text-emerald-600 dark:text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                <path d="m9 12 2 2 4-4" />
              </svg>
            </div>
            <div class="min-w-0">
              <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">Healthy</div>
              <div class="text-emerald-600 dark:text-emerald-400 font-bold text-sm">{props.summary.counts.success}</div>
              <Show
                when={props.summary.counts.running > 0}
                fallback={<div class="text-[10px] text-muted">of {props.summary.total}</div>}
              >
                <div class="text-[10px] text-blue-600 dark:text-blue-400">{props.summary.counts.running} running</div>
              </Show>
            </div>
          </div>

          <div class="flex items-center gap-3 rounded-md border border-border bg-slate-50 dark:bg-slate-800 px-3 py-2.5">
            <div
              class="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0"
              classList={{
                'bg-amber-100 dark:bg-amber-900': issues() > 0,
                'bg-slate-100 dark:bg-slate-800': issues() === 0,
              }}
            >
              <svg
                class="w-4 h-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                classList={{
                  'text-amber-600 dark:text-amber-400': issues() > 0,
                  'text-muted': issues() === 0,
                }}
              >
                <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z" />
                <line x1="12" y1="9" x2="12" y2="13" />
                <line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
            </div>
            <div class="min-w-0">
              <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">Issues</div>
              <div
                class="font-bold text-sm"
                classList={{
                  'text-amber-600 dark:text-amber-400': issues() > 0,
                  'text-muted': issues() === 0,
                }}
              >
                {issues()}
              </div>
              <Show when={issues() > 0} fallback={<div class="text-[10px] text-muted">none</div>}>
                <div class="text-[10px] text-muted">{issueParts()}</div>
              </Show>
            </div>
          </div>

          <div class="flex items-center gap-3 rounded-md border border-border bg-slate-50 dark:bg-slate-800 px-3 py-2.5">
            <div
              class="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0"
              classList={{
                'bg-orange-100 dark:bg-orange-900': props.summary.stale > 0,
                'bg-slate-100 dark:bg-slate-800': props.summary.stale === 0,
              }}
            >
              <svg
                class="w-4 h-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                classList={{
                  'text-orange-600 dark:text-orange-400': props.summary.stale > 0,
                  'text-muted': props.summary.stale === 0,
                }}
              >
                <circle cx="12" cy="12" r="10" />
                <polyline points="12 6 12 12 16 14" />
              </svg>
            </div>
            <div class="min-w-0">
              <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">Stale</div>
              <div
                class="font-bold text-sm"
                classList={{
                  'text-orange-600 dark:text-orange-400': props.summary.stale > 0,
                  'text-muted': props.summary.stale === 0,
                }}
              >
                {props.summary.stale}
              </div>
              <Show
                when={props.summary.neverSucceeded > 0}
                fallback={
                  <div class="text-[10px] text-muted">
                    {props.summary.stale > 0 ? '> 7 days old' : 'all current'}
                  </div>
                }
              >
                <div class="text-[10px] text-rose-600 dark:text-rose-400">{props.summary.neverSucceeded} never succeeded</div>
              </Show>
            </div>
          </div>
        </div>

        <div class="mt-3 h-1.5 rounded-full overflow-hidden bg-slate-100 dark:bg-slate-700">
          <div class="flex h-full">
            {segmentPercentages().map((segment) => (
              <div class={`h-full transition-all duration-500 ${segment.color}`} style={{ width: `${segment.width}%` }} />
            ))}
          </div>
        </div>
      </Card>
    </Show>
  );
};
