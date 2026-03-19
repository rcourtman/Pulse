export function normalizeChatMentionKeyPart(value?: string | null): string {
  return (value || '').trim().toLowerCase();
}

export function normalizeChatToolName(value?: string | null): string {
  let normalized = (value || '').trim();
  while (normalized.startsWith('pulse_')) {
    normalized = normalized.slice('pulse_'.length).trim();
  }
  return normalized;
}
