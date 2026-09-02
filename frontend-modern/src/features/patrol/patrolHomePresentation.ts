import type { PatrolAutonomyLevel, PatrolObjective } from '@/api/patrol';
import type { AttentionFlapping, AttentionItem } from '@/api/patrolAttention';
import type { UnifiedFinding } from '@/stores/aiIntelligence';

export interface PatrolAutonomyExperience {
  label: string;
  summary: string;
  needsYouDescription: string;
  quietWorkDescription: string;
}

export const PATROL_AUTONOMY_EXPERIENCE: Record<PatrolAutonomyLevel, PatrolAutonomyExperience> = {
  monitor: {
    label: 'Watch only',
    summary: 'Patrol checks continuously and keeps every infrastructure decision with you.',
    needsYouDescription: 'Every current issue stays in your queue for review in Watch only mode.',
    quietWorkDescription: 'Patrol continues watching these issues without making changes.',
  },
  approval: {
    label: 'Ask first',
    summary: 'Patrol investigates and prepares fixes, then waits for your approval.',
    needsYouDescription:
      'These issues are waiting for a decision or approval before work can continue.',
    quietWorkDescription: 'Patrol continues investigating while it waits for the decisions above.',
  },
  assisted: {
    label: 'Safe auto-fix',
    summary: 'Patrol handles policy-allowed safe work and asks when risk or evidence needs you.',
    needsYouDescription:
      'This queue contains only work blocked by policy, risk, missing evidence, or verification.',
    quietWorkDescription:
      'Other current issues can proceed within the safe auto-fix policy without a decision from you.',
  },
  full: {
    label: 'Autopilot',
    summary:
      'Patrol handles allowed work in the background and interrupts only when it cannot proceed safely.',
    needsYouDescription: 'This queue contains only work Patrol cannot complete safely without you.',
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

export interface FlappingPresentation {
  label: string;
  detail: string;
}

const formatFlappingWindow = (windowHours: number): string =>
  windowHours === 24 ? 'the last day' : `the last ${windowHours} hours`;

/**
 * One label for flapping across alerts and Patrol findings. The backend
 * decides *whether* an item is flapping (four or more open/resolved
 * transitions in 24 hours); this only phrases the count it reports.
 */
export function getFlappingPresentation(
  transitionCount: number,
  windowHours: number,
): FlappingPresentation {
  const changes = transitionCount === 1 ? 'change' : 'changes';
  return {
    label: `Flapping · ${transitionCount} ${changes} in ${windowHours}h`,
    detail: `This issue has switched between open and resolved ${transitionCount} times in ${formatFlappingWindow(windowHours)}. It is shown once here. Every transition is listed under the timeline.`,
  };
}

export function getAttentionFlappingPresentation(
  flapping: AttentionFlapping | undefined,
): FlappingPresentation | null {
  if (!flapping || flapping.transitionCount <= 0) return null;
  return getFlappingPresentation(flapping.transitionCount, flapping.windowHours);
}

const normalizeResourceKey = (value: string | undefined): string =>
  (value ?? '').trim().toLowerCase();

/**
 * Patrol findings that restate this attention item's alert: same resource and
 * the backend-stamped mirror type equals the alert kind. Resolved findings are
 * history and are not linked.
 */
export function getLinkedPatrolFindings(
  item: AttentionItem,
  findings: readonly UnifiedFinding[],
): UnifiedFinding[] {
  const resource = normalizeResourceKey(item.subjectResourceId);
  const kind = normalizeResourceKey(item.kind);
  if (!resource || !kind) return [];
  return findings.filter(
    (finding) =>
      finding.status !== 'resolved' &&
      Boolean(finding.mirrorsAlertId) &&
      normalizeResourceKey(finding.resourceId) === resource &&
      normalizeResourceKey(finding.mirrorsAlertType) === kind,
  );
}

export type PatrolLastingDecisionKind =
  'expected_behavior' | 'not_an_issue' | 'will_fix_later' | 'create_rule';

export interface PatrolLastingDecision {
  kind: PatrolLastingDecisionKind;
  label: string;
  /** One line: what it does and how long it lasts. */
  explanation: string;
  /** Whether the inline confirmation requires a written reason. */
  requiresReason: boolean;
}

/**
 * The durable Patrol outcomes, in the order a first-time user should read
 * them: the most common "this is normal here" first, the permanent rule last.
 * Durations mirror internal/ai/findings.go: expected_behavior and not_an_issue
 * hold until the finding is reopened (severity escalation still reactivates
 * expected_behavior); will_fix_later reminds after seven days; a rule is
 * permanent until deleted under Patrol suppression rules.
 */
export const PATROL_LASTING_DECISIONS: readonly PatrolLastingDecision[] = [
  {
    kind: 'expected_behavior',
    label: 'Remember as expected',
    explanation:
      'Patrol treats this as normal for this resource and stops raising it unless it gets worse. Lasts until you reopen the finding.',
    requiresReason: false,
  },
  {
    kind: 'not_an_issue',
    label: 'Dismiss: Not an issue',
    explanation:
      'Marks the detection wrong and silences similar findings on this resource. Lasts until you reopen the finding.',
    requiresReason: false,
  },
  {
    kind: 'will_fix_later',
    label: 'Dismiss: Later',
    explanation: 'Hides this for 7 days. If it is still happening after that, Patrol reminds you.',
    requiresReason: false,
  },
  {
    kind: 'create_rule',
    label: 'Create rule',
    explanation:
      'Permanent rule for this resource and category: matching findings are dismissed automatically until you delete the rule under Patrol suppression rules.',
    requiresReason: true,
  },
];

export interface PatrolRememberedDecisionPresentation {
  headline: string;
  detail: string;
}

/**
 * Describes a finding Patrol already remembers a decision for, so the
 * attention detail can say what was decided instead of offering it again.
 */
export function getPatrolRememberedDecisionPresentation(
  finding: UnifiedFinding,
  now: Date = new Date(),
): PatrolRememberedDecisionPresentation | null {
  if (finding.status !== 'dismissed' && !finding.dismissedReason) return null;
  switch (finding.dismissedReason) {
    case 'expected_behavior':
      return {
        headline: 'Remembered as expected',
        detail:
          'Patrol treats this as normal for this resource and will only raise it again if it gets worse.',
      };
    case 'not_an_issue':
      return {
        headline: 'Dismissed as not an issue',
        detail: 'Similar findings on this resource stay silent until you reopen this one.',
      };
    case 'will_fix_later': {
      const remindAt = finding.remindAt ? new Date(finding.remindAt) : undefined;
      const overdue =
        remindAt !== undefined && !Number.isNaN(remindAt.getTime()) && remindAt <= now;
      return {
        headline: overdue ? 'Reminder due' : 'Dismissed until later',
        detail: overdue
          ? 'You said you would fix this later and the reminder date has passed. It is still happening.'
          : remindAt && !Number.isNaN(remindAt.getTime())
            ? `Patrol stays quiet and reminds you on ${remindAt.toLocaleDateString()} if it is still happening.`
            : 'Patrol stays quiet and reminds you if it is still happening after the reminder date.',
      };
    }
    default:
      return {
        headline: 'Dismissed',
        detail: 'Patrol remembers this decision until you reopen the finding.',
      };
  }
}
