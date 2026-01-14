import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
import { logger } from '@/utils/logger';

// OpenCode API - Simplified AI interface

export interface ChatSession {
  id: string;
  title: string;
  created_at: string;
  updated_at: string;
  message_count: number;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  model?: string;
  tool_calls?: ToolCall[];
}

export interface ToolCall {
  name: string;
  input: string;
  output: string;
  success: boolean;
}

export interface StreamEvent {
  type: 'content' | 'thinking' | 'tool_start' | 'tool_end' | 'tool' | 'done' | 'error';
  data?: any;
}

export interface AIStatus {
  running: boolean;
  engine: string;
}

export class OpenCodeAPI {
  private static baseUrl = '/api/ai';

  // Get AI status
  static async getStatus(): Promise<AIStatus> {
    return apiFetchJSON(`${this.baseUrl}/status`) as Promise<AIStatus>;
  }

  // List all chat sessions
  static async listSessions(): Promise<ChatSession[]> {
    return apiFetchJSON(`${this.baseUrl}/sessions`) as Promise<ChatSession[]>;
  }

  // Create a new session
  static async createSession(): Promise<ChatSession> {
    return apiFetchJSON(`${this.baseUrl}/sessions`, {
      method: 'POST',
    }) as Promise<ChatSession>;
  }

  // Delete a session
  static async deleteSession(sessionId: string): Promise<void> {
    await apiFetch(`${this.baseUrl}/sessions/${sessionId}`, {
      method: 'DELETE',
    });
  }

  // Get messages for a session
  static async getMessages(sessionId: string): Promise<ChatMessage[]> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${sessionId}/messages`) as Promise<ChatMessage[]>;
  }

  // Abort a session
  static async abortSession(sessionId: string): Promise<void> {
    await apiFetch(`${this.baseUrl}/sessions/${sessionId}/abort`, {
      method: 'POST',
    });
  }

  // Stream chat - the main chat interface
  static async chat(
    prompt: string,
    sessionId: string | undefined,
    model: string | undefined,
    onEvent: (event: StreamEvent) => void,
    signal?: AbortSignal
  ): Promise<void> {
    logger.debug('[OpenCode] Starting chat stream', { prompt: prompt.substring(0, 50) });

    const response = await apiFetch(`${this.baseUrl}/chat`, {
      method: 'POST',
      body: JSON.stringify({
        prompt,
        session_id: sessionId,
        model,
      }),
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
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
    let lastEventTime = Date.now();
    const STREAM_TIMEOUT_MS = 300000; // 5 minutes

    try {
      for (;;) {
        if (Date.now() - lastEventTime > STREAM_TIMEOUT_MS) {
          logger.warn('[OpenCode] Stream timeout');
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
        const chunk = decoder.decode(value, { stream: true });
        buffer += chunk;

        // Process SSE messages
        const normalizedBuffer = buffer.replace(/\r\n/g, '\n');
        const messages = normalizedBuffer.split('\n\n');
        buffer = messages.pop() || '';

        for (const message of messages) {
          if (!message.trim() || message.trim().startsWith(':')) continue;

          const dataLines = message.split('\n').filter(line => line.startsWith('data: '));
          for (const line of dataLines) {
            try {
              const jsonStr = line.slice(6);
              if (!jsonStr.trim()) continue;

              const event = JSON.parse(jsonStr) as StreamEvent;
              logger.debug('[OpenCode] Event', { type: event.type });
              onEvent(event);

              if (event.type === 'done' || event.type === 'error') {
                return;
              }
            } catch (e) {
              logger.error('[OpenCode] Failed to parse event', { error: e, line });
            }
          }
        }
      }

      // Process remaining buffer
      if (buffer.trim() && buffer.trim().startsWith('data: ')) {
        try {
          const jsonStr = buffer.slice(6);
          if (jsonStr.trim()) {
            const event = JSON.parse(jsonStr) as StreamEvent;
            onEvent(event);
          }
        } catch {
          logger.warn('[OpenCode] Could not parse remaining buffer');
        }
      }

      // Ensure done event
      onEvent({ type: 'done' });
    } finally {
      reader.releaseLock();
    }
  }
}
