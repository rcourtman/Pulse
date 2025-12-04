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
  }): Promise<{ output: string; success: boolean; error?: string }> {
    return apiFetchJSON(`${this.baseUrl}/ai/run-command`, {
      method: 'POST',
      body: JSON.stringify(request),
    }) as Promise<{ output: string; success: boolean; error?: string }>;
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

    console.log('[AI SSE] Starting to read stream...');

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          console.log('[AI SSE] Stream ended');
          break;
        }

        const chunk = decoder.decode(value, { stream: true });
        console.log('[AI SSE] Received chunk:', chunk.length, 'bytes');
        buffer += chunk;

        // Process complete SSE messages
        const lines = buffer.split('\n\n');
        buffer = lines.pop() || ''; // Keep incomplete message in buffer

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6));
              console.log('[AI SSE] Parsed event:', data.type, data);
              onEvent(data as AIStreamEvent);
            } catch (e) {
              console.error('[AI SSE] Failed to parse event:', e, line);
            }
          }
        }
      }
    } finally {
      reader.releaseLock();
      console.log('[AI SSE] Reader released');
    }
  }
}
