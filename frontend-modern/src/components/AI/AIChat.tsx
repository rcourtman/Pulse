import { Component, Show, lazy, Suspense } from 'solid-js';
import { aiChatStore } from '@/stores/aiChat';

// Lazy load the Chat component for better initial load performance
const Chat = lazy(() => import('./Chat'));

interface AIChatProps {
  onClose: () => void;
}

/**
 * AIChat component - Slide-out panel for AI Assistant.
 *
 * Uses the custom Chat component which communicates with OpenCode
 * through Pulse's backend API, providing:
 * - Streaming chat responses
 * - Tool execution visualization
 * - Session management
 * - Model selection
 *
 * Pulse provides infrastructure context via MCP tools.
 */
export const AIChat: Component<AIChatProps> = (props) => {
  const isOpen = () => aiChatStore.isOpen;

  return (
    <div
      class={`flex-shrink-0 h-full bg-white dark:bg-gray-900 border-l border-gray-200 dark:border-gray-700 flex flex-col transition-all duration-300 overflow-hidden ${
        isOpen() ? 'w-[500px]' : 'w-0 border-l-0'
      }`}
    >
      <Show when={isOpen()}>
        <Suspense
          fallback={
            <div class="flex-1 flex items-center justify-center">
              <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-purple-600" />
            </div>
          }
        >
          <Chat onClose={props.onClose} />
        </Suspense>
      </Show>
    </div>
  );
};

export default AIChat;
