import { Component, Show, createSignal } from 'solid-js';
import type { ToolExecution, PendingTool } from './types';

interface ToolExecutionBlockProps {
  tool: ToolExecution;
  maxOutputLength?: number;
}

export const ToolExecutionBlock: Component<ToolExecutionBlockProps> = (props) => {
  const [expanded, setExpanded] = createSignal(false);

  const truncatedOutput = () => {
    const max = props.maxOutputLength ?? 500;
    const output = props.tool.output;
    if (!output) return '';
    return output.length > max && !expanded() ? output.substring(0, max) + '...' : output;
  };

  const needsTruncation = () => {
    const max = props.maxOutputLength ?? 500;
    return props.tool.output && props.tool.output.length > max;
  };

  return (
    <div class="rounded-lg border overflow-hidden shadow-sm transition-all hover:shadow-md">
      {/* Header */}
      <div
        class={`px-3 py-2 text-xs font-medium flex items-center gap-2 ${
          props.tool.success
            ? 'bg-gradient-to-r from-green-50 to-emerald-50 dark:from-green-900/30 dark:to-emerald-900/30 text-green-800 dark:text-green-200 border-b border-green-200 dark:border-green-800'
            : 'bg-gradient-to-r from-red-50 to-rose-50 dark:from-red-900/30 dark:to-rose-900/30 text-red-800 dark:text-red-200 border-b border-red-200 dark:border-red-800'
        }`}
      >
        <div class={`p-1 rounded ${props.tool.success ? 'bg-green-100 dark:bg-green-800/50' : 'bg-red-100 dark:bg-red-800/50'}`}>
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
        </div>
        <code class="font-mono flex-1 truncate">{props.tool.input}</code>
        <Show when={props.tool.success}>
          <svg class="w-4 h-4 text-green-500" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
          </svg>
        </Show>
        <Show when={!props.tool.success}>
          <svg class="w-4 h-4 text-red-500" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
          </svg>
        </Show>
      </div>

      {/* Output */}
      <Show when={props.tool.output}>
        <div class="relative">
          <pre class="px-3 py-2 text-xs font-mono bg-gray-50 dark:bg-gray-900/80 text-gray-700 dark:text-gray-300 overflow-x-auto max-h-48 overflow-y-auto whitespace-pre-wrap break-words">
            {truncatedOutput()}
          </pre>
          <Show when={needsTruncation()}>
            <button
              onClick={() => setExpanded(!expanded())}
              class="absolute bottom-2 right-2 px-2 py-1 text-[10px] font-medium bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-600 dark:text-gray-300 rounded transition-colors"
            >
              {expanded() ? 'Show less' : 'Show more'}
            </button>
          </Show>
        </div>
      </Show>
    </div>
  );
};

// Pending tool (still running)
interface PendingToolBlockProps {
  tool: PendingTool;
}

export const PendingToolBlock: Component<PendingToolBlockProps> = (props) => {
  return (
    <div class="rounded-lg border border-purple-300 dark:border-purple-700 overflow-hidden animate-pulse">
      <div class="px-3 py-2 text-xs font-medium flex items-center gap-2 bg-gradient-to-r from-purple-50 to-violet-50 dark:from-purple-900/30 dark:to-violet-900/30 text-purple-800 dark:text-purple-200">
        <div class="p-1 rounded bg-purple-100 dark:bg-purple-800/50">
          <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
          </svg>
        </div>
        <code class="font-mono flex-1 truncate">{props.tool.input}</code>
        <span class="text-[10px] text-purple-600 dark:text-purple-400 font-semibold uppercase tracking-wider">Running</span>
      </div>
    </div>
  );
};
