import type { ChatMessage, PendingTool, StreamDisplayEvent, WorkflowStatus } from './types';
import { formatIdentifierLabel } from '@/utils/textPresentation';
import { formatAIModelRouteLabel } from '@/utils/aiProviderPresentation';
import { extractReasoningSummaryTitle } from './reasoningSummary';
import {
  isPlaceholderToolInputSummary,
  parseToolInputSummary,
  toolValueText,
} from './toolPresentation';

export type AssistantActiveTurnStatusKind = 'thinking' | 'tool' | 'generating';

export interface AssistantActiveTurnStatus {
  text: string;
  type: AssistantActiveTurnStatusKind;
  startedAt?: number;
}

interface AssistantActiveTurnStatusCandidate extends AssistantActiveTurnStatus {
  activityAt?: number;
  order: number;
}

const formatToolName = (name?: string) =>
  formatIdentifierLabel(name, { stripPrefix: 'pulse_', fallback: 'tool', maxLength: 36 });

const escapeRegExp = (value: string) => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

const messageContainsToolLabel = (message: string, label: string) => {
  if (!label) return false;
  return new RegExp(`(^|\\W)${escapeRegExp(label)}($|\\W)`, 'i').test(message);
};

const formatRetryDelay = (milliseconds?: number): string => {
  if (!Number.isFinite(milliseconds) || !milliseconds || milliseconds <= 0) return '';
  if (milliseconds < 1000) return `${Math.round(milliseconds)}ms`;
  const seconds = milliseconds / 1000;
  if (seconds < 10) return `${Number(seconds.toFixed(1))}s`;
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const minutes = seconds / 60;
  if (minutes < 10) return `${Number(minutes.toFixed(1))}m`;
  return `${Math.round(minutes)}m`;
};

const formatRetryStatusSuffix = (status?: WorkflowStatus): string => {
  const parts: string[] = [];
  if (status?.attempt && status.maxAttempts) {
    parts.push(`attempt ${status.attempt}/${status.maxAttempts}`);
  } else if (status?.attempt) {
    parts.push(`attempt ${status.attempt}`);
  }

  const retryDelay = formatRetryDelay(status?.retryAfterMs);
  if (retryDelay) {
    parts.push(`retrying in ${retryDelay}`);
  }

  return parts.length > 0 ? ` · ${parts.join(' · ')}` : '';
};

export const formatAssistantWorkflowStatus = (status?: WorkflowStatus): string => {
  const message = status?.message?.trim();
  if (!message) return '';

  const tool = status?.tool?.trim();
  const toolLabel = tool ? formatToolName(tool) : '';
  const toolSuffix =
    toolLabel && !message.includes(tool || '') && !messageContainsToolLabel(message, toolLabel)
      ? ` · ${toolLabel}`
      : '';
  const retrySuffix = formatRetryStatusSuffix(status);

  return `${message}${toolSuffix}${retrySuffix}`;
};

const formatPendingToolStatus = (tool?: PendingTool): string => {
  if (!tool) return '';

  const progress = tool.progress?.trim();
  if (progress) return progress;

  const inputSummary = parseToolInputSummary(toolValueText(tool.input), tool.name, tool.rawInput);
  const toolActivity = isPlaceholderToolInputSummary(inputSummary) ? '' : inputSummary;
  if (toolActivity) {
    if (tool.status === 'waiting') return `Waiting on ${toolActivity}`;
    return `Running ${toolActivity}`;
  }

  const toolLabel = formatToolName(tool.name);
  if (tool.status === 'waiting') return `Waiting on ${toolLabel}`;

  return `Running ${toolLabel}`;
};

const activePendingToolFromEvents = (events?: StreamDisplayEvent[]): PendingTool | undefined => {
  const completedToolKeys = new Set<string>();

  for (let index = (events?.length || 0) - 1; index >= 0; index -= 1) {
    const event = events?.[index];
    if (!event) continue;
    if (event.type === 'pending_tool' && event.pendingTool) {
      const keys = [event.toolId, event.pendingTool.id, event.pendingTool.name]
        .map((value) => value?.trim())
        .filter((value): value is string => !!value);
      if (!keys.some((key) => completedToolKeys.has(key))) {
        return event.pendingTool;
      }
    }
    if (event.type === 'tool') {
      for (const key of [event.toolId, event.tool?.name]) {
        const normalized = key?.trim();
        if (normalized) {
          completedToolKeys.add(normalized);
        }
      }
    }
  }

  return undefined;
};

const latestPendingToolActivity = (tool?: PendingTool): number =>
  tool?.updatedAt ?? tool?.startedAt ?? 0;

const pendingToolCandidate = (
  tool: PendingTool | undefined,
  order: number,
): AssistantActiveTurnStatusCandidate | null => {
  const text = formatPendingToolStatus(tool);
  if (!text) return null;
  const activityAt = latestPendingToolActivity(tool) || undefined;
  return {
    type: 'tool',
    text,
    startedAt: tool?.startedAt,
    activityAt,
    order,
  };
};

const activePendingToolCandidateFromState = (
  pendingTools?: PendingTool[],
): AssistantActiveTurnStatusCandidate | null =>
  (pendingTools || []).reduce<AssistantActiveTurnStatusCandidate | null>((current, tool, index) => {
    const candidate = pendingToolCandidate(tool, index);
    if (!candidate) return current;
    return isFresherStatusCandidate(candidate, current) ? candidate : current;
  }, null);

const thinkingStatusText = (event: StreamDisplayEvent): string => {
  if (!event.thinking?.trim()) return '';
  const title = extractReasoningSummaryTitle(event.thinking);
  return title ? `Thinking: ${title}` : 'Thinking';
};

const eventActivityAt = (event: StreamDisplayEvent): number | undefined => {
  if (event.type === 'pending_tool') {
    return (
      latestPendingToolActivity(event.pendingTool) ||
      event.updatedAt ||
      event.startedAt ||
      undefined
    );
  }
  return event.updatedAt || event.startedAt || undefined;
};

const toolCompletionKeys = (event: StreamDisplayEvent): string[] =>
  [event.toolId, event.tool?.name]
    .map((value) => value?.trim())
    .filter((value): value is string => !!value);

const pendingToolKeys = (event: StreamDisplayEvent): string[] =>
  [event.toolId, event.pendingTool?.id, event.pendingTool?.name]
    .map((value) => value?.trim())
    .filter((value): value is string => !!value);

const isFresherStatusCandidate = (
  candidate: AssistantActiveTurnStatusCandidate,
  current: AssistantActiveTurnStatusCandidate | null,
): boolean => {
  if (!current) return true;
  const candidateTime = candidate.activityAt;
  const currentTime = current.activityAt;
  if (candidateTime !== undefined && currentTime !== undefined && candidateTime !== currentTime) {
    return candidateTime > currentTime;
  }
  if (candidateTime !== undefined && currentTime === undefined) return true;
  if (candidateTime === undefined && currentTime !== undefined) return false;
  return candidate.order >= current.order;
};

const latestStreamActivityStatus = (
  events?: StreamDisplayEvent[],
): AssistantActiveTurnStatusCandidate | null => {
  const completedToolKeys = new Set<string>();
  let current: AssistantActiveTurnStatusCandidate | null = null;

  for (let index = (events?.length || 0) - 1; index >= 0; index -= 1) {
    const event = events?.[index];
    if (!event) continue;

    let candidate: AssistantActiveTurnStatusCandidate | null = null;
    switch (event.type) {
      case 'tool': {
        for (const key of toolCompletionKeys(event)) {
          completedToolKeys.add(key);
        }
        break;
      }
      case 'pending_tool': {
        const keys = pendingToolKeys(event);
        if (!keys.some((key) => completedToolKeys.has(key))) {
          candidate = pendingToolCandidate(event.pendingTool, index);
        }
        break;
      }
      case 'thinking': {
        const text = thinkingStatusText(event);
        if (text) {
          candidate = {
            type: 'thinking',
            text,
            startedAt: event.startedAt,
            activityAt: eventActivityAt(event),
            order: index,
          };
        }
        break;
      }
      case 'content': {
        if (event.content?.trim()) {
          candidate = {
            type: 'generating',
            text: 'Generating response',
            startedAt: event.startedAt,
            activityAt: eventActivityAt(event),
            order: index,
          };
        }
        break;
      }
      case 'approval':
        if (event.approval) {
          candidate = {
            type: 'thinking',
            text: 'Waiting for approval',
            startedAt: event.startedAt,
            activityAt: eventActivityAt(event),
            order: index,
          };
        }
        break;
      case 'question':
        if (event.question) {
          candidate = {
            type: 'thinking',
            text: 'Waiting for answer',
            startedAt: event.startedAt,
            activityAt: eventActivityAt(event),
            order: index,
          };
        }
        break;
      case 'model_switch': {
        const text = modelSwitchStatusText(event);
        if (text) {
          candidate = {
            type: 'thinking',
            text,
            startedAt: event.startedAt,
            activityAt: eventActivityAt(event),
            order: index,
          };
        }
        break;
      }
      default:
        break;
    }

    if (candidate && isFresherStatusCandidate(candidate, current)) {
      current = candidate;
    }
  }

  return current;
};

const workflowStatusCandidate = (
  status: WorkflowStatus | undefined,
): AssistantActiveTurnStatusCandidate | null => {
  const text = formatAssistantWorkflowStatus(status);
  if (!text) return null;
  return {
    type: status?.tool ? 'tool' : 'thinking',
    text,
    startedAt: status?.startedAt,
    activityAt: status?.startedAt,
    order: Number.MAX_SAFE_INTEGER,
  };
};

const modelSwitchStatusText = (event: StreamDisplayEvent): string => {
  const model = event.model?.trim();
  if (!model) return '';
  const next = formatAIModelRouteLabel(model);
  const failed = event.failedModel?.trim();
  if (!failed || failed === model) return `Switched to ${next}`;
  return `Provider fallback: ${formatAIModelRouteLabel(failed)} -> ${next}`;
};

const hasVisibleAssistantOutput = (message: ChatMessage): boolean => {
  if ((message.content || '').trim() || message.error) return true;

  return (message.streamEvents || []).some((event) => {
    if (event.type === 'content') return !!event.content?.trim();
    if (event.type === 'tool') return !!event.tool;
    if (event.type === 'approval') return !!event.approval;
    if (event.type === 'question') return !!event.question;
    return false;
  });
};

export const getAssistantActiveTurnStatus = (
  messages: ChatMessage[],
  isLoading: boolean,
): AssistantActiveTurnStatus | null => {
  if (!isLoading) return null;

  const assistantMessage = [...messages].reverse().find((message) => message.role === 'assistant');
  if (!assistantMessage) {
    return { type: 'thinking', text: 'Waiting for assistant' };
  }

  const statusCandidates: AssistantActiveTurnStatusCandidate[] = [];
  const statePendingToolCandidate = activePendingToolCandidateFromState(
    assistantMessage.pendingTools,
  );
  if (statePendingToolCandidate) {
    statusCandidates.push(statePendingToolCandidate);
  }
  const eventStatusCandidate = latestStreamActivityStatus(assistantMessage.streamEvents);
  if (eventStatusCandidate) {
    statusCandidates.push(eventStatusCandidate);
  } else {
    const pendingTool = activePendingToolFromEvents(assistantMessage.streamEvents);
    const eventPendingToolCandidate = pendingToolCandidate(pendingTool, 0);
    if (eventPendingToolCandidate) {
      statusCandidates.push(eventPendingToolCandidate);
    }
  }
  if (assistantMessage.isStreaming !== false) {
    const workflowCandidate = workflowStatusCandidate(assistantMessage.workflowStatus);
    if (workflowCandidate) {
      statusCandidates.push(workflowCandidate);
    }
  }

  const freshestStatus = statusCandidates.reduce<AssistantActiveTurnStatusCandidate | null>(
    (current, candidate) => (isFresherStatusCandidate(candidate, current) ? candidate : current),
    null,
  );

  if (freshestStatus) {
    return {
      type: freshestStatus.type,
      text: freshestStatus.text,
      startedAt: freshestStatus.startedAt,
    };
  }

  if (hasVisibleAssistantOutput(assistantMessage)) {
    return { type: 'generating', text: 'Generating response' };
  }

  return { type: 'thinking', text: 'Waiting for assistant' };
};
