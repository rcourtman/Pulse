import { Component, For, Show, createSignal, onCleanup } from 'solid-js';
import type { DeployWizardState } from '@/hooks/useDeployWizard';
import { NodesAPI } from '@/api/nodes';
import { copyToClipboard } from '@/utils/clipboard';
import { getDeployInstallCommandLoadingState } from '@/utils/deployFlowPresentation';
import { DeployStatusBadge } from './DeployStatusBadge';
import { ErrorDetail } from './ErrorDetail';
import CheckCircleIcon from 'lucide-solid/icons/check-circle-2';
import XCircleIcon from 'lucide-solid/icons/x-circle';
import AlertCircleIcon from 'lucide-solid/icons/alert-circle';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import CopyIcon from 'lucide-solid/icons/copy';
import CheckIcon from 'lucide-solid/icons/check';

interface ResultsStepProps {
  wizard: DeployWizardState;
}

export const ResultsStep: Component<ResultsStepProps> = (props) => {
  const w = props.wizard;
  const [manualInstallOpen, setManualInstallOpen] = createSignal(false);
  const [installCommand, setInstallCommand] = createSignal('');
  const [installCommandLoading, setInstallCommandLoading] = createSignal(false);
  const [installCommandError, setInstallCommandError] = createSignal('');
  const [copied, setCopied] = createSignal(false);

  let commandFetched = false;
  let copyTimer: ReturnType<typeof setTimeout> | undefined;
  onCleanup(() => {
    if (copyTimer !== undefined) clearTimeout(copyTimer);
  });

  async function fetchInstallCommand() {
    if (commandFetched) return;
    commandFetched = true;
    setInstallCommandLoading(true);
    setInstallCommandError('');
    try {
      const resp = await NodesAPI.getAgentInstallCommand({ type: 'pve', enableProxmox: true });
      if (!resp.command) {
        setInstallCommandError('Server returned an empty install command');
        commandFetched = false;
        return;
      }
      setInstallCommand(resp.command);
    } catch (err) {
      setInstallCommandError(err instanceof Error ? err.message : 'Failed to load install command');
      commandFetched = false;
    } finally {
      setInstallCommandLoading(false);
    }
  }

  async function handleCopy() {
    const cmd = installCommand();
    if (!cmd) return;
    const ok = await copyToClipboard(cmd);
    if (ok) {
      setCopied(true);
      if (copyTimer !== undefined) clearTimeout(copyTimer);
      copyTimer = setTimeout(() => setCopied(false), 2000);
    }
  }

  function handleAccordionToggle() {
    const opening = !manualInstallOpen();
    setManualInstallOpen(opening);
    if (opening) void fetchInstallCommand();
  }

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
                      <td class="px-3 py-2">
                        <ErrorDetail message={target.errorMessage} />
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
            onClick={handleAccordionToggle}
            aria-expanded={manualInstallOpen()}
            aria-controls="manual-install-content"
            class="flex items-center gap-1 text-xs text-muted hover:text-base-content transition-colors"
          >
            <Show when={manualInstallOpen()} fallback={<ChevronRightIcon class="w-3 h-3" />}>
              <ChevronDownIcon class="w-3 h-3" />
            </Show>
            Manual install instructions
          </button>
          <Show when={manualInstallOpen()}>
            <div
              id="manual-install-content"
              class="rounded-md bg-surface-alt p-3 text-xs space-y-2"
            >
              <p class="text-muted">
                For nodes that failed SSH-based deployment, you can install the agent manually by
                SSHing into the node and running:
              </p>
              <Show when={installCommandLoading()}>
                <div class="flex items-center gap-2 py-2 text-muted">
                  <div class="h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent" />
                  {getDeployInstallCommandLoadingState()}
                </div>
              </Show>
              <Show when={installCommandError()}>
                <div role="alert" class="text-red-600 dark:text-red-400">
                  {installCommandError()}
                </div>
              </Show>
              <Show when={installCommand()}>
                <div class="relative">
                  {/* prettier-ignore */}
                  <pre class="bg-surface border border-border rounded p-2 pr-10 text-[11px] font-mono text-base-content overflow-x-auto whitespace-pre-wrap break-all">{installCommand()}</pre>
                  <button
                    type="button"
                    onClick={handleCopy}
                    class="absolute top-1.5 right-1.5 rounded p-1 text-muted hover:text-base-content hover:bg-surface-hover transition-colors"
                    aria-label={copied() ? 'Copied' : 'Copy to clipboard'}
                  >
                    <Show when={copied()} fallback={<CopyIcon class="w-3.5 h-3.5" />}>
                      <CheckIcon class="w-3.5 h-3.5 text-emerald-500" />
                    </Show>
                  </button>
                </div>
              </Show>
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
