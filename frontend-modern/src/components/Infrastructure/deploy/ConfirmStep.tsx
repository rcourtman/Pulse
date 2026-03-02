import { Component, For, Show, createMemo } from 'solid-js';
import type { DeployWizardState } from '@/hooks/useDeployWizard';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import CheckCircleIcon from 'lucide-solid/icons/check-circle-2';

interface ConfirmStepProps {
  wizard: DeployWizardState;
}

export const ConfirmStep: Component<ConfirmStepProps> = (props) => {
  const w = props.wizard;

  const selectedCount = createMemo(() => w.confirmSelectedNodeIds().size);
  const maxSlots = createMemo(() => w.maxAgentSlots());
  const exceedsLicense = createMemo(() => maxSlots() > 0 && selectedCount() > maxSlots());

  return (
    <div class="space-y-4">
      {/* License slot summary */}
      <Show when={maxSlots() > 0}>
        <div
          role="status"
          class={`rounded-md p-3 text-sm flex items-start gap-2 ${
            exceedsLicense()
              ? 'bg-amber-50 dark:bg-amber-900/20 text-amber-700 dark:text-amber-300'
              : 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
          }`}
        >
          <AlertCircleIcon class="w-4 h-4 mt-0.5 shrink-0" />
          <span>
            {maxSlots()} license slots available, {selectedCount()} nodes selected.
            <Show when={exceedsLicense()}>
              {' '}
              Only {maxSlots()} nodes can be deployed. Remove {selectedCount() - maxSlots()} nodes
              or upgrade your plan.
            </Show>
          </span>
        </div>
      </Show>

      {/* Ready nodes */}
      <Show when={w.readyNodes().length > 0}>
        <div class="space-y-2">
          <h4 class="text-xs font-semibold text-base-content flex items-center gap-1.5">
            <CheckCircleIcon class="w-3.5 h-3.5 text-emerald-500" />
            Ready to deploy ({w.readyNodes().length})
          </h4>
          <div class="rounded-md border border-border overflow-hidden">
            <table class="w-full text-sm">
              <thead>
                <tr class="bg-surface-alt text-left">
                  <th class="w-8 px-3 py-2" />
                  <th class="px-3 py-2 font-medium text-muted text-xs">Node</th>
                  <th class="px-3 py-2 font-medium text-muted text-xs">IP</th>
                  <th class="px-3 py-2 font-medium text-muted text-xs">Arch</th>
                </tr>
              </thead>
              <tbody>
                <For each={w.readyNodes()}>
                  {(target) => (
                    <tr
                      class="border-t border-border hover:bg-surface-hover cursor-pointer"
                      tabIndex={0}
                      onClick={() => w.toggleConfirmNode(target.nodeId)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          w.toggleConfirmNode(target.nodeId);
                        }
                      }}
                    >
                      <td class="px-3 py-2">
                        <input
                          type="checkbox"
                          checked={w.confirmSelectedNodeIds().has(target.nodeId)}
                          onChange={() => w.toggleConfirmNode(target.nodeId)}
                          onClick={(e) => e.stopPropagation()}
                          class="rounded border-border"
                        />
                      </td>
                      <td class="px-3 py-2 font-medium text-base-content">{target.nodeName}</td>
                      <td class="px-3 py-2 text-muted font-mono text-xs">{target.nodeIP}</td>
                      <td class="px-3 py-2 text-muted text-xs">{target.arch || 'amd64'}</td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div>
        </div>
      </Show>

      {/* Failed preflight nodes */}
      <Show when={w.failedPreflightNodes().length > 0}>
        <div class="space-y-2">
          <h4 class="text-xs font-semibold text-muted">
            Cannot deploy ({w.failedPreflightNodes().length})
          </h4>
          <div class="rounded-md border border-border overflow-hidden">
            <table class="w-full text-sm">
              <thead>
                <tr class="bg-surface-alt text-left">
                  <th class="px-3 py-2 font-medium text-muted text-xs">Node</th>
                  <th class="px-3 py-2 font-medium text-muted text-xs">IP</th>
                  <th class="px-3 py-2 font-medium text-muted text-xs">Reason</th>
                </tr>
              </thead>
              <tbody>
                <For each={w.failedPreflightNodes()}>
                  {(target) => (
                    <tr class="border-t border-border opacity-60">
                      <td class="px-3 py-2 font-medium text-base-content">{target.nodeName}</td>
                      <td class="px-3 py-2 text-muted font-mono text-xs">{target.nodeIP}</td>
                      <td class="px-3 py-2 text-xs text-red-600 dark:text-red-400">
                        {target.errorMessage || 'Preflight failed'}
                      </td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div>
        </div>
      </Show>
    </div>
  );
};
