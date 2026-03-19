export interface HumanizeTokenOptions {
  fallback?: string;
  preserveShortAllCaps?: boolean;
}

export interface IdentifierLabelOptions {
  fallback?: string;
  maxLength?: number;
  stripPrefix?: string;
}

export interface TitleCaseLabelOptions {
  fallback?: string;
  preserveShortAllCaps?: boolean;
  separators?: RegExp;
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

export function titleCaseDelimitedLabel(
  value?: string,
  options?: TitleCaseLabelOptions,
): string {
  const normalized = (value || '').trim();
  if (!normalized) {
    return options?.fallback ?? '';
  }

  const tokens = normalized
    .split(options?.separators ?? /[\s_-]+/)
    .map((part) => part.trim())
    .filter(Boolean);

  if (!tokens.length) {
    return options?.fallback ?? '';
  }

  return tokens
    .map((part) => {
      if (options?.preserveShortAllCaps && part === part.toUpperCase() && part.length <= 4) {
        return part;
      }
      return part.charAt(0).toUpperCase() + part.slice(1);
    })
    .join(' ');
}

export function humanizeArrowDelimitedLabel(
  value?: string,
  options?: HumanizeTokenOptions,
): string {
  return humanizeToken((value || '').replace(/\s*->\s*/g, ' → '), options);
}
