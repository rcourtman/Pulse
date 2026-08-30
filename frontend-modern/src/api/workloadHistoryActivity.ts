import { apiFetchJSON } from '@/utils/apiClient';

export type WorkloadHistoryActivity = 'preview' | 'scrub' | 'range_change' | 'details_selected';

const SESSION_KEY_PREFIX = 'pulse:workload-history-activity:v1:';
const recordedThisPage = new Set<WorkloadHistoryActivity>();

const sessionKey = (activity: WorkloadHistoryActivity): string =>
  `${SESSION_KEY_PREFIX}${activity}`;

const wasRecordedThisSession = (activity: WorkloadHistoryActivity): boolean => {
  if (recordedThisPage.has(activity)) return true;
  if (typeof window === 'undefined') return false;
  try {
    return window.sessionStorage.getItem(sessionKey(activity)) === 'true';
  } catch {
    return false;
  }
};

const rememberThisSession = (activity: WorkloadHistoryActivity): void => {
  recordedThisPage.add(activity);
  if (typeof window === 'undefined') return;
  try {
    window.sessionStorage.setItem(sessionKey(activity), 'true');
  } catch {
    // The in-memory guard still prevents a per-interaction request stream.
  }
};

const forgetFailedAttempt = (activity: WorkloadHistoryActivity): void => {
  recordedThisPage.delete(activity);
  if (typeof window === 'undefined') return;
  try {
    window.sessionStorage.removeItem(sessionKey(activity));
  } catch {
    // A later page load may retry if storage is unavailable.
  }
};

/**
 * Records one content-free workload-history milestone at most once per browser
 * session. The server persists only a daily counter for the closed activity
 * enum; no guest, user, route, coordinate, timing, or browser identity is sent.
 */
export const recordWorkloadHistoryActivity = (activity: WorkloadHistoryActivity): void => {
  if (wasRecordedThisSession(activity)) return;
  rememberThisSession(activity);

  void apiFetchJSON('/api/usage/workload-history', {
    method: 'POST',
    body: JSON.stringify({ activity }),
  }).catch(() => {
    forgetFailedAttempt(activity);
  });
};

export const resetWorkloadHistoryActivityForTest = (): void => {
  recordedThisPage.clear();
};
