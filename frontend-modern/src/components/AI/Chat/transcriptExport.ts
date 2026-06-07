import type {
  ChatMessage,
  PendingApproval,
  PendingQuestion,
  PendingTool,
  StreamDisplayEvent,
  ToolCancellation,
  ToolExecution,
  WorkflowStatus,
} from './types';
import { stripAssistantOutputArtifacts } from './assistantOutputHygiene';
import { formatAssistantWorkflowStatus } from './activeTurnStatus';
import { groupStreamEventsForDisplay } from './streamEventGrouping';
import {
  getToolLabel,
  isPlaceholderToolInputSummary,
  parseToolInputSummary,
  pendingToolActionLabel,
  toolValueText,
} from './toolPresentation';
import { formatAIModelRouteLabel } from '@/utils/aiProviderPresentation';

export interface AssistantTranscriptSession {
  id?: string;
  title?: string;
}

export interface AssistantTranscriptOptions {
  generatedAt?: Date;
  includeAssistantMetadata?: boolean;
  includeThinking?: boolean;
  includeToolOutput?: boolean;
  messages: ChatMessage[];
  session?: AssistantTranscriptSession;
  getModelRouteLabel?: (modelId: string) => string;
}

const transcriptDateFormatter = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
});

const normalizeText = (value: string | undefined): string =>
  (value || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n').trim();

const compactText = (parts: Array<string | undefined>): string =>
  parts
    .map((part) => part?.trim() || '')
    .filter(Boolean)
    .join(' - ');

const appendBlock = (lines: string[], block: string) => {
  const text = normalizeText(block);
  if (!text) return false;
  lines.push(text);
  lines.push('');
  return true;
};

const formatTimestamp = (date: Date): string => {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) return '';
  return transcriptDateFormatter.format(date);
};

const formatGeneratedAt = (date: Date): string => {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) return '';
  return date.toISOString();
};

const formatModelRoute = (
  modelId: string | undefined,
  getModelRouteLabel?: (modelId: string) => string,
): string => {
  const model = modelId?.trim();
  if (!model) return '';
  return getModelRouteLabel?.(model) || formatAIModelRouteLabel(model);
};

const visibleAssistantContent = (message: ChatMessage): string =>
  stripAssistantOutputArtifacts(message.content || '').text.trim();

const visibleContentEvent = (event: StreamDisplayEvent): string =>
  stripAssistantOutputArtifacts(event.content || '').text.trim();

const formatToolInputSummary = (name: string, input: unknown, rawInput?: string): string => {
  const summary = parseToolInputSummary(toolValueText(input), name, rawInput).trim();
  if (!summary || isPlaceholderToolInputSummary(summary)) return '';
  return summary;
};

const formatToolOutput = (output: unknown): string => {
  const text = toolValueText(output).replace(/\r\n/g, '\n').replace(/\r/g, '\n').trim();
  if (!text) return '';
  const lines = text.split('\n').slice(0, 6);
  const preview = lines.map((line) => line.trimEnd().slice(0, 160)).join('\n').trim();
  if (!preview) return '';
  return text.split('\n').length > lines.length ? `${preview}\n...` : preview;
};

const formatCompletedTool = (tool: ToolExecution, includeOutput: boolean): string => {
  const label = getToolLabel(tool.name);
  const summary = formatToolInputSummary(tool.name, tool.input, tool.rawInput);
  const status = tool.success ? 'completed' : 'failed';
  const line = compactText([`[tool:${label}]${summary ? ` ${summary}` : ''}`, status]);
  const output = includeOutput ? formatToolOutput(tool.output) : '';
  return output ? `${line}\n${output}` : line;
};

const formatPendingTool = (tool: PendingTool): string => {
  const label = getToolLabel(tool.name);
  const parsedSummary = formatToolInputSummary(tool.name, tool.input, tool.rawInput);
  const summary = parsedSummary || pendingToolActionLabel(tool.name);
  const status = tool.status || 'pending';
  return compactText([
    `[tool:${label}] ${summary}`,
    status,
    tool.progress?.trim() ? `progress: ${tool.progress.trim()}` : undefined,
  ]);
};

const formatCanceledTool = (tool: ToolCancellation): string => {
  const label = getToolLabel(tool.name);
  const parsedSummary = formatToolInputSummary(tool.name, tool.input, tool.rawInput);
  const summary = parsedSummary || pendingToolActionLabel(tool.name);
  return compactText([
    `[tool:${label}] ${summary}`,
    'skipped',
    tool.reason?.trim() ? `reason: ${tool.reason.trim()}` : undefined,
  ]);
};

const formatWorkflowStatus = (status?: WorkflowStatus): string => {
  const message = formatAssistantWorkflowStatus(status).trim();
  return message ? `[status] ${message}` : '';
};

const formatModelSwitch = (
  event: StreamDisplayEvent,
  getModelRouteLabel?: (modelId: string) => string,
): string => {
  const model = formatModelRoute(event.model, getModelRouteLabel);
  if (!model) return '';
  const failed = formatModelRoute(event.failedModel, getModelRouteLabel);
  if (event.modelEvent === 'selected') return `[model] using ${model}`;
  return failed && failed !== model ? `[model] ${model} after ${failed}` : `[model] ${model}`;
};

const formatApproval = (approval: PendingApproval): string => {
  const label = getToolLabel(approval.toolName || 'approval');
  const subject = normalizeText(approval.description || approval.command) || 'action';
  const status = approval.isExecuting ? 'executing' : 'approval required';
  return compactText([
    `[approval:${label}] ${subject}`,
    status,
    approval.risk ? `${approval.risk} risk` : undefined,
  ]);
};

const formatQuestion = (question: PendingQuestion): string => {
  const prompts = question.questions
    .map((item) => normalizeText(item.question || item.header))
    .filter(Boolean);
  if (prompts.length === 0) return '';
  return `[question] ${prompts.join(' / ')}`;
};

const hasRenderableEvent = (event: StreamDisplayEvent, includeThinking = false): boolean => {
  switch (event.type) {
    case 'thinking':
      return includeThinking && Boolean(event.thinking?.trim());
    case 'workflow_status':
      return Boolean(formatWorkflowStatus(event.workflowStatus));
    case 'tool':
      return Boolean(event.tool);
    case 'pending_tool':
      return Boolean(event.pendingTool);
    case 'tool_cancel':
      return Boolean(event.toolCancel);
    case 'content':
      return Boolean(visibleContentEvent(event));
    case 'model_switch':
      return Boolean(event.model?.trim());
    case 'approval':
      return Boolean(event.approval);
    case 'question':
      return Boolean(event.question);
    default:
      return false;
  }
};

const hasTranscriptMessageContent = (message: ChatMessage, includeThinking = false): boolean => {
  if (message.role === 'user') return Boolean(normalizeText(message.content));
  if (visibleAssistantContent(message)) return true;
  if (message.error?.trim()) return true;
  if (message.toolCalls?.length) return true;
  if (message.pendingTools?.length) return true;
  if (message.pendingApprovals?.length) return true;
  if (message.pendingQuestions?.length) return true;
  return (message.streamEvents || []).some((event) => hasRenderableEvent(event, includeThinking));
};

const appendMessageMetadata = (
  lines: string[],
  message: ChatMessage,
  options: AssistantTranscriptOptions,
) => {
  if (options.includeAssistantMetadata === false) return;
  const modelRoute =
    message.role === 'assistant'
      ? formatModelRoute(message.model, options.getModelRouteLabel)
      : '';

  const metadata = [
    message.timestamp ? `Time: ${formatTimestamp(message.timestamp)}` : undefined,
    modelRoute ? `Model: ${modelRoute}` : undefined,
    message.delivery === 'queued' ? 'Delivery: queued' : undefined,
    message.isStreaming ? 'State: streaming' : undefined,
  ].filter((item): item is string => Boolean(item));

  if (metadata.length > 0) {
    lines.push(metadata.join('\n'));
    lines.push('');
  }
};

const appendAssistantStreamEvents = (
  lines: string[],
  message: ChatMessage,
  options: AssistantTranscriptOptions,
) => {
  let contentAppended = false;
  let toolAppended = false;
  let pendingToolAppended = false;
  let approvalAppended = false;
  let questionAppended = false;

  for (const event of groupStreamEventsForDisplay(message.streamEvents || [])) {
    switch (event.type) {
      case 'thinking':
        if (options.includeThinking) {
          appendBlock(lines, `[thinking]\n${event.thinking || ''}`);
        }
        break;
      case 'workflow_status':
        appendBlock(lines, formatWorkflowStatus(event.workflowStatus));
        break;
      case 'content':
        if (appendBlock(lines, visibleContentEvent(event))) {
          contentAppended = true;
        }
        break;
      case 'tool':
        if (event.tool) {
          appendBlock(lines, formatCompletedTool(event.tool, options.includeToolOutput === true));
          toolAppended = true;
        }
        break;
      case 'pending_tool':
        if (event.pendingTool) {
          appendBlock(lines, formatPendingTool(event.pendingTool));
          pendingToolAppended = true;
        }
        break;
      case 'tool_cancel':
        if (event.toolCancel) {
          appendBlock(lines, formatCanceledTool(event.toolCancel));
        }
        break;
      case 'model_switch':
        appendBlock(lines, formatModelSwitch(event, options.getModelRouteLabel));
        break;
      case 'approval':
        if (event.approval) {
          appendBlock(lines, formatApproval(event.approval));
          approvalAppended = true;
        }
        break;
      case 'question':
        if (event.question) {
          appendBlock(lines, formatQuestion(event.question));
          questionAppended = true;
        }
        break;
      default:
        break;
    }
  }

  if (!contentAppended) {
    appendBlock(lines, visibleAssistantContent(message));
  }

  if (!toolAppended) {
    for (const tool of message.toolCalls || []) {
      appendBlock(lines, formatCompletedTool(tool, options.includeToolOutput === true));
    }
  }

  if (!pendingToolAppended) {
    for (const tool of message.pendingTools || []) {
      appendBlock(lines, formatPendingTool(tool));
    }
  }

  if (!approvalAppended) {
    for (const approval of message.pendingApprovals || []) {
      appendBlock(lines, formatApproval(approval));
    }
  }

  if (!questionAppended) {
    for (const question of message.pendingQuestions || []) {
      appendBlock(lines, formatQuestion(question));
    }
  }

  if (message.error?.trim()) {
    appendBlock(lines, `[error] ${message.error.trim()}`);
  }

  if (message.interruption === 'stopped') {
    appendBlock(lines, '[interrupted] Stopped by user');
  } else if (message.interruption === 'replaced') {
    appendBlock(lines, '[interrupted] Replaced by a later user message');
  }
};

export const hasAssistantTranscriptContent = (messages: ChatMessage[]): boolean =>
  messages.some((message) => hasTranscriptMessageContent(message));

export const formatAssistantTranscript = (options: AssistantTranscriptOptions): string => {
  const includeThinking = options.includeThinking === true;
  if (!options.messages.some((message) => hasTranscriptMessageContent(message, includeThinking))) {
    return '';
  }

  const lines: string[] = ['# Pulse Assistant Transcript', ''];
  const sessionTitle = options.session?.title?.trim();
  const sessionId = options.session?.id?.trim();
  const generatedAt = formatGeneratedAt(options.generatedAt || new Date());

  if (sessionTitle) lines.push(`Session: ${sessionTitle}`);
  if (sessionId) lines.push(`Session ID: ${sessionId}`);
  if (generatedAt) lines.push(`Generated: ${generatedAt}`);
  lines.push('');

  for (const message of options.messages) {
    if (!hasTranscriptMessageContent(message, includeThinking)) continue;

    const role = message.role === 'user' ? 'User' : 'Pulse Assistant';
    lines.push(`## ${role}`);
    lines.push('');
    appendMessageMetadata(lines, message, options);

    if (message.role === 'user') {
      appendBlock(lines, message.content);
    } else {
      appendAssistantStreamEvents(lines, message, options);
    }
  }

  return `${lines.join('\n').replace(/\n{3,}/g, '\n\n').trim()}\n`;
};

const filenameDatePart = (date: Date): string => {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) return 'session';
  return date.toISOString().slice(0, 10);
};

const sanitizeFilenamePart = (value: string): string =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48);

export const buildAssistantTranscriptFilename = (
  sessionId?: string,
  generatedAt = new Date(),
): string => {
  const sessionPart = sanitizeFilenamePart(sessionId || '');
  return sessionPart
    ? `pulse-assistant-${sessionPart}.md`
    : `pulse-assistant-${filenameDatePart(generatedAt)}.md`;
};

export const downloadAssistantTranscriptFile = (transcript: string, filename: string): void => {
  const blob = new Blob([transcript], { type: 'text/markdown;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  URL.revokeObjectURL(url);
};
