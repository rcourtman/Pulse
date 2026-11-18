import { createSessionActivityRecorder } from './sessionActivity';

export type WorkloadHistoryActivity = 'preview' | 'scrub' | 'range_change' | 'details_selected';

const recorder = createSessionActivityRecorder<WorkloadHistoryActivity>({
  endpoint: '/api/usage/workload-history',
  storageKeyPrefix: 'pulse:workload-history-activity:v1:',
});

/**
 * Records one content-free workload-history milestone at most once per browser
 * session. The server persists only a daily counter for the closed activity
 * enum; no guest, user, route, coordinate, timing, or browser identity is sent.
 */
export const recordWorkloadHistoryActivity = (activity: WorkloadHistoryActivity): void =>
  recorder.record(activity);

export const resetWorkloadHistoryActivityForTest = (): void => recorder.resetForTest();
