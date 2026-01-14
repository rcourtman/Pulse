import { createSignal, onCleanup } from 'solid-js';
import { OpenCodeAPI, type StreamEvent } from '@/api/opencode';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import type {
  ChatMessage,
  ToolExecution,
  StreamDisplayEvent,
} from '../types';

const generateId = () => Math.random().toString(36).substring(2, 9);

export interface UseChatOptions {
  sessionId?: string;
  model?: string;
}

export function useChat(options: UseChatOptions = {}) {
  // Core state
  const [messages, setMessages] = createSignal<ChatMessage[]>([]);
  const [isLoading, setIsLoading] = createSignal(false);
  const [sessionId, setSessionId] = createSignal(options.sessionId || '');
  const [model, setModel] = createSignal(options.model || '');

  // Abort controller for canceling requests
  let abortControllerRef: AbortController | null = null;

  // Cleanup on unmount
  onCleanup(() => {
    if (abortControllerRef) abortControllerRef.abort();
  });

  // Stop/cancel current request
  const stop = () => {
    if (abortControllerRef) {
      abortControllerRef.abort();
      abortControllerRef = null;
    }
    setMessages((prev) =>
      prev.map((msg) =>
        msg.isStreaming
          ? { ...msg, isStreaming: false, content: msg.content || '(Stopped)' }
          : msg
      )
    );
    setIsLoading(false);
  };

  // Helper to add stream event for chronological display
  const addStreamEvent = (msg: ChatMessage, event: StreamDisplayEvent): ChatMessage => {
    const events = msg.streamEvents || [];
    // For content events, merge consecutive content into one
    if (event.type === 'content' && events.length > 0) {
      const last = events[events.length - 1];
      if (last.type === 'content') {
        return {
          ...msg,
          streamEvents: [
            ...events.slice(0, -1),
            { ...last, content: (last.content || '') + (event.content || '') },
          ],
        };
      }
    }
    return {
      ...msg,
      streamEvents: [...events, event],
    };
  };

  // Process stream events
  const processEvent = (
    assistantId: string,
    event: StreamEvent
  ) => {
    setMessages((prev) =>
      prev.map((msg) => {
        if (msg.id !== assistantId) return msg;

        switch (event.type) {
          case 'content': {
            const content = event.data as string;
            if (!content) return msg;
            const existing = msg.content || '';
            // Add to streamEvents for chronological display
            const updated = addStreamEvent(msg, { type: 'content', content });
            return {
              ...updated,
              content: existing + content,
            };
          }

          case 'thinking': {
            const thinking = event.data as string;
            if (!thinking) return msg;
            // Add thinking to streamEvents
            const updated = addStreamEvent(msg, { type: 'thinking', thinking });
            return {
              ...updated,
              thinking: (msg.thinking || '') + thinking,
            };
          }

          case 'tool_start': {
            const data = event.data as { name: string; input: string };
            return {
              ...msg,
              pendingTools: [...(msg.pendingTools || []), { name: data.name, input: data.input }],
            };
          }

          case 'tool_end': {
            const data = event.data as { name: string; input: string; output: string; success: boolean };
            const pendingTools = msg.pendingTools || [];
            const matchingIndex = pendingTools.findIndex((t) => t.name === data.name);
            const updatedPending = matchingIndex >= 0
              ? [...pendingTools.slice(0, matchingIndex), ...pendingTools.slice(matchingIndex + 1)]
              : pendingTools;

            const newToolCall: ToolExecution = {
              name: data.name,
              input: data.input,
              output: data.output,
              success: data.success,
            };

            // Add tool to streamEvents for chronological display
            const updated = addStreamEvent(msg, { type: 'tool', tool: newToolCall });
            return {
              ...updated,
              pendingTools: updatedPending,
              toolCalls: [...(msg.toolCalls || []), newToolCall],
            };
          }

          case 'done': {
            return { ...msg, isStreaming: false, pendingTools: [] };
          }

          case 'error': {
            const errorMsg = event.data as string;
            return {
              ...msg,
              isStreaming: false,
              pendingTools: [],
              content: `Error: ${errorMsg}`,
            };
          }

          default:
            return msg;
        }
      })
    );
  };

  // Send a message
  const sendMessage = async (prompt: string) => {
    if (!prompt.trim() || isLoading()) return;

    // Add user message
    const userMessage: ChatMessage = {
      id: generateId(),
      role: 'user',
      content: prompt,
      timestamp: new Date(),
    };
    setMessages((prev) => [...prev, userMessage]);
    setIsLoading(true);

    abortControllerRef = new AbortController();

    // Create streaming assistant message
    const assistantId = generateId();
    const streamingMessage: ChatMessage = {
      id: assistantId,
      role: 'assistant',
      content: '',
      timestamp: new Date(),
      isStreaming: true,
      pendingTools: [],
      toolCalls: [],
      streamEvents: [],
    };
    setMessages((prev) => [...prev, streamingMessage]);

    try {
      await OpenCodeAPI.chat(
        prompt,
        sessionId() || undefined,
        model() || undefined,
        (event: StreamEvent) => {
          processEvent(assistantId, event);
        },
        abortControllerRef?.signal
      );
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        logger.debug('[useChat] Request aborted');
        return;
      }
      logger.error('[useChat] Chat failed:', error);
      const errorMessage = error instanceof Error ? error.message : 'Failed to get AI response';
      notificationStore.error(errorMessage);

      setMessages((prev) =>
        prev.map((msg) =>
          msg.id === assistantId
            ? { ...msg, isStreaming: false, content: `Error: ${errorMessage}` }
            : msg
        )
      );
    } finally {
      abortControllerRef = null;
      setIsLoading(false);
    }
  };

  // Clear messages
  const clearMessages = () => {
    setMessages([]);
  };

  // Load session messages
  const loadSession = async (id: string) => {
    try {
      const msgs = await OpenCodeAPI.getMessages(id);
      setMessages(msgs.map(m => ({
        id: m.id,
        role: m.role,
        content: m.content,
        timestamp: new Date(m.timestamp),
        toolCalls: m.tool_calls,
      })));
      setSessionId(id);
    } catch (error) {
      logger.error('[useChat] Failed to load session:', error);
      notificationStore.error('Failed to load session');
    }
  };

  // Create new session
  const newSession = async () => {
    try {
      const session = await OpenCodeAPI.createSession();
      setSessionId(session.id);
      setMessages([]);
      return session;
    } catch (error) {
      logger.error('[useChat] Failed to create session:', error);
      notificationStore.error('Failed to create session');
      return null;
    }
  };

  return {
    messages,
    isLoading,
    sessionId,
    model,
    setModel,
    sendMessage,
    stop,
    clearMessages,
    loadSession,
    newSession,
  };
}
