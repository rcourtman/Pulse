import { Show, createEffect, createMemo, createSignal, onCleanup, type Component } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { notificationStore } from '@/stores/notifications';
import type { ActionDetailResponse } from '@/types/actionAudit';
import { ActionDecisionPacket } from './ActionDecisionPacket';
import { formatActionName } from './actionPresentation';

export const ActionReviewDialog: Component<{
  detail: ActionDetailResponse | null;
  onClose: () => void;
  onChanged?: (detail: ActionDetailResponse) => void | Promise<void>;
}> = (props) => {
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal('');
  const [clock, setClock] = createSignal(Date.now());
  const audit = () => props.detail?.audit;
  createEffect(() => {
    if (!props.detail) return;
    setClock(Date.now());
    const timer = window.setInterval(() => setClock(Date.now()), 1000);
    onCleanup(() => window.clearInterval(timer));
  });
  const hasCurrentPolicyProvenance = createMemo(
    () => audit()?.plan.policyDecision?.status === 'resolved',
  );
  const isExpired = createMemo(() => {
    const expiresAt = audit()?.plan.expiresAt;
    if (!expiresAt) return true;
    const timestamp = new Date(expiresAt).valueOf();
    return Number.isNaN(timestamp) || timestamp <= clock();
  });
  const canDecide = () => hasCurrentPolicyProvenance() && !isExpired() && audit()?.state === 'pending_approval';
  const canExecute = () =>
    hasCurrentPolicyProvenance() &&
    !isExpired() &&
    (audit()?.state === 'approved' ||
      (audit()?.state === 'planned' && !audit()?.plan.requiresApproval));
  const invalidActionMessage = createMemo(() => {
    if (!hasCurrentPolicyProvenance()) return 'This action has no current server policy provenance. Close it and create a new plan before approving or running anything.';
    if (isExpired()) return 'This action review expired. Close it and create a new plan so current resource and policy state can be checked again.';
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
    setBusy(true); setError('');
    try {
      await ResourceActionsAPI.decideAction(action.id, outcome, `Operator ${outcome} from Actions review.`);
      await refresh();
      notificationStore.success(outcome === 'approved' ? 'Action approved. Review once more before running it.' : 'Action rejected.');
      if (outcome === 'rejected') props.onClose();
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : 'The decision could not be recorded.';
      setError(message);
    } finally { setBusy(false); }
  };

  const execute = async () => {
    const action = audit();
    if (!action || busy()) return;
    setBusy(true); setError('');
    try {
      await ResourceActionsAPI.executeAction(action.id, 'Operator confirmed execution from Actions review.');
      await refresh();
      notificationStore.success('Action dispatch response recorded. Review execution, verification, and recovery separately.');
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : 'The action could not be run.';
      setError(message);
      try { await refresh(); } catch { /* keep the actionable execution error */ }
    } finally { setBusy(false); }
  };

  return (
    <Dialog isOpen={Boolean(props.detail)} onClose={props.onClose} ariaLabelledBy="action-review-title" panelClass="max-w-3xl">
      <Show when={audit()}>
        {(record) => (
          <div class="flex max-h-[min(90vh,900px)] flex-col">
            <header class="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
              <div><p class="text-xs font-semibold uppercase tracking-wide text-muted">Governed action review</p><h2 id="action-review-title" class="mt-1 text-xl font-semibold">{formatActionName(record().request.capabilityName)}</h2><p class="mt-1 break-all text-sm text-muted">{record().request.resourceId}</p></div>
              <Button variant="ghost" size="icon" aria-label="Close action review" onClick={props.onClose}><XIcon class="h-5 w-5" /></Button>
            </header>
            <div class="overflow-y-auto px-5 py-4"><ActionDecisionPacket audit={record()} /><Show when={invalidActionMessage()}><div role="alert" data-testid="action-review-invalid" class="mt-4 rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:bg-amber-950/40 dark:text-amber-200">{invalidActionMessage()}</div></Show><Show when={error()}><div role="alert" class="mt-4 rounded border border-red-300 bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950/40 dark:text-red-200">{error()}</div></Show></div>
            <footer class="flex flex-col-reverse gap-2 border-t border-border px-5 py-4 sm:flex-row sm:justify-end">
              <Button onClick={props.onClose}>Close</Button>
              <Show when={canDecide()}><Button variant="danger" disabled={busy()} onClick={() => void decide('rejected')}>Reject</Button><Button variant="primary" isLoading={busy()} onClick={() => void decide('approved')}>Approve</Button></Show>
              <Show when={canExecute()}><Button variant="primary" isLoading={busy()} onClick={() => void execute()}>Run action</Button></Show>
            </footer>
          </div>
        )}
      </Show>
    </Dialog>
  );
};
