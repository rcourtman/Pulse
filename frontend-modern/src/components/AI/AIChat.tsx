import { Component, Show, createEffect, onMount, onCleanup } from 'solid-js';
import { aiChatStore } from '@/stores/aiChat';

interface AIChatProps {
  onClose: () => void;
}

/**
 * AIChat component - Embeds OpenCode's web UI in a slide-out panel.
 *
 * OpenCode provides the full chat experience including:
 * - Message display and streaming
 * - Tool execution visualization
 * - Session management
 * - Model selection
 *
 * Pulse provides infrastructure context via MCP tools.
 */
export const AIChat: Component<AIChatProps> = (props) => {
  const isOpen = () => aiChatStore.isOpen;
  let iframeRef: HTMLIFrameElement | undefined;

  // Focus iframe when panel opens for keyboard navigation
  createEffect(() => {
    if (isOpen() && iframeRef) {
      setTimeout(() => iframeRef?.focus(), 100);
    }
  });

  // Handle escape key to close panel
  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Escape' && isOpen()) {
      props.onClose();
    }
  };

  onMount(() => {
    document.addEventListener('keydown', handleKeyDown);
  });

  onCleanup(() => {
    document.removeEventListener('keydown', handleKeyDown);
  });

  return (
    <div
      class={`flex-shrink-0 h-full bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-700 flex flex-col transition-all duration-300 overflow-hidden ${
        isOpen() ? 'w-[500px]' : 'w-0 border-l-0'
      }`}
    >
      <Show when={isOpen()}>
        {/* Header */}
        <div class="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gradient-to-r from-purple-50 to-pink-50 dark:from-purple-900/20 dark:to-pink-900/20">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-purple-100 dark:bg-purple-900/40 rounded-lg">
              <svg
                class="w-5 h-5 text-purple-600 dark:text-purple-300"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.8"
                  d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611l-2.576.43a18.003 18.003 0 01-5.118 0l-2.576-.43c-1.717-.293-2.299-2.379-1.067-3.611L5 14.5"
                />
              </svg>
            </div>
            <div>
              <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                AI Assistant
              </h2>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                Powered by OpenCode
              </p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            {/* Open in new tab */}
            <a
              href="/opencode/"
              target="_blank"
              rel="noopener noreferrer"
              class="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800"
              title="Open in new tab"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                />
              </svg>
            </a>
            {/* Collapse panel */}
            <button
              onClick={props.onClose}
              class="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800"
              title="Collapse panel (Esc)"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M13 5l7 7-7 7M6 5l7 7-7 7"
                />
              </svg>
            </button>
          </div>
        </div>

        {/* OpenCode iframe */}
        <div class="flex-1 relative">
          <iframe
            ref={iframeRef}
            src="/opencode/"
            class="absolute inset-0 w-full h-full border-0"
            title="AI Assistant"
            allow="clipboard-read; clipboard-write"
          />
        </div>
      </Show>
    </div>
  );
};

export default AIChat;
