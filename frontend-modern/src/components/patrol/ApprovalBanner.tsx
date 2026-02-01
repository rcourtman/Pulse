/**
 * ApprovalBanner
 *
 * Sticky amber banner at top of page content area.
 * Surfaces pending investigation fix approvals (5-min expiry).
 * - Hidden when no pending approvals
 * - Single approval: inline approve/deny
 * - Multiple: count + "Review" button to scroll to first finding
 */

import { Component, Show, createMemo, createSignal, onCleanup } from 'solid-js';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { notificationStore } from '@/stores/notifications';
import type { ApprovalRequest } from '@/api/ai';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import CheckIcon from 'lucide-solid/icons/check';
import XIcon from 'lucide-solid/icons/x';

interface ApprovalBannerProps {
  onScrollToFinding?: (findingId: string) => void;
}

export const ApprovalBanner: Component<ApprovalBannerProps> = (props) => {
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);
  const [tick, setTick] = createSignal(Date.now());

  // Tick every second to keep countdown live
  const tickInterval = setInterval(() => setTick(Date.now()), 1000);
  onCleanup(() => clearInterval(tickInterval));

  const pending = createMemo(() =>
    aiIntelligenceStore.pendingApprovals.filter((a: ApprovalRequest) => a.status === 'pending')
  );

  const firstApproval = createMemo(() => pending()[0] ?? null);

  const timeRemaining = (expiresAt: string) => {
    void tick(); // subscribe to tick signal for reactivity
    const diff = new Date(expiresAt).getTime() - Date.now();
    if (diff <= 0) return 'expired';
    const mins = Math.floor(diff / 60000);
    const secs = Math.floor((diff % 60000) / 1000);
    if (mins > 0) return `${mins}m ${secs}s`;
    return `${secs}s`;
  };

  const riskBadgeColor = (level: string) => {
    switch (level) {
      case 'high': return 'bg-red-200 text-red-800 dark:bg-red-900/50 dark:text-red-300';
      case 'medium': return 'bg-amber-200 text-amber-800 dark:bg-amber-900/50 dark:text-amber-300';
      default: return 'bg-green-200 text-green-800 dark:bg-green-900/50 dark:text-green-300';
    }
  };

  const handleApprove = async (approval: ApprovalRequest) => {
    setActionLoading(approval.id);
    try {
      const result = await aiIntelligenceStore.approveInvestigationFix(approval.id);
      if (result?.success) {
        notificationStore.success('Fix executed successfully');
      } else {
        notificationStore.error(result?.error || 'Fix execution failed');
      }
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to execute fix');
    } finally {
      setActionLoading(null);
    }
  };

  const handleDeny = async (approval: ApprovalRequest) => {
    setActionLoading(approval.id);
    try {
      await aiIntelligenceStore.denyInvestigationFix(approval.id);
      notificationStore.success('Fix denied');
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to deny fix');
    } finally {
      setActionLoading(null);
    }
  };

  const handleReview = () => {
    const findings = aiIntelligenceStore.findingsWithPendingApprovals;
    if (findings.length > 0) {
      props.onScrollToFinding?.(findings[0].id);
    }
  };

  return (
    <Show when={pending().length > 0}>
      <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg px-4 py-3">
        <div class="flex items-center justify-between gap-3 flex-wrap">
          <div class="flex items-center gap-3">
            <div class="flex-shrink-0 p-1.5 bg-amber-100 dark:bg-amber-900/40 rounded-lg">
              <ShieldAlertIcon class="w-4 h-4 text-amber-600 dark:text-amber-400" />
            </div>
            <div>
              <Show when={pending().length === 1 && firstApproval()}>
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="text-sm font-medium text-amber-900 dark:text-amber-100">
                    Fix awaiting approval
                  </span>
                  <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${riskBadgeColor(firstApproval()!.riskLevel)}`}>
                    {firstApproval()!.riskLevel} risk
                  </span>
                  <span class="text-xs text-amber-700 dark:text-amber-300">
                    expires {timeRemaining(firstApproval()!.expiresAt)}
                  </span>
                </div>
                <p class="text-xs text-amber-700 dark:text-amber-300 mt-0.5 max-w-xl truncate" title={firstApproval()!.context}>
                  {firstApproval()!.context}
                </p>
              </Show>
              <Show when={pending().length > 1}>
                <span class="text-sm font-medium text-amber-900 dark:text-amber-100">
                  {pending().length} fixes awaiting your approval
                </span>
              </Show>
            </div>
          </div>

          <div class="flex items-center gap-2">
            <Show when={pending().length === 1 && firstApproval()}>
              <button
                type="button"
                onClick={() => handleApprove(firstApproval()!)}
                disabled={actionLoading() === firstApproval()!.id}
                class="flex items-center gap-1.5 px-3 py-1.5 bg-green-600 hover:bg-green-700 disabled:bg-green-400 text-white text-xs font-medium rounded-md transition-colors"
              >
                <Show when={actionLoading() === firstApproval()!.id}>
                  <span class="h-3 w-3 border-2 border-white border-t-transparent rounded-full animate-spin" />
                </Show>
                <Show when={actionLoading() !== firstApproval()!.id}>
                  <CheckIcon class="w-3.5 h-3.5" />
                </Show>
                Approve & Execute
              </button>
              <button
                type="button"
                onClick={() => handleDeny(firstApproval()!)}
                disabled={actionLoading() === firstApproval()!.id}
                class="flex items-center gap-1.5 px-3 py-1.5 bg-gray-200 hover:bg-gray-300 dark:bg-gray-700 dark:hover:bg-gray-600 disabled:opacity-50 text-gray-700 dark:text-gray-300 text-xs font-medium rounded-md transition-colors"
              >
                <XIcon class="w-3.5 h-3.5" />
                Deny
              </button>
            </Show>
            <Show when={pending().length > 1}>
              <button
                type="button"
                onClick={handleReview}
                class="flex items-center gap-1.5 px-3 py-1.5 bg-amber-600 hover:bg-amber-700 text-white text-xs font-medium rounded-md transition-colors"
              >
                Review
              </button>
            </Show>
          </div>
        </div>
      </div>
    </Show>
  );
};

export default ApprovalBanner;
