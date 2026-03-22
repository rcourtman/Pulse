import { Show } from 'solid-js';

import { Card } from '@/components/shared/Card';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableDockerIgnoredPrefixesSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Card padding="md" tone="card" class="mb-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h3 class="text-sm font-semibold text-base-content">
            {state.dockerIgnoredPrefixesPresentation.title}
          </h3>
          <p class="mt-1 text-xs text-muted">
            {state.dockerIgnoredPrefixesPresentation.description}
          </p>
        </div>
        <Show when={(tableProps.dockerIgnoredPrefixes().length ?? 0) > 0}>
          <button
            type="button"
            class="inline-flex items-center justify-center rounded-md border border-transparent px-3 py-1 text-xs font-medium transition hover:bg-surface-alt"
            onClick={state.handleResetDockerIgnored}
          >
            {state.dockerIgnoredPrefixesPresentation.resetLabel}
          </button>
        </Show>
      </div>
      <textarea
        value={state.dockerIgnoredInput()}
        onInput={(event) => state.handleDockerIgnoredChange(event.currentTarget.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            event.stopPropagation();
          }
        }}
        placeholder={state.dockerIgnoredPrefixesPresentation.placeholder}
        rows={4}
        class="mt-4 w-full rounded-md border border-border bg-surface p-3 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
      />
    </Card>
  );
}
