export function formatConfidencePercentage(value: number): string {
  if (!Number.isFinite(value)) {
    return '';
  }

  return `${Math.round(value * 100)}%`;
}
