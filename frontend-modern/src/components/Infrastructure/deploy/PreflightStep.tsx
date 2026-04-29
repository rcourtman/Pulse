import { Component, For, Show, createMemo } from 'solid-js';
import type { DeployWizardState } from '@/hooks/useDeployWizard';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { DeployStatusBadge } from './DeployStatusBadge';
import { ErrorDetail } from './ErrorDetail';
import CheckCircleIcon from 'lucide-solid/icons/check-circle-2';
import LoaderIcon from 'lucide-solid/icons/loader-2';

interface PreflightStepProps {
  wizard: DeployWizardState;
}

export const PreflightStep: Component<PreflightStepProps> = (props) => {
  const w = props.wizard;

  const completedCount = createMemo(
    () =>
      w.preflightTargets().filter((t) => t.status !== 'pending' && t.status !== 'preflighting')
        .length,
  );

  const totalCount = createMemo(() => w.preflightTargets().length);

  return (
    <div class="space-y-4">
      <Show when={w.preflightError()}>
        <div
          role="alert"
          class="rounded-md bg-red-50 dark:bg-red-900/20 p-3 text-sm text-red-700 dark:text-red-300"
        >
          {w.preflightError()}
        </div>
      </Show>

      {/* Progress summary */}
      <div role="status" aria-live="polite" class="flex items-center gap-2 text-sm text-muted">
        <Show
          when={totalCount() === 0 || completedCount() < totalCount()}
          fallback={<CheckCircleIcon class="w-4 h-4 text-emerald-500" />}
        >
          <LoaderIcon class="w-4 h-4 animate-spin" />
        </Show>
        <span>
          <Show
            when={totalCount() === 0 || completedCount() < totalCount()}
            fallback="Preflight checks complete"
          >
            Checking {completedCount()} of {totalCount()} nodes...
          </Show>
        </span>
      </div>

      {/* Per-node rows */}
      <Table wrapperClass="rounded-md border border-border" class="text-sm">
        <TableHeader>
          <TableRow class="bg-surface-alt text-left">
            <TableHead class="px-3 py-2 font-medium text-muted text-xs">Node</TableHead>
            <TableHead class="px-3 py-2 font-medium text-muted text-xs">IP</TableHead>
            <TableHead class="px-3 py-2 font-medium text-muted text-xs">Status</TableHead>
            <TableHead class="px-3 py-2 font-medium text-muted text-xs">Details</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <For each={w.preflightTargets()}>
            {(target) => (
              <TableRow>
                <TableCell class="px-3 py-2 font-medium text-base-content">
                  {target.nodeName}
                </TableCell>
                <TableCell class="px-3 py-2 text-muted font-mono text-xs">
                  {target.nodeIP}
                </TableCell>
                <TableCell class="px-3 py-2">
                  <DeployStatusBadge status={target.status} />
                </TableCell>
                <TableCell class="px-3 py-2">
                  <ErrorDetail message={target.errorMessage} />
                </TableCell>
              </TableRow>
            )}
          </For>
        </TableBody>
      </Table>
    </div>
  );
};
