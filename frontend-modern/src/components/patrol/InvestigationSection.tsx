/**
 * InvestigationSection
 *
 * Inline investigation details within expanded finding.
 * Replaces the drawer. Shows status, summary, tools used, and
 * a collapsible investigation thread.
 */

import { Component, createSignal, createMemo, Show, For } from 'solid-js';
import {
  getInvestigation,
  reinvestigateFinding,
  formatTimestamp,
  type Investigation,
} from '@/api/patrol';
import { withAPIErrorStatusNull } from '@/api/responseUtils';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import {
  getInvestigationConfidenceBadgeTone,
  getInvestigationOutcomeBadgeTone,
  getInvestigationOutcomeLabel,
  getInvestigationStatusLabel,
  getInvestigationStatusBadgeTone,
} from '@/utils/aiFindingPresentation';
import { getInvestigationSectionState } from '@/utils/patrolEmptyStatePresentation';
import { buildPatrolInvestigationRecordPresentation } from '@/features/patrol/patrolInvestigationContextModel';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { Button } from '@/components/shared/Button';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { getAPIReadAccessErrorMessage } from '@/utils/apiAccessError';
import { InvestigationMessages } from './InvestigationMessages';
import { renderMarkdown } from '@/components/AI/aiChatUtils';
import { notificationStore } from '@/stores/notifications';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import type { InvestigationRecord } from '@/api/ai';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

const INVESTIGATION_BADGE_PROPS = {
  appearance: 'outline',
  size: 'xs',
  shape: 'rounded',
} as const;

const ACTIVE_INVESTIGATION_POLL_MS = 5_000;
const INVESTIGATION_REVALIDATE_MS = 30_000;

const summaryClass =
  'text-sm prose prose-slate prose-sm dark:prose-invert max-w-none wrap-break-word prose-headings:my-2 prose-p:my-2 prose-pre:overflow-x-auto prose-code:break-all prose-code:before:content-none prose-code:after:content-none';

// A run cut short by a restart did not fail: Patrol runs it again. A run that
// stopped after planning a fix left that fix waiting for approval. Either way,
// its note reads as information rather than as an error.
const runNoteClass = (outcome: string | undefined): string =>
  outcome === 'interrupted' || outcome === 'fix_queued'
    ? 'border-border-subtle bg-surface text-muted'
    : 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/25 dark:text-red-300';

interface InvestigationSectionProps {
  findingId: string;
  investigationStatus?: string;
  investigationOutcome?: string;
  investigationAttempts?: number;
  investigationRecord?: InvestigationRecord;
}

export const InvestigationSection: Component<InvestigationSectionProps> = (props) => {
  const [showThread, setShowThread] = createSignal(false);
  const [reinvestigating, setReinvestigating] = createSignal(false);
  const [cooldownUntil, setCooldownUntil] = createSignal(0);
  const investigationRecord = createMemo(() =>
    buildPatrolInvestigationRecordPresentation(props.investigationRecord),
  );

  // Reads stay out of the app's route Suspense boundary, whose fallback
  // replaces the whole page: a read while someone reads this review (opening
  // it, polling, Re-investigate) dropped them back to the top. The last answer
  // stays on screen while the next loads, and a remounted review shows the
  // retained answer at once. The review also stays mounted across findings
  // refreshes, which used to remount it and so re-read it every 30s, so it
  // re-reads on that cadence itself, and every 5s while the run is active.
  const investigationQuery = createNonSuspendingQuery<Investigation | null, string>({
    source: () => props.findingId,
    cacheKey: (findingId) => `patrol-investigation:${findingId}`,
    retainPreviousValueOnSourceChange: false,
    // A finding Patrol has not investigated answers 404. Any other failure
    // keeps the last answer on screen for the next read to replace.
    fetcher: (findingId) => withAPIErrorStatusNull(getInvestigation(findingId), 404),
    initialValue: null,
    pollMs: (): number =>
      investigationActive() ? ACTIVE_INVESTIGATION_POLL_MS : INVESTIGATION_REVALIDATE_MS,
  });
  const investigation = investigationQuery.value;
  const refreshInvestigation = () => investigationQuery.refetch({ background: true });
  // The review stays mounted across refreshes, so its ages re-read on each
  // tick of the shared clock to keep moving.
  const now = useRelativeTimeNow();
  const formatAge = (timestamp: string) => {
    now();
    return formatTimestamp(timestamp);
  };
  const hasInvestigationContext = createMemo(
    () => Boolean(investigation()) || investigationRecord().hasRecord,
  );
  const investigationSectionState = createMemo(() =>
    getInvestigationSectionState(!investigationQuery.resolvedOnce(), hasInvestigationContext()),
  );
  const investigationActive = createMemo<boolean>(() => {
    const inv = investigation();
    return (
      inv?.status === 'running' ||
      inv?.status === 'pending' ||
      props.investigationStatus === 'running'
    );
  });
  // The thread follows the run's session id, not each re-read of the run.
  const sessionId = createMemo(() => investigation()?.session_id ?? '');

  const canReinvestigate = () => {
    // Until the first read answers, a running investigation or a queued fix
    // looks like no investigation at all, which would offer another run.
    // A failed read says nothing about whether a run already exists. Even a
    // retained answer may miss a newer run or queued fix. Retry the read, not
    // the investigation, until its current state is known.
    if (!investigationQuery.resolvedOnce() || investigationQuery.error()) return false;
    if (Date.now() < cooldownUntil()) return false;
    const inv = investigation();
    if (!inv) return true; // No investigation yet
    if (inv.status === 'running') return false;
    // A queued fix is still open, even after a run that failed later.
    // Investigating again would plan a second action for the same problem.
    if (inv.outcome === 'fix_queued') return false;
    return (
      inv.status === 'failed' ||
      inv.status === 'needs_attention' ||
      inv.outcome === 'cannot_fix' ||
      inv.outcome === 'timed_out' ||
      inv.outcome === 'unclassified' ||
      inv.outcome === 'interrupted' ||
      inv.outcome === 'fix_rejected' ||
      inv.outcome === 'fix_verification_failed' ||
      inv.outcome === 'fix_verification_unknown' ||
      inv.outcome === 'fix_failed'
    );
  };

  const handleReinvestigate = async (e: Event) => {
    e.stopPropagation();
    setReinvestigating(true);
    try {
      await reinvestigateFinding(props.findingId);
      notificationStore.success('Re-investigation started');
      setCooldownUntil(Date.now() + 60000);
      void refreshInvestigation();
      aiIntelligenceStore.loadFindings();
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to start re-investigation');
    } finally {
      setReinvestigating(false);
    }
  };

  return (
    <div class="mt-3 pt-3 border-t border-border-subtle">
      {/* Header; wraps so a long outcome label never squeezes the action */}
      <div class="flex flex-wrap items-center justify-between gap-x-2 gap-y-1 mb-2">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium text-base-content">Investigation</span>
          {/* Show outcome badge when available, otherwise show status badge */}
          <Show
            when={investigation()?.outcome}
            fallback={
              <Show when={investigation()?.status}>
                <MetadataBadge
                  {...INVESTIGATION_BADGE_PROPS}
                  tone={getInvestigationStatusBadgeTone(investigation()!.status)}
                >
                  {getInvestigationStatusLabel(investigation()!.status)}
                </MetadataBadge>
              </Show>
            }
          >
            <MetadataBadge
              {...INVESTIGATION_BADGE_PROPS}
              tone={getInvestigationOutcomeBadgeTone(investigation()!.outcome!)}
            >
              {getInvestigationOutcomeLabel(investigation()!.outcome!)}
            </MetadataBadge>
          </Show>
          <Show when={props.investigationAttempts && props.investigationAttempts > 1}>
            <span class="text-[10px] text-muted">attempt {props.investigationAttempts}</span>
          </Show>
        </div>
        <Show when={canReinvestigate()}>
          <button
            type="button"
            onClick={handleReinvestigate}
            disabled={reinvestigating()}
            class="flex items-center gap-1 whitespace-nowrap px-2 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/25 rounded-sm transition-colors disabled:opacity-50"
          >
            <RefreshCwIcon class={`w-3 h-3 ${reinvestigating() ? 'animate-spin' : ''}`} />
            Re-investigate
          </button>
        </Show>
        <Show when={!canReinvestigate() && Date.now() < cooldownUntil()}>
          <span class="text-xs text-muted">Re-investigation started</span>
        </Show>
      </div>

      <Show when={investigationQuery.error()}>
        <InlineNotice
          role="alert"
          tone="warning"
          aria-label="Investigation read failed"
          class="mb-2"
        >
          <p class="font-medium">Could not load investigation history.</p>
          <p>
            {getAPIReadAccessErrorMessage(investigationQuery.error()) ??
              (hasInvestigationContext()
                ? 'Showing previously loaded investigation evidence. It may be out of date.'
                : 'The saved investigation could not be read. This does not mean no investigation exists.')}
          </p>
          <Button
            size="sm"
            disabled={investigationQuery.loading()}
            onClick={(e) => {
              e.stopPropagation();
              void investigationQuery.refetch();
            }}
          >
            {investigationQuery.loading() ? 'Retrying history...' : 'Retry history'}
          </Button>
        </InlineNotice>
      </Show>

      {/* A failed read is neither loading nor an empty investigation. */}
      <Show
        when={
          !investigationQuery.error() &&
          !investigationSectionState().empty &&
          investigationSectionState().text
        }
      >
        <div class="flex items-center gap-2 text-xs text-muted py-2">
          <LoadingSpinner size="sm" />
          {investigationSectionState().text}
        </div>
      </Show>

      {/* No investigation data */}
      <Show when={!investigationQuery.error() && investigationSectionState().empty}>
        <p class="text-xs text-muted py-1">{investigationSectionState().text}</p>
      </Show>

      <Show when={investigationRecord().hasRecord}>
        <div class="mb-2 rounded-sm border border-border bg-surface-alt p-2">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-xs font-medium text-base-content">Patrol record</span>
            {/* As in the header, the outcome replaces the run status once there is
                one: an interrupted run's status is failed, which it is not. */}
            <Show
              when={investigationRecord().outcomeLabel}
              fallback={
                <MetadataBadge
                  {...INVESTIGATION_BADGE_PROPS}
                  tone={getInvestigationStatusBadgeTone(props.investigationRecord!.status)}
                >
                  {investigationRecord().statusLabel}
                </MetadataBadge>
              }
            >
              <MetadataBadge
                {...INVESTIGATION_BADGE_PROPS}
                tone={getInvestigationOutcomeBadgeTone(props.investigationRecord!.outcome ?? '')}
              >
                {investigationRecord().outcomeLabel}
              </MetadataBadge>
            </Show>
            <Show when={investigationRecord().confidenceLabel}>
              <MetadataBadge
                {...INVESTIGATION_BADGE_PROPS}
                tone={getInvestigationConfidenceBadgeTone(
                  props.investigationRecord!.confidence ?? '',
                )}
              >
                {investigationRecord().confidenceLabel}
              </MetadataBadge>
            </Show>
          </div>

          <Show when={investigationRecord().conclusion}>
            <div
              class={`mt-2 ${summaryClass}`}
              // eslint-disable-next-line solid/no-innerhtml
              innerHTML={renderMarkdown(investigationRecord().conclusion!)}
            />
          </Show>
          <Show when={investigationRecord().recommendedAction}>
            <p class="mt-1 text-xs text-muted">
              <span class="font-medium text-base-content">Recommended action:</span>{' '}
              {investigationRecord().recommendedAction}
            </p>
          </Show>

          <Show when={investigationRecord().proposedFix}>
            {(fix) => (
              <div class="mt-2 rounded-sm border border-border-subtle bg-surface p-2 text-xs">
                <div class="font-medium text-base-content">{fix().description}</div>
                <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-muted">
                  <Show when={fix().riskLabel}>
                    <span>{fix().riskLabel} risk</span>
                  </Show>
                  <Show when={fix().targetHost}>
                    <span>target {fix().targetHost}</span>
                  </Show>
                  <Show when={fix().commandSummary}>
                    <span>{fix().commandSummary}</span>
                  </Show>
                  <Show when={fix().destructive}>
                    <span class="text-amber-700 dark:text-amber-300">destructive</span>
                  </Show>
                </div>
                <Show when={fix().rationale}>
                  <p class="mt-1 text-muted">{fix().rationale}</p>
                </Show>
              </div>
            )}
          </Show>

          <Show when={investigationRecord().hasRecord}>
            <div class="mt-2">
              <div class="text-[10px] font-medium uppercase text-muted">Impact</div>
              <p
                class="mt-1 text-xs"
                classList={{
                  'text-muted': Boolean(investigationRecord().impact),
                  'italic ': !investigationRecord().impact,
                }}
              >
                {investigationRecord().impact || 'Impact not assessed'}
              </p>
            </div>
          </Show>

          <Show when={investigationRecord().evidenceSummaries.length > 0}>
            <div class="mt-2">
              <div class="text-[10px] font-medium uppercase text-muted">Evidence</div>
              <ul class="mt-1 space-y-1 text-xs text-muted">
                <For each={investigationRecord().evidenceSummaries}>
                  {(item) => <li>{item}</li>}
                </For>
              </ul>
            </div>
          </Show>

          <Show when={investigationRecord().verificationSummaries.length > 0}>
            <div class="mt-2">
              <div class="text-[10px] font-medium uppercase text-muted">Verification</div>
              <ul class="mt-1 space-y-1 text-xs text-muted">
                <For each={investigationRecord().verificationSummaries}>
                  {(item) => <li>{item}</li>}
                </For>
              </ul>
            </div>
          </Show>

          <Show when={investigationRecord().hasRecord}>
            <div class="mt-2">
              <div class="text-[10px] font-medium uppercase text-muted">Rollback</div>
              <Show
                when={investigationRecord().rollbackSummaries.length > 0}
                fallback={<p class="mt-1 text-xs italic">Rollback not specified</p>}
              >
                <ul class="mt-1 space-y-1 text-xs text-muted">
                  <For each={investigationRecord().rollbackSummaries}>
                    {(item) => <li>{item}</li>}
                  </For>
                </ul>
              </Show>
            </div>
          </Show>

          <Show when={investigationRecord().toolsUsed.length > 0}>
            <div class="mt-2 flex flex-wrap gap-1">
              <For each={investigationRecord().toolsUsed}>
                {(tool) => (
                  <MetadataBadge tone="muted" size="xs" shape="rounded">
                    {tool}
                  </MetadataBadge>
                )}
              </For>
            </div>
          </Show>

          <Show when={investigationRecord().error}>
            <div
              class={`mt-2 rounded-sm border p-2 text-xs ${runNoteClass(props.investigationRecord?.outcome)}`}
            >
              {investigationRecord().error}
            </div>
          </Show>
        </div>
      </Show>

      {/* Investigation details */}
      <Show when={investigation()}>
        {(inv) => (
          <div class="space-y-2">
            {/* Error message for failed investigations */}
            <Show
              when={
                inv().error &&
                inv().error?.trim() !== investigationRecord().error &&
                (inv().status === 'failed' ||
                  inv().outcome === 'timed_out' ||
                  inv().outcome === 'interrupted' ||
                  inv().outcome === 'unclassified' ||
                  inv().outcome === 'fix_failed' ||
                  inv().outcome === 'fix_rejected' ||
                  inv().outcome === 'fix_verification_failed' ||
                  inv().outcome === 'fix_verification_unknown' ||
                  inv().outcome === 'needs_attention' ||
                  inv().outcome === 'cannot_fix')
              }
            >
              <div class={`rounded-sm border p-2 text-xs ${runNoteClass(inv().outcome)}`}>
                {inv().error}
              </div>
            </Show>

            {/* Summary */}
            <Show
              when={
                inv().summary?.trim() && inv().summary?.trim() !== investigationRecord().conclusion
              }
            >
              <div
                class={`bg-surface-alt rounded-sm p-2 ${summaryClass}`}
                // eslint-disable-next-line solid/no-innerhtml
                innerHTML={renderMarkdown(inv().summary!)}
              />
            </Show>

            {/* Tools used + turn count */}
            <div class="flex items-center gap-2 flex-wrap">
              <Show when={inv().tools_used && inv().tools_used!.length > 0}>
                <div class="flex items-center gap-1 flex-wrap">
                  <For each={inv().tools_used}>
                    {(tool) => (
                      <MetadataBadge tone="neutral" size="xs" shape="rounded">
                        {tool}
                      </MetadataBadge>
                    )}
                  </For>
                </div>
              </Show>
              <span class="text-[10px] text-muted">
                {inv().turn_count} turn{inv().turn_count === 1 ? '' : 's'}
              </span>
              <Show when={inv().started_at}>
                <span class="text-[10px] text-muted">started {formatAge(inv().started_at)}</span>
              </Show>
            </div>

            {/* Show thread toggle */}
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                setShowThread(!showThread());
              }}
              class="text-xs text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1"
            >
              {showThread() ? 'Hide investigation thread' : 'Show investigation thread'}
              <svg
                class={`w-3 h-3 transition-transform ${showThread() ? 'rotate-90' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </button>

            {/* Investigation messages (lazy loaded) */}
            <Show when={showThread()}>
              <InvestigationMessages
                findingId={props.findingId}
                sessionId={sessionId()}
                live={investigationActive()}
              />
            </Show>
          </div>
        )}
      </Show>
    </div>
  );
};

export default InvestigationSection;
