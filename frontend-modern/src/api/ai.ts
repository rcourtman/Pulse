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
} from '@/types/ai';

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
  static async getModels(): Promise<{ models: { id: string; name: string; description?: string }[]; error?: string }> {
    return apiFetchJSON(`${this.baseUrl}/ai/models`) as Promise<{ models: { id: string; name: string; description?: string }[]; error?: string }>;
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

  // Run a single command (for approved commands)
  static async runCommand(request: {
    command: string;
    target_type: string;
    target_id: string;
    run_on_host: boolean;
    vmid?: string;
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
      while (true) {
        if (Date.now() - lastEventTime > STREAM_TIMEOUT_MS) {
          console.warn('[AI] Alert investigation stream timeout');
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
              console.error('[AI] Failed to parse investigation event:', e);
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
      while (true) {
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
        } catch (e) {
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
}
