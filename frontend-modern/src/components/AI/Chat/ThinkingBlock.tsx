import { Component, createSignal, Show, createMemo } from 'solid-js';
import { sanitizeThinking } from '../aiChatUtils';

interface ThinkingBlockProps {
  content: string;
  isStreaming?: boolean;
}

/**
 * ThinkingBlock - Displays AI's reasoning/thinking in a collapsed-by-default block.
 * 
 * Inspired by Pulse AI's terminal TUI which shows thinking as a subtle,
 * collapsible section that doesn't distract from the main response.
 */
export const ThinkingBlock: Component<ThinkingBlockProps> = (props) => {
  const [expanded, setExpanded] = createSignal(false);

  // Count lines and words for preview
  const stats = createMemo(() => {
    const lines = props.content.split('\n').filter(l => l.trim()).length;
    const words = props.content.split(/\s+/).filter(w => w).length;
    return { lines, words };
  });

  // Get a short preview (first line, truncated)
  const preview = createMemo(() => {
    const firstLine = props.content.split('\n').find(l => l.trim()) || '';
    const maxLen = 60;
    if (firstLine.length > maxLen) {
      return firstLine.substring(0, maxLen).trim() + '...';
    }
    return firstLine.trim();
  });

  return (
    <div class="my-2 font-mono text-xs">
      {/* Collapsed header - always visible */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded())}
        class="w-full flex items-center gap-2 px-3 py-1.5 rounded-md bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700/60 transition-colors text-left group"
      >
        {/* Thinking icon */}
        <div class={`flex items-center justify-center w-4 h-4 ${props.isStreaming ? 'animate-pulse' : ''}`}>
          <svg class="w-3.5 h-3.5 text-blue-500 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
          </svg>
        </div>

        {/* Label */}
        <span class="text-blue-600 dark:text-blue-400 font-medium uppercase text-[10px] tracking-wider">
          {props.isStreaming ? 'Thinking...' : 'Thinking'}
        </span>

        {/* Preview (when collapsed) */}
        <Show when={!expanded() && preview()}>
          <span class="text-slate-500 dark:text-slate-400 truncate flex-1">
            {preview()}
          </span>
        </Show>

        {/* Stats */}
        <span class="text-slate-400 dark:text-slate-500 text-[10px] ml-auto">
          {stats().lines} lines · {stats().words} words
        </span>

        {/* Expand/collapse chevron */}
        <svg
          class={`w-3.5 h-3.5 text-slate-400 transition-transform ${expanded() ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {/* Expanded content */}
      <Show when={expanded()}>
        <div class="mt-1 ml-4 pl-3 border-l-2 border-blue-200 dark:border-blue-800">
          <pre class="text-[11px] text-slate-600 dark:text-slate-400 whitespace-pre-wrap leading-relaxed max-h-64 overflow-y-auto">
            {sanitizeThinking(props.content)}
          </pre>
        </div>
      </Show>
    </div>
  );
};
