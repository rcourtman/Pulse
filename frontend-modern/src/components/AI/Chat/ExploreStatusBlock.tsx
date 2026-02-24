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
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500 dark:bg-emerald-900 dark:text-emerald-200';
    case 'failed':
      return 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-500 dark:bg-rose-900 dark:text-rose-200';
    case 'skipped':
      return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500 dark:bg-amber-900 dark:text-amber-200';
    case 'started':
    default:
      return 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-500 dark:bg-sky-900 dark:text-sky-200';
  }
};

export const ExploreStatusBlock: Component<ExploreStatusBlockProps> = (props) => (
  <div class={`my-2 rounded-md border px-3 py-2 text-xs ${phaseClasses(props.status.phase)}`}>
    <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
      <span class="font-semibold tracking-wide uppercase">{phaseLabel(props.status.phase)}</span>
      {props.status.model && <span class="font-mono opacity-80">{props.status.model}</span>}
      {props.status.outcome && (
        <span class="font-mono opacity-75">outcome={props.status.outcome}</span>
      )}
    </div>
    <p class="mt-1 leading-relaxed">{props.status.message}</p>
  </div>
);
