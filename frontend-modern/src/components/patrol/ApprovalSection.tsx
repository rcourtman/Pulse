/**
 * Patrol action lifecycle
 *
 * Presents the canonical typed action referenced by an investigation. Legacy
 * command-shaped proposed_fix/approval_id fields are history only and never
 * become executable UI authority.
 */

import CheckIcon from 'lucide-solid/icons/check';
import MessageSquareIcon from 'lucide-solid/icons/message-square';
import PlayIcon from 'lucide-solid/icons/play';
import XIcon from 'lucide-solid/icons/x';
import { Component, For, Show, createMemo, createResource, createSignal } from 'solid-js';
import { AIAPI } from '@/api/ai';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { Button } from '@/components/shared/Button';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import {
  buildPatrolAssistantFindingHandoff,
  buildPatrolAssistantProposedFixBriefingInput,
} from '@/features/patrol/patrolInvestigationContextModel';
import { aiChatStore } from '@/stores/aiChat';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { hasFeature } from '@/stores/license';
import { notificationStore } from '@/stores/notifications';
import type {
  ActionAuditRecord,
  ActionAuditState,
  PatrolActionReference,
} from '@/types/actionAudit';

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

function actionReferenceFromAudit(audit: ActionAuditRecord): PatrolActionReference {
  return {
    action_id: audit.id,
    proposal_id: audit.origin?.proposalId,
    resource_id: audit.request.resourceId,
    capability_name: audit.request.capabilityName,
    state: audit.state,
    plan: audit.plan,
  };
}

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
    case 'executing':
      return { label: 'Applying', tone: 'info' };
    case 'completed':
      return { label: 'Completed', tone: 'success' };
    case 'failed':
      return { label: 'Failed', tone: 'danger' };
  }
}

function capabilityLabel(value: string): string {
  return value.replace(/[._-]+/g, ' ').replace(/\b\w/g, (character) => character.toUpperCase());
}

export const ApprovalSection: Component<ApprovalSectionProps> = (props) => {
  const [busyAction, setBusyAction] = createSignal<string | null>(null);
  const [latestAudit, setLatestAudit] = createSignal<ActionAuditRecord | null>(null);

  const [investigation, { refetch }] = createResource(
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

  const action = createMemo<PatrolActionReference | null>(() => {
    const audit = latestAudit();
    if (audit) return actionReferenceFromAudit(audit);
    return investigation()?.action ?? null;
  });
  const canManageAction = createMemo(() => hasFeature('ai_autofix'));
  const verificationStatus = createMemo(() => {
    const auditedStatus = latestAudit()?.verificationOutcome?.status;
    if (auditedStatus) return auditedStatus;
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

  const refreshPatrol = async () => {
    await refetch();
    await aiIntelligenceStore.loadFindings();
  };

  const execute = async (actionId: string) => {
    const result = await ResourceActionsAPI.executeAction(
      actionId,
      'Operator requested execution from the Patrol action review',
    );
    setLatestAudit(result.audit);
    const verification = result.audit.verificationOutcome?.status;
    if (result.state === 'completed' && verification === 'verified') {
      notificationStore.success('Action completed and verified');
    } else if (result.state === 'completed') {
      notificationStore.warning('Action completed, but verification was inconclusive');
    } else {
      notificationStore.error(result.result?.errorMessage || 'Action failed');
    }
  };

  const handleApproveAndRun = async (event: Event) => {
    event.stopPropagation();
    const current = action();
    if (!current) return;
    setBusyAction('approve');
    try {
      const decision = await ResourceActionsAPI.decideAction(
        current.action_id,
        'approved',
        'Approved from the Patrol action review',
      );
      setLatestAudit(decision.audit);
      await execute(current.action_id);
      await refreshPatrol();
    } catch (error) {
      notificationStore.error((error as Error).message || 'Failed to approve and run action');
    } finally {
      setBusyAction(null);
    }
  };

  const handleRun = async (event: Event) => {
    event.stopPropagation();
    const current = action();
    if (!current) return;
    setBusyAction('execute');
    try {
      await execute(current.action_id);
      await refreshPatrol();
    } catch (error) {
      notificationStore.error((error as Error).message || 'Failed to run action');
    } finally {
      setBusyAction(null);
    }
  };

  const handleReject = async (event: Event) => {
    event.stopPropagation();
    const current = action();
    if (!current) return;
    setBusyAction('reject');
    try {
      const decision = await ResourceActionsAPI.decideAction(
        current.action_id,
        'rejected',
        'Rejected from the Patrol action review',
      );
      setLatestAudit(decision.audit);
      notificationStore.success('Action rejected');
      await refreshPatrol();
    } catch (error) {
      notificationStore.error((error as Error).message || 'Failed to reject action');
    } finally {
      setBusyAction(null);
    }
  };

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
                      {latestAudit()?.result?.errorMessage ||
                        'The action failed before verification.'}
                    </div>
                  </Show>

                  <div class="flex flex-wrap items-center gap-2 border-t border-border-subtle pt-3">
                    <Show when={canManageAction() && currentAction().state === 'pending_approval'}>
                      <Button
                        type="button"
                        variant="success"
                        size="sm"
                        onClick={handleApproveAndRun}
                        disabled={busyAction() !== null}
                      >
                        <Show
                          when={busyAction() === 'approve'}
                          fallback={<CheckIcon class="h-3.5 w-3.5" />}
                        >
                          <LoadingSpinner size="sm" tone="inverse" />
                        </Show>
                        Approve and run
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={handleReject}
                        disabled={busyAction() !== null}
                      >
                        <XIcon class="h-3.5 w-3.5" />
                        Reject
                      </Button>
                    </Show>
                    <Show
                      when={
                        canManageAction() &&
                        (currentAction().state === 'planned' ||
                          currentAction().state === 'approved')
                      }
                    >
                      <Button
                        type="button"
                        variant="success"
                        size="sm"
                        onClick={handleRun}
                        disabled={busyAction() !== null}
                      >
                        <Show
                          when={busyAction() === 'execute'}
                          fallback={<PlayIcon class="h-3.5 w-3.5" />}
                        >
                          <LoadingSpinner size="sm" tone="inverse" />
                        </Show>
                        Run action
                      </Button>
                    </Show>
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
