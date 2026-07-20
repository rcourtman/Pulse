import {
  Component,
  Show,
  Index,
  Switch,
  Match,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
} from 'solid-js';
import CircleAlertIcon from 'lucide-solid/icons/circle-alert';
import ClockIcon from 'lucide-solid/icons/clock';
import CpuIcon from 'lucide-solid/icons/cpu';
import PencilIcon from 'lucide-solid/icons/pencil';
import RotateCcwIcon from 'lucide-solid/icons/rotate-ccw';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import XIcon from 'lucide-solid/icons/x';
import { renderMarkdown } from '../aiChatUtils';
import { highlightSettledCodeBlocks } from './aiCodeHighlight';
import { morphMarkdownInto } from './markdownMorph';
import { PendingToolBlock, ToolCancellationBlock, ToolExecutionBlock } from './ToolExecutionBlock';
import { ApprovalCard } from './ApprovalCard';
import { QuestionCard } from './QuestionCard';
import { ThinkingBlock } from './ThinkingBlock';
import { getAssistantAnswerText } from './assistantAnswerText';
import { stripAssistantOutputArtifacts } from './assistantOutputHygiene';
import { formatAssistantTurnDuration } from './assistantTurnSummary';
import { assistantWorkflowStatusKind, formatAssistantWorkflowStatus } from './activeTurnStatus';
import { groupStreamEventsForDisplay } from './streamEventGrouping';
import { createPacedWorkflowStatus, WORKFLOW_STATUS_REFRESH_MS } from './workflowStatusDisplay';
import type {
  ChatMessage,
  ModelRouteRecoveryOption,
  PendingApproval,
  PendingQuestion,
  StreamDisplayEvent,
  WorkflowStatus,
} from './types';
import { AI_CHAT_ASSISTANT_MESSAGE_LABEL } from '@/utils/aiChatPresentation';
import { formatAIModelRouteLabel } from '@/utils/aiProviderPresentation';
import { ActionIconButton, CopyValueButton } from '@/components/shared/Button';
import { copyToClipboard } from '@/utils/clipboard';

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
  // Provided only for the latest settled assistant answer: re-runs the turn in
  // place (same prompt, fresh generation).
  onRegenerate?: () => void;
  // Provided only for the user prompt of the latest turn: undoes the turn and
  // restores the prompt into the composer for editing.
  onEditPrompt?: () => void;
  onChangeModel?: () => void;
  getModelRouteLabel?: (modelId: string) => string;
  modelRouteAlternative?: ModelRouteRecoveryOption | null;
  onUseModelRoute?: (modelId: string, messageId?: string) => void;
  queuedPosition?: number;
  queuedCount?: number;
  queuedPaused?: boolean;
  // The follow-up was accepted for mid-turn steering and will join the
  // running response at its next step; edit/remove are no longer offered.
  queuedSteering?: boolean;
  onEditQueued?: () => void;
  onCancelQueued?: () => void;
}

const markdownClass =
  'text-sm prose prose-slate prose-sm dark:prose-invert max-w-none prose-p:leading-relaxed prose-p:my-2 prose-pre:bg-slate-900 prose-pre:text-slate-100 prose-pre:rounded-md prose-pre:text-xs prose-pre:border prose-pre:border-slate-800 prose-code:text-blue-700 dark:prose-code:text-blue-300 prose-code:bg-blue-50 dark:prose-code:bg-blue-900 prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded-md prose-code:font-mono prose-code:text-[0.9em] prose-code:border prose-code:border-blue-100 dark:prose-code:border-blue-800 prose-code:before:content-none prose-code:after:content-none prose-headings:font-semibold prose-hr:border-slate-200 dark:prose-hr:border-slate-700 prose-ul:my-2 prose-ol:my-2 prose-li:my-1';

const TEXT_RENDER_PACE_MS = 24;
const TEXT_RENDER_SNAP = /[\s.,!?;:)\]]/;

const textRenderStep = (size: number) => {
  if (size <= 12) return 2;
  if (size <= 48) return 4;
  if (size <= 96) return 8;
  return Math.min(24, Math.ceil(size / 8));
};

const nextPacedTextIndex = (text: string, start: number) => {
  const end = Math.min(text.length, start + textRenderStep(text.length - start));
  const max = Math.min(text.length, end + 8);
  for (let idx = end; idx < max; idx += 1) {
    if (TEXT_RENDER_SNAP.test(text[idx] || '')) return idx + 1;
  }
  return end;
};

const pacedTextCache = new Map<string, string>();
const pacedTextCleanupTimers = new Map<string, ReturnType<typeof setTimeout>>();

const cancelPacedTextCleanup = (key: string) => {
  const timer = pacedTextCleanupTimers.get(key);
  if (!timer) return;
  clearTimeout(timer);
  pacedTextCleanupTimers.delete(key);
};

const deletePacedTextCache = (key: string) => {
  cancelPacedTextCleanup(key);
  pacedTextCache.delete(key);
};

const schedulePacedTextCleanup = (key: string) => {
  cancelPacedTextCleanup(key);
  pacedTextCleanupTimers.set(
    key,
    setTimeout(() => {
      pacedTextCleanupTimers.delete(key);
      pacedTextCache.delete(key);
    }, 1000),
  );
};

const createPacedText = (getText: () => string, live: () => boolean, cacheKey: () => string) => {
  const initialText = getText();
  const initialKey = cacheKey();
  if (initialKey) {
    cancelPacedTextCleanup(initialKey);
  }
  const cachedText = initialKey ? pacedTextCache.get(initialKey) : undefined;
  const initialLiveText = () => {
    if (!initialText) return initialText;
    if (initialText.length <= 48) return initialText;
    const end = nextPacedTextIndex(initialText, 0);
    return end < initialText.length ? initialText.slice(0, end) : initialText;
  };
  const initialValue =
    live() &&
    cachedText &&
    initialText.startsWith(cachedText) &&
    cachedText.length <= initialText.length
      ? cachedText
      : live()
        ? initialLiveText()
        : initialText;
  const [value, setValue] = createSignal(initialValue);
  let shown = initialValue;
  let timeout: ReturnType<typeof setTimeout> | undefined;

  if (initialKey) {
    pacedTextCache.set(initialKey, initialValue);
  }

  const clear = () => {
    if (!timeout) return;
    clearTimeout(timeout);
    timeout = undefined;
  };

  const sync = (text: string) => {
    shown = text;
    setValue(text);
    const key = cacheKey();
    if (!key) return;
    if (live()) {
      cancelPacedTextCleanup(key);
      pacedTextCache.set(key, text);
    } else {
      deletePacedTextCache(key);
    }
  };

  const run = () => {
    timeout = undefined;
    const text = getText();
    if (!live()) {
      sync(text);
      return;
    }
    if (!text.startsWith(shown) || text.length <= shown.length) {
      sync(text);
      return;
    }
    const end = nextPacedTextIndex(text, shown.length);
    sync(text.slice(0, end));
    if (end < text.length) timeout = setTimeout(run, TEXT_RENDER_PACE_MS);
  };

  createEffect(() => {
    const text = getText();
    if (!live()) {
      clear();
      sync(text);
      return;
    }
    if (!text.startsWith(shown) || text.length < shown.length) {
      clear();
      sync(text);
      return;
    }
    if (text.length === shown.length || timeout) return;
    timeout = setTimeout(run, TEXT_RENDER_PACE_MS);
  });

  onCleanup(() => {
    clear();
    const key = cacheKey();
    if (key) schedulePacedTextCleanup(key);
  });

  return value;
};

const AssistantMarkdownBlock: Component<{
  text: string;
  streaming?: boolean;
  paceKey: string;
}> = (props) => {
  const visibleText = createPacedText(
    () => props.text,
    () => props.streaming === true,
    () => props.paceKey,
  );

  let container: HTMLDivElement | undefined;
  // Morph the rendered markdown into the container instead of replacing
  // innerHTML each tick. renderMarkdown already sanitizes (DOMPurify); the morph
  // keeps the stable prefix of blocks so a streaming answer doesn't rebuild and
  // reflow every earlier paragraph on every delta. See markdownMorph.ts.
  createEffect(() => {
    const html = renderMarkdown(visibleText());
    if (container) morphMarkdownInto(container, html);
    // Highlight only once the turn settles: re-highlighting per streaming
    // delta would fight the morph (it diffs against plain markup) and churn
    // the DOM. The highlighter runs over the sanitized DOM, so its spans
    // never pass through DOMPurify.
    if (container && props.streaming !== true) {
      void highlightSettledCodeBlocks(container);
    }
  });

  return (
    <div ref={container} class={markdownClass} aria-live={props.streaming ? 'polite' : undefined} />
  );
};

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
    if (props.queuedSteering) return 'Steering the running response';
    const position = props.queuedPosition;
    const count = props.queuedCount;
    const state = props.queuedPaused ? 'Paused' : 'Queued';
    if (position && count && count > 1) {
      return `${state} ${position} of ${count}`;
    }
    return state;
  });

  // Group stream events into display blocks. Content collapses into a single
  // block even when a reasoning model interleaves hidden thinking deltas, so
  // the answer stays a coherent markdown document instead of fragmenting into
  // whitespace-trimmed pieces. See groupStreamEventsForDisplay for the rationale.
  const groupedEvents = createMemo(() =>
    groupStreamEventsForDisplay(props.message.streamEvents || []),
  );
  const isSelectedModelRouteEvent = (evt: StreamDisplayEvent) =>
    evt.type === 'model_switch' && evt.modelEvent === 'selected' && !evt.failedModel?.trim();
  // A plain selected-route row repeats the route badge already shown in the
  // turn header. Keep it only when it adds information: the header has no
  // route, the route reads differently from the turn's final model, or a
  // later switch makes the starting route part of the turn's story. Compare
  // display labels, not raw ids — the done event can rewrite message.model to
  // a provider-reported id that differs in raw form while reading identically
  // to the user.
  const shouldRenderModelRouteEvent = (evt: StreamDisplayEvent) => {
    if (evt.type !== 'model_switch' || !evt.model?.trim()) return false;
    if (!isSelectedModelRouteEvent(evt)) return true;
    const hasRouteSwitch = (props.message.streamEvents || []).some(
      (event) =>
        event.type === 'model_switch' && !!event.model?.trim() && !isSelectedModelRouteEvent(event),
    );
    return (
      hasRouteSwitch || !messageModelLabel() || modelRouteLabel(evt.model) !== messageModelLabel()
    );
  };
  const isConcreteStreamActivity = (evt: StreamDisplayEvent) => {
    switch (evt.type) {
      case 'workflow_status':
        return !!formatAssistantWorkflowStatus(evt.workflowStatus);
      case 'thinking':
        return !!evt.thinking?.trim();
      case 'content':
        return !!stripAssistantOutputArtifacts(evt.content || '').text;
      case 'tool':
        return !!evt.tool;
      case 'pending_tool':
        return !!evt.pendingTool;
      case 'tool_cancel':
        return !!evt.toolCancel;
      case 'model_switch':
        return !!evt.model?.trim() && !isSelectedModelRouteEvent(evt);
      case 'approval':
        return !!evt.approval;
      case 'question':
        return !!evt.question;
      default:
        return false;
    }
  };
  // Live workflow status ("Preparing context", "Reading inventory", "Counting
  // results", retrying, ...) is transient working-state, not a transcript
  // artifact. OpenCode keeps that in a single pinned footer spinner and never
  // narrates it into the scrolling timeline; Pulse surfaces the same live status
  // in the activity dock above the composer (currentStatus in index.tsx). So the
  // transcript itself shows only durable artifacts — reasoning, tool calls, and
  // the answer — and never renders workflow_status rows. Keep the predicate so
  // the gating/grouping code paths stay intact, but never render the row.
  const shouldRenderWorkflowStatusEvent = (_evt: StreamDisplayEvent) => false;
  const isRenderableStreamEvent = (evt: StreamDisplayEvent) => {
    switch (evt.type) {
      case 'thinking':
        return !!evt.thinking?.trim();
      case 'workflow_status':
        return shouldRenderWorkflowStatusEvent(evt);
      case 'content':
        return !!stripAssistantOutputArtifacts(evt.content || '').text;
      case 'tool':
        return !!evt.tool;
      case 'pending_tool':
        return !!evt.pendingTool;
      case 'tool_cancel':
        return !!evt.toolCancel;
      case 'model_switch':
        return shouldRenderModelRouteEvent(evt);
      case 'approval':
        return !!evt.approval;
      case 'question':
        return !!evt.question;
      default:
        return false;
    }
  };
  const hasRenderableStreamEvents = () => groupedEvents().some(isRenderableStreamEvent);
  const hasLaterConcreteLiveActivity = (eventIndex: number) =>
    props.message.isStreaming === true &&
    groupedEvents()
      .slice(eventIndex + 1)
      .some((evt) => evt.type !== 'model_switch' && isConcreteStreamActivity(evt));
  const isDurableWorkflowBoundary = (evt: StreamDisplayEvent) => {
    switch (evt.type) {
      case 'content':
        return !!stripAssistantOutputArtifacts(evt.content || '').text;
      case 'tool':
        return !!evt.tool;
      case 'pending_tool':
        return !!evt.pendingTool;
      case 'tool_cancel':
        return !!evt.toolCancel;
      case 'approval':
        return !!evt.approval;
      case 'question':
        return !!evt.question;
      default:
        return false;
    }
  };
  const hasDurableWorkflowBoundaryAround = (eventIndex: number) =>
    groupedEvents().some((evt, index) => index !== eventIndex && isDurableWorkflowBoundary(evt));
  const liveWorkflowStatusHistory = createMemo(() =>
    props.message.isStreaming === true ? props.message.workflowStatusHistory || [] : [],
  );
  const workflowStatusPaceSequenceKey = createMemo(() => {
    const first = liveWorkflowStatusHistory()[0];
    return [props.message.id, first?.phase, first?.message, first?.startedAt]
      .map((value) => String(value ?? ''))
      .join(':');
  });
  const pacedWorkflowStatus = createPacedWorkflowStatus(
    liveWorkflowStatusHistory,
    () => props.message.isStreaming === true && liveWorkflowStatusHistory().length > 1,
    workflowStatusPaceSequenceKey,
  );
  const shouldPaceWorkflowStatusEvent = (evt: StreamDisplayEvent, eventIndex: number) =>
    props.message.isStreaming === true &&
    evt.type === 'workflow_status' &&
    liveWorkflowStatusHistory().length > 1 &&
    !hasDurableWorkflowBoundaryAround(eventIndex);
  const shouldCompactCompletedToolEvent = (evt: StreamDisplayEvent, eventIndex: number) =>
    evt.type === 'tool' &&
    props.message.isStreaming === true &&
    evt.tool?.success !== false &&
    hasLaterConcreteLiveActivity(eventIndex);
  const isLeadingThinkingEvent = (index: number) =>
    groupedEvents()
      .slice(0, index)
      .every((evt) => evt.type === 'thinking');

  const visibleMessageContent = () =>
    stripAssistantOutputArtifacts(props.message.content || '').text;
  const modelRouteLabel = (route?: string) => {
    const model = route?.trim();
    if (!model) return '';
    return props.getModelRouteLabel?.(model) || formatAIModelRouteLabel(model);
  };
  const modelRouteRecoveryButtonLabel = () => {
    const alternative = props.modelRouteAlternative;
    if (!alternative) return '';
    return alternative.kind === 'same-model-route'
      ? `Switch to ${alternative.providerLabel} route and retry`
      : `Switch to ${alternative.providerLabel} model route and retry`;
  };
  const hasPreviousModelRoute = (event: StreamDisplayEvent) => {
    const model = event.model?.trim();
    const failed = event.failedModel?.trim();
    return !!model && !!failed && failed !== model;
  };
  const isSelectedModelEvent = (event: StreamDisplayEvent) =>
    event.modelEvent === 'selected' && !!event.model?.trim();
  const modelSwitchTitle = (event: StreamDisplayEvent) => {
    if (!hasPreviousModelRoute(event)) return modelRouteLabel(event.model);
    return `${modelRouteLabel(event.failedModel)} -> ${modelRouteLabel(event.model)}`;
  };
  const messageModelLabel = () => modelRouteLabel(props.message.model);
  const queuedModelRouteLabel = () =>
    isQueuedUserMessage() ? modelRouteLabel(props.message.request?.model) : '';
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
  const currentWorkflowStatus = () => props.message.workflowStatus;
  const [statusNow, setStatusNow] = createSignal(Date.now());
  createEffect(() => {
    const status = currentWorkflowStatus();
    if (!props.message.isStreaming || !status?.startedAt) return;
    setStatusNow(Date.now());
    const interval = window.setInterval(() => setStatusNow(Date.now()), WORKFLOW_STATUS_REFRESH_MS);
    onCleanup(() => window.clearInterval(interval));
  });
  const formatWorkflowStatus = (status?: WorkflowStatus, includeElapsed = false) => {
    const message = formatAssistantWorkflowStatus(
      status,
      props.message.isStreaming ? statusNow() : undefined,
    );
    if (!message) return '';
    let elapsedSuffix = '';
    if (includeElapsed && status?.startedAt) {
      const elapsedSeconds = Math.max(0, Math.floor((statusNow() - status.startedAt) / 1000));
      if (elapsedSeconds >= 2) {
        elapsedSuffix = ` (${elapsedSeconds}s)`;
      }
    }
    return `${message}${elapsedSuffix}`;
  };
  const workflowStatusToneClass = (status?: WorkflowStatus) => {
    const kind = assistantWorkflowStatusKind(status);
    if (kind === 'retrying') {
      return 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200';
    }
    if (kind === 'tool') {
      return 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200';
    }
    return 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200';
  };
  const workflowStatusDotClass = (status?: WorkflowStatus) => {
    const kind = assistantWorkflowStatusKind(status);
    if (kind === 'retrying') return 'bg-amber-500';
    if (kind === 'tool') return 'bg-blue-500';
    return 'bg-blue-500';
  };
  const workflowStatusText = createMemo(() => formatWorkflowStatus(currentWorkflowStatus(), true));
  // Workflow status is footer-owned (the activity dock in index.tsx), never shown
  // in the transcript — not as a per-event row (shouldRenderWorkflowStatusEvent)
  // and not as this early-phase header chip. During the empty-content window the
  // transcript stays blank while the dock spinner conveys the live status, which
  // is the OpenCode model. Keep the predicate so its inputs stay referenced, but
  // never render the chip.
  const shouldShowHeaderWorkflowStatus = () =>
    props.message.isStreaming &&
    !isWaitingForFirstToken() &&
    !visibleMessageContent().trim() &&
    !hasRenderableStreamEvents() &&
    !!workflowStatusText() &&
    false;
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

  // Copy-to-clipboard for completed transcript messages.
  const [copied, setCopied] = createSignal(false);
  let copiedResetTimer: ReturnType<typeof setTimeout> | undefined;
  const copyButtonLabel = () => (copied() ? 'Copied message' : 'Copy message');
  const copyableMessageText = () => {
    if (isUser()) {
      const text = props.message.content || '';
      return text.trim() ? text : '';
    }
    return getAssistantAnswerText(props.message);
  };
  const canCopy = () => !props.message.isStreaming && !!copyableMessageText();
  const copyMessage = async (value: string) => {
    const text = value.trim();
    if (!text) return;
    const ok = await copyToClipboard(text);
    if (!ok) return;
    setCopied(true);
    if (copiedResetTimer) clearTimeout(copiedResetTimer);
    copiedResetTimer = setTimeout(() => setCopied(false), 1500);
  };
  onCleanup(() => {
    if (copiedResetTimer) clearTimeout(copiedResetTimer);
  });

  return (
    <div class={`${isUser() ? 'flex justify-end' : ''} mb-4`}>
      {/* User message - compact bubble */}
      <Show when={isUser()}>
        <div class="group flex max-w-[85%] items-start justify-end gap-2">
          <Show when={props.onEditPrompt}>
            <ActionIconButton
              onClick={() => props.onEditPrompt?.()}
              label="Edit and resend"
              title="Edit and resend"
              tone="outline"
              size="sm"
              class="mt-1 opacity-0 shadow-sm transition-opacity focus:opacity-100 group-hover:opacity-100"
            >
              <PencilIcon class="h-3.5 w-3.5" aria-hidden="true" />
            </ActionIconButton>
          </Show>
          <Show when={canCopy()}>
            <CopyValueButton
              value={copyableMessageText()}
              copied={copied()}
              onCopyValue={copyMessage}
              label={copyButtonLabel()}
              size="md"
              stopPropagation
              class="mt-1 opacity-0 shadow-sm transition-opacity focus:opacity-100 group-hover:opacity-100"
            />
          </Show>
          <div
            class={`min-w-0 px-4 py-2.5 rounded-md rounded-br-sm shadow-sm ${
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
                <Show when={queuedModelRouteLabel()}>
                  {(label) => (
                    <span
                      class="inline-flex max-w-[12rem] items-center rounded border border-blue-200 bg-white/80 px-1.5 py-0.5 text-[10px] font-semibold text-blue-800 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-200"
                      title={props.message.request?.model}
                    >
                      <span class="truncate">{label()}</span>
                    </span>
                  )}
                </Show>
                <Show when={props.onEditQueued}>
                  <ActionIconButton
                    onClick={() => props.onEditQueued?.()}
                    label="Edit queued follow-up"
                    tone="accentGhost"
                    size="2xs"
                  >
                    <PencilIcon class="h-3 w-3" aria-hidden="true" />
                  </ActionIconButton>
                </Show>
                <Show when={props.onCancelQueued}>
                  <ActionIconButton
                    onClick={() => props.onCancelQueued?.()}
                    label="Remove queued follow-up"
                    tone="accentGhost"
                    size="2xs"
                  >
                    <XIcon class="h-3 w-3" aria-hidden="true" />
                  </ActionIconButton>
                </Show>
              </div>
            </Show>
          </div>
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
                  class={`inline-flex min-w-0 max-w-[18rem] items-center gap-1.5 rounded border px-1.5 py-0.5 text-[10px] font-medium ${workflowStatusToneClass(
                    currentWorkflowStatus(),
                  )}`}
                  title={workflowStatusText()}
                  aria-live="polite"
                  data-status-kind={assistantWorkflowStatusKind(currentWorkflowStatus())}
                >
                  <span
                    class={`h-1.5 w-1.5 shrink-0 rounded-full animate-pulse ${workflowStatusDotClass(
                      currentWorkflowStatus(),
                    )}`}
                  />
                  <span class="truncate">{workflowStatusText()}</span>
                </span>
              </Show>
              <Show when={canCopy()}>
                <CopyValueButton
                  value={copyableMessageText()}
                  copied={copied()}
                  onCopyValue={copyMessage}
                  label={copyButtonLabel()}
                  size="md"
                  stopPropagation
                  class="ml-auto opacity-0 shadow-sm transition-opacity focus:opacity-100 group-hover:opacity-100"
                />
              </Show>
              <Show when={props.onRegenerate}>
                <ActionIconButton
                  onClick={() => props.onRegenerate?.()}
                  label="Regenerate response"
                  title="Regenerate response"
                  tone="outline"
                  size="sm"
                  class={`${canCopy() ? '' : 'ml-auto '}opacity-0 shadow-sm transition-opacity focus:opacity-100 group-hover:opacity-100`}
                >
                  <RotateCcwIcon class="h-3.5 w-3.5" aria-hidden="true" />
                </ActionIconButton>
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
                  <Show when={workflowStatusText()} fallback={<span>Thinking...</span>}>
                    <span>{workflowStatusText()}</span>
                  </Show>
                </div>
              </Show>

              {/* Stream events - chronological display */}
              <Show when={hasRenderableStreamEvents()}>
                <Index each={groupedEvents()}>
                  {(evt, index) => {
                    const contentText = () =>
                      stripAssistantOutputArtifacts(evt()?.content || '').text;
                    const visibleWorkflowStatus = () =>
                      evt()?.type === 'workflow_status'
                        ? shouldPaceWorkflowStatusEvent(evt(), index)
                          ? pacedWorkflowStatus() || evt().workflowStatus
                          : evt().workflowStatus
                        : undefined;

                    return (
                      <Switch>
                        <Match
                          when={
                            evt()?.type === 'thinking' &&
                            evt().thinking?.trim() &&
                            isLeadingThinkingEvent(index)
                          }
                        >
                          <ThinkingBlock
                            content={evt()?.thinking || ''}
                            isStreaming={props.message.isStreaming}
                            startedAt={evt()?.startedAt}
                            updatedAt={evt()?.updatedAt}
                          />
                        </Match>

                        <Match
                          when={
                            evt()?.type === 'workflow_status' &&
                            shouldRenderWorkflowStatusEvent(evt())
                          }
                        >
                          <div
                            class={`my-1 inline-flex max-w-full items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs font-medium ${workflowStatusToneClass(
                              visibleWorkflowStatus(),
                            )}`}
                            role="status"
                            aria-live="polite"
                            title={formatWorkflowStatus(
                              visibleWorkflowStatus(),
                              props.message.isStreaming,
                            )}
                            data-status-kind={assistantWorkflowStatusKind(visibleWorkflowStatus())}
                          >
                            <span
                              class={`h-1.5 w-1.5 shrink-0 rounded-full animate-pulse ${workflowStatusDotClass(
                                visibleWorkflowStatus(),
                              )}`}
                              aria-hidden="true"
                            />
                            <span class="min-w-0 truncate">
                              {formatWorkflowStatus(
                                visibleWorkflowStatus(),
                                props.message.isStreaming,
                              )}
                            </span>
                          </div>
                        </Match>

                        <Match
                          when={evt()?.type === 'pending_tool' ? evt().pendingTool : undefined}
                        >
                          {(pendingTool) => <PendingToolBlock tool={pendingTool()} />}
                        </Match>

                        <Match when={evt()?.type === 'tool_cancel' ? evt().toolCancel : undefined}>
                          {(toolCancel) => <ToolCancellationBlock tool={toolCancel()} />}
                        </Match>

                        <Match when={evt()?.type === 'tool' ? evt().tool : undefined}>
                          {(tool) => (
                            <ToolExecutionBlock
                              startedAt={evt()?.startedAt}
                              runningAt={evt()?.runningAt}
                              completedAt={evt()?.updatedAt}
                              live={props.message.isStreaming}
                              settleUntil={evt()?.settleUntil}
                              compact={shouldCompactCompletedToolEvent(evt(), index)}
                              tool={{
                                name: tool().name || 'unknown',
                                input: tool().input || '{}',
                                rawInput: tool().rawInput,
                                output: tool().output || '',
                                success: tool().success ?? true,
                              }}
                            />
                          )}
                        </Match>

                        <Match
                          when={evt() && shouldRenderModelRouteEvent(evt()) ? evt() : undefined}
                        >
                          {(modelEvent) => {
                            const event = modelEvent();
                            return (
                              <div
                                class="my-2 inline-flex max-w-full items-center gap-2 rounded-md border border-border-subtle bg-surface-alt px-2.5 py-1.5 text-xs text-muted"
                                role="status"
                                aria-label={
                                  isSelectedModelEvent(event)
                                    ? 'Assistant model route selected'
                                    : 'Assistant model route changed'
                                }
                                title={modelSwitchTitle(event)}
                              >
                                <CpuIcon
                                  class="h-3.5 w-3.5 shrink-0 text-blue-500"
                                  aria-hidden="true"
                                />
                                <Show
                                  when={hasPreviousModelRoute(event)}
                                  fallback={
                                    <>
                                      <Show when={isSelectedModelEvent(event)}>
                                        <span class="shrink-0">Using</span>{' '}
                                      </Show>
                                      <Show when={!isSelectedModelEvent(event)}>
                                        <span class="shrink-0">Switched to</span>{' '}
                                      </Show>
                                      <span class="truncate font-medium text-base-content">
                                        {modelRouteLabel(event.model)}
                                      </span>
                                    </>
                                  }
                                >
                                  <span class="shrink-0">Switched from</span>
                                  <span class="min-w-0 truncate font-medium text-base-content">
                                    {modelRouteLabel(event.failedModel)}
                                  </span>
                                  <span class="shrink-0 text-muted" aria-hidden="true">
                                    -&gt;
                                  </span>
                                  <span class="min-w-0 truncate font-medium text-base-content">
                                    {modelRouteLabel(event.model)}
                                  </span>
                                </Show>
                              </div>
                            );
                          }}
                        </Match>

                        {/* Content/text block */}
                        <Match when={evt()?.type === 'content' && contentText()}>
                          <AssistantMarkdownBlock
                            text={contentText()}
                            streaming={props.message.isStreaming}
                            paceKey={`${props.message.id}:stream:${index}`}
                          />
                        </Match>

                        <Match when={evt()?.type === 'approval' ? evt().approval : undefined}>
                          {(approval) => (
                            <div class="my-4">
                              <ApprovalCard
                                approval={approval()}
                                onApprove={() => props.onApprove(approval())}
                                onSkip={() => props.onSkip(approval().toolId)}
                              />
                            </div>
                          )}
                        </Match>

                        <Match when={evt()?.type === 'question' ? evt().question : undefined}>
                          {(question) => (
                            <div class="my-4">
                              <QuestionCard
                                question={question()}
                                onAnswer={(answers) => props.onAnswerQuestion(question(), answers)}
                                onSkip={() => props.onSkipQuestion(question().questionId)}
                              />
                            </div>
                          )}
                        </Match>
                      </Switch>
                    );
                  }}
                </Index>
              </Show>
              {/* Fallback */}
              <Show when={visibleMessageContent() && !hasRenderableStreamEvents()}>
                <AssistantMarkdownBlock
                  text={visibleMessageContent()}
                  streaming={props.message.isStreaming}
                  paceKey={`${props.message.id}:fallback`}
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
                            aria-label={modelRouteRecoveryButtonLabel()}
                            title={alternative().label}
                            class="inline-flex max-w-[14rem] items-center gap-1.5 rounded-md border border-red-300 bg-white/80 px-2 py-1 text-xs font-medium text-red-700 transition-colors hover:bg-red-100 dark:border-red-800 dark:bg-red-950/20 dark:text-red-300 dark:hover:bg-red-900/40"
                          >
                            <CpuIcon class="h-3.5 w-3.5" />
                            <span class="truncate">{modelRouteRecoveryButtonLabel()}</span>
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
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};
