import { apiFetchJSON } from '@/utils/apiClient';

export interface SessionActivityRecorder<Activity extends string> {
  /** Posts the milestone unless this browser session already did. */
  record: (activity: Activity) => void;
  /** Forgets the in-memory guard, as a page reload would. */
  resetForTest: () => void;
}

/**
 * Builds a recorder for content-free usage milestones. Each value of a closed
 * set is posted to the local Pulse server at most once per browser session,
 * as `{ "activity": <value> }` and nothing else. The server keeps only a
 * daily count per value. A failed post is forgotten so a later call retries.
 */
export const createSessionActivityRecorder = <Activity extends string>(options: {
  endpoint: string;
  storageKeyPrefix: string;
}): SessionActivityRecorder<Activity> => {
  const recordedThisPage = new Set<Activity>();
  const sessionKey = (activity: Activity): string => `${options.storageKeyPrefix}${activity}`;

  const wasRecordedThisSession = (activity: Activity): boolean => {
    if (recordedThisPage.has(activity)) return true;
    if (typeof window === 'undefined') return false;
    try {
      return window.sessionStorage.getItem(sessionKey(activity)) === 'true';
    } catch {
      return false;
    }
  };

  const rememberThisSession = (activity: Activity): void => {
    recordedThisPage.add(activity);
    if (typeof window === 'undefined') return;
    try {
      window.sessionStorage.setItem(sessionKey(activity), 'true');
    } catch {
      // The in-memory guard still prevents a per-interaction request stream.
    }
  };

  const forgetFailedAttempt = (activity: Activity): void => {
    recordedThisPage.delete(activity);
    if (typeof window === 'undefined') return;
    try {
      window.sessionStorage.removeItem(sessionKey(activity));
    } catch {
      // A later page load may retry if storage is unavailable.
    }
  };

  return {
    record: (activity) => {
      if (wasRecordedThisSession(activity)) return;
      rememberThisSession(activity);

      void apiFetchJSON(options.endpoint, {
        method: 'POST',
        body: JSON.stringify({ activity }),
      }).catch(() => {
        forgetFailedAttempt(activity);
      });
    },
    resetForTest: () => recordedThisPage.clear(),
  };
};
