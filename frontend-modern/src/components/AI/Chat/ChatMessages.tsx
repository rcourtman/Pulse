import { Component, Show, For, createEffect, createMemo } from 'solid-js';
import { MessageItem } from './MessageItem';
import type { ChatSession } from '@/api/aiChat';
import type {
  ChatMessage,
  ModelRouteRecoveryOption,
  PendingApproval,
  PendingQuestion,
} from './types';
import { humanizeToken } from '@/utils/textPresentation';

interface ChatMessagesProps {
  messages: ChatMessage[];
  onApprove: (messageId: string, approval: PendingApproval) => void;
  onSkip: (messageId: string, toolId: string) => void;
  onAnswerQuestion: (
    messageId: string,
    question: PendingQuestion,
    answers: Array<{ id: string; value: string }>,
  ) => void;
  onSkipQuestion: (messageId: string, questionId: string) => void;
  onRetry?: (messageId: string) => void;
  onChangeModel?: () => void;
  getModelRouteAlternative?: (message: ChatMessage) => ModelRouteRecoveryOption | null;
  onUseModelRoute?: (modelId: string) => void;
  // Dashboard props
  recentSessions?: ChatSession[];
  onLoadSession?: (sessionId: string) => void;
  emptyState?: {
    title: string;
    subtitle?: string;
  };
}

/**
 * ChatMessages - Renders the scrollable message list.
 *
 * Features:
 * - Auto-scroll to bottom on new messages
 * - Empty state
 * - Smooth scrolling behavior
 */
export const ChatMessages: Component<ChatMessagesProps> = (props) => {
  let messagesEndRef: HTMLDivElement | undefined;
  let containerRef: HTMLDivElement | undefined;

  // Track content changes for auto-scroll (not just array length)
  // This tracks: message count, last message content length, streaming state, and stream events
  const scrollTrigger = createMemo(() => {
    const msgs = props.messages;
    if (msgs.length === 0) return 0;
    const lastMsg = msgs[msgs.length - 1];
    // Combine multiple signals to ensure we detect all content updates
    return (
      msgs.length +
      (lastMsg.content?.length || 0) +
      (lastMsg.isStreaming ? 1000000 : 0) +
      (lastMsg.streamEvents?.length || 0)
    );
  });

  const recentSessions = createMemo(() =>
    (props.recentSessions || []).filter((session) => session.message_count > 0).slice(0, 3),
  );

  const formatSessionMessageCount = (count: number) =>
    `${count} ${count === 1 ? 'message' : 'messages'}`;

  const formatSessionHandoffLabel = (session: ChatSession) => {
    const summary = session.handoff_summary;
    if (!summary) return '';
    const kind = summary.kind?.trim();
    if (kind) return humanizeToken(kind);
    if (summary.finding_id) return 'Patrol finding';
    if (summary.run_id) return 'Patrol run';
    return summary.has_model_context ? 'Context attached' : '';
  };

  // Auto-scroll to bottom on new messages or streaming content
  createEffect(() => {
    // Access the trigger to establish dependency (void suppresses unused var warning)
    void scrollTrigger();

    if (props.messages.length > 0 && messagesEndRef && containerRef) {
      // Only auto-scroll if user is near the bottom (within 200px)
      const { scrollTop, scrollHeight, clientHeight } = containerRef;
      const isNearBottom = scrollHeight - scrollTop - clientHeight < 200;

      if (isNearBottom) {
        // Use instant scroll during active streaming for smoother experience
        const lastMsg = props.messages[props.messages.length - 1];
        const behavior = lastMsg.isStreaming ? 'instant' : 'smooth';
        messagesEndRef.scrollIntoView({ behavior: behavior as ScrollBehavior });
      }
    }
  });

  return (
    <div ref={containerRef} class="flex-1 overflow-y-auto px-4 py-3 bg-surface">
      {/* Empty state */}
      <Show when={props.messages.length === 0 && props.emptyState}>
        <div class="flex min-h-full flex-col items-center justify-center px-2 py-8 text-center">
          <h3 class="text-base font-semibold text-base-content mb-3">{props.emptyState!.title}</h3>
          <Show when={props.emptyState!.subtitle}>
            <p class="text-sm text-muted max-w-xs">{props.emptyState!.subtitle}</p>
          </Show>
          <Show when={recentSessions().length > 0 && props.onLoadSession}>
            <div class="mt-6 w-full max-w-sm text-left" aria-label="Recent Assistant sessions">
              <div class="mb-2 text-[11px] font-semibold uppercase text-muted">Recent sessions</div>
              <div class="space-y-1.5">
                <For each={recentSessions()}>
                  {(session) => {
                    const handoffLabel = () => formatSessionHandoffLabel(session);
                    return (
                      <button
                        type="button"
                        class="w-full rounded-md border border-border bg-surface px-3 py-2 text-left transition-colors hover:border-blue-300 hover:bg-surface-alt focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                        onClick={() => props.onLoadSession?.(session.id)}
                        aria-label={`Resume ${session.title || 'Untitled Assistant session'}`}
                      >
                        <div class="truncate text-sm font-medium text-base-content">
                          {session.title || 'Untitled'}
                        </div>
                        <div class="mt-0.5 flex min-w-0 flex-wrap items-center gap-1.5 text-[11px] text-muted">
                          <span>{formatSessionMessageCount(session.message_count)}</span>
                          <Show when={handoffLabel()}>
                            {(label) => (
                              <>
                                <span aria-hidden="true">/</span>
                                <span class="truncate">{label()}</span>
                              </>
                            )}
                          </Show>
                        </div>
                      </button>
                    );
                  }}
                </For>
              </div>
            </div>
          </Show>
        </div>
      </Show>

      {/* Messages */}
      <For each={props.messages}>
        {(message) => (
          <MessageItem
            message={message}
            onApprove={(approval) => props.onApprove(message.id, approval)}
            onSkip={(toolId) => props.onSkip(message.id, toolId)}
            onAnswerQuestion={(question, answers) =>
              props.onAnswerQuestion(message.id, question, answers)
            }
            onSkipQuestion={(questionId) => props.onSkipQuestion(message.id, questionId)}
            onRetry={props.onRetry}
            onChangeModel={props.onChangeModel}
            modelRouteAlternative={props.getModelRouteAlternative?.(message)}
            onUseModelRoute={props.onUseModelRoute}
          />
        )}
      </For>

      {/* Scroll anchor */}
      <div ref={messagesEndRef} class="h-1" />
    </div>
  );
};
