import type { PatrolAutonomyLevel, PatrolObjective } from '@/api/patrol';
import type { AttentionItem } from '@/api/patrolAttention';

export interface PatrolAutonomyExperience {
  label: string;
  summary: string;
  needsYouDescription: string;
  quietWorkDescription: string;
}

export const PATROL_AUTONOMY_EXPERIENCE: Record<PatrolAutonomyLevel, PatrolAutonomyExperience> = {
  monitor: {
    label: 'Watch only',
    summary: 'Patrol watches your infrastructure. You decide what happens next.',
    needsYouDescription:
      'Patrol reports current issues here because Watch only never makes infrastructure changes.',
    quietWorkDescription: 'Patrol continues watching these issues without making changes.',
  },
  approval: {
    label: 'Ask first',
    summary: 'Patrol investigates and prepares fixes, then waits for your approval.',
    needsYouDescription:
      'These issues need your decision before Patrol can make a change or close the loop.',
    quietWorkDescription: 'Patrol continues investigating while it waits for the decisions above.',
  },
  assisted: {
    label: 'Safe auto-fix',
    summary: 'Patrol handles policy-allowed safe work and asks when risk or evidence needs you.',
    needsYouDescription:
      'Patrol only interrupts when policy, risk, missing evidence, or verification requires your decision.',
    quietWorkDescription:
      'Other current issues can proceed within the safe auto-fix policy without a decision from you.',
  },
  full: {
    label: 'Autopilot',
    summary:
      'Patrol handles allowed work in the background and interrupts only when it cannot proceed safely.',
    needsYouDescription:
      'Patrol only interrupts when it is blocked, lacks trustworthy evidence, or still requires approval.',
    quietWorkDescription:
      'Other current issues do not require a decision under Autopilot and remain visible in Alerts.',
  },
};

export interface PatrolAttentionDecision {
  item: AttentionItem;
  reason: string;
}

const itemHasTrustGap = (item: AttentionItem): boolean =>
  item.state === 'unknown' ||
  item.evidenceFreshness !== 'fresh' ||
  item.evidenceCompleteness !== 'complete' ||
  item.verificationState === 'failed' ||
  item.verificationState === 'unknown';

export function getPatrolAttentionDecisionReason(
  item: AttentionItem,
  autonomyLevel: PatrolAutonomyLevel,
  autonomyLocked = false,
): string | null {
  const effectiveLevel = autonomyLocked ? 'monitor' : autonomyLevel;

  if (effectiveLevel === 'monitor') {
    return 'Watch only will not make changes. Review this issue and decide what should happen.';
  }

  if (effectiveLevel === 'approval') {
    return item.availableActions.length > 0
      ? 'Patrol has an action to review, but Ask first requires your approval.'
      : 'Patrol cannot complete this issue without your decision.';
  }

  if (itemHasTrustGap(item)) {
    return item.verificationState === 'failed' || item.verificationState === 'unknown'
      ? 'Patrol could not verify the outcome and needs you to review the evidence.'
      : 'Patrol does not have trustworthy enough evidence to proceed automatically.';
  }

  const eligibleActions = item.availableActions.filter(
    (action) => action.eligibility === 'eligible',
  );
  if (eligibleActions.length === 0) {
    return item.availableActions.length > 0
      ? 'Available actions are blocked or ineligible, so Patrol needs your decision.'
      : 'Patrol has no governed action for this issue and needs you to decide what happens next.';
  }

  const canProceedWithoutAnotherDecision = eligibleActions.some(
    (action) =>
      action.approval === 'not-required' ||
      action.approval === 'granted' ||
      (!action.requiresApproval && action.approval !== 'denied'),
  );
  if (!canProceedWithoutAnotherDecision) {
    return 'A governed action still requires your approval or a policy decision.';
  }

  return null;
}

export function partitionPatrolAttention(
  items: AttentionItem[],
  autonomyLevel: PatrolAutonomyLevel,
  autonomyLocked = false,
): { needsUser: PatrolAttentionDecision[]; quiet: AttentionItem[] } {
  const needsUser: PatrolAttentionDecision[] = [];
  const quiet: AttentionItem[] = [];

  for (const item of items) {
    const reason = getPatrolAttentionDecisionReason(item, autonomyLevel, autonomyLocked);
    if (reason) {
      needsUser.push({ item, reason });
    } else {
      quiet.push(item);
    }
  }

  return { needsUser, quiet };
}

export interface PatrolObjectiveProtectionSummary {
  active: number;
  covered: number;
  needsCoverage: number;
  paused: number;
  headline: string;
  detail: string;
  tone: 'success' | 'warning' | 'neutral';
}

export function getPatrolObjectiveProtectionSummary(
  objectives: PatrolObjective[],
): PatrolObjectiveProtectionSummary {
  const activeObjectives = objectives.filter((objective) => objective.status === 'active');
  const covered = activeObjectives.filter(
    (objective) => objective.coverage.state === 'covered',
  ).length;
  const needsCoverage = activeObjectives.length - covered;
  const paused = objectives.filter((objective) => objective.status === 'paused').length;

  if (activeObjectives.length === 0) {
    return {
      active: 0,
      covered: 0,
      needsCoverage: 0,
      paused,
      headline: 'No outcomes protected yet',
      detail:
        paused > 0
          ? `${paused} ${paused === 1 ? 'objective is' : 'objectives are'} paused.`
          : 'Add the outcomes you want Patrol to keep true.',
      tone: 'neutral',
    };
  }

  if (needsCoverage === 0) {
    return {
      active: activeObjectives.length,
      covered,
      needsCoverage: 0,
      paused,
      headline: `Protecting ${covered} ${covered === 1 ? 'outcome' : 'outcomes'}`,
      detail: 'Every active objective has current background coverage.',
      tone: 'success',
    };
  }

  return {
    active: activeObjectives.length,
    covered,
    needsCoverage,
    paused,
    headline: `${covered} of ${activeObjectives.length} ${activeObjectives.length === 1 ? 'outcome' : 'outcomes'} protected`,
    detail: `${needsCoverage} ${needsCoverage === 1 ? 'objective needs' : 'objectives need'} monitoring coverage before Patrol can rely on them.`,
    tone: 'warning',
  };
}

export function isVerifiedPatrolReceipt(item: AttentionItem): boolean {
  return item.state === 'resolved' && item.verificationState === 'succeeded';
}

export function getVerifiedPatrolReceiptSummary(item: AttentionItem): string {
  return item.plainLanguageSummary;
}
