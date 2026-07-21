import { For, Show, createMemo, type Component } from 'solid-js';
import { RefreshCw, X } from 'lucide-solid';
import { Dialog } from '@/components/shared/Dialog';
import { Button, CommandCopyButton } from '@/components/shared/Button';
import { copyToClipboard } from '@/utils/clipboard';
import { notificationStore } from '@/stores/notifications';
import {
  getUnifiedAgentClipboardCopyErrorMessage,
  getUnifiedAgentClipboardCopySuccessMessage,
} from '@/utils/unifiedAgentInventoryPresentation';
import {
  summarizeInfrastructureAgentDoctorTargets,
  type InfrastructureAgentDoctorStatus,
  type InfrastructureAgentDoctorTarget,
} from './infrastructureAgentUpdateCommandsModel';
import { useInfrastructureOperationsContext } from './useInfrastructureOperationsState';

interface InfrastructureAgentUpdatesDialogProps {
  isOpen: boolean;
  targets: readonly InfrastructureAgentDoctorTarget[];
  diagnosticsLoading?: boolean;
  diagnosticsError?: unknown;
  onRetryDiagnostics?: () => void;
  onClose: () => void;
}

const STATUS_PRESENTATION: Record<
  InfrastructureAgentDoctorStatus,
  { label: string; badgeClass: string }
> = {
  healthy: {
    label: 'Healthy',
    badgeClass: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-200',
  },
  waiting: {
    label: 'Waiting for updater',
    badgeClass: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  },
  warning: {
    label: 'Needs attention',
    badgeClass: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
  },
  critical: {
    label: 'Critical',
    badgeClass: 'bg-rose-100 text-rose-800 dark:bg-rose-900 dark:text-rose-200',
  },
  removed: {
    label: 'Removed',
    badgeClass: 'bg-surface-alt text-muted',
  },
  unknown: {
    label: 'Unknown',
    badgeClass: 'bg-surface-alt text-base-content',
  },
};

const formatLastSeen = (value?: number | string | null): string | undefined => {
  if (!value) return undefined;
  const timestamp = typeof value === 'number' ? value : Date.parse(value);
  if (!Number.isFinite(timestamp)) return undefined;
  return new Date(timestamp).toLocaleString();
};

export const InfrastructureAgentUpdatesDialog: Component<InfrastructureAgentUpdatesDialogProps> = (
  props,
) => {
  const operations = useInfrastructureOperationsContext();
  const summary = createMemo(() => summarizeInfrastructureAgentDoctorTargets(props.targets));
  const anyTargetNeedsUpdate = createMemo(() => props.targets.some((target) => target.needsUpdate));
  const summaryChips = createMemo(() => {
    const counts = summary();
    const order: { status: InfrastructureAgentDoctorStatus; count: number }[] = [
      { status: 'critical', count: counts.critical },
      { status: 'warning', count: counts.warning },
      { status: 'waiting', count: counts.waiting },
      { status: 'unknown', count: counts.unknown },
      { status: 'removed', count: counts.removed },
      { status: 'healthy', count: counts.healthy },
    ];
    return order.filter((entry) => entry.count > 0);
  });
  const commandTargets = createMemo(() =>
    props.targets.filter(
      (target) =>
        target.needsUpdate &&
        Boolean(target.connection) &&
        Boolean(target.commandPlatform) &&
        !target.commandBlockedReason,
    ),
  );
  const tokenGatedTargetCount = createMemo(
    () =>
      commandTargets().filter(
        (target) =>
          target.connection &&
          operations.getAgentConnectionUpgradeCommandRequiresToken(
            target.connection,
            target.commandPlatform ?? undefined,
          ),
      ).length,
  );
  const commandReadyForTarget = (target: InfrastructureAgentDoctorTarget) =>
    Boolean(target.connection && target.commandPlatform && !target.commandBlockedReason) &&
    (!operations.getAgentConnectionUpgradeCommandRequiresToken(
      target.connection!,
      target.commandPlatform!,
    ) ||
      operations.commandsUnlocked());

  const copyCommand = async (command: string) => {
    const success = await copyToClipboard(command);
    if (success) {
      notificationStore.success(getUnifiedAgentClipboardCopySuccessMessage());
      return;
    }
    notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
  };

  return (
    <Dialog isOpen={props.isOpen} onClose={props.onClose} ariaLabel="Agent Doctor">
      <div class="flex h-full min-h-0 flex-col">
        <div class="flex items-start justify-between gap-4 border-b border-border bg-surface-alt px-4 py-4 sm:px-6">
          <div class="space-y-1">
            <h2 class="text-base font-semibold text-base-content">Agent Doctor</h2>
            <p class="text-sm text-muted">
              Diagnose fleet connectivity, versions, identity, profiles, and removed-agent state.
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="iconMd"
            onClick={props.onClose}
            aria-label="Close Agent Doctor"
          >
            <X class="h-4 w-4" />
          </Button>
        </div>

        <div class="min-h-0 flex-1 space-y-4 overflow-y-auto p-4 sm:p-6">
          <Show when={props.diagnosticsError}>
            <section class="flex flex-col gap-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <div class="font-semibold">Structured diagnostics are temporarily unavailable</div>
                <p class="mt-1 text-xs leading-5 text-amber-800 dark:text-amber-200">
                  Showing the last known connection-ledger assessment. Profile and removed-agent
                  details may be incomplete.
                </p>
              </div>
              <Show when={props.onRetryDiagnostics}>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  class="gap-2 self-start"
                  onClick={() => props.onRetryDiagnostics?.()}
                >
                  <RefreshCw class="h-3.5 w-3.5" aria-hidden="true" />
                  Retry
                </Button>
              </Show>
            </section>
          </Show>

          <Show when={props.diagnosticsLoading && props.targets.length === 0}>
            <div class="rounded-md border border-border bg-surface-alt px-4 py-3 text-sm text-muted">
              Checking agent fleet health…
            </div>
          </Show>

          <Show
            when={props.targets.length > 0}
            fallback={
              <Show when={!props.diagnosticsLoading}>
                <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-100">
                  No Pulse Agent connections are currently in scope.
                </div>
              </Show>
            }
          >
            <section aria-label="Agent Doctor summary" class="flex flex-wrap items-center gap-2">
              <For each={summaryChips()}>
                {(chip) => (
                  <span
                    class={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium ${STATUS_PRESENTATION[chip.status].badgeClass}`}
                  >
                    <span class="font-semibold">{chip.count}</span>
                    {STATUS_PRESENTATION[chip.status].label}
                  </span>
                )}
              </For>
            </section>

            <Show when={anyTargetNeedsUpdate()}>
              <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-xs leading-5 text-blue-900 dark:border-blue-800 dark:bg-blue-950/30 dark:text-blue-100">
                Update commands are host-local: copy one to the affected machine to update its Pulse
                Agent from this server. They do not update the Pulse server runtime and Pulse does
                not run them remotely.
              </div>
            </Show>

            <Show
              when={
                tokenGatedTargetCount() > 0 &&
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
                      ? 'One Windows repair needs a scoped install token before Pulse can show its command.'
                      : `${tokenGatedTargetCount()} Windows repairs need a scoped install token before Pulse can show their commands.`}
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
                tokenGatedTargetCount() > 0 &&
                !operations.requiresToken() &&
                !operations.commandsUnlocked()
              }
            >
              <section class="space-y-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100">
                <p class="text-xs leading-5">
                  Tokens are optional on this Pulse instance. Confirm to generate Windows update
                  commands without embedding a token.
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
                  const status = () => STATUS_PRESENTATION[target.status];
                  const lastSeen = () => formatLastSeen(target.lastSeen);
                  const command = () =>
                    target.connection && target.commandPlatform
                      ? operations.getAgentConnectionUpgradeCommand(
                          target.connection,
                          target.installFlags,
                          target.commandPlatform,
                        )
                      : '';
                  const otherRepairs = () =>
                    (target.diagnostic?.repairActions ?? []).filter(
                      (action) => action.code !== 'copy_upgrade_command',
                    );

                  return (
                    <section class="space-y-3 rounded-md border border-border bg-surface px-4 py-3">
                      <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                        <div class="space-y-1">
                          <h3 class="text-sm font-semibold text-base-content">
                            {target.displayName}
                          </h3>
                          <p class="text-xs text-muted">
                            {target.contextLabel} · {target.connectionId}
                            <Show when={lastSeen()}> · Last seen {lastSeen()}</Show>
                          </p>
                        </div>
                        <span
                          class={`inline-flex w-fit items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${status().badgeClass}`}
                        >
                          {status().label}
                        </span>
                      </div>

                      <Show when={target.currentVersion || target.expectedVersion}>
                        <div class="grid gap-2 text-xs sm:grid-cols-2">
                          <Show when={target.currentVersion}>
                            <Detail label="Reported agent" value={target.currentVersion!} />
                          </Show>
                          <Show when={target.expectedVersion}>
                            <Detail label="Supported target" value={target.expectedVersion!} />
                          </Show>
                        </div>
                      </Show>

                      <Show when={target.updaterLabel}>
                        <Detail label="Agent updater" value={target.updaterLabel!} />
                      </Show>

                      <Show when={target.profileLabel}>
                        <div class="rounded-md border border-border-subtle bg-surface-alt px-3 py-2 text-xs text-base-content">
                          <span class="font-medium">Profile: {target.profileLabel}</span>
                          <Show when={target.profileVersionLabel}>
                            <span class="text-muted"> · {target.profileVersionLabel}</span>
                          </Show>
                        </div>
                      </Show>

                      <Show
                        when={target.reasons.length > 0}
                        fallback={
                          <p class="text-xs text-muted">
                            {target.status === 'healthy'
                              ? 'No fleet-health issues detected.'
                              : 'Pulse has not received a structured diagnostic for this agent yet. This usually clears after its next report.'}
                          </p>
                        }
                      >
                        <ul class="space-y-2">
                          <For each={target.reasons}>
                            {(reason) => (
                              <li class="rounded-md border border-border-subtle bg-surface-alt px-3 py-2">
                                <div class="text-xs font-medium text-base-content">
                                  {reason.message}
                                </div>
                                <Show when={reason.evidence?.length}>
                                  <div class="mt-1 text-[11px] leading-4 text-muted">
                                    {(reason.evidence ?? []).join(' · ')}
                                  </div>
                                </Show>
                              </li>
                            )}
                          </For>
                        </ul>
                      </Show>

                      <Show when={target.evidence.length > 0}>
                        <details class="text-xs text-muted">
                          <summary class="cursor-pointer font-medium text-base-content">
                            Identity evidence
                          </summary>
                          <ul class="mt-2 list-disc space-y-1 pl-5">
                            <For each={target.evidence}>{(item) => <li>{item}</li>}</For>
                          </ul>
                        </details>
                      </Show>

                      <For each={otherRepairs()}>
                        {(repair) => (
                          <div class="rounded-md border border-border bg-surface-alt px-3 py-2 text-xs">
                            <div class="font-medium text-base-content">{repair.label}</div>
                            <div class="mt-0.5 text-muted">{repair.description}</div>
                            <Show when={repair.scope}>
                              <div class="mt-1 text-[11px] text-muted">
                                Required scope: {repair.scope}
                              </div>
                            </Show>
                          </div>
                        )}
                      </For>

                      <Show when={target.needsUpdate}>
                        <Show
                          when={!target.commandBlockedReason}
                          fallback={
                            <div class="rounded-md border border-border bg-surface-alt px-3 py-3 text-xs text-muted">
                              {target.commandBlockedReason}
                            </div>
                          }
                        >
                          <Show
                            when={commandReadyForTarget(target)}
                            fallback={
                              <div class="rounded-md border border-border bg-surface-alt px-3 py-3 text-xs text-muted">
                                Generate a token to unlock this host-local update command.
                              </div>
                            }
                          >
                            <div class="relative">
                              <CommandCopyButton
                                onClick={() => void copyCommand(command())}
                                title="Copy host-local agent update command"
                                label={`Copy update command for ${target.displayName}`}
                              />
                              <pre class="overflow-x-auto rounded-md bg-base p-3 pr-12 text-xs text-base-content">
                                <code>{command()}</code>
                              </pre>
                            </div>
                          </Show>
                        </Show>
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

const Detail: Component<{ label: string; value: string }> = (props) => (
  <div class="rounded-md border border-border-subtle bg-surface-alt px-3 py-2">
    <div class="text-[11px] text-muted">{props.label}</div>
    <div class="mt-0.5 font-medium text-base-content">{props.value}</div>
  </div>
);
