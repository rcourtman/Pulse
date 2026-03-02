import { Component, For, Show, createMemo } from 'solid-js';
import type { DeployWizardState } from '@/hooks/useDeployWizard';
import { DeployStatusBadge } from './DeployStatusBadge';
import { ErrorDetail } from './ErrorDetail';
import LoaderIcon from 'lucide-solid/icons/loader-2';
import CheckCircleIcon from 'lucide-solid/icons/check-circle-2';

interface DeployingStepProps {
  wizard: DeployWizardState;
}

export const DeployingStep: Component<DeployingStepProps> = (props) => {
  const w = props.wizard;

  const inProgressStatuses = new Set(['installing', 'enrolling', 'verifying']);

  const completedCount = createMemo(
    () =>
      w
        .jobTargets()
        .filter(
          (t) =>
            t.status === 'succeeded' ||
            t.status === 'failed_retryable' ||
            t.status === 'failed_permanent' ||
            t.status === 'skipped_already_agent' ||
            t.status === 'skipped_license' ||
            t.status === 'canceled',
        ).length,
  );

  const inProgressCount = createMemo(
    () => w.jobTargets().filter((t) => inProgressStatuses.has(t.status)).length,
  );

  const totalCount = createMemo(() => w.jobTargets().length);

  return (
    <div class="space-y-4">
      <Show when={w.deployError()}>
        <div
          role="alert"
          class="rounded-md bg-red-50 dark:bg-red-900/20 p-3 text-sm text-red-700 dark:text-red-300"
        >
          {w.deployError()}
        </div>
      </Show>

      {/* Progress summary */}
      <div class="flex items-center justify-between">
        <div role="status" aria-live="polite" class="flex items-center gap-2 text-sm text-muted">
          <Show
            when={totalCount() === 0 || completedCount() < totalCount()}
            fallback={<CheckCircleIcon class="w-4 h-4 text-emerald-500" />}
          >
            <LoaderIcon class="w-4 h-4 animate-spin" />
          </Show>
          <span>
            Installing {completedCount()} of {totalCount()} nodes...
            <Show when={inProgressCount() > 0}> ({inProgressCount()} in progress)</Show>
          </span>
        </div>
      </div>

      {/* Per-node rows */}
      <div class="rounded-md border border-border overflow-hidden">
        <table class="w-full text-sm">
          <thead>
            <tr class="bg-surface-alt text-left">
              <th class="px-3 py-2 font-medium text-muted text-xs">Node</th>
              <th class="px-3 py-2 font-medium text-muted text-xs">IP</th>
              <th class="px-3 py-2 font-medium text-muted text-xs">Status</th>
              <th class="px-3 py-2 font-medium text-muted text-xs">Details</th>
            </tr>
          </thead>
          <tbody>
            <For each={w.jobTargets()}>
              {(target) => (
                <tr class="border-t border-border">
                  <td class="px-3 py-2 font-medium text-base-content">{target.nodeName}</td>
                  <td class="px-3 py-2 text-muted font-mono text-xs">{target.nodeIP}</td>
                  <td class="px-3 py-2">
                    <DeployStatusBadge status={target.status} />
                  </td>
                  <td class="px-3 py-2">
                    <ErrorDetail message={target.errorMessage} />
                  </td>
                </tr>
              )}
            </For>
          </tbody>
        </table>
      </div>
    </div>
  );
};
