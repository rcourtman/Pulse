/**
 * ApprovalSection
 *
 * Shows when investigation has a proposed fix.
 * States: Pending, Expired, Executed, Denied, Failed.
 */

import { Component, Show, createSignal, createResource, createMemo } from 'solid-js';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { notificationStore } from '@/stores/notifications';
import { AIAPI, type ApprovalRequest, type ApprovalExecutionResult } from '@/api/ai';
import { RemediationStatus } from './RemediationStatus';

interface ApprovalSectionProps {
  findingId: string;
  investigationOutcome?: string;
}

export const ApprovalSection: Component<ApprovalSectionProps> = (props) => {
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);
  const [executionResult, setExecutionResult] = createSignal<ApprovalExecutionResult | null>(null);

  // Find the pending approval for this finding from the store
  const pendingApproval = createMemo(() => {
    return aiIntelligenceStore.pendingApprovals.find(
      (a: ApprovalRequest) => a.toolId === 'investigation_fix' && a.targetId === props.findingId && a.status === 'pending'
    ) ?? null;
  });

  // Load investigation details for expired approvals (to show proposed fix info)
  const [investigation] = createResource(
    () => props.findingId,
    async (findingId) => {
      // Only load if outcome indicates a fix was queued but no pending approval
      if (props.investigationOutcome !== 'fix_queued' && props.investigationOutcome !== 'fix_executed' && props.investigationOutcome !== 'fix_failed') {
        return null;
      }
      try {
        return await AIAPI.getInvestigation(findingId);
      } catch {
        return null;
      }
    }
  );

  // Determine state
  const isExpired = createMemo(() =>
    !pendingApproval() &&
    props.investigationOutcome === 'fix_queued' &&
    investigation()?.proposed_fix
  );

  const isExecuted = createMemo(() =>
    props.investigationOutcome === 'fix_executed' || executionResult()?.success
  );

  const isFailed = createMemo(() =>
    props.investigationOutcome === 'fix_failed' || (executionResult() && !executionResult()!.success)
  );

  // Show section only when there's something to display
  const shouldShow = createMemo(() =>
    pendingApproval() || isExpired() || isExecuted() || isFailed() || executionResult()
  );

  const riskBadgeColor = (level?: string) => {
    switch (level) {
      case 'high': case 'critical': return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300';
      case 'medium': return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300';
      default: return 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300';
    }
  };

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
      }
    } catch (err) {
      notificationStore.error((err as Error).message || 'Failed to execute fix');
    } finally {
      setActionLoading(null);
    }
  };

  return (
    <Show when={shouldShow()}>
      <div class="mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
        {/* Pending approval */}
        <Show when={pendingApproval() && !executionResult()}>
          {(() => {
            const approval = pendingApproval()!;
            return (
              <>
                <div class="flex items-center gap-2 mb-2">
                  <svg class="w-4 h-4 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                  </svg>
                  <span class="text-sm font-medium text-gray-900 dark:text-gray-100">Fix Available</span>
                  <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${riskBadgeColor(approval.riskLevel)}`}>
                    {approval.riskLevel} risk
                  </span>
                </div>
                <div class="space-y-2 text-sm">
                  <div class="text-gray-600 dark:text-gray-400">{approval.context}</div>
                  <div class="bg-gray-50 dark:bg-gray-800 rounded p-2 font-mono text-xs text-gray-700 dark:text-gray-300 break-all">
                    {approval.command}
                  </div>
                </div>
                <div class="flex items-center gap-2 mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
                  <button
                    type="button"
                    onClick={(e) => handleApprove(approval, e)}
                    disabled={actionLoading() === approval.id}
                    class="flex-1 px-3 py-1.5 bg-green-600 hover:bg-green-700 disabled:bg-green-400 text-white text-xs font-medium rounded flex items-center justify-center gap-1.5"
                  >
                    <Show when={actionLoading() === approval.id}>
                      <span class="h-3 w-3 border-2 border-white border-t-transparent rounded-full animate-spin" />
                    </Show>
                    <Show when={actionLoading() !== approval.id}>
                      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                      </svg>
                    </Show>
                    Approve & Execute
                  </button>
                  <button
                    type="button"
                    onClick={(e) => handleDeny(approval, e)}
                    disabled={actionLoading() === approval.id}
                    class="px-3 py-1.5 bg-gray-100 hover:bg-gray-200 dark:bg-gray-700 dark:hover:bg-gray-600 disabled:opacity-50 text-gray-600 dark:text-gray-400 text-xs font-medium rounded"
                  >
                    Deny
                  </button>
                </div>
              </>
            );
          })()}
        </Show>

        {/* Expired approval - show re-approve */}
        <Show when={isExpired() && !executionResult()}>
          {(() => {
            const fix = investigation()!.proposed_fix!;
            return (
              <>
                <div class="flex items-center gap-2 mb-2">
                  <svg class="w-4 h-4 text-amber-600 dark:text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                  <span class="text-sm font-medium text-gray-900 dark:text-gray-100">Fix Pending Approval</span>
                  <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${riskBadgeColor(fix.risk_level)}`}>
                    {fix.risk_level || 'unknown'} risk
                  </span>
                  <span class="px-1.5 py-0.5 text-[10px] font-medium rounded bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
                    approval expired
                  </span>
                </div>
                <div class="space-y-2 text-sm">
                  <div class="text-gray-600 dark:text-gray-400">{fix.description}</div>
                  <Show when={fix.commands && fix.commands.length > 0}>
                    <div class="bg-gray-50 dark:bg-gray-800 rounded p-2 font-mono text-xs text-gray-700 dark:text-gray-300 break-all">
                      {fix.commands![0]}
                    </div>
                  </Show>
                  <Show when={fix.target_host}>
                    <div class="text-xs text-gray-500 dark:text-gray-400">Target: {fix.target_host}</div>
                  </Show>
                </div>
                <div class="flex items-center gap-2 mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
                  <button
                    type="button"
                    onClick={handleReapprove}
                    disabled={actionLoading() === 'reapprove'}
                    class="flex-1 px-3 py-1.5 bg-amber-600 hover:bg-amber-700 disabled:bg-amber-400 text-white text-xs font-medium rounded flex items-center justify-center gap-1.5"
                  >
                    <Show when={actionLoading() === 'reapprove'}>
                      <span class="h-3 w-3 border-2 border-white border-t-transparent rounded-full animate-spin" />
                    </Show>
                    <Show when={actionLoading() !== 'reapprove'}>
                      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                      </svg>
                    </Show>
                    Re-approve & Execute
                  </button>
                </div>
              </>
            );
          })()}
        </Show>

        {/* Execution result */}
        <Show when={executionResult()}>
          <RemediationStatus result={executionResult()!} />
        </Show>

        {/* Executed (from backend state, no local result) */}
        <Show when={isExecuted() && !executionResult()}>
          <div class="flex items-center gap-2 text-green-600 dark:text-green-400">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span class="text-sm font-medium">Fix executed successfully</span>
          </div>
        </Show>

        {/* Failed (from backend state, no local result) */}
        <Show when={isFailed() && !executionResult()}>
          <div class="flex items-center gap-2 text-red-600 dark:text-red-400">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span class="text-sm font-medium">Fix execution failed</span>
          </div>
        </Show>
      </div>
    </Show>
  );
};

export default ApprovalSection;
