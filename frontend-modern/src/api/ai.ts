import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
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
      const text = await response.text();
      throw new Error(text || `Request failed with status ${response.status}`);
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

    try {
      for (; ;) {
        if (Date.now() - lastEventTime > STREAM_TIMEOUT_MS) {
          logger.warn('[AI] Alert investigation stream timeout');
          break;
        }

        const readPromise = reader.read();
        const timeoutPromise = new Promise<never>((_, reject) => {
          setTimeout(() => reject(new Error('Read timeout')), STREAM_TIMEOUT_MS);
        });

        let result: ReadableStreamReadResult<Uint8Array>;
        try {
          result = await Promise.race([readPromise, timeoutPromise]);
        } catch (e) {
          if ((e as Error).message === 'Read timeout') break;
          throw e;
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

  static async approveRemediationPlan(planId: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/ai/remediation/approve`, {
      method: 'POST',
      body: JSON.stringify({ plan_id: planId }),
    }) as Promise<{ success: boolean }>;
  }

  static async executeRemediationPlan(planId: string): Promise<RemediationExecutionResult> {
    return apiFetchJSON(`${this.baseUrl}/ai/remediation/execute`, {
      method: 'POST',
      body: JSON.stringify({ plan_id: planId }),
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

export interface RemediationExecutionResult {
  execution_id: string;
  plan_id: string;
  status: 'success' | 'failed' | 'partial';
  steps_completed: number;
  error?: string;
}

export interface CircuitBreakerStatus {
  state: 'closed' | 'open' | 'half-open';
  can_patrol: boolean;
  consecutive_failures: number;
  total_successes: number;
  total_failures: number;
}
