import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import type {
  AISettings,
  AISettingsUpdateRequest,
  AITestResult,
  AIExecuteRequest,
  AIExecuteResponse,
  AIStreamEvent,
  AICostSummary,
  AIChatSession,
  AIChatSessionSummary,
} from '@/types/ai';
import type {
  AnomaliesResponse,
  LearningStatusResponse,
} from '@/types/aiIntelligence';

const toNumber = (value: unknown, fallback = 0) => {
  if (typeof value === 'number' && !Number.isNaN(value)) return value;
  if (typeof value === 'string') {
    const num = Number(value);
    if (!Number.isNaN(num)) return num;
  }
  return fallback;
};

const normalizeForecastResponse = (data: any): ForecastResponse => {
  const raw = data?.forecast ?? data;
  if (!raw) {
    throw new Error('No forecast data available');
  }

  const trendDirection = typeof raw.trend === 'string' ? raw.trend : raw.trend?.direction;
  const trend =
    trendDirection === 'increasing' || trendDirection === 'decreasing' || trendDirection === 'stable'
      ? trendDirection
      : 'stable';

  const currentValue = toNumber(raw.current_value ?? raw.currentValue, 0);
  const predictedValue = toNumber(raw.predicted_value ?? raw.predictedValue, currentValue);
  const startTime = Date.now();
  const endTime = raw.predicted_at ? new Date(raw.predicted_at).getTime() : startTime;
  const points = 12;
  const stepMs = points > 1 ? (endTime - startTime) / (points - 1) : 0;
  const confidence = Math.min(Math.max(toNumber(raw.confidence, 0), 0), 1);
  const baseSpread = Math.max(Math.abs(predictedValue - currentValue) * (1 - confidence + 0.15), Math.abs(currentValue) * 0.05);

  const predictions: ForecastPrediction[] = [];
  for (let i = 0; i < points; i += 1) {
    const ratio = points > 1 ? i / (points - 1) : 1;
    const value = currentValue + (predictedValue - currentValue) * ratio;
    const spread = baseSpread * (0.6 + ratio * 0.8);
    predictions.push({
      timestamp: new Date(startTime + stepMs * i).toISOString(),
      value,
      lower_bound: Math.max(0, value - spread),
      upper_bound: value + spread,
    });
  }

  return {
    resource_id: raw.resource_id ?? raw.resourceId ?? '',
    metric: raw.metric ?? '',
    predictions,
    confidence,
    trend,
    message: raw.description ?? raw.message,
  };
};

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

  // Start OAuth flow for Claude Pro/Max subscription
  // Returns the authorization URL to redirect the user to
  static async startOAuth(): Promise<{ auth_url: string; state: string }> {
    return apiFetchJSON(`${this.baseUrl}/ai/oauth/start`, {
      method: 'POST',
    }) as Promise<{ auth_url: string; state: string }>;
  }

  // Exchange manually-pasted authorization code for tokens
  static async exchangeOAuthCode(code: string, state: string): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON(`${this.baseUrl}/ai/oauth/exchange`, {
      method: 'POST',
      body: JSON.stringify({ code, state }),
    }) as Promise<{ success: boolean; message: string }>;
  }

  // Disconnect OAuth and clear tokens
  static async disconnectOAuth(): Promise<{ success: boolean; message: string }> {
    return apiFetchJSON(`${this.baseUrl}/ai/oauth/disconnect`, {
      method: 'POST',
    }) as Promise<{ success: boolean; message: string }>;
  }


  // Execute an AI prompt
  static async execute(request: AIExecuteRequest): Promise<AIExecuteResponse> {
    return apiFetchJSON(`${this.baseUrl}/ai/execute`, {
      method: 'POST',
      body: JSON.stringify(request),
    }) as Promise<AIExecuteResponse>;
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

  // Execute an AI prompt with streaming
  // Returns an abort function to cancel the request
  static async executeStream(
    request: AIExecuteRequest,
    onEvent: (event: AIStreamEvent) => void,
    signal?: AbortSignal
  ): Promise<void> {
    logger.debug('[AI SSE] Starting streaming request', request);

    const response = await apiFetch(`${this.baseUrl}/ai/execute/stream`, {
      method: 'POST',
      body: JSON.stringify(request),
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
      },
      signal,
    });

    logger.debug('[AI SSE] Response status', { status: response.status, contentType: response.headers.get('content-type') });

    if (!response.ok) {
      const text = await response.text();
      logger.error('[AI SSE] Request failed', text);
      throw new Error(text || `Request failed with status ${response.status}`);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      logger.error('[AI SSE] No response body');
      throw new Error('No response body');
    }

    const decoder = new TextDecoder();
    let buffer = '';
    let lastEventTime = Date.now();
    let receivedComplete = false;
    let receivedDone = false;

    // Timeout to detect stalled streams (5 minutes - Opus models can take a long time)
    const STREAM_TIMEOUT_MS = 300000;

    logger.debug('[AI SSE] Starting to read stream');

    try {
      for (; ;) {
        // Check for stream timeout
        if (Date.now() - lastEventTime > STREAM_TIMEOUT_MS) {
          logger.warn('[AI SSE] Stream timeout', { seconds: STREAM_TIMEOUT_MS / 1000 });
          break;
        }

        // Create a promise with timeout for the read operation
        const readPromise = reader.read();
        const timeoutPromise = new Promise<never>((_, reject) => {
          setTimeout(() => reject(new Error('Read timeout')), STREAM_TIMEOUT_MS);
        });

        let result: ReadableStreamReadResult<Uint8Array>;
        try {
          result = await Promise.race([readPromise, timeoutPromise]);
        } catch (e) {
          if ((e as Error).message === 'Read timeout') {
            logger.warn('[AI SSE] Read timeout, ending stream');
            break;
          }
          throw e;
        }

        const { done, value } = result;
        if (done) {
          logger.debug('[AI SSE] Stream ended normally');
          break;
        }

        lastEventTime = Date.now();
        const chunk = decoder.decode(value, { stream: true });

        // Log chunk info only if it's not just a heartbeat
        if (!chunk.includes(': heartbeat')) {
          logger.debug('[AI SSE] Received chunk', { bytes: chunk.length });
        }

        buffer += chunk;

        // Process complete SSE messages (separated by double newlines)
        // Handle both \n\n and \r\n\r\n for cross-platform compatibility
        const normalizedBuffer = buffer.replace(/\r\n/g, '\n');
        const messages = normalizedBuffer.split('\n\n');
        buffer = messages.pop() || ''; // Keep incomplete message in buffer

        for (const message of messages) {
          // Skip empty messages and heartbeat comments
          if (!message.trim() || message.trim().startsWith(':')) {
            if (message.includes('heartbeat')) {
              logger.debug('[AI SSE] Received heartbeat');
            }
            continue;
          }

          // Parse SSE message (can have multiple lines, look for data: prefix)
          const dataLines = message.split('\n').filter(line => line.startsWith('data: '));
          for (const line of dataLines) {
            try {
              const jsonStr = line.slice(6); // Remove 'data: ' prefix
              if (!jsonStr.trim()) continue;

              const data = JSON.parse(jsonStr);
              logger.debug('[AI SSE] Parsed event', { type: data.type, data });

              // Track completion events
              if (data.type === 'complete') {
                receivedComplete = true;
              }
              if (data.type === 'done') {
                receivedDone = true;
              }

              onEvent(data as AIStreamEvent);
            } catch (e) {
              logger.error('[AI SSE] Failed to parse event', { error: e, line });
            }
          }
        }
      }

      // Process any remaining buffer content
      if (buffer.trim() && buffer.trim().startsWith('data: ')) {
        try {
          const jsonStr = buffer.slice(6);
          if (jsonStr.trim()) {
            const data = JSON.parse(jsonStr);
            logger.debug('[AI SSE] Parsed final buffered event', { type: data.type });
            onEvent(data as AIStreamEvent);
            if (data.type === 'complete') receivedComplete = true;
            if (data.type === 'done') receivedDone = true;
          }
        } catch {
          logger.warn('[AI SSE] Could not parse remaining buffer', { preview: buffer.substring(0, 100) });
        }
      }

      // If we ended without receiving a done event, send a synthetic one
      // This ensures the UI properly clears the streaming state
      if (!receivedDone) {
        logger.warn('[AI SSE] Stream ended without done event, sending synthetic done');
        onEvent({ type: 'done', data: undefined });
      }

    } finally {
      reader.releaseLock();
      logger.debug('[AI SSE] Reader released', { receivedComplete, receivedDone });
    }
  }

  // ============================================
  // AI Chat Sessions API - sync across devices
  // ============================================

  // List all chat sessions for the current user
  static async listChatSessions(): Promise<AIChatSessionSummary[]> {
    return apiFetchJSON(`${this.baseUrl}/ai/chat/sessions`) as Promise<AIChatSessionSummary[]>;
  }

  // Get a specific chat session by ID
  static async getChatSession(sessionId: string): Promise<AIChatSession> {
    const response = await apiFetchJSON(`${this.baseUrl}/ai/chat/sessions/${sessionId}`);
    // Convert server format to client format (snake_case to camelCase)
    return this.deserializeChatSession(response);
  }

  // Save a chat session (create or update)
  static async saveChatSession(session: AIChatSession): Promise<AIChatSession> {
    const response = await apiFetchJSON(`${this.baseUrl}/ai/chat/sessions/${session.id}`, {
      method: 'PUT',
      body: JSON.stringify(this.serializeChatSession(session)),
    });
    return this.deserializeChatSession(response);
  }

  // Delete a chat session
  static async deleteChatSession(sessionId: string): Promise<void> {
    await apiFetch(`${this.baseUrl}/ai/chat/sessions/${sessionId}`, {
      method: 'DELETE',
    });
  }

  // Helper to convert server format (snake_case) to client format (camelCase)
  private static deserializeChatSession(data: any): AIChatSession {
    return {
      id: data.id,
      username: data.username || '',
      title: data.title || '',
      createdAt: new Date(data.created_at || data.createdAt),
      updatedAt: new Date(data.updated_at || data.updatedAt),
      messages: (data.messages || []).map((m: any) => ({
        id: m.id,
        role: m.role,
        content: m.content,
        timestamp: new Date(m.timestamp),
        model: m.model,
        tokens: m.tokens,
        toolCalls: m.tool_calls || m.toolCalls,
      })),
    };
  }

  // Helper to convert client format (camelCase) to server format (snake_case)
  private static serializeChatSession(session: AIChatSession): any {
    return {
      id: session.id,
      title: session.title,
      messages: session.messages.map((m) => ({
        id: m.id,
        role: m.role,
        content: m.content,
        timestamp: m.timestamp.toISOString(),
        model: m.model,
        tokens: m.tokens,
        tool_calls: m.toolCalls,
      })),
    };
  }

  // ============================================
  // Phase 7: Event-Driven Intelligence API
  // ============================================

  // Get forecast for a metric
  static async getForecast(options: { resourceId: string; metric: string; horizonHours?: number }): Promise<ForecastResponse> {
    const params = new URLSearchParams();
    params.set('resource_id', options.resourceId);
    params.set('metric', options.metric);
    if (options.horizonHours) params.set('horizon_hours', String(options.horizonHours));
    const data = await apiFetchJSON(`${this.baseUrl}/ai/forecast?${params.toString()}`);
    return normalizeForecastResponse(data);
  }

  // Get forecast overview for all resources sorted by urgency
  static async getForecastOverview(params: {
    metric: string;
    horizonHours?: number;
    threshold?: number;
  }): Promise<ForecastOverviewResponse> {
    const urlParams = new URLSearchParams();
    urlParams.set('metric', params.metric);
    if (params.horizonHours) urlParams.set('horizon_hours', String(params.horizonHours));
    if (params.threshold) urlParams.set('threshold', String(params.threshold));
    return apiFetchJSON(`${this.baseUrl}/ai/forecasts/overview?${urlParams.toString()}`) as Promise<ForecastOverviewResponse>;
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

export interface ForecastResponse {
  resource_id: string;
  metric: string;
  predictions: ForecastPrediction[];
  confidence: number;
  trend: 'increasing' | 'decreasing' | 'stable';
  message?: string;
}

export interface ForecastPrediction {
  timestamp: string;
  value: number;
  lower_bound: number;
  upper_bound: number;
}

export interface ForecastOverviewItem {
  resource_id: string;
  resource_name: string;
  resource_type: 'node' | 'vm' | 'lxc';
  metric: string;
  current_value: number;
  predicted_value: number;
  time_to_threshold: number | null;  // seconds, null if won't breach
  confidence: number;
  trend: 'increasing' | 'decreasing' | 'stable' | 'volatile';
}

export interface ForecastOverviewResponse {
  forecasts: ForecastOverviewItem[];
  metric: string;
  threshold: number;
  horizon_hours: number;
  error?: string;
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
