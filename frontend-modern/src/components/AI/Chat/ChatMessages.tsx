import { Component, Show, For, createEffect, createMemo } from 'solid-js';
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
      // Only auto-scroll if user is near the bottom (within 200px)
      const { scrollTop, scrollHeight, clientHeight } = containerRef;
      const isNearBottom = scrollHeight - scrollTop - clientHeight < 200;

      if (isNearBottom) {
        // Use instant scroll during active streaming for smoother experience
        const lastMsg = props.messages[props.messages.length - 1];
        const behavior = lastMsg.isStreaming ? 'instant' : 'smooth';
        messagesEndRef.scrollIntoView({ behavior: behavior as ScrollBehavior });
      }
    }
  });

  return (
    <div ref={containerRef} class="flex-1 overflow-y-auto px-4 py-3 bg-surface">
      <Show when={props.messages.length === 0 && recentSessions().length > 0 && props.onLoadSession}>
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
      <For each={props.messages}>
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
  );
};
