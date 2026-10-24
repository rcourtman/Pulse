import { For, Show, createEffect, createMemo, createSignal, type Component } from 'solid-js';
import { ChevronLeft, RefreshCw } from 'lucide-solid';
import { Button, CommandCopyButton } from '@/components/shared/Button';
import { FormSelect } from '@/components/shared/FormSelect';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import {
  DetailSectionTable,
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { SummaryRowActionButton } from '@/components/shared/SummaryRowActionButton';
import { TechnicalDetailsDisclosure } from '@/components/shared/TechnicalDetailsDisclosure';
import type { StatusIndicatorVariant } from '@/utils/status';
import { copyToClipboard } from '@/utils/clipboard';
import { formatRelativeTime } from '@/utils/format';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { notificationStore } from '@/stores/notifications';
import { MonitoringAPI } from '@/api/monitoring';
import { AgentDiagnosticsAPI } from '@/api/agentDiagnostics';
import {
  getUnifiedAgentClipboardCopyErrorMessage,
  getUnifiedAgentClipboardCopySuccessMessage,
} from '@/utils/unifiedAgentInventoryPresentation';
import {
  formatInfrastructureAgentDoctorReport,
  buildActionRunnerInstallCommand,
  buildActionRunnerTokenFileCommand,
  buildSafeCollectorApplyCommand,
  buildSafeCollectorInspectCommand,
  getInfrastructureAgentDoctorUninstallHandoff,
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

// The row badge and the drawer's status dot read from this one table.
const STATUS_PRESENTATION: Record<
  InfrastructureAgentDoctorStatus,
  { label: string; badgeClass: string; variant: StatusIndicatorVariant }
> = {
  healthy: {
    label: 'Healthy',
    variant: 'success',
    badgeClass: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/25 dark:text-emerald-200',
  },
  waiting: {
    variant: 'info',
    label: 'Waiting for updater',
    badgeClass: 'bg-blue-100 text-blue-800 dark:bg-blue-900/25 dark:text-blue-200',
  },
  warning: {
    variant: 'warning',
    label: 'Needs attention',
    badgeClass: 'bg-amber-100 text-amber-800 dark:bg-amber-900/25 dark:text-amber-200',
  },
  critical: {
    variant: 'danger',
    label: 'Critical',
    badgeClass: 'bg-rose-100 text-rose-800 dark:bg-rose-900/25 dark:text-rose-200',
  },
  removed: {
    variant: 'muted',
    label: 'Removed',
    badgeClass: 'bg-surface-alt text-muted',
  },
  unknown: {
    variant: 'muted',
    label: 'Unknown',
    badgeClass: 'bg-surface-alt text-base-content',
  },
};

const doctorDetailRowId = (target: InfrastructureAgentDoctorTarget): string =>
  `agent-doctor-detail-${target.key.replace(/[^A-Za-z0-9_-]/g, '-')}`;

// The drawer's one-line verdict; the reasons themselves follow as notices.
const doctorTargetSummary = (target: InfrastructureAgentDoctorTarget): string => {
  const count = target.reasons.length;
  if (count > 0) return `${count} ${count === 1 ? 'issue' : 'issues'} to look at`;
  return target.status === 'healthy'
    ? 'No fleet-health issues detected.'
    : 'Pulse has not received a structured diagnostic for this agent yet. This usually clears after its next report.';
};

// Evidence reads "Hostname: host-1"; the label goes in the label column.
const evidenceDetailRow = (item: string) => {
  const separator = item.indexOf(': ');
  return separator > 0
    ? makeDetailRow(item.slice(0, separator), item.slice(separator + 2), { wrap: true })
    : makeDetailRow('Evidence', item, { wrap: true });
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
  const now = useRelativeTimeNow();
  const operations = useInfrastructureOperationsContext();
  const summary = createMemo(() => summarizeInfrastructureAgentDoctorTargets(props.targets));
  const anyTargetNeedsRepair = createMemo(() =>
    props.targets.some((target) => target.needsUpdate || target.needsCredentialRepair),
  );
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
        (target.needsUpdate || target.needsCredentialRepair) &&
        Boolean(target.connection) &&
        Boolean(target.commandPlatform) &&
        !target.commandBlockedReason,
    ),
  );
  const tokenGatedTargets = createMemo(() =>
    commandTargets().filter(
      (target) =>
        target.connection &&
        operations.getAgentConnectionUpgradeCommandRequiresToken(
          target.connection,
          target.commandPlatform ?? undefined,
          target.needsCredentialRepair,
        ),
    ),
  );
  const tokenGatedTargetCount = createMemo(() => tokenGatedTargets().length);
  const [selectedTokenTargetKey, setSelectedTokenTargetKey] = createSignal('');
  const [tokenMintedForTargetKey, setTokenMintedForTargetKey] = createSignal('');
  const selectedTokenTarget = createMemo(
    () =>
      tokenGatedTargets().find((target) => target.key === selectedTokenTargetKey()) ??
      tokenGatedTargets()[0],
  );
  const commandRequiresToken = (target: InfrastructureAgentDoctorTarget) =>
    operations.getAgentConnectionUpgradeCommandRequiresToken(
      target.connection!,
      target.commandPlatform!,
      target.needsCredentialRepair,
    );
  const commandReadyForTarget = (target: InfrastructureAgentDoctorTarget) => {
    if (!target.connection || !target.commandPlatform || target.commandBlockedReason) return false;
    if (!commandRequiresToken(target)) return true;
    if (!operations.commandsUnlocked()) return false;
    return tokenGatedTargetCount() === 1 || tokenMintedForTargetKey() === target.key;
  };

  const generateRepairToken = async () => {
    const target = selectedTokenTarget();
    if (!target) return;
    const previousToken = operations.currentToken();
    await operations.setEnableCommands(Boolean(target.connection?.agentIdentity?.commandsEnabled));
    if (!operations.currentToken() || operations.currentToken() === previousToken) {
      await operations.handleGenerateToken();
    }
    const generatedToken = operations.currentToken();
    if (generatedToken && generatedToken !== previousToken) {
      setTokenMintedForTargetKey(target.key);
      setSelectedTokenTargetKey(target.key);
    }
  };

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

  // One target open at a time, like every other drawer table. A lone target
  // (the common case when a platform page deep-links one stale agent) starts
  // open so its diagnosis and update command are immediately visible; larger
  // fleets start collapsed. `undefined` is that default, `null` is closed.
  const [openTargetId, setOpenTargetId] = createSignal<string | null | undefined>(undefined);
  const [reenrollPendingId, setReenrollPendingId] = createSignal('');
  const [runnerCredentialPendingKey, setRunnerCredentialPendingKey] = createSignal('');
  const [issuedRunnerCredentialKeys, setIssuedRunnerCredentialKeys] = createSignal<
    ReadonlySet<string>
  >(new Set());
  const [runnerCredentialReveal, setRunnerCredentialReveal] = createSignal<{
    targetKey: string;
    token: string;
  } | null>(null);

  const issueActionRunnerCredential = async (target: InfrastructureAgentDoctorTarget) => {
    if (
      !target.actionRunnerCredentialEligible ||
      !target.actionRunnerAgentId ||
      !target.actionRunnerHostname
    ) {
      return;
    }
    setRunnerCredentialReveal(null);
    setRunnerCredentialPendingKey(target.key);
    try {
      const response = await AgentDiagnosticsAPI.issueActionRunnerCredential({
        agentId: target.actionRunnerAgentId,
        hostname: target.actionRunnerHostname,
        name: `${target.displayName} action runner`,
      });
      const normalizedHostname = (value: string) => value.trim().toLowerCase().replace(/\.+$/, '');
      if (
        response.agentId !== target.actionRunnerAgentId ||
        normalizedHostname(response.hostname) !== normalizedHostname(target.actionRunnerHostname) ||
        response.runtimeRole !== 'action-runner' ||
        response.actionCapability !== 'typed_actions.v1' ||
        !response.token
      ) {
        throw new Error(
          'Pulse returned an unexpected action-runner credential identity or authority.',
        );
      }
      setRunnerCredentialReveal({ targetKey: target.key, token: response.token });
      setIssuedRunnerCredentialKeys((previous) => new Set(previous).add(target.key));
      notificationStore.success('Action-runner credential issued. Save it now. It is shown once.');
      try {
        await props.onRetryDiagnostics?.();
      } catch {
        notificationStore.error(
          'The credential was issued, but Pulse could not refresh its runner posture.',
        );
      }
    } catch (error) {
      notificationStore.error(
        error instanceof Error ? error.message : 'Failed to issue action-runner credential.',
      );
    } finally {
      setRunnerCredentialPendingKey('');
    }
  };
  const allowReenroll = async (target: InfrastructureAgentDoctorTarget) => {
    const separator = target.connectionId.indexOf(':');
    const type = separator > 0 ? target.connectionId.slice(0, separator) : '';
    const id = separator > 0 ? target.connectionId.slice(separator + 1) : '';
    if (!id) {
      notificationStore.error('This diagnostic does not include a reconnectable agent ID.');
      return;
    }
    setReenrollPendingId(target.connectionId);
    try {
      if (type === 'docker') {
        await MonitoringAPI.allowDockerRuntimeReenroll(id);
      } else if (type === 'kubernetes') {
        await MonitoringAPI.allowKubernetesClusterReenroll(id);
      } else if (type === 'agent') {
        await MonitoringAPI.allowHostAgentReenroll(id);
      } else {
        throw new Error(`Allow re-enrol is not supported for ${type || 'unknown'} diagnostics.`);
      }
      notificationStore.success(`${target.displayName} can re-enrol on its next report.`);
      await props.onRetryDiagnostics?.();
    } catch (error) {
      notificationStore.error(
        error instanceof Error
          ? error.message
          : `Failed to allow ${target.displayName} to re-enrol.`,
      );
    } finally {
      setReenrollPendingId('');
    }
  };
  const isExpanded = (target: InfrastructureAgentDoctorTarget) => {
    const open = openTargetId();
    return open === undefined ? props.targets.length === 1 : open === target.connectionId;
  };
  const toggleExpanded = (target: InfrastructureAgentDoctorTarget) =>
    setOpenTargetId(isExpanded(target) ? null : target.connectionId);
  const closeTarget = (target: InfrastructureAgentDoctorTarget) => {
    if (isExpanded(target)) setOpenTargetId(null);
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
        <section class="flex flex-col gap-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100 sm:flex-row sm:items-center sm:justify-between">
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
            <div class="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-100">
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

        <Show when={anyTargetNeedsRepair()}>
          <div class="rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-xs leading-5 text-blue-800 dark:border-blue-800 dark:bg-blue-950/30 dark:text-blue-100">
            Update and authentication-repair commands are host-local: copy one to the affected
            machine to repair its Pulse Agent from this server. They do not update the Pulse server
            runtime and Pulse does not run them remotely.
          </div>
        </Show>

        <Show
          when={
            tokenGatedTargetCount() > 0 &&
            operations.requiresToken() &&
            (!operations.commandsUnlocked() || tokenGatedTargetCount() > 1)
          }
        >
          <section class="space-y-3 rounded-md border border-blue-200 bg-blue-50 px-4 py-3 dark:border-blue-800 dark:bg-blue-950/30">
            <div class="space-y-1">
              <h3 class="text-sm font-semibold text-blue-800 dark:text-blue-100">
                Generate update token
              </h3>
              <p class="text-xs leading-5 text-blue-800 dark:text-blue-200">
                {tokenGatedTargetCount() === 1
                  ? 'One repair needs a fresh scoped agent credential before Pulse can show its command.'
                  : `${tokenGatedTargetCount()} repairs need separate scoped credentials. Each credential binds to the first agent that uses it. Select one installation, generate and run its command, then continue with the next.`}
              </p>
            </div>
            <div class="flex flex-col gap-2 sm:flex-row">
              <Show when={tokenGatedTargetCount() > 1}>
                <FormSelect
                  label="Agent installation to repair"
                  fieldClass="flex-1"
                  labelClass="text-xs font-medium text-blue-800 dark:text-blue-100"
                  value={selectedTokenTarget()?.key}
                  onChange={(event) => setSelectedTokenTargetKey(event.currentTarget.value)}
                  selectBaseClass="min-h-10 w-full rounded-md border border-blue-200 bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-200 dark:border-blue-700 dark:bg-blue-950/25 dark:focus:ring-blue-900/25"
                >
                  <For each={tokenGatedTargets()}>
                    {(target) => (
                      <option value={target.key}>
                        {target.displayName} · {target.connectionId}
                        {target.currentVersion ? ` · ${target.currentVersion}` : ''}
                      </option>
                    )}
                  </For>
                </FormSelect>
              </Show>
              <input
                type="text"
                value={operations.tokenName()}
                onInput={(event) => operations.setTokenName(event.currentTarget.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && !operations.isGeneratingToken()) {
                    void generateRepairToken();
                  }
                }}
                placeholder="Token name (optional)"
                class="min-h-10 flex-1 rounded-md border border-blue-200 bg-surface px-3 py-2 text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-200 dark:border-blue-700 dark:bg-blue-950/25 dark:focus:ring-blue-900/25"
              />
              <button
                type="button"
                onClick={() => void generateRepairToken()}
                disabled={operations.isGeneratingToken()}
                class="inline-flex min-h-10 items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {operations.isGeneratingToken()
                  ? 'Generating...'
                  : operations.commandsUnlocked()
                    ? 'Generate fresh token'
                    : 'Generate token'}
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
          <section class="space-y-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-100">
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
          <Table class="w-full min-w-0 table-fixed text-sm">
            <TableHeader class="">
              <TableRow>
                <TableHead class="w-[40%] py-1.5 pl-3 pr-3 text-left text-[11px] font-medium text-muted whitespace-nowrap sm:w-[26%]">
                  Agent
                </TableHead>
                <TableHead class="hidden w-[15%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap sm:table-cell">
                  System
                </TableHead>
                <TableHead class="w-[38%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap sm:w-[20%] lg:w-[15%]">
                  Status
                </TableHead>
                <TableHead class="hidden w-[12%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap sm:table-cell">
                  Reported
                </TableHead>
                <TableHead class="agent-doctor-target-column w-[12%] px-3 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap">
                  Target
                </TableHead>
                <TableHead class="w-[22%] px-2 py-1.5 text-left text-[11px] font-medium text-muted whitespace-nowrap sm:w-[20%] sm:px-3">
                  <span class="platform-table-label-compact">Seen</span>
                  <span class="platform-table-label-full">Last seen</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <For each={visibleTargets()}>
                {(target) => {
                  const status = () => STATUS_PRESENTATION[target.status];
                  const lastSeen = () => formatLastSeen(target.lastSeen);
                  const lastSeenRelative = () =>
                    formatRelativeTime(target.lastSeen ?? undefined, {
                      compact: true,
                      emptyText: '—',
                      now: now(),
                    });
                  const command = () =>
                    target.connection && target.commandPlatform
                      ? operations.getAgentConnectionUpgradeCommand(
                          target.connection,
                          target.installFlags,
                          target.commandPlatform,
                          target.needsCredentialRepair,
                        )
                      : '';
                  const otherRepairs = () =>
                    (target.diagnostic?.repairActions ?? []).filter(
                      (action) =>
                        action.code !== 'copy_upgrade_command' &&
                        action.code !== 'repair_authentication' &&
                        action.code !== 'allow_reenroll',
                    );
                  const canAllowReenroll = () =>
                    (target.diagnostic?.repairActions ?? []).some(
                      (action) => action.code === 'allow_reenroll' && action.supported,
                    );
                  const uninstallHandoff = () =>
                    getInfrastructureAgentDoctorUninstallHandoff(target);
                  const expanded = () => isExpanded(target);
                  const detailRowId = () => doctorDetailRowId(target);
                  const factSections = (): DetailSection[] =>
                    compactDetailSections([
                      {
                        label: 'Agent',
                        rows: compactDetailRows([
                          makeDetailRow('System', target.contextLabel, { wrap: true }),
                          makeDetailRow('Version', target.currentVersion),
                          makeDetailRow('Target version', target.expectedVersion),
                          makeDetailRow(
                            'Last seen',
                            lastSeen() ? `${lastSeen()} (${lastSeenRelative()})` : undefined,
                            { wrap: true },
                          ),
                          makeDetailRow('Updater', target.updaterLabel, { wrap: true }),
                          makeDetailRow(
                            'Profile',
                            target.profileLabel
                              ? [target.profileLabel, target.profileVersionLabel]
                                  .filter(Boolean)
                                  .join(' ')
                              : undefined,
                            { wrap: true },
                          ),
                        ]),
                      },
                      {
                        label: 'Security',
                        rows: compactDetailRows([
                          makeDetailRow('Privilege', target.privilegeLabel, { wrap: true }),
                          target.status !== 'removed'
                            ? makeDetailRow(
                                'Safe collector',
                                target.safeCollector ? 'Confirmed' : 'Not confirmed',
                                { tone: target.safeCollector ? 'success' : 'warning' },
                              )
                            : null,
                          makeDetailRow('Action runner', target.actionRunnerPosture.join(' · '), {
                            wrap: true,
                          }),
                          makeDetailRow(
                            'Runner credential',
                            target.actionRunnerCredentialEligible
                              ? undefined
                              : target.actionRunnerCredentialBlockReason,
                            { wrap: true },
                          ),
                        ]),
                      },
                    ]);
                  const evidenceSections = (): DetailSection[] =>
                    compactDetailSections([
                      {
                        label: 'Connection',
                        rows: compactDetailRows([
                          makeDetailRow('Connection ID', target.connectionId, { wrap: true }),
                        ]),
                      },
                      {
                        label: 'Identity evidence',
                        rows: compactDetailRows(target.evidence.map(evidenceDetailRow)),
                      },
                    ]);

                  return (
                    <>
                      <TableRow class="cursor-pointer" onClick={() => toggleExpanded(target)}>
                        <TableCell class="py-1.5 pl-3 pr-3">
                          <div class="flex items-center gap-1.5">
                            <SummaryRowActionButton
                              kind="disclosure"
                              expanded={expanded()}
                              subjectLabel={`details for ${target.displayName}`}
                              controlsId={detailRowId()}
                              hideWhenRowTappableOnMobile={false}
                              onAction={() => toggleExpanded(target)}
                            />
                            <span class="truncate text-sm font-medium text-base-content">
                              {target.displayName}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell class="hidden px-3 py-1.5 sm:table-cell">
                          <span class="truncate text-xs text-muted">{target.contextLabel}</span>
                        </TableCell>
                        <TableCell class="px-3 py-1.5">
                          <span
                            class={`inline-flex w-fit items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${status().badgeClass}`}
                          >
                            {status().label}
                          </span>
                        </TableCell>
                        <TableCell class="hidden px-3 py-1.5 text-xs text-base-content sm:table-cell">
                          {target.currentVersion || '—'}
                        </TableCell>
                        <TableCell class="agent-doctor-target-column px-3 py-1.5 text-xs text-base-content">
                          {target.expectedVersion || '—'}
                        </TableCell>
                        <TableCell
                          class="px-2 py-1.5 text-xs text-muted sm:px-3"
                          title={lastSeen() || ''}
                        >
                          <span class="platform-table-label-compact">{lastSeenRelative()}</span>
                          <span class="platform-table-label-full">{lastSeen() || '—'}</span>
                        </TableCell>
                      </TableRow>

                      <Show when={expanded()}>
                        <InlineDetailTableRow
                          cellId={detailRowId()}
                          colspan={6}
                          data-agent-doctor-detail-for={target.connectionId}
                        >
                          <InlineDetailPanel
                            testId="agent-doctor-detail"
                            detailFor={target.key}
                            title={target.displayName}
                            status={{ variant: status().variant, label: status().label }}
                            summary={doctorTargetSummary(target)}
                            sections={[]}
                            onClose={() => closeTarget(target)}
                          >
                            <For each={target.reasons}>
                              {(reason) => (
                                <InlineNotice
                                  tone={target.status === 'critical' ? 'danger' : 'warning'}
                                >
                                  <p class="font-medium">{reason.message}</p>
                                  <Show when={reason.evidence?.length}>
                                    <p class="mt-1 text-xs wrap-anywhere">
                                      {(reason.evidence ?? []).join(' · ')}
                                    </p>
                                  </Show>
                                </InlineNotice>
                              )}
                            </For>

                            <Show when={target.needsUpdate || target.needsCredentialRepair}>
                              <Show
                                when={!target.commandBlockedReason}
                                fallback={
                                  <InlineNotice tone="info">
                                    {target.commandBlockedReason}
                                  </InlineNotice>
                                }
                              >
                                <Show
                                  when={commandReadyForTarget(target)}
                                  fallback={
                                    <InlineNotice tone="info">
                                      {commandRequiresToken(target) && tokenGatedTargetCount() > 1
                                        ? 'Select this installation above and generate a separate credential to unlock its host-local repair command.'
                                        : 'Generate a token to unlock this host-local repair command.'}
                                    </InlineNotice>
                                  }
                                >
                                  <div class="relative">
                                    <CommandCopyButton
                                      onClick={() => void copyCommand(command())}
                                      title="Copy host-local agent repair command"
                                      label={`Copy ${target.needsCredentialRepair ? 'authentication repair' : 'update'} command for ${target.displayName}`}
                                    />
                                    <pre
                                      tabIndex={0}
                                      class="overflow-x-auto rounded-md bg-page p-3 pr-12 text-xs text-base-content focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                                    >
                                      <code>{command()}</code>
                                    </pre>
                                    <Show
                                      when={
                                        commandRequiresToken(target) &&
                                        target.commandPlatform !== 'windows'
                                      }
                                    >
                                      <p class="mt-2 text-xs text-muted">
                                        Run this command in a console or SSH session first, then
                                        paste the separately revealed token at the silent Pulse
                                        agent token prompt. Never insert the token into the command.
                                      </p>
                                    </Show>
                                  </div>
                                </Show>
                              </Show>
                            </Show>

                            <Show when={canAllowReenroll()}>
                              <InlineNotice tone="info">
                                <p class="font-medium">Allow re-enrol</p>
                                <p class="mt-0.5">
                                  Clear the removal block so this agent can reconnect on its next
                                  report.
                                </p>
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="sm"
                                  class="mt-2"
                                  disabled={reenrollPendingId() === target.connectionId}
                                  onClick={() => void allowReenroll(target)}
                                >
                                  {reenrollPendingId() === target.connectionId
                                    ? 'Allowing…'
                                    : 'Allow re-enrol'}
                                </Button>
                              </InlineNotice>
                            </Show>

                            <For each={otherRepairs()}>
                              {(repair) => (
                                <InlineNotice tone="info">
                                  <p class="font-medium">{repair.label}</p>
                                  <p class="mt-0.5">{repair.description}</p>
                                  <Show when={repair.scope}>
                                    <p class="mt-1 text-xs">Required scope: {repair.scope}</p>
                                  </Show>
                                </InlineNotice>
                              )}
                            </For>

                            <Show when={uninstallHandoff()}>
                              {(handoff) => (
                                <div class="space-y-2">
                                  <InlineNotice tone="info">
                                    This agent was removed from Pulse, but the agent software may
                                    still be installed on its host. Finish detaching it by running
                                    the uninstall command on the affected host itself. Pulse does
                                    not run commands remotely. Removal uses the host's saved
                                    connection and credentials. Do not generate a new token.
                                  </InlineNotice>
                                  <For each={handoff().commands}>
                                    {(entry) => (
                                      <div>
                                        <div class="mb-1 text-xs font-medium text-base-content">
                                          {entry.label}
                                        </div>
                                        <div class="relative">
                                          <CommandCopyButton
                                            onClick={() =>
                                              void copyCommand(
                                                operations.getPlatformUninstallCommand(
                                                  entry.platform,
                                                  handoff().identity,
                                                ),
                                              )
                                            }
                                            title="Copy host-local agent uninstall command"
                                            label={`Copy ${entry.label} uninstall command for ${target.displayName}`}
                                          />
                                          <pre
                                            tabIndex={0}
                                            class="overflow-x-auto rounded-md bg-page p-3 pr-12 text-xs text-base-content focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                                          >
                                            <code>
                                              {operations.getPlatformUninstallCommand(
                                                entry.platform,
                                                handoff().identity,
                                              )}
                                            </code>
                                          </pre>
                                        </div>
                                      </div>
                                    )}
                                  </For>
                                </div>
                              )}
                            </Show>

                            <DetailSectionTable
                              sections={factSections()}
                              dataTestId="agent-doctor-facts"
                              wrapValues
                            />

                            <Show when={target.safeProfileGuidance}>
                              {(guidance) => <InlineNotice tone="info">{guidance()}</InlineNotice>}
                            </Show>
                            <Show
                              when={
                                !target.safeCollector &&
                                target.status !== 'removed' &&
                                target.commandPlatform === 'linux' &&
                                target.actionRunnerAgentId &&
                                target.actionRunnerHostname
                              }
                            >
                              <div class="space-y-2 text-xs">
                                <div>
                                  <div class="mb-1 font-medium text-base-content">
                                    Inspect current service profile
                                  </div>
                                  <div class="relative">
                                    <CommandCopyButton
                                      onClick={() =>
                                        void copyCommand(
                                          buildSafeCollectorInspectCommand({
                                            pulseUrl: operations.selectedAgentUrl(),
                                            insecure: operations.insecureMode(),
                                            customCaPath: operations.customCaPath(),
                                          }),
                                        )
                                      }
                                      label="Copy safe-profile inspection command"
                                    />
                                    <pre
                                      tabIndex={0}
                                      class="overflow-x-auto rounded-md bg-page p-3 pr-12 text-xs text-base-content focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                                    >
                                      <code>
                                        {buildSafeCollectorInspectCommand({
                                          pulseUrl: operations.selectedAgentUrl(),
                                          insecure: operations.insecureMode(),
                                          customCaPath: operations.customCaPath(),
                                        })}
                                      </code>
                                    </pre>
                                  </div>
                                </div>
                                <div>
                                  <div class="mb-1 font-medium text-base-content">
                                    Apply reviewed safe collector profile
                                  </div>
                                  <div class="relative">
                                    <CommandCopyButton
                                      onClick={() =>
                                        void copyCommand(
                                          buildSafeCollectorApplyCommand({
                                            pulseUrl: operations.selectedAgentUrl(),
                                            insecure: operations.insecureMode(),
                                            customCaPath: operations.customCaPath(),
                                          }),
                                        )
                                      }
                                      label="Copy safe-profile apply command"
                                    />
                                    <pre
                                      tabIndex={0}
                                      class="overflow-x-auto rounded-md bg-page p-3 pr-12 text-xs text-base-content focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                                    >
                                      <code>
                                        {buildSafeCollectorApplyCommand({
                                          pulseUrl: operations.selectedAgentUrl(),
                                          insecure: operations.insecureMode(),
                                          customCaPath: operations.customCaPath(),
                                        })}
                                      </code>
                                    </pre>
                                  </div>
                                </div>
                              </div>
                            </Show>

                            <Show when={target.actionRunnerCredentialEligible}>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                isLoading={runnerCredentialPendingKey() === target.key}
                                disabled={
                                  Boolean(runnerCredentialReveal()) ||
                                  issuedRunnerCredentialKeys().has(target.key)
                                }
                                onClick={() => void issueActionRunnerCredential(target)}
                              >
                                {issuedRunnerCredentialKeys().has(target.key)
                                  ? 'Credential issued for this page session'
                                  : target.actionRunnerCredentialAction === 'rotate'
                                    ? 'Rotate action-runner credential'
                                    : 'Issue one-time action-runner credential'}
                              </Button>
                            </Show>

                            <Show
                              when={
                                runnerCredentialReveal()?.targetKey === target.key
                                  ? runnerCredentialReveal()
                                  : null
                              }
                            >
                              {(reveal) => {
                                const tokenFileCommand = () => buildActionRunnerTokenFileCommand();
                                const installCommand = () =>
                                  buildActionRunnerInstallCommand({
                                    pulseUrl: operations.selectedAgentUrl(),
                                    agentId: target.actionRunnerAgentId!,
                                    hostname: target.actionRunnerHostname!,
                                    insecure: operations.insecureMode(),
                                    customCaPath: operations.customCaPath(),
                                  });
                                return (
                                  <InlineNotice tone="warning">
                                    <div class="space-y-3 text-xs">
                                      <p class="text-sm font-semibold">Save this credential now</p>
                                      <p class="leading-5">
                                        This secret is held only in this page's memory and cannot be
                                        recovered after you clear or leave it. It is never placed in
                                        a URL, diagnostic report, installer command, or process
                                        argument.
                                      </p>
                                      <div>
                                        <div class="mb-1 font-medium">
                                          1. Start a private token prompt
                                        </div>
                                        <div class="relative">
                                          <CommandCopyButton
                                            onClick={() => void copyCommand(tokenFileCommand())}
                                            label="Copy private token-file command"
                                          />
                                          <pre
                                            tabIndex={0}
                                            class="overflow-x-auto rounded-md bg-page p-3 pr-12 text-xs text-base-content focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                                          >
                                            <code>{tokenFileCommand()}</code>
                                          </pre>
                                        </div>
                                      </div>
                                      <div>
                                        <div class="mb-1 font-medium">
                                          2. Copy this credential and paste it at the prompt
                                        </div>
                                        <div class="relative">
                                          <CommandCopyButton
                                            onClick={() => void copyCommand(reveal().token)}
                                            title="Copy one-time action-runner credential"
                                            label={`Copy action-runner credential for ${target.displayName}`}
                                          />
                                          <pre
                                            tabIndex={0}
                                            class="overflow-x-auto rounded-md bg-page p-3 pr-12 text-xs text-base-content focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                                          >
                                            <code>{reveal().token}</code>
                                          </pre>
                                        </div>
                                      </div>
                                      <div>
                                        <div class="mb-1 font-medium">
                                          3. Install the separate runner
                                        </div>
                                        <div class="relative">
                                          <CommandCopyButton
                                            onClick={() => void copyCommand(installCommand())}
                                            label="Copy action-runner installer command"
                                          />
                                          <pre
                                            tabIndex={0}
                                            class="overflow-x-auto rounded-md bg-page p-3 pr-12 text-xs text-base-content focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
                                          >
                                            <code>{installCommand()}</code>
                                          </pre>
                                        </div>
                                      </div>
                                      <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        onClick={() => setRunnerCredentialReveal(null)}
                                      >
                                        Clear credential from this page
                                      </Button>
                                    </div>
                                  </InlineNotice>
                                );
                              }}
                            </Show>

                            <TechnicalDetailsDisclosure
                              title="Details"
                              subtitle="Connection and identity evidence"
                              dataTestId="agent-doctor-evidence"
                              sections={evidenceSections()}
                            />

                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              onClick={() => void copyReport([target])}
                            >
                              Copy diagnostic report for {target.displayName}
                            </Button>
                          </InlineDetailPanel>
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
