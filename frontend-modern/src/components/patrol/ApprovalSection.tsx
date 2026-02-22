/**
 * ApprovalSection
 *
 * Shows when investigation has a proposed fix.
 * States: Pending, Expired, Executed, Denied, Failed, Verified, VerificationFailed.
 */

import { Component, Show, createSignal, createResource, createMemo } from 'solid-js';
import { aiIntelligenceStore } from '@/stores/aiIntelligence';
import { notificationStore } from '@/stores/notifications';
import { aiChatStore } from '@/stores/aiChat';
import { hasFeature } from '@/stores/license';
import { AIAPI, type ApprovalRequest, type ApprovalExecutionResult } from '@/api/ai';
import { RemediationStatus } from './RemediationStatus';

interface ApprovalSectionProps {
  findingId: string;
  investigationOutcome?: string;
  findingTitle?: string;
  resourceName?: string;
  resourceType?: string;
  resourceId?: string;
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

  const canAutoFix = createMemo(() => hasFeature('ai_autofix'));

  const handleFixWithAssistant = (approval: ApprovalRequest | null, fix: { description?: string; commands?: string[]; target_host?: string; risk_level?: string; rationale?: string } | null, e: Event) => {
    e.stopPropagation();
    const desc = approval?.context || fix?.description || 'No description available';
    const command = approval?.command || (fix?.commands && fix.commands.length > 0 ? fix.commands[0] : undefined);
    const targetHost = approval?.targetName || fix?.target_host;
    const riskLevel = approval?.riskLevel || fix?.risk_level || 'unknown';
    const rationale = fix?.rationale;

    let prompt = `Patrol investigated a finding and proposed a fix. Please help me execute it.\n\n**Finding:** ${props.findingTitle || 'Unknown finding'} on ${props.resourceName || 'unknown resource'}\n**Proposed fix:** ${desc}`;
    if (command) prompt += `\n**Command:** \`${command}\``;
    if (targetHost) prompt += `\n**Target:** ${targetHost}`;
    prompt += `\n**Risk level:** ${riskLevel}`;
    if (rationale) prompt += `\n**Rationale:** ${rationale}`;
    prompt += `\n\nPlease execute this fix on the target host.`;

    aiChatStore.openWithPrompt(prompt, {
      targetType: props.resourceType,
      targetId: props.resourceId,
      findingId: props.findingId,
    });
  };

  // Load investigation details when outcome indicates a fix was proposed/executed
  const fixRelatedOutcomes = new Set(['fix_queued', 'fix_executed', 'fix_failed', 'fix_verified', 'fix_verification_failed', 'fix_verification_unknown']);
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
    }
  );

  // Determine state
  const isExpired = createMemo(() =>
    !pendingApproval() &&
    props.investigationOutcome === 'fix_queued' &&
    investigation()?.proposed_fix
  );

  const isVerificationUnknown = createMemo(() =>
    props.investigationOutcome === 'fix_verification_unknown'
  );

  const isExecuted = createMemo(() =>
    props.investigationOutcome === 'fix_executed' ||
    props.investigationOutcome === 'fix_verified' ||
    props.investigationOutcome === 'fix_verification_unknown' ||
    executionResult()?.success
  );

  const isFailed = createMemo(() =>
    props.investigationOutcome === 'fix_failed' || props.investigationOutcome === 'fix_verification_failed' || (executionResult() && !executionResult()!.success)
  );

  // Show section only when there's something to display
  const shouldShow = createMemo(() =>
    pendingApproval() || isExpired() || isExecuted() || isFailed() || executionResult()
  );

  const riskBadgeColor = (level?: string) => {
    switch (level) {
      case 'high': case 'critical': return 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300';
      case 'medium': return 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300';
      default: return 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300';
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

  return (
    <Show when={shouldShow()}>
      <div class="mt-3 pt-3 border-t border-slate-100 dark:border-slate-700">
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
                  <span class="text-sm font-medium text-base-content">Fix Available</span>
                  <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${riskBadgeColor(approval.riskLevel)}`}>
                    {approval.riskLevel} risk
                  </span>
                </div>
                <div class="space-y-2 text-sm">
                  <div class="text-muted">{approval.context}</div>
                  <div class="bg-slate-50 dark:bg-slate-800 rounded p-2 font-mono text-xs text-slate-700 dark:text-slate-300 break-all">
                    {approval.command}
                  </div>
                </div>
                <div class="flex items-center gap-2 mt-3 pt-3 border-t border-slate-100 dark:border-slate-700">
                  <Show when={canAutoFix()}>
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
                      class="px-3 py-1.5 bg-slate-100 hover:bg-slate-200 dark:bg-slate-700 dark:hover:bg-slate-600 disabled:opacity-50 text-muted text-xs font-medium rounded"
                    >
                      Deny
                    </button>
                  </Show>
                  <Show when={!canAutoFix()}>
                    <button
                      type="button"
                      onClick={(e) => handleFixWithAssistant(approval, null, e)}
                      class="flex-1 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-xs font-medium rounded flex items-center justify-center gap-1.5"
                    >
                      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                      </svg>
                      Fix with Assistant
                    </button>
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
            return (
              <>
                <div class="flex items-center gap-2 mb-2">
                  <svg class="w-4 h-4 text-amber-600 dark:text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                  <span class="text-sm font-medium text-base-content">Fix Pending Approval</span>
                  <span class={`px-1.5 py-0.5 text-[10px] font-medium rounded ${riskBadgeColor(fix.risk_level)}`}>
                    {fix.risk_level || 'unknown'} risk
                  </span>
                  <span class="px-1.5 py-0.5 text-[10px] font-medium rounded bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300">
                    approval expired
                  </span>
                </div>
                <div class="space-y-2 text-sm">
                  <div class="text-muted">{fix.description}</div>
                  <Show when={fix.commands && fix.commands.length > 0}>
                    <div class="bg-slate-50 dark:bg-slate-800 rounded p-2 font-mono text-xs text-slate-700 dark:text-slate-300 break-all">
                      {fix.commands![0]}
                    </div>
                  </Show>
                  <Show when={fix.target_host}>
                    <div class="text-xs text-muted">Target: {fix.target_host}</div>
                  </Show>
                </div>
                <div class="flex items-center gap-2 mt-3 pt-3 border-t border-slate-100 dark:border-slate-700">
                  <Show when={canAutoFix()}>
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
                  </Show>
                  <Show when={!canAutoFix()}>
                    <button
                      type="button"
                      onClick={(e) => handleFixWithAssistant(null, fix, e)}
                      class="flex-1 px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white text-xs font-medium rounded flex items-center justify-center gap-1.5"
                    >
                      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                      </svg>
                      Fix with Assistant
                    </button>
                  </Show>
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
          <div class={`flex items-center gap-2 ${isVerificationUnknown() ? 'text-amber-700 dark:text-amber-300' : 'text-green-600 dark:text-green-400'}`}>
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
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
                  <div class="bg-slate-50 dark:bg-slate-800 rounded p-2 font-mono text-xs text-slate-700 dark:text-slate-300 break-all">
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
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span class="text-sm font-medium">{props.investigationOutcome === 'fix_verification_failed' ? 'Fix executed but issue persists' : 'Fix execution failed'}</span>
          </div>
          <Show when={investigation()?.proposed_fix}>
            {(fix) => (
              <div class="mt-2 space-y-1 text-sm">
                <div class="text-muted">{fix().description}</div>
                <Show when={fix().commands && fix().commands!.length > 0}>
                  <div class="bg-slate-50 dark:bg-slate-800 rounded p-2 font-mono text-xs text-slate-700 dark:text-slate-300 break-all">
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
