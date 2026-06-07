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
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import CircleAlertIcon from 'lucide-solid/icons/circle-alert';
import ClockIcon from 'lucide-solid/icons/clock';
import CopyIcon from 'lucide-solid/icons/copy';
import CpuIcon from 'lucide-solid/icons/cpu';
import PencilIcon from 'lucide-solid/icons/pencil';
import RotateCcwIcon from 'lucide-solid/icons/rotate-ccw';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import XIcon from 'lucide-solid/icons/x';
import { renderMarkdown } from '../aiChatUtils';
import { PendingToolBlock, ToolCancellationBlock, ToolExecutionBlock } from './ToolExecutionBlock';
import { ApprovalCard } from './ApprovalCard';
import { QuestionCard } from './QuestionCard';
import { ThinkingBlock } from './ThinkingBlock';
import { getAssistantAnswerText } from './assistantAnswerText';
import { stripAssistantOutputArtifacts } from './assistantOutputHygiene';
import { formatAssistantWorkflowStatus, isInitialRequestStartStatus } from './activeTurnStatus';
import { groupStreamEventsForDisplay } from './streamEventGrouping';
import {
  latestWorkflowStatus,
  normalizeWorkflowStatusSequence,
} from './workflowStatusPresentation';
import {
  isPlaceholderToolInputSummary,
  parseToolInputSummary,
  toolValueText,
} from './toolPresentation';
import type {
  ChatMessage,
  ModelRouteRecoveryOption,
  PendingApproval,
  PendingQuestion,
  PendingTool,
  StreamDisplayEvent,
  ToolExecution,
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

const TEXT_RENDER_PACE_MS = 24;
const TEXT_RENDER_SNAP = /[\s.,!?;:)\]]/;

type ContextToolStreamEvent =
  | (StreamDisplayEvent & { type: 'pending_tool'; pendingTool: PendingTool })
  | (StreamDisplayEvent & { type: 'tool'; tool: ToolExecution });

type DisplayStreamItem =
  | { kind: 'event'; event: StreamDisplayEvent }
  | { kind: 'context_tool_group'; events: ContextToolStreamEvent[]; key: string };

const CONTEXT_TOOL_NAMES = new Set([
  'read',
  'query',
  'fetch_url',
  'get_infrastructure_state',
  'get_active_alerts',
  'get_metrics',
  'get_metrics_history',
  'get_baselines',
  'get_patterns',
  'get_disk_health',
  'get_storage',
  'get_storage_config',
  'get_resource_details',
]);

const normalizedContextToolName = (name?: string) => name?.trim().replace(/^pulse_/, '') || '';

const isContextToolName = (name?: string) =>
  CONTEXT_TOOL_NAMES.has(normalizedContextToolName(name));

const asContextToolStreamEvent = (event: StreamDisplayEvent): ContextToolStreamEvent | null => {
  if (
    event.type === 'pending_tool' &&
    event.pendingTool &&
    isContextToolName(event.pendingTool.name)
  ) {
    return event as ContextToolStreamEvent;
  }
  if (event.type === 'tool' && event.tool && isContextToolName(event.tool.name)) {
    return event as ContextToolStreamEvent;
  }
  return null;
};

const contextToolEventKey = (event: ContextToolStreamEvent) =>
  [
    event.type,
    event.toolId,
    event.type === 'pending_tool' ? event.pendingTool.id : event.tool.name,
    event.startedAt,
    event.updatedAt,
  ]
    .map((value) => String(value ?? ''))
    .join(':');

const groupContextToolStreamItems = (events: StreamDisplayEvent[]): DisplayStreamItem[] => {
  const items: DisplayStreamItem[] = [];
  let pendingGroup: ContextToolStreamEvent[] = [];

  const flushGroup = () => {
    if (pendingGroup.length >= 2) {
      items.push({
        kind: 'context_tool_group',
        events: pendingGroup,
        key: `context-tool:${pendingGroup.map(contextToolEventKey).join('|')}`,
      });
    } else {
      for (const event of pendingGroup) {
        items.push({ kind: 'event', event });
      }
    }
    pendingGroup = [];
  };

  for (const event of events) {
    const contextToolEvent = asContextToolStreamEvent(event);
    if (contextToolEvent) {
      pendingGroup.push(contextToolEvent);
      continue;
    }

    flushGroup();
    items.push({ kind: 'event', event });
  }

  flushGroup();
  return items;
};

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
  const initialValue =
    live() &&
    cachedText &&
    initialText.startsWith(cachedText) &&
    cachedText.length < initialText.length
      ? cachedText
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

  return (
    <div
      class={markdownClass}
      aria-live={props.streaming ? 'polite' : undefined}
      // eslint-disable-next-line solid/no-innerhtml
      innerHTML={renderMarkdown(visibleText())}
    />
  );
};

const ContextToolActivityGroup: Component<{
  events: ContextToolStreamEvent[];
  live: boolean;
}> = (props) => {
  const [expanded, setExpanded] = createSignal(false);
  const active = createMemo(() => props.events.some((event) => event.type === 'pending_tool'));
  const count = createMemo(() => props.events.length);
  const countLabel = createMemo(() => `${count()} context ${count() === 1 ? 'check' : 'checks'}`);
  const statusLabel = createMemo(() => (active() ? 'Gathering context' : 'Context gathered'));
  const title = createMemo(() => `${statusLabel()} · ${countLabel()}`);
  const toggle = () => setExpanded((value) => !value);

  return (
    <div
      class="my-1 overflow-hidden rounded-md border border-blue-200 bg-blue-50/60 text-[11px] dark:border-blue-900/60 dark:bg-blue-950/20"
      data-testid="context-tool-group"
      role="group"
      aria-label={title()}
    >
      <button
        type="button"
        class="flex w-full min-w-0 items-center gap-2 px-2.5 py-2 text-left transition-colors hover:bg-blue-100/60 focus:outline-none focus:ring-2 focus:ring-blue-500/30 focus:ring-inset dark:hover:bg-blue-950/30"
        aria-expanded={expanded()}
        onClick={toggle}
      >
        <span
          class={`h-1.5 w-1.5 shrink-0 rounded-full ${
            active() ? 'animate-pulse bg-blue-500' : 'bg-emerald-500'
          }`}
          aria-hidden="true"
        />
        <span class="shrink-0 text-[10px] font-semibold uppercase tracking-wide text-muted">
          {statusLabel()}
        </span>
        <span class="min-w-0 truncate text-[12px] font-medium text-base-content">
          {countLabel()}
        </span>
        <ChevronRightIcon
          class={`ml-auto h-3.5 w-3.5 shrink-0 text-muted transition-transform ${
            expanded() ? 'rotate-90' : ''
          }`}
          aria-hidden="true"
        />
      </button>
      <Show when={expanded()}>
        <div class="border-t border-blue-200/70 px-2 py-1.5 dark:border-blue-900/60">
          <For each={props.events}>
            {(event) => {
              const pendingTool = event.type === 'pending_tool' ? event.pendingTool : undefined;
              const tool = event.type === 'tool' ? event.tool : undefined;

              return (
                <Switch>
                  <Match when={pendingTool}>
                    {(pending) => <PendingToolBlock tool={pending()} />}
                  </Match>
                  <Match when={tool}>
                    {(completedTool) => (
                      <ToolExecutionBlock
                        startedAt={event.startedAt}
                        completedAt={event.updatedAt}
                        live={props.live}
                        tool={{
                          name: completedTool().name || 'unknown',
                          input: completedTool().input || '{}',
                          rawInput: completedTool().rawInput,
                          output: completedTool().output || '',
                          success: completedTool().success ?? true,
                        }}
                      />
                    )}
                  </Match>
                </Switch>
              );
            }}
          </For>
        </div>
      </Show>
    </div>
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
  const displayStreamItems = createMemo(() => groupContextToolStreamItems(groupedEvents()));
  const hasContextToolActivityGroup = createMemo(() =>
    displayStreamItems().some((item) => item.kind === 'context_tool_group'),
  );
  const isSelectedModelRouteEvent = (evt: StreamDisplayEvent) =>
    evt.type === 'model_switch' && evt.modelEvent === 'selected' && !evt.failedModel?.trim();
  const isConcreteStreamActivity = (evt: StreamDisplayEvent) => {
    switch (evt.type) {
      case 'workflow_status':
        return (
          !isInitialRequestStartStatus(evt.workflowStatus) &&
          !!formatAssistantWorkflowStatus(evt.workflowStatus)
        );
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
  const hasConcreteStreamActivity = createMemo(() =>
    groupedEvents().some(isConcreteStreamActivity),
  );
  const shouldRenderWorkflowStatusEvent = (evt: StreamDisplayEvent) =>
    !!formatAssistantWorkflowStatus(evt.workflowStatus) &&
    (!isInitialRequestStartStatus(evt.workflowStatus) || !hasConcreteStreamActivity());
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

  const contextToolSummaries = createMemo(() => {
    const events = props.message.streamEvents || [];
    const summaries = new Set<string>();

    for (const evt of events) {
      if (evt.type === 'tool' && evt.tool?.name) {
        const summary = parseToolInputSummary(
          toolValueText(evt.tool.input),
          evt.tool.name,
          evt.tool.rawInput,
        );
        const label =
          summary && !isPlaceholderToolInputSummary(summary)
            ? summary
            : formatIdentifierLabel(evt.tool.name, { stripPrefix: 'pulse_' });
        summaries.add(label);
      }
    }

    return Array.from(summaries);
  });
  const visibleMessageContent = () =>
    stripAssistantOutputArtifacts(props.message.content || '').text;
  const modelRouteLabel = (route?: string) => {
    const model = route?.trim();
    if (!model) return '';
    return props.getModelRouteLabel?.(model) || formatAIModelRouteLabel(model);
  };
  const isProviderFallbackEvent = (event: StreamDisplayEvent) => {
    const model = event.model?.trim();
    const failed = event.failedModel?.trim();
    return !!model && !!failed && failed !== model;
  };
  const isSelectedModelEvent = (event: StreamDisplayEvent) =>
    event.modelEvent === 'selected' && !!event.model?.trim();
  const modelSwitchTitle = (event: StreamDisplayEvent) => {
    if (!isProviderFallbackEvent(event)) return modelRouteLabel(event.model);
    return `${modelRouteLabel(event.failedModel)} -> ${modelRouteLabel(event.model)}`;
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
  const workflowStatusText = createMemo(() =>
    formatWorkflowStatus(props.message.workflowStatus, true),
  );
  const workflowStatusSequence = (status?: WorkflowStatus) =>
    normalizeWorkflowStatusSequence([...(props.message.workflowStatusHistory || []), status]);
  const WorkflowStatusText: Component<{
    status?: WorkflowStatus;
    includeElapsed?: boolean;
  }> = (statusProps) => {
    const statuses = createMemo(() => workflowStatusSequence(statusProps.status));
    const displayedStatus = createMemo(() => latestWorkflowStatus(statuses()));
    const text = createMemo(() =>
      formatWorkflowStatus(displayedStatus(), statusProps.includeElapsed),
    );
    return <>{text()}</>;
  };
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
  const copyableMessageText = () => getAssistantAnswerText(props.message);
  const canCopy = () => !props.message.isStreaming && !!copyableMessageText();
  const copyMessage = async () => {
    const text = copyableMessageText();
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
                  <span class="truncate">
                    <WorkflowStatusText status={props.message.workflowStatus} includeElapsed />
                  </span>
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
                  <Show when={workflowStatusText()} fallback={<span>Thinking...</span>}>
                    <span>
                      <WorkflowStatusText status={props.message.workflowStatus} includeElapsed />
                    </span>
                  </Show>
                </div>
              </Show>

              {/* Stream events - chronological display */}
              <Show when={hasRenderableStreamEvents()}>
                <For each={displayStreamItems()}>
                  {(item, index) => {
                    const contextGroup = item.kind === 'context_tool_group' ? item : undefined;
                    const evt = item.kind === 'event' ? item.event : undefined;
                    const contentText = () =>
                      stripAssistantOutputArtifacts(evt?.content || '').text;

                    return (
                      <Switch>
                        <Match when={contextGroup}>
                          {(group) => (
                            <ContextToolActivityGroup
                              events={group().events}
                              live={props.message.isStreaming === true}
                            />
                          )}
                        </Match>
                        <Match
                          when={
                            evt?.type === 'thinking' &&
                            evt.thinking?.trim() &&
                            isLeadingThinkingEvent(index())
                          }
                        >
                          <ThinkingBlock
                            content={evt?.thinking || ''}
                            isStreaming={props.message.isStreaming}
                            startedAt={evt?.startedAt}
                            updatedAt={evt?.updatedAt}
                          />
                        </Match>

                        <Match
                          when={
                            evt?.type === 'workflow_status' && shouldRenderWorkflowStatusEvent(evt)
                          }
                        >
                          <div
                            class="my-1 inline-flex max-w-full items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-2.5 py-1.5 text-xs font-medium text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200"
                            role="status"
                            aria-live="polite"
                            title={formatWorkflowStatus(
                              evt?.workflowStatus,
                              props.message.isStreaming,
                            )}
                          >
                            <span
                              class="h-1.5 w-1.5 shrink-0 rounded-full bg-blue-500 animate-pulse"
                              aria-hidden="true"
                            />
                            <span class="min-w-0 truncate">
                              <WorkflowStatusText
                                status={evt?.workflowStatus}
                                includeElapsed={props.message.isStreaming}
                              />
                            </span>
                          </div>
                        </Match>

                        <Match when={evt?.type === 'pending_tool' ? evt.pendingTool : undefined}>
                          {(pendingTool) => <PendingToolBlock tool={pendingTool()} />}
                        </Match>

                        <Match when={evt?.type === 'tool_cancel' ? evt.toolCancel : undefined}>
                          {(toolCancel) => <ToolCancellationBlock tool={toolCancel()} />}
                        </Match>

                        <Match when={evt?.type === 'tool' ? evt.tool : undefined}>
                          {(tool) => (
                            <ToolExecutionBlock
                              startedAt={evt?.startedAt}
                              completedAt={evt?.updatedAt}
                              live={props.message.isStreaming}
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
                          when={evt?.type === 'model_switch' && evt.model?.trim() ? evt : undefined}
                        >
                          {(modelEvent) => {
                            const event = modelEvent();
                            return (
                              <div
                                class="my-2 inline-flex max-w-full items-center gap-2 rounded-md border border-border-subtle bg-surface-alt px-2.5 py-1.5 text-xs text-muted"
                                role="status"
                                aria-label={
                                  isProviderFallbackEvent(event)
                                    ? 'Assistant provider fallback route changed'
                                    : isSelectedModelEvent(event)
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
                                  when={isProviderFallbackEvent(event)}
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
                                  <span class="shrink-0">Provider fallback</span>
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
                        <Match when={evt?.type === 'content' && contentText()}>
                          <AssistantMarkdownBlock
                            text={contentText()}
                            streaming={props.message.isStreaming}
                            paceKey={`${props.message.id}:stream:${index()}`}
                          />
                        </Match>

                        <Match when={evt?.type === 'approval' ? evt.approval : undefined}>
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

                        <Match when={evt?.type === 'question' ? evt.question : undefined}>
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
                </For>
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

              <Show
                when={
                  !props.message.isStreaming &&
                  !hasContextToolActivityGroup() &&
                  contextToolSummaries().length > 0
                }
              >
                <div class="mt-4 pt-3 border-t border-border-subtle flex flex-wrap gap-2">
                  <span class="text-[10px] uppercase font-semibold text-muted">
                    {AI_CHAT_CONTEXT_USED_LABEL}
                  </span>
                  <div class="flex flex-wrap gap-1.5">
                    {contextToolSummaries().map((summary) => (
                      <span
                        class="max-w-[18rem] truncate px-1.5 py-0.5 rounded text-[10px] bg-surface-hover text-muted border border-border font-medium"
                        title={summary}
                      >
                        {summary}
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
