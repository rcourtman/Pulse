export function recoveryDateKeyFromTimestamp(timestamp: number): string {
  const date = new Date(timestamp);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

const RECOVERY_DATE_SEARCH_MONTHS = [
  ['january', 'jan'],
  ['february', 'feb'],
  ['march', 'mar'],
  ['april', 'apr'],
  ['may', 'may'],
  ['june', 'jun'],
  ['july', 'jul'],
  ['august', 'aug'],
  ['september', 'sep', 'sept'],
  ['october', 'oct'],
  ['november', 'nov'],
  ['december', 'dec'],
] as const;

const RECOVERY_DATE_SEARCH_WEEKDAYS = new Set([
  'monday',
  'mon',
  'tuesday',
  'tue',
  'wednesday',
  'wed',
  'thursday',
  'thu',
  'friday',
  'fri',
  'saturday',
  'sat',
  'sunday',
  'sun',
]);

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

export function getRecoveryFilterDateLabel(key: string): string {
  return parseRecoveryDateKey(key).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

function getRecoveryDateSearchMonthIndex(value: string): number | null {
  const normalized = value.toLowerCase();
  const index = RECOVERY_DATE_SEARCH_MONTHS.findIndex((aliases) =>
    (aliases as readonly string[]).includes(normalized),
  );
  return index >= 0 ? index : null;
}

function getRecoveryDateSearchKey(year: number, monthIndex: number, day: number): string | null {
  if (!Number.isInteger(year) || !Number.isInteger(monthIndex) || !Number.isInteger(day)) {
    return null;
  }
  const date = new Date(year, monthIndex, day);
  if (date.getFullYear() !== year || date.getMonth() !== monthIndex || date.getDate() !== day) {
    return null;
  }
  return recoveryDateKeyFromTimestamp(date.getTime());
}

export function normalizeRecoveryDateSearchText(value: string): string {
  return String(value || '')
    .toLowerCase()
    .replace(/\b(\d{1,2})(st|nd|rd|th)\b/g, '$1')
    .replace(/[^a-z0-9]+/g, ' ')
    .trim()
    .replace(/\s+/g, ' ');
}

export function resolveRecoveryDateSearchKey(
  value: string,
  candidateKeys: readonly string[] = [],
  now: Date = new Date(),
): string | null {
  const normalized = normalizeRecoveryDateSearchText(value);
  if (normalized.length < 5) return null;

  const isoMatch = normalized.match(/^(\d{4}) (\d{1,2}) (\d{1,2})$/);
  if (isoMatch) {
    return getRecoveryDateSearchKey(
      Number.parseInt(isoMatch[1], 10),
      Number.parseInt(isoMatch[2], 10) - 1,
      Number.parseInt(isoMatch[3], 10),
    );
  }

  const tokens = normalized.split(' ').filter(Boolean);
  if (RECOVERY_DATE_SEARCH_WEEKDAYS.has(tokens[0])) tokens.shift();
  if (tokens.length < 2 || tokens.length > 3) return null;

  const monthIndex = getRecoveryDateSearchMonthIndex(tokens[0]);
  const day = Number.parseInt(tokens[1], 10);
  if (monthIndex === null || !Number.isInteger(day)) return null;

  if (tokens[2]) {
    if (!/^\d{4}$/.test(tokens[2])) return null;
    return getRecoveryDateSearchKey(Number.parseInt(tokens[2], 10), monthIndex, day);
  }

  const candidateMatches = candidateKeys
    .map((key) => String(key || '').trim())
    .filter(Boolean)
    .filter((key) => {
      const date = parseRecoveryDateKey(key);
      return date.getMonth() === monthIndex && date.getDate() === day;
    });
  const uniqueCandidateMatches = [...new Set(candidateMatches)];
  if (uniqueCandidateMatches.length === 1) return uniqueCandidateMatches[0];
  if (uniqueCandidateMatches.length > 1) return null;

  return getRecoveryDateSearchKey(now.getFullYear(), monthIndex, day);
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
