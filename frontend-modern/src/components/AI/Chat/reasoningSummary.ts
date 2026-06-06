export const extractReasoningSummaryTitle = (content?: string): string => {
  const match = content?.trim().match(/^\*\*([^*\n]+)\*\*(?:\r?\n\r?\n|$)/);
  return match?.[1]?.trim().replace(/\s+/g, ' ') || '';
};
