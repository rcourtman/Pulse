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
  return {
    counts: Object.fromEntries(
      keys.map((key) => [key, record[key]]),
    ) as unknown as BackupJobsCounts,
    historyState: historyState(state),
  };
};

export function parseBackupJobsPayload(body: unknown): BackupJobsPayload | null {
  const data = isRecord(body) && isRecord(body.data) ? body.data : null;
  if (!data || !Array.isArray(data.days)) return null;
  const last = isRecord(data.last24h) ? data.last24h : {};
  return {
    historyState: historyState(data.historyState),
    timezone: text(data.timezone),
    last24h: {
      ...parseWindow(last.counts, last.historyState),
      failures: (Array.isArray(last.failures) ? last.failures : [])
        .filter(isRecord)
        .map((failure) => ({
          workloadId: text(failure.workloadId),
          workloadName: text(failure.workloadName),
          instance: text(failure.instance),
          at: text(failure.at),
          outcome: text(failure.outcome),
        })),
    },
    days: data.days.filter(isRecord).map((day) => ({
      date: text(day.date),
      ...parseWindow(day.counts, day.historyState),
    })),
  };
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
  return parseBackupJobsPayload(await response.json());
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
    backupJobsTotal(payload.last24h.counts) > 0 ||
    payload.days.some((day) => backupJobsTotal(day.counts) > 0));

// A failed read after history was shown keeps the card, marked unavailable.
export const backupJobsUnavailable = (previous: BackupJobsPayload): BackupJobsPayload => ({
  ...previous,
  historyState: 'unavailable',
  last24h: { historyState: 'unavailable', counts: { ...ZERO_COUNTS }, failures: [] },
  days: previous.days.map((day) => ({
    ...day,
    historyState: 'unavailable',
    counts: { ...ZERO_COUNTS },
  })),
});
