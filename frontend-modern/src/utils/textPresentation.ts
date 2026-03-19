export interface HumanizeTokenOptions {
  fallback?: string;
  preserveShortAllCaps?: boolean;
}

export function humanizeToken(value?: string, options?: HumanizeTokenOptions): string {
  const normalized = (value || '').trim();
  if (!normalized) {
    return options?.fallback ?? '';
  }

  if (options?.preserveShortAllCaps && normalized === normalized.toUpperCase() && normalized.length <= 4) {
    return normalized;
  }

  return normalized
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase());
}
