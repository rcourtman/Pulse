export function recoveryDateKeyFromTimestamp(timestamp: number): string {
  const date = new Date(timestamp);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

export function parseRecoveryDateKey(key: string): Date {
  const [year, month, day] = key.split('-').map((value) => Number.parseInt(value, 10));
  if (!year || !month || !day) return new Date(key);
  return new Date(year, month - 1, day);
}

export function getRecoveryPrettyDateLabel(key: string): string {
  return parseRecoveryDateKey(key).toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
  });
}

export function getRecoveryFullDateLabel(key: string): string {
  return parseRecoveryDateKey(key).toLocaleDateString(undefined, {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  });
}

export function getRecoveryCompactAxisLabel(key: string, days: 7 | 30 | 90 | 365): string {
  const date = parseRecoveryDateKey(key);
  if (days <= 30) {
    if (date.getDate() === 1) return `${date.getMonth() + 1}/1`;
    return `${date.getDate()}`;
  }
  return `${date.getMonth() + 1}/${date.getDate()}`;
}

export function formatRecoveryTimeOnly(timestamp: number | null): string {
  if (!timestamp) return '—';
  return new Date(timestamp).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
}

export function getRecoveryNiceAxisMax(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 1;
  if (value <= 5) return Math.max(1, value);
  const magnitude = 10 ** Math.floor(Math.log10(value));
  const normalized = value / magnitude;
  if (normalized <= 1) return magnitude;
  if (normalized <= 2) return 2 * magnitude;
  if (normalized <= 5) return 5 * magnitude;
  return 10 * magnitude;
}
