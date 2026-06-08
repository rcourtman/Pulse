import { AI_CHAT_LAST_TURN_SUMMARY_LABEL } from '@/utils/aiChatPresentation';
import type { ChatMessage } from './types';

export interface AssistantTurnSummary {
  label: string;
  title: string;
}

const tokenNumberFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 });

const normalizeAssistantTokenCount = (value: number | undefined): number => {
  if (!Number.isFinite(value) || !value || value < 0) return 0;
  return Math.floor(value);
};

const formatAssistantTokenCount = (count: number): string => tokenNumberFormat.format(count);

export const formatAssistantTurnDuration = (startedAt: Date, completedAt?: Date): string => {
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

const assistantTokenSummary = (message: ChatMessage) => {
  const input = normalizeAssistantTokenCount(message.tokens?.input);
  const output = normalizeAssistantTokenCount(message.tokens?.output);
  if (output <= 0) return null;

  const total = input + output;
  return {
    label: `${formatAssistantTokenCount(total)} ${total === 1 ? 'token' : 'tokens'}`,
    detail: `${formatAssistantTokenCount(total)} total, ${formatAssistantTokenCount(
      input,
    )} input, ${formatAssistantTokenCount(output)} output`,
  };
};

export const getAssistantTurnSummary = (
  message: ChatMessage,
  options: { getModelRouteLabel?: (modelId: string) => string } = {},
): AssistantTurnSummary | null => {
  if (message.role !== 'assistant' || message.isStreaming) return null;

  const model = message.model?.trim();
  const modelLabel = model ? options.getModelRouteLabel?.(model) || model : '';
  const durationLabel = formatAssistantTurnDuration(message.timestamp, message.completedAt);
  const tokenSummary = assistantTokenSummary(message);

  const labelParts = [modelLabel, durationLabel, tokenSummary?.label].filter(
    (part): part is string => Boolean(part?.trim()),
  );
  if (labelParts.length === 0) return null;

  const titleParts = [
    modelLabel ? `Model: ${modelLabel}` : undefined,
    durationLabel ? `Duration: ${durationLabel}` : undefined,
    tokenSummary ? `Usage: ${tokenSummary.detail}` : undefined,
  ].filter((part): part is string => Boolean(part));

  return {
    label: `Last turn: ${labelParts.join(' · ')}`,
    title: `${AI_CHAT_LAST_TURN_SUMMARY_LABEL}: ${titleParts.join('. ')}`,
  };
};
