import { Component, For, Show, createSignal } from 'solid-js';
import type { DeployWizardState } from '@/hooks/useDeployWizard';
import { DeployStatusBadge } from './DeployStatusBadge';
import CheckCircleIcon from 'lucide-solid/icons/check-circle-2';
import XCircleIcon from 'lucide-solid/icons/x-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';

interface ResultsStepProps {
  wizard: DeployWizardState;
}

export const ResultsStep: Component<ResultsStepProps> = (props) => {
  const w = props.wizard;
  const [manualInstallOpen, setManualInstallOpen] = createSignal(false);

  return (
    <div class="space-y-4">
      {/* Succeeded */}
      <Show when={w.succeededTargets().length > 0}>
        <div class="space-y-2">
          <h4 class="text-xs font-semibold text-emerald-600 dark:text-emerald-400 flex items-center gap-1.5">
            <CheckCircleIcon class="w-3.5 h-3.5" />
            Deployed ({w.succeededTargets().length})
          </h4>
          <div class="rounded-md border border-emerald-200 dark:border-emerald-800 overflow-hidden">
            <table class="w-full text-sm">
              <tbody>
                <For each={w.succeededTargets()}>
                  {(target) => (
                    <tr class="border-t border-emerald-100 dark:border-emerald-900 first:border-t-0">
                      <td class="px-3 py-2 font-medium text-base-content">{target.nodeName}</td>
                      <td class="px-3 py-2 text-muted font-mono text-xs">{target.nodeIP}</td>
                      <td class="px-3 py-2">
                        <DeployStatusBadge status={target.status} />
                      </td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div>
        </div>
      </Show>

      {/* Failed */}
      <Show when={w.failedTargets().length > 0}>
        <div class="space-y-2">
          <h4 class="text-xs font-semibold text-red-600 dark:text-red-400 flex items-center gap-1.5">
            <XCircleIcon class="w-3.5 h-3.5" />
            Failed ({w.failedTargets().length})
          </h4>
          <div class="rounded-md border border-red-200 dark:border-red-800 overflow-hidden">
            <table class="w-full text-sm">
              <tbody>
                <For each={w.failedTargets()}>
                  {(target) => (
                    <tr class="border-t border-red-100 dark:border-red-900 first:border-t-0">
                      <td class="px-3 py-2 font-medium text-base-content">{target.nodeName}</td>
                      <td class="px-3 py-2 text-muted font-mono text-xs">{target.nodeIP}</td>
                      <td class="px-3 py-2">
                        <DeployStatusBadge status={target.status} />
                      </td>
                      <td
                        class="px-3 py-2 text-xs text-red-600 dark:text-red-400 max-w-[200px] truncate"
                        title={target.errorMessage}
                      >
                        {target.errorMessage}
                      </td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div>

          {/* Manual install accordion for failed nodes */}
          <button
            type="button"
            onClick={() => setManualInstallOpen((o) => !o)}
            class="flex items-center gap-1 text-xs text-muted hover:text-base-content transition-colors"
          >
            <Show when={manualInstallOpen()} fallback={<ChevronRightIcon class="w-3 h-3" />}>
              <ChevronDownIcon class="w-3 h-3" />
            </Show>
            Manual install instructions
          </button>
          <Show when={manualInstallOpen()}>
            <div class="rounded-md bg-surface-alt p-3 text-xs space-y-2">
              <p class="text-muted">
                For nodes that failed SSH-based deployment, you can install the agent manually by
                SSHing into the node and running:
              </p>
              <pre class="bg-surface border border-border rounded p-2 text-[11px] font-mono text-base-content overflow-x-auto select-all">
                curl -fsSL http://&lt;pulse-url&gt;:7655/api/agent/install.sh | bash
              </pre>
              <p class="text-muted">
                Generate a bootstrap token in Settings &gt; Agents, then pass it with{' '}
                <code class="text-base-content">PULSE_TOKEN=&lt;token&gt;</code>.
              </p>
            </div>
          </Show>
        </div>
      </Show>

      {/* Skipped */}
      <Show when={w.skippedTargets().length > 0}>
        <div class="space-y-2">
          <h4 class="text-xs font-semibold text-muted flex items-center gap-1.5">
            <AlertCircleIcon class="w-3.5 h-3.5" />
            Skipped ({w.skippedTargets().length})
          </h4>
          <div class="rounded-md border border-border overflow-hidden">
            <table class="w-full text-sm">
              <tbody>
                <For each={w.skippedTargets()}>
                  {(target) => (
                    <tr class="border-t border-border first:border-t-0 opacity-60">
                      <td class="px-3 py-2 font-medium text-base-content">{target.nodeName}</td>
                      <td class="px-3 py-2 text-muted font-mono text-xs">{target.nodeIP}</td>
                      <td class="px-3 py-2">
                        <DeployStatusBadge status={target.status} />
                      </td>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div>
        </div>
      </Show>

      {/* Canceled */}
      <Show when={w.canceledTargets().length > 0}>
        <div class="space-y-2">
          <h4 class="text-xs font-semibold text-muted">Canceled ({w.canceledTargets().length})</h4>
          <div class="rounded-md border border-border overflow-hidden">
            <table class="w-full text-sm">
              <tbody>
                <For each={w.canceledTargets()}>
                  {(target) => (
                    <tr class="border-t border-border first:border-t-0 opacity-60">
                      <td class="px-3 py-2 font-medium text-base-content">{target.nodeName}</td>
                      <td class="px-3 py-2 text-muted font-mono text-xs">{target.nodeIP}</td>
                      <td class="px-3 py-2">
                        <DeployStatusBadge status={target.status} />
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
