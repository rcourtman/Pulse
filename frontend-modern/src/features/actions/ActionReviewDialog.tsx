import { Show, createEffect, createMemo, createSignal, onCleanup, type Component } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import ArrowUpRightIcon from 'lucide-solid/icons/arrow-up-right';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { SecurityAPI } from '@/api/security';
import { Button, ButtonLink } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { FormTextarea } from '@/components/shared/FormTextarea';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { notificationStore } from '@/stores/notifications';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import type { ActionDetailResponse } from '@/types/actionAudit';
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
}> = (props) => {
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal('');
  const [clock, setClock] = createSignal(Date.now());
  const [canForceFail, setCanForceFail] = createSignal(false);
  const [recoveryOpen, setRecoveryOpen] = createSignal(false);
  const [recoveryReason, setRecoveryReason] = createSignal('');
  const [recoveryConfirmed, setRecoveryConfirmed] = createSignal(false);
  const audit = () => props.detail?.audit;
  const originDestination = () => getActionOriginDestination(audit()?.origin);
  const resource = createMemo(() => {
    const record = audit();
    return record
      ? getActionResourcePresentation(record.request.resourceId, record.resource)
      : { label: '', detail: '' };
  });
  const readOnly = createMemo(
    () => props.detail?.readOnly === true || presentationPolicyIsReadOnly(),
  );
  const canOfferRecovery = createMemo(
    () => !readOnly() && canForceFail() && agedReceiptPending(props.detail, clock()),
  );
  createEffect(() => {
    const detail = props.detail;
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
          ? 'Action approved. Review once more before running it.'
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
    setBusy(true);
    setError('');
    try {
      await ResourceActionsAPI.decideAction(
        action.id,
        'approved',
        reviewedPlanHash(),
        'Operator approved from Actions review.',
      );
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
      setBusy(false);
    }
  };

  const execute = async () => {
    const action = audit();
    if (!action || busy()) return;
    setBusy(true);
    setError('');
    try {
      await ResourceActionsAPI.executeAction(
        action.id,
        reviewedPlanHash(),
        'Operator confirmed execution from Actions review.',
      );
      await refresh();
      notificationStore.success(
        'Action dispatch response recorded. Review execution, verification, and recovery separately.',
      );
    } catch (cause) {
      setError(actionableErrorMessage(cause, 'The action could not be run.'));
      try {
        await refresh();
      } catch {
        /* keep the actionable execution error */
      }
    } finally {
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
                  <MetadataBadge tone={getActionAuditStatePresentation(record()).tone}>
                    {getActionAuditStatePresentation(record()).label}
                  </MetadataBadge>
                </div>
                <p class="mt-1 text-sm text-muted">
                  {resource().label}
                  <Show when={resource().detail}> · {resource().detail}</Show>
                </p>
                <Show when={originDestination()}>
                  {(destination) => (
                    <ButtonLink
                      href={destination().href}
                      variant="ghost"
                      size="xs"
                      class="mt-2 -ml-2.5 gap-1.5"
                    >
                      {destination().exact ? 'Open Patrol record' : 'Open Patrol'}
                      <ArrowUpRightIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </ButtonLink>
                  )}
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
                  class="mt-4 rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:bg-amber-950/40 dark:text-amber-200"
                >
                  {invalidActionMessage()}
                </div>
              </Show>
              <Show when={error()}>
                <div
                  role="alert"
                  class="mt-4 rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950/40 dark:text-red-200"
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
                    Approve and run
                  </Button>
                </Show>
              </Show>
              <Show when={canExecute()}>
                <Button variant="primary" isLoading={busy()} onClick={() => void execute()}>
                  Run action
                </Button>
              </Show>
            </footer>
          </div>
        )}
      </Show>
    </Dialog>
  );
};
