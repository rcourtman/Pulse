import { Component, Show, For, createEffect } from 'solid-js';
import { MessageItem } from './MessageItem';
import type { ChatMessage, PendingApproval } from './types';

interface ChatMessagesProps {
  messages: ChatMessage[];
  onApprove: (messageId: string, approval: PendingApproval) => void;
  onSkip: (messageId: string, toolId: string) => void;
  emptyState?: {
    title: string;
    subtitle: string;
    suggestions?: string[];
    onSuggestionClick?: (suggestion: string) => void;
  };
}

export const ChatMessages: Component<ChatMessagesProps> = (props) => {
  let messagesEndRef: HTMLDivElement | undefined;

  // Auto-scroll to bottom
  createEffect(() => {
    if (props.messages.length > 0 && messagesEndRef) {
      messagesEndRef.scrollIntoView({ behavior: 'smooth' });
    }
  });

  return (
    <div class="flex-1 overflow-y-auto p-4 space-y-4">
      {/* Empty state */}
      <Show when={props.messages.length === 0 && props.emptyState}>
        <div class="flex flex-col items-center justify-center h-full text-center py-12">
          {/* AI Icon */}
          <div class="w-16 h-16 mb-4 rounded-2xl bg-gradient-to-br from-purple-100 to-violet-100 dark:from-purple-900/30 dark:to-violet-900/30 flex items-center justify-center">
            <svg
              class="w-8 h-8 text-purple-500 dark:text-purple-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.5"
                d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611l-2.576.43a18.003 18.003 0 01-5.118 0l-2.576-.43c-1.717-.293-2.299-2.379-1.067-3.611L5 14.5"
              />
            </svg>
          </div>

          <h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-1">
            {props.emptyState!.title}
          </h3>
          <p class="text-sm text-gray-500 dark:text-gray-400 max-w-xs mb-6">
            {props.emptyState!.subtitle}
          </p>

          {/* Suggestions */}
          <Show when={props.emptyState!.suggestions && props.emptyState!.suggestions!.length > 0}>
            <div class="space-y-2 w-full max-w-xs">
              <For each={props.emptyState!.suggestions}>
                {(suggestion) => (
                  <button
                    type="button"
                    onClick={() => props.emptyState!.onSuggestionClick?.(suggestion)}
                    class="w-full text-left px-4 py-2.5 rounded-xl bg-purple-50 dark:bg-purple-900/20 text-purple-700 dark:text-purple-300 text-sm hover:bg-purple-100 dark:hover:bg-purple-900/40 transition-colors border border-purple-100 dark:border-purple-800"
                  >
                    "{suggestion}"
                  </button>
                )}
              </For>
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
          />
        )}
      </For>

      {/* Scroll anchor */}
      <div ref={messagesEndRef} />
    </div>
  );
};
