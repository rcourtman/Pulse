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

// Session spend is cumulative and backend-estimated; it is absent whenever
// any of the session's models has no known pricing, so no figure beats a
// wrong figure. Sub-cent totals round up to a floor instead of showing $0.00.
export const formatSessionCostUSD = (usd: number | undefined): string => {
  if (!Number.isFinite(usd) || !usd || usd <= 0) return '';
  if (usd < 0.01) return '<$0.01';
  return `$${usd.toFixed(2)}`;
};

const assistantTokenSummary = (message: ChatMessage) => {
  const input = normalizeAssistantTokenCount(message.tokens?.input);
  const output = normalizeAssistantTokenCount(message.tokens?.output);
  if (output <= 0) return null;

  const total = input + output;
  // How full the model's context window was this turn (input side carries
  // the conversation), so the operator can see compaction coming.
  const contextLimit = normalizeAssistantTokenCount(message.tokens?.contextLimit);
  const contextPercent =
    contextLimit > 0 && input > 0 ? Math.min(100, Math.round((input / contextLimit) * 100)) : 0;
  const contextLabel = contextPercent > 0 ? ` (${contextPercent}% of context)` : '';
  const contextDetail =
    contextPercent > 0
      ? `, context ${formatAssistantTokenCount(input)} of ${formatAssistantTokenCount(contextLimit)} (${contextPercent}%)`
      : '';
  return {
    label: `${formatAssistantTokenCount(total)} ${total === 1 ? 'token' : 'tokens'}${contextLabel}`,
    detail: `${formatAssistantTokenCount(total)} total, ${formatAssistantTokenCount(
      input,
    )} input, ${formatAssistantTokenCount(output)} output${contextDetail}`,
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
  const sessionCost = formatSessionCostUSD(message.tokens?.sessionCostUsd);
  const sessionCostLabel = sessionCost ? `${sessionCost} session` : '';

  const labelParts = [modelLabel, durationLabel, tokenSummary?.label, sessionCostLabel].filter(
    (part): part is string => Boolean(part?.trim()),
  );
  if (labelParts.length === 0) return null;

  const titleParts = [
    modelLabel ? `Model: ${modelLabel}` : undefined,
    durationLabel ? `Duration: ${durationLabel}` : undefined,
    tokenSummary ? `Usage: ${tokenSummary.detail}` : undefined,
    sessionCost ? `Estimated session cost: ${sessionCost}` : undefined,
  ].filter((part): part is string => Boolean(part));

  return {
    label: `Last turn: ${labelParts.join(' · ')}`,
    title: `${AI_CHAT_LAST_TURN_SUMMARY_LABEL}: ${titleParts.join('. ')}`,
  };
};
