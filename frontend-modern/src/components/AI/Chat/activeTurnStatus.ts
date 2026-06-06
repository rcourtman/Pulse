import type { ChatMessage, PendingTool, StreamDisplayEvent, WorkflowStatus } from './types';
import { formatIdentifierLabel } from '@/utils/textPresentation';

export type AssistantActiveTurnStatusKind = 'thinking' | 'tool' | 'generating';

export interface AssistantActiveTurnStatus {
  text: string;
  type: AssistantActiveTurnStatusKind;
}

const formatToolName = (name?: string) =>
  formatIdentifierLabel(name, { stripPrefix: 'pulse_', fallback: 'tool', maxLength: 36 });

const escapeRegExp = (value: string) => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

const messageContainsToolLabel = (message: string, label: string) => {
  if (!label) return false;
  return new RegExp(`(^|\\W)${escapeRegExp(label)}($|\\W)`, 'i').test(message);
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

  return `${message}${toolSuffix}`;
};

const formatPendingToolStatus = (tool?: PendingTool): string => {
  if (!tool) return '';

  const progress = tool.progress?.trim();
  if (progress) return progress;

  const toolLabel = formatToolName(tool.name);
  if (tool.status === 'waiting') return `Waiting on ${toolLabel}`;

  return `Running ${toolLabel}`;
};

const activePendingToolFromEvents = (
  events?: StreamDisplayEvent[],
): PendingTool | undefined => {
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

  const pendingTool =
    assistantMessage.pendingTools?.at(-1) || activePendingToolFromEvents(assistantMessage.streamEvents);
  const pendingToolText = formatPendingToolStatus(pendingTool);
  if (pendingToolText) {
    return { type: 'tool', text: pendingToolText };
  }

  const workflowStatusText = formatAssistantWorkflowStatus(assistantMessage.workflowStatus);
  if (workflowStatusText) {
    return {
      type: assistantMessage.workflowStatus?.tool ? 'tool' : 'thinking',
      text: workflowStatusText,
    };
  }

  if (hasVisibleAssistantOutput(assistantMessage)) {
    return { type: 'generating', text: 'Generating response' };
  }

  return { type: 'thinking', text: 'Waiting for assistant' };
};
