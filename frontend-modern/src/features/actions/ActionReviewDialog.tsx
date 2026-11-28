import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  type Component,
} from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import ArrowUpRightIcon from 'lucide-solid/icons/arrow-up-right';
import CheckIcon from 'lucide-solid/icons/check';
import Loader2Icon from 'lucide-solid/icons/loader-2';
import TerminalIcon from 'lucide-solid/icons/terminal';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { SecurityAPI } from '@/api/security';
import { Button, ButtonLink } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { FormTextarea } from '@/components/shared/FormTextarea';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { notificationStore } from '@/stores/notifications';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import type { ActionDetailResponse, ActionProgressEntry } from '@/types/actionAudit';
import { buildResourcePagePath } from '@/features/platformNavigation/resourcePagePath';
import { useResolvedResource } from '@/features/platformNavigation/useResolvedResource';
import { ActionDecisionPacket } from './ActionDecisionPacket';
import {
  formatActionName,
  getActionAuditStatePresentation,
  getActionOriginDestination,
  getActionResourcePresentation,
} from './actionPresentation';
import { getAPTActionPresentation } from './aptActionPresentation';

// The server's bounded reconciliation window is one hour. This is only a
// presentation threshold: the server remains the authority for every write.
const RECEIPT_WAIT_RECOVERY_AGE_MS = 60 * 60 * 1000;
const LIVE_PROGRESS_INITIAL_DELAY_MS = 400;
const LIVE_PROGRESS_POLL_INTERVAL_MS = 1000;

const formatElapsedDuration = (ms: number): string => {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, '0')}`;
};

const formatProgressTimestamp = (iso: string): string => {
  const date = new Date(iso);
  if (Number.isNaN(date.valueOf())) return '';
  return date.toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
};

const agedReceiptPending = (detail: ActionDetailResponse | null, now: number): boolean => {
  if (detail?.audit.state !== 'executing' || detail.attempt?.state !== 'receipt_pending') {
    return false;
  }
  const createdAt = Date.parse(detail.attempt.createdAt);
  const updatedAt = Date.parse(detail.attempt.updatedAt);
  return (
    Number.isFinite(createdAt) &&
    Number.isFinite(updatedAt) &&
    now >= Math.max(createdAt, updatedAt) + RECEIPT_WAIT_RECOVERY_AGE_MS
  );
};

export const ActionReviewDialog: Component<{
  detail: ActionDetailResponse | null;
  onClose: () => void;
  onChanged?: (detail: ActionDetailResponse) => void | Promise<void>;
  /**
   * Link the resource the action acts on. Only a host that keeps the review
   * in the address (the Actions page's `?action=`) should ask: leaving for
   * the resource and pressing Back must return to the same review, and a
   * review opened from the resource itself has nowhere new to go.
   */
  showResourceLink?: boolean;
}> = (props) => {
  const [busy, setBusy] = createSignal(false);
  const [executingStage, setExecutingStage] = createSignal<'idle' | 'approving' | 'running'>(
    'idle',
  );
  const [executionStartedAt, setExecutionStartedAt] = createSignal<number | null>(null);
  const [liveProgress, setLiveProgress] = createSignal<ActionProgressEntry[]>([]);
  const [error, setError] = createSignal('');
  const [receiptRefreshError, setReceiptRefreshError] = createSignal('');
  const [clock, setClock] = createSignal(Date.now());
  let liveLogContainer: HTMLDivElement | undefined;
  const [canForceFail, setCanForceFail] = createSignal(false);
  const [recoveryOpen, setRecoveryOpen] = createSignal(false);
  const [recoveryReason, setRecoveryReason] = createSignal('');
  const [recoveryConfirmed, setRecoveryConfirmed] = createSignal(false);
  const audit = () => props.detail?.audit;
  const receiptWaiting = () =>
    audit()?.state === 'executing' && props.detail?.attempt?.state === 'receipt_pending';
  const originDestination = () => getActionOriginDestination(audit()?.origin);
  const resource = createMemo(() => {
    const record = audit();
    return record
      ? getActionResourcePresentation(record.request.resourceId, record.resource)
      : { label: '', detail: '' };
  });
  // The review asks the operator to check the resource before approving or
  // retrying, so it links to the page that shows it.
  const registryResource = useResolvedResource(() =>
    props.showResourceLink ? audit()?.request.resourceId : undefined,
  );
  const resourceHref = createMemo(() => {
    const record = audit();
    if (!record || !props.showResourceLink) return null;
    const resolved = registryResource();
    // A bare VM might belong to another platform or to a view this build
    // does not have. Don't focus or offer a provisional Proxmox link while
    // its lookup can still change or remove the destination.
    if (resolved === undefined && record.resource?.type?.trim().toLowerCase() === 'vm') {
      return null;
    }
    return buildResourcePagePath({
      id: record.request.resourceId,
      name: record.resource?.name,
      type: record.resource?.type,
      resource: resolved,
    });
  });
  const readOnly = createMemo(
    () => props.detail?.readOnly === true || presentationPolicyIsReadOnly(),
  );
  const canOfferRecovery = createMemo(
    () => !readOnly() && canForceFail() && agedReceiptPending(props.detail, clock()),
  );
  let trackedActionId: string | undefined;
  createEffect(() => {
    const nextActionId = props.detail?.audit.id;
    if (nextActionId !== trackedActionId) {
      trackedActionId = nextActionId;
      setExecutingStage('idle');
      setExecutionStartedAt(null);
      setLiveProgress(props.detail?.progress ?? []);
      setError('');
    } else if (props.detail?.progress && props.detail.progress.length > liveProgress().length) {
      setLiveProgress(props.detail.progress);
    }
  });

  createEffect(() => {
    const stage = executingStage();
    const actionId = audit()?.id;
    if (stage !== 'running' || !actionId) return;

    let active = true;
    let pollInFlight = false;
    let intervalId: number | undefined;

    const pollProgress = async () => {
      if (!active || pollInFlight) return;
      pollInFlight = true;
      try {
        const latest = await ResourceActionsAPI.getAction(actionId);
        if (!active) return;
        if (latest.progress && latest.progress.length > 0) {
          setLiveProgress(latest.progress);
        }
      } catch {
        // Keep the active run calm if an intermediate progress poll misses.
      } finally {
        pollInFlight = false;
      }
    };

    const initialTimer = window.setTimeout(() => {
      void pollProgress();
      intervalId = window.setInterval(() => {
        void pollProgress();
      }, LIVE_PROGRESS_POLL_INTERVAL_MS);
    }, LIVE_PROGRESS_INITIAL_DELAY_MS);

    onCleanup(() => {
      active = false;
      window.clearTimeout(initialTimer);
      if (intervalId !== undefined) window.clearInterval(intervalId);
    });
  });

  createEffect(() => {
    const count = liveProgress().length;
    if (count > 0 && liveLogContainer) {
      liveLogContainer.scrollTop = liveLogContainer.scrollHeight;
    }
  });

  const displayedLiveProgress = createMemo<ActionProgressEntry[]>(() => {
    const serverEntries = liveProgress();
    if (serverEntries.length > 0) return serverEntries;
    const stage = executingStage();
    const started = executionStartedAt();
    if (stage === 'idle' || started === null) return [];
    const ts = new Date(started).toISOString();
    if (stage === 'approving') {
      return [{ timestamp: ts, source: 'server', message: 'Recording approval...' }];
    }
    return [
      {
        timestamp: ts,
        source: 'server',
        message: 'Dispatching action to agent...',
      },
    ];
  });

  const showLiveExecutionPanel = createMemo(
    () =>
      executingStage() !== 'idle' ||
      (audit()?.state === 'executing' && !receiptWaiting() && displayedLiveProgress().length > 0),
  );

  const elapsedMs = createMemo(() => {
    const started = executionStartedAt();
    if (started === null) return 0;
    return Math.max(0, clock() - started);
  });

  const hasAgentProgress = createMemo(() =>
    displayedLiveProgress().some((entry) => entry.source === 'agent'),
  );

  const headerBadgePresentation = createMemo(() => {
    if (executingStage() === 'approving') {
      return { tone: 'info' as const, label: 'Approving' };
    }
    if (executingStage() === 'running') {
      return { tone: 'info' as const, label: 'Running' };
    }
    if (receiptWaiting()) {
      return { tone: 'warning' as const, label: 'Receipt pending' };
    }
    const record = audit();
    return record
      ? getActionAuditStatePresentation(record)
      : { tone: 'neutral' as const, label: '' };
  });

  createEffect(() => {
    const detail = props.detail;
    setReceiptRefreshError('');
    setRecoveryOpen(false);
    setRecoveryReason('');
    setRecoveryConfirmed(false);
    setCanForceFail(false);
    if (
      detail?.audit.state !== 'executing' ||
      detail.attempt?.state !== 'receipt_pending' ||
      readOnly()
    ) {
      return;
    }
    let current = true;
    void SecurityAPI.getStatus()
      .then((status) => {
        // The route repeats this admin/settings-write gate and additionally
        // checks action-execute authority. Never infer permission from the UI.
        if (current) setCanForceFail(status.settingsCapabilities?.authenticationWrite === true);
      })
      .catch(() => {
        if (current) setCanForceFail(false);
      });
    onCleanup(() => {
      current = false;
    });
  });
  createEffect(() => {
    if (!props.detail) return;
    setClock(Date.now());
    const timer = window.setInterval(() => setClock(Date.now()), 1000);
    onCleanup(() => window.clearInterval(timer));
  });
  const hasCurrentPolicyProvenance = createMemo(
    () => audit()?.plan.policyDecision?.status === 'resolved',
  );
  const readiness = createMemo(() => props.detail?.readiness);
  const reviewedPlanHash = createMemo(() => audit()?.plan.planHash?.trim() || '');
  const aptParametersValid = createMemo(() => {
    const action = audit();
    return action ? getAPTActionPresentation(action)?.parametersValid !== false : false;
  });
  const isExpired = createMemo(() => {
    const expiresAt = audit()?.plan.expiresAt;
    if (!expiresAt) return true;
    const timestamp = new Date(expiresAt).valueOf();
    return Number.isNaN(timestamp) || timestamp <= clock();
  });
  const canReject = () =>
    !readOnly() &&
    reviewedPlanHash() &&
    hasCurrentPolicyProvenance() &&
    aptParametersValid() &&
    !isExpired() &&
    audit()?.state === 'pending_approval';
  const canApprove = () => canReject() && readiness()?.ready === true;
  // Low-risk capabilities (rollback-supported, routine) collapse the decision
  // to one confirmation: a single click records the approval and dispatches
  // execution. Both lifecycle records are still written server-side.
  const singleConfirmation = createMemo(() => audit()?.capabilityAutoAuthorization === 'low_risk');
  const canExecute = () =>
    !readOnly() &&
    reviewedPlanHash() &&
    hasCurrentPolicyProvenance() &&
    aptParametersValid() &&
    !isExpired() &&
    readiness()?.ready === true &&
    (audit()?.state === 'approved' ||
      (audit()?.state === 'planned' && !audit()?.plan.requiresApproval));
  const invalidActionMessage = createMemo(() => {
    const state = audit()?.state;
    const actionable =
      state === 'pending_approval' ||
      state === 'approved' ||
      state === 'planned' ||
      state === 'expired';
    if (!actionable) return '';
    if (readOnly())
      return 'This session is read-only. You can inspect the action and its policy evidence, but you cannot approve or run it.';
    if (!reviewedPlanHash())
      return 'This action has no reviewed plan identity. Close it and create a new plan before approving or running anything.';
    if (!hasCurrentPolicyProvenance())
      return 'This action has no current server policy provenance. Close it and create a new plan before approving or running anything.';
    if (!aptParametersValid())
      return 'This host-maintenance action contains unexpected operator-selected parameters. Close it and create a new plan. Do not approve or run this record.';
    if (readiness() && !readiness()!.ready) {
      return [readiness()!.message, readiness()!.remediation].filter(Boolean).join(' ');
    }
    if (isExpired())
      return 'This action review expired. Refresh the plan so current resource and policy state can be checked again.';
    return '';
  });

  const canRefreshPlan = createMemo(
    () =>
      !readOnly() &&
      Boolean(reviewedPlanHash()) &&
      (readiness()?.refreshable === true || isExpired()) &&
      ['planned', 'pending_approval', 'approved', 'expired'].includes(audit()?.state ?? ''),
  );

  const actionableErrorMessage = (cause: unknown, fallback: string): string => {
    if (!(cause instanceof Error)) return fallback;
    const details = (cause as Error & { details?: Record<string, string> }).details;
    const reason = details?.reason?.trim();
    return reason || cause.message || fallback;
  };

  const refresh = async () => {
    const actionId = audit()?.id;
    if (!actionId) return;
    const detail = await ResourceActionsAPI.getAction(actionId);
    await props.onChanged?.(detail);
  };

  const refreshReceipt = async () => {
    const actionId = audit()?.id;
    if (!actionId || !receiptWaiting() || busy()) return;
    setBusy(true);
    setReceiptRefreshError('');
    try {
      const latest = await ResourceActionsAPI.getAction(actionId);
      await props.onChanged?.(latest);
    } catch {
      setReceiptRefreshError('Could not re-read this action. No new action was sent.');
    } finally {
      setBusy(false);
    }
  };

  const decide = async (outcome: 'approved' | 'rejected') => {
    const action = audit();
    if (!action || busy()) return;
    setBusy(true);
    setError('');
    try {
      await ResourceActionsAPI.decideAction(
        action.id,
        outcome,
        reviewedPlanHash(),
        `Operator ${outcome} from Actions review.`,
      );
      await refresh();
      notificationStore.success(
        outcome === 'approved'
          ? 'Approved. Nothing has run yet. Press Run action when you are ready.'
          : 'Action rejected.',
      );
      if (outcome === 'rejected') props.onClose();
    } catch (cause) {
      setError(actionableErrorMessage(cause, 'The decision could not be recorded.'));
    } finally {
      setBusy(false);
    }
  };

  const refreshPlan = async () => {
    const action = audit();
    if (!action || busy()) return;
    setBusy(true);
    setError('');
    try {
      const replacement = await ResourceActionsAPI.refreshAction(action.id, reviewedPlanHash());
      await props.onChanged?.(replacement);
      notificationStore.success('Plan refreshed. Review the replacement before approving it.');
    } catch (cause) {
      setError(actionableErrorMessage(cause, 'The action plan could not be refreshed.'));
      try {
        await refresh();
      } catch {
        /* preserve the refresh failure */
      }
    } finally {
      setBusy(false);
    }
  };

  const approveAndRun = async () => {
    const action = audit();
    if (!action || busy()) return;
    const now = Date.now();
    setBusy(true);
    setExecutingStage('approving');
    setExecutionStartedAt(now);
    setLiveProgress([]);
    setError('');
    try {
      await ResourceActionsAPI.decideAction(
        action.id,
        'approved',
        reviewedPlanHash(),
        'Operator approved from Actions review.',
      );
      setExecutingStage('running');
      await ResourceActionsAPI.executeAction(
        action.id,
        reviewedPlanHash(),
        'Operator confirmed execution from Actions review.',
      );
      await refresh();
      notificationStore.success(
        'Action approved and dispatched. Review the recorded outcome below.',
      );
    } catch (cause) {
      setError(actionableErrorMessage(cause, 'The action could not be approved and run.'));
      try {
        await refresh();
      } catch {
        /* keep the actionable error; a refresh failure must not mask it */
      }
    } finally {
      setExecutingStage('idle');
      setBusy(false);
    }
  };

  const execute = async () => {
    const action = audit();
    if (!action || busy()) return;
    const now = Date.now();
    setBusy(true);
    setExecutingStage('running');
    setExecutionStartedAt(now);
    setLiveProgress([]);
    setError('');
    try {
      await ResourceActionsAPI.executeAction(
        action.id,
        reviewedPlanHash(),
        'Operator confirmed execution from Actions review.',
      );
      await refresh();
      notificationStore.success('Action sent. Its recorded outcome is below.');
    } catch (cause) {
      setError(actionableErrorMessage(cause, 'The action could not be run.'));
      try {
        await refresh();
      } catch {
        /* keep the actionable execution error */
      }
    } finally {
      setExecutingStage('idle');
      setBusy(false);
    }
  };

  const closeStuckAudit = async () => {
    const currentDetail = props.detail;
    const reason = recoveryReason().trim();
    if (
      !currentDetail ||
      !canOfferRecovery() ||
      !recoveryOpen() ||
      !recoveryConfirmed() ||
      reason.length < 15 ||
      busy()
    )
      return;

    setBusy(true);
    setError('');
    let recorded = false;
    try {
      const latest = await ResourceActionsAPI.getAction(currentDetail.audit.id);
      if (
        !agedReceiptPending(latest, Date.now()) ||
        latest.attempt?.id !== currentDetail.attempt?.id ||
        latest.attempt?.updatedAt !== currentDetail.attempt?.updatedAt
      ) {
        await props.onChanged?.(latest);
        setRecoveryOpen(false);
        setError(
          'The action changed while you were reviewing it. No audit override was sent. Review its latest outcome.',
        );
        return;
      }

      const outcome = await ResourceActionsAPI.forceFailAction(currentDetail.audit.id, reason);
      recorded = true;
      // Keep the server's terminal audit visible even if the follow-up read
      // fails; do not invite a second override after a successful mutation.
      let displayed: ActionDetailResponse = { ...latest, audit: outcome.audit };
      try {
        displayed = await ResourceActionsAPI.getAction(currentDetail.audit.id);
      } catch {
        setError(
          'The audit was closed, but its latest details could not be loaded. Refresh the action history.',
        );
      }
      await props.onChanged?.(displayed);
      setRecoveryOpen(false);
      notificationStore.success(
        'Audit closed with an unknown operation outcome. Check the resource before any retry.',
      );
    } catch (cause) {
      setError(
        recorded
          ? 'The audit was closed, but the view could not refresh. Reload action history before doing anything else.'
          : actionableErrorMessage(cause, 'The audit could not be closed.'),
      );
      if (!recorded) {
        try {
          await refresh();
        } catch {
          /* preserve the override failure */
        }
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      isOpen={Boolean(props.detail)}
      onClose={props.onClose}
      ariaLabelledBy="action-review-title"
      panelClass="max-w-3xl"
    >
      <Show when={audit()}>
        {(record) => (
          <div class="flex max-h-[min(90vh,900px)] flex-col">
            <header class="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
              <div>
                <p class="text-xs font-semibold uppercase tracking-wide text-muted">
                  Governed action review
                </p>
                <div class="mt-1 flex flex-wrap items-center gap-2">
                  <h2 id="action-review-title" class="text-xl font-semibold">
                    {formatActionName(record().request.capabilityName)}
                  </h2>
                  <MetadataBadge tone={headerBadgePresentation().tone}>
                    {headerBadgePresentation().label}
                  </MetadataBadge>
                </div>
                <p class="mt-1 text-sm text-muted">
                  {resource().label}
                  <Show when={resource().detail}> · {resource().detail}</Show>
                </p>
                <Show when={resourceHref() || originDestination()}>
                  <div class="mt-2 -ml-2.5 flex flex-wrap items-center gap-1">
                    <Show when={resourceHref()}>
                      {(href) => (
                        <ButtonLink
                          href={href()}
                          variant="ghost"
                          size="xs"
                          class="min-h-11 gap-1.5 sm:min-h-0"
                        >
                          Open resource
                          <ArrowUpRightIcon class="h-3.5 w-3.5" aria-hidden="true" />
                        </ButtonLink>
                      )}
                    </Show>
                    <Show when={originDestination()}>
                      {(destination) => (
                        <ButtonLink
                          href={destination().href}
                          variant="ghost"
                          size="xs"
                          class="min-h-11 gap-1.5 sm:min-h-0"
                        >
                          {destination().exact ? 'Open Patrol record' : 'Open Patrol'}
                          <ArrowUpRightIcon class="h-3.5 w-3.5" aria-hidden="true" />
                        </ButtonLink>
                      )}
                    </Show>
                  </div>
                </Show>
              </div>
              <Button
                variant="ghost"
                size="icon"
                aria-label="Close action review"
                onClick={props.onClose}
              >
                <XIcon class="h-5 w-5" />
              </Button>
            </header>
            <div class="overflow-y-auto px-5 py-4">
              <Show when={showLiveExecutionPanel()}>
                <section
                  aria-labelledby="action-live-execution-heading"
                  aria-live="polite"
                  data-testid="action-live-execution"
                  class="mb-4 overflow-hidden rounded-lg border border-blue-200 bg-slate-50 dark:border-blue-800/70 dark:bg-slate-950/70"
                >
                  <div class="flex flex-wrap items-center justify-between gap-2 border-b border-blue-200/80 bg-blue-50/70 px-3.5 py-2.5 dark:border-blue-900/60 dark:bg-blue-950/30">
                    <div class="flex min-w-0 items-center gap-2">
                      <TerminalIcon
                        class="h-4 w-4 shrink-0 text-blue-600 dark:text-blue-400"
                        aria-hidden="true"
                      />
                      <h3
                        id="action-live-execution-heading"
                        class="truncate text-xs font-semibold text-base-content"
                      >
                        {executingStage() === 'approving'
                          ? `Approving ${formatActionName(record().request.capabilityName).toLowerCase()} for ${resource().label}`
                          : `Running ${formatActionName(record().request.capabilityName).toLowerCase()} on ${resource().label}`}
                      </h3>
                    </div>
                    <Show when={executionStartedAt() !== null}>
                      <span
                        data-testid="action-live-elapsed"
                        class="font-mono text-xs tabular-nums text-muted"
                      >
                        {formatElapsedDuration(elapsedMs())}
                      </span>
                    </Show>
                  </div>
                  <div
                    ref={liveLogContainer}
                    data-testid="action-live-log"
                    class="max-h-56 overflow-y-auto px-3.5 py-2.5 font-mono text-xs leading-relaxed"
                  >
                    <ol class="space-y-1.5">
                      <For each={displayedLiveProgress()}>
                        {(entry, index) => {
                          const isLatest = () =>
                            index() === displayedLiveProgress().length - 1 &&
                            executingStage() !== 'idle';
                          return (
                            <li class="flex items-start gap-2">
                              <span class="mt-0.5 shrink-0" aria-hidden="true">
                                <Show
                                  when={isLatest()}
                                  fallback={
                                    <CheckIcon class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                                  }
                                >
                                  <Loader2Icon class="h-3.5 w-3.5 animate-spin text-blue-600 dark:text-blue-400" />
                                </Show>
                              </span>
                              <Show when={formatProgressTimestamp(entry.timestamp)}>
                                {(time) => (
                                  <span class="shrink-0 tabular-nums text-muted">{time()}</span>
                                )}
                              </Show>
                              <span
                                class={
                                  isLatest()
                                    ? 'min-w-0 break-words font-medium text-base-content'
                                    : 'min-w-0 break-words text-muted'
                                }
                              >
                                {entry.message}
                              </span>
                            </li>
                          );
                        }}
                      </For>
                    </ol>
                    <Show
                      when={
                        executingStage() === 'running' && !hasAgentProgress() && elapsedMs() >= 5000
                      }
                    >
                      <p class="mt-2 font-sans text-xs text-muted">
                        Waiting for the agent to report the next step. Image pulls and container
                        restarts can take a minute.
                      </p>
                    </Show>
                  </div>
                  <div class="border-t border-border-subtle px-3.5 py-2 text-xs text-muted">
                    You can close this dialog safely. The action continues on the server and its
                    outcome will be recorded in Actions.
                  </div>
                </section>
              </Show>
              <Show when={receiptWaiting()}>
                <section
                  aria-labelledby="action-receipt-waiting-heading"
                  data-testid="action-receipt-waiting"
                  class="mb-4 rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm dark:bg-amber-950/40"
                >
                  <h3 id="action-receipt-waiting-heading" class="font-semibold">
                    Waiting for the agent receipt
                  </h3>
                  <p class="mt-2">
                    Pulse sent this action once, but its outcome is not known yet. The agent may
                    still be working or may have finished. Do not start the action again from a new
                    plan. Check the actual resource if the wait persists.
                  </p>
                  <Button
                    variant="secondary"
                    size="sm"
                    class="mt-3"
                    isLoading={busy()}
                    onClick={() => void refreshReceipt()}
                  >
                    Check for receipt
                  </Button>
                  <Show when={receiptRefreshError()}>
                    <p role="alert" class="mt-2 text-red-800 dark:text-red-200">
                      {receiptRefreshError()}
                    </p>
                  </Show>
                </section>
              </Show>
              <ActionDecisionPacket audit={record()} detail={props.detail ?? undefined} />
              <Show when={canOfferRecovery()}>
                <section
                  aria-labelledby="action-stuck-recovery-heading"
                  data-testid="action-stuck-recovery"
                  class="mt-4 rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm dark:bg-amber-950/40"
                >
                  <h3 id="action-stuck-recovery-heading" class="font-semibold">
                    Receipt still missing
                  </h3>
                  <p class="mt-2">
                    Pulse sent this action but cannot confirm what the agent did. Check the actual
                    resource directly before any retry. For a container update, check the running
                    container and image, not just this audit record.
                  </p>
                  <Show
                    when={recoveryOpen()}
                    fallback={
                      <Button
                        variant="secondary"
                        size="sm"
                        class="mt-3"
                        onClick={() => setRecoveryOpen(true)}
                      >
                        Close stuck audit record…
                      </Button>
                    }
                  >
                    <div class="mt-4 space-y-3 border-t border-amber-300 pt-4 dark:border-amber-800">
                      <p>
                        This only closes the Pulse audit with an <strong>inconclusive</strong>{' '}
                        outcome. It does not cancel the agent operation, roll back the change, or
                        prove that the operation failed.
                      </p>
                      <FormTextarea
                        label="What did you verify directly?"
                        help="Record the resource state you checked and why the receipt cannot be recovered. This becomes part of the audit."
                        value={recoveryReason()}
                        onInput={(event) => setRecoveryReason(event.currentTarget.value)}
                        maxLength={500}
                        rows={3}
                      />
                      <label class="flex cursor-pointer items-start gap-2">
                        <input
                          type="checkbox"
                          class="mt-1 h-4 w-4"
                          checked={recoveryConfirmed()}
                          onChange={(event) => setRecoveryConfirmed(event.currentTarget.checked)}
                        />
                        <span>
                          I checked the actual resource and understand the operation outcome remains
                          unknown.
                        </span>
                      </label>
                      <div class="flex flex-wrap gap-2">
                        <Button
                          variant="danger"
                          size="sm"
                          disabled={
                            busy() || !recoveryConfirmed() || recoveryReason().trim().length < 15
                          }
                          onClick={() => void closeStuckAudit()}
                        >
                          Close audit as inconclusive
                        </Button>
                        <Button size="sm" disabled={busy()} onClick={() => setRecoveryOpen(false)}>
                          Keep waiting
                        </Button>
                      </div>
                    </div>
                  </Show>
                </section>
              </Show>
              <Show when={invalidActionMessage()}>
                <div
                  role="alert"
                  data-testid="action-review-invalid"
                  class="mt-4 rounded-sm border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:bg-amber-950/40 dark:text-amber-200"
                >
                  {invalidActionMessage()}
                </div>
              </Show>
              <Show when={error()}>
                <div
                  role="alert"
                  class="mt-4 rounded-sm border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950/40 dark:text-red-200"
                >
                  {error()}
                </div>
              </Show>
            </div>
            <footer class="flex flex-col-reverse gap-2 border-t border-border px-5 py-4 sm:flex-row sm:justify-end">
              <Button onClick={props.onClose}>Close</Button>
              <Show when={canRefreshPlan()}>
                <Button variant="primary" isLoading={busy()} onClick={() => void refreshPlan()}>
                  Refresh plan
                </Button>
              </Show>
              <Show when={canReject()}>
                <Button variant="danger" disabled={busy()} onClick={() => void decide('rejected')}>
                  Reject
                </Button>
              </Show>
              <Show when={canApprove()}>
                <Show
                  when={singleConfirmation()}
                  fallback={
                    <Button
                      variant="primary"
                      isLoading={busy()}
                      onClick={() => void decide('approved')}
                    >
                      Approve
                    </Button>
                  }
                >
                  <Button variant="primary" isLoading={busy()} onClick={() => void approveAndRun()}>
                    {executingStage() === 'approving'
                      ? 'Approving...'
                      : executingStage() === 'running'
                        ? 'Running...'
                        : 'Approve and run'}
                  </Button>
                </Show>
              </Show>
              <Show when={canExecute()}>
                <Button variant="primary" isLoading={busy()} onClick={() => void execute()}>
                  {executingStage() === 'running' ? 'Running...' : 'Run action'}
                </Button>
              </Show>
            </footer>
          </div>
        )}
      </Show>
    </Dialog>
  );
};
