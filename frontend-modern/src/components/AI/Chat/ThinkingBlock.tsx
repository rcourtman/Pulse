import { Component } from 'solid-js';
import { sanitizeThinking } from '../aiChatUtils';

interface ThinkingBlockProps {
  content: string;
  maxLength?: number;
}

export const ThinkingBlock: Component<ThinkingBlockProps> = (props) => {
  const truncated = () => {
    const max = props.maxLength ?? 500;
    const text = props.content;
    return text.length > max ? text.substring(0, max) + '...' : text;
  };

  return (
    <div class="px-3 py-2 text-xs bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 text-gray-700 dark:text-gray-300 rounded-lg border-l-3 border-blue-400 dark:border-blue-500 whitespace-pre-wrap">
      <div class="flex items-center gap-1.5 mb-1.5 text-blue-600 dark:text-blue-400 font-medium">
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
        </svg>
        <span class="text-[10px] uppercase tracking-wider">Thinking</span>
      </div>
      <div class="text-gray-600 dark:text-gray-400 leading-relaxed">
        {sanitizeThinking(truncated())}
      </div>
    </div>
  );
};
