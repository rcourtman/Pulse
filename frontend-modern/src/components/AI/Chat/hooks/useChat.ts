import { createSignal, onCleanup } from 'solid-js';
import {
  AIChatAPI,
  type ChatHandoffAction,
  type ChatHandoffMetadata,
  type ChatHandoffResource,
  type ChatMention,
  type StreamEvent,
  type ToolCall as PersistedToolCall,
} from '@/api/aiChat';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { normalizeChatToolName } from '@/utils/chatIdentifiers';
import { getAIProviderDisplayName, getProviderFromModelId } from '@/utils/aiProviderPresentation';
import {
  appendVisibleTextBeforeAssistantOutputArtifacts,
  createAssistantOutputArtifactStreamState,
  flushPendingAssistantOutputText,
  stripAssistantOutputArtifacts,
  type AssistantOutputArtifactStreamState,
} from '../assistantOutputHygiene';
import { isAssistantExplicitModelRoute } from '../assistantModelRoutes';
import { getAssistantFastToolCompletionSettleUntil } from '../streamActivityTiming';
import type {
  ChatMessage,
  ToolExecution,
  StreamDisplayEvent,
  PendingApproval,
  PendingQuestion,
  PendingTool,
  WorkflowStatus,
  ChatMessageRequestContext,
} from '../types';

const generateId = () => Math.random().toString(36).substring(2, 9);
type AssistantInterruption = NonNullable<ChatMessage['interruption']>;
const WORKFLOW_STATUS_HISTORY_LIMIT = 8;
const LOCAL_ASSISTANT_WAIT_STATUS_DELAY_MS = 900;

const messageModelCameFromRouteSwitch = (message: ChatMessage, modelRoute: string): boolean =>
  Boolean(
    message.streamEvents?.some((event) => {
      if (event.type !== 'model_switch') return false;
      if ((event.model?.trim() || '') !== modelRoute) return false;
      if (!event.failedModel?.trim()) return false;
      return event.modelEvent !== 'selected';
    }),
  );

export const latestExplicitModelRouteFromTranscript = (messages: ChatMessage[]): string => {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    const requestModel = message.request?.model?.trim() || '';
    if (requestModel && isAssistantExplicitModelRoute(requestModel)) {
      return requestModel;
    }

    const messageModel = message.model?.trim() || '';
    if (
      messageModel &&
      isAssistantExplicitModelRoute(messageModel) &&
      !messageModelCameFromRouteSwitch(message, messageModel)
    ) {
      return messageModel;
    }
  }

  return '';
};

const createInitialAssistantWorkflowStatus = (startedAt = Date.now()): WorkflowStatus => ({
  phase: 'request_send',
  message: 'Sending prompt.',
  startedAt,
});

const createLocalAssistantWaitWorkflowStatus = (startedAt = Date.now()): WorkflowStatus => ({
  phase: 'request_wait',
  message: 'Waiting for assistant.',
  startedAt,
});

export interface UseChatOptions {
  sessionId?: string;
  model?: string;
  defaultModel?: () => string;
  onConversationChanged?: () => void | Promise<void>;
}

export interface SendMessageOptions {
  model?: string;
  autonomousMode?: boolean;
  handoffContext?: string;
  handoffResources?: ChatHandoffResource[];
  handoffActions?: ChatHandoffAction[];
  handoffMetadata?: ChatHandoffMetadata;
}

export interface QueuedFollowUp {
  id: string;
  messageId: string;
  prompt: string;
  mentions?: ChatMention[];
  findingId?: string;
  sendOptions?: SendMessageOptions;
  timestamp: Date;
}

export interface RestoredPromptDraft {
  prompt: string;
  request?: ChatMessageRequestContext;
}

export interface RedoTurnResult {
  success: boolean;
  canRedo: boolean;
}

export function useChat(options: UseChatOptions = {}) {
  // Core state
  const [messages, setMessages] = createSignal<ChatMessage[]>([]);
  const [isLoading, setIsLoading] = createSignal(false);
  const [sessionId, setSessionId] = createSignal(options.sessionId || '');
  const [model, setModel] = createSignal(options.model || '');
  const [queuedFollowUps, setQueuedFollowUps] = createSignal<QueuedFollowUp[]>([]);
  const [queuedFollowUpsPaused, setQueuedFollowUpsPaused] = createSignal(false);

  const effectiveModelRoute = (sendOptions?: Pick<SendMessageOptions, 'model'>) => {
    const explicitModel = sendOptions?.model?.trim();
    if (explicitModel) return explicitModel;
    const selected = model().trim();
    if (selected) return selected;
    return options.defaultModel?.().trim() || '';
  };

  const notifyConversationChanged = async () => {
    if (!options.onConversationChanged) return;
    try {
      await options.onConversationChanged();
    } catch (error) {
      logger.warn('[useChat] Failed to refresh conversations:', error);
    }
  };

  // Abort controller for canceling requests
  let abortControllerRef: AbortController | null = null;
  let activeRequestId = 0;
  let pendingBackendAbort: Promise<void> | null = null;
  let localAssistantWaitStatusTimer: ReturnType<typeof setTimeout> | undefined;

  const clearLocalAssistantWaitStatusTimer = () => {
    if (localAssistantWaitStatusTimer === undefined) return;
    clearTimeout(localAssistantWaitStatusTimer);
    localAssistantWaitStatusTimer = undefined;
  };
  let isDrainingQueuedFollowUps = false;
  const suppressedRawContentMessageIds = new Set<string>();
  const outputArtifactStreamStates = new Map<string, AssistantOutputArtifactStreamState>();

  const outputArtifactStateFor = (assistantId: string) => {
    let state = outputArtifactStreamStates.get(assistantId);
    if (!state) {
      state = createAssistantOutputArtifactStreamState();
      outputArtifactStreamStates.set(assistantId, state);
    }
    return state;
  };

  const clearOutputArtifactState = (assistantId: string) => {
    outputArtifactStreamStates.delete(assistantId);
  };

  const clearSuppressedOutputBoundary = (assistantId: string) => {
    suppressedRawContentMessageIds.delete(assistantId);
    clearOutputArtifactState(assistantId);
  };

  const workflowStatusesMatch = (a?: WorkflowStatus, b?: WorkflowStatus) =>
    !!a &&
    !!b &&
    (a.phase || '') === (b.phase || '') &&
    a.message === b.message &&
    (a.state || '') === (b.state || '') &&
    (a.tool || '') === (b.tool || '') &&
    (a.provider || '') === (b.provider || '') &&
    (a.model || '') === (b.model || '') &&
    (a.attempt || 0) === (b.attempt || 0) &&
    (a.maxAttempts || 0) === (b.maxAttempts || 0) &&
    (a.retryAfterMs || 0) === (b.retryAfterMs || 0);

  const appendWorkflowStatusHistory = (
    history: WorkflowStatus[] | undefined,
    workflowStatus: WorkflowStatus,
  ): WorkflowStatus[] => {
    const current = history || [];
    const previous = current[current.length - 1];
    if (workflowStatusesMatch(previous, workflowStatus)) {
      return current;
    }
    return [...current, workflowStatus].slice(-WORKFLOW_STATUS_HISTORY_LIMIT);
  };

  const isStreamIdleWorkflowStatus = (status?: WorkflowStatus) => status?.phase === 'stream_idle';

  const workflowStatusProvider = (
    message: Pick<ChatMessage, 'model' | 'workflowStatus'> | undefined,
    status: WorkflowStatus | undefined,
  ) => {
    const provider = status?.provider?.trim() || message?.workflowStatus?.provider?.trim();
    if (provider) return provider;
    const model =
      status?.model?.trim() || message?.workflowStatus?.model?.trim() || message?.model?.trim();
    return model ? getProviderFromModelId(model) : '';
  };

  // stream_idle is transport liveness, but it must be visible as the latest
  // active progress state instead of leaving the user on a stale phase label.
  const visibleWorkflowStatusForHeartbeat = (
    message: Pick<ChatMessage, 'model' | 'workflowStatus'> | undefined,
    next: WorkflowStatus,
  ): WorkflowStatus => {
    if (!isStreamIdleWorkflowStatus(next)) return next;

    const provider = workflowStatusProvider(message, next);
    if (!provider) return next;

    return {
      ...next,
      provider,
      model:
        next.model?.trim() ||
        message?.workflowStatus?.model?.trim() ||
        message?.model?.trim() ||
        undefined,
      message: `${getAIProviderDisplayName(provider)} is still working; waiting for more response data.`,
    };
  };

  const setAssistantWorkflowStatus = (
    assistantId: string,
    requestId: number,
    workflowStatus: WorkflowStatus,
  ) => {
    if (requestId !== activeRequestId) return;
    setMessages((prev) =>
      prev.map((msg) => {
        if (msg.id !== assistantId) return msg;
        if (msg.isStreaming === false) return msg;
        const visibleStatus = visibleWorkflowStatusForHeartbeat(msg, workflowStatus);
        if (workflowStatusesMatch(msg.workflowStatus, visibleStatus)) return msg;
        return { ...msg, workflowStatus: visibleStatus };
      }),
    );
  };

  const abortBackendSession = (targetSessionId: string): Promise<void> | null => {
    const normalizedSessionId = targetSessionId.trim();
    if (!normalizedSessionId) return null;

    const promise = AIChatAPI.abortSession(normalizedSessionId)
      .catch((error) => {
        logger.warn('[useChat] Failed to abort chat session:', error);
      })
      .finally(() => {
        if (pendingBackendAbort === promise) {
          pendingBackendAbort = null;
        }
      });

    pendingBackendAbort = promise;
    return promise;
  };

  const isLocalPromptProgressEvent = (event: StreamDisplayEvent) =>
    event.type === 'workflow_status' &&
    (event.workflowStatus?.phase === 'request_send' ||
      event.workflowStatus?.phase === 'request_wait');

  const streamEventsWithoutLocalPromptProgressRows = (
    events: StreamDisplayEvent[] | undefined,
  ): StreamDisplayEvent[] => (events || []).filter((event) => !isLocalPromptProgressEvent(event));

  const streamEventsWithoutUnresolvedInteractiveRows = (
    events: StreamDisplayEvent[] | undefined,
  ): StreamDisplayEvent[] | undefined => {
    if (!events) return events;
    return streamEventsWithoutLocalPromptProgressRows(events).filter(
      (event) =>
        event.type !== 'workflow_status' &&
        event.type !== 'pending_tool' &&
        event.type !== 'approval' &&
        event.type !== 'question',
    );
  };

  const cancelActiveRequest = (interruption?: AssistantInterruption): Promise<void> | null => {
    const hasActiveRequest = isLoading() || abortControllerRef !== null;
    if (!hasActiveRequest) return null;

    const targetSessionId = sessionId();
    if (abortControllerRef) {
      abortControllerRef.abort();
      abortControllerRef = null;
    }
    clearLocalAssistantWaitStatusTimer();
    activeRequestId += 1;

    setMessages((prev) =>
      prev.map((msg) =>
        msg.isStreaming
          ? {
              ...msg,
              isStreaming: false,
              completedAt: new Date(),
              interruption,
              pendingTools: [],
              pendingApprovals: [],
              pendingQuestions: [],
              streamEvents: interruption
                ? streamEventsWithoutUnresolvedInteractiveRows(msg.streamEvents)
                : msg.streamEvents,
              workflowStatus: undefined,
              workflowStatusHistory: undefined,
            }
          : msg,
      ),
    );
    setIsLoading(false);

    return abortBackendSession(targetSessionId);
  };

  const removeQueuedMessages = (messageIds: Set<string>) => {
    if (messageIds.size === 0) return;
    setMessages((prev) =>
      prev.filter(
        (msg) => !(msg.role === 'user' && msg.delivery === 'queued' && messageIds.has(msg.id)),
      ),
    );
  };

  const cancelQueuedFollowUp = (id: string) => {
    const item = queuedFollowUps().find((entry) => entry.id === id);
    if (!item) return;
    setQueuedFollowUps((prev) => {
      const next = prev.filter((entry) => entry.id !== id);
      if (next.length === 0) {
        setQueuedFollowUpsPaused(false);
      }
      return next;
    });
    removeQueuedMessages(new Set([item.messageId]));
  };

  const takeQueuedFollowUp = (id: string): QueuedFollowUp | undefined => {
    const item = queuedFollowUps().find((entry) => entry.id === id);
    if (!item) return undefined;
    setQueuedFollowUps((prev) => {
      const next = prev.filter((entry) => entry.id !== id);
      if (next.length === 0) {
        setQueuedFollowUpsPaused(false);
      }
      return next;
    });
    removeQueuedMessages(new Set([item.messageId]));
    return item;
  };

  const moveQueuedMessageToFront = (messageId: string) => {
    setMessages((prev) => {
      const targetIndex = prev.findIndex(
        (msg) => msg.id === messageId && msg.role === 'user' && msg.delivery === 'queued',
      );
      if (targetIndex < 0) return prev;

      const target = prev[targetIndex];
      const withoutTarget = prev.filter((msg) => msg.id !== messageId);
      const firstQueuedIndex = withoutTarget.findIndex(
        (msg) => msg.role === 'user' && msg.delivery === 'queued',
      );
      if (firstQueuedIndex < 0) return prev;
      return [
        ...withoutTarget.slice(0, firstQueuedIndex),
        target,
        ...withoutTarget.slice(firstQueuedIndex),
      ];
    });
  };

  const promoteQueuedFollowUp = (id: string): boolean => {
    const item = queuedFollowUps().find((entry) => entry.id === id);
    if (!item) return false;

    setQueuedFollowUps((prev) => {
      const currentIndex = prev.findIndex((entry) => entry.id === id);
      if (currentIndex <= 0) return prev;
      return [prev[currentIndex], ...prev.slice(0, currentIndex), ...prev.slice(currentIndex + 1)];
    });
    moveQueuedMessageToFront(item.messageId);
    return true;
  };

  const clearQueuedFollowUps = () => {
    const messageIds = new Set(queuedFollowUps().map((entry) => entry.messageId));
    setQueuedFollowUps([]);
    setQueuedFollowUpsPaused(false);
    removeQueuedMessages(messageIds);
  };

  // Cleanup on unmount
  onCleanup(() => {
    void cancelActiveRequest();
  });

  // Stop/cancel current request
  const stop = () => {
    if (queuedFollowUps().length > 0) {
      setQueuedFollowUpsPaused(true);
    }
    void cancelActiveRequest('stopped');
  };

  // Helper to add stream event for chronological display
  const withStreamEventTiming = (
    event: StreamDisplayEvent,
    now = Date.now(),
  ): StreamDisplayEvent => {
    const startedAt = event.startedAt ?? event.pendingTool?.startedAt ?? now;
    const updatedAt = event.updatedAt ?? event.pendingTool?.updatedAt ?? startedAt;
    return {
      ...event,
      startedAt,
      updatedAt,
    };
  };

  const addStreamEvent = (msg: ChatMessage, event: StreamDisplayEvent): ChatMessage => {
    const nextEvent = withStreamEventTiming(event);
    const events =
      nextEvent.type === 'workflow_status' || nextEvent.type === 'model_switch'
        ? msg.streamEvents || []
        : streamEventsWithoutLocalPromptProgressRows(msg.streamEvents);

    // For content events, merge consecutive content into one
    if (nextEvent.type === 'content' && events.length > 0) {
      const last = events[events.length - 1];
      if (last.type === 'content') {
        const now = Date.now();
        return {
          ...msg,
          streamEvents: [
            ...events.slice(0, -1),
            {
              ...last,
              content: (last.content || '') + (nextEvent.content || ''),
              startedAt: last.startedAt || nextEvent.startedAt || now,
              updatedAt: nextEvent.updatedAt || now,
            },
          ],
        };
      }
    }

    // For thinking events, merge consecutive thinking into one
    if (nextEvent.type === 'thinking' && events.length > 0) {
      const last = events[events.length - 1];
      if (last.type === 'thinking') {
        const now = Date.now();
        return {
          ...msg,
          streamEvents: [
            ...events.slice(0, -1),
            {
              ...last,
              thinking: (last.thinking || '') + (nextEvent.thinking || ''),
              startedAt: last.startedAt || nextEvent.startedAt || now,
              updatedAt: nextEvent.updatedAt || now,
            },
          ],
        };
      }
    }

    return {
      ...msg,
      streamEvents: [...events, nextEvent],
    };
  };

  const withWorkflowStatusEvent = (
    msg: ChatMessage,
    workflowStatus: WorkflowStatus,
  ): ChatMessage => {
    const events = isLocalPromptProgressEvent({ type: 'workflow_status', workflowStatus })
      ? msg.streamEvents || []
      : streamEventsWithoutLocalPromptProgressRows(msg.streamEvents);
    const now = Date.now();
    const visibleStatus = visibleWorkflowStatusForHeartbeat(msg, workflowStatus);
    const nextEvent = withStreamEventTiming(
      {
        type: 'workflow_status',
        workflowStatus: visibleStatus,
        startedAt: visibleStatus.startedAt,
        updatedAt: workflowStatus.startedAt,
      },
      now,
    );
    const last = events[events.length - 1];

    if (last?.type === 'workflow_status') {
      if (workflowStatusesMatch(last.workflowStatus, visibleStatus)) {
        if (isStreamIdleWorkflowStatus(workflowStatus)) {
          return {
            ...msg,
            streamEvents: [
              ...events.slice(0, -1),
              {
                ...last,
                updatedAt: nextEvent.updatedAt || now,
              },
            ],
            workflowStatusHistory: appendWorkflowStatusHistory(
              msg.workflowStatusHistory,
              visibleStatus,
            ),
          };
        }
        return msg;
      }
      return {
        ...msg,
        streamEvents: [
          ...events.slice(0, -1),
          {
            ...nextEvent,
            startedAt: nextEvent.startedAt || last.startedAt,
          },
        ],
        workflowStatusHistory: appendWorkflowStatusHistory(
          msg.workflowStatusHistory,
          visibleStatus,
        ),
      };
    }

    const msgWithFilteredEvents = { ...msg, streamEvents: events };
    return {
      ...addStreamEvent(msgWithFilteredEvents, nextEvent),
      workflowStatusHistory: appendWorkflowStatusHistory(msg.workflowStatusHistory, visibleStatus),
    };
  };

  const streamModelEventKind = (event: StreamDisplayEvent): StreamDisplayEvent['modelEvent'] => {
    if (event.modelEvent === 'selected' || event.modelEvent === 'switch') return event.modelEvent;
    return 'switch';
  };

  const isDurableAssistantStreamBoundary = (event: StreamDisplayEvent): boolean =>
    event.type !== 'workflow_status' && event.type !== 'model_switch';

  const replaceableSelectedModelEventIndex = (events: StreamDisplayEvent[]): number => {
    for (let index = events.length - 1; index >= 0; index -= 1) {
      const event = events[index];
      if (isDurableAssistantStreamBoundary(event)) return -1;
      if (
        event.type === 'model_switch' &&
        streamModelEventKind(event) === 'selected' &&
        !event.failedModel?.trim()
      ) {
        return index;
      }
    }
    return -1;
  };

  const withModelRouteEvent = (
    msg: ChatMessage,
    route: string,
    options: { failedModel?: string; modelEvent: NonNullable<StreamDisplayEvent['modelEvent']> },
  ): ChatMessage => {
    const model = route.trim();
    if (!model) return msg;
    const failedModel = options.failedModel?.trim();
    const streamEvents = streamEventsWithoutLocalPromptProgressRows(msg.streamEvents);
    const duplicate = streamEvents.some(
      (event) =>
        event.type === 'model_switch' &&
        event.model?.trim() === model &&
        (event.failedModel?.trim() || '') === (failedModel || '') &&
        streamModelEventKind(event) === options.modelEvent,
    );
    if (duplicate) {
      return { ...msg, model, streamEvents };
    }
    const events = streamEvents;
    if (options.modelEvent === 'selected' && !failedModel) {
      const replaceableIndex = replaceableSelectedModelEventIndex(events);
      if (replaceableIndex >= 0) {
        const replacement = withStreamEventTiming({
          type: 'model_switch',
          model,
          modelEvent: 'selected',
        });
        return {
          ...msg,
          model,
          streamEvents: [
            ...events.slice(0, replaceableIndex),
            replacement,
            ...events.slice(replaceableIndex + 1),
          ],
        };
      }
    }
    const updated = addStreamEvent(msg, {
      type: 'model_switch',
      model,
      failedModel: failedModel || undefined,
      modelEvent: options.modelEvent,
    });
    return { ...updated, model };
  };

  const normalizePendingToolStatus = (phase?: string): PendingTool['status'] => {
    const normalized = (phase || 'running').trim().toLowerCase();
    if (normalized === 'pending') return 'pending';
    if (normalized === 'waiting') return 'waiting';
    return 'running';
  };

  const replacePendingToolStreamEvents = (
    events: StreamDisplayEvent[],
    resolvedTool: PendingTool,
    matchesTool: (tool?: PendingTool, toolId?: string) => boolean,
    now: number,
  ): StreamDisplayEvent[] => {
    let replacedEvent = false;
    const updatedEvents: StreamDisplayEvent[] = [];

    for (const event of events) {
      if (event.type !== 'pending_tool' || !matchesTool(event.pendingTool, event.toolId)) {
        updatedEvents.push(event);
        continue;
      }
      if (replacedEvent) {
        continue;
      }
      replacedEvent = true;
      updatedEvents.push(
        withStreamEventTiming(
          {
            ...event,
            toolId: resolvedTool.id,
            pendingTool: resolvedTool,
          },
          now,
        ),
      );
    }

    if (!replacedEvent) {
      updatedEvents.push(
        withStreamEventTiming(
          {
            type: 'pending_tool',
            pendingTool: resolvedTool,
            toolId: resolvedTool.id,
          },
          now,
        ),
      );
    }

    return updatedEvents;
  };

  const replacePendingToolWithCancelEvent = (
    events: StreamDisplayEvent[],
    canceledTool: PendingTool,
    reason: string | undefined,
    matchesTool: (tool?: PendingTool, toolId?: string) => boolean,
    now: number,
  ): StreamDisplayEvent[] => {
    let replacedEvent = false;
    const updatedEvents: StreamDisplayEvent[] = [];
    const cancelEvent = () =>
      withStreamEventTiming(
        {
          type: 'tool_cancel',
          toolId: canceledTool.id,
          toolCancel: {
            id: canceledTool.id,
            name: canceledTool.name,
            input: canceledTool.input,
            rawInput: canceledTool.rawInput,
            reason,
          },
          startedAt: canceledTool.startedAt,
          updatedAt: now,
        },
        now,
      );

    for (const event of events) {
      if (event.type !== 'pending_tool' || !matchesTool(event.pendingTool, event.toolId)) {
        updatedEvents.push(event);
        continue;
      }
      if (replacedEvent) {
        continue;
      }
      replacedEvent = true;
      updatedEvents.push(cancelEvent());
    }

    if (!replacedEvent) {
      updatedEvents.push(cancelEvent());
    }

    return updatedEvents;
  };

  const upsertPendingToolStart = (
    msg: ChatMessage,
    data: {
      id?: string;
      name?: string;
      input?: string;
      raw_input?: string;
    },
  ): ChatMessage => {
    const normalizedName = normalizeChatToolName(data.name || '');
    const pendingTools = msg.pendingTools || [];
    const now = Date.now();

    const matchesTool = (tool?: PendingTool, toolId?: string) => {
      if (!tool) return false;
      if (data.id && toolId === data.id) return true;
      if (data.id && tool.id === data.id) return true;
      return normalizedName !== '' && normalizeChatToolName(tool.name) === normalizedName;
    };

    let replacedTool = false;
    let resolvedTool: PendingTool | undefined;
    const updatedPendingTools: PendingTool[] = [];

    for (const tool of pendingTools) {
      if (!matchesTool(tool, tool.id)) {
        updatedPendingTools.push(tool);
        continue;
      }
      if (replacedTool) {
        continue;
      }
      replacedTool = true;
      resolvedTool = {
        ...tool,
        id: data.id || tool.id,
        name: data.name || tool.name,
        input: data.input || tool.input,
        rawInput: data.raw_input || tool.rawInput,
        status: tool.status || 'pending',
        progress: tool.progress,
        startedAt: tool.startedAt || now,
        updatedAt: now,
      };
      updatedPendingTools.push(resolvedTool);
    }

    if (!resolvedTool) {
      resolvedTool = {
        id: data.id || generateId(),
        name: data.name || 'unknown',
        input: data.input || '{}',
        rawInput: data.raw_input,
        status: 'pending',
        startedAt: now,
        updatedAt: now,
      };
      updatedPendingTools.push(resolvedTool);
    }

    return {
      ...msg,
      streamEvents: replacePendingToolStreamEvents(
        msg.streamEvents || [],
        resolvedTool,
        matchesTool,
        now,
      ),
      workflowStatus: undefined,
      workflowStatusHistory: undefined,
      pendingTools: updatedPendingTools,
    };
  };

  const updatePendingToolProgress = (
    msg: ChatMessage,
    data: {
      id?: string;
      name?: string;
      input?: string;
      raw_input?: string;
      phase?: string;
      message?: string;
    },
  ): ChatMessage => {
    const normalizedName = normalizeChatToolName(data.name || '');
    const status = normalizePendingToolStatus(data.phase);
    const progress = data.message?.trim() || undefined;
    const pendingTools = msg.pendingTools || [];
    const now = Date.now();

    const matchesTool = (tool?: PendingTool, toolId?: string) => {
      if (!tool) return false;
      if (data.id && toolId === data.id) return true;
      if (data.id && tool.id === data.id) return true;
      return normalizedName !== '' && normalizeChatToolName(tool.name) === normalizedName;
    };

    const mergeTool = (tool: PendingTool): PendingTool => ({
      ...tool,
      name: data.name || tool.name,
      input: data.input || tool.input,
      rawInput: data.raw_input || tool.rawInput,
      status,
      progress: progress || tool.progress,
      startedAt: tool.startedAt || now,
      updatedAt: now,
    });

    let resolvedTool: PendingTool | undefined;
    let replacedTool = false;
    const updatedPendingTools: PendingTool[] = [];

    for (const tool of pendingTools) {
      if (!matchesTool(tool, tool.id)) {
        updatedPendingTools.push(tool);
        continue;
      }
      if (replacedTool) {
        continue;
      }
      replacedTool = true;
      resolvedTool = mergeTool(tool);
      updatedPendingTools.push(resolvedTool);
    }

    if (!resolvedTool) {
      resolvedTool = {
        id: data.id || generateId(),
        name: data.name || 'unknown',
        input: data.input || '{}',
        rawInput: data.raw_input,
        status,
        progress,
        startedAt: now,
        updatedAt: now,
      };
      updatedPendingTools.push(resolvedTool);
    }

    return {
      ...msg,
      streamEvents: resolvedTool
        ? replacePendingToolStreamEvents(msg.streamEvents || [], resolvedTool, matchesTool, now)
        : msg.streamEvents,
      workflowStatus: undefined,
      workflowStatusHistory: undefined,
      pendingTools: updatedPendingTools,
    };
  };

  const appendMessageContent = (msg: ChatMessage, content: string): string => {
    const existing = msg.content || '';
    if (!existing || !content) {
      return existing + content;
    }

    const events = msg.streamEvents || [];
    const lastEvent = events[events.length - 1];
    if (!lastEvent || lastEvent.type === 'content') {
      return existing + content;
    }

    if (/\s$/.test(existing) || /^\s|^[,.;:!?)]/.test(content)) {
      return existing + content;
    }
    return `${existing} ${content}`;
  };

  const appendTextAfterBoundary = (existing: string, content: string): string => {
    if (!existing || !content) {
      return existing + content;
    }
    if (/\s$/.test(existing) || /^\s|^[,.;:!?)]/.test(content)) {
      return existing + content;
    }
    return `${existing} ${content}`;
  };

  const replaceCurrentAssistantOutputSegment = (
    msg: ChatMessage,
    previousVisibleText: string,
    replacementText: string,
  ): ChatMessage => {
    const events = msg.streamEvents || [];
    let boundaryIndex = -1;
    for (let i = events.length - 1; i >= 0; i -= 1) {
      if (events[i].type !== 'content' && events[i].type !== 'thinking') {
        boundaryIndex = i;
        break;
      }
    }

    const retainedEvents = [
      ...events.slice(0, boundaryIndex + 1),
      ...events.slice(boundaryIndex + 1).filter((evt) => evt.type !== 'content'),
    ];
    const now = Date.now();
    const streamEvents = replacementText
      ? [
          ...retainedEvents,
          withStreamEventTiming({ type: 'content' as const, content: replacementText }, now),
        ]
      : retainedEvents;

    let content = msg.content || '';
    if (previousVisibleText && content.endsWith(previousVisibleText)) {
      content = content.slice(0, -previousVisibleText.length);
    }
    content = replacementText
      ? appendTextAfterBoundary(content, replacementText)
      : content.trimEnd();

    return {
      ...msg,
      content,
      streamEvents,
    };
  };

  // Process stream events
  const extractText = (value: unknown): string => {
    if (typeof value === 'string') return value;
    if (value && typeof value === 'object') {
      const record = value as Record<string, unknown>;
      if (typeof record.text === 'string') return record.text;
      if (typeof record.content === 'string') return record.content;
    }
    return '';
  };

  const extractTokens = (data: unknown): { input: number; output: number } | null => {
    if (!data || typeof data !== 'object') return null;
    const record = data as Record<string, unknown>;
    const input = Number(record.input_tokens ?? record.inputTokens ?? record.input);
    const output = Number(record.output_tokens ?? record.outputTokens ?? record.output);
    if (!Number.isFinite(input) && !Number.isFinite(output)) return null;
    return {
      input: Number.isFinite(input) && input > 0 ? input : 0,
      output: Number.isFinite(output) && output > 0 ? output : 0,
    };
  };

  const positiveNumber = (value: unknown): number | undefined => {
    const numberValue = Number(value);
    return Number.isFinite(numberValue) && numberValue > 0 ? numberValue : undefined;
  };

  const extractCompletedModel = (data: unknown): string => {
    if (!data || typeof data !== 'object') return '';
    const record = data as Record<string, unknown>;
    const modelRoute = record.model ?? record.model_route ?? record.modelRoute;
    return typeof modelRoute === 'string' ? modelRoute.trim() : '';
  };

  const extractWorkflowModel = (data: unknown): string => extractCompletedModel(data);

  const extractErrorMessage = (data: unknown): string => {
    if (typeof data === 'string') return data;
    if (data && typeof data === 'object') {
      const record = data as Record<string, unknown>;
      if (typeof record.message === 'string') return record.message;
    }
    return '';
  };

  const extractWorkflowStatus = (data: unknown): WorkflowStatus | null => {
    if (!data || typeof data !== 'object') return null;
    const record = data as Record<string, unknown>;
    const phase = typeof record.phase === 'string' ? record.phase.trim() : '';
    if (phase === 'provider_fallback') {
      return null;
    }

    const message = typeof record.message === 'string' ? record.message.trim() : '';
    if (!message) return null;
    const state = typeof record.state === 'string' ? record.state.trim() : '';
    const tool = typeof record.tool === 'string' ? record.tool.trim() : '';
    const provider = typeof record.provider === 'string' ? record.provider.trim() : '';
    const model = extractWorkflowModel(record);
    const attempt = positiveNumber(record.attempt);
    const maxAttempts = positiveNumber(record.max_attempts ?? record.maxAttempts);
    const retryAfterMs = positiveNumber(record.retry_after_ms ?? record.retryAfterMs);

    return {
      message,
      phase: phase || undefined,
      state: state || undefined,
      tool: tool || undefined,
      provider: provider || undefined,
      model: model || undefined,
      attempt,
      maxAttempts,
      retryAfterMs,
      startedAt: Date.now(),
    };
  };

  const extractSessionId = (data: unknown): string => {
    if (!data || typeof data !== 'object') return '';
    const record = data as Record<string, unknown>;
    const id = record.id ?? record.session_id;
    return typeof id === 'string' ? id.trim() : '';
  };

  const cloneMentions = (mentions?: ChatMention[]): ChatMention[] | undefined =>
    mentions?.map((mention) => ({ ...mention }));

  const cloneRequestContext = (
    request?: ChatMessageRequestContext,
  ): ChatMessageRequestContext | undefined => {
    if (!request) return undefined;

    const cloned: ChatMessageRequestContext = {};
    if (request.mentions?.length) {
      cloned.mentions = cloneMentions(request.mentions);
    }
    if (request.findingId) {
      cloned.findingId = request.findingId;
    }
    if (request.model) {
      cloned.model = request.model;
    }
    if (typeof request.autonomousMode === 'boolean') {
      cloned.autonomousMode = request.autonomousMode;
    }
    if (request.handoffContext) {
      cloned.handoffContext = request.handoffContext;
    }
    if (request.handoffResources?.length) {
      cloned.handoffResources = request.handoffResources.map((resource) => ({ ...resource }));
    }
    if (request.handoffActions?.length) {
      cloned.handoffActions = request.handoffActions.map((action) => ({ ...action }));
    }
    if (request.handoffMetadata) {
      cloned.handoffMetadata = { ...request.handoffMetadata };
    }

    return Object.keys(cloned).length > 0 ? cloned : undefined;
  };

  const cloneSendOptions = (
    sendOptions?: SendMessageOptions,
  ): ChatMessageRequestContext | undefined => {
    if (!sendOptions) return undefined;

    const requestContext: ChatMessageRequestContext = {};
    const modelRoute = sendOptions.model?.trim();
    if (modelRoute) {
      requestContext.model = modelRoute;
    }
    if (typeof sendOptions.autonomousMode === 'boolean') {
      requestContext.autonomousMode = sendOptions.autonomousMode;
    }
    if (sendOptions.handoffContext) {
      requestContext.handoffContext = sendOptions.handoffContext;
    }
    if (sendOptions.handoffResources?.length) {
      requestContext.handoffResources = sendOptions.handoffResources.map((resource) => ({
        ...resource,
      }));
    }
    if (sendOptions.handoffActions?.length) {
      requestContext.handoffActions = sendOptions.handoffActions.map((action) => ({ ...action }));
    }
    if (sendOptions.handoffMetadata) {
      requestContext.handoffMetadata = { ...sendOptions.handoffMetadata };
    }

    return Object.keys(requestContext).length > 0 ? requestContext : undefined;
  };

  const snapshotSendOptions = (
    sendOptions?: SendMessageOptions,
  ): SendMessageOptions | undefined => {
    const modelRoute = effectiveModelRoute(sendOptions);
    const next: SendMessageOptions = {};
    if (modelRoute) {
      next.model = modelRoute;
    }
    if (typeof sendOptions?.autonomousMode === 'boolean') {
      next.autonomousMode = sendOptions.autonomousMode;
    }
    if (sendOptions?.handoffContext) {
      next.handoffContext = sendOptions.handoffContext;
    }
    if (sendOptions?.handoffResources?.length) {
      next.handoffResources = sendOptions.handoffResources.map((resource) => ({ ...resource }));
    }
    if (sendOptions?.handoffActions?.length) {
      next.handoffActions = sendOptions.handoffActions.map((action) => ({ ...action }));
    }
    if (sendOptions?.handoffMetadata) {
      next.handoffMetadata = { ...sendOptions.handoffMetadata };
    }

    return Object.keys(next).length > 0 ? next : undefined;
  };

  const buildRequestContext = (
    mentions?: ChatMention[],
    findingId?: string,
    sendOptions?: SendMessageOptions,
  ): ChatMessageRequestContext | undefined => {
    const requestContext: ChatMessageRequestContext = {
      ...(cloneSendOptions(sendOptions) || {}),
    };
    const clonedMentions = cloneMentions(mentions);
    if (clonedMentions?.length) {
      requestContext.mentions = clonedMentions;
    }
    if (findingId) {
      requestContext.findingId = findingId;
    }

    return Object.keys(requestContext).length > 0 ? requestContext : undefined;
  };

  const sendOptionsFromRequestContext = (
    request?: ChatMessageRequestContext,
  ): SendMessageOptions | undefined => {
    if (!request) return undefined;

    const sendOptions: SendMessageOptions = {};
    if (request.model) {
      sendOptions.model = request.model;
    }
    if (typeof request.autonomousMode === 'boolean') {
      sendOptions.autonomousMode = request.autonomousMode;
    }
    if (request.handoffContext) {
      sendOptions.handoffContext = request.handoffContext;
    }
    if (request.handoffResources?.length) {
      sendOptions.handoffResources = request.handoffResources.map((resource) => ({ ...resource }));
    }
    if (request.handoffActions?.length) {
      sendOptions.handoffActions = request.handoffActions.map((action) => ({ ...action }));
    }
    if (request.handoffMetadata) {
      sendOptions.handoffMetadata = { ...request.handoffMetadata };
    }

    return Object.keys(sendOptions).length > 0 ? sendOptions : undefined;
  };

  const normalizePersistedToolInput = (input: unknown): string => {
    if (typeof input === 'string') {
      return input.trim() ? input : '{}';
    }
    if (input && typeof input === 'object') {
      try {
        return JSON.stringify(input);
      } catch {
        return '{}';
      }
    }
    return '{}';
  };

  const normalizePersistedToolExecution = (tool: PersistedToolCall): ToolExecution => ({
    name: tool.name || 'unknown',
    input: normalizePersistedToolInput(tool.input),
    output: tool.output || '',
    success: tool.success ?? true,
  });

  const buildPersistedStreamEvents = (
    content: string,
    toolCalls?: PersistedToolCall[],
  ): StreamDisplayEvent[] | undefined => {
    const events: StreamDisplayEvent[] = [];
    for (const tool of toolCalls || []) {
      events.push({
        type: 'tool',
        tool: normalizePersistedToolExecution(tool),
      });
    }
    if (content.trim()) {
      events.push({ type: 'content', content });
    }
    return events.length > 0 ? events : undefined;
  };

  const applyStreamSessionId = (streamSessionId: string) => {
    if (!streamSessionId) return;
    const previousSessionId = sessionId();
    if (previousSessionId === streamSessionId) return;
    setSessionId(streamSessionId);
    if (!previousSessionId) {
      void notifyConversationChanged();
    }
  };

  const processEvent = (assistantId: string, requestId: number, event: StreamEvent) => {
    if (requestId !== activeRequestId) return;

    if (event.type === 'session') {
      applyStreamSessionId(extractSessionId(event.data));
      return;
    }
    if (event.type === 'done' || event.type === 'question') {
      applyStreamSessionId(extractSessionId(event.data));
    }

    if (event.type === 'workflow_state') {
      const workflowStatus = extractWorkflowStatus(event.data);
      const startedModel =
        workflowStatus?.phase === 'provider_start' ? extractWorkflowModel(event.data) : '';
      const routeEvent = startedModel
        ? {
            model: startedModel,
            modelEvent: 'selected' as const,
          }
        : null;
      if (!workflowStatus && !routeEvent) return;

      setMessages((prev) =>
        prev.map((msg) => {
          if (msg.id !== assistantId) return msg;
          let next = msg;
          if (routeEvent) {
            next = withModelRouteEvent(next, routeEvent.model, {
              modelEvent: routeEvent.modelEvent,
            });
          }
          if (workflowStatus && next.isStreaming !== false) {
            next = withWorkflowStatusEvent(next, workflowStatus);
          }
          return next;
        }),
      );
      if (workflowStatus) setAssistantWorkflowStatus(assistantId, requestId, workflowStatus);
      return;
    }

    setMessages((prev) =>
      prev.map((msg) => {
        if (msg.id !== assistantId) return msg;

        try {
          switch (event.type) {
            case 'content': {
              if (suppressedRawContentMessageIds.has(assistantId)) {
                return msg;
              }
              const content = extractText(event.data);
              if (!content) return msg;
              const visible = appendVisibleTextBeforeAssistantOutputArtifacts(
                outputArtifactStateFor(assistantId),
                content,
              );
              if (visible.stripped) {
                suppressedRawContentMessageIds.add(assistantId);
              }
              const baseMsg =
                visible.replacementText !== undefined
                  ? replaceCurrentAssistantOutputSegment(
                      msg,
                      visible.previousVisibleText || '',
                      visible.replacementText,
                    )
                  : msg;
              if (!visible.text) return baseMsg;
              // Add to streamEvents for chronological display
              const now = Date.now();
              const updated = addStreamEvent(baseMsg, {
                type: 'content',
                content: visible.text,
                startedAt: now,
                updatedAt: now,
              });
              return {
                ...updated,
                content: appendMessageContent(baseMsg, visible.text),
                workflowStatus: undefined,
                workflowStatusHistory: undefined,
              };
            }

            case 'thinking': {
              const thinking = extractText(event.data);
              if (!thinking) return msg;
              const now = Date.now();
              const updated = addStreamEvent(msg, {
                type: 'thinking',
                thinking,
                startedAt: now,
                updatedAt: now,
              });
              return {
                ...updated,
                thinking: (msg.thinking || '') + thinking,
              };
            }

            case 'tool_start': {
              clearSuppressedOutputBoundary(assistantId);
              const data = (event.data || {}) as {
                id?: string;
                name?: string;
                input?: string;
                raw_input?: string;
              };

              // Skip tool_start for "question" - these are handled by the question event type
              if (data.name === 'question' || data.name === 'Question') {
                return msg;
              }

              return upsertPendingToolStart(msg, data);
            }

            case 'tool_progress': {
              clearSuppressedOutputBoundary(assistantId);
              const data = event.data as {
                id?: string;
                name?: string;
                input?: string;
                raw_input?: string;
                phase?: string;
                message?: string;
              };
              if (data.name === 'question' || data.name === 'Question') {
                return msg;
              }
              return updatePendingToolProgress(msg, data);
            }

            case 'tool_cancel': {
              clearSuppressedOutputBoundary(assistantId);
              const data = event.data as {
                id?: string;
                name?: string;
                reason?: string;
              };
              const normalizedName = normalizeChatToolName(data.name || '');
              const matchesTool = (tool?: PendingTool, toolId?: string) => {
                if (!tool) return false;
                if (data.id && toolId === data.id) return true;
                if (data.id && tool.id === data.id) return true;
                return normalizedName !== '' && normalizeChatToolName(tool.name) === normalizedName;
              };
              const pendingTools = msg.pendingTools || [];
              const resolvedTool =
                pendingTools.find((tool) => matchesTool(tool, tool.id)) ||
                ({
                  id: data.id || generateId(),
                  name: data.name || 'unknown',
                  input: '{}',
                  startedAt: Date.now(),
                  updatedAt: Date.now(),
                } satisfies PendingTool);
              const now = Date.now();

              return {
                ...msg,
                workflowStatus: undefined,
                workflowStatusHistory: undefined,
                pendingTools: pendingTools.filter((tool) => !matchesTool(tool, tool.id)),
                streamEvents: replacePendingToolWithCancelEvent(
                  streamEventsWithoutLocalPromptProgressRows(msg.streamEvents),
                  {
                    ...resolvedTool,
                    updatedAt: now,
                  },
                  data.reason?.trim() || undefined,
                  matchesTool,
                  now,
                ),
              };
            }

            case 'tool_end': {
              clearSuppressedOutputBoundary(assistantId);
              const data = event.data as {
                id?: string;
                name?: string;
                input?: string;
                raw_input?: string;
                output?: string;
                success?: boolean;
              };
              const pendingTools = msg.pendingTools || [];
              const events = msg.streamEvents || [];

              const explicitEndName = data.name?.trim() || '';
              const normalizedExplicitEndName = normalizeChatToolName(explicitEndName);

              // Find the matching pending tool (prefer tool ID, then fall back to name).
              const resolvedPendingIndex = data.id
                ? pendingTools.findIndex((t) => t.id === data.id)
                : normalizedExplicitEndName
                  ? pendingTools.findIndex(
                      (t) => normalizeChatToolName(t.name) === normalizedExplicitEndName,
                    )
                  : pendingTools.length === 1
                    ? 0
                    : -1;
              const resolvedPendingTool =
                resolvedPendingIndex >= 0 ? pendingTools[resolvedPendingIndex] : undefined;
              const completedToolId = data.id || resolvedPendingTool?.id;
              const completedToolName = explicitEndName || resolvedPendingTool?.name || 'unknown';
              const normalizedEndName = normalizeChatToolName(completedToolName);
              const completedInput =
                data.input && data.input.trim() ? data.input : resolvedPendingTool?.input || '{}';
              const completedRawInput = data.raw_input ?? resolvedPendingTool?.rawInput;
              const matchesCompletedTool = (toolId?: string, toolName?: string) => {
                if (data.id && toolId === data.id) return true;
                if (completedToolId && toolId === completedToolId) return true;
                return (
                  normalizedEndName !== '' &&
                  normalizeChatToolName(toolName || '') === normalizedEndName
                );
              };
              const updatedPending =
                resolvedPendingIndex >= 0
                  ? [
                      ...pendingTools.slice(0, resolvedPendingIndex),
                      ...pendingTools.slice(resolvedPendingIndex + 1),
                    ]
                  : pendingTools;

              const newToolCall: ToolExecution = {
                name: completedToolName,
                input: completedInput,
                rawInput: completedRawInput,
                output: data.output || '',
                success: data.success ?? true,
              };

              // Check if there's an approval card for this tool
              // If so, we need to remove both the pending_tool AND the approval,
              // then add the completed tool at the end (since execution happened AFTER approval)
              const hasApproval = events.some(
                (evt) =>
                  evt.type === 'approval' &&
                  matchesCompletedTool(evt.approval?.toolId, evt.approval?.toolName),
              );

              let updatedEvents: typeof events;
              if (hasApproval) {
                // Remove pending_tool and approval, add completed tool at end
                updatedEvents = events.filter((evt) => {
                  if (
                    evt.type === 'pending_tool' &&
                    matchesCompletedTool(evt.toolId, evt.pendingTool?.name)
                  ) {
                    return false;
                  }
                  if (
                    evt.type === 'approval' &&
                    matchesCompletedTool(evt.approval?.toolId, evt.approval?.toolName)
                  ) {
                    return false;
                  }
                  return true;
                });
                const completedAt = Date.now();
                const settleUntil =
                  newToolCall.success && resolvedPendingTool?.startedAt
                    ? getAssistantFastToolCompletionSettleUntil(
                        resolvedPendingTool.startedAt,
                        completedAt,
                        completedAt,
                      )
                    : undefined;
                updatedEvents.push({
                  type: 'tool',
                  tool: newToolCall,
                  toolId: completedToolId,
                  startedAt: resolvedPendingTool?.startedAt,
                  updatedAt: completedAt,
                  settleUntil,
                });
              } else {
                // No approval - replace the pending_tool in place. If the terminal
                // event is the first visible evidence, keep the completed row.
                updatedEvents = [...events];
                let replacedPendingTool = false;
                const completedAt = Date.now();
                for (let i = events.length - 1; i >= 0; i--) {
                  const evt = events[i];
                  if (
                    evt.type === 'pending_tool' &&
                    matchesCompletedTool(evt.toolId, evt.pendingTool?.name)
                  ) {
                    const startedAt = evt.pendingTool?.startedAt || evt.startedAt;
                    const settleUntil = newToolCall.success
                      ? getAssistantFastToolCompletionSettleUntil(
                          startedAt,
                          completedAt,
                          completedAt,
                        )
                      : undefined;
                    updatedEvents[i] = {
                      type: 'tool',
                      tool: newToolCall,
                      toolId: completedToolId,
                      startedAt,
                      updatedAt: completedAt,
                      settleUntil,
                    };
                    replacedPendingTool = true;
                    break;
                  }
                }
                if (!replacedPendingTool) {
                  updatedEvents.push({
                    type: 'tool',
                    tool: newToolCall,
                    toolId: completedToolId,
                    updatedAt: completedAt,
                  });
                }
              }

              // Also remove from pendingApprovals if present
              const updatedApprovals = (msg.pendingApprovals || []).filter(
                (a) => !matchesCompletedTool(a.toolId, a.toolName),
              );

              return {
                ...msg,
                streamEvents: updatedEvents,
                workflowStatus: undefined,
                workflowStatusHistory: undefined,
                pendingTools: updatedPending,
                pendingApprovals: updatedApprovals,
                toolCalls: [...(msg.toolCalls || []), newToolCall],
              };
            }

            case 'approval_needed': {
              clearSuppressedOutputBoundary(assistantId);
              const data = event.data as {
                command: string;
                tool_id: string;
                tool_name: string;
                run_on_host: boolean;
                target_host?: string;
                target_type?: string;
                target_id?: string;
                risk?: string;
                description?: string;
                audit_id?: string;
                plan?: {
                  action_id?: string;
                  request_id?: string;
                  summary?: string;
                  requires_approval: boolean;
                  approval_policy?: string;
                  blast_radius?: string;
                  rollback_available: boolean;
                  plan_hash?: string;
                  expires_at?: string;
                };
                context_confidence?: {
                  level?: string;
                  summary?: string;
                  evidence?: string[];
                };
                preflight?: {
                  target?: string;
                  current_state?: string;
                  intended_change?: string;
                  dry_run_available: boolean;
                  dry_run_summary?: string;
                  safety_checks?: string[];
                  verification_steps?: string[];
                  generated_at?: string;
                };
                approval_id?: string;
              };

              const approval: PendingApproval = {
                command: data.command,
                toolId: data.tool_id,
                toolName: data.tool_name,
                runOnHost: data.run_on_host,
                targetHost: data.target_host,
                targetType: data.target_type,
                targetId: data.target_id,
                isExecuting: false,
                approvalId: data.approval_id,
              };
              if (typeof data.risk === 'string') {
                approval.risk = data.risk;
              }
              if (typeof data.description === 'string') {
                approval.description = data.description;
              }
              if (typeof data.audit_id === 'string') {
                approval.auditId = data.audit_id;
              }
              if (data.plan && typeof data.plan === 'object') {
                approval.plan = data.plan;
              }
              if (data.context_confidence && typeof data.context_confidence === 'object') {
                approval.contextConfidence = data.context_confidence;
              }
              if (data.preflight && typeof data.preflight === 'object') {
                approval.preflight = data.preflight;
              }

              // Add to streamEvents for chronological display
              const updated = addStreamEvent(msg, {
                type: 'approval',
                approval,
              });

              return {
                ...updated,
                workflowStatus: undefined,
                workflowStatusHistory: undefined,
                pendingApprovals: [...(msg.pendingApprovals || []), approval],
              };
            }

            case 'question': {
              clearSuppressedOutputBoundary(assistantId);
              const data = event.data as { question_id: string; questions: Array<any> };

              const pendingQuestion: PendingQuestion = {
                questionId: data.question_id,
                questions: (data.questions || []).map((q) => ({
                  id: String(q.id || ''),
                  type:
                    q.type === 'select' || (Array.isArray(q.options) && q.options.length > 0)
                      ? 'select'
                      : 'text',
                  header: typeof q.header === 'string' ? q.header : undefined,
                  question: String(q.question || ''),
                  options: Array.isArray(q.options)
                    ? q.options.map((opt: any) => ({
                        label: String(opt.label || ''),
                        value: String(opt.value ?? opt.label ?? ''),
                        description:
                          typeof opt.description === 'string' ? opt.description : undefined,
                      }))
                    : undefined,
                })),
                isAnswering: false,
              };

              // Add to streamEvents for chronological display
              const updated = addStreamEvent(msg, {
                type: 'question',
                question: pendingQuestion,
              });

              return {
                ...updated,
                workflowStatus: undefined,
                workflowStatusHistory: undefined,
                pendingQuestions: [...(msg.pendingQuestions || []), pendingQuestion],
              };
            }

            case 'done': {
              const completedAt = new Date();
              const pendingText = suppressedRawContentMessageIds.has(assistantId)
                ? ''
                : flushPendingAssistantOutputText(outputArtifactStateFor(assistantId));
              suppressedRawContentMessageIds.delete(assistantId);
              clearOutputArtifactState(assistantId);
              const flushedMsg = pendingText
                ? {
                    ...addStreamEvent(msg, {
                      type: 'content',
                      content: pendingText,
                      startedAt: Date.now(),
                      updatedAt: Date.now(),
                    }),
                    content: appendMessageContent(msg, pendingText),
                  }
                : msg;
              const tokens = extractTokens(event.data);
              const completedModel = extractCompletedModel(event.data);
              if (tokens && (tokens.input > 0 || tokens.output > 0)) {
                return {
                  ...flushedMsg,
                  isStreaming: false,
                  completedAt,
                  ...(completedModel ? { model: completedModel } : {}),
                  pendingTools: [],
                  pendingApprovals: [],
                  pendingQuestions: [],
                  streamEvents: streamEventsWithoutUnresolvedInteractiveRows(
                    flushedMsg.streamEvents,
                  ),
                  tokens,
                  workflowStatus: undefined,
                  workflowStatusHistory: undefined,
                };
              }
              return {
                ...flushedMsg,
                isStreaming: false,
                completedAt,
                ...(completedModel ? { model: completedModel } : {}),
                pendingTools: [],
                pendingApprovals: [],
                pendingQuestions: [],
                streamEvents: streamEventsWithoutUnresolvedInteractiveRows(
                  flushedMsg.streamEvents,
                ),
                workflowStatus: undefined,
                workflowStatusHistory: undefined,
              };
            }

            case 'error': {
              clearSuppressedOutputBoundary(assistantId);
              const errorMsg = extractErrorMessage(event.data);
              // Keep any content streamed before the failure; surface the error
              // as a distinct, recoverable block rather than overwriting the answer.
              return {
                ...msg,
                isStreaming: false,
                completedAt: new Date(),
                pendingTools: [],
                pendingApprovals: [],
                pendingQuestions: [],
                streamEvents: streamEventsWithoutUnresolvedInteractiveRows(msg.streamEvents),
                workflowStatus: undefined,
                workflowStatusHistory: undefined,
                error: errorMsg || 'Request failed',
              };
            }

            default:
              return msg;
          }
        } catch (err) {
          logger.error('[useChat] Error processing event', { event, error: err });
          return msg; // Return unchanged message on error
        }
      }),
    );
  };

  const queueFollowUp = (
    prompt: string,
    mentions?: ChatMention[],
    findingId?: string,
    sendOptions?: SendMessageOptions,
  ) => {
    const trimmedPrompt = prompt.trim();
    if (!trimmedPrompt) return false;

    const id = generateId();
    const messageId = generateId();
    const timestamp = new Date();
    const queuedSendOptions = snapshotSendOptions(sendOptions);

    const queuedUserMessage: ChatMessage = {
      id: messageId,
      role: 'user',
      content: trimmedPrompt,
      timestamp,
      delivery: 'queued',
      request: buildRequestContext(mentions, findingId, queuedSendOptions),
    };

    const queuedFollowUp: QueuedFollowUp = {
      id,
      messageId,
      prompt: trimmedPrompt,
      mentions,
      findingId,
      sendOptions: queuedSendOptions,
      timestamp,
    };

    setMessages((prev) => [...prev, queuedUserMessage]);
    setQueuedFollowUps((prev) => [...prev, queuedFollowUp]);
    setQueuedFollowUpsPaused(false);
    logger.debug('[useChat] Queued follow-up while assistant response is streaming', {
      queuedFollowUpId: id,
    });
    return true;
  };

  const startMessageSend = async (
    prompt: string,
    mentions?: ChatMention[],
    findingId?: string,
    sendOptions?: SendMessageOptions,
    options?: {
      queuedMessageId?: string;
      drainAfter?: boolean;
    },
  ): Promise<boolean> => {
    const trimmedPrompt = prompt.trim();
    if (!trimmedPrompt) return false;

    const requestSendOptions = snapshotSendOptions(sendOptions);
    const requestModel = requestSendOptions?.model?.trim() || '';

    // Echo the user's message before any network work. Cold sessions can spend
    // noticeable time creating the server-side session; the chat surface should
    // still feel immediate.
    const turnStartedAt = Date.now();
    const userMessage: ChatMessage = {
      id: options?.queuedMessageId || generateId(),
      role: 'user',
      content: trimmedPrompt,
      timestamp: new Date(turnStartedAt),
      request: buildRequestContext(mentions, findingId, requestSendOptions),
    };

    const assistantId = generateId();
    const initialWorkflowStatus = createInitialAssistantWorkflowStatus(turnStartedAt);
    const initialStreamEvents: StreamDisplayEvent[] = [
      ...(requestModel
        ? [
            withStreamEventTiming(
              {
                type: 'model_switch' as const,
                model: requestModel,
                modelEvent: 'selected' as const,
              },
              turnStartedAt,
            ),
          ]
        : []),
      withStreamEventTiming(
        {
          type: 'workflow_status',
          workflowStatus: initialWorkflowStatus,
          startedAt: initialWorkflowStatus.startedAt,
          updatedAt: initialWorkflowStatus.startedAt,
        },
        turnStartedAt,
      ),
    ];
    const streamingMessage: ChatMessage = {
      id: assistantId,
      role: 'assistant',
      content: '',
      timestamp: new Date(turnStartedAt),
      model: requestModel || undefined,
      isStreaming: true,
      pendingTools: [],
      toolCalls: [],
      streamEvents: initialStreamEvents,
      workflowStatus: initialWorkflowStatus,
      workflowStatusHistory: [],
    };

    if (options?.queuedMessageId) {
      setMessages((prev) => [
        ...prev.map((msg) =>
          msg.id === options.queuedMessageId ? { ...msg, delivery: 'sent' as const } : msg,
        ),
        streamingMessage,
      ]);
    } else {
      setMessages((prev) => [...prev, userMessage, streamingMessage]);
    }
    setIsLoading(true);
    const requestId = ++activeRequestId;

    // Existing sessions preserve conversation continuity. Cold chats let the
    // backend create the session inside the stream and bind via the session SSE
    // event, avoiding a separate preflight request before first token.
    const currentSessionId = sessionId();

    const abortController = new AbortController();
    abortControllerRef = abortController;
    let visibleTurnCompleted = false;

    const promoteLocalWaitStatus = () => {
      localAssistantWaitStatusTimer = undefined;
      if (requestId !== activeRequestId) return;
      const waitStatus = createLocalAssistantWaitWorkflowStatus(turnStartedAt);
      setMessages((prev) =>
        prev.map((msg) => {
          if (msg.id !== assistantId) return msg;
          if (msg.isStreaming === false) return msg;
          if (msg.workflowStatus?.phase !== 'request_send') return msg;
          const updated = withWorkflowStatusEvent(msg, waitStatus);
          return { ...updated, workflowStatus: waitStatus };
        }),
      );
    };

    clearLocalAssistantWaitStatusTimer();
    localAssistantWaitStatusTimer = setTimeout(
      promoteLocalWaitStatus,
      LOCAL_ASSISTANT_WAIT_STATUS_DELAY_MS,
    );

    const completeVisibleTurn = () => {
      if (requestId !== activeRequestId || visibleTurnCompleted) return;
      visibleTurnCompleted = true;
      clearLocalAssistantWaitStatusTimer();
      abortControllerRef = null;
      setIsLoading(false);
      if (options?.drainAfter !== false) {
        queueMicrotask(() => {
          void drainQueuedFollowUps();
        });
      }
    };

    try {
      await AIChatAPI.chat(
        trimmedPrompt,
        currentSessionId || undefined,
        requestModel || undefined,
        (event: StreamEvent) => {
          if (event.type !== 'session') {
            clearLocalAssistantWaitStatusTimer();
          }
          processEvent(assistantId, requestId, event);
        },
        abortController.signal,
        mentions,
        findingId,
        requestSendOptions?.autonomousMode,
        requestSendOptions?.handoffContext,
        requestSendOptions?.handoffResources,
        requestSendOptions?.handoffActions,
        requestSendOptions?.handoffMetadata,
      );
      if (requestId !== activeRequestId) {
        return false;
      }
      completeVisibleTurn();
      await notifyConversationChanged();
      return true;
    } catch (error) {
      if (requestId !== activeRequestId) {
        return false;
      }
      if (error instanceof Error && error.name === 'AbortError') {
        logger.debug('[useChat] Request aborted');
        return false;
      }
      logger.error('[useChat] Chat failed:', error);
      const errorMessage =
        error instanceof Error ? error.message : 'Failed to get Pulse Assistant response';
      notificationStore.error(errorMessage);

      setMessages((prev) =>
        prev.map((msg) =>
          msg.id === assistantId
            ? {
                ...msg,
                isStreaming: false,
                pendingTools: [],
                pendingApprovals: [],
                pendingQuestions: [],
                streamEvents: streamEventsWithoutUnresolvedInteractiveRows(msg.streamEvents),
                workflowStatus: undefined,
                workflowStatusHistory: undefined,
                error: errorMessage,
              }
            : msg,
        ),
      );
      completeVisibleTurn();
      await notifyConversationChanged();
      return false;
    } finally {
      if (requestId === activeRequestId) {
        completeVisibleTurn();
      }
    }
  };

  async function drainQueuedFollowUps() {
    if (isDrainingQueuedFollowUps || isLoading() || queuedFollowUpsPaused()) return;

    isDrainingQueuedFollowUps = true;
    try {
      while (!isLoading()) {
        const next = queuedFollowUps()[0];
        if (!next) return;

        setQueuedFollowUps((prev) => prev.filter((entry) => entry.id !== next.id));
        await startMessageSend(next.prompt, next.mentions, next.findingId, next.sendOptions, {
          queuedMessageId: next.messageId,
          drainAfter: false,
        });
      }
    } finally {
      isDrainingQueuedFollowUps = false;
    }

    if (queuedFollowUps().length > 0 && !isLoading() && !queuedFollowUpsPaused()) {
      queueMicrotask(() => {
        void drainQueuedFollowUps();
      });
    }
  }

  // Send a message. While an assistant response is active, accept the user's
  // follow-up immediately and queue it behind the current turn instead of
  // replacing or aborting the stream.
  const sendMessage = async (
    prompt: string,
    mentions?: ChatMention[],
    findingId?: string,
    sendOptions?: SendMessageOptions,
  ): Promise<boolean> => {
    if (!prompt.trim()) return false;
    if (isLoading()) {
      return queueFollowUp(prompt, mentions, findingId, sendOptions);
    }
    return startMessageSend(prompt, mentions, findingId, sendOptions);
  };

  const sendQueuedFollowUpNow = async (id: string): Promise<boolean> => {
    const item = queuedFollowUps().find((entry) => entry.id === id);
    if (!item) return false;
    if (isLoading()) {
      return promoteQueuedFollowUp(id);
    }

    setQueuedFollowUpsPaused(false);
    setQueuedFollowUps((prev) => prev.filter((entry) => entry.id !== id));
    return startMessageSend(item.prompt, item.mentions, item.findingId, item.sendOptions, {
      queuedMessageId: item.messageId,
    });
  };

  // Clear messages and reset session (for starting fresh)
  const clearMessages = () => {
    void cancelActiveRequest();
    setQueuedFollowUps([]);
    setQueuedFollowUpsPaused(false);
    setMessages([]);
    setSessionId(''); // Clear session so next message creates a new one
  };

  // Load session messages
  const loadSession = async (id: string): Promise<boolean> => {
    void cancelActiveRequest();
    setQueuedFollowUps([]);
    setQueuedFollowUpsPaused(false);
    try {
      const msgs = await AIChatAPI.getMessages(id);
      const loadedMessages = msgs.map((m) => {
        const persistedToolCalls = m.tool_calls || [];
        const toolCalls = persistedToolCalls.map(normalizePersistedToolExecution);
        const content =
          m.role === 'assistant' ? stripAssistantOutputArtifacts(m.content).text : m.content;
        const streamEvents =
          m.role === 'assistant'
            ? buildPersistedStreamEvents(content, persistedToolCalls)
            : undefined;
        return {
          id: m.id,
          role: m.role,
          content,
          timestamp: new Date(m.timestamp),
          model: m.model,
          toolCalls,
          streamEvents,
        };
      });
      const restoredModel = latestExplicitModelRouteFromTranscript(loadedMessages);
      if (restoredModel) {
        setModel(restoredModel);
      }
      setMessages(loadedMessages);
      setSessionId(id);
      return true;
    } catch (error) {
      logger.error('[useChat] Failed to load session:', error);
      notificationStore.error('Failed to load session');
      return false;
    }
  };

  // Start a blank conversation. The durable backend session is created by the
  // next chat stream so the UI does not create empty server-side sessions.
  const newSession = async (): Promise<boolean> => {
    void cancelActiveRequest();
    setQueuedFollowUps([]);
    setQueuedFollowUpsPaused(false);
    setSessionId('');
    setMessages([]);
    return true;
  };

  const latestUserPromptDraft = (): RestoredPromptDraft | null => {
    const currentMessages = messages();
    for (let index = currentMessages.length - 1; index >= 0; index -= 1) {
      const message = currentMessages[index];
      if (message.role !== 'user' || !message.content.trim() || message.delivery === 'queued') {
        continue;
      }
      return {
        prompt: message.content,
        request: cloneRequestContext(message.request),
      };
    }
    return null;
  };

  const undoLastTurn = async (): Promise<RestoredPromptDraft | null> => {
    const currentSessionId = sessionId().trim();
    if (!currentSessionId || isLoading()) return null;

    const localDraft = latestUserPromptDraft();
    const result = await AIChatAPI.undoLastTurn(currentSessionId);
    if (!result.success) {
      if (result.message) {
        notificationStore.info(result.message);
      }
      return null;
    }

    await loadSession(currentSessionId);
    await notifyConversationChanged();
    const prompt = localDraft?.prompt || result.restored_prompt || '';
    if (!prompt.trim()) return null;
    return {
      prompt,
      request: localDraft?.request,
    };
  };

  const redoLastTurn = async (): Promise<RedoTurnResult> => {
    const currentSessionId = sessionId().trim();
    if (!currentSessionId || isLoading()) return { success: false, canRedo: false };

    const result = await AIChatAPI.redoLastTurn(currentSessionId);
    if (!result.success) {
      if (result.message) {
        notificationStore.info(result.message);
      }
      return { success: false, canRedo: result.can_redo };
    }

    const loaded = await loadSession(currentSessionId);
    await notifyConversationChanged();
    return { success: loaded, canRedo: result.can_redo };
  };

  // Update pending approval state (e.g., to mark as executing or remove)
  const updateApproval = (
    messageId: string,
    toolId: string,
    update: Partial<{ isExecuting: boolean; removed: boolean }>,
  ) => {
    setMessages((prev) =>
      prev.map((msg) => {
        if (msg.id !== messageId) return msg;
        if (update.removed) {
          // Remove from pendingApprovals
          return {
            ...msg,
            pendingApprovals: (msg.pendingApprovals || []).filter((a) => a.toolId !== toolId),
            streamEvents: (msg.streamEvents || []).filter(
              (event) => !(event.type === 'approval' && event.approval?.toolId === toolId),
            ),
          };
        }

        const shouldUpdateExecuting = typeof update.isExecuting === 'boolean';
        const updatedAt = Date.now();

        // Update the approval in place
        return {
          ...msg,
          pendingApprovals: (msg.pendingApprovals || []).map((a) =>
            a.toolId === toolId ? { ...a, ...update } : a,
          ),
          streamEvents: (msg.streamEvents || []).map((event) => {
            if (event.type !== 'approval' || event.approval?.toolId !== toolId) {
              return event;
            }
            if (!shouldUpdateExecuting) {
              return event;
            }
            return {
              ...event,
              updatedAt,
              approval: {
                ...event.approval,
                isExecuting: update.isExecuting,
              },
            };
          }),
        };
      }),
    );
  };

  // Add a tool call result to a message (after approval execution)
  const addToolResult = (
    messageId: string,
    toolCall: { name: string; input: string; output: string; success: boolean },
  ) => {
    setMessages((prev) =>
      prev.map((msg) => {
        if (msg.id !== messageId) return msg;
        return {
          ...msg,
          toolCalls: [...(msg.toolCalls || []), toolCall],
          streamEvents: [
            ...(msg.streamEvents || []),
            withStreamEventTiming({ type: 'tool' as const, tool: toolCall }),
          ],
        };
      }),
    );
  };

  // Update pending question state (e.g., to mark as answering or remove)
  const updateQuestion = (
    messageId: string,
    questionId: string,
    update: Partial<{ isAnswering: boolean; removed: boolean }>,
  ) => {
    setMessages((prev) =>
      prev.map((msg) => {
        if (msg.id !== messageId) return msg;
        if (update.removed) {
          // Remove from pendingQuestions
          return {
            ...msg,
            pendingQuestions: (msg.pendingQuestions || []).filter(
              (q) => q.questionId !== questionId,
            ),
            streamEvents: (msg.streamEvents || []).filter(
              (event) => !(event.type === 'question' && event.question?.questionId === questionId),
            ),
          };
        }

        const shouldUpdateAnswering = typeof update.isAnswering === 'boolean';
        const updatedAt = Date.now();

        // Update the question in place
        return {
          ...msg,
          pendingQuestions: (msg.pendingQuestions || []).map((q) =>
            q.questionId === questionId ? { ...q, ...update } : q,
          ),
          streamEvents: (msg.streamEvents || []).map((event) => {
            if (event.type !== 'question' || event.question?.questionId !== questionId) {
              return event;
            }
            if (!shouldUpdateAnswering) {
              return event;
            }
            return {
              ...event,
              updatedAt,
              question: {
                ...event.question,
                isAnswering: update.isAnswering,
              },
            };
          }),
        };
      }),
    );
  };

  // Answer a pending question
  const answerQuestion = async (
    messageId: string,
    questionId: string,
    answers: Array<{ id: string; value: string }>,
  ) => {
    updateQuestion(messageId, questionId, { isAnswering: true });

    try {
      // Send answer to Pulse AI via API
      await AIChatAPI.answerQuestion(questionId, answers);

      // Remove the question card - it's been handled
      updateQuestion(messageId, questionId, { removed: true });

      logger.debug('[useChat] Question answered, waiting for AI to continue', {
        questionId,
      });

      // Brief delay to allow backend processing to settle
      await new Promise((resolve) => setTimeout(resolve, 500));
    } catch (error) {
      logger.error('[useChat] Failed to answer question:', error);
      notificationStore.error('Failed to answer question');
      updateQuestion(messageId, questionId, { isAnswering: false });
    }
  };

  // Wait for the chat to become idle (not loading)
  // Useful for sending follow-up messages after approvals
  const waitForIdle = (timeoutMs = 30000): Promise<boolean> => {
    return new Promise((resolve) => {
      const hasPendingQueuedDrain = queuedFollowUps().length > 0 && !queuedFollowUpsPaused();
      if (!isLoading() && !hasPendingQueuedDrain && !isDrainingQueuedFollowUps) {
        resolve(true);
        return;
      }

      const startTime = Date.now();
      const checkInterval = setInterval(() => {
        const hasPendingQueuedDrain = queuedFollowUps().length > 0 && !queuedFollowUpsPaused();
        if (!isLoading() && !hasPendingQueuedDrain && !isDrainingQueuedFollowUps) {
          clearInterval(checkInterval);
          resolve(true);
        } else if (Date.now() - startTime > timeoutMs) {
          clearInterval(checkInterval);
          logger.warn('[useChat] waitForIdle timed out');
          resolve(false);
        }
      }, 100);
    });
  };

  // Retry a failed assistant turn: drop the failed assistant message and the
  // user prompt that triggered it from the view, then re-send that prompt so the
  // conversation shows a single clean attempt instead of a dead-end error.
  const retryMessage = (assistantMessageId: string, sendOptionOverrides?: SendMessageOptions) => {
    const msgs = messages();
    const idx = msgs.findIndex((m) => m.id === assistantMessageId);
    if (idx < 0) return;
    let userIdx = idx - 1;
    while (userIdx >= 0 && msgs[userIdx].role !== 'user') userIdx--;
    if (userIdx < 0) return;
    const prompt = msgs[userIdx].content;
    if (!prompt.trim()) return;
    const request = msgs[userIdx].request;
    const sendOptions = {
      ...(sendOptionsFromRequestContext(request) || {}),
      ...(sendOptionOverrides || {}),
    };
    const removeIds = new Set([msgs[userIdx].id, assistantMessageId]);
    setMessages((prev) => prev.filter((m) => !removeIds.has(m.id)));
    void sendMessage(
      prompt,
      cloneMentions(request?.mentions),
      request?.findingId,
      Object.keys(sendOptions).length > 0 ? sendOptions : undefined,
    );
  };

  return {
    messages,
    isLoading,
    sessionId,
    model,
    setModel,
    queuedFollowUps,
    queuedFollowUpsPaused,
    queuedFollowUpCount: () => queuedFollowUps().length,
    sendMessage,
    sendQueuedFollowUpNow,
    retryMessage,
    undoLastTurn,
    redoLastTurn,
    stop,
    cancelQueuedFollowUp,
    takeQueuedFollowUp,
    clearQueuedFollowUps,
    clearMessages,
    loadSession,
    newSession,
    updateApproval,
    addToolResult,
    updateQuestion,
    answerQuestion,
    waitForIdle,
  };
}
