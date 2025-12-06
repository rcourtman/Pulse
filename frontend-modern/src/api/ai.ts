import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
import type {
  AISettings,
  AISettingsUpdateRequest,
  AITestResult,
  AIExecuteRequest,
  AIExecuteResponse,
  AIStreamEvent,
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
    console.log('[AI] runCommand request:', request);
    console.log('[AI] runCommand sanitized:', sanitizedRequest);
    console.log('[AI] runCommand body:', body);
    console.log('[AI] runCommand body length:', body.length);
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
    console.log('[AI] Starting alert investigation:', request);

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
    console.log('[AI SSE] Starting streaming request:', request);

    const response = await apiFetch(`${this.baseUrl}/ai/execute/stream`, {
      method: 'POST',
      body: JSON.stringify(request),
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
      },
      signal,
    });

    console.log('[AI SSE] Response status:', response.status, response.headers.get('content-type'));

    if (!response.ok) {
      const text = await response.text();
      console.error('[AI SSE] Request failed:', text);
      throw new Error(text || `Request failed with status ${response.status}`);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      console.error('[AI SSE] No response body');
      throw new Error('No response body');
    }

    const decoder = new TextDecoder();
    let buffer = '';
    let lastEventTime = Date.now();
    let receivedComplete = false;
    let receivedDone = false;

    // Timeout to detect stalled streams (5 minutes - Opus models can take a long time)
    const STREAM_TIMEOUT_MS = 300000;

    console.log('[AI SSE] Starting to read stream...');

    try {
      while (true) {
        // Check for stream timeout
        if (Date.now() - lastEventTime > STREAM_TIMEOUT_MS) {
          console.warn('[AI SSE] Stream timeout - no data for', STREAM_TIMEOUT_MS / 1000, 'seconds');
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
            console.warn('[AI SSE] Read timeout, ending stream');
            break;
          }
          throw e;
        }

        const { done, value } = result;
        if (done) {
          console.log('[AI SSE] Stream ended normally');
          break;
        }

        lastEventTime = Date.now();
        const chunk = decoder.decode(value, { stream: true });

        // Log chunk info only if it's not just a heartbeat
        if (!chunk.includes(': heartbeat')) {
          console.log('[AI SSE] Received chunk:', chunk.length, 'bytes');
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
              console.debug('[AI SSE] Received heartbeat');
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
              console.log('[AI SSE] Parsed event:', data.type, data);

              // Track completion events
              if (data.type === 'complete') {
                receivedComplete = true;
              }
              if (data.type === 'done') {
                receivedDone = true;
              }

              onEvent(data as AIStreamEvent);
            } catch (e) {
              console.error('[AI SSE] Failed to parse event:', e, line);
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
            console.log('[AI SSE] Parsed final buffered event:', data.type);
            onEvent(data as AIStreamEvent);
            if (data.type === 'complete') receivedComplete = true;
            if (data.type === 'done') receivedDone = true;
          }
        } catch (e) {
          console.warn('[AI SSE] Could not parse remaining buffer:', buffer.substring(0, 100));
        }
      }

      // If we ended without receiving a done event, send a synthetic one
      // This ensures the UI properly clears the streaming state
      if (!receivedDone) {
        console.warn('[AI SSE] Stream ended without done event, sending synthetic done');
        onEvent({ type: 'done', data: undefined });
      }

    } finally {
      reader.releaseLock();
      console.log('[AI SSE] Reader released, receivedComplete:', receivedComplete, 'receivedDone:', receivedDone);
    }
  }
}
