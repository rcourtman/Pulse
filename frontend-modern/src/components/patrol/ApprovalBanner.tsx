/**
 * ApprovalBanner
 *
 * Sticky amber banner at top of page content area.
 * Surfaces pending investigation fix approvals (5-min expiry).
 * - Hidden when no pending approvals
 * - Single approval: inline approve/deny
 * - Multiple: count + "Review" button to scroll to first finding
 */

import { Component, Show, createMemo, createSignal, createEffect, onCleanup } from 'solid-js';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { notificationStore } from '@/stores/notifications';
import type { ApprovalRequest } from '@/api/ai';
import {
  getApprovalExpiryStatusLabel,
  getApprovalRiskPresentation,
} from '@/utils/approvalRiskPresentation';
import { Button } from '@/components/shared/Button';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import CheckIcon from 'lucide-solid/icons/check';
import XIcon from 'lucide-solid/icons/x';

interface ApprovalBannerProps {
  onScrollToFinding?: (findingId: string) => void;
}

const APPROVAL_BANNER_BADGE_PROPS = { size: 'xs', shape: 'rounded' } as const;

export const ApprovalBanner: Component<ApprovalBannerProps> = (props) => {
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);
  const [tick, setTick] = createSignal(Date.now());

  const pending = createMemo(() => aiIntelligenceStore.patrolPendingApprovals);

  // Only tick when there are pending approvals to avoid unnecessary work
  createEffect(() => {
    if (pending().length > 0) {
      const tickInterval = setInterval(() => setTick(Date.now()), 1000);
      onCleanup(() => clearInterval(tickInterval));
    }
  });

  const firstApproval = createMemo(() => pending()[0] ?? null);

  const expiryStatusLabel = (expiresAt?: string | null) => {
    return getApprovalExpiryStatusLabel(expiresAt, tick());
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

  const firstApprovalRisk = createMemo(() =>
    firstApproval() ? getApprovalRiskPresentation(firstApproval()!.riskLevel) : null,
  );

  return (
    <Show when={pending().length > 0}>
      <div class="bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md px-4 py-3">
        <div class="flex items-center justify-between gap-3 flex-wrap">
          <div class="flex items-center gap-3">
            <div class="flex-shrink-0 p-1.5 border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900 rounded-md">
              <ShieldAlertIcon class="w-4 h-4 text-amber-500 dark:text-amber-400" />
            </div>
            <div>
              <Show when={pending().length === 1 && firstApproval()}>
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="text-sm font-medium text-amber-900 dark:text-amber-100">
                    Fix awaiting approval
                  </span>
                  <MetadataBadge
                    {...APPROVAL_BANNER_BADGE_PROPS}
                    tone={firstApprovalRisk()!.badgeTone}
                  >
                    {firstApprovalRisk()!.label} risk
                  </MetadataBadge>
                  <span class="text-xs text-amber-700 dark:text-amber-300">
                    {expiryStatusLabel(firstApproval()!.expiresAt)}
                  </span>
                </div>
                <p
                  class="text-xs text-amber-700 dark:text-amber-300 mt-0.5 max-w-xl truncate"
                  title={firstApproval()!.context}
                >
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
              <Button
                type="button"
                variant="success"
                size="sm"
                onClick={() => handleApprove(firstApproval()!)}
                disabled={actionLoading() === firstApproval()!.id}
                class="gap-1.5"
              >
                <Show when={actionLoading() === firstApproval()!.id}>
                  <LoadingSpinner size="sm" tone="inverse" />
                </Show>
                <Show when={actionLoading() !== firstApproval()!.id}>
                  <CheckIcon class="w-3.5 h-3.5" />
                </Show>
                Approve & Execute
              </Button>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={() => handleDeny(firstApproval()!)}
                disabled={actionLoading() === firstApproval()!.id}
                class="gap-1.5"
              >
                <XIcon class="w-3.5 h-3.5" />
                Deny
              </Button>
            </Show>
            <Show when={pending().length > 1}>
              <Button
                type="button"
                variant="warningSolid"
                size="sm"
                onClick={handleReview}
                class="gap-1.5"
              >
                Review
              </Button>
            </Show>
          </div>
        </div>
      </div>
    </Show>
  );
};

export default ApprovalBanner;
