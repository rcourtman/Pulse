// Decision logic for the post-update reload in UpdateProgressModal.
//
// During a self-update the backend emits 'restarting'/'completed' and then
// schedules its own exit a couple of seconds later (see
// internal/updates/manager.go), so the OLD process is still serving — and
// still healthy — when the frontend first probes. Reloading on "healthy"
// alone therefore reloads the old bundle before the restart, and nothing
// re-triggers afterwards, stranding the user on the old version. The only
// trustworthy restart signal is the reported version moving off the version
// that started the update (works for rollbacks too).

// Healthy-but-same-version responses before giving up and reloading anyway.
// Covers deployments where the process intentionally never exits (mock/CI)
// and re-applies of an identical version. With the modal's backoff schedule
// this allows roughly 30-45 seconds for a real restart to surface.
export const MAX_SAME_VERSION_HEALTHY_ATTEMPTS = 6;

export type PostUpdateReloadDecision = 'reload' | 'wait';

export const resolvePostUpdateReload = (input: {
  preUpdateVersion: string | null;
  reportedVersion: string;
  sameVersionHealthyAttempts: number;
  // Whether the backend itself reported 'restarting' or 'completed'. Without
  // that, an unchanged version may simply mean the update is still running,
  // so the same-version fallback must never fire.
  completionConfirmed?: boolean;
  // Whether a probe failed (connection refused or a non-OK answer) after
  // completion was confirmed, i.e. the old process was seen going away.
  restartObserved?: boolean;
}): PostUpdateReloadDecision => {
  // Without a known pre-update version a healthy answer cannot tell the old
  // process from the new one. Never reload before the backend confirms
  // completion. After that, reload once the old process was seen going away
  // and something healthy answers again, or after the bounded fallback.
  if (!input.preUpdateVersion) {
    if (input.completionConfirmed !== true) {
      return 'wait';
    }
    if (input.restartObserved === true) {
      return 'reload';
    }
    return input.sameVersionHealthyAttempts >= MAX_SAME_VERSION_HEALTHY_ATTEMPTS
      ? 'reload'
      : 'wait';
  }
  if (input.reportedVersion && input.reportedVersion !== input.preUpdateVersion) {
    return 'reload';
  }
  // Healthy but still the pre-update version: the about-to-exit process is
  // answering, or this deployment never restarts. Wait, but not forever,
  // and only once completion is confirmed. Unconfirmed, the modal's stall
  // state offers a manual reload instead.
  if (input.completionConfirmed === false) {
    return 'wait';
  }
  if (input.sameVersionHealthyAttempts >= MAX_SAME_VERSION_HEALTHY_ATTEMPTS) {
    return 'reload';
  }
  return 'wait';
};

// The progress stream can go quiet without ever erroring: a reverse proxy or
// compressing intermediary can hold events back, and a held connection never
// reports the old process exiting. After this much SSE silence during an
// in-progress update the modal also polls /api/updates/status, while leaving
// the stream open in case it recovers.
export const UPDATE_STREAM_SILENCE_FALLBACK_MS = 6000;

// Cadence of the /api/updates/status fallback poll.
export const UPDATE_STATUS_POLL_INTERVAL_MS = 2500;

// With no new stage, progress, or restart signal for this long, stop implying
// the update is still running and offer a reload so the user can check.
export const UPDATE_PROGRESS_STALL_TIMEOUT_MS = 120_000;

const UPDATE_IN_PROGRESS_STAGES = new Set([
  'downloading',
  'verifying',
  'extracting',
  'backing-up',
  'applying',
  'restoring',
]);

// Consecutive failed status polls, with the stream closed and the update at
// a stage that restarts the service, before the modal starts probing for the
// restarted backend.
export const RESTART_SUSPECT_POLL_FAILURES = 2;

// Stages after which the service restarts on success.
const UPDATE_LATE_STAGES = new Set(['applying', 'restoring', 'restarting', 'completed']);

export const isLateUpdateStage = (status: string | undefined): boolean =>
  status !== undefined && UPDATE_LATE_STAGES.has(status);

// Update checks share the status channel with the apply lifecycle.
const UPDATE_CHECK_STATUSES = new Set(['checking', 'available']);

export const isUpdateInProgressStage = (status: string | undefined): boolean =>
  status !== undefined && UPDATE_IN_PROGRESS_STAGES.has(status);

// SSE and polling both feed the modal, and a rate-limited or slow poll can
// answer with a stage the stream already moved past. Once an apply is under
// way, ignore update-check chatter and in-progress stages that would move
// the progress backwards.
export const shouldApplyUpdateStatus = (
  current: { status: string; progress: number } | null,
  next: { status: string; progress: number },
): boolean => {
  if (!current || !isUpdateInProgressStage(current.status)) {
    return true;
  }
  if (UPDATE_CHECK_STATUSES.has(next.status)) {
    return false;
  }
  if (isUpdateInProgressStage(next.status) && next.progress < current.progress) {
    return false;
  }
  return true;
};
