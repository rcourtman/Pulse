import { type Component } from 'solid-js';
import type { ExploreStatus } from './types';

interface ExploreStatusBlockProps {
  status: ExploreStatus;
}

const phaseLabel = (phase: string): string => {
  switch (phase) {
    case 'started':
      return 'Explore Started';
    case 'completed':
      return 'Explore Completed';
    case 'failed':
      return 'Explore Failed';
    case 'skipped':
      return 'Explore Skipped';
    default:
      return 'Explore Status';
  }
};

const phaseClasses = (phase: string): string => {
  switch (phase) {
    case 'completed':
      return 'border-emerald-200/80 bg-emerald-50/70 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-900/20 dark:text-emerald-200';
    case 'failed':
      return 'border-rose-200/80 bg-rose-50/70 text-rose-700 dark:border-rose-500/30 dark:bg-rose-900/20 dark:text-rose-200';
    case 'skipped':
      return 'border-amber-200/80 bg-amber-50/70 text-amber-700 dark:border-amber-500/30 dark:bg-amber-900/20 dark:text-amber-200';
    case 'started':
    default:
      return 'border-sky-200/80 bg-sky-50/70 text-sky-700 dark:border-sky-500/30 dark:bg-sky-900/20 dark:text-sky-200';
  }
};

export const ExploreStatusBlock: Component<ExploreStatusBlockProps> = (props) => (
  <div class={`my-2 rounded-lg border px-3 py-2 text-xs ${phaseClasses(props.status.phase)}`}>
    <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
      <span class="font-semibold tracking-wide uppercase">{phaseLabel(props.status.phase)}</span>
      {props.status.model && <span class="font-mono opacity-80">{props.status.model}</span>}
      {props.status.outcome && <span class="font-mono opacity-75">outcome={props.status.outcome}</span>}
    </div>
    <p class="mt-1 leading-relaxed">{props.status.message}</p>
  </div>
);
