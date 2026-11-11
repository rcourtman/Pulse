/**
 * Patrol action lifecycle
 *
 * Presents the canonical typed action referenced by an investigation. Legacy
 * command-shaped proposed_fix/approval_id fields are history only and never
 * become executable UI authority.
 *
 * An action still open is reviewed where the reader is: the button asks the
 * surface that hosts this section to open the shared Actions review dialog,
 * which owns every decision and execution call. The host owns the dialog
 * because a finding review redraws when its finding changes, and approving
 * changes it: a dialog mounted in here closed itself the moment it was used.
 */

import ArrowRightIcon from 'lucide-solid/icons/arrow-right';
import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import { AIAPI, type InvestigationSession } from '@/api/ai';
import { Button, ButtonLink } from '@/components/shared/Button';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { buildActionReviewPath } from '@/features/actions/actionRouting';
import { useResolvedResource } from '@/features/platformNavigation/useResolvedResource';
import type { ActionAuditState, PatrolActionReference } from '@/types/actionAudit';

interface ApprovalSectionProps {
  findingId: string;
  hasAction?: boolean;
  investigationOutcome?: string;
  /** The finding's resource. The action may act on a different one. */
  resourceId?: string;
  resourceName?: string;
  /**
   * Opens the shared Actions review for this action where the reader is.
   * Without it the section only links to the Actions page.
   */
  onReviewAction?: (actionId: string) => Promise<void> | void;
}

const FIX_RELATED_OUTCOMES = new Set([
  'fix_queued',
  'fix_executed',
  'fix_failed',
  'fix_rejected',
  'fix_verified',
  'fix_verification_failed',
  'fix_verification_unknown',
]);

const BADGE_PROPS = { size: 'xs', shape: 'rounded' } as const;

const ACTION_REVALIDATE_MS = 30_000;

function statePresentation(state: ActionAuditState): {
  label: string;
  tone: 'neutral' | 'info' | 'warning' | 'success' | 'danger';
} {
  switch (state) {
    case 'planned':
      return { label: 'Ready to run', tone: 'info' };
    case 'pending_approval':
      return { label: 'Approval required', tone: 'warning' };
    case 'approved':
      return { label: 'Approved', tone: 'success' };
    case 'rejected':
      return { label: 'Rejected', tone: 'warning' };
    case 'expired':
      return { label: 'Expired', tone: 'warning' };
    case 'executing':
      return { label: 'Applying', tone: 'info' };
    case 'completed':
      return { label: 'Completed', tone: 'success' };
    case 'failed':
      return { label: 'Failed', tone: 'danger' };
  }
}

function actionHubLabel(state: ActionAuditState): string {
  switch (state) {
    case 'executing':
      return 'Track in Actions';
    case 'rejected':
    case 'expired':
    case 'completed':
    case 'failed':
      return 'View outcome in Actions';
    case 'planned':
    case 'pending_approval':
    case 'approved':
      return 'Review in Actions';
  }
}

function capabilityLabel(value: string): string {
  return value.replace(/[._-]+/g, ' ').replace(/\b\w/g, (character) => character.toUpperCase());
}

/** An action that still waits on a person, so its review opens in place. */
function isOpenForReview(state: ActionAuditState): boolean {
  return state === 'planned' || state === 'pending_approval' || state === 'approved';
}

function reviewButtonLabel(state: ActionAuditState): string {
  switch (state) {
    case 'pending_approval':
      return 'Review and approve';
    case 'approved':
    case 'planned':
      return 'Review and run';
    default:
      return 'Review';
  }
}

/** What the action is waiting for, in the reader's terms. */
function waitingSentence(state: ActionAuditState): string {
  switch (state) {
    case 'pending_approval':
      return 'Nothing has changed yet. It runs only after you approve it.';
    case 'planned':
    case 'approved':
      return 'Nothing has changed yet. It is ready to run.';
    case 'executing':
      return 'Running now.';
    case 'rejected':
      return 'Rejected. Nothing was changed.';
    case 'expired':
      return 'The plan expired before anyone approved it. Nothing was changed.';
    case 'completed':
    case 'failed':
      return '';
  }
}

export const ApprovalSection: Component<ApprovalSectionProps> = (props) => {
  const shouldShow = createMemo(() =>
    Boolean(
      props.hasAction ||
      (props.investigationOutcome && FIX_RELATED_OUTCOMES.has(props.investigationOutcome)),
    ),
  );
  // As in InvestigationSection, reads stay out of the app's route Suspense
  // boundary, whose fallback replaces the page and loses the reader's place,
  // and the mounted review re-reads its action on the cadence at which
  // findings refreshes used to remount it. A finding without an investigation
  // reads null; any other failure keeps the last answer for the next read to
  // replace.
  const investigationQuery = createNonSuspendingQuery<InvestigationSession | null, string>({
    source: () => (shouldShow() ? props.findingId : null),
    cacheKey: (findingId) => `patrol-action-investigation:${findingId}`,
    retainPreviousValueOnSourceChange: false,
    fetcher: (findingId) => AIAPI.getInvestigation(findingId),
    initialValue: null,
    pollMs: ACTION_REVALIDATE_MS,
  });
  const investigation = investigationQuery.value;

  const action = createMemo<PatrolActionReference | null>(() => investigation()?.action ?? null);
  // The fix often lands on another resource than the finding's: the finding
  // is on the app that broke, the action on the database it needs. The target
  // is the action's own resource, by its registry name.
  const actionResource = useResolvedResource(() => action()?.resource_id);
  const targetName = createMemo(() => {
    const current = action();
    if (!current) return '';
    const resolved = actionResource();
    if (resolved) return resolved.displayName || resolved.name;
    if (props.resourceId && props.resourceId === current.resource_id && props.resourceName) {
      return props.resourceName;
    }
    return current.resource_id;
  });
  const [reviewBusy, setReviewBusy] = createSignal(false);
  const [reviewError, setReviewError] = createSignal('');
  const openReview = async (actionId: string) => {
    if (reviewBusy()) return;
    setReviewBusy(true);
    setReviewError('');
    try {
      await props.onReviewAction?.(actionId);
    } catch (cause) {
      setReviewError(cause instanceof Error ? cause.message : 'The action could not be opened.');
    } finally {
      setReviewBusy(false);
    }
  };
  const verificationStatus = createMemo(() => {
    switch (investigation()?.outcome) {
      case 'fix_verified':
        return 'verified';
      case 'fix_verification_failed':
        return 'failed';
      case 'fix_verification_unknown':
        return 'unverified';
      default:
        return 'unknown';
    }
  });
  return (
    <Show when={shouldShow()}>
      <div class="mt-3 border-t border-border-subtle pt-3">
        <Show
          when={investigationQuery.resolvedOnce()}
          fallback={
            <div class="flex items-center gap-2 text-sm text-muted">
              <LoadingSpinner size="sm" />
              Loading the proposed action…
            </div>
          }
        >
          <Show
            when={action()}
            fallback={
              <div class="space-y-3">
                <div>
                  <div class="text-sm font-medium text-base-content">
                    Action details unavailable
                  </div>
                  <div class="mt-1 text-sm text-muted">
                    This investigation predates the typed action lifecycle or its action record is
                    no longer available. It cannot be approved or executed from legacy fix data.
                  </div>
                </div>
              </div>
            }
          >
            {(currentAction) => {
              const presentation = createMemo(() => statePresentation(currentAction().state));
              const preflight = createMemo(() => currentAction().plan.preflight);
              const reviewedPlanHash = createMemo(
                () => currentAction().plan.planHash?.trim() || '',
              );
              const canReviewInPlace = () =>
                Boolean(
                  props.onReviewAction &&
                  isOpenForReview(currentAction().state) &&
                  reviewedPlanHash(),
                );
              return (
                <div class="space-y-3">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="text-sm font-medium text-base-content">
                      {capabilityLabel(currentAction().capability_name)}
                    </span>
                    <MetadataBadge {...BADGE_PROPS} tone={presentation().tone}>
                      {presentation().label}
                    </MetadataBadge>
                    <Show when={currentAction().plan.rollbackAvailable}>
                      <MetadataBadge {...BADGE_PROPS} tone="neutral">
                        Rollback available
                      </MetadataBadge>
                    </Show>
                  </div>

                  <div class="space-y-1 text-sm text-muted">
                    <div class="text-base-content" data-testid="patrol-action-target">
                      {capabilityLabel(currentAction().capability_name)} {targetName()}
                    </div>
                    <Show when={waitingSentence(currentAction().state)}>
                      {(sentence) => <div class="text-xs">{sentence()}</div>}
                    </Show>
                    <Show when={preflight()?.intendedChange}>
                      <div class="text-xs">Change: {preflight()!.intendedChange}</div>
                    </Show>
                    <Show when={preflight()?.dryRunAvailable && preflight()?.dryRunSummary}>
                      <div class="text-xs">Dry run: {preflight()!.dryRunSummary}</div>
                    </Show>
                  </div>

                  <Show when={(preflight()?.safetyChecks?.length || 0) > 0}>
                    <details class="rounded-sm border border-border bg-surface-alt px-2 py-1.5 text-xs">
                      <summary class="cursor-pointer font-medium text-muted">
                        Safety and verification
                      </summary>
                      <ul class="mt-2 list-disc space-y-1 pl-4 text-muted">
                        <For each={preflight()?.safetyChecks}>{(check) => <li>{check}</li>}</For>
                        <For each={preflight()?.verificationSteps}>{(step) => <li>{step}</li>}</For>
                      </ul>
                    </details>
                  </Show>

                  <Show when={currentAction().state === 'completed'}>
                    <div
                      class={`text-sm font-medium ${
                        verificationStatus() === 'verified'
                          ? 'text-green-600 dark:text-green-400'
                          : 'text-amber-700 dark:text-amber-300'
                      }`}
                    >
                      {verificationStatus() === 'verified'
                        ? 'Outcome verified'
                        : 'Execution finished. Verification was not conclusive'}
                    </div>
                  </Show>
                  <Show when={currentAction().state === 'failed'}>
                    <div class="text-sm font-medium text-red-600 dark:text-red-400">
                      The action failed before verification.
                    </div>
                  </Show>

                  <Show when={!reviewedPlanHash()}>
                    <div
                      role="alert"
                      class="rounded-sm border border-amber-300 bg-amber-50 p-2 text-sm text-amber-900 dark:bg-amber-950/40 dark:text-amber-200"
                    >
                      This action has no reviewed plan identity. Create a new plan before approving
                      or running it.
                    </div>
                  </Show>

                  <Show when={reviewError()}>
                    <div role="alert" class="text-xs text-red-700 dark:text-red-300">
                      {reviewError()}
                    </div>
                  </Show>
                  <div class="flex flex-wrap items-center gap-2 border-t border-border-subtle pt-3">
                    <Show when={canReviewInPlace()}>
                      <Button
                        variant="primary"
                        size="sm"
                        isLoading={reviewBusy()}
                        onClick={(event) => {
                          event.stopPropagation();
                          void openReview(currentAction().action_id);
                        }}
                      >
                        {reviewButtonLabel(currentAction().state)}
                      </Button>
                    </Show>
                    <ButtonLink
                      href={buildActionReviewPath(currentAction().action_id)}
                      variant={canReviewInPlace() ? 'ghost' : 'primary'}
                      size="sm"
                      class="gap-1.5"
                    >
                      {actionHubLabel(currentAction().state)}
                      <ArrowRightIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </ButtonLink>
                  </div>
                </div>
              );
            }}
          </Show>
        </Show>
      </div>
    </Show>
  );
};

export default ApprovalSection;
