import { For, Show, createEffect, createMemo, createSignal, type Component } from 'solid-js';
import { ChevronDown, ChevronLeft, ChevronRight, RefreshCw } from 'lucide-solid';
import { Button, CommandCopyButton } from '@/components/shared/Button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { copyToClipboard } from '@/utils/clipboard';
import { notificationStore } from '@/stores/notifications';
import {
  getUnifiedAgentClipboardCopyErrorMessage,
  getUnifiedAgentClipboardCopySuccessMessage,
} from '@/utils/unifiedAgentInventoryPresentation';
import {
  formatInfrastructureAgentDoctorReport,
  summarizeInfrastructureAgentDoctorTargets,
  type InfrastructureAgentDoctorStatus,
  type InfrastructureAgentDoctorTarget,
} from './infrastructureAgentUpdateCommandsModel';
import { buildInfrastructureWorkspacePath } from './infrastructureWorkspaceModel';
import { useInfrastructureOperationsContext } from './useInfrastructureOperationsState';

interface InfrastructureAgentDoctorPageProps {
  targets: readonly InfrastructureAgentDoctorTarget[];
  diagnosticsLoading?: boolean;
  diagnosticsError?: unknown;
  onRetryDiagnostics?: () => void;
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

export const InfrastructureAgentDoctorPage: Component<InfrastructureAgentDoctorPageProps> = (
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

  const copyReport = async (reportTargets: readonly InfrastructureAgentDoctorTarget[]) => {
    const success = await copyToClipboard(formatInfrastructureAgentDoctorReport(reportTargets));
    if (success) {
      notificationStore.success('Diagnostic report copied');
      return;
    }
    notificationStore.error(getUnifiedAgentClipboardCopyErrorMessage());
  };

  const [statusFilter, setStatusFilter] = createSignal<InfrastructureAgentDoctorStatus | null>(
    null,
  );
  // A filtered status can empty out as agents recover, taking its chip with
  // it; clear the filter then so the table never dead-ends under a filter the
  // user can no longer see or unset.
  createEffect(() => {
    const filter = statusFilter();
    if (filter && !props.targets.some((target) => target.status === filter)) {
      setStatusFilter(null);
    }
  });
  const visibleTargets = createMemo(() => {
    const filter = statusFilter();
    if (!filter) return props.targets;
    return props.targets.filter((target) => target.status === filter);
  });

  // A lone target (the common case when a platform page deep-links one stale
  // agent) starts expanded so its diagnosis and update command are immediately
  // visible; larger fleets start collapsed and expand per row.
  const [expansionOverrides, setExpansionOverrides] = createSignal<ReadonlyMap<string, boolean>>(
    new Map(),
  );
  const isExpanded = (target: InfrastructureAgentDoctorTarget) =>
    expansionOverrides().get(target.connectionId) ?? props.targets.length === 1;
  const toggleExpanded = (target: InfrastructureAgentDoctorTarget) => {
    setExpansionOverrides((previous) => {
      const next = new Map(previous);
      next.set(target.connectionId, !isExpanded(target));
      return next;
    });
  };

  return (
    <div class="space-y-4">
      <div class="space-y-1">
        <a
          href={buildInfrastructureWorkspacePath()}
          class="inline-flex items-center gap-1 text-sm text-muted transition-colors hover:text-base-content"
        >
          <ChevronLeft class="h-4 w-4" aria-hidden="true" />
          Infrastructure
        </a>
        <h2 class="text-base font-semibold text-base-content">Agent Doctor</h2>
        <p class="text-sm text-muted">
          Diagnose fleet connectivity, versions, identity, profiles, and removed-agent state.
        </p>
      </div>

      <Show when={props.diagnosticsError}>
        <section class="flex flex-col gap-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div class="font-semibold">Structured diagnostics are temporarily unavailable</div>
            <p class="mt-1 text-xs leading-5 text-amber-800 dark:text-amber-200">
              Showing the last known connection-ledger assessment. Profile and removed-agent details
              may be incomplete.
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
        <section
          aria-label="Agent Doctor summary"
          class="flex flex-wrap items-center justify-between gap-2"
        >
          <div class="flex flex-wrap items-center gap-2">
            <For each={summaryChips()}>
              {(chip) => (
                <button
                  type="button"
                  aria-pressed={statusFilter() === chip.status}
                  onClick={() =>
                    setStatusFilter((current) => (current === chip.status ? null : chip.status))
                  }
                  class={`inline-flex cursor-pointer items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium transition-shadow ${STATUS_PRESENTATION[chip.status].badgeClass} ${
                    statusFilter() === chip.status
                      ? 'ring-2 ring-blue-500 ring-offset-1 ring-offset-surface'
                      : ''
                  }`}
                >
                  <span class="font-semibold">{chip.count}</span>
                  {STATUS_PRESENTATION[chip.status].label}
                </button>
              )}
            </For>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void copyReport(visibleTargets())}
          >
            Copy diagnostic report
          </Button>
        </section>

        <Show when={anyTargetNeedsUpdate()}>
          <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-xs leading-5 text-blue-900 dark:border-blue-800 dark:bg-blue-950/30 dark:text-blue-100">
            Update commands are host-local: copy one to the affected machine to update its Pulse
            Agent from this server. They do not update the Pulse server runtime and Pulse does not
            run them remotely.
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

        <div class="rounded-md border border-border bg-surface">
          <Table class="w-full min-w-[760px] table-fixed text-sm">
            <TableHeader class="bg-surface-alt/60">
              <TableRow>
                <TableHead class="w-[26%] py-1.5 pl-3 pr-3 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  Agent
                </TableHead>
                <TableHead class="w-[15%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  System
                </TableHead>
                <TableHead class="w-[15%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  Status
                </TableHead>
                <TableHead class="w-[12%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  Reported
                </TableHead>
                <TableHead class="w-[12%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  Target
                </TableHead>
                <TableHead class="w-[20%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  Last seen
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <For each={visibleTargets()}>
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
                  const expanded = () => isExpanded(target);

                  return (
                    <>
                      <TableRow class="cursor-pointer" onClick={() => toggleExpanded(target)}>
                        <TableCell class="py-1.5 pl-3 pr-3">
                          <div class="flex items-center gap-1.5">
                            <button
                              type="button"
                              class="inline-flex items-center rounded p-0.5 text-muted transition-colors hover:bg-surface-alt hover:text-base-content"
                              aria-expanded={expanded()}
                              aria-label={`${expanded() ? 'Hide' : 'Show'} details for ${target.displayName}`}
                              onClick={(event) => {
                                event.stopPropagation();
                                toggleExpanded(target);
                              }}
                            >
                              <Show
                                when={expanded()}
                                fallback={<ChevronRight class="h-3.5 w-3.5" />}
                              >
                                <ChevronDown class="h-3.5 w-3.5" />
                              </Show>
                            </button>
                            <span class="truncate text-sm font-medium text-base-content">
                              {target.displayName}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class="px-3 py-1.5">
                          <span class="truncate text-xs text-muted">{target.contextLabel}</span>
                        </TableCell>
                        <TableCell class="px-3 py-1.5">
                          <span
                            class={`inline-flex w-fit items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${status().badgeClass}`}
                          >
                            {status().label}
                          </span>
                        </TableCell>
                        <TableCell class="px-3 py-1.5 text-xs text-base-content">
                          {target.currentVersion || '—'}
                        </TableCell>
                        <TableCell class="px-3 py-1.5 text-xs text-base-content">
                          {target.expectedVersion || '—'}
                        </TableCell>
                        <TableCell class="px-3 py-1.5 text-xs text-muted">
                          {lastSeen() || '—'}
                        </TableCell>
                      </TableRow>

                      <Show when={expanded()}>
                        <InlineDetailTableRow colspan={6}>
                          <div class="space-y-3 whitespace-normal">
                            <p class="text-xs text-muted">
                              {target.connectionId}
                              <Show when={target.updaterLabel}>
                                {' '}
                                · Updater: {target.updaterLabel}
                              </Show>
                              <Show when={target.profileLabel}>
                                {' '}
                                · Profile: {target.profileLabel}
                                <Show when={target.profileVersionLabel}>
                                  {' '}
                                  ({target.profileVersionLabel})
                                </Show>
                              </Show>
                            </p>

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
                                    <li class="rounded-md border border-border-subtle bg-surface px-3 py-2">
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
                                <div class="rounded-md border border-border bg-surface px-3 py-2 text-xs">
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
                                  <div class="rounded-md border border-border bg-surface px-3 py-3 text-xs text-muted">
                                    {target.commandBlockedReason}
                                  </div>
                                }
                              >
                                <Show
                                  when={commandReadyForTarget(target)}
                                  fallback={
                                    <div class="rounded-md border border-border bg-surface px-3 py-3 text-xs text-muted">
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

                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              onClick={() => void copyReport([target])}
                            >
                              Copy diagnostic report for {target.displayName}
                            </Button>
                          </div>
                        </InlineDetailTableRow>
                      </Show>
                    </>
                  );
                }}
              </For>
            </TableBody>
          </Table>
        </div>
      </Show>
    </div>
  );
};
