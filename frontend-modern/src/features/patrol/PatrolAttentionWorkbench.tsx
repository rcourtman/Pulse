import { useLocation } from '@solidjs/router';
import {
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
import CheckCircleIcon from 'lucide-solid/icons/circle-check';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import ClockIcon from 'lucide-solid/icons/clock';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import RefreshIcon from 'lucide-solid/icons/refresh-cw';
import RotateCwIcon from 'lucide-solid/icons/rotate-cw';
import ShieldOffIcon from 'lucide-solid/icons/shield-off';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import XIcon from 'lucide-solid/icons/x';
import type {
  AttentionFilter,
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
import { aiChatStore } from '@/stores/aiChat';
import { patrolAttentionStore } from '@/stores/patrolAttention';
import {
  buildPatrolAttentionPath,
  buildStandalonePath,
  buildWorkloadsRouteSearch,
  parsePatrolAttentionItemId,
} from '@/routing/resourceLinks';
import type { EvidenceEnvelope } from '@/types/operationalTrust';
import type { ActionDetailResponse } from '@/types/actionAudit';
import { getAlertResourceIncidentAcknowledgedByLabel } from '@/utils/alertIncidentPresentation';
import { formatRelativeTime } from '@/utils/format';

const PRIMARY_EVIDENCE_LIMIT = 3;

const FILTERS: Array<{ id: AttentionFilter; label: string }> = [
  { id: 'active', label: 'Active' },
  { id: 'open', label: 'Open' },
  { id: 'acknowledged', label: 'Acknowledged' },
  { id: 'suppressed', label: 'Suppressed' },
  { id: 'stale_unknown', label: 'Stale or unknown' },
  { id: 'resolved', label: 'Recent resolved' },
];

export function PatrolAttentionWorkbench() {
  const location = useLocation();
  const [selectedItemId, setSelectedItemId] = createSignal('');
  const [actionDetail, setActionDetail] = createSignal<ActionDetailResponse | null>(null);
  const [actionBusy, setActionBusy] = createSignal(false);
  const [actionError, setActionError] = createSignal('');
  const [lifecycleBusy, setLifecycleBusy] = createSignal(false);
  const [lifecycleError, setLifecycleError] = createSignal('');
  const itemButtons = new Map<string, HTMLButtonElement>();
  let detailPanel: HTMLDivElement | undefined;
  let actionTrigger: HTMLButtonElement | undefined;

  const selectedDetail = () => patrolAttentionStore.selectedDetail();
  const summary = () => patrolAttentionStore.summary();
  const filterCount = (filter: AttentionFilter): number | undefined => {
    const value = summary();
    if (!value) return undefined;
    switch (filter) {
      case 'active':
        return value.activeCount;
      case 'open':
        return value.openCount;
      case 'acknowledged':
        return value.acknowledgedCount;
      case 'suppressed':
        return value.suppressedCount;
      case 'stale_unknown':
        return value.uncertainCount;
      case 'resolved':
        return value.resolvedCount;
      default:
        return undefined;
    }
  };

  const loadCurrentFilter = () => patrolAttentionStore.load(patrolAttentionStore.filter());
  const scrollDetailIntoView = () => {
    queueMicrotask(() => {
      if (window.matchMedia?.('(max-width: 1023px)').matches) {
        detailPanel?.scrollIntoView?.({ block: 'start' });
      }
    });
  };
  const selectItem = (itemId: string) => {
    setSelectedItemId(itemId);
    replaceAttentionLocation(itemId);
    void patrolAttentionStore.select(itemId);
    scrollDetailIntoView();
  };
  const closeDetail = () => {
    const previous = selectedItemId();
    setSelectedItemId('');
    replaceAttentionLocation('');
    void patrolAttentionStore.select(null);
    queueMicrotask(() => itemButtons.get(previous)?.focus());
  };
  const changeFilter = (filter: AttentionFilter) => {
    closeDetail();
    void patrolAttentionStore.load(filter);
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
  const changeLifecycle = async (operation: () => Promise<unknown>) => {
    if (lifecycleBusy()) return;
    setLifecycleBusy(true);
    setLifecycleError('');
    try {
      await operation();
      const selected = selectedItemId();
      await Promise.all([
        selected ? patrolAttentionStore.select(selected) : Promise.resolve(),
        patrolAttentionStore.load(patrolAttentionStore.filter()),
      ]);
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
      setSelectedItemId(deepLinkedItem);
      void patrolAttentionStore.select(deepLinkedItem);
      scrollDetailIntoView();
    } else if (!deepLinkedItem && currentItem) {
      setSelectedItemId('');
      void patrolAttentionStore.select(null);
    }
  });

  const activeCountLabel = createMemo(() => {
    const count = summary()?.activeCount;
    if (count === undefined) return 'Attention count unavailable';
    return `${count} active attention ${count === 1 ? 'item' : 'items'}`;
  });

  return (
    <section
      aria-labelledby="patrol-attention-heading"
      class="overflow-hidden rounded-lg border border-border bg-surface shadow-sm"
    >
      <div class="border-b border-border px-4 py-4 sm:px-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2 id="patrol-attention-heading" class="text-base font-semibold text-base-content">
                Needs attention
              </h2>
              <Show when={summary()}>
                <MetadataBadge
                  tone={(summary()?.activeCount ?? 0) > 0 ? 'warning' : 'success'}
                  size="sm"
                  shape="rounded"
                  aria-label={activeCountLabel()}
                >
                  {summary()?.activeCount ?? 0}
                </MetadataBadge>
              </Show>
            </div>
            <p class="mt-1 max-w-3xl text-sm leading-5 text-muted">
              Current operational issues, ordered by urgency, affected resources, protection
              concern, evidence quality, and age.
            </p>
          </div>
          <Button
            variant="secondary"
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

        <div
          class="mt-4 flex gap-2 overflow-x-auto pb-1"
          role="group"
          aria-label="Attention filter"
        >
          <For each={FILTERS}>
            {(option) => {
              const selected = () => patrolAttentionStore.filter() === option.id;
              const count = () => filterCount(option.id);
              return (
                <button
                  type="button"
                  class={`inline-flex shrink-0 items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-xs font-medium focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 ${
                    selected()
                      ? 'border-blue-500 bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-200'
                      : 'border-border bg-surface text-muted hover:bg-surface-hover hover:text-base-content'
                  }`}
                  aria-pressed={selected()}
                  onClick={() => changeFilter(option.id)}
                >
                  {option.label}
                  <Show when={count() !== undefined}>
                    <span class="tabular-nums text-[10px]">{count()}</span>
                  </Show>
                </button>
              );
            }}
          </For>
        </div>
      </div>

      <div
        class={`grid min-w-0 ${selectedItemId() ? 'lg:grid-cols-[minmax(18rem,0.8fr)_minmax(0,1.2fr)]' : ''}`}
      >
        <div
          class={`min-w-0 ${selectedItemId() ? 'border-b border-border lg:border-b-0 lg:border-r' : ''}`}
        >
          <AttentionList
            selectedItemId={selectedItemId()}
            itemButtons={itemButtons}
            onSelect={selectItem}
          />
        </div>
        <Show when={selectedItemId()}>
          <div ref={detailPanel} class="order-first min-w-0 scroll-mt-4 lg:order-none">
            <AttentionDetail
              detail={selectedDetail()}
              loading={patrolAttentionStore.detailLoading()}
              onClose={closeDetail}
              actionBusy={actionBusy()}
              actionError={actionError()}
              onReviewAction={reviewAction}
              lifecycleBusy={lifecycleBusy()}
              lifecycleError={lifecycleError()}
              onAcknowledge={(itemId) => changeLifecycle(() => acknowledgePatrolAttention(itemId))}
              onUnacknowledge={(itemId) =>
                changeLifecycle(() => unacknowledgePatrolAttention(itemId))
              }
              onSuppress={(itemId, reason, expiresAt) =>
                changeLifecycle(() => suppressPatrolAttention(itemId, reason, expiresAt))
              }
              onUnsuppress={(itemId) => changeLifecycle(() => unsuppressPatrolAttention(itemId))}
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
        when={!patrolAttentionStore.loading() || patrolAttentionStore.items().length > 0}
        fallback={
          <div class="flex min-h-40 items-center justify-center gap-2 text-sm text-muted">
            <LoadingSpinner size="sm" />
            Loading current attention
          </div>
        }
      >
        <Show when={patrolAttentionStore.items().length > 0} fallback={<AttentionEmptyState />}>
          <ul class="divide-y divide-border" aria-label="Patrol attention items">
            <For each={patrolAttentionStore.items()}>
              {(item) => (
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
                      <SeverityMarker item={item} />
                      <div class="min-w-0 flex-1">
                        <div class="flex min-w-0 flex-wrap items-center gap-2">
                          <span class="truncate text-sm font-semibold text-base-content">
                            {item.title}
                          </span>
                          <StateBadge item={item} />
                        </div>
                        <p class="mt-1 line-clamp-2 text-xs leading-5 text-muted">
                          {item.plainLanguageSummary}
                        </p>
                        <div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted">
                          <span class="truncate font-medium text-base-content">
                            {item.subjectResourceName}
                          </span>
                          <EvidenceLabel item={item} />
                          <ProtectionLabel item={item} />
                          <span>{formatRelativeTime(item.firstObservedAt, { compact: true })}</span>
                        </div>
                      </div>
                      <ChevronRightIcon
                        class="mt-1 h-4 w-4 shrink-0 text-muted"
                        aria-hidden="true"
                      />
                    </div>
                  </button>
                </li>
              )}
            </For>
          </ul>
        </Show>
      </Show>
    </div>
  );
}

function AttentionEmptyState() {
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
        when={trustworthyCalm()}
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
        <h3 class="mt-3 text-sm font-semibold text-base-content">Nothing needs your attention</h3>
        <p class="mt-1 max-w-md text-xs leading-5 text-muted">
          The current operational lifecycle evaluation has no active items.
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
  onAcknowledge: (itemId: string) => Promise<void>;
  onUnacknowledge: (itemId: string) => Promise<void>;
  onSuppress: (itemId: string, reason: string, expiresAt: string) => Promise<void>;
  onUnsuppress: (itemId: string) => Promise<void>;
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
      <div class="flex items-start justify-between gap-3 border-b border-border px-4 py-3 sm:px-5">
        <div class="min-w-0">
          <p class="text-[11px] font-semibold uppercase tracking-wider text-muted">
            Attention detail
          </p>
          <h3 id="attention-detail-title" class="mt-1 text-sm font-semibold text-base-content">
            {item()?.title ?? 'Loading attention item'}
          </h3>
        </div>
        <button
          type="button"
          class="rounded p-1 text-muted hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          aria-label="Close attention detail"
          onClick={props.onClose}
        >
          <XIcon class="h-4 w-4" aria-hidden="true" />
        </button>
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

            <AttentionLifecycleControls
              detail={loaded()}
              busy={props.lifecycleBusy}
              error={props.lifecycleError}
              onAcknowledge={props.onAcknowledge}
              onUnacknowledge={props.onUnacknowledge}
              onSuppress={props.onSuppress}
              onUnsuppress={props.onUnsuppress}
            />

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
                          class="mt-3 gap-1.5"
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
                    <summary class="cursor-pointer px-3 py-2 text-xs font-medium text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500">
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
                              <span class="text-muted">
                                {' '}
                                · {formatLabel(provider.jobState)} ·{' '}
                                {formatLabel(provider.historyCompleteness)}
                              </span>
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
              <ButtonLink href={resourceHref()} variant="secondary" size="sm" class="gap-1.5">
                <ExternalLinkIcon class="h-4 w-4" aria-hidden="true" />
                Open resource
              </ButtonLink>
              <Show when={aiChatStore.enabled === true}>
                <Button variant="secondary" size="sm" class="gap-1.5" onClick={openAssistant}>
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
              isLoading={props.busy}
              onClick={() => void props.onAcknowledge(props.detail.item.id)}
            >
              Acknowledge
            </Button>
          </Show>
          <Show when={state() === 'acknowledged'}>
            <Button
              variant="secondary"
              size="sm"
              isLoading={props.busy}
              onClick={() => void props.onUnacknowledge(props.detail.item.id)}
            >
              Return to open
            </Button>
          </Show>
          <Show when={state() === 'suppressed'}>
            <Button
              variant="secondary"
              size="sm"
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
              class="gap-1.5"
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
                class="mt-1 block min-h-9 rounded-md border border-border bg-surface px-3 py-1.5 text-sm text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500"
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
                isLoading={props.busy}
                disabled={!reason().trim()}
                type="submit"
              >
                Suppress temporarily
              </Button>
              <Button
                variant="ghost"
                size="sm"
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

function SeverityMarker(props: { item: AttentionItem }) {
  const classes = () => {
    switch (props.item.severity) {
      case 'critical':
        return 'bg-red-500';
      case 'warning':
        return 'bg-amber-500';
      case 'info':
        return 'bg-blue-500';
      default:
        return 'bg-slate-400';
    }
  };
  return (
    <span
      class={`mt-1 h-2.5 w-2.5 shrink-0 rounded-full ${classes()}`}
      title={`${formatLabel(props.item.severity)} severity`}
      aria-hidden="true"
    />
  );
}

function StateBadge(props: { item: AttentionItem }) {
  return (
    <MetadataBadge tone={stateTone(props.item)} size="xs" shape="rounded" appearance="outline">
      {formatLabel(props.item.state)}
    </MetadataBadge>
  );
}

function EvidenceLabel(props: { item: AttentionItem; badge?: boolean }) {
  const label = () =>
    props.item.evidenceFreshness === 'fresh' && props.item.evidenceCompleteness === 'complete'
      ? 'Evidence current'
      : `${formatLabel(props.item.evidenceFreshness)} / ${formatLabel(props.item.evidenceCompleteness)}`;
  if (props.badge) {
    return (
      <MetadataBadge
        tone={
          props.item.evidenceFreshness === 'fresh' && props.item.evidenceCompleteness === 'complete'
            ? 'success'
            : 'warning'
        }
        size="xs"
        shape="rounded"
      >
        {label()}
      </MetadataBadge>
    );
  }
  return <span>{label()}</span>;
}

function ProtectionLabel(props: { item: AttentionItem; badge?: boolean }) {
  const state = () => props.item.protectionPosture?.state ?? 'unknown';
  const label = () => `Protection ${formatLabel(state())}`;
  if (props.badge) {
    return (
      <MetadataBadge
        tone={state() === 'protected' ? 'success' : state() === 'unknown' ? 'muted' : 'warning'}
        size="xs"
        shape="rounded"
      >
        {label()}
      </MetadataBadge>
    );
  }
  return <span>{label()}</span>;
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
