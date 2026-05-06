/**
 * InvestigationSection
 *
 * Inline investigation details within expanded finding.
 * Replaces the drawer. Shows status, summary, tools used, and
 * a collapsible investigation thread.
 */

import {
  Component,
  createSignal,
  createResource,
  createEffect,
  createMemo,
  onCleanup,
  Show,
  For,
} from 'solid-js';
import { getInvestigation, reinvestigateFinding, formatTimestamp } from '@/api/patrol';
import {
  getInvestigationOutcomeBadgeClasses,
  getInvestigationOutcomeLabel,
  getInvestigationStatusLabel,
  getInvestigationStatusBadgeClasses,
} from '@/utils/aiFindingPresentation';
import { getInvestigationSectionState } from '@/utils/patrolEmptyStatePresentation';
import { buildPatrolInvestigationRecordPresentation } from '@/features/patrol/patrolInvestigationContextModel';
import { InvestigationMessages } from './InvestigationMessages';
import { notificationStore } from '@/stores/notifications';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import type { InvestigationRecord } from '@/api/ai';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

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

  const [investigation, { refetch }] = createResource(
    () => props.findingId,
    async (findingId) => {
      try {
        return await getInvestigation(findingId);
      } catch {
        return null;
      }
    },
  );

  // Auto-poll while investigation is active
  createEffect(() => {
    const inv = investigation();
    const isActive =
      inv?.status === 'running' ||
      inv?.status === 'pending' ||
      props.investigationStatus === 'running';
    if (!isActive) return;

    const interval = setInterval(() => refetch(), 5000);
    onCleanup(() => clearInterval(interval));
  });

  const canReinvestigate = () => {
    if (Date.now() < cooldownUntil()) return false;
    const inv = investigation();
    if (!inv) return true; // No investigation yet
    if (inv.status === 'running') return false;
    return (
      inv.status === 'failed' ||
      inv.status === 'needs_attention' ||
      inv.outcome === 'cannot_fix' ||
      inv.outcome === 'timed_out' ||
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
      refetch();
      aiIntelligenceStore.loadFindings();
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to start re-investigation');
    } finally {
      setReinvestigating(false);
    }
  };

  return (
    <div class="mt-3 pt-3 border-t border-border-subtle">
      {/* Header */}
      <div class="flex items-center justify-between mb-2">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium text-base-content">Investigation</span>
          {/* Show outcome badge when available, otherwise show status badge */}
          <Show
            when={investigation()?.outcome}
            fallback={
              <Show when={investigation()?.status}>
                <span
                  class={`px-1.5 py-0.5 border text-[10px] font-medium rounded ${getInvestigationStatusBadgeClasses(investigation()!.status)}`}
                >
                  {getInvestigationStatusLabel(investigation()!.status)}
                </span>
              </Show>
            }
          >
            <span
              class={`px-1.5 py-0.5 border text-[10px] font-medium rounded ${getInvestigationOutcomeBadgeClasses(investigation()!.outcome!)}`}
            >
              {getInvestigationOutcomeLabel(investigation()!.outcome!)}
            </span>
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
            class="flex items-center gap-1 px-2 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900 rounded transition-colors disabled:opacity-50"
          >
            <RefreshCwIcon class={`w-3 h-3 ${reinvestigating() ? 'animate-spin' : ''}`} />
            Re-investigate
          </button>
        </Show>
        <Show when={!canReinvestigate() && Date.now() < cooldownUntil()}>
          <span class="text-xs text-muted">Re-investigation started</span>
        </Show>
      </div>

      {/* Loading */}
      <Show
        when={
          !getInvestigationSectionState(investigation.loading, !!investigation()).empty &&
          getInvestigationSectionState(investigation.loading, !!investigation()).text
        }
      >
        <div class="flex items-center gap-2 text-xs text-muted py-2">
          <span class="h-3 w-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
          {getInvestigationSectionState(investigation.loading, !!investigation()).text}
        </div>
      </Show>

      {/* No investigation data */}
      <Show when={getInvestigationSectionState(investigation.loading, !!investigation()).empty}>
        <p class="text-xs text-muted py-1">
          {getInvestigationSectionState(investigation.loading, !!investigation()).text}
        </p>
      </Show>

      <Show when={investigationRecord().hasRecord}>
        <div class="mb-2 rounded border border-border bg-surface-alt p-2">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-xs font-medium text-base-content">Patrol record</span>
            <span class="rounded border border-border bg-surface px-1.5 py-0.5 text-[10px] font-medium text-muted">
              {investigationRecord().statusLabel}
            </span>
            <Show when={investigationRecord().outcomeLabel}>
              <span class="rounded border border-border bg-surface px-1.5 py-0.5 text-[10px] font-medium text-muted">
                {investigationRecord().outcomeLabel}
              </span>
            </Show>
            <Show when={investigationRecord().confidenceLabel}>
              <span class="text-[10px] text-muted">{investigationRecord().confidenceLabel}</span>
            </Show>
          </div>

          <Show when={investigationRecord().conclusion}>
            <p class="mt-2 text-sm text-base-content">{investigationRecord().conclusion}</p>
          </Show>
          <Show when={investigationRecord().recommendedAction}>
            <p class="mt-1 text-xs text-muted">
              <span class="font-medium text-base-content">Recommended action:</span>{' '}
              {investigationRecord().recommendedAction}
            </p>
          </Show>

          <Show when={investigationRecord().proposedFix}>
            {(fix) => (
              <div class="mt-2 rounded border border-border-subtle bg-surface p-2 text-xs">
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

          <Show when={investigationRecord().toolsUsed.length > 0}>
            <div class="mt-2 flex flex-wrap gap-1">
              <For each={investigationRecord().toolsUsed}>
                {(tool) => (
                  <span class="rounded bg-surface px-1.5 py-0.5 text-[10px] font-medium text-muted">
                    {tool}
                  </span>
                )}
              </For>
            </div>
          </Show>

          <Show when={investigationRecord().error}>
            <div class="mt-2 rounded border border-red-200 bg-red-50 p-2 text-xs text-red-700 dark:border-red-800 dark:bg-red-900 dark:text-red-300">
              {investigationRecord().error}
            </div>
          </Show>
        </div>
      </Show>

      {/* Investigation details */}
      <Show when={!investigation.loading && investigation()}>
        {(inv) => (
          <div class="space-y-2">
            {/* Error message for failed investigations */}
            <Show
              when={
                inv().error &&
                (inv().status === 'failed' ||
                  inv().outcome === 'timed_out' ||
                  inv().outcome === 'fix_failed' ||
                  inv().outcome === 'fix_verification_failed' ||
                  inv().outcome === 'fix_verification_unknown' ||
                  inv().outcome === 'needs_attention' ||
                  inv().outcome === 'cannot_fix')
              }
            >
              <div class="text-xs text-red-700 dark:text-red-300 bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 rounded p-2">
                {inv().error}
              </div>
            </Show>

            {/* Summary */}
            <Show when={inv().summary}>
              <div class="text-sm text-muted bg-surface-alt rounded p-2">{inv().summary}</div>
            </Show>

            {/* Tools used + turn count */}
            <div class="flex items-center gap-2 flex-wrap">
              <Show when={inv().tools_used && inv().tools_used!.length > 0}>
                <div class="flex items-center gap-1 flex-wrap">
                  <For each={inv().tools_used}>
                    {(tool) => (
                      <span class="px-1.5 py-0.5 rounded text-base-content text-[10px] font-medium">
                        {tool}
                      </span>
                    )}
                  </For>
                </div>
              </Show>
              <span class="text-[10px] text-muted">
                {inv().turn_count} turn{inv().turn_count === 1 ? '' : 's'}
              </span>
              <Show when={inv().started_at}>
                <span class="text-[10px] text-muted">
                  started {formatTimestamp(inv().started_at)}
                </span>
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
              <InvestigationMessages findingId={props.findingId} />
            </Show>
          </div>
        )}
      </Show>
    </div>
  );
};

export default InvestigationSection;
