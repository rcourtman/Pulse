export const ASSISTANT_FAST_TOOL_COMPLETION_SETTLE_MS = 900;

export const getAssistantFastToolCompletionSettleUntil = (
  startedAt: number | undefined,
  completedAt: number | undefined,
  now = Date.now(),
): number | undefined => {
  if (!startedAt || !completedAt) return undefined;

  const durationMs = completedAt - startedAt;
  if (
    !Number.isFinite(durationMs) ||
    durationMs < 0 ||
    durationMs >= ASSISTANT_FAST_TOOL_COMPLETION_SETTLE_MS
  ) {
    return undefined;
  }

  return now + (ASSISTANT_FAST_TOOL_COMPLETION_SETTLE_MS - durationMs);
};
