import { A, useLocation } from '@solidjs/router';
import {
  type Accessor,
  createEffect,
  createMemo,
  createSignal,
  For,
  onCleanup,
  onMount,
  Show,
  untrack,
} from 'solid-js';
import AlertTriangleIcon from 'lucide-solid/icons/triangle-alert';
import ArrowLeftIcon from 'lucide-solid/icons/arrow-left';
import CheckCircleIcon from 'lucide-solid/icons/circle-check';
import ChevronLeftIcon from 'lucide-solid/icons/chevron-left';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import ClipboardCheckIcon from 'lucide-solid/icons/clipboard-check';
import ClockIcon from 'lucide-solid/icons/clock';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import RefreshIcon from 'lucide-solid/icons/refresh-cw';
import RotateCwIcon from 'lucide-solid/icons/rotate-cw';
import ShieldOffIcon from 'lucide-solid/icons/shield-off';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import XIcon from 'lucide-solid/icons/x';
import type {
  AttentionActionOffer,
  AttentionItem,
  AttentionItemDetail,
} from '@/api/patrolAttention';
import {
  acknowledgePatrolAttention,
  planPatrolAttentionAction,
  suppressPatrolAttention,
  unacknowledgePatrolAttention,
  unsuppressPatrolAttention,
} from '@/api/patrolAttention';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { Button, ButtonLink } from '@/components/shared/Button';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { MetadataBadge, type MetadataBadgeTone } from '@/components/shared/MetadataBadge';
import { ActionReviewDialog } from '@/features/actions/ActionReviewDialog';
import {
  getPatrolAttentionEvidencePresentation,
  getPatrolAttentionProtectionPresentation,
  getPatrolProtectionProviderLabels,
} from '@/features/patrol/patrolControlPresentation';
import { aiChatStore } from '@/stores/aiChat';
import { patrolAttentionStore } from '@/stores/patrolAttention';
import {
  ALERT_THRESHOLDS_PATH,
  buildPatrolAttentionPath,
  buildStandalonePath,
  buildWorkloadsRouteSearch,
  parsePatrolAttentionItemId,
} from '@/routing/resourceLinks';
import type { EvidenceEnvelope } from '@/types/operationalTrust';
import type { ActionDetailResponse } from '@/types/actionAudit';
import { getAlertResourceIncidentAcknowledgedByLabel } from '@/utils/alertIncidentPresentation';
import { formatRelativeTime } from '@/utils/format';
import type { PatrolAutonomyLevel } from '@/api/patrol';
import {
  partitionPatrolAttention,
  PATROL_AUTONOMY_EXPERIENCE,
  type PatrolAttentionDecision,
} from './patrolHomePresentation';

const PRIMARY_EVIDENCE_LIMIT = 3;
const PRIMARY_DECISION_LIMIT = 5;

const SEVERITY_PRIORITY: Record<AttentionItem['severity'], number> = {
  critical: 0,
  warning: 1,
  info: 2,
  unknown: 3,
};

export function sortPatrolAttentionDecisions(
  decisions: PatrolAttentionDecision[],
): PatrolAttentionDecision[] {
  return [...decisions].sort((left, right) => {
    const severityDelta =
      SEVERITY_PRIORITY[left.item.severity] - SEVERITY_PRIORITY[right.item.severity];
    if (severityDelta !== 0) return severityDelta;

    const leftActionable = left.item.availableActions.some(
      (action) => action.approval === 'required' || action.requiresApproval || action.actionId,
    );
    const rightActionable = right.item.availableActions.some(
      (action) => action.approval === 'required' || action.requiresApproval || action.actionId,
    );
    if (leftActionable !== rightActionable) return leftActionable ? -1 : 1;

    const recencyDelta =
      new Date(right.item.lastObservedAt).getTime() - new Date(left.item.lastObservedAt).getTime();
    if (recencyDelta !== 0) return recencyDelta;

    return left.item.id.localeCompare(right.item.id);
  });
}

export function PatrolAttentionWorkbench(
  props: {
    autonomyLevel?: PatrolAutonomyLevel;
    autonomyLocked?: boolean;
    pendingActionCount?: number;
    onOpenFindings?: () => void;
  } = {},
) {
  const location = useLocation();
  const [selectedItemId, setSelectedItemId] = createSignal('');
  const [actionDetail, setActionDetail] = createSignal<ActionDetailResponse | null>(null);
  const [actionBusy, setActionBusy] = createSignal(false);
  const [actionError, setActionError] = createSignal('');
  const [lifecycleBusy, setLifecycleBusy] = createSignal(false);
  const [lifecycleError, setLifecycleError] = createSignal('');
  const [reviewNotice, setReviewNotice] = createSignal('');
  const [reviewOrder, setReviewOrder] = createSignal<string[]>([]);
  const [showAllDecisions, setShowAllDecisions] = createSignal(false);
  const itemButtons = new Map<string, HTMLButtonElement>();
  let detailPanel: HTMLDivElement | undefined;
  let actionTrigger: HTMLButtonElement | undefined;

  const selectedDetail = () => patrolAttentionStore.selectedDetail();
  const summary = () => patrolAttentionStore.summary();
  const effectiveAutonomyLevel = createMemo<PatrolAutonomyLevel>(() =>
    props.autonomyLocked ? 'monitor' : (props.autonomyLevel ?? 'monitor'),
  );
  const autonomyExperience = createMemo(() => PATROL_AUTONOMY_EXPERIENCE[effectiveAutonomyLevel()]);
  const attention = createMemo(() =>
    partitionPatrolAttention(
      patrolAttentionStore.items(),
      props.autonomyLevel ?? 'monitor',
      props.autonomyLocked ?? false,
    ),
  );
  const sortedDecisions = createMemo(() => sortPatrolAttentionDecisions(attention().needsUser));
  const orderedDecisions = createMemo(() => {
    const current = sortedDecisions();
    const order = reviewOrder();
    if (order.length === 0) return current;

    const byId = new Map(current.map((decision) => [decision.item.id, decision]));
    const knownIds = new Set(order);
    return [
      ...order.flatMap((itemId) => {
        const decision = byId.get(itemId);
        return decision ? [decision] : [];
      }),
      ...current.filter((decision) => !knownIds.has(decision.item.id)),
    ];
  });
  const visibleDecisions = createMemo(() =>
    showAllDecisions() ? orderedDecisions() : orderedDecisions().slice(0, PRIMARY_DECISION_LIMIT),
  );
  const highestPriorityDecision = createMemo(() => orderedDecisions()[0]);
  const selectedDecisionIndex = createMemo(() =>
    orderedDecisions().findIndex((decision) => decision.item.id === selectedItemId()),
  );
  const previousDecision = createMemo(() => {
    const index = selectedDecisionIndex();
    return index > 0 ? orderedDecisions()[index - 1] : undefined;
  });
  const nextDecision = createMemo(() => {
    const index = selectedDecisionIndex();
    return index >= 0 ? orderedDecisions()[index + 1] : undefined;
  });
  const criticalDecisionCount = createMemo(
    () => attention().needsUser.filter((decision) => decision.item.severity === 'critical').length,
  );
  const briefingHeadline = createMemo(() => {
    if (patrolAttentionStore.loading() && !summary()) return 'Building your current briefing';
    const count = attention().needsUser.length;
    if (count === 0) return 'No decisions are waiting';
    return `${count} ${count === 1 ? 'decision needs' : 'decisions need'} you`;
  });

  const loadCurrentFilter = () => patrolAttentionStore.load(patrolAttentionStore.filter());
  const scrollDetailIntoView = () => {
    queueMicrotask(() => {
      detailPanel?.scrollTo?.({ top: 0 });
      if (window.matchMedia?.('(max-width: 1023px)').matches) {
        detailPanel?.scrollIntoView?.({ block: 'start' });
        detailPanel?.focus?.({ preventScroll: true });
      }
    });
  };
  const selectItem = (itemId: string, preserveNotice = false) => {
    if (!preserveNotice) setReviewNotice('');
    if (reviewOrder().length === 0) {
      setReviewOrder(sortedDecisions().map((decision) => decision.item.id));
    }
    setSelectedItemId(itemId);
    replaceAttentionLocation(itemId);
    void patrolAttentionStore.select(itemId);
    scrollDetailIntoView();
  };
  const closeDetail = () => {
    const previous = selectedItemId();
    setReviewOrder([]);
    setSelectedItemId('');
    replaceAttentionLocation('');
    void patrolAttentionStore.select(null);
    queueMicrotask(() => itemButtons.get(previous)?.focus());
  };
  const reviewAction = async (
    item: AttentionItem,
    offer: AttentionActionOffer,
    trigger: HTMLButtonElement,
  ) => {
    if (actionBusy()) return;
    actionTrigger = trigger;
    setActionBusy(true);
    setActionError('');
    try {
      const actionId =
        offer.actionId || (await planPatrolAttentionAction(item.id, offer.capability)).actionId;
      setActionDetail(await ResourceActionsAPI.getAction(actionId));
    } catch (cause) {
      setActionError(
        cause instanceof Error ? cause.message : 'The governed action could not be opened.',
      );
    } finally {
      setActionBusy(false);
    }
  };
  const closeActionReview = () => {
    setActionDetail(null);
    queueMicrotask(() => {
      const currentTrigger = detailPanel?.querySelector<HTMLButtonElement>(
        '[data-patrol-action-trigger]',
      );
      (currentTrigger ?? actionTrigger)?.focus();
    });
  };
  const actionChanged = async (next: ActionDetailResponse) => {
    setActionDetail(next);
    const selected = selectedItemId();
    await Promise.all([
      selected ? patrolAttentionStore.select(selected) : Promise.resolve(),
      patrolAttentionStore.load(patrolAttentionStore.filter()),
    ]);
  };
  const changeLifecycle = async (
    operation: () => Promise<unknown>,
    options: { advanceAfter?: boolean; successLabel?: string } = {},
  ) => {
    if (lifecycleBusy()) return;
    const selected = selectedItemId();
    const queueBefore = orderedDecisions();
    const selectedIndex = queueBefore.findIndex((decision) => decision.item.id === selected);
    const nextCandidateId =
      queueBefore[selectedIndex + 1]?.item.id ?? queueBefore[selectedIndex - 1]?.item.id ?? '';
    setLifecycleBusy(true);
    setLifecycleError('');
    try {
      await operation();
      await patrolAttentionStore.load(patrolAttentionStore.filter());
      if (options.advanceAfter) {
        const remaining = orderedDecisions();
        const next =
          remaining.find((decision) => decision.item.id === nextCandidateId) ?? remaining[0];
        const successLabel = options.successLabel ?? 'Decision updated';
        setReviewNotice(
          remaining.length > 0
            ? `${successLabel}. ${remaining.length} ${remaining.length === 1 ? 'decision remains' : 'decisions remain'}.`
            : `${successLabel}. Your decision inbox is clear.`,
        );
        if (next) {
          selectItem(next.item.id, true);
        } else {
          closeDetail();
        }
      } else if (selected) {
        await patrolAttentionStore.select(selected);
      }
    } catch (cause) {
      setLifecycleError(
        cause instanceof Error ? cause.message : 'The lifecycle change could not be saved.',
      );
    } finally {
      setLifecycleBusy(false);
    }
  };

  onMount(() => {
    void patrolAttentionStore.load('active');
    const interval = window.setInterval(loadCurrentFilter, 30000);
    onCleanup(() => window.clearInterval(interval));
  });

  createEffect(() => {
    const deepLinkedItem = parsePatrolAttentionItemId(location.search);
    const currentItem = untrack(selectedItemId);
    if (deepLinkedItem && deepLinkedItem !== currentItem) {
      setShowAllDecisions(true);
      setSelectedItemId(deepLinkedItem);
      void patrolAttentionStore.select(deepLinkedItem);
      scrollDetailIntoView();
    } else if (!deepLinkedItem && currentItem) {
      setReviewOrder([]);
      setSelectedItemId('');
      void patrolAttentionStore.select(null);
    }
  });

  createEffect(() => {
    if (selectedItemId() && sortedDecisions().length > 0 && reviewOrder().length === 0) {
      setReviewOrder(sortedDecisions().map((decision) => decision.item.id));
    }
  });

  const decisionCountLabel = createMemo(() => {
    const count = attention().needsUser.length;
    return `${count} ${count === 1 ? 'item requires' : 'items require'} review`;
  });

  return (
    <section
      aria-labelledby="patrol-attention-heading"
      class="overflow-hidden rounded-xl border border-border bg-surface shadow-sm"
    >
      <div class="border-b border-border bg-blue-50/50 px-4 py-5 dark:bg-blue-950/20 sm:px-6 sm:py-6">
        <div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
          <div class="min-w-0 flex-1">
            <p class="text-[11px] font-semibold uppercase tracking-[0.16em] text-blue-700 dark:text-blue-300">
              Today's Patrol briefing
            </p>
            <h2
              id="patrol-attention-heading"
              class="mt-2 text-base font-semibold text-base-content"
            >
              Needs your attention
            </h2>
            <p class="mt-1 text-2xl font-semibold tracking-tight text-base-content sm:text-3xl">
              {briefingHeadline()}
            </p>
            <p class="mt-2 max-w-3xl text-sm leading-6 text-muted">
              {attention().needsUser.length > 0
                ? 'Patrol has already ordered the queue by severity, actionability, and the freshest evidence.'
                : autonomyExperience().needsYouDescription}
            </p>
          </div>
          <div class="w-full lg:w-[32rem]">
            <div class="flex flex-wrap items-center gap-2 lg:justify-end">
              <Show when={!selectedItemId() && highestPriorityDecision()}>
                {(decision) => (
                  <Button
                    variant="primary"
                    size="sm"
                    class="gap-1.5"
                    onClick={() => selectItem(decision().item.id)}
                  >
                    Start review
                    <ChevronRightIcon class="h-4 w-4" aria-hidden="true" />
                  </Button>
                )}
              </Show>
              <Show when={(props.pendingActionCount ?? 0) > 0}>
                <ButtonLink href="/actions" variant="secondary" size="sm" class="gap-1.5">
                  <ClipboardCheckIcon class="h-4 w-4" aria-hidden="true" />
                  Review {props.pendingActionCount}{' '}
                  {props.pendingActionCount === 1 ? 'approval' : 'approvals'}
                </ButtonLink>
              </Show>
              <Button
                variant="ghost"
                size="sm"
                class="gap-1.5"
                onClick={() => void loadCurrentFilter()}
                disabled={patrolAttentionStore.loading()}
                aria-label="Refresh Patrol attention"
              >
                <RefreshIcon
                  class={`h-4 w-4 ${patrolAttentionStore.loading() ? 'motion-safe:animate-spin' : ''}`}
                  aria-hidden="true"
                />
                Refresh
              </Button>
            </div>
            <Show when={summary()}>
              <div
                class="mt-4 grid grid-cols-3 overflow-hidden rounded-lg border border-border bg-surface/80 shadow-sm"
                aria-label={decisionCountLabel()}
              >
                <div class="px-3 py-3 sm:px-4">
                  <span class="block text-xl font-semibold text-red-700 dark:text-red-300">
                    {criticalDecisionCount()}
                  </span>
                  <span class="mt-0.5 block text-[11px] font-medium uppercase tracking-wide text-muted">
                    Critical
                  </span>
                </div>
                <div class="border-l border-border px-3 py-3 sm:px-4">
                  <span class="block text-xl font-semibold text-base-content">
                    {attention().needsUser.length}
                  </span>
                  <span class="mt-0.5 block text-[11px] font-medium uppercase tracking-wide text-muted">
                    Decisions
                  </span>
                </div>
                <div class="border-l border-border px-3 py-3 sm:px-4">
                  <span class="block text-xl font-semibold text-amber-700 dark:text-amber-300">
                    {props.pendingActionCount ?? 0}
                  </span>
                  <span class="mt-0.5 block text-[11px] font-medium uppercase tracking-wide text-muted">
                    Approvals
                  </span>
                </div>
              </div>
            </Show>
          </div>
        </div>
        <Show when={attention().quiet.length > 0}>
          <p class="mt-3 text-xs leading-5 text-muted">
            {attention().quiet.length} other current{' '}
            {attention().quiet.length === 1 ? 'issue is' : 'issues are'} continuing without a
            decision under {autonomyExperience().label}.
          </p>
        </Show>
      </div>

      <Show when={reviewNotice()}>
        {(notice) => (
          <div
            role="status"
            class="flex items-center gap-2 border-b border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200 sm:px-6"
          >
            <CheckCircleIcon class="h-4 w-4 shrink-0" aria-hidden="true" />
            {notice()}
          </div>
        )}
      </Show>

      <div
        class={`grid min-w-0 ${attention().needsUser.length > 0 ? 'lg:grid-cols-[minmax(20rem,0.78fr)_minmax(0,1.22fr)]' : ''}`}
      >
        <div
          class={`min-w-0 ${selectedItemId() ? 'hidden lg:block' : ''} ${attention().needsUser.length > 0 ? 'lg:max-h-[52rem] lg:overflow-y-auto lg:border-r lg:border-border' : ''}`}
        >
          <Show when={attention().needsUser.length > 0}>
            <div class="sticky top-0 z-10 flex items-center justify-between gap-3 border-b border-border bg-surface/95 px-4 py-3 backdrop-blur sm:px-5">
              <div>
                <h3 class="text-sm font-semibold text-base-content">Decision inbox</h3>
                <p class="mt-0.5 text-[11px] text-muted">Highest priority first</p>
              </div>
              <MetadataBadge tone="neutral" size="xs" shape="rounded">
                {attention().needsUser.length} open
              </MetadataBadge>
            </div>
          </Show>
          <AttentionList
            decisions={visibleDecisions()}
            selectedItemId={selectedItemId()}
            itemButtons={itemButtons}
            onSelect={selectItem}
          />
          <Show when={attention().needsUser.length > PRIMARY_DECISION_LIMIT}>
            <div class="border-t border-border px-4 py-3 text-center sm:px-5">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setShowAllDecisions((current) => !current)}
              >
                {showAllDecisions()
                  ? 'Show fewer decisions'
                  : `Show all ${attention().needsUser.length} decisions`}
              </Button>
            </div>
          </Show>
        </div>
        <Show
          when={selectedItemId()}
          fallback={
            <Show when={highestPriorityDecision()}>
              {(decision) => (
                <div class="hidden min-h-[30rem] flex-col items-start justify-start bg-surface-alt/20 px-8 py-8 text-left lg:flex">
                  <span class="inline-flex h-12 w-12 items-center justify-center rounded-xl border border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-300">
                    <SparklesIcon class="h-5 w-5" aria-hidden="true" />
                  </span>
                  <p class="mt-5 text-[11px] font-semibold uppercase tracking-[0.14em] text-muted">
                    Start here
                  </p>
                  <h3 class="mt-2 max-w-lg text-lg font-semibold text-base-content">
                    {decision().item.title}
                  </h3>
                  <p class="mt-2 max-w-lg text-sm leading-6 text-muted">
                    {decision().item.plainLanguageSummary}
                  </p>
                  <Button
                    variant="primary"
                    size="md"
                    class="mt-5 gap-2"
                    onClick={() => selectItem(decision().item.id)}
                  >
                    Review this decision
                    <ChevronRightIcon class="h-4 w-4" aria-hidden="true" />
                  </Button>
                  <p class="mt-4 max-w-md text-xs leading-5 text-muted">
                    Patrol chose this from severity, available action, and evidence recency. It will
                    not make a change from this screen without the governed action flow.
                  </p>
                </div>
              )}
            </Show>
          }
        >
          <div
            ref={detailPanel}
            tabindex="-1"
            class="order-first min-w-0 scroll-mt-4 lg:order-none lg:max-h-[52rem] lg:overflow-y-auto"
          >
            <AttentionDetail
              detail={selectedDetail()}
              loading={patrolAttentionStore.detailLoading()}
              onClose={closeDetail}
              actionBusy={actionBusy()}
              actionError={actionError()}
              onReviewAction={reviewAction}
              lifecycleBusy={lifecycleBusy()}
              lifecycleError={lifecycleError()}
              queuePosition={() =>
                selectedDecisionIndex() >= 0 ? selectedDecisionIndex() + 1 : undefined
              }
              queueCount={() => orderedDecisions().length}
              canPrevious={() => Boolean(previousDecision())}
              canNext={() => Boolean(nextDecision())}
              onPrevious={() => {
                const previous = previousDecision();
                if (previous) selectItem(previous.item.id);
              }}
              onNext={() => {
                const next = nextDecision();
                if (next) selectItem(next.item.id);
              }}
              onAcknowledge={(itemId) =>
                changeLifecycle(() => acknowledgePatrolAttention(itemId), {
                  advanceAfter: true,
                  successLabel: 'Marked reviewed',
                })
              }
              onUnacknowledge={(itemId) =>
                changeLifecycle(() => unacknowledgePatrolAttention(itemId))
              }
              onSuppress={(itemId, reason, expiresAt) =>
                changeLifecycle(() => suppressPatrolAttention(itemId, reason, expiresAt), {
                  advanceAfter: true,
                  successLabel: 'Suppressed temporarily',
                })
              }
              onUnsuppress={(itemId) => changeLifecycle(() => unsuppressPatrolAttention(itemId))}
              onOpenFindings={props.onOpenFindings}
            />
          </div>
        </Show>
      </div>
      <ActionReviewDialog
        detail={actionDetail()}
        onClose={closeActionReview}
        onChanged={actionChanged}
      />
    </section>
  );
}

function replaceAttentionLocation(itemId: string) {
  const nextPath = buildPatrolAttentionPath(itemId);
  if (typeof window !== 'undefined') {
    window.history.replaceState(window.history.state, '', nextPath);
  }
}

function AttentionList(props: {
  decisions: PatrolAttentionDecision[];
  selectedItemId: string;
  itemButtons: Map<string, HTMLButtonElement>;
  onSelect: (itemId: string) => void;
}) {
  return (
    <div aria-live="polite">
      <Show when={patrolAttentionStore.error()}>
        {(message) => (
          <div class="m-4 flex items-start gap-3 rounded-md border border-red-200 bg-red-50 p-4 text-red-800 dark:border-red-900 dark:bg-red-950/30 dark:text-red-200">
            <AlertTriangleIcon class="mt-0.5 h-5 w-5 shrink-0" aria-hidden="true" />
            <div>
              <h3 class="text-sm font-semibold">Patrol attention is unavailable</h3>
              <p class="mt-1 text-xs leading-5">{message()}</p>
              <p class="mt-1 text-xs leading-5">
                Pulse has not inferred a calm or healthy state from this failure.
              </p>
            </div>
          </div>
        )}
      </Show>

      <Show
        when={!patrolAttentionStore.loading() || props.decisions.length > 0}
        fallback={
          <div class="flex min-h-40 items-center justify-center gap-2 text-sm text-muted">
            <LoadingSpinner size="sm" />
            Loading current attention
          </div>
        }
      >
        <Show
          when={props.decisions.length > 0}
          fallback={<AttentionEmptyState hasQuietWork={patrolAttentionStore.items().length > 0} />}
        >
          <ul class="divide-y divide-border" aria-label="Patrol attention items">
            <For each={props.decisions}>
              {(decision) => {
                const item = decision.item;
                const decisionLabel = () => {
                  if (
                    item.availableActions.some(
                      (action) => action.approval === 'required' || action.requiresApproval,
                    )
                  ) {
                    return 'Approval needed';
                  }
                  if (item.verificationState === 'failed' || item.verificationState === 'unknown') {
                    return 'Verify result';
                  }
                  return 'Review';
                };
                return (
                  <li>
                    <button
                      ref={(element) => props.itemButtons.set(item.id, element)}
                      type="button"
                      class={`w-full px-4 py-3 text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 sm:px-5 ${
                        props.selectedItemId === item.id
                          ? 'bg-blue-50/80 dark:bg-blue-950/30'
                          : 'hover:bg-surface-hover'
                      }`}
                      aria-pressed={props.selectedItemId === item.id}
                      aria-label={`Open ${item.title}`}
                      onClick={() => props.onSelect(item.id)}
                    >
                      <div class="flex min-w-0 items-start gap-3">
                        <div class="min-w-0 flex-1">
                          <div class="flex min-w-0 flex-wrap items-center gap-2">
                            <MetadataBadge tone={severityTone(item)} size="xs" shape="rounded">
                              {formatLabel(item.severity)}
                            </MetadataBadge>
                            <span class="min-w-0 flex-1 truncate text-sm font-semibold text-base-content">
                              {item.title}
                            </span>
                            <StateBadge item={item} />
                          </div>
                          <p class="mt-1 line-clamp-2 text-xs leading-5 text-muted">
                            {item.plainLanguageSummary}
                          </p>
                          <div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted">
                            <span
                              class="font-semibold text-amber-700 dark:text-amber-300"
                              title={decision.reason}
                            >
                              {decisionLabel()}
                            </span>
                            <span class="truncate font-medium text-base-content">
                              {item.subjectResourceName}
                            </span>
                            <span>
                              Last seen {formatRelativeTime(item.lastObservedAt, { compact: true })}
                            </span>
                          </div>
                        </div>
                        <ChevronRightIcon
                          class="mt-1 h-4 w-4 shrink-0 text-muted"
                          aria-hidden="true"
                        />
                      </div>
                    </button>
                  </li>
                );
              }}
            </For>
          </ul>
        </Show>
      </Show>
    </div>
  );
}

function AttentionEmptyState(props: { hasQuietWork: boolean }) {
  const summary = () => patrolAttentionStore.summary();
  const activeFilter = () => patrolAttentionStore.filter() === 'active';
  const trustworthyCalm = () =>
    activeFilter() &&
    summary()?.calm === true &&
    summary()?.coverageState === 'current' &&
    !patrolAttentionStore.error();

  return (
    <div class="flex min-h-52 flex-col items-center justify-center px-6 py-10 text-center">
      <Show
        when={trustworthyCalm() || props.hasQuietWork}
        fallback={
          <>
            <ClockIcon class="h-8 w-8 text-muted" aria-hidden="true" />
            <h3 class="mt-3 text-sm font-semibold text-base-content">No items in this view</h3>
            <p class="mt-1 max-w-md text-xs leading-5 text-muted">
              {summary()?.coverageState === 'partial'
                ? 'The lifecycle queue is empty, but protection context is incomplete. Pulse is not treating that gap as proof of health.'
                : 'Choose another lifecycle filter or refresh the current evaluation.'}
            </p>
          </>
        }
      >
        <CheckCircleIcon class="h-9 w-9 text-emerald-500" aria-hidden="true" />
        <h3 class="mt-3 text-sm font-semibold text-base-content">Nothing needs you right now</h3>
        <p class="mt-1 max-w-md text-xs leading-5 text-muted">
          {props.hasQuietWork
            ? 'Patrol can continue with the current issues under this mode.'
            : 'The current operational evaluation has no active items.'}
          <Show when={summary()?.evaluatedAt}>
            {(evaluatedAt) => ` Checked ${formatRelativeTime(evaluatedAt(), { compact: true })}.`}
          </Show>
        </p>
      </Show>
    </div>
  );
}

function AttentionDetail(props: {
  detail: AttentionItemDetail | null;
  loading: boolean;
  onClose: () => void;
  actionBusy: boolean;
  actionError: string;
  onReviewAction: (
    item: AttentionItem,
    offer: AttentionActionOffer,
    trigger: HTMLButtonElement,
  ) => void;
  lifecycleBusy: boolean;
  lifecycleError: string;
  queuePosition: Accessor<number | undefined>;
  queueCount: Accessor<number>;
  canPrevious: Accessor<boolean>;
  canNext: Accessor<boolean>;
  onPrevious: () => void;
  onNext: () => void;
  onAcknowledge: (itemId: string) => Promise<void>;
  onUnacknowledge: (itemId: string) => Promise<void>;
  onSuppress: (itemId: string, reason: string, expiresAt: string) => Promise<void>;
  onUnsuppress: (itemId: string) => Promise<void>;
  onOpenFindings?: () => void;
}) {
  const detail = () => props.detail;
  const item = () => detail()?.item;
  const orderedEvidence = createMemo(() =>
    [...(detail()?.evidence ?? [])].sort(
      (left, right) => new Date(right.observedAt).getTime() - new Date(left.observedAt).getTime(),
    ),
  );
  const primaryEvidence = createMemo(() => orderedEvidence().slice(0, PRIMARY_EVIDENCE_LIMIT));
  const olderEvidence = createMemo(() => orderedEvidence().slice(PRIMARY_EVIDENCE_LIMIT));
  const resourceHref = () => {
    const value = item();
    if (!value) return buildStandalonePath('machines');
    return `${buildStandalonePath('machines')}${buildWorkloadsRouteSearch({
      resource: value.subjectResourceId,
    })}`;
  };
  const openAssistant = () => {
    const value = detail();
    if (!value) return;
    const current = value.item;
    const evidence = value.evidence.map(
      (entry) =>
        `${entry.source.provider}/${entry.source.collector}: ${entry.completeness}, ${entry.confidence}, observed ${entry.observedAt}`,
    );
    aiChatStore.open({
      targetType: current.subjectResourceType || 'resource',
      targetId: current.subjectResourceId,
      autonomousMode: false,
      handoffResources: [
        {
          id: current.subjectResourceId,
          name: current.subjectResourceName,
          type: current.subjectResourceType,
        },
      ],
      briefing: {
        sourceLabel: 'Pulse Patrol',
        title: 'Selected attention item',
        subject: current.title,
        statusLabel: `${formatLabel(current.severity)} · ${formatLabel(current.state)}`,
        detailLines: [
          current.plainLanguageSummary,
          current.impact ? `Impact: ${current.impact}` : undefined,
          current.recommendedNextStep ? `Next step: ${current.recommendedNextStep}` : undefined,
        ].filter((line): line is string => Boolean(line)),
        evidence: evidence.slice(0, 5),
        actionLabel: `Explain ${current.title}`,
        safetyNote:
          'This context explains evidence only. It does not grant approval or action authority.',
      },
      handoffContext: [
        `Attention Item: ${current.id}`,
        `Operational Record: ${current.operationalRecordId}`,
        `Resource: ${current.subjectResourceName} (${current.subjectResourceId})`,
        `State: ${current.state}`,
        `Severity: ${current.severity}`,
        `Summary: ${current.plainLanguageSummary}`,
        `Evidence: ${current.evidenceFreshness}/${current.evidenceCompleteness}`,
        current.impact ? `Impact: ${current.impact}` : '',
        current.recommendedNextStep ? `Recommended Next Step: ${current.recommendedNextStep}` : '',
        'Authority Boundary: Explain selected evidence only. Do not infer capabilities or bypass approval.',
      ]
        .filter(Boolean)
        .join('\n'),
      context: {
        attentionItemId: current.id,
        operationalRecordId: current.operationalRecordId,
        lifecycleState: current.state,
        evidenceFreshness: current.evidenceFreshness,
        evidenceCompleteness: current.evidenceCompleteness,
        protectionPosture: current.protectionPosture,
      },
    });
  };

  return (
    <aside
      class="min-w-0 bg-surface"
      aria-labelledby="attention-detail-title"
      aria-busy={props.loading}
    >
      <div class="sticky top-0 z-20 border-b border-border bg-surface/95 px-4 py-3 backdrop-blur sm:px-5">
        <div class="flex items-center justify-between gap-3">
          <button
            type="button"
            class="inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded px-2 text-sm font-medium text-muted hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 lg:hidden"
            aria-label="Back to attention list"
            onClick={props.onClose}
          >
            <ArrowLeftIcon class="h-4 w-4" aria-hidden="true" />
            Back to list
          </button>
          <p class="text-[11px] font-semibold uppercase tracking-wider text-muted">
            <Show when={props.queuePosition()} fallback="Decision context">
              {(position) => `Decision ${position()} of ${props.queueCount()}`}
            </Show>
          </p>
          <div class="hidden items-center gap-1 lg:flex">
            <button
              type="button"
              class="inline-flex h-8 w-8 items-center justify-center rounded text-muted hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-35"
              aria-label="Previous decision"
              title="Previous decision"
              disabled={!props.canPrevious()}
              onClick={props.onPrevious}
            >
              <ChevronLeftIcon class="h-4 w-4" aria-hidden="true" />
            </button>
            <button
              type="button"
              class="inline-flex h-8 w-8 items-center justify-center rounded text-muted hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-35"
              aria-label="Next decision"
              title="Next decision"
              disabled={!props.canNext()}
              onClick={props.onNext}
            >
              <ChevronRightIcon class="h-4 w-4" aria-hidden="true" />
            </button>
            <span class="mx-1 h-4 w-px bg-border" aria-hidden="true" />
            <button
              type="button"
              class="inline-flex h-8 w-8 items-center justify-center rounded text-muted hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              aria-label="Close attention detail"
              onClick={props.onClose}
            >
              <XIcon class="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
        </div>
        <h3 id="attention-detail-title" class="mt-1 text-sm font-semibold text-base-content">
          {item()?.title ?? 'Loading attention item'}
        </h3>
        <Show when={props.queuePosition() && props.queueCount() > 1}>
          <div class="mt-3 grid grid-cols-2 gap-2 lg:hidden" aria-label="Review queue navigation">
            <Button
              variant="secondary"
              size="sm"
              class="min-h-11 gap-1.5"
              disabled={!props.canPrevious()}
              onClick={props.onPrevious}
            >
              <ChevronLeftIcon class="h-4 w-4" aria-hidden="true" />
              Previous
            </Button>
            <Button
              variant="secondary"
              size="sm"
              class="min-h-11 gap-1.5"
              disabled={!props.canNext()}
              onClick={props.onNext}
            >
              Next decision
              <ChevronRightIcon class="h-4 w-4" aria-hidden="true" />
            </Button>
          </div>
        </Show>
      </div>

      <Show
        when={!props.loading && detail()}
        fallback={
          <div class="flex min-h-48 items-center justify-center gap-2 text-sm text-muted">
            <LoadingSpinner size="sm" />
            Loading evidence
          </div>
        }
      >
        {(loaded) => (
          <div class="space-y-5 px-4 py-4 sm:px-5">
            <section>
              <div class="flex flex-wrap items-center gap-2">
                <StateBadge item={loaded().item} />
                <EvidenceLabel item={loaded().item} badge />
                <ProtectionLabel item={loaded().item} badge />
              </div>
              <p class="mt-3 text-sm leading-6 text-base-content">
                {loaded().item.plainLanguageSummary}
              </p>
            </section>

            <DetailSection title="Affected resource">
              <p class="text-sm font-medium text-base-content">
                {loaded().item.subjectResourceName}
              </p>
              <p class="mt-1 break-all text-xs text-muted">{loaded().item.subjectResourceId}</p>
              <Show when={loaded().item.relatedResources.length > 0}>
                <p class="mt-2 text-xs text-muted">
                  {loaded().item.relatedResources.length} related{' '}
                  {loaded().item.relatedResources.length === 1 ? 'resource' : 'resources'}
                </p>
              </Show>
            </DetailSection>

            <Show when={loaded().item.impact || loaded().item.recommendedNextStep}>
              <DetailSection title="What to do">
                <Show when={loaded().item.impact}>
                  {(impact) => <p class="text-xs leading-5 text-muted">Impact: {impact()}</p>}
                </Show>
                <Show when={loaded().item.recommendedNextStep}>
                  {(nextStep) => (
                    <p class="mt-2 text-sm font-medium leading-5 text-base-content">{nextStep()}</p>
                  )}
                </Show>
              </DetailSection>
            </Show>

            <Show when={loaded().item.availableActions[0]}>
              {(offer) => (
                <DetailSection title="Safe action">
                  <div class="rounded-md border border-blue-200 bg-blue-50/70 p-3 dark:border-blue-900 dark:bg-blue-950/30">
                    <div class="flex items-start gap-2">
                      <RotateCwIcon
                        class="mt-0.5 h-4 w-4 shrink-0 text-blue-600 dark:text-blue-300"
                        aria-hidden="true"
                      />
                      <div class="min-w-0">
                        <p class="text-sm font-semibold text-base-content">{offer().label}</p>
                        <p class="mt-1 text-xs leading-5 text-muted">
                          {offer().expectedPostcondition}{' '}
                          {attentionActionGuidance(loaded().item, offer())}
                        </p>
                        <ActionVerificationMessage state={loaded().item.verificationState} />
                        <Show when={props.actionError}>
                          <p
                            role="alert"
                            class="mt-2 text-xs leading-5 text-red-700 dark:text-red-300"
                          >
                            {props.actionError}
                          </p>
                        </Show>
                        <Button
                          variant="primary"
                          size="sm"
                          class="mt-3 min-h-11 gap-1.5 sm:min-h-0"
                          data-patrol-action-trigger
                          isLoading={props.actionBusy}
                          onClick={(event) =>
                            props.onReviewAction(loaded().item, offer(), event.currentTarget)
                          }
                        >
                          <RotateCwIcon class="h-4 w-4" aria-hidden="true" />
                          {offer().actionId ? 'Review action' : 'Review and approve'}
                        </Button>
                      </div>
                    </div>
                  </div>
                </DetailSection>
              )}
            </Show>

            <AttentionLifecycleControls
              detail={loaded()}
              busy={props.lifecycleBusy}
              error={props.lifecycleError}
              onAcknowledge={props.onAcknowledge}
              onUnacknowledge={props.onUnacknowledge}
              onSuppress={props.onSuppress}
              onUnsuppress={props.onUnsuppress}
              onOpenFindings={props.onOpenFindings}
            />

            <DetailSection title="Evidence">
              <Show
                when={orderedEvidence().length > 0}
                fallback={
                  <p class="text-xs leading-5 text-amber-700 dark:text-amber-300">
                    Evidence detail is unavailable. Pulse has not presented this as confirmed.
                  </p>
                }
              >
                <Show when={olderEvidence().length > 0}>
                  <p class="mb-2 text-xs text-muted">
                    Showing the latest {primaryEvidence().length} of {orderedEvidence().length}{' '}
                    observations.
                  </p>
                </Show>
                <ul class="space-y-2">
                  <For each={primaryEvidence()}>
                    {(evidence) => <EvidenceObservation evidence={evidence} />}
                  </For>
                </ul>
                <Show when={olderEvidence().length > 0}>
                  <details class="mt-2 rounded-md border border-border-subtle bg-surface-alt/30">
                    <summary class="flex min-h-11 cursor-pointer items-center px-3 py-2 text-xs font-medium text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 sm:min-h-0">
                      Show {olderEvidence().length} older{' '}
                      {olderEvidence().length === 1 ? 'observation' : 'observations'}
                    </summary>
                    <ul class="space-y-2 border-t border-border-subtle p-2">
                      <For each={olderEvidence()}>
                        {(evidence) => <EvidenceObservation evidence={evidence} />}
                      </For>
                    </ul>
                  </details>
                </Show>
              </Show>
            </DetailSection>

            <DetailSection title="Protection">
              <Show
                when={loaded().item.protectionPosture}
                fallback={
                  <p class="text-xs leading-5 text-muted">
                    No current protection posture is attached to this resource. This means unknown,
                    not unprotected.
                  </p>
                }
              >
                {(posture) => (
                  <>
                    <p class="text-xs leading-5 text-muted">{posture().explanation}</p>
                    <Show when={posture().providerStates.length > 0}>
                      <ul class="mt-2 space-y-2">
                        <For each={posture().providerStates}>
                          {(provider) => (
                            <li class="rounded-md border border-border-subtle px-3 py-2 text-xs">
                              <span class="font-medium text-base-content">
                                {formatProvider(provider.provider)}
                              </span>
                              <ProtectionProviderMetadata provider={provider} />
                            </li>
                          )}
                        </For>
                      </ul>
                    </Show>
                  </>
                )}
              </Show>
            </DetailSection>

            <DetailSection title="Timeline">
              <Show
                when={loaded().timeline.length > 0}
                fallback={
                  <p class="text-xs leading-5 text-muted">
                    No earlier lifecycle transitions are recorded. This is the first observed state.
                  </p>
                }
              >
                <ol class="space-y-3">
                  <For each={loaded().timeline}>
                    {(transition) => (
                      <li class="border-l-2 border-border pl-3">
                        <p class="text-xs font-medium text-base-content">
                          {formatLabel(transition.from)} to {formatLabel(transition.to)}
                        </p>
                        <p class="mt-0.5 text-[11px] text-muted">
                          {formatRelativeTime(transition.at, { compact: true })} ·{' '}
                          {formatLabel(transition.cause)}
                        </p>
                        <Show when={transition.reason}>
                          {(reason) => <p class="mt-1 text-xs leading-5 text-muted">{reason()}</p>}
                        </Show>
                      </li>
                    )}
                  </For>
                </ol>
              </Show>
            </DetailSection>

            <div class="flex flex-wrap gap-2 border-t border-border pt-4">
              <ButtonLink
                href={resourceHref()}
                variant="secondary"
                size="sm"
                class="min-h-11 gap-1.5 sm:min-h-0"
              >
                <ExternalLinkIcon class="h-4 w-4" aria-hidden="true" />
                Open resource
              </ButtonLink>
              <Show when={aiChatStore.enabled === true}>
                <Button
                  variant="secondary"
                  size="sm"
                  class="min-h-11 gap-1.5 sm:min-h-0"
                  onClick={openAssistant}
                >
                  <SparklesIcon class="h-4 w-4" aria-hidden="true" />
                  Explain with Assistant
                </Button>
              </Show>
            </div>
          </div>
        )}
      </Show>
    </aside>
  );
}

const SUPPRESSION_DURATIONS = [
  { value: 60 * 60 * 1000, label: '1 hour' },
  { value: 24 * 60 * 60 * 1000, label: '24 hours' },
  { value: 7 * 24 * 60 * 60 * 1000, label: '7 days' },
] as const;

function AttentionLifecycleControls(props: {
  detail: AttentionItemDetail;
  busy: boolean;
  error: string;
  onAcknowledge: (itemId: string) => Promise<void>;
  onUnacknowledge: (itemId: string) => Promise<void>;
  onSuppress: (itemId: string, reason: string, expiresAt: string) => Promise<void>;
  onUnsuppress: (itemId: string) => Promise<void>;
  onOpenFindings?: () => void;
}) {
  const [showSuppression, setShowSuppression] = createSignal(false);
  const [reason, setReason] = createSignal('');
  const [durationMs, setDurationMs] = createSignal<number>(SUPPRESSION_DURATIONS[1].value);
  const state = () => props.detail.item.state;
  const canAcknowledge = () => ['open', 'stale', 'unknown', 'resolving'].includes(state());
  const canSuppress = () =>
    ['open', 'acknowledged', 'stale', 'unknown', 'resolving'].includes(state());
  const submitSuppression = async (event: SubmitEvent) => {
    event.preventDefault();
    const value = reason().trim();
    if (!value) return;
    await props.onSuppress(
      props.detail.item.id,
      value,
      new Date(Date.now() + durationMs()).toISOString(),
    );
    setShowSuppression(false);
    setReason('');
  };

  return (
    <DetailSection title="Lifecycle">
      <div class="rounded-md border border-border-subtle bg-surface-alt/40 p-3">
        <Show when={props.detail.operationalRecord.acknowledgement}>
          {(acknowledgement) => (
            <p class="mb-2 text-xs leading-5 text-muted">
              {getAlertResourceIncidentAcknowledgedByLabel(acknowledgement().by)}{' '}
              {formatRelativeTime(acknowledgement().at, { compact: true })}.
            </p>
          )}
        </Show>
        <Show when={props.detail.operationalRecord.suppression}>
          {(suppression) => (
            <div class="mb-3 text-xs leading-5 text-muted">
              <p>
                Suppressed by {suppression().by}: {suppression().reason}
              </p>
              <Show when={suppression().expiresAt}>
                {(expiresAt) => (
                  <p>
                    Returns to active attention {formatRelativeTime(expiresAt(), { compact: true })}
                    .
                  </p>
                )}
              </Show>
            </div>
          )}
        </Show>
        <div class="flex flex-wrap gap-2">
          <Show when={canAcknowledge()}>
            <Button
              variant="primary"
              size="sm"
              class="min-h-11 sm:min-h-0"
              isLoading={props.busy}
              onClick={() => void props.onAcknowledge(props.detail.item.id)}
            >
              Mark reviewed
            </Button>
          </Show>
          <Show when={state() === 'acknowledged'}>
            <Button
              variant="secondary"
              size="sm"
              class="min-h-11 sm:min-h-0"
              isLoading={props.busy}
              onClick={() => void props.onUnacknowledge(props.detail.item.id)}
            >
              Return to decision inbox
            </Button>
          </Show>
          <Show when={state() === 'suppressed'}>
            <Button
              variant="secondary"
              size="sm"
              class="min-h-11 sm:min-h-0"
              isLoading={props.busy}
              onClick={() => void props.onUnsuppress(props.detail.item.id)}
            >
              Return to active attention
            </Button>
          </Show>
          <Show when={canSuppress() && !showSuppression()}>
            <Button
              variant="secondary"
              size="sm"
              class="min-h-11 gap-1.5 sm:min-h-0"
              disabled={props.busy}
              onClick={() => setShowSuppression(true)}
            >
              <ShieldOffIcon class="h-4 w-4" aria-hidden="true" />
              Suppress temporarily
            </Button>
          </Show>
        </div>
        <Show when={showSuppression()}>
          <form
            class="mt-3 space-y-3 border-t border-border-subtle pt-3"
            onSubmit={submitSuppression}
          >
            <div>
              <label
                for={`attention-suppression-reason-${props.detail.item.id}`}
                class="text-xs font-medium text-base-content"
              >
                Why is this safe to hide from active attention?
              </label>
              <textarea
                id={`attention-suppression-reason-${props.detail.item.id}`}
                class="mt-1 min-h-20 w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={reason()}
                maxlength={240}
                required
                disabled={props.busy}
                onInput={(event) => setReason(event.currentTarget.value)}
              />
            </div>
            <div>
              <label
                for={`attention-suppression-duration-${props.detail.item.id}`}
                class="text-xs font-medium text-base-content"
              >
                Return it to active attention after
              </label>
              <select
                id={`attention-suppression-duration-${props.detail.item.id}`}
                class="mt-1 block min-h-11 rounded-md border border-border bg-surface px-3 py-1.5 text-sm text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500 sm:min-h-9"
                value={String(durationMs())}
                disabled={props.busy}
                onChange={(event) => setDurationMs(Number(event.currentTarget.value))}
              >
                <For each={SUPPRESSION_DURATIONS}>
                  {(duration) => <option value={duration.value}>{duration.label}</option>}
                </For>
              </select>
            </div>
            <div class="flex flex-wrap gap-2">
              <Button
                variant="warning"
                size="sm"
                class="min-h-11 sm:min-h-0"
                isLoading={props.busy}
                disabled={!reason().trim()}
                type="submit"
              >
                Suppress temporarily
              </Button>
              <Button
                variant="ghost"
                size="sm"
                class="min-h-11 sm:min-h-0"
                disabled={props.busy}
                onClick={() => setShowSuppression(false)}
              >
                Cancel
              </Button>
            </div>
          </form>
        </Show>
        <Show when={props.error}>
          <p role="alert" class="mt-3 text-xs leading-5 text-red-700 dark:text-red-300">
            {props.error}
          </p>
        </Show>
        <div class="mt-3 border-t border-border-subtle pt-3 text-xs leading-5 text-muted">
          <p>
            Mark reviewed removes this occurrence from today's decision inbox while keeping its
            record. Suppression hides it only until the selected return time.
          </p>
          <p class="mt-1">
            For a permanent change,{' '}
            <A
              href={ALERT_THRESHOLDS_PATH}
              class="font-medium text-blue-700 hover:underline dark:text-blue-300"
            >
              adjust alert thresholds
            </A>{' '}
            to change or turn off this alert for the affected resource
            <Show when={props.onOpenFindings} fallback=".">
              , or{' '}
              <button
                type="button"
                class="font-medium text-blue-700 hover:underline dark:text-blue-300"
                onClick={() => props.onOpenFindings?.()}
              >
                review Patrol findings
              </button>{' '}
              to mark a finding as expected so Patrol stops raising it.
            </Show>
          </p>
        </div>
      </div>
    </DetailSection>
  );
}

function ActionVerificationMessage(props: { state: AttentionItem['verificationState'] }) {
  const message = () => {
    switch (props.state) {
      case 'pending':
        return 'The action is awaiting a decision, execution, or verification.';
      case 'succeeded':
        return 'The restart postcondition was confirmed. This issue stays open until fresh health evidence shows the container is healthy.';
      case 'failed':
        return 'The restart did not satisfy its postcondition. The issue remains open.';
      case 'unknown':
        return 'Pulse could not conclusively verify the restart. The issue remains open.';
      default:
        return '';
    }
  };
  return (
    <Show when={message()}>
      {(value) => <p class="mt-2 text-xs font-medium leading-5 text-base-content">{value()}</p>}
    </Show>
  );
}

function attentionActionGuidance(item: AttentionItem, offer: AttentionActionOffer): string {
  if (!offer.actionId) {
    return 'This requires an explicit review and approval before Pulse sends anything.';
  }
  switch (item.verificationState) {
    case 'pending':
      return 'Open the existing review to continue the governed action.';
    case 'succeeded':
    case 'failed':
    case 'unknown':
      return 'Pulse recorded the action result below. Open the review for the full audit.';
    default:
      return 'Open the existing review to inspect the recorded decision.';
  }
}

function DetailSection(props: { title: string; children: import('solid-js').JSX.Element }) {
  return (
    <section>
      <h4 class="text-[11px] font-semibold uppercase tracking-wider text-muted">{props.title}</h4>
      <div class="mt-2">{props.children}</div>
    </section>
  );
}

function EvidenceObservation(props: { evidence: EvidenceEnvelope }) {
  return (
    <li class="rounded-md border border-border-subtle bg-surface-alt/60 p-3">
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-xs font-semibold text-base-content">
          {formatProvider(props.evidence.source.provider)}
        </span>
        <MetadataBadge
          tone={props.evidence.completeness === 'complete' ? 'success' : 'warning'}
          size="xs"
          shape="rounded"
        >
          {formatLabel(props.evidence.completeness)}
        </MetadataBadge>
        <MetadataBadge
          tone={props.evidence.confidence === 'confirmed' ? 'info' : 'muted'}
          size="xs"
          shape="rounded"
        >
          {formatLabel(props.evidence.confidence)}
        </MetadataBadge>
      </div>
      <p class="mt-1 text-xs text-muted">
        {props.evidence.source.collector} · observed{' '}
        {formatRelativeTime(props.evidence.observedAt, { compact: true })}
      </p>
      <Show when={props.evidence.reason?.message}>
        {(reason) => <p class="mt-1 text-xs leading-5 text-muted">{reason()}</p>}
      </Show>
    </li>
  );
}

function severityTone(item: AttentionItem): MetadataBadgeTone {
  switch (item.severity) {
    case 'critical':
      return 'danger';
    case 'warning':
      return 'warning';
    case 'info':
      return 'info';
    default:
      return 'muted';
  }
}

function StateBadge(props: { item: AttentionItem }) {
  return (
    <MetadataBadge tone={stateTone(props.item)} size="xs" shape="rounded" appearance="outline">
      {formatLabel(props.item.state)}
    </MetadataBadge>
  );
}

function EvidenceLabel(props: { item: AttentionItem; badge?: boolean }) {
  const presentation = () =>
    getPatrolAttentionEvidencePresentation(
      props.item.evidenceFreshness,
      props.item.evidenceCompleteness,
    );
  if (props.badge) {
    return (
      <MetadataBadge tone={presentation().tone} size="xs" shape="rounded">
        {presentation().detailLabel}
      </MetadataBadge>
    );
  }
  return <Show when={presentation().rowLabel}>{(label) => <span>{label()}</span>}</Show>;
}

function ProtectionLabel(props: { item: AttentionItem; badge?: boolean }) {
  const presentation = () =>
    getPatrolAttentionProtectionPresentation(props.item.protectionPosture?.state);
  if (props.badge) {
    return (
      <MetadataBadge tone={presentation().tone} size="xs" shape="rounded">
        {presentation().detailLabel}
      </MetadataBadge>
    );
  }
  return <Show when={presentation().rowLabel}>{(label) => <span>{label()}</span>}</Show>;
}

function ProtectionProviderMetadata(props: {
  provider: NonNullable<AttentionItem['protectionPosture']>['providerStates'][number];
}) {
  const labels = () => getPatrolProtectionProviderLabels(props.provider);
  return (
    <span class="text-muted">
      {' · '}
      <span>{labels().job}</span>
      {' · '}
      <span>{labels().history}</span>
    </span>
  );
}

function stateTone(item: AttentionItem): MetadataBadgeTone {
  switch (item.state) {
    case 'resolved':
      return 'success';
    case 'acknowledged':
      return 'info';
    case 'suppressed':
      return 'muted';
    case 'stale':
    case 'unknown':
      return 'warning';
    default:
      return item.severity === 'critical' ? 'danger' : 'warning';
  }
}

function formatLabel(value: string): string {
  const normalized = value.trim().replace(/[_-]+/g, ' ');
  return normalized ? normalized.charAt(0).toUpperCase() + normalized.slice(1) : 'Unknown';
}

function formatProvider(value: string): string {
  switch (value.trim().toLowerCase()) {
    case 'pbs':
      return 'Proxmox Backup Server';
    case 'pve':
      return 'Proxmox VE';
    default:
      return formatLabel(value);
  }
}
