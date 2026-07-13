/**
 * Patrol action lifecycle
 *
 * Presents the canonical typed action referenced by an investigation. Legacy
 * command-shaped proposed_fix/approval_id fields are history only and never
 * become executable UI authority.
 */

import ArrowRightIcon from 'lucide-solid/icons/arrow-right';
import MessageSquareIcon from 'lucide-solid/icons/message-square';
import { Component, For, Show, createMemo, createResource } from 'solid-js';
import { AIAPI } from '@/api/ai';
import { Button, ButtonLink } from '@/components/shared/Button';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { buildActionReviewPath } from '@/features/actions/actionRouting';
import {
  buildPatrolAssistantFindingHandoff,
  buildPatrolAssistantProposedFixBriefingInput,
} from '@/features/patrol/patrolInvestigationContextModel';
import { aiChatStore } from '@/stores/aiChat';
import type { ActionAuditState, PatrolActionReference } from '@/types/actionAudit';

interface ApprovalSectionProps {
  findingId: string;
  investigationOutcome?: string;
  findingTitle?: string;
  resourceName?: string;
  resourceType?: string;
  resourceId?: string;
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

export const ApprovalSection: Component<ApprovalSectionProps> = (props) => {
  const [investigation] = createResource(
    () => ({ findingId: props.findingId, outcome: props.investigationOutcome }),
    async ({ findingId, outcome }) => {
      if (!outcome || !FIX_RELATED_OUTCOMES.has(outcome)) return null;
      try {
        return await AIAPI.getInvestigation(findingId);
      } catch {
        return null;
      }
    },
  );

  const action = createMemo<PatrolActionReference | null>(() => investigation()?.action ?? null);
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
  const shouldShow = createMemo(() =>
    Boolean(props.investigationOutcome && FIX_RELATED_OUTCOMES.has(props.investigationOutcome)),
  );

  const handleDiscuss = (event: Event) => {
    event.stopPropagation();
    const current = action();
    const handoff = buildPatrolAssistantFindingHandoff({
      id: props.findingId,
      title: props.findingTitle || 'Patrol finding',
      subject: props.resourceName || 'affected resource',
      description:
        current?.plan.message ||
        investigation()?.summary ||
        'Review the current Patrol finding and its governed action state.',
      findingStatus: 'active',
      investigationOutcome: props.investigationOutcome,
      loopState: props.investigationOutcome || current?.state,
      resourceId: props.resourceId || current?.resource_id,
      resourceName: props.resourceName,
      resourceType: props.resourceType,
      pendingApproval: current
        ? {
            status: current.state,
            targetName: props.resourceName || current.resource_id,
            actionId: current.action_id,
            actionApprovalPolicy: current.plan.approvalPolicy,
            actionPlanExpiresAt: current.plan.expiresAt,
            actionPlanMessage: current.plan.message,
            actionPreflight: current.plan.preflight?.intendedChange,
            actionDryRunSummary: current.plan.preflight?.dryRunSummary,
            actionRequestedBy: 'pulse_patrol',
          }
        : null,
      proposedFix: buildPatrolAssistantProposedFixBriefingInput(
        current
          ? {
              description: current.plan.message || capabilityLabel(current.capability_name),
              targetHost: props.resourceName || current.resource_id,
              commandCount: 0,
              destructive: false,
            }
          : null,
      ),
    });
    aiChatStore.open(handoff.context);
  };

  return (
    <Show when={shouldShow()}>
      <div class="mt-3 border-t border-border-subtle pt-3">
        <Show
          when={!investigation.loading}
          fallback={
            <div class="flex items-center gap-2 text-sm text-muted">
              <LoadingSpinner size="sm" />
              Loading governed action…
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
                <Button type="button" variant="primary" size="sm" onClick={handleDiscuss}>
                  <MessageSquareIcon class="h-3.5 w-3.5" />
                  Discuss with Assistant
                </Button>
              </div>
            }
          >
            {(currentAction) => {
              const presentation = createMemo(() => statePresentation(currentAction().state));
              const preflight = createMemo(() => currentAction().plan.preflight);
              const reviewedPlanHash = createMemo(
                () => currentAction().plan.planHash?.trim() || '',
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
                    <div>
                      {currentAction().plan.message || 'Patrol proposed a governed action.'}
                    </div>
                    <div class="text-xs">
                      Target: {props.resourceName || currentAction().resource_id}
                    </div>
                    <Show when={preflight()?.intendedChange}>
                      <div class="text-xs">Change: {preflight()!.intendedChange}</div>
                    </Show>
                    <Show when={preflight()?.dryRunSummary}>
                      <div class="text-xs">Dry run: {preflight()!.dryRunSummary}</div>
                    </Show>
                  </div>

                  <Show when={(preflight()?.safetyChecks?.length || 0) > 0}>
                    <details class="rounded border border-border bg-surface-alt px-2 py-1.5 text-xs">
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
                        : 'Execution finished; verification was not conclusive'}
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
                      class="rounded border border-amber-300 bg-amber-50 p-2 text-sm text-amber-900 dark:bg-amber-950/40 dark:text-amber-200"
                    >
                      This action has no reviewed plan identity. Create a new plan before approving
                      or running it.
                    </div>
                  </Show>

                  <div class="flex flex-wrap items-center gap-2 border-t border-border-subtle pt-3">
                    <ButtonLink
                      href={buildActionReviewPath(currentAction().action_id)}
                      variant="primary"
                      size="sm"
                      class="gap-1.5"
                    >
                      {actionHubLabel(currentAction().state)}
                      <ArrowRightIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </ButtonLink>
                    <Button type="button" variant="ghost" size="sm" onClick={handleDiscuss}>
                      <MessageSquareIcon class="h-3.5 w-3.5" />
                      Discuss with Assistant
                    </Button>
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
