import { Component, For, Show } from 'solid-js';
import type { DeployWizardState } from '@/hooks/useDeployWizard';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import CheckCircleIcon from 'lucide-solid/icons/check-circle-2';

interface ConfirmStepProps {
  wizard: DeployWizardState;
}

export const ConfirmStep: Component<ConfirmStepProps> = (props) => {
  const w = props.wizard;

  return (
    <div class="space-y-4">
      {/* Ready nodes */}
      <Show when={w.readyNodes().length > 0}>
        <div class="space-y-2">
          <h4 class="text-xs font-semibold text-base-content flex items-center gap-1.5">
            <CheckCircleIcon class="w-3.5 h-3.5 text-emerald-500" />
            Ready to deploy ({w.readyNodes().length})
          </h4>
          <Table wrapperClass="rounded-md border border-border" class="text-sm">
            <TableHeader>
              <TableRow class="bg-surface-alt text-left">
                <TableHead class="w-8 px-3 py-2" />
                <TableHead class="px-3 py-2 font-medium text-muted text-xs">Node</TableHead>
                <TableHead class="px-3 py-2 font-medium text-muted text-xs">IP</TableHead>
                <TableHead class="px-3 py-2 font-medium text-muted text-xs">Arch</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <For each={w.readyNodes()}>
                {(target) => (
                  <TableRow
                    class="hover:bg-surface-hover cursor-pointer"
                    tabIndex={0}
                    onClick={() => w.toggleConfirmNode(target.nodeId)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        w.toggleConfirmNode(target.nodeId);
                      }
                    }}
                  >
                    <TableCell class="px-3 py-2">
                      <input
                        type="checkbox"
                        checked={w.confirmSelectedNodeIds().has(target.nodeId)}
                        onChange={() => w.toggleConfirmNode(target.nodeId)}
                        onClick={(e) => e.stopPropagation()}
                        class="rounded border-border"
                      />
                    </TableCell>
                    <TableCell class="px-3 py-2 font-medium text-base-content">
                      {target.nodeName}
                    </TableCell>
                    <TableCell class="px-3 py-2 text-muted font-mono text-xs">
                      {target.nodeIP}
                    </TableCell>
                    <TableCell class="px-3 py-2 text-muted text-xs">
                      {target.arch || 'amd64'}
                    </TableCell>
                  </TableRow>
                )}
              </For>
            </TableBody>
          </Table>
        </div>
      </Show>

      {/* Failed preflight nodes */}
      <Show when={w.failedPreflightNodes().length > 0}>
        <div class="space-y-2">
          <h4 class="text-xs font-semibold text-muted">
            Cannot deploy ({w.failedPreflightNodes().length})
          </h4>
          <Table wrapperClass="rounded-md border border-border" class="text-sm">
            <TableHeader>
              <TableRow class="bg-surface-alt text-left">
                <TableHead class="px-3 py-2 font-medium text-muted text-xs">Node</TableHead>
                <TableHead class="px-3 py-2 font-medium text-muted text-xs">IP</TableHead>
                <TableHead class="px-3 py-2 font-medium text-muted text-xs">Reason</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <For each={w.failedPreflightNodes()}>
                {(target) => (
                  <TableRow class="opacity-60">
                    <TableCell class="px-3 py-2 font-medium text-base-content">
                      {target.nodeName}
                    </TableCell>
                    <TableCell class="px-3 py-2 text-muted font-mono text-xs">
                      {target.nodeIP}
                    </TableCell>
                    <TableCell class="px-3 py-2 text-xs text-red-600 dark:text-red-400">
                      {target.errorMessage || 'Preflight failed'}
                    </TableCell>
                  </TableRow>
                )}
              </For>
            </TableBody>
          </Table>
        </div>
      </Show>
    </div>
  );
};
