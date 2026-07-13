/**
 * ApprovalBanner
 *
 * Contextual handoff from Patrol to the canonical Actions inbox.
 * Surfaces pending investigation fix approvals without creating a second
 * approval or execution surface inside Patrol.
 * - Hidden when no pending approvals
 * - Single approval: deep-links to the exact governed action when available
 * - Multiple approvals: links to the open Actions inbox
 */

import { Component, Show, createMemo, createSignal, createEffect, onCleanup } from 'solid-js';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import {
  getApprovalExpiryStatusLabel,
  getApprovalRiskPresentation,
} from '@/utils/approvalRiskPresentation';
import { ButtonLink } from '@/components/shared/Button';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { buildActionReviewPath } from '@/features/actions/actionRouting';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import ArrowRightIcon from 'lucide-solid/icons/arrow-right';

const APPROVAL_BANNER_BADGE_PROPS = { size: 'xs', shape: 'rounded' } as const;

export const ApprovalBanner: Component = () => {
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

  const firstApprovalRisk = createMemo(() =>
    firstApproval() ? getApprovalRiskPresentation(firstApproval()!.riskLevel) : null,
  );
  const reviewPath = createMemo(() =>
    buildActionReviewPath(pending().length === 1 ? firstApproval()?.plan?.actionId : undefined),
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
                    Action awaiting approval
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
                  {pending().length} actions awaiting your approval
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
      </div>
    </Show>
  );
};

export default ApprovalBanner;
