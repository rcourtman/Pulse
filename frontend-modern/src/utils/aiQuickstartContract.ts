export const AI_QUICKSTART_EXHAUSTED_REASON =
  'Quickstart credits exhausted. Connect your API key to continue using AI Patrol.';

export const AI_QUICKSTART_ACTIVATION_REQUIRED_REASON =
  'Activate this install or start a trial to use AI Patrol quickstart. Otherwise connect your API key.';

export const AI_QUICKSTART_UNAVAILABLE_REASON =
  'Quickstart credits require internet access. Connect your API key for offline AI Patrol.';

export function normalizeQuickstartReason(reason?: string | null): string {
  return (reason ?? '').trim();
}

export function isQuickstartActivationRequiredReason(reason?: string | null): boolean {
  return normalizeQuickstartReason(reason) === AI_QUICKSTART_ACTIVATION_REQUIRED_REASON;
}

export function isQuickstartExhaustedReason(reason?: string | null): boolean {
  return normalizeQuickstartReason(reason) === AI_QUICKSTART_EXHAUSTED_REASON;
}

export function isQuickstartUnavailableReason(reason?: string | null): boolean {
  return normalizeQuickstartReason(reason) === AI_QUICKSTART_UNAVAILABLE_REASON;
}
