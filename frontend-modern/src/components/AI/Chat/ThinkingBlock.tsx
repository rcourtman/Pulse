import { Component } from 'solid-js';
import BrainIcon from 'lucide-solid/icons/brain';

interface ThinkingBlockProps {
  content?: string;
  isStreaming?: boolean;
}

export const ThinkingBlock: Component<ThinkingBlockProps> = (props) => (
  <div
    class="my-2 inline-flex items-center gap-2 rounded-md border border-border-subtle bg-surface-alt px-2.5 py-1.5 text-xs text-muted"
    role="status"
  >
    <BrainIcon
      class={`h-3.5 w-3.5 text-blue-500 ${props.isStreaming ? 'animate-pulse' : ''}`}
      aria-hidden="true"
    />
    <span>{props.isStreaming ? 'Thinking...' : 'Thinking complete'}</span>
  </div>
);
