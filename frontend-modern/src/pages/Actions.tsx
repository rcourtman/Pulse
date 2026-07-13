import { For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import EyeIcon from 'lucide-solid/icons/eye';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { Button } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { PageHeader } from '@/components/shared/PageHeader';
import { Subtabs } from '@/components/shared/Subtabs';
import { ActionReviewDialog } from '@/features/actions/ActionReviewDialog';
import {
  formatActionName,
  getActionInboxStatePresentation,
  getActionResourcePresentation,
  sortOpenActionsForReview,
} from '@/features/actions/actionPresentation';
import type { ActionAuditRecord, ActionDetailResponse, ActionInboxView } from '@/types/actionAudit';
import { formatRelativeTime } from '@/utils/format';

export function Actions() {
  const [view, setView] = createSignal<ActionInboxView>('pending');
  const [selected, setSelected] = createSignal<ActionDetailResponse | null>(null);
  const [detailError, setDetailError] = createSignal('');
  const [actions, setActions] = createSignal<ActionAuditRecord[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [loadError, setLoadError] = createSignal('');
  const [readOnly, setReadOnly] = createSignal(false);

  const loadActions = async () => {
    setLoading(true);
    setLoadError('');
    try {
      const response = await ResourceActionsAPI.listActions(view());
      setActions(response.actions);
      setReadOnly(response.readOnly === true);
    } catch (cause) {
      setActions([]);
      setReadOnly(false);
      setLoadError(cause instanceof Error ? cause.message : 'The action store is unavailable.');
    } finally {
      setLoading(false);
    }
  };

  createEffect(() => {
    view();
    void loadActions();
  });

  const queueSummary = createMemo(() => {
    const count = actions().length;
    if (view() === 'settled') {
      return `${count} recorded ${count === 1 ? 'outcome' : 'outcomes'}`;
    }
    const approvals = actions().filter((action) => action.state === 'pending_approval').length;
    const base = `${count} open ${count === 1 ? 'action' : 'actions'}`;
    return approvals > 0
      ? `${base} · ${approvals} ${approvals === 1 ? 'awaits' : 'await'} approval`
      : base;
  });

  const displayedActions = createMemo(() =>
    view() === 'pending' ? sortOpenActionsForReview(actions()) : actions(),
  );

  const openAction = async (record: ActionAuditRecord) => {
    setDetailError('');
    try {
      setSelected(await ResourceActionsAPI.getAction(record.id));
    } catch (cause) {
      setDetailError(cause instanceof Error ? cause.message : 'Action details are unavailable.');
    }
  };

  return (
    <div class="mx-auto w-full max-w-6xl space-y-4 px-3 py-4 sm:px-5">
      <PageHeader
        title="Actions"
        description="Review proposed infrastructure changes and track their outcomes."
        actions={
          <Button
            variant="ghost"
            size="sm"
            aria-label="Refresh actions"
            onClick={() => void loadActions()}
            isLoading={loading()}
          >
            <RefreshCwIcon class="mr-2 h-4 w-4" />
            Refresh
          </Button>
        }
      />
      <Subtabs
        value={view()}
        onChange={(nextView) => setView(nextView as ActionInboxView)}
        tabs={[
          { value: 'pending', label: 'Open' },
          { value: 'settled', label: 'History' },
        ]}
        ariaLabel="Action views"
        trailing={
          <Show when={!loading() && actions().length > 0}>
            <span class="pb-2 text-xs text-muted">{queueSummary()}</span>
          </Show>
        }
      />

      <Show when={readOnly()}>
        <div class="flex items-center gap-2 rounded-md border border-border-subtle bg-surface-alt/50 px-3 py-2 text-xs text-muted">
          <EyeIcon class="h-4 w-4 shrink-0" aria-hidden="true" />
          <span>
            <strong class="font-medium text-base-content">Read-only demo data.</strong> You can
            inspect plans, policy evidence, and recorded outcomes.
          </span>
        </div>
      </Show>

      <Show when={detailError()}>
        <div
          role="alert"
          class="rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950/40 dark:text-amber-200"
        >
          {detailError()}
        </div>
      </Show>
      <Show when={loadError()}>
        <div
          role="alert"
          class="rounded-lg border border-red-300 bg-red-50 p-4 text-sm text-red-800 dark:bg-red-950/40 dark:text-red-200"
        >
          <p class="font-medium">Actions could not be loaded.</p>
          <p class="mt-1">{loadError()}</p>
          <Button class="mt-3" onClick={() => void loadActions()}>
            Try again
          </Button>
        </div>
      </Show>
      <Show when={!loading() && !loadError() && actions().length === 0}>
        <div
          data-testid="actions-calm-state"
          class="rounded-md border border-dashed border-border bg-surface p-6 text-center"
        >
          <h2 class="text-lg font-semibold">
            {view() === 'pending' ? 'No actions need attention' : 'No action history yet'}
          </h2>
          <p class="mt-2 text-sm text-muted">
            {view() === 'pending'
              ? 'Pulse will place proposed or in-progress changes here for review.'
              : 'Completed, rejected, expired, and failed actions will appear here.'}
          </p>
        </div>
      </Show>
      <Show when={actions().length > 0}>
        <Card padding="none" class="overflow-hidden" data-testid="actions-queue">
          <ul
            class="divide-y divide-border-subtle"
            aria-label={view() === 'pending' ? 'Open actions' : 'Action history'}
          >
            <For each={displayedActions()}>
              {(action) => {
                const state = () => getActionInboxStatePresentation(action.state);
                const resource = () => getActionResourcePresentation(action.request.resourceId);
                const title = () => formatActionName(action.request.capabilityName);
                return (
                  <li>
                    <button
                      type="button"
                      aria-label={`Review ${title()} on ${action.request.resourceId}, ${state().label}`}
                      class={`group w-full border-l-2 px-3 py-3 text-left transition-colors hover:bg-surface-hover/70 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500 sm:px-4 ${state().accentClass}`}
                      onClick={() => void openAction(action)}
                    >
                      <div class="flex items-center gap-3">
                        <div class="min-w-0 flex-1">
                          <div class="flex flex-wrap items-center gap-2">
                            <MetadataBadge
                              tone={state().tone}
                              size="xs"
                              shape="rounded"
                              appearance="outline"
                            >
                              {state().label}
                            </MetadataBadge>
                            <span class="truncate text-sm font-semibold text-base-content">
                              {title()}
                            </span>
                          </div>
                          <div
                            class="mt-1 flex min-w-0 items-center gap-1.5 text-xs text-muted"
                            title={action.request.resourceId}
                          >
                            <span class="truncate">{resource().label}</span>
                            <Show when={resource().detail}>
                              <span aria-hidden="true">·</span>
                              <span class="truncate font-mono text-[11px]">
                                {resource().detail}
                              </span>
                            </Show>
                            <span aria-hidden="true">·</span>
                            <span class="shrink-0">{formatRelativeTime(action.updatedAt)}</span>
                          </div>
                          <p class="mt-1.5 truncate text-sm text-muted">{action.request.reason}</p>
                        </div>
                        <span class="inline-flex shrink-0 items-center gap-1 rounded border border-border px-2 py-1 text-xs font-medium text-muted transition-colors group-hover:bg-surface group-hover:text-base-content">
                          Review
                          <ChevronRightIcon class="h-3.5 w-3.5" aria-hidden="true" />
                        </span>
                      </div>
                    </button>
                  </li>
                );
              }}
            </For>
          </ul>
        </Card>
      </Show>
      <ActionReviewDialog
        detail={selected()}
        onClose={() => setSelected(null)}
        onChanged={async (detail) => {
          setSelected(detail);
          await loadActions();
        }}
      />
    </div>
  );
}

export default Actions;
