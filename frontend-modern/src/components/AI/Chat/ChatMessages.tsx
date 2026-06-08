import { Component, Show, For, createEffect, createMemo, createSignal } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import ArrowDownIcon from 'lucide-solid/icons/arrow-down';
import { MessageItem } from './MessageItem';
import type { ChatSession } from '@/api/aiChat';
import type { QueuedFollowUp } from './hooks/useChat';
import type {
  ChatMessage,
  ModelRouteRecoveryOption,
  PendingApproval,
  PendingTool,
  PendingQuestion,
  StreamDisplayEvent,
} from './types';
import { humanizeToken } from '@/utils/textPresentation';

const STICKY_SCROLL_BOTTOM_THRESHOLD_PX = 200;

interface ChatMessagesProps {
  messages: ChatMessage[];
  onApprove: (messageId: string, approval: PendingApproval) => void;
  onSkip: (messageId: string, toolId: string) => void;
  onAnswerQuestion: (
    messageId: string,
    question: PendingQuestion,
    answers: Array<{ id: string; value: string }>,
  ) => void;
  onSkipQuestion: (messageId: string, questionId: string) => void;
  onRetry?: (messageId: string) => void;
  onChangeModel?: () => void;
  getModelRouteLabel?: (modelId: string) => string;
  getModelRouteAlternative?: (message: ChatMessage) => ModelRouteRecoveryOption | null;
  onUseModelRoute?: (modelId: string, messageId?: string) => void;
  queuedFollowUps?: QueuedFollowUp[];
  queuedFollowUpsPaused?: boolean;
  onEditQueuedFollowUp?: (id: string) => void;
  onCancelQueuedFollowUp?: (id: string) => void;
  // Dashboard props
  recentSessions?: ChatSession[];
  onLoadSession?: (sessionId: string) => void;
}

/**
 * ChatMessages - Renders the scrollable message list.
 *
 * Features:
 * - Auto-scroll to bottom on new messages
 * - Recent-session resume actions
 * - Smooth scrolling behavior
 */
export const ChatMessages: Component<ChatMessagesProps> = (props) => {
  let messagesEndRef: HTMLDivElement | undefined;
  let containerRef: HTMLDivElement | undefined;
  const [isPinnedToBottom, setIsPinnedToBottom] = createSignal(true);

  // useChat hands us a fresh, immutably-rebuilt message array on every stream
  // event (each content chunk, workflow-status change, tool update spreads a new
  // message object). Rendering that array directly through <For>, which keys by
  // object reference, tears down and recreates the whole MessageItem on every
  // event — the visible flashing / rows popping in and out and the transcript
  // jumping up and down during a turn.
  //
  // Reconcile the incoming array into a keyed store mirror so each message keeps
  // a stable identity across updates (matched by id). MessageItem already reads
  // every field through `() => props.message.x` accessors, so once it stops
  // re-mounting, only the genuinely changed text/rows update in place. This keeps
  // the streaming transcript stable the way OpenCode's timeline is.
  const [mirroredMessages, setMirroredMessages] = createStore<ChatMessage[]>([]);
  createEffect(() => {
    setMirroredMessages(reconcile(props.messages, { key: 'id', merge: false }));
  });

  const isContainerNearBottom = () => {
    if (!containerRef) return true;
    const { scrollTop, scrollHeight, clientHeight } = containerRef;
    return scrollHeight - scrollTop - clientHeight < STICKY_SCROLL_BOTTOM_THRESHOLD_PX;
  };

  const updatePinnedToBottom = () => {
    setIsPinnedToBottom(isContainerNearBottom());
  };

  const jumpToLatest = () => {
    setIsPinnedToBottom(true);
    messagesEndRef?.scrollIntoView({ behavior: 'smooth' });
  };

  const textActivityFingerprint = (value?: string) =>
    value ? `${value.length}:${value.slice(-32)}` : '0:';

  const pendingToolActivityFingerprint = (tool?: PendingTool) =>
    [
      tool?.id,
      tool?.name,
      tool?.status,
      tool?.progress,
      textActivityFingerprint(tool?.input),
      textActivityFingerprint(tool?.rawInput),
      tool?.startedAt,
      tool?.updatedAt,
    ]
      .map((value) => String(value ?? ''))
      .join(':');

  const streamEventActivityFingerprint = (event: StreamDisplayEvent) => {
    const base = [event.type, event.toolId, event.startedAt, event.updatedAt];
    switch (event.type) {
      case 'pending_tool':
        return [...base, pendingToolActivityFingerprint(event.pendingTool)].join(':');
      case 'tool':
        return [
          ...base,
          event.tool?.name,
          event.tool?.success,
          textActivityFingerprint(event.tool?.input),
          textActivityFingerprint(event.tool?.rawInput),
          textActivityFingerprint(event.tool?.output),
        ].join(':');
      case 'tool_cancel':
        return [
          ...base,
          event.toolCancel?.id,
          event.toolCancel?.name,
          textActivityFingerprint(event.toolCancel?.input),
          textActivityFingerprint(event.toolCancel?.rawInput),
          textActivityFingerprint(event.toolCancel?.reason),
        ].join(':');
      case 'content':
        return [...base, textActivityFingerprint(event.content)].join(':');
      case 'thinking':
        return [...base, textActivityFingerprint(event.thinking)].join(':');
      case 'model_switch':
        return [...base, event.model, event.failedModel, event.modelEvent].join(':');
      case 'approval':
        return [
          ...base,
          event.approval?.toolId,
          event.approval?.toolName,
          event.approval?.isExecuting,
          event.approval?.approvalId,
        ].join(':');
      case 'question':
        return [
          ...base,
          event.question?.questionId,
          event.question?.isAnswering,
          event.question?.questions.length,
        ].join(':');
      default:
        return base.join(':');
    }
  };

  const messageActivityFingerprint = (message: ChatMessage) =>
    [
      message.id,
      message.role,
      message.isStreaming ? 'streaming' : 'idle',
      message.interruption,
      textActivityFingerprint(message.content),
      textActivityFingerprint(message.error),
      message.workflowStatus?.phase,
      message.workflowStatus?.message,
      message.workflowStatus?.tool,
      message.workflowStatus?.startedAt,
      ...(message.pendingTools || []).map(pendingToolActivityFingerprint),
      ...(message.streamEvents || []).map(streamEventActivityFingerprint),
    ]
      .map((value) => String(value ?? ''))
      .join('|');

  // Track content changes for auto-scroll (not just array length)
  // This tracks in-place streamed activity as well as message/event count so
  // patched tool progress remains visible while the transcript row stays put.
  const scrollTrigger = createMemo(() => {
    const msgs = props.messages;
    if (msgs.length === 0) return 0;
    const lastMsg = msgs[msgs.length - 1];
    return `${msgs.length}:${messageActivityFingerprint(lastMsg)}`;
  });

  const recentSessions = createMemo(() =>
    (props.recentSessions || []).filter((session) => session.message_count > 0).slice(0, 3),
  );
  const queuedFollowUpMetaByMessageId = createMemo(() => {
    const entries = props.queuedFollowUps || [];
    return new Map(
      entries.map((entry, index) => [
        entry.messageId,
        {
          id: entry.id,
          position: index + 1,
          count: entries.length,
          paused: Boolean(props.queuedFollowUpsPaused),
        },
      ]),
    );
  });

  const formatSessionMessageCount = (count: number) =>
    `${count} ${count === 1 ? 'message' : 'messages'}`;

  const formatSessionHandoffLabel = (session: ChatSession) => {
    const summary = session.handoff_summary;
    if (!summary) return '';
    const kind = summary.kind?.trim();
    if (kind) return humanizeToken(kind);
    if (summary.finding_id) return 'Patrol finding';
    if (summary.run_id) return 'Patrol run';
    return summary.has_model_context ? 'Context attached' : '';
  };

  // Auto-scroll to bottom on new messages or streaming content
  createEffect(() => {
    // Access the trigger to establish dependency (void suppresses unused var warning)
    void scrollTrigger();

    if (props.messages.length > 0 && messagesEndRef && containerRef) {
      if (isPinnedToBottom() || isContainerNearBottom()) {
        // Use instant scroll during active streaming for smoother experience
        const lastMsg = props.messages[props.messages.length - 1];
        const behavior = lastMsg.isStreaming ? 'instant' : 'smooth';
        messagesEndRef.scrollIntoView({ behavior: behavior as ScrollBehavior });
      }
    }
  });

  return (
    <div class="relative flex-1 min-h-0 bg-surface">
      <div
        ref={containerRef}
        class="h-full overflow-y-auto px-4 py-3 bg-surface"
        data-testid="assistant-message-list"
        onScroll={updatePinnedToBottom}
      >
        <Show
          when={props.messages.length === 0 && recentSessions().length > 0 && props.onLoadSession}
        >
          <section class="mb-3 w-full" aria-label="Recent Assistant sessions">
            <div class="mb-2 text-[11px] font-semibold uppercase text-muted">Recent sessions</div>
            <div class="space-y-1.5">
              <For each={recentSessions()}>
                {(session) => {
                  const handoffLabel = () => formatSessionHandoffLabel(session);
                  return (
                    <button
                      type="button"
                      class="w-full rounded-md border border-border bg-surface px-3 py-2 text-left transition-colors hover:border-blue-300 hover:bg-surface-alt focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                      onClick={() => props.onLoadSession?.(session.id)}
                      aria-label={`Resume ${session.title || 'Untitled Assistant session'}`}
                    >
                      <div class="truncate text-sm font-medium text-base-content">
                        {session.title || 'Untitled'}
                      </div>
                      <div class="mt-0.5 flex min-w-0 flex-wrap items-center gap-1.5 text-[11px] text-muted">
                        <span>{formatSessionMessageCount(session.message_count)}</span>
                        <Show when={handoffLabel()}>
                          {(label) => (
                            <>
                              <span aria-hidden="true">/</span>
                              <span class="truncate">{label()}</span>
                            </>
                          )}
                        </Show>
                      </div>
                    </button>
                  );
                }}
              </For>
            </div>
          </section>
        </Show>

        {/* Messages */}
        <For each={mirroredMessages}>
          {(message) => {
            const queuedMeta = createMemo(() => queuedFollowUpMetaByMessageId().get(message.id));
            return (
              <MessageItem
                message={message}
                onApprove={(approval) => props.onApprove(message.id, approval)}
                onSkip={(toolId) => props.onSkip(message.id, toolId)}
                onAnswerQuestion={(question, answers) =>
                  props.onAnswerQuestion(message.id, question, answers)
                }
                onSkipQuestion={(questionId) => props.onSkipQuestion(message.id, questionId)}
                onRetry={props.onRetry}
                onChangeModel={props.onChangeModel}
                getModelRouteLabel={props.getModelRouteLabel}
                modelRouteAlternative={props.getModelRouteAlternative?.(message)}
                onUseModelRoute={props.onUseModelRoute}
                queuedPosition={queuedMeta()?.position}
                queuedCount={queuedMeta()?.count}
                queuedPaused={queuedMeta()?.paused}
                onEditQueued={
                  queuedMeta() && props.onEditQueuedFollowUp
                    ? () => {
                        const meta = queuedMeta();
                        if (meta) props.onEditQueuedFollowUp?.(meta.id);
                      }
                    : undefined
                }
                onCancelQueued={
                  queuedMeta() && props.onCancelQueuedFollowUp
                    ? () => {
                        const meta = queuedMeta();
                        if (meta) props.onCancelQueuedFollowUp?.(meta.id);
                      }
                    : undefined
                }
              />
            );
          }}
        </For>

        {/* Scroll anchor */}
        <div ref={messagesEndRef} class="h-1" />
      </div>
      <Show when={props.messages.length > 0 && !isPinnedToBottom()}>
        <button
          type="button"
          class="absolute bottom-3 left-1/2 z-10 flex -translate-x-1/2 items-center gap-1.5 rounded-md border border-border bg-surface px-3 py-1.5 text-xs font-medium text-base-content shadow-lg transition-colors hover:bg-surface-alt focus:outline-none focus:ring-2 focus:ring-blue-500/40"
          onClick={jumpToLatest}
          aria-label="Jump to latest Assistant message"
        >
          <ArrowDownIcon class="h-3.5 w-3.5" aria-hidden="true" />
          <span>Latest</span>
        </button>
      </Show>
    </div>
  );
};
