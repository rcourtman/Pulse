import { apiFetch } from '@/utils/apiClient';

// PVE backup job outcomes from `GET /api/backups/jobs`: the last 24 hours and
// each of the last 14 local days, summarized server-side from retained task
// history by the same authority as the backup health report. The browser only
// presents them; it never re-derives outcomes from the live task list, which
// spans just the latest collection window.

export type BackupJobsHistoryState = 'complete' | 'partial' | 'not_observed' | 'unavailable';

export interface BackupJobsCounts {
  success: number;
  failed: number;
  warning: number;
  running: number;
  unknown: number;
}

export interface BackupJobsFailure {
  workloadId: string;
  workloadName: string;
  instance: string;
  at: string;
  outcome: string;
}

export interface BackupJobsDay {
  date: string;
  historyState: BackupJobsHistoryState;
  counts: BackupJobsCounts;
  failures: BackupJobsFailure[];
}

export interface BackupJobsPayload {
  historyState: BackupJobsHistoryState;
  timezone: string;
  last24h: {
    historyState: BackupJobsHistoryState;
    counts: BackupJobsCounts;
    failures: BackupJobsFailure[];
  };
  days: BackupJobsDay[];
}

const HISTORY_STATES = new Set<BackupJobsHistoryState>([
  'complete',
  'partial',
  'not_observed',
  'unavailable',
]);

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);

const isCount = (value: unknown): value is number =>
  typeof value === 'number' && Number.isInteger(value) && value >= 0;

const text = (value: unknown): string => (typeof value === 'string' ? value : '');

// An unknown or missing state reads as unavailable, never as clean history.
const historyState = (value: unknown): BackupJobsHistoryState =>
  HISTORY_STATES.has(value as BackupJobsHistoryState)
    ? (value as BackupJobsHistoryState)
    : 'unavailable';

const ZERO_COUNTS: BackupJobsCounts = { success: 0, failed: 0, warning: 0, running: 0, unknown: 0 };

// Unreadable counts must not become clean zeroes: the window that carried
// them reads unavailable instead.
const parseWindow = (
  counts: unknown,
  state: unknown,
): { counts: BackupJobsCounts; historyState: BackupJobsHistoryState } => {
  const record = isRecord(counts) ? counts : null;
  const keys = Object.keys(ZERO_COUNTS) as (keyof BackupJobsCounts)[];
  if (!record || !keys.every((key) => isCount(record[key]))) {
    return { counts: { ...ZERO_COUNTS }, historyState: 'unavailable' };
  }
  const stateValue = historyState(state);
  return {
    counts:
      stateValue === 'unavailable'
        ? { ...ZERO_COUNTS }
        : (Object.fromEntries(
            keys.map((key) => [key, record[key]]),
          ) as unknown as BackupJobsCounts),
    historyState: stateValue,
  };
};

const parseFailures = (failures: unknown): BackupJobsFailure[] =>
  (Array.isArray(failures) ? failures : []).filter(isRecord).map((failure) => ({
    workloadId: text(failure.workloadId),
    workloadName: text(failure.workloadName),
    instance: text(failure.instance),
    at: text(failure.at),
    outcome: text(failure.outcome),
  }));

export function parseBackupJobsPayload(body: unknown): BackupJobsPayload | null {
  const data = isRecord(body) && isRecord(body.data) ? body.data : null;
  if (!data || !Array.isArray(data.days) || data.days.length === 0) return null;
  // Do not silently drop malformed or duplicate days and call the remaining
  // calendar a successful read. In particular, an empty calendar is not a
  // valid "no jobs" response: the server sends observed/unobserved days.
  const dates = new Set<string>();
  for (const day of data.days) {
    if (!isRecord(day) || typeof day.date !== 'string') return null;
    const date = new Date(`${day.date}T00:00:00Z`);
    if (
      !/^\d{4}-\d{2}-\d{2}$/.test(day.date) ||
      !Number.isFinite(date.valueOf()) ||
      date.toISOString().slice(0, 10) !== day.date ||
      dates.has(day.date)
    ) {
      return null;
    }
    dates.add(day.date);
  }
  const last = isRecord(data.last24h) ? data.last24h : {};
  const lastWindow = parseWindow(last.counts, last.historyState);
  const payload: BackupJobsPayload = {
    historyState: historyState(data.historyState),
    timezone: text(data.timezone),
    last24h: {
      ...lastWindow,
      failures: lastWindow.historyState === 'unavailable' ? [] : parseFailures(last.failures),
    },
    days: data.days.map((day) => {
      const record = day as Record<string, unknown>;
      const window = parseWindow(record.counts, record.historyState);
      return {
        date: text(record.date),
        ...window,
        failures: window.historyState === 'unavailable' ? [] : parseFailures(record.failures),
      };
    }),
  };
  return payload.historyState === 'unavailable' ? backupJobsUnavailable(payload) : payload;
}

export async function fetchBackupJobs(signal: AbortSignal): Promise<BackupJobsPayload | null> {
  let timezone = '';
  try {
    timezone = Intl.DateTimeFormat().resolvedOptions().timeZone ?? '';
  } catch {
    timezone = '';
  }
  const query = timezone ? `?timezone=${encodeURIComponent(timezone)}` : '';
  const response = await apiFetch(`/api/backups/jobs${query}`, { signal });
  if (!response.ok) {
    throw Object.assign(new Error(`Failed to load backup jobs (${response.status})`), {
      status: response.status,
    });
  }
  const payload = parseBackupJobsPayload(await response.json());
  if (!payload) throw new Error('Backup job history response is invalid.');
  return payload;
}

export const backupJobsTotal = (counts: BackupJobsCounts): number =>
  counts.success + counts.failed + counts.warning + counts.running + counts.unknown;

// A day Pulse did not fully observe is never drawn as a clean empty day.
export const backupJobsDayObserved = (day: BackupJobsDay): boolean =>
  day.historyState === 'complete';

// Coverage alone is not history: a PVE install without vzdump jobs collects
// successfully and finds nothing. Show the card once any task is counted, or
// when history could not be read, so a read failure is never silent.
export const backupJobsHasHistory = (payload: BackupJobsPayload | null): boolean =>
  payload !== null &&
  (payload.historyState === 'unavailable' ||
    payload.last24h.historyState === 'unavailable' ||
    payload.days.some((day) => day.historyState === 'unavailable') ||
    backupJobsTotal(payload.last24h.counts) > 0 ||
    payload.days.some((day) => backupJobsTotal(day.counts) > 0));

// A failed read after history was shown keeps the card, marked unavailable.
export const backupJobsUnavailable = (previous: BackupJobsPayload | null): BackupJobsPayload => ({
  historyState: 'unavailable',
  timezone: previous?.timezone ?? '',
  last24h: { historyState: 'unavailable', counts: { ...ZERO_COUNTS }, failures: [] },
  days: (previous?.days ?? []).map((day) => ({
    date: day.date,
    historyState: 'unavailable',
    counts: { ...ZERO_COUNTS },
    failures: [],
  })),
});
