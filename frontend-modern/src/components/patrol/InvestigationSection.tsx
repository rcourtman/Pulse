/**
 * InvestigationSection
 *
 * Inline investigation details within expanded finding.
 * Replaces the drawer. Shows status, summary, tools used, and
 * a collapsible investigation thread.
 */

import { Component, createSignal, createResource, createEffect, onCleanup, Show, For } from 'solid-js';
import {
  getInvestigation,
  reinvestigateFinding,
  investigationStatusLabels,
  investigationOutcomeLabels,
  investigationOutcomeColors,
  formatTimestamp,
} from '@/api/patrol';
import { InvestigationMessages } from './InvestigationMessages';
import { notificationStore } from '@/stores/notifications';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

interface InvestigationSectionProps {
  findingId: string;
  investigationStatus?: string;
  investigationOutcome?: string;
  investigationAttempts?: number;
}

const statusColors: Record<string, string> = {
  pending: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400',
  running: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
  completed: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300',
  failed: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
  needs_attention: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
};


export const InvestigationSection: Component<InvestigationSectionProps> = (props) => {
  const [showThread, setShowThread] = createSignal(false);
  const [reinvestigating, setReinvestigating] = createSignal(false);
  const [cooldownUntil, setCooldownUntil] = createSignal(0);

  const [investigation, { refetch }] = createResource(
    () => props.findingId,
    async (findingId) => {
      try {
        return await getInvestigation(findingId);
      } catch {
        return null;
      }
    }
  );

  // Auto-poll while investigation is active
  createEffect(() => {
    const inv = investigation();
    const isActive = inv?.status === 'running' || inv?.status === 'pending'
      || props.investigationStatus === 'running';
    if (!isActive) return;

    const interval = setInterval(() => refetch(), 5000);
    onCleanup(() => clearInterval(interval));
  });

  const canReinvestigate = () => {
    if (Date.now() < cooldownUntil()) return false;
    const inv = investigation();
    if (!inv) return true; // No investigation yet
    if (inv.status === 'running') return false;
    return inv.status === 'failed' || inv.status === 'needs_attention' || inv.outcome === 'cannot_fix' || inv.outcome === 'timed_out' || inv.outcome === 'fix_verification_failed';
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
    <div class="mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
      {/* Header */}
      <div class="flex items-center justify-between mb-2">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium text-gray-900 dark:text-gray-100">Investigation</span>
          {/* Show outcome badge when available, otherwise show status badge */}
          <Show when={investigation()?.outcome}
            fallback={
              <Show when={investigation()?.status}>
                <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${statusColors[investigation()!.status] || statusColors.pending}`}>
                  {investigationStatusLabels[investigation()!.status] || investigation()!.status}
                </span>
              </Show>
            }
          >
            <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${investigationOutcomeColors[investigation()!.outcome!] || investigationOutcomeColors.needs_attention}`}>
              {investigationOutcomeLabels[investigation()!.outcome!] || investigation()!.outcome}
            </span>
          </Show>
          <Show when={props.investigationAttempts && props.investigationAttempts > 1}>
            <span class="text-[10px] text-gray-500 dark:text-gray-400">
              attempt {props.investigationAttempts}
            </span>
          </Show>
        </div>
        <Show when={canReinvestigate()}>
          <button
            type="button"
            onClick={handleReinvestigate}
            disabled={reinvestigating()}
            class="flex items-center gap-1 px-2 py-1 text-xs font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded transition-colors disabled:opacity-50"
          >
            <RefreshCwIcon class={`w-3 h-3 ${reinvestigating() ? 'animate-spin' : ''}`} />
            Re-investigate
          </button>
        </Show>
        <Show when={!canReinvestigate() && Date.now() < cooldownUntil()}>
          <span class="text-xs text-gray-500 dark:text-gray-400">Re-investigation started</span>
        </Show>
      </div>

      {/* Loading */}
      <Show when={investigation.loading}>
        <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400 py-2">
          <span class="h-3 w-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
          Loading investigation...
        </div>
      </Show>

      {/* No investigation data */}
      <Show when={!investigation.loading && !investigation()}>
        <p class="text-xs text-gray-500 dark:text-gray-400 py-1">
          No investigation data available. Enable patrol autonomy to investigate findings.
        </p>
      </Show>

      {/* Investigation details */}
      <Show when={!investigation.loading && investigation()}>
        {(inv) => (
          <div class="space-y-2">
            {/* Summary */}
            <Show when={inv().summary}>
              <div class="text-sm text-gray-600 dark:text-gray-400 bg-gray-50 dark:bg-gray-800/50 rounded p-2">
                {inv().summary}
              </div>
            </Show>

            {/* Tools used + turn count */}
            <div class="flex items-center gap-2 flex-wrap">
              <Show when={inv().tools_used && inv().tools_used!.length > 0}>
                <div class="flex items-center gap-1 flex-wrap">
                  <For each={inv().tools_used}>
                    {(tool) => (
                      <span class="px-1.5 py-0.5 rounded bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 text-[10px] font-medium">
                        {tool}
                      </span>
                    )}
                  </For>
                </div>
              </Show>
              <span class="text-[10px] text-gray-500 dark:text-gray-400">
                {inv().turn_count} turn{inv().turn_count === 1 ? '' : 's'}
              </span>
              <Show when={inv().started_at}>
                <span class="text-[10px] text-gray-500 dark:text-gray-400">
                  started {formatTimestamp(inv().started_at)}
                </span>
              </Show>
            </div>

            {/* Show thread toggle */}
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); setShowThread(!showThread()); }}
              class="text-xs text-purple-600 dark:text-purple-400 hover:underline flex items-center gap-1"
            >
              {showThread() ? 'Hide investigation thread' : 'Show investigation thread'}
              <svg
                class={`w-3 h-3 transition-transform ${showThread() ? 'rotate-90' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
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
