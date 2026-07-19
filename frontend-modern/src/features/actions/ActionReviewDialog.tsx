import { Show, createEffect, createMemo, createSignal, onCleanup, type Component } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { notificationStore } from '@/stores/notifications';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import type { ActionDetailResponse } from '@/types/actionAudit';
import { ActionDecisionPacket } from './ActionDecisionPacket';
import { formatActionName, getActionResourcePresentation } from './actionPresentation';
import { getAPTActionPresentation } from './aptActionPresentation';

export const ActionReviewDialog: Component<{
  detail: ActionDetailResponse | null;
  onClose: () => void;
  onChanged?: (detail: ActionDetailResponse) => void | Promise<void>;
}> = (props) => {
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal('');
  const [clock, setClock] = createSignal(Date.now());
  const audit = () => props.detail?.audit;
  const resource = createMemo(() => {
    const record = audit();
    return record
      ? getActionResourcePresentation(record.request.resourceId, record.resource)
      : { label: '', detail: '' };
  });
  const readOnly = createMemo(
    () => props.detail?.readOnly === true || presentationPolicyIsReadOnly(),
  );
  createEffect(() => {
    if (!props.detail) return;
    setClock(Date.now());
    const timer = window.setInterval(() => setClock(Date.now()), 1000);
    onCleanup(() => window.clearInterval(timer));
  });
  const hasCurrentPolicyProvenance = createMemo(
    () => audit()?.plan.policyDecision?.status === 'resolved',
  );
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
  const canDecide = () =>
    !readOnly() &&
    reviewedPlanHash() &&
    hasCurrentPolicyProvenance() &&
    aptParametersValid() &&
    !isExpired() &&
    audit()?.state === 'pending_approval';
  // Low-risk capabilities (rollback-supported, routine) collapse the decision
  // to one confirmation: a single click records the approval and dispatches
  // execution. Both lifecycle records are still written server-side.
  const singleConfirmation = createMemo(
    () => audit()?.capabilityAutoAuthorization === 'low_risk',
  );
  const canExecute = () =>
    !readOnly() &&
    reviewedPlanHash() &&
    hasCurrentPolicyProvenance() &&
    aptParametersValid() &&
    !isExpired() &&
    (audit()?.state === 'approved' ||
      (audit()?.state === 'planned' && !audit()?.plan.requiresApproval));
  const invalidActionMessage = createMemo(() => {
    const state = audit()?.state;
    const actionable = state === 'pending_approval' || state === 'approved' || state === 'planned';
    if (!actionable) return '';
    if (readOnly())
      return 'This session is read-only. You can inspect the action and its policy evidence, but you cannot approve or run it.';
    if (!reviewedPlanHash())
      return 'This action has no reviewed plan identity. Close it and create a new plan before approving or running anything.';
    if (!hasCurrentPolicyProvenance())
      return 'This action has no current server policy provenance. Close it and create a new plan before approving or running anything.';
    if (!aptParametersValid())
      return 'This host-maintenance action contains unexpected operator-selected parameters. Close it and create a new plan; do not approve or run this record.';
    if (isExpired())
      return 'This action review expired. Close it and create a new plan so current resource and policy state can be checked again.';
    return '';
  });

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
      const message =
        cause instanceof Error ? cause.message : 'The decision could not be recorded.';
      setError(message);
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
      const message =
        cause instanceof Error ? cause.message : 'The action could not be approved and run.';
      setError(message);
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
      const message = cause instanceof Error ? cause.message : 'The action could not be run.';
      setError(message);
      try {
        await refresh();
      } catch {
        /* keep the actionable execution error */
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
                <h2 id="action-review-title" class="mt-1 text-xl font-semibold">
                  {formatActionName(record().request.capabilityName)}
                </h2>
                <p class="mt-1 text-sm text-muted">
                  {resource().label}
                  <Show when={resource().detail}> · {resource().detail}</Show>
                </p>
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
              <Show when={canDecide()}>
                <Button variant="danger" disabled={busy()} onClick={() => void decide('rejected')}>
                  Reject
                </Button>
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
                  <Button
                    variant="primary"
                    isLoading={busy()}
                    onClick={() => void approveAndRun()}
                  >
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
