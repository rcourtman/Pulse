export const PATROL_SUPPORTING_CONTEXT_TITLE = 'Supporting context';
export const PATROL_SUPPORTING_CONTEXT_DESCRIPTION =
  'Recent changes, learned correlations, and policy coverage that may explain findings or incomplete verification.';
export const PATROL_SUPPORTING_CONTEXT_EVIDENCE_LABEL = 'How to read this';
export const PATROL_SUPPORTING_CONTEXT_EVIDENCE_NOTE =
  'Findings and run history are Patrol verification evidence. The cards below add explanatory context and do not count as a fresh full patrol.';
export const PATROL_SUPPORTING_CONTEXT_CHANGE_SUBTITLE =
  'Observed from the canonical timeline in the last 24 hours.';
export const PATROL_SUPPORTING_CONTEXT_POLICY_SUBTITLE =
  'Coverage posture for policy-covered resources.';

export function getPatrolSupportingContextToggleLabel(expanded: boolean) {
  return expanded ? 'Hide supporting context' : 'View supporting context';
}

export function getPatrolSupportingContextCorrelationSummary(count: number) {
  if (!Number.isFinite(count) || count <= 0) {
    return 'Learned pattern context';
  }
  return `${count} learned pattern${count === 1 ? '' : 's'} · explanatory context`;
}
