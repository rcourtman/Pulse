import { Component, Show, For, createEffect, createMemo } from 'solid-js';
import { MessageItem } from './MessageItem';
import type { ChatSession } from '@/api/aiChat';
import type { ChatMessage, PendingApproval, PendingQuestion } from './types';

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
        <div class="flex flex-col items-center justify-center min-h-full text-center py-8">
          <h3 class="text-base font-semibold text-base-content mb-3">{props.emptyState!.title}</h3>
          <Show when={props.emptyState!.subtitle}>
            <p class="text-sm text-muted max-w-xs">{props.emptyState!.subtitle}</p>
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
          />
        )}
      </For>

      {/* Scroll anchor */}
      <div ref={messagesEndRef} class="h-1" />
    </div>
  );
};
