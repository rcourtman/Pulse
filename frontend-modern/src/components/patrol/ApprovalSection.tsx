/**
 * ApprovalSection
 *
 * Shows when investigation has a proposed fix.
 * States: Pending, Expired, Executed, Denied, Failed, Verified, VerificationFailed.
 */

import CheckIcon from 'lucide-solid/icons/check';
import MessageSquareIcon from 'lucide-solid/icons/message-square';
import { Component, Show, createSignal, createResource, createMemo } from 'solid-js';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { notificationStore } from '@/stores/notifications';
import { aiChatStore } from '@/stores/aiChat';
import { hasFeature } from '@/stores/license';
import { AIAPI, type ApprovalRequest, type ApprovalExecutionResult } from '@/api/ai';
import { Button } from '@/components/shared/Button';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { getApprovalRiskPresentation } from '@/utils/approvalRiskPresentation';
import {
  buildPatrolAssistantFindingHandoff,
  buildPatrolAssistantApprovalBriefingInput,
  buildPatrolAssistantProposedFixBriefingInput,
  type PatrolAssistantProposedFixBriefingSource,
} from '@/features/patrol/patrolInvestigationContextModel';
import { RemediationStatus } from './RemediationStatus';

interface ApprovalSectionProps {
  findingId: string;
  investigationOutcome?: string;
  findingTitle?: string;
  resourceName?: string;
  resourceType?: string;
  resourceId?: string;
}

const APPROVAL_SECTION_BADGE_PROPS = { size: 'xs', shape: 'rounded' } as const;

export const ApprovalSection: Component<ApprovalSectionProps> = (props) => {
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);
  const [executionResult, setExecutionResult] = createSignal<ApprovalExecutionResult | null>(null);

  // Find the pending approval for this finding from the store
  const pendingApproval = createMemo(() => {
    return (
      aiIntelligenceStore.patrolPendingApprovals.find(
        (a: ApprovalRequest) => a.toolId === 'investigation_fix' && a.targetId === props.findingId,
      ) ?? null
    );
  });

  const canAutoFix = createMemo(() => hasFeature('ai_autofix'));

  const proposedFixBriefing = (
    approval: ApprovalRequest | null,
    fix?: PatrolAssistantProposedFixBriefingSource | null,
  ) =>
    buildPatrolAssistantProposedFixBriefingInput(
      fix ||
        (approval
          ? {
              description: approval.context,
              riskLevel: approval.riskLevel,
              targetHost: approval.targetName,
              commandCount: approval.command ? 1 : 0,
            }
          : null),
    );

  const assistantHandoff = (
    approval: ApprovalRequest | null,
    fix?: PatrolAssistantProposedFixBriefingSource | null,
  ) =>
    buildPatrolAssistantFindingHandoff({
      id: props.findingId,
      title: props.findingTitle || 'Patrol finding',
      subject: props.resourceName || 'affected resource',
      description:
        approval?.context ||
        fix?.description ||
        (!approval && props.investigationOutcome === 'fix_queued'
          ? 'The original approval details are no longer available. Recover or regenerate the governed approval before execution.'
          : undefined),
      findingStatus: 'active',
      investigationOutcome: props.investigationOutcome,
      loopState: props.investigationOutcome || 'awaiting_approval',
      resourceId: props.resourceId,
      resourceName: props.resourceName,
      resourceType: props.resourceType,
      pendingApproval: buildPatrolAssistantApprovalBriefingInput(approval),
      proposedFix: proposedFixBriefing(approval, fix),
    });

  const handleFixWithAssistant = (
    approval: ApprovalRequest | null,
    fix: PatrolAssistantProposedFixBriefingSource | null,
    e: Event,
  ) => {
    e.stopPropagation();
    const handoff = assistantHandoff(approval, fix);
    aiChatStore.open(handoff.context);
  };

  const handleDiscussQueuedFix = (e: Event) => {
    e.stopPropagation();
    const handoff = assistantHandoff(null);
    aiChatStore.open(handoff.context);
  };

  // Load investigation details when outcome indicates a fix was proposed/executed
  const fixRelatedOutcomes = new Set([
    'fix_queued',
    'fix_executed',
    'fix_failed',
    'fix_verified',
    'fix_verification_failed',
    'fix_verification_unknown',
  ]);
  const [investigation] = createResource(
    () => ({ findingId: props.findingId, outcome: props.investigationOutcome }),
    async ({ findingId, outcome }) => {
      if (!outcome || !fixRelatedOutcomes.has(outcome)) {
        return null;
      }
      try {
        return await AIAPI.getInvestigation(findingId);
      } catch {
        return null;
      }
    },
  );

  // Determine state
  const isExpired = createMemo(
    () =>
      !pendingApproval() &&
      props.investigationOutcome === 'fix_queued' &&
      investigation()?.proposed_fix,
  );

  const isQueuedWithoutDetails = createMemo(
    () =>
      !pendingApproval() &&
      props.investigationOutcome === 'fix_queued' &&
      !investigation.loading &&
      !investigation()?.proposed_fix,
  );

  const isVerificationUnknown = createMemo(
    () => props.investigationOutcome === 'fix_verification_unknown',
  );

  const isExecuted = createMemo(
    () =>
      props.investigationOutcome === 'fix_executed' ||
      props.investigationOutcome === 'fix_verified' ||
      props.investigationOutcome === 'fix_verification_unknown' ||
      executionResult()?.success,
  );

  const isFailed = createMemo(
    () =>
      props.investigationOutcome === 'fix_failed' ||
      props.investigationOutcome === 'fix_verification_failed' ||
      (executionResult() && !executionResult()!.success),
  );

  // Show section only when there's something to display
  const shouldShow = createMemo(
    () =>
      pendingApproval() ||
      isExpired() ||
      isQueuedWithoutDetails() ||
      isExecuted() ||
      isFailed() ||
      executionResult(),
  );

  const handleApprove = async (approval: ApprovalRequest, e: Event) => {
    e.stopPropagation();
    setActionLoading(approval.id);
    try {
      const result = await aiIntelligenceStore.approveInvestigationFix(approval.id);
      if (result) {
        setExecutionResult(result);
        if (result.success) {
          notificationStore.success('Fix executed successfully');
        } else {
          notificationStore.error(result.error || 'Fix execution failed');
        }
      } else {
        notificationStore.error('Failed to execute fix — no response from server');
      }
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to execute fix');
    } finally {
      setActionLoading(null);
    }
  };

  const handleDeny = async (approval: ApprovalRequest, e: Event) => {
    e.stopPropagation();
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

  const handleReapprove = async (e: Event) => {
    e.stopPropagation();
    setActionLoading('reapprove');
    try {
      const result = await AIAPI.reapproveInvestigationFix(props.findingId);
      const execResult = await aiIntelligenceStore.approveInvestigationFix(result.approval_id);
      if (execResult) {
        setExecutionResult(execResult);
        if (execResult.success) {
          notificationStore.success('Fix executed successfully');
        } else {
          notificationStore.error(execResult.error || 'Fix execution failed');
        }
      } else {
        notificationStore.error('Failed to execute fix — no response from server');
      }
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to execute fix');
    } finally {
      setActionLoading(null);
    }
  };

  const renderRecoveryActions = (assistantLabel: string, onAssistantClick: (e: Event) => void) => (
    <div class="flex items-center gap-2 mt-3 pt-3 border-t border-border-subtle">
      <Show when={canAutoFix()}>
        <Button
          type="button"
          variant="warningSolid"
          size="sm"
          onClick={handleReapprove}
          disabled={actionLoading() === 'reapprove'}
          class="flex-1 gap-1.5"
        >
          <Show when={actionLoading() === 'reapprove'}>
            <LoadingSpinner size="sm" tone="inverse" />
          </Show>
          <Show when={actionLoading() !== 'reapprove'}>
            <CheckIcon class="w-3.5 h-3.5" />
          </Show>
          Re-approve & Execute
        </Button>
      </Show>
      <Show when={!canAutoFix()}>
        <Button
          type="button"
          variant="primary"
          size="sm"
          onClick={onAssistantClick}
          class="flex-1 gap-1.5"
        >
          <MessageSquareIcon class="w-3.5 h-3.5" />
          {assistantLabel}
        </Button>
      </Show>
    </div>
  );

  return (
    <Show when={shouldShow()}>
      <div class="mt-3 pt-3 border-t border-border-subtle">
        {/* Pending approval */}
        <Show when={pendingApproval() && !executionResult()}>
          {(() => {
            const approval = pendingApproval()!;
            const approvalRisk = getApprovalRiskPresentation(approval.riskLevel);
            return (
              <>
                <div class="flex items-center gap-2 mb-2">
                  <svg
                    class="w-4 h-4 text-green-600 dark:text-green-400"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M13 10V3L4 14h7v7l9-11h-7z"
                    />
                  </svg>
                  <span class="text-sm font-medium text-base-content">Fix Available</span>
                  <MetadataBadge {...APPROVAL_SECTION_BADGE_PROPS} tone={approvalRisk.badgeTone}>
                    {approvalRisk.label} risk
                  </MetadataBadge>
                </div>
                <div class="space-y-2 text-sm">
                  <div class="text-muted">{approval.context}</div>
                  <div class="bg-surface-alt rounded p-2 font-mono text-xs text-base-content break-all">
                    {approval.command}
                  </div>
                </div>
                <div class="flex items-center gap-2 mt-3 pt-3 border-t border-border-subtle">
                  <Show when={canAutoFix()}>
                    <Button
                      type="button"
                      variant="success"
                      size="sm"
                      onClick={(e) => handleApprove(approval, e)}
                      disabled={actionLoading() === approval.id}
                      class="flex-1 gap-1.5"
                    >
                      <Show when={actionLoading() === approval.id}>
                        <LoadingSpinner size="sm" tone="inverse" />
                      </Show>
                      <Show when={actionLoading() !== approval.id}>
                        <CheckIcon class="w-3.5 h-3.5" />
                      </Show>
                      Approve & Execute
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={(e) => handleDeny(approval, e)}
                      disabled={actionLoading() === approval.id}
                      class="text-muted"
                    >
                      Deny
                    </Button>
                  </Show>
                  <Show when={!canAutoFix()}>
                    <Button
                      type="button"
                      variant="primary"
                      size="sm"
                      onClick={(e) => handleFixWithAssistant(approval, null, e)}
                      class="flex-1 gap-1.5"
                    >
                      <MessageSquareIcon class="w-3.5 h-3.5" />
                      Fix with Assistant
                    </Button>
                  </Show>
                </div>
              </>
            );
          })()}
        </Show>

        {/* Expired approval - show re-approve */}
        <Show when={isExpired() && !executionResult()}>
          {(() => {
            const fix = investigation()!.proposed_fix!;
            const fixRisk = getApprovalRiskPresentation(fix.risk_level);
            return (
              <>
                <div class="flex items-center gap-2 mb-2">
                  <svg
                    class="w-4 h-4 text-amber-600 dark:text-amber-400"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                    />
                  </svg>
                  <span class="text-sm font-medium text-base-content">Fix Pending Approval</span>
                  <MetadataBadge {...APPROVAL_SECTION_BADGE_PROPS} tone={fixRisk.badgeTone}>
                    {fixRisk.label} risk
                  </MetadataBadge>
                  <MetadataBadge {...APPROVAL_SECTION_BADGE_PROPS} tone="warning">
                    approval expired
                  </MetadataBadge>
                </div>
                <div class="space-y-2 text-sm">
                  <div class="text-muted">{fix.description}</div>
                  <Show when={fix.commands && fix.commands.length > 0}>
                    <div class="bg-surface-alt rounded p-2 font-mono text-xs text-base-content break-all">
                      {fix.commands![0]}
                    </div>
                  </Show>
                  <Show when={fix.target_host}>
                    <div class="text-xs text-muted">Target: {fix.target_host}</div>
                  </Show>
                </div>
                {renderRecoveryActions('Fix with Assistant', (e) =>
                  handleFixWithAssistant(null, fix, e),
                )}
              </>
            );
          })()}
        </Show>

        {/* Queued approval with missing detail payload - keep recovery path visible */}
        <Show when={isQueuedWithoutDetails() && !executionResult()}>
          <>
            <div class="flex items-center gap-2 mb-2">
              <svg
                class="w-4 h-4 text-amber-600 dark:text-amber-400"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <span class="text-sm font-medium text-base-content">Fix Pending Approval</span>
              <MetadataBadge {...APPROVAL_SECTION_BADGE_PROPS} tone="warning">
                details unavailable
              </MetadataBadge>
            </div>
            <div class="space-y-2 text-sm">
              <div class="text-muted">
                Patrol queued a fix for this finding, but the original approval details are no
                longer available.
              </div>
              <div class="text-xs text-muted">
                Regenerate the approval to continue, or rerun the investigation to let Patrol
                rebuild the remediation plan.
              </div>
            </div>
            {renderRecoveryActions('Discuss with Assistant', handleDiscussQueuedFix)}
          </>
        </Show>

        {/* Execution result */}
        <Show when={executionResult()}>
          <RemediationStatus result={executionResult()!} />
        </Show>

        {/* Executed (from backend state, no local result) */}
        <Show when={isExecuted() && !executionResult()}>
          <div
            class={`flex items-center gap-2 ${isVerificationUnknown() ? 'text-amber-700 dark:text-amber-300' : 'text-green-600 dark:text-green-400'}`}
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span class="text-sm font-medium">
              {props.investigationOutcome === 'fix_verified'
                ? 'Fix verified — issue resolved'
                : props.investigationOutcome === 'fix_verification_unknown'
                  ? 'Fix executed — verification inconclusive'
                  : 'Fix executed successfully'}
            </span>
          </div>
          <Show when={investigation()?.proposed_fix}>
            {(fix) => (
              <div class="mt-2 space-y-1 text-sm">
                <div class="text-muted">{fix().description}</div>
                <Show when={fix().commands && fix().commands!.length > 0}>
                  <div class="bg-surface-alt rounded p-2 font-mono text-xs text-base-content break-all">
                    {fix().commands![0]}
                  </div>
                </Show>
                <Show when={fix().target_host}>
                  <div class="text-xs text-muted">Target: {fix().target_host}</div>
                </Show>
                <Show when={fix().rationale}>
                  <div class="text-xs text-muted whitespace-pre-line mt-1">{fix().rationale}</div>
                </Show>
              </div>
            )}
          </Show>
        </Show>

        {/* Failed (from backend state, no local result) */}
        <Show when={isFailed() && !executionResult()}>
          <div class="flex items-center gap-2 text-red-600 dark:text-red-400">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span class="text-sm font-medium">
              {props.investigationOutcome === 'fix_verification_failed'
                ? 'Fix executed but issue persists'
                : 'Fix execution failed'}
            </span>
          </div>
          <Show when={investigation()?.proposed_fix}>
            {(fix) => (
              <div class="mt-2 space-y-1 text-sm">
                <div class="text-muted">{fix().description}</div>
                <Show when={fix().commands && fix().commands!.length > 0}>
                  <div class="bg-surface-alt rounded p-2 font-mono text-xs text-base-content break-all">
                    {fix().commands![0]}
                  </div>
                </Show>
                <Show when={fix().target_host}>
                  <div class="text-xs text-muted">Target: {fix().target_host}</div>
                </Show>
                <Show when={fix().rationale}>
                  <div class="text-xs text-muted whitespace-pre-line mt-1">{fix().rationale}</div>
                </Show>
              </div>
            )}
          </Show>
        </Show>
      </div>
    </Show>
  );
};

export default ApprovalSection;
