import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
import { readAPIErrorMessage } from './responseUtils';
import { logger } from '@/utils/logger';
import type {
  AISettings,
  AISettingsUpdateRequest,
  AITestResult,
  AIExecuteResponse,
  AIStreamEvent,
  AICostSummary,
} from '@/types/ai';
import type {
  AnomaliesResponse,
  LearningStatusResponse,
} from '@/types/aiIntelligence';

export class AIAPI {
  private static baseUrl = '/api';

  // Get AI settings
  static async getSettings(): Promise<AISettings> {
    return apiFetchJSON(`${this.baseUrl}/settings/ai`) as Promise<AISettings>;
  }

  // Update AI settings
  static async updateSettings(settings: AISettingsUpdateRequest): Promise<AISettings> {
    return apiFetchJSON(`${this.baseUrl}/settings/ai/update`, {
      method: 'PUT',
      body: JSON.stringify(settings),
    }) as Promise<AISettings>;
  }

  // Test AI connection
  static async testConnection(): Promise<AITestResult> {
    return apiFetchJSON(`${this.baseUrl}/ai/test`, {
      method: 'POST',
    }) as Promise<AITestResult>;
  }

  // Test a specific provider connection
  static async testProvider(provider: string): Promise<{ success: boolean; message: string; provider: string }> {
    return apiFetchJSON(`${this.baseUrl}/ai/test/${provider}`, {
      method: 'POST',
    }) as Promise<{ success: boolean; message: string; provider: string }>;
  }

  // Get available models from the AI provider
  static async getModels(): Promise<{ models: { id: string; name: string; description?: string; notable?: boolean }[]; error?: string }> {
    return apiFetchJSON(`${this.baseUrl}/ai/models`) as Promise<{ models: { id: string; name: string; description?: string; notable?: boolean }[]; error?: string }>;
  }

  // Get AI cost/usage summary
  static async getCostSummary(days = 30): Promise<AICostSummary> {
    return apiFetchJSON(`${this.baseUrl}/ai/cost/summary?days=${days}`) as Promise<AICostSummary>;
  }

  // Reset AI usage history (admin-only)
  static async resetCostHistory(): Promise<{ ok: boolean; backup_file?: string }> {
    return apiFetchJSON(`${this.baseUrl}/ai/cost/reset`, {
      method: 'POST',
      body: JSON.stringify({}),
    }) as Promise<{ ok: boolean; backup_file?: string }>;
  }

  static async exportCostHistory(days = 30, format: 'json' | 'csv' = 'csv'): Promise<Response> {
    return apiFetch(`${this.baseUrl}/ai/cost/export?days=${days}&format=${format}`, { method: 'GET' });
  }

  // ============================================
  // AI Intelligence API - Patterns, Predictions, Correlations
  // ============================================

  // Get unified findings (alerts + AI findings)
  static async getUnifiedFindings(options?: {
    resourceId?: string;
    source?: string;
    includeResolved?: boolean;
  }): Promise<UnifiedFindingsResponse> {
    const params = new URLSearchParams();
    if (options?.resourceId) params.set('resource_id', options.resourceId);
    if (options?.source) params.set('source', options.source);
    if (options?.includeResolved) params.set('include_resolved', '1');
    const query = params.toString();
    return apiFetchJSON(`${this.baseUrl}/ai/unified/findings${query ? `?${query}` : ''}`) as Promise<UnifiedFindingsResponse>;
  }

  // Get current anomalies (real-time baseline deviation detection)
  // Returns metrics that are currently deviating significantly from learned baselines
  static async getAnomalies(resourceId?: string): Promise<AnomaliesResponse> {
    const params = resourceId ? `?resource_id=${encodeURIComponent(resourceId)}` : '';
    return apiFetchJSON(`${this.baseUrl}/ai/intelligence/anomalies${params}`) as Promise<AnomaliesResponse>;
  }

  // Get learning/baseline status (FREE - no license required)
  // Shows how many resources have been baselined and the overall learning state
  static async getLearningStatus(): Promise<LearningStatusResponse> {
    return apiFetchJSON(`${this.baseUrl}/ai/intelligence/learning`) as Promise<LearningStatusResponse>;
  }

  // Analyze a Kubernetes cluster with AI
  static async analyzeKubernetesCluster(clusterId: string): Promise<AIExecuteResponse> {
    return apiFetchJSON(`${this.baseUrl}/ai/kubernetes/analyze`, {
      method: 'POST',
      body: JSON.stringify({ cluster_id: clusterId }),
    }) as Promise<AIExecuteResponse>;
  }

  // Run a single command (for approved commands)
  static async runCommand(request: {
    command: string;
    target_type: string;
    target_id: string;
    run_on_host: boolean;
    vmid?: string | number;
    target_host?: string; // Explicit host for command routing
  }): Promise<{ output: string; success: boolean; error?: string }> {
    // Ensure run_on_host is explicitly a boolean (not undefined)
    const sanitizedRequest = {
      command: request.command,
      target_type: request.target_type,
      target_id: request.target_id,
      run_on_host: Boolean(request.run_on_host),
      ...(request.vmid ? { vmid: String(request.vmid) } : {}),
      ...(request.target_host ? { target_host: request.target_host } : {}),
    };
    const body = JSON.stringify(sanitizedRequest);
    logger.debug('[AI] runCommand', { request: sanitizedRequest, bodyLength: body.length });
    return apiFetchJSON(`${this.baseUrl}/ai/run-command`, {
      method: 'POST',
      body,
    }) as Promise<{ output: string; success: boolean; error?: string }>;
  }


  // Investigate an alert with AI (one-click investigation)
  static async investigateAlert(
    request: {
      alert_id: string;
      resource_id: string;
      resource_name: string;
      resource_type: string;
      alert_type: string;
      level: string;
      value: number;
      threshold: number;
      message: string;
      duration: string;
      node?: string;
      vmid?: number;
    },
    onEvent: (event: AIStreamEvent) => void,
    signal?: AbortSignal
  ): Promise<void> {
    logger.debug('[AI] Starting alert investigation', request);

    const response = await apiFetch(`${this.baseUrl}/ai/investigate-alert`, {
      method: 'POST',
      body: JSON.stringify(request),
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
      },
      signal,
    });

    if (!response.ok) {
      throw new Error(await readAPIErrorMessage(response, `Request failed with status ${response.status}`));
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error('No response body');
    }

    const decoder = new TextDecoder();
    let buffer = '';
    // 5 minutes timeout - Opus models can take a long time
    const STREAM_TIMEOUT_MS = 300000;
    let lastEventTime = Date.now();

    const readWithTimeout = async (): Promise<ReadableStreamReadResult<Uint8Array>> => {
      let timeoutId: ReturnType<typeof setTimeout> | undefined;
      const readPromise = reader.read();
      const timeoutPromise = new Promise<never>((_, reject) => {
        timeoutId = setTimeout(() => reject(new Error('Read timeout')), STREAM_TIMEOUT_MS);
      });

      try {
        return await Promise.race([readPromise, timeoutPromise]);
      } finally {
        if (timeoutId !== undefined) {
          clearTimeout(timeoutId);
        }
      }
    };

    try {
      for (; ;) {
        if (Date.now() - lastEventTime > STREAM_TIMEOUT_MS) {
          logger.warn('[AI] Alert investigation stream timeout');
          break;
        }

<<<<<<< HEAD
=======
        const readPromise = reader.read();
        let timeoutId: ReturnType<typeof setTimeout> | undefined;
        const timeoutPromise = new Promise<never>((_, reject) => {
          timeoutId = setTimeout(() => reject(new Error('Read timeout')), STREAM_TIMEOUT_MS);
        });

>>>>>>> refactor/parallel-44-circuit-breakers
        let result: ReadableStreamReadResult<Uint8Array>;
        try {
          result = await readWithTimeout();
        } catch (e) {
          if ((e as Error).message === 'Read timeout') break;
          throw e;
        } finally {
          if (timeoutId) {
            clearTimeout(timeoutId);
          }
        }

        const { done, value } = result;
        if (done) break;

        lastEventTime = Date.now();
        buffer += decoder.decode(value, { stream: true });

        const normalizedBuffer = buffer.replace(/\r\n/g, '\n');
        const messages = normalizedBuffer.split('\n\n');
        buffer = messages.pop() || '';

        for (const message of messages) {
          if (!message.trim() || message.trim().startsWith(':')) continue;

          const dataLines = message.split('\n').filter((line) => line.startsWith('data: '));
          for (const line of dataLines) {
            try {
              const jsonStr = line.slice(6);
              if (!jsonStr.trim()) continue;
              const data = JSON.parse(jsonStr);
              onEvent(data as AIStreamEvent);
            } catch (e) {
              logger.error('[AI] Failed to parse investigation event:', e);
            }
          }
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  // Remediation plans
  static async getRemediationPlans(): Promise<RemediationPlansResponse> {
    const data = await apiFetchJSON(`${this.baseUrl}/ai/remediation/plans`) as { plans?: RemediationPlan[]; executions?: unknown[] };
    if (Array.isArray(data?.plans)) {
      return { plans: data.plans };
    }
    if (Array.isArray(data?.executions)) {
      return { plans: [] };
    }
    return { plans: [] };
  }

  static async getRemediationPlan(planId: string): Promise<RemediationPlan> {
    return apiFetchJSON(`${this.baseUrl}/ai/remediation/plan?plan_id=${planId}`) as Promise<RemediationPlan>;
  }

  static async approveRemediationPlan(planId: string): Promise<{ success: boolean; execution?: { id: string } }> {
    return apiFetchJSON(`${this.baseUrl}/ai/remediation/approve`, {
      method: 'POST',
      body: JSON.stringify({ plan_id: planId }),
    }) as Promise<{ success: boolean; execution?: { id: string } }>;
  }

  static async executeRemediationPlan(executionId: string): Promise<RemediationExecutionResult> {
    return apiFetchJSON(`${this.baseUrl}/ai/remediation/execute`, {
      method: 'POST',
      body: JSON.stringify({ execution_id: executionId }),
    }) as Promise<RemediationExecutionResult>;
  }

  static async rollbackRemediationPlan(executionId: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/ai/remediation/rollback`, {
      method: 'POST',
      body: JSON.stringify({ execution_id: executionId }),
    }) as Promise<{ success: boolean }>;
  }

  // Circuit breaker status
  static async getCircuitBreakerStatus(): Promise<CircuitBreakerStatus> {
    return apiFetchJSON(`${this.baseUrl}/ai/circuit/status`) as Promise<CircuitBreakerStatus>;
  }

  // ============================================
  // Investigation Fix Approvals
  // ============================================

  // Get pending approval requests (investigation fixes waiting for user approval)
  static async getPendingApprovals(): Promise<ApprovalRequest[]> {
    const response = await apiFetchJSON(`${this.baseUrl}/ai/approvals`) as { approvals: ApprovalRequest[] };
    return response.approvals || [];
  }

  // Approve and execute an investigation fix
  static async approveInvestigationFix(approvalId: string): Promise<ApprovalExecutionResult> {
    return apiFetchJSON(`${this.baseUrl}/ai/approvals/${approvalId}/approve`, {
      method: 'POST',
    }) as Promise<ApprovalExecutionResult>;
  }

  // Deny an investigation fix
  static async denyInvestigationFix(approvalId: string, reason?: string): Promise<ApprovalRequest> {
    return apiFetchJSON(`${this.baseUrl}/ai/approvals/${approvalId}/deny`, {
      method: 'POST',
      body: JSON.stringify({ reason: reason || 'User declined' }),
    }) as Promise<ApprovalRequest>;
  }

  // Get investigation details for a finding (includes proposed fix)
  static async getInvestigation(findingId: string): Promise<InvestigationSession | null> {
    try {
      return await apiFetchJSON(`${this.baseUrl}/ai/findings/${findingId}/investigation`) as InvestigationSession;
    } catch {
      return null;
    }
  }

  // Re-create an approval for an investigation fix (when original approval expired)
  static async reapproveInvestigationFix(findingId: string): Promise<{ approval_id: string; message: string }> {
    return apiFetchJSON(`${this.baseUrl}/ai/findings/${findingId}/reapprove`, {
      method: 'POST',
    }) as Promise<{ approval_id: string; message: string }>;
  }
}

// ============================================
// Phase 7 Type Definitions
// ============================================

export interface UnifiedFindingRecord {
  id: string;
  source: string;
  severity: string;
  category: string;
  resource_id: string;
  resource_name: string;
  resource_type: string;
  node?: string;
  title: string;
  description: string;
  recommendation?: string;
  evidence?: string;
  alert_id?: string;
  alert_type?: string;
  value?: number;
  threshold?: number;
  is_threshold?: boolean;
  ai_context?: string;
  root_cause_id?: string;
  correlated_ids?: string[];
  remediation_id?: string;
  ai_confidence?: number;
  enhanced_by_ai?: boolean;
  ai_enhanced_at?: string;
  // Investigation fields (snake_case from Go backend)
  investigation_session_id?: string;
  investigation_status?: string;
  investigation_outcome?: string;
  last_investigated_at?: string;
  investigation_attempts?: number;
  loop_state?: string;
  lifecycle?: Array<{
    at: string;
    type: string;
    message?: string;
    from?: string;
    to?: string;
    metadata?: Record<string, string>;
  }>;
  regression_count?: number;
  last_regression_at?: string;
  detected_at: string;
  last_seen_at?: string;
  resolved_at?: string;
  acknowledged_at?: string;
  snoozed_until?: string;
  dismissed_reason?: string;
  user_note?: string;
  suppressed?: boolean;
  times_raised?: number;
  status?: string;
}

export interface UnifiedFindingsResponse {
  findings: UnifiedFindingRecord[];
  count?: number;
  active_count?: number;
}

export interface RemediationPlansResponse {
  plans: RemediationPlan[];
}

export interface RemediationPlan {
  id: string;
  finding_id: string;
  resource_id: string;
  title: string;
  description: string;
  steps: RemediationStep[];
  risk_level: 'low' | 'medium' | 'high';
  status: 'pending' | 'approved' | 'executing' | 'completed' | 'failed' | 'rolled_back';
  created_at: string;
}

export interface RemediationStep {
  order: number;
  action: string;
  command?: string;
  rollback_command?: string;
  risk_level: 'low' | 'medium' | 'high';
}

export interface StepResult {
  step: number;
  success: boolean;
  output?: string;
  error?: string;
  duration_ms: number;
  run_at: string;
}

export interface RemediationExecution {
  id: string;
  plan_id: string;
  status: 'pending' | 'approved' | 'running' | 'completed' | 'failed' | 'rolled_back';
  approved_by?: string;
  approved_at?: string;
  started_at?: string;
  completed_at?: string;
  current_step: number;
  step_results?: StepResult[];
  error?: string;
  rollback_error?: string;
}

// Legacy type for backwards compatibility
export interface RemediationExecutionResult {
  execution_id: string;
  plan_id: string;
  status: 'success' | 'failed' | 'partial';
  steps_completed: number;
  error?: string;
  // Full execution details from backend
  id?: string;
  step_results?: StepResult[];
  started_at?: string;
  completed_at?: string;
}

export interface CircuitBreakerStatus {
  state: 'closed' | 'open' | 'half-open';
  can_patrol: boolean;
  consecutive_failures: number;
  total_successes: number;
  total_failures: number;
}

// ============================================
// Investigation Fix Approval Types
// ============================================

export type ApprovalStatus = 'pending' | 'approved' | 'denied' | 'expired';
export type RiskLevel = 'low' | 'medium' | 'high';

export interface ApprovalRequest {
  id: string;
  executionId?: string;
  toolId: string;  // "investigation_fix" for patrol findings
  command: string;
  targetType: string;
  targetId: string;
  targetName: string;
  context: string;
  riskLevel: RiskLevel;
  status: ApprovalStatus;
  requestedAt: string;
  expiresAt: string;
  decidedAt?: string;
  decidedBy?: string;
  denyReason?: string;
}

export interface ApprovalExecutionResult {
  approved: boolean;
  executed: boolean;
  success: boolean;
  output: string;
  exit_code: number;
  error?: string;
  finding_id: string;
  message: string;
}

// ============================================
// Investigation Session Types
// ============================================

export type InvestigationStatus = 'pending' | 'running' | 'completed' | 'failed' | 'needs_attention';
export type InvestigationOutcome =
  | 'resolved'
  | 'fix_queued'
  | 'fix_executed'
  | 'fix_failed'
  | 'needs_attention'
  | 'cannot_fix'
  | 'timed_out'
  | 'fix_verified'
  | 'fix_verification_failed'
  | 'fix_verification_unknown';

export interface ProposedFix {
  id: string;
  description: string;
  commands?: string[];
  risk_level?: 'low' | 'medium' | 'high' | 'critical';
  destructive: boolean;
  target_host?: string;
  rationale?: string;
}

export interface InvestigationSession {
  id: string;
  finding_id: string;
  session_id: string;
  status: InvestigationStatus;
  started_at: string;
  completed_at?: string;
  turn_count: number;
  outcome?: InvestigationOutcome;
  tools_available?: string[];
  tools_used?: string[];
  evidence_ids?: string[];
  proposed_fix?: ProposedFix;
  approval_id?: string;
  summary?: string;
  error?: string;
}
