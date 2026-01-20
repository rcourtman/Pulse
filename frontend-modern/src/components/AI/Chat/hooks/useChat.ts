import { createSignal, onCleanup } from 'solid-js';
import { AIChatAPI, type StreamEvent } from '@/api/aiChat';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import type {
  ChatMessage,
  ToolExecution,
  StreamDisplayEvent,
  PendingQuestion,
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

    // For thinking events, merge consecutive thinking into one
    if (event.type === 'thinking' && events.length > 0) {
      const last = events[events.length - 1];
      if (last.type === 'thinking') {
        return {
          ...msg,
          streamEvents: [
            ...events.slice(0, -1),
            { ...last, thinking: (last.thinking || '') + (event.thinking || '') },
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

        try {
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
            const data = (event.data || {}) as { name?: string; input?: string };

            // Skip tool_start for "question" - these are handled by the question event type
            if (data.name === 'question' || data.name === 'Question') {
              return msg;
            }

            const toolId = generateId(); // Unique ID to track this tool
            const pendingTool = { name: data.name || 'unknown', input: data.input || '{}' };

            // Add to streamEvents in chronological position
            const updated = addStreamEvent(msg, {
              type: 'pending_tool',
              pendingTool,
              toolId,
            });

            return {
              ...updated,
              pendingTools: [...(msg.pendingTools || []), { ...pendingTool, id: toolId } as any],
            };
          }

          case 'tool_end': {
            const data = event.data as { name: string; input: string; output: string; success: boolean };
            const pendingTools = msg.pendingTools || [];
            const events = msg.streamEvents || [];

            // Normalize tool name for matching - strip MCP server prefix (pulse_) which may be doubled
            const normalizeToolName = (name: string) => (name || '').replace(/^(pulse_)+/, '');
            const normalizedEndName = normalizeToolName(data.name || '');

            // Find the matching pending tool (by normalized name)
            const matchingPendingIndex = pendingTools.findIndex(
              (t) => normalizeToolName(t.name) === normalizedEndName
            );
            const updatedPending = matchingPendingIndex >= 0
              ? [...pendingTools.slice(0, matchingPendingIndex), ...pendingTools.slice(matchingPendingIndex + 1)]
              : pendingTools;

            const newToolCall: ToolExecution = {
              name: data.name || 'unknown',
              input: data.input || '{}',
              output: data.output || '',
              success: data.success ?? true,
            };

            // Check if there's an approval card for this tool
            // If so, we need to remove both the pending_tool AND the approval,
            // then add the completed tool at the end (since execution happened AFTER approval)
            const hasApproval = events.some(
              (evt) => evt.type === 'approval' && normalizeToolName(evt.approval?.toolName || '') === normalizedEndName
            );

            let updatedEvents: typeof events;
            if (hasApproval) {
              // Remove pending_tool and approval, add completed tool at end
              updatedEvents = events.filter((evt) => {
                if (evt.type === 'pending_tool' && normalizeToolName(evt.pendingTool?.name || '') === normalizedEndName) {
                  return false;
                }
                if (evt.type === 'approval' && normalizeToolName(evt.approval?.toolName || '') === normalizedEndName) {
                  return false;
                }
                return true;
              });
              updatedEvents.push({ type: 'tool', tool: newToolCall });
            } else {
              // No approval - just replace pending_tool in place
              updatedEvents = [...events];
              for (let i = events.length - 1; i >= 0; i--) {
                const evt = events[i];
                if (evt.type === 'pending_tool' && normalizeToolName(evt.pendingTool?.name || '') === normalizedEndName) {
                  updatedEvents[i] = { type: 'tool', tool: newToolCall };
                  break;
                }
              }
            }

            // Also remove from pendingApprovals if present
            const updatedApprovals = (msg.pendingApprovals || []).filter(
              (a) => normalizeToolName(a.toolName || '') !== normalizedEndName
            );

            return {
              ...msg,
              streamEvents: updatedEvents,
              pendingTools: updatedPending,
              pendingApprovals: updatedApprovals,
              toolCalls: [...(msg.toolCalls || []), newToolCall],
            };
          }

          case 'approval_needed': {
            const data = event.data as {
              command: string;
              tool_id: string;
              tool_name: string;
              run_on_host: boolean;
              target_host?: string;
              approval_id?: string;
            };

            const approval = {
              command: data.command,
              toolId: data.tool_id,
              toolName: data.tool_name,
              runOnHost: data.run_on_host,
              targetHost: data.target_host,
              isExecuting: false,
              approvalId: data.approval_id,
            };

            // Add to streamEvents for chronological display
            const updated = addStreamEvent(msg, { type: 'approval', approval });

            return {
              ...updated,
              pendingApprovals: [...(msg.pendingApprovals || []), approval],
            };
          }

          case 'question': {
            const data = event.data as {
              question_id: string;
              session_id: string;
              questions: Array<{
                id: string;
                type: 'text' | 'select';
                question: string;
                options?: Array<{ label: string; value: string }>;
              }>;
            };

            const pendingQuestion: PendingQuestion = {
              questionId: data.question_id,
              sessionId: data.session_id,
              questions: data.questions,
              isAnswering: false,
            };

            // Add to streamEvents for chronological display
            const updated = addStreamEvent(msg, { type: 'question', question: pendingQuestion });

            return {
              ...updated,
              pendingQuestions: [...(msg.pendingQuestions || []), pendingQuestion],
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
        } catch (err) {
          logger.error('[useChat] Error processing event', { event, error: err });
          return msg; // Return unchanged message on error
        }
      })
    );
  };

  // Send a message - allows sending mid-stream (aborts current response like Pulse AI TUI)
  const sendMessage = async (prompt: string) => {
    if (!prompt.trim()) return;

    // If already streaming, abort the current request first
    if (isLoading() && abortControllerRef) {
      logger.debug('[useChat] Aborting current stream to send new message');
      abortControllerRef.abort();
      abortControllerRef = null;
      // Mark any streaming messages as stopped
      setMessages((prev) =>
        prev.map((msg) =>
          msg.isStreaming
            ? { ...msg, isStreaming: false, pendingTools: [] }
            : msg
        )
      );
    }

    // Ensure we have a session for conversation continuity
    // Without this, every message creates a new session and loses context
    let currentSessionId = sessionId();
    if (!currentSessionId) {
      try {
        const session = await AIChatAPI.createSession();
        currentSessionId = session.id;
        setSessionId(currentSessionId);
        logger.debug('[useChat] Created new session', { sessionId: currentSessionId });
      } catch (error) {
        logger.error('[useChat] Failed to create session:', error);
        notificationStore.error('Failed to create chat session');
        return;
      }
    }

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
      await AIChatAPI.chat(
        prompt,
        currentSessionId,
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

  // Clear messages and reset session (for starting fresh)
  const clearMessages = () => {
    setMessages([]);
    setSessionId(''); // Clear session so next message creates a new one
  };

  // Load session messages
  const loadSession = async (id: string) => {
    try {
      const msgs = await AIChatAPI.getMessages(id);
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
      const session = await AIChatAPI.createSession();
      setSessionId(session.id);
      setMessages([]);
      return session;
    } catch (error) {
      logger.error('[useChat] Failed to create session:', error);
      notificationStore.error('Failed to create session');
      return null;
    }
  };

  // Update pending approval state (e.g., to mark as executing or remove)
  const updateApproval = (messageId: string, toolId: string, update: Partial<{ isExecuting: boolean; removed: boolean }>) => {
    setMessages((prev) =>
      prev.map((msg) => {
        if (msg.id !== messageId) return msg;
        if (update.removed) {
          // Remove from pendingApprovals
          return {
            ...msg,
            pendingApprovals: (msg.pendingApprovals || []).filter((a) => a.toolId !== toolId),
          };
        }
        // Update the approval in place
        return {
          ...msg,
          pendingApprovals: (msg.pendingApprovals || []).map((a) =>
            a.toolId === toolId ? { ...a, ...update } : a
          ),
        };
      })
    );
  };

  // Add a tool call result to a message (after approval execution)
  const addToolResult = (messageId: string, toolCall: { name: string; input: string; output: string; success: boolean }) => {
    setMessages((prev) =>
      prev.map((msg) => {
        if (msg.id !== messageId) return msg;
        return {
          ...msg,
          toolCalls: [...(msg.toolCalls || []), toolCall],
          streamEvents: [
            ...(msg.streamEvents || []),
            { type: 'tool' as const, tool: toolCall },
          ],
        };
      })
    );
  };

  // Update pending question state (e.g., to mark as answering or remove)
  const updateQuestion = (messageId: string, questionId: string, update: Partial<{ isAnswering: boolean; removed: boolean }>) => {
    setMessages((prev) =>
      prev.map((msg) => {
        if (msg.id !== messageId) return msg;
        if (update.removed) {
          // Remove from pendingQuestions
          return {
            ...msg,
            pendingQuestions: (msg.pendingQuestions || []).filter((q) => q.questionId !== questionId),
          };
        }
        // Update the question in place
        return {
          ...msg,
          pendingQuestions: (msg.pendingQuestions || []).map((q) =>
            q.questionId === questionId ? { ...q, ...update } : q
          ),
        };
      })
    );
  };

  // Answer a pending question
  const answerQuestion = async (messageId: string, questionId: string, answers: Array<{ id: string; value: string }>) => {
    updateQuestion(messageId, questionId, { isAnswering: true });

    try {
      // Send answer to Pulse AI via API
      await AIChatAPI.answerQuestion(questionId, answers);

      // Remove the question card - it's been handled
      updateQuestion(messageId, questionId, { removed: true });

      // After answering, check if the stream is still active.
      // If it closed (e.g. on question), we force a re-connection to receive continuation events.
      if (!isLoading()) {
        logger.debug('[useChat] Stream closed, re-initiating to catch continuation', {
          questionId,
          messageId,
        });

        const currentSessionId = sessionId();
        if (currentSessionId) {
          setIsLoading(true);
          abortControllerRef = new AbortController();

          // Set the message back to streaming state to show the AI is working
          setMessages((prev) =>
            prev.map((m) => (m.id === messageId ? { ...m, isStreaming: true } : m))
          );

          AIChatAPI.chat(
            '', // Empty prompt - just resume listening for completion
            currentSessionId,
            model() || undefined,
            (event) => {
              processEvent(messageId, event);
            },
            abortControllerRef.signal
          )
            .catch((err) => {
              if (err instanceof Error && err.name === 'AbortError') return;
              logger.error('[useChat] Re-connection failed:', err);
            })
            .finally(() => {
              setIsLoading(false);
              abortControllerRef = null;
            });
        }
      }

      logger.debug('[useChat] Question answered, waiting for AI to continue', {
        questionId,
      });

      // Brief delay to allow backend processing to settle
      await new Promise((resolve) => setTimeout(resolve, 500));
    } catch (error) {
      logger.error('[useChat] Failed to answer question:', error);
      notificationStore.error('Failed to answer question');
      updateQuestion(messageId, questionId, { isAnswering: false });
    }
  };


  // Wait for the chat to become idle (not loading)
  // Useful for sending follow-up messages after approvals
  const waitForIdle = (timeoutMs = 30000): Promise<boolean> => {
    return new Promise((resolve) => {
      if (!isLoading()) {
        resolve(true);
        return;
      }

      const startTime = Date.now();
      const checkInterval = setInterval(() => {
        if (!isLoading()) {
          clearInterval(checkInterval);
          resolve(true);
        } else if (Date.now() - startTime > timeoutMs) {
          clearInterval(checkInterval);
          logger.warn('[useChat] waitForIdle timed out');
          resolve(false);
        }
      }, 100);
    });
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
    updateApproval,
    addToolResult,
    updateQuestion,
    answerQuestion,
    waitForIdle,
  };
}
