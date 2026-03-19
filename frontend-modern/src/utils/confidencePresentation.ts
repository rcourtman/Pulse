import { humanizeToken } from '@/utils/textPresentation';

export function formatConfidencePercentage(value: number): string {
  if (!Number.isFinite(value)) {
    return '';
  }

  return `${Math.round(value * 100)}%`;
}

export function formatConfidenceLabel(value?: string | number | null): string {
  if (typeof value === 'number') {
    return formatConfidencePercentage(value);
  }

  if (typeof value === 'string') {
    const normalized = value.trim();
    if (!normalized) {
      return '';
    }

    return humanizeToken(normalized, { fallback: '' });
  }

  return '';
}
