import { For, Show, createMemo, type Component } from 'solid-js';
import { X } from 'lucide-solid';
import { Dialog } from '@/components/shared/Dialog';
import { Button, CommandCopyButton } from '@/components/shared/Button';
import { copyToClipboard } from '@/utils/clipboard';
import { notificationStore } from '@/stores/notifications';
import {
  getUnifiedAgentClipboardCopyErrorMessage,
  getUnifiedAgentClipboardCopySuccessMessage,
} from '@/utils/unifiedAgentInventoryPresentation';
import type { InfrastructureAgentUpdateTarget } from './infrastructureAgentUpdateCommandsModel';
import { useInfrastructureOperationsContext } from './useInfrastructureOperationsState';

interface InfrastructureAgentUpdatesDialogProps {
  isOpen: boolean;
  targets: readonly InfrastructureAgentUpdateTarget[];
  onClose: () => void;
}

export const InfrastructureAgentUpdatesDialog: Component<InfrastructureAgentUpdatesDialogProps> = (
  props,
) => {
  const operations = useInfrastructureOperationsContext();
  const targetCount = createMemo(() => props.targets.length);
  const tokenGatedTargetCount = createMemo(
    () =>
      props.targets.filter((target) =>
        operations.getAgentConnectionUpgradeCommandRequiresToken(target.connection),
      ).length,
  );
  const hasTokenGatedTargets = createMemo(() => tokenGatedTargetCount() > 0);
  const targetSummary = createMemo(() =>
    targetCount() === 1 ? '1 agent needs an update' : `${targetCount()} agents need updates`,
  );
  const commandReadyForTarget = (target: InfrastructureAgentUpdateTarget) =>
    !operations.getAgentConnectionUpgradeCommandRequiresToken(target.connection) ||
    operations.commandsUnlocked();

  const copyCommand = async (command: string) => {
    const success = await copyToClipboard(command);
    if (success) {
      notificationStore.success(getUnifiedAgentClipboardCopySuccessMessage());
      return;
    }
    notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
  };

  return (
    <Dialog isOpen={props.isOpen} onClose={props.onClose} ariaLabel="Update Pulse Agents">
      <div class="flex h-full min-h-0 flex-col">
        <div class="flex items-start justify-between gap-4 border-b border-border bg-surface-alt px-4 py-4 sm:px-6">
          <div class="space-y-1">
            <h2 class="text-base font-semibold text-base-content">Update Pulse Agents</h2>
            <p class="text-sm text-muted">
              Copy the update command for each host that is behind the current Pulse Agent target.
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="iconMd"
            onClick={props.onClose}
            aria-label="Close agent update commands"
          >
            <X class="h-4 w-4" />
          </Button>
        </div>

        <div class="min-h-0 flex-1 space-y-4 overflow-y-auto p-4 sm:p-6">
          <Show
            when={targetCount() > 0}
            fallback={
              <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-100">
                Pulse does not currently see any agents behind the target version.
              </div>
            }
          >
            <div class="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100">
              <div class="font-semibold">{targetSummary()}</div>
              <p class="mt-1 text-xs leading-5 text-amber-800 dark:text-amber-200">
                Run these commands on the affected hosts. The installer reuses the existing agent
                connection state where supported, verifies the matching agent binary, preserves host
                identity, and restarts the service after the update.
              </p>
            </div>

            <Show
              when={
                hasTokenGatedTargets() &&
                operations.requiresToken() &&
                !operations.commandsUnlocked()
              }
            >
              <section class="space-y-3 rounded-md border border-blue-200 bg-blue-50 px-4 py-3 dark:border-blue-800 dark:bg-blue-950/30">
                <div class="space-y-1">
                  <h3 class="text-sm font-semibold text-blue-900 dark:text-blue-100">
                    Generate update token
                  </h3>
                  <p class="text-xs leading-5 text-blue-800 dark:text-blue-200">
                    {tokenGatedTargetCount() === 1
                      ? 'One target still needs a scoped install token before Pulse can show its update command.'
                      : `${tokenGatedTargetCount()} targets still need a scoped install token before Pulse can show their update commands.`}
                  </p>
                </div>
                <div class="flex flex-col gap-2 sm:flex-row">
                  <input
                    type="text"
                    value={operations.tokenName()}
                    onInput={(event) => operations.setTokenName(event.currentTarget.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' && !operations.isGeneratingToken()) {
                        void operations.handleGenerateToken();
                      }
                    }}
                    placeholder="Token name (optional)"
                    class="min-h-10 flex-1 rounded-md border border-blue-200 bg-surface px-3 py-2 text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-200 dark:border-blue-700 dark:bg-blue-950 dark:focus:ring-blue-900"
                  />
                  <button
                    type="button"
                    onClick={() => void operations.handleGenerateToken()}
                    disabled={operations.isGeneratingToken()}
                    class="inline-flex min-h-10 items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {operations.isGeneratingToken() ? 'Generating...' : 'Generate token'}
                  </button>
                </div>
              </section>
            </Show>

            <Show
              when={
                hasTokenGatedTargets() &&
                !operations.requiresToken() &&
                !operations.commandsUnlocked()
              }
            >
              <section class="space-y-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100">
                <p class="text-xs leading-5">
                  Tokens are optional on this Pulse instance. Confirm to generate update commands
                  without embedding a token.
                </p>
                <button
                  type="button"
                  onClick={operations.acknowledgeNoToken}
                  disabled={operations.confirmedNoToken()}
                  class="inline-flex min-h-9 items-center justify-center rounded-md border border-amber-300 bg-surface px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-default disabled:opacity-60"
                >
                  {operations.confirmedNoToken() ? 'No token confirmed' : 'Confirm without token'}
                </button>
              </section>
            </Show>

            <div class="space-y-3">
              <For each={props.targets}>
                {(target) => {
                  const command = () =>
                    operations.getAgentConnectionUpgradeCommand(
                      target.connection,
                      target.installFlags,
                    );

                  return (
                    <section class="space-y-3 rounded-md border border-border bg-surface px-4 py-3">
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                        <div class="space-y-1">
                          <h3 class="text-sm font-semibold text-base-content">
                            {target.displayName}
                          </h3>
                          <p class="text-xs text-muted">
                            {target.contextLabel}
                            <Show when={target.currentVersion && target.expectedVersion}>
                              {' '}
                              · {target.currentVersion} -&gt; {target.expectedVersion}
                            </Show>
                          </p>
                        </div>
                        <span class="inline-flex w-fit items-center rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-900 dark:text-amber-200">
                          Update available
                        </span>
                      </div>

                      <Show
                        when={commandReadyForTarget(target)}
                        fallback={
                          <div class="rounded-md border border-border bg-surface-alt px-3 py-3 text-xs text-muted">
                            Generate a token to unlock the copyable update command.
                          </div>
                        }
                      >
                        <div class="relative">
                          <CommandCopyButton
                            onClick={() => void copyCommand(command())}
                            title="Copy update command"
                            label={`Copy update command for ${target.displayName}`}
                          />
                          <pre class="overflow-x-auto rounded-md bg-base p-3 pr-12 text-xs text-base-content">
                            <code>{command()}</code>
                          </pre>
                        </div>
                      </Show>
                    </section>
                  );
                }}
              </For>
            </div>
          </Show>
        </div>
      </div>
    </Dialog>
  );
};
