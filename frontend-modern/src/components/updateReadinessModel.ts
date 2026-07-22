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
}): PostUpdateReloadDecision => {
  // Without a known pre-update version there is nothing to compare against;
  // a healthy response is the best remaining signal.
  if (!input.preUpdateVersion) {
    return 'reload';
  }
  if (input.reportedVersion && input.reportedVersion !== input.preUpdateVersion) {
    return 'reload';
  }
  // Healthy but still the pre-update version: the about-to-exit process is
  // answering, or this deployment never restarts. Wait, but not forever.
  if (input.sameVersionHealthyAttempts >= MAX_SAME_VERSION_HEALTHY_ATTEMPTS) {
    return 'reload';
  }
  return 'wait';
};
