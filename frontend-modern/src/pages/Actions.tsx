import { For, Show, createEffect, createSignal } from 'solid-js';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { Button } from '@/components/shared/Button';
import { PageHeader } from '@/components/shared/PageHeader';
import { ActionReviewDialog } from '@/features/actions/ActionReviewDialog';
import { formatActionName } from '@/features/actions/actionPresentation';
import type { ActionAuditRecord, ActionDetailResponse, ActionInboxView } from '@/types/actionAudit';
import { formatRelativeTime } from '@/utils/format';

export function Actions() {
  const [view, setView] = createSignal<ActionInboxView>('pending');
  const [selected, setSelected] = createSignal<ActionDetailResponse | null>(null);
  const [detailError, setDetailError] = createSignal('');
  const [actions, setActions] = createSignal<ActionAuditRecord[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [loadError, setLoadError] = createSignal('');

  const loadActions = async () => {
    setLoading(true);
    setLoadError('');
    try {
      const response = await ResourceActionsAPI.listActions(view());
      setActions(response.actions);
    } catch (cause) {
      setActions([]);
      setLoadError(cause instanceof Error ? cause.message : 'The action store is unavailable.');
    } finally {
      setLoading(false);
    }
  };

  createEffect(() => {
    view();
    void loadActions();
  });

  const openAction = async (record: ActionAuditRecord) => {
    setDetailError('');
    try { setSelected(await ResourceActionsAPI.getAction(record.id)); }
    catch (cause) { setDetailError(cause instanceof Error ? cause.message : 'Action details are unavailable.'); }
  };

  return (
    <div class="mx-auto w-full max-w-6xl space-y-5 px-3 py-4 sm:px-5">
      <PageHeader title="Actions" description="Review every pending change and its recorded outcome in one place." />
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div role="tablist" aria-label="Action views" class="inline-flex rounded-lg border border-border bg-surface p-1">
          <button role="tab" aria-selected={view() === 'pending'} class={`rounded px-4 py-2 text-sm font-medium ${view() === 'pending' ? 'bg-blue-600 text-white' : 'text-muted hover:text-base-content'}`} onClick={() => setView('pending')}>Needs attention</button>
          <button role="tab" aria-selected={view() === 'settled'} class={`rounded px-4 py-2 text-sm font-medium ${view() === 'settled' ? 'bg-blue-600 text-white' : 'text-muted hover:text-base-content'}`} onClick={() => setView('settled')}>History</button>
        </div>
        <Button aria-label="Refresh actions" onClick={() => void loadActions()} isLoading={loading()}><RefreshCwIcon class="mr-2 h-4 w-4" />Refresh</Button>
      </div>

      <Show when={detailError()}><div role="alert" class="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950/40 dark:text-amber-200">{detailError()}</div></Show>
      <Show when={loadError()}><div role="alert" class="rounded-lg border border-red-300 bg-red-50 p-4 text-sm text-red-800 dark:bg-red-950/40 dark:text-red-200"><p class="font-medium">Actions could not be loaded.</p><p class="mt-1">{loadError()}</p><Button class="mt-3" onClick={() => void loadActions()}>Try again</Button></div></Show>
      <Show when={!loading() && !loadError() && actions().length === 0}>
        <div data-testid="actions-calm-state" class="rounded-xl border border-dashed border-border bg-surface p-8 text-center"><h2 class="text-lg font-semibold">{view() === 'pending' ? 'No actions need attention' : 'No action history yet'}</h2><p class="mt-2 text-sm text-muted">{view() === 'pending' ? 'Pulse will place proposed or in-progress changes here for review.' : 'Completed, rejected, expired, and failed actions will appear here.'}</p></div>
      </Show>
      <Show when={actions().length > 0}>
        <ul class="space-y-3" aria-label={view() === 'pending' ? 'Actions needing attention' : 'Action history'}>
          <For each={actions()}>{(action) => <li><button type="button" class="w-full rounded-xl border border-border bg-surface p-4 text-left transition hover:border-blue-400 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500" onClick={() => void openAction(action)}><div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between"><div class="min-w-0"><div class="font-semibold">{formatActionName(action.request.capabilityName)}</div><div class="mt-1 break-all text-sm text-muted">{action.request.resourceId}</div><p class="mt-2 text-sm">{action.request.reason}</p></div><div class="shrink-0 text-left sm:text-right"><span class="inline-flex rounded-full border border-border-subtle bg-surface-hover px-2.5 py-1 text-xs font-medium">{formatActionName(action.state)}</span><div class="mt-2 text-xs text-muted">{formatRelativeTime(action.updatedAt)}</div></div></div></button></li>}</For>
        </ul>
      </Show>
      <ActionReviewDialog detail={selected()} onClose={() => setSelected(null)} onChanged={async (detail) => { setSelected(detail); await loadActions(); }} />
    </div>
  );
}

export default Actions;
