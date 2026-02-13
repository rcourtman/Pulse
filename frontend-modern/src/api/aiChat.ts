import { apiFetchJSON, apiFetch } from '@/utils/apiClient';
import { readAPIErrorMessage } from './responseUtils';
import { logger } from '@/utils/logger';
import type {
  AIChatStreamEvent,
} from './generated/aiChatEvents';

// AI Chat API - Simplified AI interface

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

export type StreamEvent = AIChatStreamEvent;

export interface AIStatus {
  running: boolean;
  engine: string;
}

// AI Agent (build, code, etc.) with specific permissions and model
export interface Agent {
  name: string;
  description?: string;
  mode: 'subagent' | 'primary' | 'all';
  native?: boolean;
  hidden?: boolean;
  color?: string;
  model?: {
    providerID: string;
    modelID: string;
  };
}

// File change from a session
export interface FileChange {
  path: string;
  status: 'added' | 'modified' | 'deleted';
  added: number;
  removed: number;
}

// Session diff showing all file changes
export interface SessionDiff {
  files: FileChange[];
  summary?: string;
}

export class AIChatAPI {
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

  // Approve a pending command
  static async approveCommand(approvalId: string): Promise<{ approved: boolean; message: string }> {
    return apiFetchJSON(`${this.baseUrl}/approvals/${approvalId}/approve`, {
      method: 'POST',
    }) as Promise<{ approved: boolean; message: string }>;
  }

  // Deny a pending command
  static async denyCommand(approvalId: string, reason?: string): Promise<{ denied: boolean; message: string }> {
    return apiFetchJSON(`${this.baseUrl}/approvals/${approvalId}/deny`, {
      method: 'POST',
      body: JSON.stringify({ reason: reason || 'User skipped' }),
    }) as Promise<{ denied: boolean; message: string }>;
  }

  // Answer a pending question from the AI chat
  static async answerQuestion(questionId: string, answers: Array<{ id: string; value: string }>): Promise<void> {
    await apiFetch(`${this.baseUrl}/question/${questionId}/answer`, {
      method: 'POST',
      body: JSON.stringify({ answers }),
    });
  }

  // ============================================
  // AI Chat Extended Features
  // ============================================

  // List available agents (build, code, etc.)
  static async listAgents(): Promise<Agent[]> {
    return apiFetchJSON(`${this.baseUrl}/agents`) as Promise<Agent[]>;
  }

  // Summarize a session (compress context when nearing limits)
  static async summarizeSession(sessionId: string): Promise<{ success: boolean; message?: string }> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${sessionId}/summarize`, {
      method: 'POST',
    }) as Promise<{ success: boolean; message?: string }>;
  }

  // Get file changes/diff for a session
  static async getSessionDiff(sessionId: string): Promise<SessionDiff> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${sessionId}/diff`) as Promise<SessionDiff>;
  }

  // Fork a session (create a branch point)
  static async forkSession(sessionId: string): Promise<ChatSession> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${sessionId}/fork`, {
      method: 'POST',
    }) as Promise<ChatSession>;
  }

  // Revert session changes
  static async revertSession(sessionId: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${sessionId}/revert`, {
      method: 'POST',
    }) as Promise<{ success: boolean }>;
  }

  // Unrevert session changes (redo)
  static async unrevertSession(sessionId: string): Promise<{ success: boolean }> {
    return apiFetchJSON(`${this.baseUrl}/sessions/${sessionId}/unrevert`, {
      method: 'POST',
    }) as Promise<{ success: boolean }>;
  }

  // Stream chat - the main chat interface
  static async chat(
    prompt: string,
    sessionId: string | undefined,
    model: string | undefined,
    onEvent: (event: StreamEvent) => void,
    signal?: AbortSignal,
    mentions?: Array<{ id: string; name: string; type: string; node?: string }>,
    findingId?: string
  ): Promise<void> {
    logger.debug('[AI Chat] Starting chat stream', { prompt: prompt.substring(0, 50) });

    const body: Record<string, unknown> = {
      prompt,
      session_id: sessionId,
      model,
    };
    if (mentions && mentions.length > 0) {
      body.mentions = mentions;
    }
    if (findingId) {
      body.finding_id = findingId;
    }

    const response = await apiFetch(`${this.baseUrl}/chat`, {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
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
    let lastEventTime = Date.now();
    const STREAM_TIMEOUT_MS = 300000; // 5 minutes

    try {
      for (; ;) {
        if (Date.now() - lastEventTime > STREAM_TIMEOUT_MS) {
          logger.warn('[AI Chat] Stream timeout');
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
              logger.debug('[AI Chat] Event', { type: event.type });
              onEvent(event);

              if (event.type === 'done' || event.type === 'error') {
                return;
              }
            } catch (e) {
              logger.error('[AI Chat] Failed to parse event', { error: e, line });
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
          logger.warn('[AI Chat] Could not parse remaining buffer');
        }
      }

      // Ensure done event
      onEvent({ type: 'done' });
    } finally {
      reader.releaseLock();
    }
  }
}
