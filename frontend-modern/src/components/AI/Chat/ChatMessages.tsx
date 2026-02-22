import { Component, Show, For, createEffect, createMemo } from 'solid-js';
import { MessageItem } from './MessageItem';
import type { ChatSession } from '@/api/aiChat';
import type { ChatMessage, PendingApproval, PendingQuestion } from './types';

interface ChatMessagesProps {
  messages: ChatMessage[];
  onApprove: (messageId: string, approval: PendingApproval) => void;
  onSkip: (messageId: string, toolId: string) => void;
  onAnswerQuestion: (messageId: string, question: PendingQuestion, answers: Array<{ id: string; value: string }>) => void;
  onSkipQuestion: (messageId: string, questionId: string) => void;
  // Dashboard props
  recentSessions?: ChatSession[];
  onLoadSession?: (sessionId: string) => void;
  emptyState?: {
    title: string;
    subtitle?: string;
    suggestions?: string[];
    onSuggestionClick?: (suggestion: string) => void;
  };
}

/**
 * ChatMessages - Renders the scrollable message list.
 *
 * Features:
 * - Auto-scroll to bottom on new messages
 * - Empty state with suggestions
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
    return msgs.length +
      (lastMsg.content?.length || 0) +
      (lastMsg.isStreaming ? 1000000 : 0) +
      (lastMsg.streamEvents?.length || 0);
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
    <div
      ref={containerRef}
      class="flex-1 overflow-y-auto px-4 py-3 bg-white dark:bg-slate-900"
    >
      {/* Empty state */}
      <Show when={props.messages.length === 0 && props.emptyState}>
        <div class="flex flex-col items-center justify-center min-h-full text-center py-8">
          {/* AI Icon */}
          <div class="w-14 h-14 mb-3 rounded-md bg-slate-100 dark:bg-slate-800 flex items-center justify-center shadow-sm">
            <svg
              class="w-7 h-7 text-blue-500 dark:text-blue-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.5"
                d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456zM16.894 20.567L16.5 21.75l-.394-1.183a2.25 2.25 0 00-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 001.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 001.423 1.423l1.183.394-1.183.394a2.25 2.25 0 00-1.423 1.423z"
              />
            </svg>
          </div>

          <h3 class="text-base font-semibold text-base-content mb-6">
            {props.emptyState!.title}
          </h3>
          <Show when={props.emptyState!.subtitle}>
            <p class="text-sm text-muted max-w-xs mb-6">
              {props.emptyState!.subtitle}
            </p>
          </Show>

          <div class="w-full max-w-xs space-y-6">


            {/* Suggestions */}
            <Show when={props.emptyState!.suggestions && props.emptyState!.suggestions!.length > 0}>
              <div class="space-y-2">
                <div class="text-xs font-medium text-muted text-left uppercase tracking-wider pl-1">
                  Or try asking
                </div>
                <For each={props.emptyState!.suggestions}>
                  {(suggestion) => (
                    <button
                      type="button"
                      onClick={() => props.emptyState!.onSuggestionClick?.(suggestion)}
                      class="w-full text-left px-4 py-2.5 rounded-md bg-slate-50 dark:bg-slate-800 text-muted text-sm hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors border border-transparent hover:border-slate-200 dark:hover:border-slate-700"
                    >
                      <span class="text-blue-500 dark:text-blue-400 mr-2 opacity-50">→</span>
                      {suggestion}
                    </button>
                  )}
                </For>
              </div>
            </Show>
          </div>
        </div>
      </Show>

      {/* Messages */}
      <For each={props.messages}>
        {(message) => (
          <MessageItem
            message={message}
            onApprove={(approval) => props.onApprove(message.id, approval)}
            onSkip={(toolId) => props.onSkip(message.id, toolId)}
            onAnswerQuestion={(question, answers) => props.onAnswerQuestion(message.id, question, answers)}
            onSkipQuestion={(questionId) => props.onSkipQuestion(message.id, questionId)}
          />
        )}
      </For>

      {/* Scroll anchor */}
      <div ref={messagesEndRef} class="h-1" />
    </div>
  );
};
