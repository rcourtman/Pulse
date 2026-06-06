import {
  Component,
  Show,
  For,
  Switch,
  Match,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
} from 'solid-js';
import CheckIcon from 'lucide-solid/icons/check';
import CircleAlertIcon from 'lucide-solid/icons/circle-alert';
import ClockIcon from 'lucide-solid/icons/clock';
import CopyIcon from 'lucide-solid/icons/copy';
import CpuIcon from 'lucide-solid/icons/cpu';
import PencilIcon from 'lucide-solid/icons/pencil';
import RotateCcwIcon from 'lucide-solid/icons/rotate-ccw';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import XIcon from 'lucide-solid/icons/x';
import { renderMarkdown } from '../aiChatUtils';
import { PendingToolBlock, ToolExecutionBlock } from './ToolExecutionBlock';
import { ApprovalCard } from './ApprovalCard';
import { QuestionCard } from './QuestionCard';
import { ThinkingBlock } from './ThinkingBlock';
import { stripAssistantOutputArtifacts } from './assistantOutputHygiene';
import { groupStreamEventsForDisplay } from './streamEventGrouping';
import type {
  ChatMessage,
  ModelRouteRecoveryOption,
  PendingApproval,
  PendingQuestion,
  StreamDisplayEvent,
  WorkflowStatus,
} from './types';
import {
  AI_CHAT_ASSISTANT_MESSAGE_LABEL,
  AI_CHAT_CONTEXT_USED_LABEL,
} from '@/utils/aiChatPresentation';
import { formatAIModelRouteLabel } from '@/utils/aiProviderPresentation';
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
  onChangeModel?: () => void;
  getModelRouteLabel?: (modelId: string) => string;
  modelRouteAlternative?: ModelRouteRecoveryOption | null;
  onUseModelRoute?: (modelId: string, messageId?: string) => void;
  queuedPosition?: number;
  queuedCount?: number;
  onEditQueued?: () => void;
  onCancelQueued?: () => void;
}

const formatAssistantTurnDuration = (startedAt: Date, completedAt?: Date): string => {
  if (!completedAt) return '';
  const durationMs = completedAt.getTime() - startedAt.getTime();
  if (!Number.isFinite(durationMs) || durationMs < 0) return '';
  if (durationMs < 1000) return '<1s';

  const totalSeconds = Math.max(1, Math.round(durationMs / 1000));
  if (totalSeconds < 60) return `${totalSeconds}s`;

  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes < 60) return seconds ? `${minutes}m ${seconds}s` : `${minutes}m`;

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return remainingMinutes ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
};

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
  const isQueuedUserMessage = () => isUser() && props.message.delivery === 'queued';
  const queuedStatusLabel = createMemo(() => {
    if (!isQueuedUserMessage()) return '';
    const position = props.queuedPosition;
    const count = props.queuedCount;
    if (position && count && count > 1) {
      return `Queued ${position} of ${count}`;
    }
    return 'Queued';
  });

  // Group stream events into display blocks. Content collapses into a single
  // block even when a reasoning model interleaves hidden thinking deltas, so
  // the answer stays a coherent markdown document instead of fragmenting into
  // whitespace-trimmed pieces. See groupStreamEventsForDisplay for the rationale.
  const groupedEvents = createMemo(() =>
    groupStreamEventsForDisplay(props.message.streamEvents || []),
  );
  const isRenderableStreamEvent = (evt: StreamDisplayEvent) => {
    switch (evt.type) {
      case 'thinking':
        return !!evt.thinking?.trim();
      case 'content':
        return !!stripAssistantOutputArtifacts(evt.content || '').text;
      case 'tool':
        return !!evt.tool;
      case 'pending_tool':
        return !!evt.pendingTool;
      case 'model_switch':
        return !!evt.model?.trim();
      case 'approval':
        return !!evt.approval;
      case 'question':
        return !!evt.question;
      default:
        return false;
    }
  };
  const hasRenderableStreamEvents = () => groupedEvents().some(isRenderableStreamEvent);
  const isLeadingThinkingEvent = (index: number) =>
    groupedEvents()
      .slice(0, index)
      .every((evt) => evt.type === 'thinking');

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
  const visibleMessageContent = () =>
    stripAssistantOutputArtifacts(props.message.content || '').text;
  const modelRouteLabel = (route?: string) => {
    const model = route?.trim();
    if (!model) return '';
    return props.getModelRouteLabel?.(model) || formatAIModelRouteLabel(model);
  };
  const messageModelLabel = () => modelRouteLabel(props.message.model);
  const messageDurationLabel = () =>
    props.message.isStreaming
      ? ''
      : formatAssistantTurnDuration(props.message.timestamp, props.message.completedAt);

  // Check if currently streaming content (no tools pending, still streaming)
  const isStreamingText = () =>
    props.message.isStreaming &&
    (!props.message.pendingTools || props.message.pendingTools.length === 0);
  const isWaitingForFirstToken = () =>
    isStreamingText() &&
    !props.message.content.trim() &&
    !hasRenderableStreamEvents() &&
    !props.message.error;
  const [statusNow, setStatusNow] = createSignal(Date.now());
  createEffect(() => {
    const status = props.message.workflowStatus;
    if (!props.message.isStreaming || !status?.startedAt) return;
    setStatusNow(Date.now());
    const interval = window.setInterval(() => setStatusNow(Date.now()), 1000);
    onCleanup(() => window.clearInterval(interval));
  });
  const formatWorkflowStatus = (status?: WorkflowStatus, includeElapsed = false) => {
    const message = status?.message?.trim();
    if (!message) return '';
    const tool = status?.tool?.trim();
    const toolSuffix = tool && !message.includes(tool) ? ` · ${formatIdentifierLabel(tool)}` : '';
    let elapsedSuffix = '';
    if (includeElapsed && status?.startedAt) {
      const elapsedSeconds = Math.max(0, Math.floor((statusNow() - status.startedAt) / 1000));
      if (elapsedSeconds >= 2) {
        elapsedSuffix = ` (${elapsedSeconds}s)`;
      }
    }
    return `${message}${toolSuffix}${elapsedSuffix}`;
  };
  const workflowStatusText = createMemo(() =>
    formatWorkflowStatus(props.message.workflowStatus, true),
  );
  const inlineStreamingStatusText = createMemo(() => {
    return workflowStatusText() || 'Thinking...';
  });
  const shouldShowHeaderWorkflowStatus = () =>
    props.message.isStreaming &&
    !isWaitingForFirstToken() &&
    !visibleMessageContent().trim() &&
    !hasRenderableStreamEvents() &&
    !!workflowStatusText();
  const interruptionLabel = createMemo(() => {
    switch (props.message.interruption) {
      case 'replaced':
        return 'Stopped when you sent the next message';
      case 'stopped':
        return 'Stopped';
      default:
        return '';
    }
  });

  // Copy-to-clipboard for a completed assistant answer.
  const [copied, setCopied] = createSignal(false);
  const canCopy = () => !props.message.isStreaming && !!visibleMessageContent().trim();
  const copyMessage = async () => {
    const text = visibleMessageContent();
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
        <div
          class={`max-w-[85%] px-4 py-2.5 rounded-md rounded-br-sm shadow-sm ${
            isQueuedUserMessage()
              ? 'border border-blue-200 bg-blue-50 text-blue-950 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-100'
              : 'bg-blue-600 text-white'
          }`}
        >
          <p class="text-sm whitespace-pre-wrap">{props.message.content}</p>
          <Show when={isQueuedUserMessage()}>
            <div
              class="mt-1.5 flex flex-wrap items-center justify-end gap-1.5 text-[11px] font-medium text-blue-700 dark:text-blue-300"
              role="status"
            >
              <ClockIcon class="h-3 w-3" aria-hidden="true" />
              <span>{queuedStatusLabel()}</span>
              <Show when={props.onEditQueued}>
                <button
                  type="button"
                  onClick={() => props.onEditQueued?.()}
                  aria-label="Edit queued follow-up"
                  title="Edit queued follow-up"
                  class="inline-flex h-5 w-5 items-center justify-center rounded text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 focus:bg-blue-100 focus:outline-none focus:ring-2 focus:ring-blue-500/30 dark:text-blue-200 dark:hover:bg-blue-900/60"
                >
                  <PencilIcon class="h-3 w-3" aria-hidden="true" />
                </button>
              </Show>
              <Show when={props.onCancelQueued}>
                <button
                  type="button"
                  onClick={() => props.onCancelQueued?.()}
                  aria-label="Remove queued follow-up"
                  title="Remove queued follow-up"
                  class="inline-flex h-5 w-5 items-center justify-center rounded text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 focus:bg-blue-100 focus:outline-none focus:ring-2 focus:ring-blue-500/30 dark:text-blue-200 dark:hover:bg-blue-900/60"
                >
                  <XIcon class="h-3 w-3" aria-hidden="true" />
                </button>
              </Show>
            </div>
          </Show>
        </div>
      </Show>

      {/* Assistant message */}
      <Show when={!isUser()}>
        <div class="group flex w-full min-w-0 gap-3 px-1 py-2">
          <div class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border-subtle bg-surface-alt text-blue-600 shadow-sm dark:text-blue-400">
            <SparklesIcon class="h-3.5 w-3.5" />
          </div>

          <div class="min-w-0 flex-1">
            <div class="mb-2 flex min-h-7 min-w-0 items-center gap-2">
              <span class="shrink-0 text-xs font-semibold text-base-content">
                {AI_CHAT_ASSISTANT_MESSAGE_LABEL}
              </span>
              <Show when={messageModelLabel()}>
                <span
                  class="max-w-[12rem] truncate rounded border border-border-subtle bg-surface-alt px-1.5 py-0.5 text-[10px] font-medium text-muted"
                  title={props.message.model}
                >
                  {messageModelLabel()}
                </span>
              </Show>
              <Show when={messageDurationLabel()}>
                <span
                  class="inline-flex shrink-0 items-center gap-1 rounded border border-border-subtle bg-surface-alt px-1.5 py-0.5 text-[10px] font-medium text-muted"
                  title="Turn duration"
                  aria-label={`Turn duration ${messageDurationLabel()}`}
                >
                  <ClockIcon class="h-3 w-3" aria-hidden="true" />
                  {messageDurationLabel()}
                </span>
              </Show>
              <Show when={shouldShowHeaderWorkflowStatus()}>
                <span
                  class="inline-flex min-w-0 max-w-[18rem] items-center gap-1.5 rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200"
                  title={workflowStatusText()}
                  aria-live="polite"
                >
                  <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-blue-500 animate-pulse" />
                  <span class="truncate">{workflowStatusText()}</span>
                </span>
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
                  <span>{inlineStreamingStatusText()}</span>
                </div>
              </Show>

              {/* Stream events - chronological display */}
              <Show when={hasRenderableStreamEvents()}>
                <For each={groupedEvents()}>
                  {(evt, index) => (
                    <Switch>
                      <Match
                        when={
                          evt.type === 'thinking' &&
                          evt.thinking?.trim() &&
                          isLeadingThinkingEvent(index())
                        }
                      >
                        <ThinkingBlock
                          content={evt.thinking || ''}
                          isStreaming={props.message.isStreaming}
                          startedAt={evt.startedAt}
                          updatedAt={evt.updatedAt}
                        />
                      </Match>

                      <Match when={evt.type === 'pending_tool' && evt.pendingTool}>
                        <PendingToolBlock tool={evt.pendingTool!} />
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

                      <Match when={evt.type === 'model_switch' && evt.model?.trim()}>
                        <div
                          class="my-2 inline-flex max-w-full items-center gap-2 rounded-md border border-border-subtle bg-surface-alt px-2.5 py-1.5 text-xs text-muted"
                          role="status"
                          aria-label="Assistant model route changed"
                          title={evt.model}
                        >
                          <CpuIcon class="h-3.5 w-3.5 shrink-0 text-blue-500" aria-hidden="true" />
                          <span class="shrink-0">Switched to</span>
                          {' '}
                          <span class="truncate font-medium text-base-content">
                            {modelRouteLabel(evt.model)}
                          </span>
                        </div>
                      </Match>

                      {/* Content/text block */}
                      <Match
                        when={
                          evt.type === 'content' &&
                          stripAssistantOutputArtifacts(evt.content || '').text
                        }
                      >
                        <div
                          class={markdownClass}
                          // eslint-disable-next-line solid/no-innerhtml
                          innerHTML={renderMarkdown(
                            stripAssistantOutputArtifacts(evt.content || '').text,
                          )}
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
              <Show when={visibleMessageContent() && !hasRenderableStreamEvents()}>
                <div
                  class={markdownClass}
                  // eslint-disable-next-line solid/no-innerhtml
                  innerHTML={renderMarkdown(visibleMessageContent())}
                />
              </Show>

              <Show when={interruptionLabel()}>
                <div
                  class="mt-2 inline-flex items-center gap-1.5 rounded-md border border-border-subtle bg-surface-alt px-2 py-1 text-[11px] font-medium text-muted"
                  role="status"
                >
                  <span>{interruptionLabel()}</span>
                </div>
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
                    <div class="mt-2 flex flex-wrap gap-1.5">
                      <Show
                        when={
                          props.modelRouteAlternative && props.onUseModelRoute
                            ? props.modelRouteAlternative
                            : null
                        }
                      >
                        {(alternative) => (
                          <button
                            type="button"
                            onClick={() =>
                              props.onUseModelRoute?.(alternative().id, props.message.id)
                            }
                            aria-label={`Retry via ${alternative().providerLabel} provider route`}
                            title={alternative().label}
                            class="inline-flex max-w-[14rem] items-center gap-1.5 rounded-md border border-red-300 bg-white/80 px-2 py-1 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 dark:border-red-800 dark:bg-red-950/20 dark:text-red-300 dark:hover:bg-red-900/40"
                          >
                            <CpuIcon class="h-3.5 w-3.5" />
                            <span class="truncate">Retry via {alternative().providerLabel}</span>
                          </button>
                        )}
                      </Show>
                      <Show when={props.onChangeModel}>
                        <button
                          type="button"
                          onClick={() => props.onChangeModel?.()}
                          class="inline-flex items-center gap-1.5 rounded-md border border-red-300 bg-white/80 px-2 py-1 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 dark:border-red-800 dark:bg-red-950/20 dark:text-red-300 dark:hover:bg-red-900/40"
                        >
                          <CpuIcon class="h-3.5 w-3.5" />
                          Change model
                        </button>
                      </Show>
                      <Show when={props.onRetry}>
                        <button
                          type="button"
                          onClick={() => props.onRetry?.(props.message.id)}
                          class="inline-flex items-center gap-1.5 rounded-md border border-red-300 px-2 py-1 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 dark:border-red-800 dark:text-red-300 dark:hover:bg-red-900/40"
                        >
                          <RotateCcwIcon class="h-3.5 w-3.5" />
                          Try again
                        </button>
                      </Show>
                    </div>
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
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};
