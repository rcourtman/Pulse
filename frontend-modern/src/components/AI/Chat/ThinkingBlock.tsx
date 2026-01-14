import { Component, createSignal } from 'solid-js';
import { sanitizeThinking } from '../aiChatUtils';

interface ThinkingBlockProps {
  content: string;
  maxLength?: number;
}

export const ThinkingBlock: Component<ThinkingBlockProps> = (props) => {
  const [expanded, setExpanded] = createSignal(false);

  const truncated = () => {
    const max = props.maxLength ?? 300;
    const text = props.content;
    return text.length > max && !expanded() ? text.substring(0, max) + '...' : text;
  };

  const needsExpansion = () => {
    const max = props.maxLength ?? 300;
    return props.content.length > max;
  };

  return (
    <div class="rounded-lg overflow-hidden border border-blue-200 dark:border-blue-800 bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20">
      {/* Header - clickable to toggle */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded())}
        class="w-full px-3 py-2 flex items-center gap-2 text-left hover:bg-blue-100/50 dark:hover:bg-blue-800/30 transition-colors"
      >
        <div class="p-1 rounded bg-blue-100 dark:bg-blue-800/50">
          <svg class="w-3 h-3 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
          </svg>
        </div>
        <span class="text-[10px] font-medium uppercase tracking-wider text-blue-600 dark:text-blue-400">
          Thinking
        </span>
        <span class="flex-1" />
        <svg
          class={`w-4 h-4 text-blue-500 transition-transform ${expanded() ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {/* Content */}
      <div
        class={`px-3 overflow-hidden transition-all duration-200 ${
          expanded() ? 'max-h-96 py-2' : needsExpansion() ? 'max-h-20 py-2' : 'max-h-40 py-2'
        }`}
      >
        <div class="text-xs text-gray-600 dark:text-gray-400 leading-relaxed whitespace-pre-wrap overflow-y-auto max-h-80">
          {sanitizeThinking(truncated())}
        </div>
      </div>
    </div>
  );
};
