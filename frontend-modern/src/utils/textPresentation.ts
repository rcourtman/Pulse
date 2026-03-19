export interface HumanizeTokenOptions {
  fallback?: string;
  preserveShortAllCaps?: boolean;
}

export interface IdentifierLabelOptions {
  fallback?: string;
  maxLength?: number;
  stripPrefix?: string;
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

export function formatIdentifierLabel(
  value?: string,
  options?: IdentifierLabelOptions,
): string {
  let normalized = (value || '').trim();
  if (!normalized) {
    return options?.fallback ?? '';
  }

  if (options?.stripPrefix && normalized.startsWith(options.stripPrefix)) {
    normalized = normalized.slice(options.stripPrefix.length).trim();
  }

  if (!normalized) {
    return options?.fallback ?? '';
  }

  const label = normalized.replace(/_/g, ' ');
  if (options?.maxLength && options.maxLength > 0) {
    return label.substring(0, options.maxLength);
  }

  return label;
}
