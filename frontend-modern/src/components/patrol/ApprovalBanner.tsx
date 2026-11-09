/**
 * ApprovalBanner
 *
 * Contextual handoff from Patrol to the canonical Actions inbox.
 * Surfaces pending investigation fix approvals without creating a second
 * approval or execution surface inside Patrol.
 * - Hidden only when no pending approvals and no failed read
 * - Single approval: deep-links to the exact governed action when available
 * - Multiple approvals: links to the open Actions inbox
 * - Given targetIds (a findings list scoped to one resource), only the
 *   approvals for those findings
 */

import { Component, Show, createMemo, createSignal, createEffect, onCleanup } from 'solid-js';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import {
  getApprovalExpiryStatusLabel,
  getApprovalRiskPresentation,
} from '@/utils/approvalRiskPresentation';
import { Button, ButtonLink } from '@/components/shared/Button';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { buildActionReviewPath } from '@/features/actions/actionRouting';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import ArrowRightIcon from 'lucide-solid/icons/arrow-right';

const APPROVAL_BANNER_BADGE_PROPS = { size: 'xs', shape: 'rounded' } as const;

export const ApprovalBanner: Component<{ targetIds?: ReadonlySet<string> }> = (props) => {
  const [tick, setTick] = createSignal(Date.now());

  const pending = createMemo(() => {
    const approvals = aiIntelligenceStore.patrolPendingApprovals;
    const targetIds = props.targetIds;
    return targetIds ? approvals.filter((approval) => targetIds.has(approval.targetId)) : approvals;
  });

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

  const firstApprovalRisk = createMemo(() =>
    firstApproval() ? getApprovalRiskPresentation(firstApproval()!.riskLevel) : null,
  );
  const reviewPath = createMemo(() =>
    buildActionReviewPath(pending().length === 1 ? firstApproval()?.plan?.actionId : undefined),
  );

  return (
    <Show when={pending().length > 0 || aiIntelligenceStore.approvalsError}>
      <div class="space-y-2 bg-amber-50 dark:bg-amber-900/25 border border-amber-200 dark:border-amber-800 rounded-md px-3 py-2">
        <Show when={aiIntelligenceStore.approvalsError}>
          <div class="flex items-start justify-between gap-2">
            <div role="status" class="min-w-0 flex-1 text-amber-900 dark:text-amber-100">
              <p class="text-sm font-medium">Approval status unavailable</p>
              <p class="mt-0.5 text-xs">
                <Show
                  when={pending().length > 0}
                  fallback="Pending approvals could not be checked. This does not mean there are none."
                >
                  Showing last-known approvals. Their current status could not be checked.
                </Show>
              </p>
            </div>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              class="min-h-11 shrink-0"
              aria-label={
                aiIntelligenceStore.approvalsLoading ? 'Checking approvals…' : 'Refresh approvals'
              }
              disabled={aiIntelligenceStore.approvalsLoading}
              onClick={() => void aiIntelligenceStore.loadPendingApprovals()}
            >
              {aiIntelligenceStore.approvalsLoading ? 'Checking…' : 'Refresh'}
            </Button>
          </div>
        </Show>
        <Show when={pending().length > 0}>
          <div class="flex items-center justify-between gap-2 flex-wrap">
            <div class="flex min-w-0 items-center gap-2">
              <div class="shrink-0 p-1.5 border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900/25 rounded-md">
                <ShieldAlertIcon class="w-4 h-4 text-amber-500 dark:text-amber-400" />
              </div>
              <div class="min-w-0">
                <Show when={pending().length === 1 && firstApproval()}>
                  <div class="flex items-center gap-2 flex-wrap">
                    <span class="text-sm font-medium text-amber-900 dark:text-amber-100">
                      Action awaiting approval
                    </span>
                    <Show when={aiIntelligenceStore.approvalsError}>
                      <MetadataBadge {...APPROVAL_BANNER_BADGE_PROPS} tone="warning">
                        Last known
                      </MetadataBadge>
                    </Show>
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
                  <p class="text-xs text-amber-700 dark:text-amber-300 mt-0.5 max-w-xl break-words">
                    {firstApproval()!.context}
                  </p>
                </Show>
                <Show when={pending().length > 1}>
                  <span class="text-sm font-medium text-amber-900 dark:text-amber-100">
                    {pending().length} actions awaiting your approval
                    <Show when={aiIntelligenceStore.approvalsError}>{' · last known'}</Show>
                  </span>
                </Show>
              </div>
            </div>

            <div class="flex items-center gap-2">
              <ButtonLink href={reviewPath()} variant="warningSolid" size="sm" class="gap-1.5">
                Review in Actions
                <ArrowRightIcon class="h-3.5 w-3.5" aria-hidden="true" />
              </ButtonLink>
            </div>
          </div>
        </Show>
      </div>
    </Show>
  );
};

export default ApprovalBanner;
