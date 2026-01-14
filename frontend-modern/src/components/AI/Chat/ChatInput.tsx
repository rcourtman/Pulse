import { Component, Show, For } from 'solid-js';
import type { ChatContextItem } from './types';

interface ChatInputProps {
  value: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
  onStop: () => void;
  isLoading: boolean;
  queuedMessage: string | null;
  onClearQueue: () => void;
  placeholder?: string;
  contextItems: ChatContextItem[];
  onAddContext: () => void;
  onRemoveContext: (id: string) => void;
  onClearAllContext: () => void;
  disabled?: boolean;
}

export const ChatInput: Component<ChatInputProps> = (props) => {
  let inputRef: HTMLTextAreaElement | undefined;

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      props.onSubmit();
    }
  };

  const placeholder = () => {
    if (props.isLoading) {
      return props.queuedMessage
        ? 'Type another message to replace queued...'
        : 'Type to queue your next message...';
    }
    return props.placeholder || 'Ask about your infrastructure...';
  };

  return (
    <div class="border-t border-gray-200 dark:border-gray-700 p-4 bg-white dark:bg-gray-900">
      {/* Context section */}
      <div class="mb-3 px-3 py-2.5 bg-gray-50 dark:bg-gray-800/50 rounded-xl border border-gray-200 dark:border-gray-700">
        <div class="flex items-center justify-between mb-2">
          <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-400">
            <svg class="w-3.5 h-3.5 text-purple-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span class="font-medium">
              Context {props.contextItems.length > 0 ? `(${props.contextItems.length})` : ''}
            </span>
          </div>
          <Show when={props.contextItems.length > 0}>
            <button
              type="button"
              onClick={props.onClearAllContext}
              class="text-[10px] text-gray-400 hover:text-red-500 transition-colors"
            >
              Clear all
            </button>
          </Show>
        </div>

        {/* Context items */}
        <div class="flex flex-wrap gap-1.5">
          <For each={props.contextItems}>
            {(item) => (
              <span class="inline-flex items-center gap-1 px-2 py-1 text-[11px] rounded-lg bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-200">
                <span class="text-[9px] uppercase text-purple-500 dark:text-purple-400 font-semibold">
                  {item.type}
                </span>
                <span class="font-medium">{item.name}</span>
                <button
                  type="button"
                  onClick={() => props.onRemoveContext(item.id)}
                  class="ml-0.5 p-0.5 rounded hover:bg-purple-200 dark:hover:bg-purple-800 transition-colors"
                >
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </span>
            )}
          </For>

          {/* Add context button */}
          <button
            type="button"
            onClick={props.onAddContext}
            class="inline-flex items-center gap-1 px-2 py-1 text-[11px] rounded-lg border border-dashed border-gray-300 dark:border-gray-600 text-gray-500 dark:text-gray-400 hover:border-purple-400 hover:text-purple-600 dark:hover:border-purple-500 dark:hover:text-purple-400 transition-colors"
          >
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            <span>Add</span>
          </button>
        </div>

        <Show when={props.contextItems.length === 0}>
          <p class="mt-2 text-[10px] text-gray-400 dark:text-gray-500">
            Add VMs, containers, or hosts to provide context for your questions
          </p>
        </Show>
      </div>

      {/* Queued message indicator */}
      <Show when={props.queuedMessage}>
        <div class="flex items-center gap-2 px-3 py-2 mb-2 text-xs rounded-lg bg-amber-50 dark:bg-amber-900/30 border border-amber-200 dark:border-amber-700 text-amber-700 dark:text-amber-300">
          <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span class="flex-1 truncate">
            <span class="font-medium">Queued:</span> "{props.queuedMessage!.substring(0, 50)}
            {props.queuedMessage!.length > 50 ? '...' : ''}"
          </span>
          <button
            type="button"
            onClick={props.onClearQueue}
            class="p-0.5 rounded hover:bg-amber-200 dark:hover:bg-amber-800 transition-colors"
          >
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </Show>

      {/* Input form */}
      <form onSubmit={(e) => { e.preventDefault(); props.onSubmit(); }} class="flex gap-2">
        <textarea
          ref={inputRef}
          value={props.value}
          onInput={(e) => props.onChange(e.currentTarget.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder()}
          rows={2}
          disabled={props.disabled}
          class={`flex-1 px-4 py-3 text-sm rounded-xl border bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-2 focus:border-transparent resize-none transition-all ${
            props.isLoading
              ? 'border-amber-300 dark:border-amber-600 focus:ring-amber-500'
              : 'border-gray-200 dark:border-gray-700 focus:ring-purple-500'
          } ${props.disabled ? 'opacity-50 cursor-not-allowed' : ''}`}
        />

        <div class="flex flex-col gap-1.5 self-end">
          <Show
            when={props.isLoading}
            fallback={
              <button
                type="submit"
                disabled={!props.value.trim() || props.disabled}
                class="px-4 py-3 bg-gradient-to-r from-purple-600 to-violet-600 hover:from-purple-700 hover:to-violet-700 text-white rounded-xl disabled:opacity-50 disabled:cursor-not-allowed transition-all shadow-lg hover:shadow-xl disabled:shadow-none"
              >
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                </svg>
              </button>
            }
          >
            {/* Queue button when loading */}
            <button
              type="submit"
              disabled={!props.value.trim()}
              class="px-4 py-2.5 bg-amber-500 hover:bg-amber-600 text-white rounded-xl disabled:opacity-50 disabled:cursor-not-allowed transition-colors shadow-sm"
              title={props.queuedMessage ? 'Replace queued message' : 'Queue message'}
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </button>
            {/* Stop button */}
            <button
              type="button"
              onClick={props.onStop}
              class="px-4 py-2.5 bg-red-500 hover:bg-red-600 text-white rounded-xl transition-colors shadow-sm"
              title="Stop generating"
            >
              <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                <rect x="6" y="6" width="12" height="12" rx="1" />
              </svg>
            </button>
          </Show>
        </div>
      </form>

      {/* Help text */}
      <p class="text-xs text-gray-400 dark:text-gray-500 mt-2 text-center">
        {props.isLoading
          ? 'Type and press Enter to queue your next message'
          : 'Press Enter to send, Shift+Enter for new line'}
      </p>
    </div>
  );
};
