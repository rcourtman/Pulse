import { Component, Show, For, Switch, Match, createMemo, createSignal } from 'solid-js';
import CheckIcon from 'lucide-solid/icons/check';
import CircleAlertIcon from 'lucide-solid/icons/circle-alert';
import CopyIcon from 'lucide-solid/icons/copy';
import RotateCcwIcon from 'lucide-solid/icons/rotate-ccw';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import { renderMarkdown } from '../aiChatUtils';
import { ThinkingBlock } from './ThinkingBlock';
import { ToolExecutionBlock } from './ToolExecutionBlock';
import { ApprovalCard } from './ApprovalCard';
import { QuestionCard } from './QuestionCard';
import { groupStreamEventsForDisplay } from './streamEventGrouping';
import type { ChatMessage, PendingApproval, PendingQuestion } from './types';
import {
  AI_CHAT_ASSISTANT_MESSAGE_LABEL,
  AI_CHAT_CONTEXT_USED_LABEL,
} from '@/utils/aiChatPresentation';
import { formatIdentifierLabel } from '@/utils/textPresentation';

interface MessageItemProps {
  message: ChatMessage;
  onApprove: (approval: PendingApproval) => void;
  onSkip: (toolId: string) => void;
  onAnswerQuestion: (
    question: PendingQuestion,
    answers: Array<{ id: string; value: string }>,
  ) => void;
  onSkipQuestion: (questionId: string) => void;
  onRetry?: (messageId: string) => void;
}

const markdownClass =
  'text-sm prose prose-slate prose-sm dark:prose-invert max-w-none prose-p:leading-relaxed prose-p:my-2 prose-pre:bg-slate-900 prose-pre:text-slate-100 prose-pre:rounded-md prose-pre:text-xs prose-pre:border prose-pre:border-slate-800 prose-code:text-blue-700 dark:prose-code:text-blue-300 prose-code:bg-blue-50 dark:prose-code:bg-blue-900 prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded-md prose-code:font-mono prose-code:text-[0.9em] prose-code:border prose-code:border-blue-100 dark:prose-code:border-blue-800 prose-code:before:content-none prose-code:after:content-none prose-headings:font-semibold prose-hr:border-slate-200 dark:prose-hr:border-slate-700 prose-ul:my-2 prose-ol:my-2 prose-li:my-1';

/**
 * MessageItem - Renders a single message in the chat.
 *
 * User messages: Compact, right-aligned bubble
 * Assistant messages: Full-width transcript rows with clear sections
 */
export const MessageItem: Component<MessageItemProps> = (props) => {
  const isUser = () => props.message.role === 'user';

  const hasStreamEvents = () => props.message.streamEvents && props.message.streamEvents.length > 0;

  // Group stream events into display blocks. Content and reasoning each collapse
  // into a single block even when a reasoning model interleaves them, so the
  // answer stays a coherent markdown document instead of fragmenting into
  // whitespace-trimmed pieces. See groupStreamEventsForDisplay for the rationale.
  const groupedEvents = createMemo(() =>
    groupStreamEventsForDisplay(props.message.streamEvents || []),
  );

  const contextTools = createMemo(() => {
    const events = props.message.streamEvents || [];
    const names = new Set<string>();

    for (const evt of events) {
      if (evt.type === 'tool' && evt.tool?.name) {
        names.add(evt.tool.name);
      }
    }

    return Array.from(names);
  });

  // Check if currently streaming content (no tools pending, still streaming)
  const isStreamingText = () =>
    props.message.isStreaming &&
    (!props.message.pendingTools || props.message.pendingTools.length === 0);
  const isWaitingForFirstToken = () =>
    isStreamingText() &&
    !props.message.content.trim() &&
    !hasStreamEvents() &&
    !props.message.error;

  // Copy-to-clipboard for a completed assistant answer.
  const [copied, setCopied] = createSignal(false);
  const canCopy = () => !props.message.isStreaming && !!props.message.content?.trim();
  const copyMessage = async () => {
    const text = props.message.content || '';
    try {
      await navigator.clipboard?.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard can be unavailable (permissions / insecure context); fail quietly.
    }
  };

  return (
    <div class={`${isUser() ? 'flex justify-end' : ''} mb-4`}>
      {/* User message - compact bubble */}
      <Show when={isUser()}>
        <div class="max-w-[85%] px-4 py-2.5 rounded-md rounded-br-sm bg-blue-600 text-white shadow-sm">
          <p class="text-sm whitespace-pre-wrap">{props.message.content}</p>
        </div>
      </Show>

      {/* Assistant message */}
      <Show when={!isUser()}>
        <div class="group flex w-full min-w-0 gap-3 px-1 py-2">
          <div class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border-subtle bg-surface-alt text-blue-600 shadow-sm dark:text-blue-400">
            <SparklesIcon class="h-3.5 w-3.5" />
          </div>

          <div class="min-w-0 flex-1">
            <div class="mb-2 flex min-h-7 items-center gap-2">
              <span class="text-xs font-semibold text-base-content">
                {AI_CHAT_ASSISTANT_MESSAGE_LABEL}
              </span>
              <Show when={props.message.model && !props.message.isStreaming}>
                <span class="text-[10px] text-muted font-mono">{props.message.model}</span>
              </Show>
              <Show when={canCopy()}>
                <button
                  type="button"
                  onClick={copyMessage}
                  aria-label={copied() ? 'Copied' : 'Copy message'}
                  title={copied() ? 'Copied' : 'Copy message'}
                  class="ml-auto inline-flex h-7 w-7 items-center justify-center rounded-md border border-border-subtle bg-surface text-muted opacity-0 shadow-sm transition-opacity hover:text-base-content focus:opacity-100 group-hover:opacity-100"
                >
                  <Show
                    when={copied()}
                    fallback={<CopyIcon class="h-3.5 w-3.5" aria-hidden="true" />}
                  >
                    <CheckIcon class="h-3.5 w-3.5 text-emerald-500" aria-hidden="true" />
                  </Show>
                </button>
              </Show>
            </div>

            <div>
              <Show when={isWaitingForFirstToken()}>
                <div class="flex items-center gap-2 py-1 text-sm text-muted">
                  <span class="flex gap-1" aria-hidden="true">
                    <span class="h-1.5 w-1.5 rounded-full bg-blue-500 animate-bounce" />
                    <span
                      class="h-1.5 w-1.5 rounded-full bg-blue-500 animate-bounce"
                      style="animation-delay: 120ms"
                    />
                    <span
                      class="h-1.5 w-1.5 rounded-full bg-blue-500 animate-bounce"
                      style="animation-delay: 240ms"
                    />
                  </span>
                  <span>Thinking...</span>
                </div>
              </Show>

              {/* Stream events - chronological display */}
              <Show when={hasStreamEvents()}>
                <For each={groupedEvents()}>
                  {(evt) => (
                    <Switch>
                      {/* Thinking block */}
                      <Match when={evt.type === 'thinking' && evt.thinking}>
                        <ThinkingBlock
                          content={evt.thinking || ''}
                          isStreaming={props.message.isStreaming}
                        />
                      </Match>

                      <Match when={evt.type === 'pending_tool' && evt.pendingTool}>
                        <></>
                      </Match>

                      <Match when={evt.type === 'tool' && evt.tool}>
                        <ToolExecutionBlock
                          tool={{
                            name: evt.tool?.name || 'unknown',
                            input: evt.tool?.input || '{}',
                            output: evt.tool?.output || '',
                            success: evt.tool?.success ?? true,
                          }}
                        />
                      </Match>

                      {/* Content/text block */}
                      <Match when={evt.type === 'content' && evt.content}>
                        <div
                          class={markdownClass}
                          // eslint-disable-next-line solid/no-innerhtml
                          innerHTML={renderMarkdown(evt.content || '')}
                        />
                      </Match>

                      <Match when={evt.type === 'approval' && evt.approval}>
                        <div class="my-4">
                          <ApprovalCard
                            approval={evt.approval!}
                            onApprove={() => props.onApprove(evt.approval!)}
                            onSkip={() => props.onSkip(evt.approval!.toolId)}
                          />
                        </div>
                      </Match>

                      <Match when={evt.type === 'question' && evt.question}>
                        <div class="my-4">
                          <QuestionCard
                            question={evt.question!}
                            onAnswer={(answers) => props.onAnswerQuestion(evt.question!, answers)}
                            onSkip={() => props.onSkipQuestion(evt.question!.questionId)}
                          />
                        </div>
                      </Match>
                    </Switch>
                  )}
                </For>
              </Show>

              {/* Fallback */}
              <Show when={props.message.content && !hasStreamEvents()}>
                <div
                  class={markdownClass}
                  // eslint-disable-next-line solid/no-innerhtml
                  innerHTML={renderMarkdown(props.message.content)}
                />
              </Show>

              {/* Error block - distinct, recoverable */}
              <Show when={props.message.error}>
                <div
                  class="mt-2 flex items-start gap-2.5 rounded-md border border-red-200 dark:border-red-900/60 bg-red-50 dark:bg-red-950/30 px-3 py-2.5"
                  role="alert"
                >
                  <CircleAlertIcon class="mt-0.5 h-4 w-4 shrink-0 text-red-500 dark:text-red-400" />
                  <div class="flex-1 min-w-0">
                    <p class="text-sm text-red-700 dark:text-red-300">{props.message.error}</p>
                    <Show when={props.onRetry}>
                      <button
                        type="button"
                        onClick={() => props.onRetry?.(props.message.id)}
                        class="mt-2 inline-flex items-center gap-1.5 rounded-md border border-red-300 dark:border-red-800 px-2 py-1 text-xs font-medium text-red-700 dark:text-red-300 transition-colors hover:bg-red-100 dark:hover:bg-red-900/40"
                      >
                        <RotateCcwIcon class="h-3.5 w-3.5" />
                        Try again
                      </button>
                    </Show>
                  </div>
                </div>
              </Show>

              {/* Streaming cursor */}
              <Show when={isStreamingText() && !isWaitingForFirstToken()}>
                <span class="inline-block w-1.5 h-4 ml-0.5 align-middle bg-blue-500 dark:bg-blue-400 animate-pulse rounded-full" />
              </Show>

              <Show when={!props.message.isStreaming && contextTools().length > 0}>
                <div class="mt-4 pt-3 border-t border-border-subtle flex flex-wrap gap-2">
                  <span class="text-[10px] uppercase font-semibold text-muted">
                    {AI_CHAT_CONTEXT_USED_LABEL}
                  </span>
                  <div class="flex flex-wrap gap-1.5">
                    {contextTools().map((name) => (
                      <span class="px-1.5 py-0.5 rounded text-[10px] bg-surface-hover text-muted border border-border font-medium">
                        {formatIdentifierLabel(name, { stripPrefix: 'pulse_' })}
                      </span>
                    ))}
                  </div>
                </div>
              </Show>

              <Show when={props.message.tokens && !props.message.isStreaming}>
                <div class="mt-1 flex justify-end">
                  <span class="text-[9px] text-muted font-mono">
                    {props.message.tokens!.input} in · {props.message.tokens!.output} out
                  </span>
                </div>
              </Show>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};
