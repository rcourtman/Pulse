import { Component, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import BrainIcon from 'lucide-solid/icons/brain';
import { extractReasoningSummaryTitle } from './reasoningSummary';

interface ThinkingBlockProps {
  content?: string;
  isStreaming?: boolean;
  startedAt?: number;
  updatedAt?: number;
}

const formatThinkingDuration = (durationMs: number): string => {
  if (!Number.isFinite(durationMs) || durationMs < 0) return '';
  if (durationMs < 1000) return '<1s';

  const totalSeconds = Math.max(1, Math.floor(durationMs / 1000));
  if (totalSeconds < 60) return `${totalSeconds}s`;

  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes < 60) return seconds ? `${minutes}m ${seconds}s` : `${minutes}m`;

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return remainingMinutes ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
};

export const ThinkingBlock: Component<ThinkingBlockProps> = (props) => {
  const [now, setNow] = createSignal(Date.now());

  createEffect(() => {
    if (!props.isStreaming || !props.startedAt) return;
    setNow(Date.now());
    const interval = window.setInterval(() => setNow(Date.now()), 1000);
    onCleanup(() => window.clearInterval(interval));
  });

  const summaryTitle = createMemo(() => extractReasoningSummaryTitle(props.content));
  const durationLabel = createMemo(() => {
    if (!props.startedAt) return '';
    const end = props.isStreaming ? now() : props.updatedAt;
    if (!end) return '';
    const duration = formatThinkingDuration(end - props.startedAt);
    if (props.isStreaming && duration === '<1s') return '';
    return duration;
  });
  const statusText = createMemo(() => {
    const title = summaryTitle();
    if (props.isStreaming) {
      return title ? `Thinking: ${title}` : 'Thinking...';
    }
    return title ? `Thought: ${title}` : 'Thinking complete';
  });

  return (
    <div
      class="my-2 inline-flex max-w-full items-center gap-2 rounded-md border border-border-subtle bg-surface-alt px-2.5 py-1.5 text-xs text-muted"
      role="status"
    >
      <BrainIcon
        class={`h-3.5 w-3.5 shrink-0 text-blue-500 ${props.isStreaming ? 'animate-pulse' : ''}`}
        aria-hidden="true"
      />
      <span class="min-w-0 truncate">
        {statusText()}
        <span class="text-muted/80">
          {durationLabel() ? (props.isStreaming ? ` (${durationLabel()})` : ` · ${durationLabel()}`) : ''}
        </span>
      </span>
    </div>
  );
};
