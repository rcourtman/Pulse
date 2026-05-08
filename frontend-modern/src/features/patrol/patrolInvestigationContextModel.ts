import type {
  CorrelationsResponse,
  IntelligencePolicyPostureSummary,
  ResourceCorrelation,
} from '@/types/aiIntelligence';
import type { ApprovalRequest, InvestigationRecord, RemediationPlan } from '@/api/ai';
import type { PatrolRunRecord } from '@/api/patrol';
import type {
  AIChatContext,
  AIChatContextBriefing,
  AIChatHandoffAction,
  AIChatHandoffResource,
} from '@/stores/aiChat';
import type { ResourceChange } from '@/types/resource';
import {
  formatResourceChangeKind,
  sortResourceChangesByObservedAt,
} from '@/utils/resourceChangePresentation';
import {
  formatResourceCorrelationEndpoint,
  formatResourceCorrelationPattern,
  formatResourceCorrelationSummary,
  sortResourceCorrelations,
} from '@/utils/resourceCorrelationPresentation';
import {
  formatDurationMs,
  formatPatrolRuntimeFailureSummary,
  formatScope,
  formatTriggerReason,
  getCanonicalScopeResourceIds,
  sanitizeAnalysis,
} from '@/utils/patrolFormat';
import {
  getPatrolRunCoverageSummary,
  getPatrolRunKindLabel,
  getPatrolRunStatusPresentation,
} from '@/utils/patrolRunPresentation';

export interface PatrolInvestigationContextSummaryInput {
  recentChangesCount?: number | null;
  correlations?: CorrelationsResponse | null;
  policyPosture?: IntelligencePolicyPostureSummary | null;
}

export interface PatrolInvestigationContextSummary {
  recentChangeCount: number;
  correlationCount: number;
  governedResourceCount: number;
  hasContext: boolean;
  summaryText: string;
}

export interface PatrolInvestigationRecordPresentation {
  hasRecord: boolean;
  statusLabel: string;
  outcomeLabel?: string;
  confidenceLabel?: string;
  conclusion?: string;
  recommendedAction?: string;
  evidenceSummaries: string[];
  verificationSummaries: string[];
  toolsUsed: string[];
  proposedFix?: {
    description: string;
    riskLabel?: string;
    targetHost?: string;
    rationale?: string;
    commandSummary?: string;
    destructive: boolean;
  };
  error?: string;
}

export interface PatrolAssistantFindingPromptInput {
  title: string;
  subject: string;
  description: string;
  investigationOutcome?: string | null;
  remediationId?: string | null;
  pendingApproval?: PatrolAssistantApprovalBriefingInput | null;
  proposedFix?: PatrolAssistantProposedFixBriefingInput | null;
  investigationRecord?: InvestigationRecord | null;
  nextStepAction?: PatrolAssistantNextStepInput | null;
}

export interface PatrolAssistantApprovalBriefingInput {
  id?: string | null;
  status?: string | null;
  riskLevel?: string | null;
  requestedAt?: string | null;
  expiresAt?: string | null;
  targetName?: string | null;
  actionId?: string | null;
  actionApprovalPolicy?: string | null;
  actionPlanExpiresAt?: string | null;
  actionPlanMessage?: string | null;
  actionPreflight?: string | null;
  actionDryRunSummary?: string | null;
  actionRequestedBy?: string | null;
}

export interface PatrolAssistantProposedFixBriefingInput {
  description?: string | null;
  riskLevel?: string | null;
  targetHost?: string | null;
  rationale?: string | null;
  commandCount?: number | null;
  destructive?: boolean | null;
}

export interface PatrolAssistantNextStepInput {
  label?: string | null;
  href?: string | null;
}

export interface PatrolAssistantProposedFixBriefingSource {
  description?: string | null;
  riskLevel?: string | null;
  risk_level?: string | null;
  targetHost?: string | null;
  target_host?: string | null;
  rationale?: string | null;
  commandCount?: number | null;
  commands?: readonly string[] | null;
  destructive?: boolean | null;
}

export interface PatrolAssistantFindingBriefingInput {
  title: string;
  subject: string;
  severity?: string | null;
  findingStatus?: string | null;
  investigationOutcome?: string | null;
  loopState?: string | null;
  timesRaised?: number | null;
  regressionCount?: number | null;
  lastRegressionAt?: string | null;
  remediationId?: string | null;
  pendingApproval?: PatrolAssistantApprovalBriefingInput | null;
  proposedFix?: PatrolAssistantProposedFixBriefingInput | null;
  investigationRecord?: InvestigationRecord | null;
  nextStepAction?: PatrolAssistantNextStepInput | null;
}

export interface PatrolAssistantFindingHandoffInput {
  id?: string | null;
  title: string;
  subject: string;
  description?: string | null;
  severity?: string | null;
  findingStatus?: string | null;
  investigationStatus?: string | null;
  investigationOutcome?: string | null;
  loopState?: string | null;
  timesRaised?: number | null;
  regressionCount?: number | null;
  lastRegressionAt?: string | null;
  remediationId?: string | null;
  resourceId?: string | null;
  resourceName?: string | null;
  resourceType?: string | null;
  detectedAt?: string | null;
  lastSeenAt?: string | null;
  pendingApproval?: PatrolAssistantApprovalBriefingInput | null;
  proposedFix?: PatrolAssistantProposedFixBriefingInput | null;
  investigationRecord?: InvestigationRecord | null;
  nextStepAction?: PatrolAssistantNextStepInput | null;
}

export interface PatrolAssistantFindingModeInput {
  investigationOutcome?: string | null;
  remediationId?: string | null;
  pendingApproval?: PatrolAssistantApprovalBriefingInput | null;
  investigationRecord?: InvestigationRecord | null;
}

export interface PatrolRemediationPlanAssistantInput {
  title: string;
  subject: string;
  plan: RemediationPlan;
}

export interface PatrolAssessmentAssistantFindingInput {
  id?: string | null;
  title?: string | null;
  description?: string | null;
  severity?: string | null;
  status?: string | null;
  resourceId?: string | null;
  resourceName?: string | null;
  resourceType?: string | null;
  detectedAt?: string | null;
  lastSeenAt?: string | null;
  investigationStatus?: string | null;
  investigationOutcome?: string | null;
  loopState?: string | null;
  timesRaised?: number | null;
  regressionCount?: number | null;
  lastRegressionAt?: string | null;
  pendingApproval?: PatrolAssistantApprovalBriefingInput | null;
  proposedFix?: PatrolAssistantProposedFixBriefingInput | null;
  investigationRecord?: InvestigationRecord | null;
}

export type PatrolAssessmentRecommendedNextStepActionKind =
  | 'discuss_assessment'
  | 'open_provider_settings'
  | 'review_approvals'
  | 'review_findings'
  | 'run_patrol';

export interface PatrolAssessmentRecommendedNextStepInput {
  title?: string | null;
  description?: string | null;
  actionLabel?: string | null;
  actionKind?: PatrolAssessmentRecommendedNextStepActionKind | string | null;
  actionDisabledReason?: string | null;
}

export interface PatrolAssessmentAssistantHandoffInput {
  assessment?: {
    title?: string | null;
    description?: string | null;
    eyebrow?: string | null;
  } | null;
  overallHealth?: {
    factors?: Array<{ category?: string | null }> | null;
    grade?: string | null;
    prediction?: string | null;
    score?: number | null;
  } | null;
  scoreChipLabel?: string | null;
  metricState?: {
    primaryLabel?: string | null;
    primaryValue?: number | null;
    secondaryLabel?: string | null;
    secondaryValue?: number | null;
    fixedLabel?: string | null;
    fixedValue?: number | null;
  } | null;
  verification?: {
    title?: string | null;
    description?: string | null;
    lastFullRunAt?: string | null;
    activityMixLabel?: string | null;
  } | null;
  recency?: {
    label?: string | null;
    timestamp?: string | null;
  } | null;
  latestRun?: {
    kindLabel?: string | null;
    status?: {
      label?: string | null;
    } | null;
    timestamp?: string | null;
    coverageSummary?: string | null;
    findingsSnapshotAvailable?: boolean | null;
  } | null;
  investigationContext?: PatrolInvestigationContextSummary | null;
  supportingEvidence?: {
    recentChanges?: ResourceChange[] | null;
    correlations?: ResourceCorrelation[] | null;
  } | null;
  recommendedNextStep?: PatrolAssessmentRecommendedNextStepInput | null;
  activeFindings?: PatrolAssessmentAssistantFindingInput[] | null;
}

export interface PatrolAssessmentAssistantHandoff {
  prompt: string;
  context: Omit<AIChatContext, 'initialPrompt'>;
}

export interface PatrolAssistantFindingHandoff {
  prompt: string;
  context: Omit<AIChatContext, 'initialPrompt'>;
}

export interface PatrolRunAssistantHandoff {
  prompt: string;
  context: Omit<AIChatContext, 'initialPrompt'>;
}

export interface PatrolConfigurationFailureInput {
  message: string;
  code?: string;
  status?: number;
  saved?: boolean;
  details?: Record<string, string>;
  autonomyLevel?: string;
  fullModeUnlocked?: boolean;
  investigationBudget?: number;
  investigationTimeoutSec?: number;
  readiness?: {
    status?: string;
    cause?: string;
    summary?: string;
    provider?: string;
    model?: string;
  } | null;
  runtimeState?: string;
  blockedReason?: string;
  blockedCause?: string;
}

export interface PatrolConfigurationFailureHandoff {
  prompt: string;
  context: Omit<AIChatContext, 'initialPrompt'>;
}

const MAX_ASSESSMENT_FINDINGS = 5;
const MAX_ASSESSMENT_RECENT_CHANGES = 3;
const MAX_ASSESSMENT_CORRELATIONS = 3;
const MAX_ASSESSMENT_RESOURCES = 8;
const MAX_ASSESSMENT_HANDOFF_ACTIONS = 4;
const MAX_PATROL_RUN_HANDOFF_RESOURCES = 8;
const MAX_PATROL_BRIEFING_SUGGESTED_PROMPTS = 3;
const SAME_STATE_CHANGED_FIELD_LABELS: Record<string, string> = {
  'docker.command': 'Docker command',
  'docker.updateStatus': 'Docker image status',
  'proxmox.lifecycle': 'Proxmox lifecycle',
  status: 'status',
  incidents: 'incident state',
  relationships: 'relationships',
  capabilities: 'capabilities',
  tags: 'tags',
  parentId: 'parent relationship',
  customUrl: 'custom URL',
  identity: 'identity',
};

interface NormalizedPatrolAssessmentRecommendedNextStep {
  title: string;
  description?: string;
  actionLabel?: string;
  actionKind?: PatrolAssessmentRecommendedNextStepActionKind;
  actionDisabledReason?: string;
  actionSummary?: string;
}

const PATROL_ASSESSMENT_RECOMMENDED_NEXT_STEP_ACTION_LABELS: Record<
  PatrolAssessmentRecommendedNextStepActionKind,
  string
> = {
  discuss_assessment: 'Discuss with Assistant',
  open_provider_settings: 'Open provider settings',
  review_approvals: 'Review approvals',
  review_findings: 'Review findings',
  run_patrol: 'Run Patrol',
};
const WITHHELD_RECOMMENDATION_TEXT = 'sensitive or command detail withheld';

export function buildPatrolInvestigationContextSummary(
  input: PatrolInvestigationContextSummaryInput,
): PatrolInvestigationContextSummary {
  const recentChangeCount = normalizeNonNegativeCount(input.recentChangesCount);
  const correlationCount = normalizeCorrelationCount(input.correlations);
  const governedResourceCount = normalizeNonNegativeCount(input.policyPosture?.total_resources);

  const parts: string[] = [];
  if (recentChangeCount > 0) {
    parts.push(`${recentChangeCount} recent change${recentChangeCount === 1 ? '' : 's'}`);
  }
  if (correlationCount > 0) {
    parts.push(`${correlationCount} correlation${correlationCount === 1 ? '' : 's'}`);
  }
  if (governedResourceCount > 0) {
    parts.push(
      `${governedResourceCount} policy-covered resource${governedResourceCount === 1 ? '' : 's'}`,
    );
  }

  return {
    recentChangeCount,
    correlationCount,
    governedResourceCount,
    hasContext: parts.length > 0,
    summaryText: parts.join(' · '),
  };
}

export function selectPatrolSupportingRecentChanges(
  changes?: ResourceChange[] | null,
): ResourceChange[] {
  return normalizeAssessmentRecentChanges(changes);
}

export function buildPatrolInvestigationRecordPresentation(
  record?: InvestigationRecord | null,
): PatrolInvestigationRecordPresentation {
  if (!record) {
    return {
      hasRecord: false,
      statusLabel: '',
      evidenceSummaries: [],
      verificationSummaries: [],
      toolsUsed: [],
    };
  }

  const proposedFix = normalizeProposedFixBriefing(
    buildPatrolAssistantProposedFixBriefingInput(record.proposed_fix),
  );

  return {
    hasRecord: true,
    statusLabel: formatIdentifierLabel(record.status) || 'Investigation recorded',
    outcomeLabel: formatIdentifierLabel(record.outcome),
    confidenceLabel: record.confidence
      ? `${formatIdentifierLabel(record.confidence)} confidence`
      : undefined,
    conclusion: normalizeText(record.conclusion),
    recommendedAction: normalizeText(record.recommended_action),
    evidenceSummaries: (record.evidence || [])
      .map((item) => normalizeText(item.summary || item.kind || item.id))
      .filter(Boolean)
      .slice(0, 3),
    verificationSummaries: (record.verification || [])
      .map(normalizeText)
      .filter(Boolean)
      .slice(0, 3),
    toolsUsed: (record.tools_used || []).map(formatToolLabel).filter(Boolean).slice(0, 4),
    proposedFix:
      proposedFix && proposedFix.description
        ? proposedFix
        : proposedFix && (proposedFix.commandSummary || proposedFix.rationale)
          ? proposedFix
          : undefined,
    error: normalizeText(record.error),
  };
}

export function buildPatrolAssistantProposedFixBriefingInput(
  source?: PatrolAssistantProposedFixBriefingSource | null,
): PatrolAssistantProposedFixBriefingInput | undefined {
  if (!source) return undefined;
  const commandCount =
    typeof source.commandCount === 'number'
      ? source.commandCount
      : Array.isArray(source.commands)
        ? source.commands.length
        : null;
  const briefing = {
    description: normalizeText(source.description),
    riskLevel: normalizeText(source.riskLevel || source.risk_level),
    targetHost: normalizeText(source.targetHost || source.target_host),
    rationale: normalizeText(source.rationale),
    commandCount: normalizeNonNegativeCount(commandCount),
    destructive: typeof source.destructive === 'boolean' ? source.destructive : null,
  };

  if (
    !briefing.description &&
    !briefing.riskLevel &&
    !briefing.targetHost &&
    !briefing.rationale &&
    !briefing.commandCount &&
    briefing.destructive !== true
  ) {
    return undefined;
  }

  return briefing;
}

export function buildPatrolAssistantApprovalBriefingInput(
  approval?: ApprovalRequest | null,
): PatrolAssistantApprovalBriefingInput | undefined {
  if (!approval) return undefined;
  return {
    id: normalizeText(approval.id),
    status: normalizeText(approval.status),
    riskLevel: normalizeText(approval.riskLevel),
    requestedAt: normalizeText(approval.requestedAt),
    expiresAt: normalizeText(approval.expiresAt),
    targetName: normalizeText(approval.targetName),
    actionId: normalizeText(approval.plan?.actionId),
    actionApprovalPolicy: normalizeText(approval.plan?.approvalPolicy),
    actionPlanExpiresAt: normalizeText(approval.plan?.expiresAt),
    actionPlanMessage: normalizeText(approval.plan?.message || approval.plan?.summary),
    actionPreflight: normalizeText(approval.preflight?.intendedChange),
    actionDryRunSummary: normalizeText(approval.preflight?.dryRunSummary),
    actionRequestedBy: normalizeText(approval.requestedBy),
  };
}

export function buildPatrolAssistantFindingPrompt(
  input: PatrolAssistantFindingPromptInput,
): string {
  const title = normalizeText(input.title) || 'Patrol finding';
  const subject = normalizeText(input.subject) || 'the affected resource';
  const description = normalizeText(input.description);
  const hasRecord = Boolean(input.investigationRecord?.id);
  const actionInstruction = buildPatrolAssistantFindingActionPromptInstruction(input);
  const nextStepAction = normalizePatrolAssistantNextStepAction(input.nextStepAction);

  let prompt = `I'd like to discuss this Patrol finding: "${title}" on ${subject}.`;
  if (hasRecord) {
    prompt +=
      '\n\nPulse Patrol has a structured investigation record for this finding. Use that record as the main context before suggesting next actions.';
  }
  if (actionInstruction) {
    prompt += `\n\n${actionInstruction}`;
  }
  if (nextStepAction.label) {
    prompt += `\n\nPatrol's visible next step is "${nextStepAction.label}". Treat it as operator guidance for review, not an execution command.`;
  }
  if (description) {
    prompt += `\n\n${description}`;
  }
  return prompt;
}

function buildPatrolAssistantFindingActionPromptInstruction(
  input: PatrolAssistantFindingPromptInput,
): string | undefined {
  const pendingApproval = normalizeApprovalBriefing(input.pendingApproval);
  const record = buildPatrolInvestigationRecordPresentation(input.investigationRecord);
  const proposedFix = record.proposedFix || normalizeProposedFixBriefing(input.proposedFix);
  const remediationId = normalizeText(input.remediationId);
  const outcome = normalizeText(input.investigationOutcome || input.investigationRecord?.outcome);
  const normalizedOutcome = outcome.toLowerCase();

  if (pendingApproval.id) {
    const contextParts = [
      pendingApproval.status ? `approval status ${pendingApproval.status}` : undefined,
      pendingApproval.riskLevel ? `${pendingApproval.riskLevel} risk` : undefined,
      pendingApproval.targetName
        ? `target ${truncateContextText(pendingApproval.targetName, 120)}`
        : undefined,
      pendingApproval.actionApprovalPolicy ? 'approval policy attached' : undefined,
      pendingApproval.actionPreflight || pendingApproval.actionDryRunSummary
        ? 'dry-run posture attached'
        : undefined,
    ].filter(isNonEmptyString);

    return `Start by reviewing governed approval ${pendingApproval.id}${
      contextParts.length > 0 ? ` (${contextParts.join('; ')})` : ''
    }. Use the attached Patrol context to explain prerequisites, risk, and the safest next step before any execution.`;
  }

  const hasGovernedActionPosture = Boolean(
    proposedFix ||
    remediationId ||
    normalizeText(input.investigationRecord?.approval_id) ||
    GOVERNED_ACTION_OUTCOMES.has(normalizedOutcome),
  );
  if (!hasGovernedActionPosture) {
    return undefined;
  }

  const contextParts = [
    proposedFix?.description
      ? `proposed fix ${truncateContextText(proposedFix.description, 120)}`
      : undefined,
    proposedFix?.riskLabel ? `${proposedFix.riskLabel.toLowerCase()} risk` : undefined,
    proposedFix?.targetHost
      ? `target ${truncateContextText(proposedFix.targetHost, 120)}`
      : undefined,
    proposedFix?.commandSummary,
    proposedFix?.destructive ? 'destructive action' : undefined,
    remediationId ? `remediation ${truncateContextText(remediationId, 120)}` : undefined,
    outcome ? `outcome ${formatIdentifierLabel(outcome)?.toLowerCase() || outcome}` : undefined,
  ].filter(isNonEmptyString);

  return `Start by reviewing the governed proposed fix or action posture${
    contextParts.length > 0 ? ` (${contextParts.join('; ')})` : ''
  }. Use the attached Patrol context to explain risk, prerequisites, and the safest next step without repeating command text.`;
}

export function buildPatrolAssessmentAssistantHandoff(
  input: PatrolAssessmentAssistantHandoffInput,
): PatrolAssessmentAssistantHandoff {
  const title = normalizeText(input.assessment?.title) || 'Pulse Patrol assessment';
  const description = normalizeText(input.assessment?.description);
  const recommendedNextStep = normalizeAssessmentRecommendedNextStep(input.recommendedNextStep);
  const recommendedNextStepActionHref =
    getAssessmentRecommendedNextStepActionHref(recommendedNextStep);
  const handoffContext = buildPatrolAssessmentAssistantModelContext(input);
  const recentChanges = normalizeAssessmentRecentChanges(input.supportingEvidence?.recentChanges);
  const correlations = normalizeAssessmentCorrelations(input.supportingEvidence?.correlations);
  const handoffActions = buildPatrolAssessmentHandoffActions(input);

  return {
    prompt: buildPatrolAssessmentAssistantPrompt(input, title, description, handoffActions),
    context: {
      targetType: 'patrol-assessment',
      targetId: 'pulse-patrol-assessment',
      autonomousMode: false,
      handoffContext,
      handoffResources: buildPatrolAssessmentHandoffResources(input),
      handoffActions: handoffActions.length > 0 ? handoffActions : undefined,
      handoffMetadata: {
        kind: 'patrol_assessment',
        recommendedNextStep: recommendedNextStep?.title,
        recommendedNextStepDetail: recommendedNextStep?.description,
        recommendedNextStepAction: recommendedNextStep?.actionLabel,
        recommendedNextStepActionKind: recommendedNextStep?.actionKind,
        recommendedNextStepActionHref,
      },
      briefing: buildPatrolAssessmentAssistantBriefing(input),
      context: {
        source: 'pulse-patrol-assessment',
        activeFindingCount: normalizeNonNegativeCount(input.activeFindings?.length),
        recentChangeCount: input.investigationContext?.recentChangeCount ?? 0,
        correlationCount: input.investigationContext?.correlationCount ?? 0,
        recentChangeDetailCount: recentChanges.length,
        correlationDetailCount: correlations.length,
        governedResourceCount: input.investigationContext?.governedResourceCount ?? 0,
        pendingApprovalCount: normalizeAssessmentPendingApprovalCount(input.activeFindings),
        ...(recommendedNextStep
          ? {
              recommendedNextStepTitle: recommendedNextStep.title,
              recommendedNextStepActionKind: recommendedNextStep.actionKind,
              recommendedNextStepActionDisabledReason: recommendedNextStep.actionDisabledReason,
            }
          : {}),
      },
    },
  };
}

export function buildPatrolAssistantFindingHandoff(
  input: PatrolAssistantFindingHandoffInput,
): PatrolAssistantFindingHandoff {
  const findingId = normalizeText(input.id) || normalizeText(input.investigationRecord?.finding_id);
  const resource = buildPatrolFindingHandoffResource(input);
  const handoffResources = resource ? [resource] : [];
  const handoffActions = buildPatrolAssistantFindingHandoffActions(input);
  const nextStepAction = normalizePatrolAssistantNextStepAction(input.nextStepAction);

  return {
    prompt: buildPatrolAssistantFindingPrompt({
      title: input.title,
      subject: input.subject,
      description: normalizeText(input.description),
      investigationOutcome: input.investigationOutcome,
      remediationId: input.remediationId,
      pendingApproval: input.pendingApproval,
      proposedFix: input.proposedFix,
      investigationRecord: input.investigationRecord,
      nextStepAction: input.nextStepAction,
    }),
    context: {
      targetType: resource?.type,
      targetId: resource?.id,
      findingId: findingId || undefined,
      autonomousMode: false,
      handoffContext: buildPatrolAssistantFindingModelContext(input),
      handoffResources: handoffResources.length > 0 ? handoffResources : undefined,
      handoffActions: handoffActions.length > 0 ? handoffActions : undefined,
      handoffMetadata: nextStepAction.label
        ? {
            kind: 'patrol_finding',
            recommendedNextStep: nextStepAction.label,
            recommendedNextStepAction: nextStepAction.label,
            recommendedNextStepActionHref: nextStepAction.href || undefined,
          }
        : undefined,
      briefing: buildPatrolAssistantFindingBriefing({
        title: input.title,
        subject: input.subject,
        severity: input.severity,
        findingStatus: input.findingStatus,
        investigationOutcome: input.investigationOutcome,
        loopState: input.loopState,
        timesRaised: input.timesRaised,
        regressionCount: input.regressionCount,
        lastRegressionAt: input.lastRegressionAt,
        remediationId: input.remediationId,
        pendingApproval: input.pendingApproval,
        proposedFix: input.proposedFix,
        investigationRecord: input.investigationRecord,
        nextStepAction: input.nextStepAction,
      }),
      context: {
        source: 'pulse-patrol-finding',
        findingId: findingId || undefined,
        investigationRecordId: normalizeText(input.investigationRecord?.id) || undefined,
        resourceId: resource?.id,
        resourceName: resource?.name,
        resourceType: resource?.type,
        pendingApprovalId: normalizeApprovalBriefing(input.pendingApproval).id || undefined,
        nextStepActionLabel: nextStepAction.label || undefined,
        nextStepActionHref: nextStepAction.href || undefined,
        actionReferenceCount: handoffActions.length,
      },
    },
  };
}

export function buildPatrolRunAssistantHandoff(run: PatrolRunRecord): PatrolRunAssistantHandoff {
  const runId = normalizeText(run.id);
  const kindLabel = getPatrolRunKindLabel(run.type);
  const findingsSnapshotAvailable = run.finding_ids !== undefined;
  const statusLabel = getPatrolRunStatusPresentation(
    run.status || 'unknown',
    run.error_count || 0,
    findingsSnapshotAvailable,
  ).label;
  const runtimeFailure = formatPatrolRunRuntimeFailure(run);
  const handoffResources = buildPatrolRunHandoffResources(run);

  return {
    prompt: buildPatrolRunAssistantPrompt(run, kindLabel, statusLabel, runtimeFailure),
    context: {
      targetType: 'patrol-run',
      targetId: runId || undefined,
      autonomousMode: false,
      handoffMetadata: {
        kind: 'patrol_run',
        runId: runId || undefined,
        runType: kindLabel,
        runStatus: statusLabel,
        runtimeFailure: Boolean(runtimeFailure),
      },
      briefing: buildPatrolRunAssistantBriefing(run, kindLabel, statusLabel, runtimeFailure),
      context: {
        source: 'pulse-patrol-run',
        runId: runId || undefined,
        runType: normalizeText(run.type) || undefined,
        triggerReason: normalizeText(run.trigger_reason) || undefined,
        status: normalizeText(run.status) || undefined,
        effectiveStatus: statusLabel,
        errorCount: normalizeNonNegativeCount(run.error_count),
        resourcesChecked: normalizeNonNegativeCount(run.resources_checked),
        findingSnapshotCount: Array.isArray(run.finding_ids) ? run.finding_ids.length : undefined,
        handoffResourceCount: handoffResources.length,
      },
    },
  };
}

export function buildPatrolConfigurationFailureHandoff(
  input: PatrolConfigurationFailureInput,
): PatrolConfigurationFailureHandoff {
  const message = normalizeText(input.message) || 'Patrol configuration could not be saved.';
  const code = normalizeText(input.code);
  const issueLabel = input.saved ? 'Patrol configuration issue' : 'Patrol configuration failure';
  const readinessSummary = normalizeText(input.readiness?.summary);
  const cause = normalizeText(input.readiness?.cause) || normalizeText(input.blockedCause);
  const provider = normalizeText(input.readiness?.provider);
  const model = normalizeText(input.readiness?.model);
  const detailLines = [
    readinessSummary ? `Readiness: ${readinessSummary}` : undefined,
    cause ? `Cause: ${formatIdentifierLabel(cause) || cause}` : undefined,
    provider ? `Provider: ${provider}` : undefined,
    model ? `Model: ${model}` : undefined,
    formatConfigurationFailureSettings(input),
  ].filter(isNonEmptyString);

  return {
    prompt: buildPatrolConfigurationFailurePrompt(input, message, detailLines),
    context: {
      targetType: 'patrol-configuration',
      targetId: 'pulse-patrol-configuration',
      autonomousMode: false,
      handoffContext: buildPatrolConfigurationFailureModelContext(input, message, detailLines),
      handoffMetadata: {
        kind: 'patrol_configuration_failure',
        runtimeFailure: true,
      },
      briefing: {
        sourceLabel: 'Pulse Patrol',
        title: `${issueLabel.charAt(0).toUpperCase()}${issueLabel.slice(1)} attached`,
        subject: code ? `${code}: ${message}` : message,
        statusLabel:
          [input.status ? `HTTP ${input.status}` : undefined, cause]
            .filter(isNonEmptyString)
            .join(' · ') || undefined,
        detailLines: detailLines.slice(0, 4),
        evidence: formatConfigurationFailureDetails(input.details).slice(0, 4),
        actionLabel: `Review ${issueLabel}`,
        safetyNote:
          'Assistant can explain the configuration state; provider changes, retries, and remediation remain operator-controlled.',
        suggestedPrompts: formatPatrolSuggestedPrompts([
          'Explain why Patrol configuration failed',
          'List provider or model checks',
          'What should I change before retrying?',
        ]),
      },
      context: {
        source: 'pulse-patrol-configuration-failure',
        code: code || undefined,
        status: input.status,
        readinessStatus: normalizeText(input.readiness?.status) || undefined,
        readinessCause: cause || undefined,
        provider: provider || undefined,
        model: model || undefined,
        runtimeState: normalizeText(input.runtimeState) || undefined,
      },
    },
  };
}

export function buildPatrolAssistantFindingHandoffActions(
  finding: PatrolAssessmentAssistantFindingInput,
): AIChatHandoffAction[] {
  const action = buildPatrolFindingHandoffAction(finding);
  return action ? [action] : [];
}

function buildPatrolAssessmentAssistantPrompt(
  input: PatrolAssessmentAssistantHandoffInput,
  title: string,
  description: string,
  handoffActions: AIChatHandoffAction[],
): string {
  const pendingApprovalCount = normalizeAssessmentPendingApprovalCount(input.activeFindings);
  const actionCount = handoffActions.length;
  const hasCoverageGap = assessmentHasCoverageGap(input);
  const recommendationInstruction = buildPatrolAssessmentRecommendationPromptInstruction(input);
  const reviewInstruction =
    pendingApprovalCount > 0
      ? `Start by reviewing ${formatAssessmentMetricCount(
          'pending governed approvals',
          pendingApprovalCount,
        )}, approval policy, dry-run posture, and the safest next step from the attached context.`
      : actionCount > 0
        ? `Start by reviewing ${formatAssessmentMetricCount(
            'governed action references',
            actionCount,
          )}, risk, and the safest next step from the attached context.`
        : hasCoverageGap
          ? 'Start by explaining why Patrol coverage is incomplete, what the latest scoped activity did and did not prove, and whether a full Patrol verification should run before action.'
          : 'Use the attached model-only Patrol assessment context before suggesting next actions. Help me understand priority, risk, and safe next steps.';

  return [
    `Discuss the current Pulse Patrol assessment: ${title}.`,
    description,
    reviewInstruction,
    recommendationInstruction,
    'Do not infer, repeat, or execute raw command text from this handoff.',
  ]
    .filter(isNonEmptyString)
    .join('\n\n');
}

function buildPatrolAssessmentAssistantBriefing(
  input: PatrolAssessmentAssistantHandoffInput,
): AIChatContextBriefing {
  const title = normalizeText(input.assessment?.title) || 'Pulse Patrol assessment';
  const description = normalizeText(input.assessment?.description);
  const health = formatAssessmentHealth(input);
  const attentionSummary = formatAssessmentAttentionSummary(input);
  const verification = formatAssessmentVerification(input);
  const latestRun = formatAssessmentLatestRun(input);
  const contextSummary = normalizeText(input.investigationContext?.summaryText);
  const recommendedNextStep = normalizeAssessmentRecommendedNextStep(input.recommendedNextStep);
  const recommendedNextStepLines =
    formatAssessmentRecommendedNextStepDetailLines(recommendedNextStep);
  const findings = normalizeAssessmentFindings(input.activeFindings);
  const recentChanges = normalizeAssessmentRecentChanges(input.supportingEvidence?.recentChanges);
  const correlations = normalizeAssessmentCorrelations(input.supportingEvidence?.correlations);
  const findingEvidence = findings.map(formatAssessmentFindingEvidence).filter(isNonEmptyString);
  const handoffActions = buildPatrolAssessmentHandoffActions(input);
  const actionPosture = buildPatrolAssessmentActionPosture(input, handoffActions);
  const supportingEvidence = [
    ...recentChanges.map(formatAssessmentRecentChangeEvidence),
    ...correlations.map(formatAssessmentCorrelationEvidence),
  ]
    .filter(isNonEmptyString)
    .slice(0, 2);

  return {
    sourceLabel: 'Pulse Patrol',
    title: 'Patrol assessment attached',
    subject: title,
    statusLabel: [health, attentionSummary].filter(isNonEmptyString).join(' · ') || undefined,
    detailLines: [description, ...recommendedNextStepLines, verification, latestRun, contextSummary]
      .filter(isNonEmptyString)
      .slice(0, 7),
    evidence: [...findingEvidence.slice(0, 3), ...supportingEvidence].slice(0, 5),
    actionLabel: actionPosture.actionLabel,
    actionHref: actionPosture.actionHref,
    safetyNote: actionPosture.safetyNote,
    suggestedPrompts: buildPatrolAssessmentSuggestedPrompts(input, {
      findings,
      recentChanges,
      correlations,
    }),
  };
}

function buildPatrolAssessmentRecommendationPromptInstruction(
  input: PatrolAssessmentAssistantHandoffInput,
): string | undefined {
  const recommendedNextStep = normalizeAssessmentRecommendedNextStep(input.recommendedNextStep);
  if (!recommendedNextStep) return undefined;

  const parts = [
    `Patrol's visible recommended next step is "${recommendedNextStep.title}"`,
    recommendedNextStep.description
      ? `detail: ${truncateContextText(recommendedNextStep.description, 180)}`
      : undefined,
    formatAssessmentRecommendedNextStepActionInstruction(recommendedNextStep),
  ].filter(isNonEmptyString);

  return `${parts.join('; ')}. Explain that recommendation before alternatives, but keep Patrol runs, settings changes, diagnostics, remediation, and approvals in governed controls.`;
}

function formatAssessmentRecommendedNextStepActionInstruction(
  recommendedNextStep: NormalizedPatrolAssessmentRecommendedNextStep,
): string | undefined {
  if (!recommendedNextStep.actionLabel) return undefined;
  if (recommendedNextStep.actionDisabledReason) {
    return `Patrol-owned action "${recommendedNextStep.actionLabel}" is currently unavailable: ${recommendedNextStep.actionDisabledReason}`;
  }
  return `available Patrol-owned action: ${recommendedNextStep.actionLabel}`;
}

function formatAssessmentRecommendedNextStepActionBriefingDetail(
  recommendedNextStep: NormalizedPatrolAssessmentRecommendedNextStep,
): string | undefined {
  if (!recommendedNextStep.actionLabel) return undefined;
  return recommendedNextStep.actionDisabledReason
    ? `Action unavailable: ${recommendedNextStep.actionLabel} - ${recommendedNextStep.actionDisabledReason}`
    : `Available action: ${recommendedNextStep.actionLabel}`;
}

function formatAssessmentRecommendedNextStepActionAvailability(
  recommendedNextStep?: NormalizedPatrolAssessmentRecommendedNextStep,
): string | undefined {
  if (!recommendedNextStep?.actionDisabledReason) return undefined;
  return `unavailable - ${recommendedNextStep.actionDisabledReason}`;
}

function formatAssessmentRecommendationSafetyNote(
  base: string,
  recommendedNextStep?: NormalizedPatrolAssessmentRecommendedNextStep,
): string {
  if (!recommendedNextStep?.actionLabel || !recommendedNextStep.actionDisabledReason) {
    return base;
  }
  return `${base} ${recommendedNextStep.actionLabel} is currently unavailable: ${recommendedNextStep.actionDisabledReason}.`;
}

function formatAssessmentRecommendedNextStepDetailLines(
  recommendedNextStep?: NormalizedPatrolAssessmentRecommendedNextStep,
): string[] {
  if (!recommendedNextStep) return [];

  return [
    `Recommended next step: ${recommendedNextStep.title}`,
    recommendedNextStep.description ? `Reason: ${recommendedNextStep.description}` : undefined,
    formatAssessmentRecommendedNextStepActionBriefingDetail(recommendedNextStep),
  ].filter(isNonEmptyString);
}

function getAssessmentRecommendedNextStepActionHref(
  recommendedNextStep?: NormalizedPatrolAssessmentRecommendedNextStep,
): string | undefined {
  switch (recommendedNextStep?.actionKind) {
    case 'open_provider_settings':
      return '/settings/system-ai';
    case 'review_approvals':
    case 'review_findings':
      return '/patrol';
    default:
      return undefined;
  }
}

function buildPatrolAssessmentActionPosture(
  input: PatrolAssessmentAssistantHandoffInput,
  handoffActions: AIChatHandoffAction[],
): { actionLabel: string; actionHref?: string; safetyNote: string } {
  const pendingApprovalCount = normalizeAssessmentPendingApprovalCount(input.activeFindings);
  const actionCount = handoffActions.length;
  const hasCoverageGap = assessmentHasCoverageGap(input);
  const recommendedNextStep = normalizeAssessmentRecommendedNextStep(input.recommendedNextStep);
  const recommendedNextStepActionHref =
    getAssessmentRecommendedNextStepActionHref(recommendedNextStep);
  const hasDryRunPosture = handoffActions.some((action) =>
    Boolean(normalizeText(action.actionDryRunSummary) || normalizeText(action.actionPreflight)),
  );
  const hasDestructiveAction = handoffActions.some((action) => Boolean(action.destructive));
  const hasApprovalPolicy = handoffActions.some((action) =>
    Boolean(normalizeText(action.actionApprovalPolicy)),
  );

  if (pendingApprovalCount > 0) {
    return {
      actionLabel: `${formatAssessmentMetricCount(
        'Pending governed approvals',
        pendingApprovalCount,
      )} attached`,
      safetyNote: formatAssessmentActionSafetyNote({
        primary: 'Review approvals in the governed flow',
        hasDryRunPosture,
        hasDestructiveAction,
        hasApprovalPolicy,
      }),
    };
  }

  if (actionCount > 0) {
    return {
      actionLabel: `${formatAssessmentMetricCount(
        'Governed action references',
        actionCount,
      )} attached`,
      safetyNote: formatAssessmentActionSafetyNote({
        primary: 'Review action posture in the governed flow',
        hasDryRunPosture,
        hasDestructiveAction,
        hasApprovalPolicy,
      }),
    };
  }

  if (hasCoverageGap) {
    return {
      actionLabel: recommendedNextStep?.actionLabel
        ? `Recommended: ${recommendedNextStep.actionLabel}`
        : 'Review coverage gap',
      actionHref: recommendedNextStep?.actionLabel ? recommendedNextStepActionHref : undefined,
      safetyNote: formatAssessmentRecommendationSafetyNote(
        'Assistant can explain the gap; full Patrol runs, diagnostics, and remediation remain operator-controlled.',
        recommendedNextStep,
      ),
    };
  }

  if (recommendedNextStep?.actionLabel || recommendedNextStep?.title) {
    return {
      actionLabel: `Recommended: ${recommendedNextStep.actionLabel || recommendedNextStep.title}`,
      actionHref: recommendedNextStep.actionLabel ? recommendedNextStepActionHref : undefined,
      safetyNote: formatAssessmentRecommendationSafetyNote(
        'Assistant can explain the Patrol recommendation; Patrol runs, settings changes, diagnostics, and remediation remain operator-controlled.',
        recommendedNextStep,
      ),
    };
  }

  return {
    actionLabel: 'Discuss Patrol assessment',
    safetyNote: 'Diagnostics and remediation require governed approval.',
  };
}

function formatAssessmentActionSafetyNote(input: {
  primary: string;
  hasDryRunPosture: boolean;
  hasDestructiveAction: boolean;
  hasApprovalPolicy: boolean;
}): string {
  const parts = [input.primary];
  if (input.hasApprovalPolicy) {
    parts.push('approval policy is attached');
  }
  if (input.hasDryRunPosture) {
    parts.push('dry-run posture is attached');
  }
  if (input.hasDestructiveAction) {
    parts.push('destructive actions remain approval-bound');
  }
  parts.push('raw command payloads stay out of Assistant');
  return `${parts.join('; ')}.`;
}

function buildPatrolAssessmentSuggestedPrompts(
  input: PatrolAssessmentAssistantHandoffInput,
  normalized: {
    findings: PatrolAssessmentAssistantFindingInput[];
    recentChanges: ResourceChange[];
    correlations: ResourceCorrelation[];
  },
): string[] {
  const prompts: string[] = [];
  const activeFindingCount = normalizeNonNegativeCount(input.activeFindings?.length);
  const hasCoverageGap = assessmentHasCoverageGap(input);
  const hasCoverageOnlyGap = hasCoverageGap && activeFindingCount === 0;
  const hasSupportingEvidence =
    normalized.recentChanges.length > 0 ||
    normalized.correlations.length > 0 ||
    input.investigationContext?.hasContext === true;
  const hasGovernedAction = normalized.findings.some(assessmentFindingHasGovernedAction);

  if (activeFindingCount > 0) {
    prompts.push('Prioritize findings and safest next step');
  } else if (hasCoverageOnlyGap) {
    prompts.push('Explain why coverage is incomplete');
  } else {
    prompts.push('Explain current health and what to watch');
  }

  if (hasCoverageOnlyGap) {
    prompts.push(
      input.verification?.activityMixLabel || input.latestRun?.kindLabel
        ? 'Explain scoped activity and full-run gap'
        : 'What should a full Patrol verify next?',
    );
  } else if (hasSupportingEvidence) {
    prompts.push('Explain recent changes and correlations');
  }

  if (hasGovernedAction) {
    prompts.push(
      normalizeAssessmentPendingApprovalCount(input.activeFindings) > 0
        ? 'Review pending approvals and safest next step'
        : 'Summarize governed remediation risks',
    );
  } else if (activeFindingCount > 0) {
    prompts.push('List evidence to verify before action');
  } else if (hasCoverageOnlyGap) {
    prompts.push(
      hasSupportingEvidence
        ? 'Identify early warning signals before full verification'
        : 'What should a full Patrol verify next?',
    );
  } else if (hasSupportingEvidence) {
    prompts.push('Identify early warning signals');
  }

  return formatPatrolSuggestedPrompts(prompts);
}

function assessmentHasCoverageGap(input: PatrolAssessmentAssistantHandoffInput): boolean {
  const title = normalizeText(input.assessment?.title).toLowerCase();
  const description = normalizeText(input.assessment?.description).toLowerCase();
  const prediction = normalizeText(input.overallHealth?.prediction).toLowerCase();
  const hasCoverageFactor = Boolean(
    input.overallHealth?.factors?.some(
      (factor) => normalizeText(factor.category).toLowerCase() === 'coverage',
    ),
  );

  return (
    hasCoverageFactor ||
    title.includes('coverage incomplete') ||
    description.includes('coverage incomplete') ||
    description.includes('coverage is incomplete') ||
    prediction.includes('coverage incomplete') ||
    prediction.includes('coverage is incomplete')
  );
}

function assessmentFindingHasGovernedAction(
  finding: PatrolAssessmentAssistantFindingInput,
): boolean {
  if (
    patrolAssistantFindingHandoffRequiresApprovalMode({
      investigationOutcome: finding.investigationOutcome,
      pendingApproval: finding.pendingApproval,
      investigationRecord: finding.investigationRecord,
    })
  ) {
    return true;
  }

  if (normalizeProposedFixBriefing(finding.proposedFix)) {
    return true;
  }

  const loopState = normalizeText(finding.loopState).toLowerCase();
  return loopState.includes('approval') || loopState.includes('remediation');
}

function buildPatrolAssessmentAssistantModelContext(
  input: PatrolAssessmentAssistantHandoffInput,
): string {
  const recommendedNextStep = normalizeAssessmentRecommendedNextStep(input.recommendedNextStep);
  const findings = normalizeAssessmentFindings(input.activeFindings);
  const recentChanges = normalizeAssessmentRecentChanges(input.supportingEvidence?.recentChanges);
  const correlations = normalizeAssessmentCorrelations(input.supportingEvidence?.correlations);
  const totalFindingCount = normalizeNonNegativeCount(input.activeFindings?.length);
  const omittedFindingCount = Math.max(0, totalFindingCount - findings.length);
  const totalRecentChangeCount = Math.max(
    normalizeNonNegativeCount(input.investigationContext?.recentChangeCount),
    (input.supportingEvidence?.recentChanges ?? []).length,
  );
  const omittedRecentChangeCount = Math.max(0, totalRecentChangeCount - recentChanges.length);
  const totalCorrelationCount = Math.max(
    normalizeNonNegativeCount(input.investigationContext?.correlationCount),
    (input.supportingEvidence?.correlations ?? []).length,
  );
  const omittedCorrelationCount = Math.max(0, totalCorrelationCount - correlations.length);

  return [
    '[Patrol Assessment Context]',
    'Source: Pulse Patrol current assessment',
    formatContextLine(
      'Assessment',
      normalizeText(input.assessment?.title) || 'Pulse Patrol assessment',
    ),
    formatContextLine('Assessment Description', input.assessment?.description),
    formatContextLine('Assessment Scope', input.assessment?.eyebrow),
    formatContextLine('Health', formatAssessmentHealth(input)),
    formatContextLine('Attention', formatAssessmentAttentionSummary(input)),
    formatContextLine('Recommended Next Step', recommendedNextStep?.title),
    formatContextLine('Recommended Next Step Detail', recommendedNextStep?.description),
    formatContextLine('Recommended Next Step Action', recommendedNextStep?.actionSummary),
    formatContextLine(
      'Recommended Next Step Action Status',
      formatAssessmentRecommendedNextStepActionAvailability(recommendedNextStep),
    ),
    formatContextLine('Verification', formatAssessmentVerification(input)),
    formatContextLine('Last Patrol', formatAssessmentRecency(input)),
    formatContextLine('Latest Run', formatAssessmentLatestRun(input)),
    formatContextLine('Supporting Context', input.investigationContext?.summaryText),
    ...recentChanges.map((change, index) =>
      formatAssessmentRecentChangeContextLine(change, index + 1),
    ),
    omittedRecentChangeCount > 0
      ? `${omittedRecentChangeCount} additional recent change${
          omittedRecentChangeCount === 1 ? '' : 's'
        } omitted from this bounded handoff summary.`
      : undefined,
    ...correlations.map((correlation, index) =>
      formatAssessmentCorrelationContextLine(correlation, index + 1),
    ),
    omittedCorrelationCount > 0
      ? `${omittedCorrelationCount} additional correlation${
          omittedCorrelationCount === 1 ? '' : 's'
        } omitted from this bounded handoff summary.`
      : undefined,
    ...findings.map((finding, index) => formatAssessmentFindingContextLine(finding, index + 1)),
    omittedFindingCount > 0
      ? `${omittedFindingCount} additional Patrol finding${omittedFindingCount === 1 ? '' : 's'} omitted from this bounded handoff summary.`
      : undefined,
    'Operator Boundary: This Patrol assessment handoff is model-only context for explanation and review. Diagnostics, remediation, and command execution require explicit governed approval.',
  ]
    .filter(isNonEmptyString)
    .join('\n');
}

function buildPatrolRunAssistantPrompt(
  run: PatrolRunRecord,
  kindLabel: string,
  statusLabel: string,
  runtimeFailure: string | undefined,
): string {
  const runId = normalizeText(run.id);
  const trigger = formatTriggerReason(run.trigger_reason);
  const runLabel = [kindLabel, runId].filter(isNonEmptyString).join(' ') || 'Patrol run';
  const focus = runtimeFailure
    ? `Start by explaining the Patrol runtime failure (${truncateContextText(runtimeFailure, 180)}), what likely caused it, and what should be checked before retrying Patrol.`
    : `Start by explaining the run outcome (${statusLabel}${
        trigger ? `, ${trigger.toLowerCase()}` : ''
      }) and the safest next operational step from the attached context.`;

  return [
    `Discuss this Pulse Patrol run: ${runLabel}.`,
    focus,
    'Use the attached model-only run history context before suggesting next actions.',
    'Do not infer, repeat, or execute raw command text from this handoff.',
  ]
    .filter(isNonEmptyString)
    .join('\n\n');
}

function buildPatrolRunAssistantBriefing(
  run: PatrolRunRecord,
  kindLabel: string,
  statusLabel: string,
  runtimeFailure: string | undefined,
): AIChatContextBriefing {
  const coverage = formatPatrolRunCoverage(run);
  const outcomes = formatPatrolRunOutcomes(run);
  const timing = formatPatrolRunTiming(run);
  const effort = formatPatrolRunEffort(run);
  const analysis = truncateContextText(sanitizeAnalysis(run.ai_analysis), 220);

  return {
    sourceLabel: 'Pulse Patrol',
    title: 'Patrol run attached',
    subject: [kindLabel, normalizeText(run.id)].filter(isNonEmptyString).join(' ') || kindLabel,
    statusLabel: [statusLabel, formatTriggerReason(run.trigger_reason), coverage]
      .filter(isNonEmptyString)
      .join(' · '),
    detailLines: [
      runtimeFailure ? `Runtime failure: ${runtimeFailure}` : undefined,
      timing,
      formatScope(run),
      effort,
    ]
      .filter(isNonEmptyString)
      .slice(0, 4),
    evidence: [outcomes, run.findings_summary, analysis].filter(isNonEmptyString).slice(0, 4),
    actionLabel: runtimeFailure ? 'Review Patrol runtime failure' : 'Discuss Patrol run outcome',
    safetyNote:
      'Assistant can explain the Patrol run context; retries, configuration changes, and remediation remain operator-controlled.',
    suggestedPrompts: buildPatrolRunSuggestedPrompts(Boolean(runtimeFailure)),
  };
}

function buildPatrolRunSuggestedPrompts(hasRuntimeFailure: boolean): string[] {
  if (hasRuntimeFailure) {
    return formatPatrolSuggestedPrompts([
      'Explain why this Patrol run failed',
      'List provider or model checks',
      'What should I retry after fixing it?',
    ]);
  }

  return formatPatrolSuggestedPrompts([
    'Summarize this Patrol run',
    'What needs attention from this run?',
    'What should I verify next?',
  ]);
}

function buildPatrolConfigurationFailurePrompt(
  input: PatrolConfigurationFailureInput,
  message: string,
  detailLines: string[],
): string {
  const code = normalizeText(input.code);
  const issueLabel = input.saved ? 'configuration issue' : 'configuration failure';
  return [
    `Discuss this Pulse Patrol ${issueLabel}.`,
    code ? `Server code: ${code}.` : undefined,
    `Start by explaining this failure: ${truncateContextText(message, 220)}.`,
    detailLines.length > 0 ? `Attached details: ${detailLines.join('; ')}.` : undefined,
    'Use the attached model-only configuration context before suggesting next actions.',
    'Do not infer, repeat, or execute raw command text from this handoff.',
  ]
    .filter(isNonEmptyString)
    .join('\n\n');
}

function buildPatrolConfigurationFailureModelContext(
  input: PatrolConfigurationFailureInput,
  message: string,
  detailLines: string[],
): string {
  const details = formatConfigurationFailureDetails(input.details);
  return [
    '[Patrol Configuration Failure Context]',
    'Source: Pulse Patrol configuration surface',
    formatContextLine('Server Message', message),
    formatContextLine('Server Code', input.code),
    input.status ? `HTTP Status: ${input.status}` : undefined,
    ...detailLines.map((line) => formatContextLine('Configuration Detail', line)),
    ...details.map((line) => formatContextLine('Backend Detail', line)),
    'Operator Boundary: This Patrol configuration handoff is model-only context for explanation and review. Provider changes, retries, diagnostics, remediation, and command execution require explicit governed operator action.',
  ]
    .filter(isNonEmptyString)
    .join('\n');
}

function formatConfigurationFailureSettings(
  input: PatrolConfigurationFailureInput,
): string | undefined {
  const settings = [
    input.autonomyLevel ? `mode ${input.autonomyLevel}` : undefined,
    typeof input.fullModeUnlocked === 'boolean'
      ? `autonomous critical remediation ${input.fullModeUnlocked ? 'enabled' : 'disabled'}`
      : undefined,
    typeof input.investigationBudget === 'number'
      ? `budget ${input.investigationBudget}`
      : undefined,
    typeof input.investigationTimeoutSec === 'number'
      ? `timeout ${input.investigationTimeoutSec}s`
      : undefined,
  ].filter(isNonEmptyString);

  return settings.length > 0 ? `Requested settings: ${settings.join(', ')}` : undefined;
}

function formatConfigurationFailureDetails(details?: Record<string, string>): string[] {
  if (!details) return [];
  return Object.entries(details)
    .map(([key, value]) => formatSafeConfigurationFailureDetail(key, value))
    .filter(isNonEmptyString)
    .slice(0, 6);
}

function formatSafeConfigurationFailureDetail(key: string, value: string): string | undefined {
  const normalizedKey = normalizeText(key);
  const normalizedValue = normalizeText(value);
  if (!normalizedKey || !normalizedValue) return undefined;

  const label = formatIdentifierLabel(normalizedKey) || normalizedKey;
  if (configurationFailureDetailShouldBeWithheld(normalizedKey, normalizedValue)) {
    return `${label}: sensitive or command detail withheld`;
  }

  return `${label}: ${truncateContextText(normalizedValue, 180)}`;
}

function configurationFailureDetailShouldBeWithheld(key: string, value: string): boolean {
  const normalizedKey = key.toLowerCase();
  const normalizedValue = value.toLowerCase();
  return (
    /(password|secret|token|api[_-]?key|credential|command|script|shell)/.test(normalizedKey) ||
    /\b(systemctl|sudo|bash|sh\s+-c|curl|wget|kubectl|docker|ssh)\b/.test(normalizedValue)
  );
}

function buildPatrolRunHandoffResources(run: PatrolRunRecord): AIChatHandoffResource[] {
  const type =
    run.scope_resource_types?.length === 1 ? normalizeText(run.scope_resource_types[0]) : '';
  const resources = new Map<string, AIChatHandoffResource>();

  for (const id of getCanonicalScopeResourceIds(run) ?? []) {
    const normalizedID = normalizeText(id);
    if (!normalizedID || resources.size >= MAX_PATROL_RUN_HANDOFF_RESOURCES) continue;
    resources.set(normalizedID, {
      id: normalizedID,
      type: type || undefined,
    });
  }

  return Array.from(resources.values());
}

function formatPatrolRunRuntimeFailure(run: PatrolRunRecord): string | undefined {
  return formatPatrolRuntimeFailureSummary({
    errorSummary: run.error_summary,
    errorDetail: run.error_detail,
    errorCount: run.error_count,
  });
}

function formatPatrolRunCoverage(run: PatrolRunRecord): string | undefined {
  const coverage = getPatrolRunCoverageSummary(run);
  if (coverage) return coverage;

  return formatBriefingStringList(
    [
      normalizeNonNegativeCount(run.resources_checked) > 0
        ? `${normalizeNonNegativeCount(run.resources_checked)} resources checked`
        : undefined,
      normalizeNonNegativeCount(run.nodes_checked) > 0
        ? `${normalizeNonNegativeCount(run.nodes_checked)} nodes`
        : undefined,
      normalizeNonNegativeCount(run.guests_checked) > 0
        ? `${normalizeNonNegativeCount(run.guests_checked)} VMs`
        : undefined,
      normalizeNonNegativeCount(run.docker_checked) > 0
        ? `${normalizeNonNegativeCount(run.docker_checked)} containers`
        : undefined,
      normalizeNonNegativeCount(run.storage_checked) > 0
        ? `${normalizeNonNegativeCount(run.storage_checked)} storage resources`
        : undefined,
      normalizeNonNegativeCount(run.hosts_checked) > 0
        ? `${normalizeNonNegativeCount(run.hosts_checked)} agents`
        : undefined,
      normalizeNonNegativeCount(run.truenas_checked) > 0
        ? `${normalizeNonNegativeCount(run.truenas_checked)} TrueNAS systems`
        : undefined,
      normalizeNonNegativeCount(run.kubernetes_checked) > 0
        ? `${normalizeNonNegativeCount(run.kubernetes_checked)} Kubernetes resources`
        : undefined,
    ],
    8,
    'coverage facts',
  );
}

function formatPatrolRunOutcomes(run: PatrolRunRecord): string | undefined {
  return formatBriefingStringList(
    [
      normalizeNonNegativeCount(run.new_findings) > 0
        ? `${normalizeNonNegativeCount(run.new_findings)} new finding${
            normalizeNonNegativeCount(run.new_findings) === 1 ? '' : 's'
          }`
        : undefined,
      normalizeNonNegativeCount(run.existing_findings) > 0
        ? `${normalizeNonNegativeCount(run.existing_findings)} existing finding${
            normalizeNonNegativeCount(run.existing_findings) === 1 ? '' : 's'
          }`
        : undefined,
      normalizeNonNegativeCount(run.resolved_findings) > 0
        ? `${normalizeNonNegativeCount(run.resolved_findings)} resolved finding${
            normalizeNonNegativeCount(run.resolved_findings) === 1 ? '' : 's'
          }`
        : undefined,
      normalizeNonNegativeCount(run.rejected_findings) > 0
        ? `${normalizeNonNegativeCount(run.rejected_findings)} rejected finding${
            normalizeNonNegativeCount(run.rejected_findings) === 1 ? '' : 's'
          }`
        : undefined,
      normalizeNonNegativeCount(run.auto_fix_count) > 0
        ? `${normalizeNonNegativeCount(run.auto_fix_count)} auto-remediation${
            normalizeNonNegativeCount(run.auto_fix_count) === 1 ? '' : 's'
          }`
        : undefined,
      normalizeNonNegativeCount(run.error_count) > 0
        ? `${normalizeNonNegativeCount(run.error_count)} error${
            normalizeNonNegativeCount(run.error_count) === 1 ? '' : 's'
          }`
        : undefined,
    ],
    8,
    'outcome facts',
  );
}

function formatPatrolRunTiming(run: PatrolRunRecord): string | undefined {
  return formatBriefingStringList(
    [
      normalizeText(run.started_at) ? `started ${normalizeText(run.started_at)}` : undefined,
      normalizeText(run.completed_at) ? `completed ${normalizeText(run.completed_at)}` : undefined,
      formatDurationMs(run.duration_ms)
        ? `duration ${formatDurationMs(run.duration_ms)}`
        : undefined,
    ],
    3,
    'timing facts',
  );
}

function formatPatrolRunEffort(run: PatrolRunRecord): string | undefined {
  const tokenCount =
    normalizeNonNegativeCount(run.input_tokens) + normalizeNonNegativeCount(run.output_tokens);
  return formatBriefingStringList(
    [
      normalizeNonNegativeCount(run.tool_call_count) > 0
        ? `${normalizeNonNegativeCount(run.tool_call_count)} tool call${
            normalizeNonNegativeCount(run.tool_call_count) === 1 ? '' : 's'
          }`
        : undefined,
      normalizeNonNegativeCount(run.triage_flags) > 0
        ? `${normalizeNonNegativeCount(run.triage_flags)} triage flag${
            normalizeNonNegativeCount(run.triage_flags) === 1 ? '' : 's'
          }`
        : undefined,
      run.triage_skipped_llm ? 'LLM skipped for deterministic triage' : undefined,
      tokenCount > 0 ? `${tokenCount} tokens` : undefined,
    ],
    4,
    'effort facts',
  );
}

function buildPatrolAssessmentHandoffResources(
  input: PatrolAssessmentAssistantHandoffInput,
): AIChatHandoffResource[] {
  const resources = new Map<string, AIChatHandoffResource>();
  const recentChanges = normalizeAssessmentRecentChanges(input.supportingEvidence?.recentChanges);
  const correlations = normalizeAssessmentCorrelations(input.supportingEvidence?.correlations);

  for (const finding of normalizeAssessmentFindings(input.activeFindings)) {
    addAssessmentHandoffResource(resources, getAssessmentFindingResource(finding));
  }

  for (const change of recentChanges) {
    addAssessmentHandoffResource(resources, {
      id: normalizeText(change.resourceId),
    });
    for (const relatedResource of change.relatedResources ?? []) {
      addAssessmentHandoffResource(resources, {
        id: normalizeText(relatedResource),
      });
    }
  }

  for (const correlation of correlations) {
    addAssessmentHandoffResource(resources, {
      id: normalizeText(correlation.source_id),
      name: normalizeText(correlation.source_name) || undefined,
      type: normalizeText(correlation.source_type) || undefined,
    });
    addAssessmentHandoffResource(resources, {
      id: normalizeText(correlation.target_id),
      name: normalizeText(correlation.target_name) || undefined,
      type: normalizeText(correlation.target_type) || undefined,
    });
  }

  return Array.from(resources.values());
}

function buildPatrolFindingHandoffResource(
  input: PatrolAssistantFindingHandoffInput,
): AIChatHandoffResource | undefined {
  const subject = input.investigationRecord?.subject;
  const id = normalizeText(input.resourceId) || normalizeText(subject?.resource_id);
  if (!id) return undefined;

  return {
    id,
    name:
      normalizeText(input.resourceName) ||
      normalizeText(subject?.resource_name) ||
      normalizeText(input.subject) ||
      undefined,
    type: normalizeText(input.resourceType) || normalizeText(subject?.resource_type) || undefined,
    node: normalizeText(subject?.node) || undefined,
  };
}

function addAssessmentHandoffResource(
  resources: Map<string, AIChatHandoffResource>,
  resource: AIChatHandoffResource,
): void {
  const id = normalizeText(resource.id);
  if (!id) return;

  const normalizedResource: AIChatHandoffResource = {
    id,
    name: normalizeText(resource.name) || undefined,
    type: normalizeText(resource.type) || undefined,
    node: normalizeText(resource.node) || undefined,
  };
  const existingEntry = Array.from(resources.entries()).find(([, existing]) => {
    if (existing.id !== id) return false;
    if (!normalizedResource.type || !existing.type) return true;
    return existing.type === normalizedResource.type;
  });
  const key =
    existingEntry?.[0] ||
    [normalizedResource.type, normalizedResource.id].filter(isNonEmptyString).join(':') ||
    id;
  const existing = existingEntry?.[1] || resources.get(key);

  if (!existing && resources.size >= MAX_ASSESSMENT_RESOURCES) return;

  resources.set(key, {
    id,
    name: existing?.name || normalizedResource.name,
    type: existing?.type || normalizedResource.type,
    node: existing?.node || normalizedResource.node,
  });
}

function buildPatrolAssessmentHandoffActions(
  input: PatrolAssessmentAssistantHandoffInput,
): AIChatHandoffAction[] {
  const actions: AIChatHandoffAction[] = [];

  for (const finding of normalizeAssessmentFindings(input.activeFindings)) {
    if (actions.length >= MAX_ASSESSMENT_HANDOFF_ACTIONS) break;
    const action = buildPatrolFindingHandoffAction(finding);
    if (action) actions.push(action);
  }

  return actions;
}

function buildPatrolFindingHandoffAction(
  finding: PatrolAssessmentAssistantFindingInput,
): AIChatHandoffAction | undefined {
  const pendingApproval = normalizeApprovalBriefing(finding.pendingApproval);
  const proposedFix = normalizeProposedFixBriefing(finding.proposedFix);
  const record = finding.investigationRecord;
  const recordFix = record?.proposed_fix;
  const approvalId = pendingApproval.id || normalizeText(record?.approval_id);
  const fixId = normalizeText(recordFix?.id);
  const description =
    normalizeText(proposedFix?.description) || normalizeText(recordFix?.description);

  if (!approvalId && !fixId && !description && !pendingApproval.actionId) {
    return undefined;
  }

  return {
    findingId: normalizeText(finding.id) || normalizeText(record?.finding_id) || undefined,
    recordId: normalizeText(record?.id) || undefined,
    approvalId: approvalId || undefined,
    approvalStatus: pendingApproval.status || undefined,
    approvalRequestedAt: pendingApproval.requestedAt || undefined,
    approvalExpiresAt: pendingApproval.expiresAt || undefined,
    actionId: pendingApproval.actionId || undefined,
    actionRequestedBy: pendingApproval.actionRequestedBy || undefined,
    actionApprovalPolicy: pendingApproval.actionApprovalPolicy || undefined,
    actionRequiresApproval: Boolean(approvalId),
    actionPlanExpiresAt: pendingApproval.actionPlanExpiresAt || undefined,
    actionPlanMessage: pendingApproval.actionPlanMessage || undefined,
    actionPreflight: pendingApproval.actionPreflight || undefined,
    actionDryRunSummary: pendingApproval.actionDryRunSummary || undefined,
    fixId: fixId || undefined,
    description: description || undefined,
    riskLevel:
      pendingApproval.riskLevel ||
      normalizeText(finding.proposedFix?.riskLevel) ||
      normalizeText(recordFix?.risk_level) ||
      undefined,
    destructive: Boolean(proposedFix?.destructive || recordFix?.destructive),
    targetHost:
      normalizeText(proposedFix?.targetHost) ||
      normalizeText(recordFix?.target_host) ||
      pendingApproval.targetName ||
      undefined,
    targetResourceId:
      normalizeText(finding.resourceId || record?.subject?.resource_id) || undefined,
    targetResourceName:
      normalizeText(finding.resourceName || record?.subject?.resource_name) ||
      pendingApproval.targetName ||
      undefined,
    targetResourceType:
      normalizeText(finding.resourceType || record?.subject?.resource_type) || undefined,
    targetNode: normalizeText(record?.subject?.node) || undefined,
  };
}

function normalizeAssessmentFindings(
  findings?: PatrolAssessmentAssistantFindingInput[] | null,
): PatrolAssessmentAssistantFindingInput[] {
  return (findings ?? [])
    .filter((finding) =>
      Boolean(
        normalizeText(finding.id) ||
        normalizeText(finding.title) ||
        normalizeText(finding.resourceId) ||
        normalizeText(finding.investigationRecord?.id),
      ),
    )
    .slice(0, MAX_ASSESSMENT_FINDINGS);
}

function normalizeAssessmentPendingApprovalCount(
  findings?: PatrolAssessmentAssistantFindingInput[] | null,
): number {
  return (findings ?? []).filter((finding) => {
    const approval = normalizeApprovalBriefing(finding.pendingApproval);
    return approval.id && approval.status === 'pending';
  }).length;
}

function normalizeAssessmentRecentChanges(changes?: ResourceChange[] | null): ResourceChange[] {
  return sortResourceChangesByObservedAt(
    (changes ?? []).filter((change) =>
      Boolean(
        normalizeText(change.id) || normalizeText(change.resourceId) || normalizeText(change.kind),
      ),
    ),
  )
    .map(normalizePatrolSupportingRecentChange)
    .slice(0, MAX_ASSESSMENT_RECENT_CHANGES);
}

function normalizePatrolSupportingRecentChange(change: ResourceChange): ResourceChange {
  if (!isSameStateTransition(change)) {
    return change;
  }

  const reason = formatSameStateTransitionReason(change);
  return {
    ...change,
    from: undefined,
    to: undefined,
    reason,
  };
}

function isSameStateTransition(change: ResourceChange): boolean {
  if (change.kind !== 'state_transition' && change.kind !== 'restart') {
    return false;
  }
  const from = normalizeText(change.from).toLowerCase();
  const to = normalizeText(change.to).toLowerCase();
  return Boolean(from && to && from === to);
}

function formatSameStateTransitionReason(change: ResourceChange): string {
  const state = normalizeText(change.from) || normalizeText(change.to) || 'current state';
  const changedFieldLabels = getSameStateChangedFieldLabels(change);
  if (changedFieldLabels.length > 0) {
    return `${formatCompactLabelList(changedFieldLabels)} changed while ${state}`;
  }

  const reason = normalizeText(change.reason);
  if (reason && reason.toLowerCase() !== 'resource state changed') {
    return `${reason} while ${state}`;
  }

  return `state details changed while ${state}`;
}

function getSameStateChangedFieldLabels(change: ResourceChange): string[] {
  const changedFields = change.metadata?.changedFields;
  if (!Array.isArray(changedFields)) {
    return [];
  }

  const labels: string[] = [];
  for (const field of changedFields) {
    const key = normalizeText(String(field));
    if (!key) continue;
    const label = SAME_STATE_CHANGED_FIELD_LABELS[key] ?? formatIdentifierLabel(key);
    if (label && !labels.includes(label)) {
      labels.push(label);
    }
  }
  return labels;
}

function formatCompactLabelList(labels: string[]): string {
  if (labels.length === 0) return '';
  if (labels.length === 1) return labels[0];
  if (labels.length === 2) return `${labels[0]} and ${labels[1]}`;
  return `${labels[0]}, ${labels[1]}, and ${labels.length - 2} more`;
}

function normalizeAssessmentCorrelations(
  correlations?: ResourceCorrelation[] | null,
): ResourceCorrelation[] {
  return sortResourceCorrelations(
    (correlations ?? []).filter((correlation) =>
      Boolean(
        normalizeText(correlation.source_id) ||
        normalizeText(correlation.source_name) ||
        normalizeText(correlation.target_id) ||
        normalizeText(correlation.target_name) ||
        normalizeText(correlation.event_pattern),
      ),
    ),
  ).slice(0, MAX_ASSESSMENT_CORRELATIONS);
}

function formatAssessmentHealth(input: PatrolAssessmentAssistantHandoffInput): string | undefined {
  const label = normalizeText(input.scoreChipLabel) || 'Health';
  const grade = normalizeText(input.overallHealth?.grade);
  const score = input.overallHealth?.score;
  const scoreLabel =
    typeof score === 'number' && Number.isFinite(score) ? `${Math.round(score)}/100` : '';
  return [label, grade, scoreLabel].filter(isNonEmptyString).join(' ') || undefined;
}

function formatAssessmentAttentionSummary(
  input: PatrolAssessmentAssistantHandoffInput,
): string | undefined {
  const metricState = input.metricState;
  const parts: string[] = [];
  const primaryValue = normalizeNonNegativeCount(metricState?.primaryValue);
  const secondaryValue = normalizeNonNegativeCount(metricState?.secondaryValue);
  const fixedValue = normalizeNonNegativeCount(metricState?.fixedValue);

  if (primaryValue > 0) {
    parts.push(
      formatAssessmentMetricCount(metricState?.primaryLabel || 'Active findings', primaryValue),
    );
  }
  if (
    secondaryValue > 0 &&
    normalizeText(metricState?.secondaryLabel) !== normalizeText(metricState?.primaryLabel)
  ) {
    parts.push(
      formatAssessmentMetricCount(metricState?.secondaryLabel || 'Warnings', secondaryValue),
    );
  }
  if (fixedValue > 0) {
    parts.push(formatAssessmentMetricCount(metricState?.fixedLabel || 'Fixed', fixedValue));
  }

  if (parts.length === 0) {
    const activeFindingCount = normalizeNonNegativeCount(input.activeFindings?.length);
    return activeFindingCount > 0
      ? formatAssessmentMetricCount('Active findings', activeFindingCount)
      : 'No active Patrol findings';
  }

  return parts.join(' · ');
}

function formatAssessmentMetricCount(label: string | null | undefined, value: number): string {
  const normalizedLabel = normalizeText(label) || 'Items';
  const displayLabel = value === 1 ? singularizeMetricLabel(normalizedLabel) : normalizedLabel;
  return `${value} ${displayLabel.toLowerCase()}`;
}

function singularizeMetricLabel(label: string): string {
  if (label.toLowerCase().endsWith('ies')) {
    return `${label.slice(0, -3)}y`;
  }
  if (label.toLowerCase().endsWith('s')) {
    return label.slice(0, -1);
  }
  return label;
}

function formatAssessmentVerification(
  input: PatrolAssessmentAssistantHandoffInput,
): string | undefined {
  const verification = input.verification;
  if (!verification) return undefined;

  return [
    normalizeText(verification.title),
    normalizeText(verification.description),
    normalizeText(verification.lastFullRunAt)
      ? `last full run ${normalizeText(verification.lastFullRunAt)}`
      : undefined,
    normalizeText(verification.activityMixLabel)
      ? `recent activity mix ${normalizeText(verification.activityMixLabel)}`
      : undefined,
  ]
    .filter(isNonEmptyString)
    .join('; ');
}

function formatAssessmentRecency(input: PatrolAssessmentAssistantHandoffInput): string | undefined {
  const label = normalizeText(input.recency?.label);
  const timestamp = normalizeText(input.recency?.timestamp);
  if (!label && !timestamp) return undefined;
  return [label || 'Last Patrol', timestamp].filter(isNonEmptyString).join(' ');
}

function formatAssessmentLatestRun(
  input: PatrolAssessmentAssistantHandoffInput,
): string | undefined {
  const latestRun = input.latestRun;
  if (!latestRun) return undefined;

  return [
    normalizeText(latestRun.kindLabel),
    normalizeText(latestRun.status?.label),
    normalizeText(latestRun.timestamp),
    normalizeText(latestRun.coverageSummary),
    latestRun.findingsSnapshotAvailable === false ? 'findings snapshot unavailable' : undefined,
  ]
    .filter(isNonEmptyString)
    .join('; ');
}

function formatAssessmentFindingEvidence(
  finding: PatrolAssessmentAssistantFindingInput,
): string | undefined {
  const title = normalizeText(finding.title) || 'Patrol finding';
  const resource = getAssessmentFindingResource(finding);
  const pendingApproval = normalizeApprovalBriefing(finding.pendingApproval);
  const severityStatus = [
    formatIdentifierLabel(finding.severity),
    formatIdentifierLabel(finding.status),
  ]
    .filter(isNonEmptyString)
    .join(' ');
  const approvalPosture = [
    pendingApproval.id
      ? pendingApproval.status === 'pending'
        ? 'live approval pending'
        : formatIdentifierLabel(pendingApproval.status)
      : undefined,
    formatAssessmentApprovalRiskLabel(pendingApproval.riskLevel),
  ]
    .filter(isNonEmptyString)
    .join(' ');
  const resourceLabel = formatAssessmentResourceLabel(resource);
  return [title, resourceLabel, severityStatus, approvalPosture]
    .filter(isNonEmptyString)
    .join(' · ');
}

function formatAssessmentRecentChangeEvidence(change: ResourceChange): string | undefined {
  const summary = formatAssessmentRecentChangeSummary(change);
  const resource = normalizeText(change.resourceId);
  const observedAt = normalizeText(change.observedAt);
  return [
    summary,
    resource ? `resource ${resource}` : undefined,
    observedAt ? `observed ${observedAt}` : undefined,
  ]
    .filter(isNonEmptyString)
    .join(' · ');
}

function formatAssessmentCorrelationEvidence(correlation: ResourceCorrelation): string | undefined {
  const source = formatAssessmentCorrelationEndpoint(correlation, 'source');
  const target = formatAssessmentCorrelationEndpoint(correlation, 'target');
  const pattern = truncateContextText(formatResourceCorrelationPattern(correlation), 120);
  return [
    source && target ? `${source} to ${target}` : source || target,
    pattern ? `pattern ${pattern}` : undefined,
    formatResourceCorrelationSummary(correlation),
  ]
    .filter(isNonEmptyString)
    .join(' · ');
}

function formatAssessmentRecentChangeContextLine(change: ResourceChange, index: number): string {
  const relatedResources = (change.relatedResources ?? [])
    .map(normalizeText)
    .filter(isNonEmptyString)
    .slice(0, 4);
  const parts = [
    formatAssessmentRecentChangeSummary(change),
    normalizeText(change.id) ? `change ${normalizeText(change.id)}` : undefined,
    normalizeText(change.resourceId) ? `resource ${normalizeText(change.resourceId)}` : undefined,
    normalizeText(change.observedAt) ? `observed ${normalizeText(change.observedAt)}` : undefined,
    normalizeText(change.occurredAt) ? `occurred ${normalizeText(change.occurredAt)}` : undefined,
    normalizeText(change.sourceType)
      ? `source ${formatIdentifierLabel(change.sourceType)?.toLowerCase()}`
      : undefined,
    normalizeText(change.sourceAdapter)
      ? `adapter ${formatIdentifierLabel(change.sourceAdapter)}`
      : undefined,
    normalizeText(change.confidence)
      ? `${formatIdentifierLabel(change.confidence)?.toLowerCase()} confidence`
      : undefined,
    normalizeText(change.actor) ? `actor ${truncateContextText(change.actor, 80)}` : undefined,
    relatedResources.length > 0 ? `related ${relatedResources.join(', ')}` : undefined,
  ].filter(isNonEmptyString);

  return `Recent Change ${index}: ${parts.join('; ')}`;
}

function formatAssessmentCorrelationContextLine(
  correlation: ResourceCorrelation,
  index: number,
): string {
  const source = formatAssessmentCorrelationEndpoint(correlation, 'source');
  const target = formatAssessmentCorrelationEndpoint(correlation, 'target');
  const parts = [
    source && target ? `${source} to ${target}` : source || target,
    normalizeText(correlation.event_pattern)
      ? `pattern ${truncateContextText(formatResourceCorrelationPattern(correlation), 140)}`
      : undefined,
    formatResourceCorrelationSummary(correlation),
    normalizeText(correlation.last_seen)
      ? `last seen ${normalizeText(correlation.last_seen)}`
      : undefined,
    normalizeText(correlation.description)
      ? `description ${truncateContextText(correlation.description, 180)}`
      : undefined,
  ].filter(isNonEmptyString);

  return `Correlation ${index}: ${parts.join('; ')}`;
}

function formatAssessmentRecentChangeSummary(change: ResourceChange): string {
  const kind = formatResourceChangeKind(change.kind);
  if (isCommandBearingResourceChange(change)) {
    return `${kind}: execution event recorded`;
  }

  if (
    (change.kind === 'state_transition' || change.kind === 'restart') &&
    normalizeText(change.from) &&
    normalizeText(change.to)
  ) {
    return `${kind}: ${truncateContextText(change.from, 80)} to ${truncateContextText(
      change.to,
      80,
    )}`;
  }

  if (normalizeText(change.reason)) {
    return `${kind}: ${truncateContextText(change.reason, 160)}`;
  }

  return `${kind}: ${normalizeText(change.resourceId) || normalizeText(change.id) || 'resource'}`;
}

function isCommandBearingResourceChange(change: ResourceChange): boolean {
  return change.kind === 'command_executed' || change.kind === 'runbook_executed';
}

function formatAssessmentCorrelationEndpoint(
  correlation: ResourceCorrelation,
  role: 'source' | 'target',
): string | undefined {
  const label = normalizeText(formatResourceCorrelationEndpoint(correlation, role));
  const id =
    role === 'source' ? normalizeText(correlation.source_id) : normalizeText(correlation.target_id);
  const type =
    role === 'source'
      ? normalizeText(correlation.source_type)
      : normalizeText(correlation.target_type);
  const displayLabel = label || id;
  if (!displayLabel) return undefined;

  const qualifiers = [
    type ? formatIdentifierLabel(type)?.toLowerCase() : undefined,
    id && id !== displayLabel ? id : undefined,
  ]
    .filter(isNonEmptyString)
    .join(' ');
  return qualifiers ? `${displayLabel} (${qualifiers})` : displayLabel;
}

function formatAssessmentFindingContextLine(
  finding: PatrolAssessmentAssistantFindingInput,
  index: number,
): string {
  const title = truncateContextText(normalizeText(finding.title) || 'Patrol finding', 120);
  const resource = getAssessmentFindingResource(finding);
  const record = buildPatrolInvestigationRecordPresentation(finding.investigationRecord);
  const proposedFix = record.proposedFix || normalizeProposedFixBriefing(finding.proposedFix);
  const pendingApproval = normalizeApprovalBriefing(finding.pendingApproval);
  const recordApprovalId = normalizeText(finding.investigationRecord?.approval_id);
  const statusParts = [
    formatIdentifierLabel(finding.severity),
    formatIdentifierLabel(finding.status),
    formatIdentifierLabel(finding.investigationStatus),
    formatIdentifierLabel(finding.investigationOutcome),
    formatIdentifierLabel(finding.loopState),
  ].filter(isNonEmptyString);
  const raisedParts = [
    normalizeNonNegativeCount(finding.timesRaised) > 1
      ? `raised ${normalizeNonNegativeCount(finding.timesRaised)} times`
      : undefined,
    normalizeNonNegativeCount(finding.regressionCount) > 0
      ? `regressed ${normalizeNonNegativeCount(finding.regressionCount)} time${
          normalizeNonNegativeCount(finding.regressionCount) === 1 ? '' : 's'
        }`
      : undefined,
    normalizeText(finding.lastRegressionAt)
      ? `last regression ${normalizeText(finding.lastRegressionAt)}`
      : undefined,
  ];
  const recordParts = [
    record.statusLabel,
    record.outcomeLabel,
    record.confidenceLabel,
    record.conclusion ? `conclusion ${truncateContextText(record.conclusion, 180)}` : undefined,
    record.recommendedAction
      ? `recommended ${truncateContextText(record.recommendedAction, 180)}`
      : undefined,
    recordApprovalId && recordApprovalId !== pendingApproval.id
      ? `approval ${recordApprovalId}`
      : undefined,
    ...formatAssessmentPendingApprovalContextParts(pendingApproval),
    proposedFix?.description
      ? `proposed fix ${truncateContextText(proposedFix.description, 160)}`
      : undefined,
    proposedFix?.commandSummary,
    proposedFix?.destructive ? 'destructive proposed fix' : undefined,
    record.error ? `investigation error ${truncateContextText(record.error, 160)}` : undefined,
  ];

  const parts = [
    `${title} on ${formatAssessmentResourceLabel(resource) || 'affected resource'}`,
    normalizeText(finding.id) ? `finding ${normalizeText(finding.id)}` : undefined,
    statusParts.length > 0 ? statusParts.join(' ') : undefined,
    normalizeText(finding.description)
      ? `description ${truncateContextText(finding.description, 180)}`
      : undefined,
    normalizeText(finding.detectedAt) ? `detected ${normalizeText(finding.detectedAt)}` : undefined,
    normalizeText(finding.lastSeenAt)
      ? `last seen ${normalizeText(finding.lastSeenAt)}`
      : undefined,
    ...raisedParts,
    ...recordParts,
  ].filter(isNonEmptyString);

  return `Finding ${index}: ${parts.join('; ')}`;
}

function buildPatrolAssistantFindingModelContext(
  input: PatrolAssistantFindingHandoffInput,
): string {
  const title = normalizeText(input.title) || 'Patrol finding';
  const subject = normalizeText(input.subject) || 'affected resource';
  const record = buildPatrolInvestigationRecordPresentation(input.investigationRecord);
  const pendingApproval = normalizeApprovalBriefing(input.pendingApproval);
  const proposedFix = record.proposedFix || normalizeProposedFixBriefing(input.proposedFix);
  const resource = buildPatrolFindingHandoffResource(input);
  const findingId = normalizeText(input.id) || normalizeText(input.investigationRecord?.finding_id);
  const statusParts = [
    formatIdentifierLabel(input.severity),
    formatIdentifierLabel(input.findingStatus),
    formatIdentifierLabel(input.investigationStatus),
    formatIdentifierLabel(input.investigationOutcome || input.investigationRecord?.outcome),
    formatIdentifierLabel(input.loopState),
  ].filter(isNonEmptyString);
  const raisedParts = [
    normalizeNonNegativeCount(input.timesRaised) > 1
      ? `raised ${normalizeNonNegativeCount(input.timesRaised)} times`
      : undefined,
    normalizeNonNegativeCount(input.regressionCount) > 0
      ? `regressed ${normalizeNonNegativeCount(input.regressionCount)} time${
          normalizeNonNegativeCount(input.regressionCount) === 1 ? '' : 's'
        }`
      : undefined,
    normalizeText(input.lastRegressionAt)
      ? `last regression ${normalizeText(input.lastRegressionAt)}`
      : undefined,
  ].filter(isNonEmptyString);
  const nextStepAction = normalizePatrolAssistantNextStepAction(input.nextStepAction);
  const attentionReason = buildPatrolAssistantAttentionReason(input, record);
  const operatorDecision = buildPatrolAssistantOperatorDecision(input);
  const proposedFixFacts = proposedFix
    ? formatBriefingStringList(
        [
          proposedFix.description,
          proposedFix.targetHost ? `target ${proposedFix.targetHost}` : undefined,
          proposedFix.riskLabel ? `${proposedFix.riskLabel.toLowerCase()} risk` : undefined,
          proposedFix.commandSummary,
          proposedFix.destructive ? 'destructive proposed fix' : undefined,
          proposedFix.rationale ? `rationale ${proposedFix.rationale}` : undefined,
        ],
        6,
        'proposed-fix facts',
      )
    : undefined;

  return [
    '[Patrol Finding Context]',
    'Source: Pulse Patrol finding handoff',
    formatContextLine('Finding', title),
    formatContextLine('Finding ID', findingId),
    formatContextLine('Subject', subject),
    formatContextLine('Resource', resource ? formatAssessmentResourceLabel(resource) : undefined),
    formatContextLine('Status', statusParts.join(' · ')),
    formatContextLine('Detected At', input.detectedAt),
    formatContextLine('Last Seen At', input.lastSeenAt),
    formatContextLine('Recurrence', raisedParts.join('; ')),
    formatContextLine('Description', input.description),
    formatContextLine('Attention', attentionReason),
    formatContextLine('Investigation Record', input.investigationRecord?.id),
    formatContextLine('Investigation Status', record.statusLabel),
    formatContextLine('Investigation Outcome', record.outcomeLabel),
    formatContextLine('Investigation Confidence', record.confidenceLabel),
    formatContextLine('Conclusion', record.conclusion),
    formatContextLine('Recommended Action', record.recommendedAction),
    ...record.evidenceSummaries.map((summary, index) =>
      formatContextLine(`Evidence ${index + 1}`, summary),
    ),
    ...record.verificationSummaries.map((summary, index) =>
      formatContextLine(`Verification ${index + 1}`, summary),
    ),
    formatContextLine('Tools Used', record.toolsUsed.join(', ')),
    formatContextLine('Approval', pendingApproval.id),
    formatContextLine('Approval Status', pendingApproval.status),
    formatContextLine('Approval Risk', pendingApproval.riskLevel),
    formatContextLine('Approval Target', pendingApproval.targetName),
    formatContextLine('Approval Requested At', pendingApproval.requestedAt),
    formatContextLine('Approval Expires At', pendingApproval.expiresAt),
    formatContextLine('Approval Policy', pendingApproval.actionApprovalPolicy),
    formatContextLine('Action Requested By', pendingApproval.actionRequestedBy),
    formatContextLine('Approval Plan Expires At', pendingApproval.actionPlanExpiresAt),
    formatContextLine('Action Plan Summary', pendingApproval.actionPlanMessage),
    formatContextLine('Action Preflight', pendingApproval.actionPreflight),
    formatContextLine('Dry-Run Posture', pendingApproval.actionDryRunSummary),
    formatContextLine('Proposed Fix', proposedFixFacts),
    formatContextLine('Patrol Next Step', nextStepAction.label),
    formatContextLine('Patrol Next Step Route', nextStepAction.href),
    formatContextLine('Operator Decision', operatorDecision),
    'Command Boundary: Command details stay in governed approval or remediation context; this model-only handoff may include command counts but not raw command text.',
    'Operator Boundary: This Patrol finding handoff is model-only context for explanation and review. Diagnostics, remediation, and command execution require explicit governed approval.',
  ]
    .filter(isNonEmptyString)
    .join('\n');
}

function formatAssessmentPendingApprovalContextParts(
  approval: Required<PatrolAssistantApprovalBriefingInput>,
): string[] {
  const parts: string[] = [];
  if (!approval.id) return parts;

  parts.push(`approval ${approval.id}`);
  if (approval.status === 'pending') {
    parts.push('live approval pending');
  } else {
    const statusLabel = formatIdentifierLabel(approval.status)?.toLowerCase();
    if (statusLabel) {
      parts.push(`${statusLabel} approval`);
    }
  }
  const riskLabel = formatIdentifierLabel(approval.riskLevel)?.toLowerCase();
  if (riskLabel) {
    parts.push(`${riskLabel} risk`);
  }
  if (approval.targetName) {
    parts.push(`approval target ${truncateContextText(approval.targetName, 120)}`);
  }
  if (approval.expiresAt) {
    parts.push(`expires ${approval.expiresAt}`);
  }
  if (approval.requestedAt) {
    parts.push(`requested ${approval.requestedAt}`);
  }
  if (approval.actionRequestedBy) {
    parts.push(`requested by ${approval.actionRequestedBy}`);
  }
  return parts;
}

function formatAssessmentApprovalRiskLabel(riskLevel?: string | null): string | undefined {
  const riskLabel = formatIdentifierLabel(riskLevel);
  return riskLabel ? `${riskLabel} risk` : undefined;
}

function getAssessmentFindingResource(
  finding: PatrolAssessmentAssistantFindingInput,
): AIChatHandoffResource {
  const subject = finding.investigationRecord?.subject;
  return {
    id: normalizeText(finding.resourceId) || normalizeText(subject?.resource_id),
    name: normalizeText(finding.resourceName) || normalizeText(subject?.resource_name) || undefined,
    type: normalizeText(finding.resourceType) || normalizeText(subject?.resource_type) || undefined,
    node: normalizeText(subject?.node) || undefined,
  };
}

function formatAssessmentResourceLabel(resource: AIChatHandoffResource): string | undefined {
  const name = normalizeText(resource.name);
  const id = normalizeText(resource.id);
  const type = normalizeText(resource.type);
  const node = normalizeText(resource.node);
  const label = name || id;
  if (!label) return undefined;

  const qualifiers = [type, id && id !== label ? id : undefined, node ? `node ${node}` : undefined]
    .filter(isNonEmptyString)
    .join(' ');
  return qualifiers ? `${label} (${qualifiers})` : label;
}

export function patrolAssistantFindingHandoffRequiresApprovalMode(
  input: PatrolAssistantFindingModeInput,
): boolean {
  const pendingApproval = normalizeApprovalBriefing(input.pendingApproval);
  if (pendingApproval.id) return true;
  if (normalizeText(input.remediationId)) return true;

  const record = input.investigationRecord;
  if (normalizeText(record?.approval_id)) return true;
  if (record?.proposed_fix) return true;

  const outcome = normalizeText(input.investigationOutcome || record?.outcome).toLowerCase();
  return GOVERNED_ACTION_OUTCOMES.has(outcome);
}

export function buildPatrolRemediationPlanAssistantPrompt(
  input: PatrolRemediationPlanAssistantInput,
): string {
  const title = normalizeText(input.title) || 'Patrol finding';
  const subject = normalizeText(input.subject) || 'the affected resource';
  const plan = input.plan;
  const planTitle = normalizeText(plan.title);
  const planDescription = normalizeText(plan.description);
  const riskLabel = formatIdentifierLabel(plan.risk_level)?.toLowerCase();
  const statusLabel = formatIdentifierLabel(plan.status)?.toLowerCase();

  let prompt =
    'Pulse Patrol generated a governed remediation plan for a finding. Review it from the attached plan context before suggesting next actions.\n\n';
  prompt += `**Finding:** ${title} on ${subject}\n`;
  if (planTitle) prompt += `**Plan:** ${planTitle}\n`;
  if (statusLabel) prompt += `**Plan status:** ${statusLabel}\n`;
  if (riskLabel) prompt += `**Risk level:** ${riskLabel}\n`;
  if (planDescription) prompt += `\n**Plan context:** ${planDescription}\n`;

  const steps = Array.isArray(plan.steps) ? plan.steps : [];
  if (steps.length > 0) {
    prompt += '\n**Steps:**\n';
    for (const step of steps) {
      const action = normalizeText(step.action) || `Step ${step.order}`;
      const qualifiers = [
        formatIdentifierLabel(step.risk_level)?.toLowerCase()
          ? `${formatIdentifierLabel(step.risk_level)?.toLowerCase()} risk`
          : undefined,
        step.command ? 'command recorded in governed plan' : undefined,
        step.rollback_command ? 'rollback command recorded in governed plan' : undefined,
      ].filter(isNonEmptyString);
      prompt += `${step.order}. ${action}${qualifiers.length > 0 ? ` (${qualifiers.join('; ')})` : ''}\n`;
    }
  }

  const commandSummary = formatPlanCommandSummary(plan);
  if (commandSummary) {
    prompt += `\n**Governed action details:** ${commandSummary}.\n`;
  }
  prompt +=
    '\nCommand details stay in the remediation or approval surface. Do not infer, repeat, or execute raw command text from this chat handoff. If any step is risky or ambiguous, ask before proceeding.';
  return prompt;
}

export function buildPatrolRemediationPlanAssistantBriefing(
  input: PatrolRemediationPlanAssistantInput,
): AIChatContextBriefing {
  const title = normalizeText(input.title) || 'Patrol finding';
  const subject = normalizeText(input.subject) || 'affected resource';
  const plan = input.plan;
  const steps = Array.isArray(plan.steps) ? plan.steps : [];
  const statusParts = [
    formatIdentifierLabel(plan.status),
    formatIdentifierLabel(plan.risk_level)
      ? `${formatIdentifierLabel(plan.risk_level)} risk`
      : undefined,
  ].filter(isNonEmptyString);
  const planTitle = normalizeText(plan.title);
  const planDescription = normalizeText(plan.description);
  const commandSummary = formatPlanCommandSummary(plan);
  const stepSummaries = steps
    .map((step) => {
      const action = normalizeText(step.action);
      if (!action) return undefined;
      const risk = formatIdentifierLabel(step.risk_level);
      return risk ? `${action} (${risk} risk)` : action;
    })
    .filter(isNonEmptyString)
    .slice(0, 4);

  return {
    sourceLabel: 'Pulse Patrol',
    title: 'Remediation plan attached',
    subject: `${title} on ${subject}`,
    statusLabel: statusParts.join(' · ') || undefined,
    detailLines: [
      planTitle ? `Plan: ${planTitle}` : undefined,
      planDescription,
      steps.length > 0 ? `${steps.length} planned step${steps.length === 1 ? '' : 's'}` : undefined,
    ].filter(isNonEmptyString),
    evidence: stepSummaries,
    actionLabel: planTitle || undefined,
    commandSummary,
    safetyNote: commandSummary
      ? 'Command details stay in governed remediation context; execution requires the approval flow.'
      : 'Review the governed remediation context before execution.',
    suggestedPrompts: buildPatrolRemediationPlanSuggestedPrompts(plan, steps, commandSummary),
  };
}

export function buildPatrolAssistantFindingBriefing(
  input: PatrolAssistantFindingBriefingInput,
): AIChatContextBriefing | undefined {
  const record = buildPatrolInvestigationRecordPresentation(input.investigationRecord);
  const title = normalizeText(input.title) || 'Patrol finding';
  const subject = normalizeText(input.subject) || 'affected resource';
  const pendingApproval = normalizeApprovalBriefing(input.pendingApproval);
  const proposedFix = record.proposedFix || normalizeProposedFixBriefing(input.proposedFix);
  const nextStepAction = normalizePatrolAssistantNextStepAction(input.nextStepAction);
  const approvalStatusParts = !record.hasRecord
    ? [
        pendingApproval.status ? `${formatIdentifierLabel(pendingApproval.status)} approval` : '',
        pendingApproval.riskLevel ? `${formatIdentifierLabel(pendingApproval.riskLevel)} risk` : '',
        !pendingApproval.id ? formatIdentifierLabel(input.investigationOutcome) || '' : '',
      ]
    : [];
  const statusParts = [
    record.statusLabel,
    record.outcomeLabel,
    record.confidenceLabel,
    ...approvalStatusParts,
  ].filter(isNonEmptyString);
  const attentionReason = buildPatrolAssistantAttentionReason(input, record);
  const operatorDecision = buildPatrolAssistantOperatorDecision(input);
  if (!record.hasRecord && !attentionReason && !operatorDecision) {
    return undefined;
  }
  const proposedFixDetail = record.proposedFix
    ? undefined
    : formatPatrolAssistantProposedFixDetail(proposedFix);

  const detailLines = [
    attentionReason ? `Attention: ${attentionReason}` : undefined,
    record.conclusion,
    record.recommendedAction,
    proposedFixDetail,
    operatorDecision ? `Decision: ${operatorDecision}` : undefined,
  ]
    .filter(isNonEmptyString)
    .slice(0, 4);
  const verificationLines = record.verificationSummaries.map((summary) => `Verified: ${summary}`);
  const actionLabel =
    proposedFix?.description ||
    (pendingApproval.id ? `Approval ${pendingApproval.id}` : undefined) ||
    nextStepAction.label ||
    undefined;

  return {
    sourceLabel: 'Pulse Patrol',
    title: 'Operator briefing attached',
    subject: `${title} on ${subject}`,
    statusLabel: statusParts.join(' · ') || undefined,
    detailLines,
    evidence: [...record.evidenceSummaries, ...verificationLines].slice(0, 4),
    actionLabel,
    actionHref: actionLabel === nextStepAction.label ? nextStepAction.href || undefined : undefined,
    commandSummary: proposedFix?.commandSummary,
    safetyNote: buildPatrolAssistantSafetyNote(proposedFix, pendingApproval),
    suggestedPrompts: buildPatrolFindingSuggestedPrompts(
      input,
      record,
      pendingApproval,
      proposedFix,
    ),
  };
}

function buildPatrolRemediationPlanSuggestedPrompts(
  plan: RemediationPlan,
  steps: RemediationPlan['steps'],
  commandSummary?: string,
): string[] {
  const prompts = ['Review plan risk and prerequisites'];
  const hasSteps = Array.isArray(steps) && steps.length > 0;
  const riskLevel = normalizeText(plan.risk_level).toLowerCase();

  if (commandSummary) {
    prompts.push('Explain commands without command text');
  }

  if (hasSteps) {
    prompts.push('Check rollback and verification steps');
  } else {
    prompts.push('Identify missing plan details');
  }

  if (!commandSummary && (riskLevel === 'high' || riskLevel === 'critical')) {
    prompts.push('Check rollback and failure handling');
  }

  return formatPatrolSuggestedPrompts(prompts);
}

function buildPatrolFindingSuggestedPrompts(
  input: PatrolAssistantFindingBriefingInput,
  record: PatrolInvestigationRecordPresentation,
  pendingApproval: Required<PatrolAssistantApprovalBriefingInput>,
  proposedFix?: PatrolInvestigationRecordPresentation['proposedFix'],
): string[] {
  const prompts: string[] = [];
  const requiresApproval = patrolAssistantFindingHandoffRequiresApprovalMode({
    investigationOutcome: input.investigationOutcome || input.investigationRecord?.outcome,
    remediationId: input.remediationId,
    pendingApproval,
    investigationRecord: input.investigationRecord,
  });
  const hasLoopState = Boolean(normalizeText(input.loopState));
  const hasRecurrence =
    normalizeNonNegativeCount(input.regressionCount) > 0 ||
    normalizeNonNegativeCount(input.timesRaised) > 1;
  const nextStepAction = normalizePatrolAssistantNextStepAction(input.nextStepAction);

  if (requiresApproval) {
    prompts.push('Review approval risk and next step');
  } else if (record.hasRecord) {
    prompts.push('Prioritize finding and safest next step');
  } else if (nextStepAction.label) {
    prompts.push('Review Patrol next step');
  } else {
    prompts.push('Explain current finding status');
  }

  if (record.hasRecord) {
    prompts.push('Explain Patrol evidence and confidence');
  } else if (requiresApproval) {
    prompts.push('Explain current finding status');
  } else if (hasLoopState) {
    prompts.push('Explain current Patrol loop state');
  } else {
    prompts.push('List evidence to gather before action');
  }

  if (proposedFix?.commandSummary) {
    prompts.push('Summarize remediation without command text');
  } else if (requiresApproval) {
    prompts.push('List approval prerequisites before action');
  } else if (nextStepAction.label) {
    prompts.push('Check prerequisites before next step');
  } else if (hasRecurrence) {
    prompts.push('Explain recurrence and what changed');
  } else if (hasLoopState) {
    prompts.push('Explain current Patrol loop state');
  } else {
    prompts.push('List evidence to gather before action');
  }

  return formatPatrolSuggestedPrompts(prompts);
}

function buildPatrolAssistantAttentionReason(
  input: PatrolAssistantFindingBriefingInput,
  record: PatrolInvestigationRecordPresentation,
): string | undefined {
  const parts: string[] = [];
  const status = normalizeText(input.findingStatus).toLowerCase();
  const severity = normalizeText(input.severity).toLowerCase();

  switch (status) {
    case 'active':
      parts.push(severity ? `active ${severity} finding` : 'active finding');
      break;
    case 'resolved':
      parts.push(
        normalizeNonNegativeCount(input.regressionCount) > 0
          ? 'resolved after prior regression'
          : 'resolved finding',
      );
      break;
    case 'snoozed':
      parts.push('snoozed finding');
      break;
    case 'dismissed':
      parts.push('dismissed finding');
      break;
  }

  const regressionCount = normalizeNonNegativeCount(input.regressionCount);
  const timesRaised = normalizeNonNegativeCount(input.timesRaised);
  if (regressionCount > 0) {
    parts.push(`regressed ${regressionCount} time${regressionCount === 1 ? '' : 's'}`);
  } else if (timesRaised > 1) {
    parts.push(`raised ${timesRaised} times`);
  }

  const lastRegressionAt = normalizeText(input.lastRegressionAt);
  if (lastRegressionAt) {
    parts.push(`last regression ${lastRegressionAt}`);
  }

  const loopState = formatIdentifierLabel(input.loopState)?.toLowerCase();
  if (loopState) {
    parts.push(`loop ${loopState}`);
  }

  const rawRecord = input.investigationRecord;
  const approvalId = normalizeText(rawRecord?.approval_id);
  if (approvalId) {
    parts.push(`approval ${approvalId}`);
  }
  const pendingApproval = normalizeApprovalBriefing(input.pendingApproval);
  if (pendingApproval.status === 'pending') {
    parts.push('live approval pending');
  }
  if (record.proposedFix?.destructive) {
    parts.push('destructive proposed fix');
  }

  switch (normalizeText(rawRecord?.outcome || input.investigationOutcome)) {
    case 'fix_queued':
      parts.push('fix queued for governed review');
      break;
    case 'fix_executed':
      parts.push('fix executed awaiting verification');
      break;
    case 'fix_failed':
      parts.push('fix failed');
      break;
    case 'fix_verification_failed':
      parts.push('verification failed');
      break;
    case 'fix_verification_unknown':
      parts.push('verification inconclusive');
      break;
    case 'needs_attention':
      parts.push('needs operator attention');
      break;
    case 'cannot_fix':
      parts.push('Patrol cannot safely fix');
      break;
    case 'timed_out':
      parts.push('Patrol timed out');
      break;
  }

  return formatBriefingStringList(parts, 8, 'attention facts');
}

function buildPatrolAssistantOperatorDecision(
  input: PatrolAssistantFindingBriefingInput,
): string | undefined {
  if (normalizeText(input.findingStatus).toLowerCase() === 'resolved') {
    return 'Finding is resolved; explain the resolution and monitoring follow-up without proposing execution.';
  }

  const record = input.investigationRecord;
  if (record) {
    const pendingApproval = normalizeApprovalBriefing(input.pendingApproval);
    const approvalId = pendingApproval.id || normalizeText(record.approval_id);
    if (approvalId) {
      const parts = [
        `review ${pendingApproval.status === 'pending' ? 'live ' : ''}governed approval ${approvalId} before execution`,
      ];
      if (pendingApproval.status) {
        parts.push(`approval ${pendingApproval.status}`);
      }
      if (pendingApproval.targetName) {
        parts.push(`target ${pendingApproval.targetName}`);
      }
      if (pendingApproval.expiresAt) {
        parts.push(`expires ${pendingApproval.expiresAt}`);
      }
      if (pendingApproval.requestedAt) {
        parts.push(`requested ${pendingApproval.requestedAt}`);
      }
      if (pendingApproval.actionRequestedBy) {
        parts.push(`requested by ${pendingApproval.actionRequestedBy}`);
      }
      if (record.proposed_fix) {
        const fixId = normalizeText(record.proposed_fix.id);
        if (fixId) {
          parts.push(`proposed fix ${fixId}`);
        } else if (normalizeText(record.proposed_fix.description)) {
          parts.push('proposed fix recorded');
        }
        const risk = pendingApproval.riskLevel || normalizeText(record.proposed_fix.risk_level);
        if (risk) {
          parts.push(`risk ${risk}`);
        }
        if (record.proposed_fix.destructive) {
          parts.push('destructive true');
        }
      }
      return parts.join('; ');
    }

    switch (normalizeText(record.outcome)) {
      case 'fix_queued':
        return 'Review the proposed fix in the governed approval or remediation flow before execution.';
      case 'fix_executed':
        return 'Verify the execution result before closing or resolving the finding.';
      case 'fix_failed':
      case 'fix_verification_failed':
        return 'Review failed remediation evidence before retrying or escalating.';
      case 'fix_verification_unknown':
        return 'Gather verification evidence before closing or retrying remediation.';
      case 'needs_attention':
      case 'cannot_fix':
        return 'Operator intervention is required; use the evidence to choose the next manual step.';
      case 'timed_out':
        return 'Patrol timed out; rerun investigation or gather more evidence before remediation.';
    }

    switch (normalizeText(record.status)) {
      case 'pending':
      case 'running':
        return 'Wait for Patrol to finish the investigation before approving remediation.';
      case 'failed':
        return 'Review the Patrol investigation failure and gather evidence before remediation.';
      case 'needs_attention':
        return 'Operator intervention is required; use the evidence to choose the next manual step.';
    }
  }

  const pendingApproval = normalizeApprovalBriefing(input.pendingApproval);
  if (pendingApproval.id) {
    const parts = [`Review live governed approval ${pendingApproval.id} before execution.`];
    if (pendingApproval.status) {
      parts.push(`Status: ${pendingApproval.status}.`);
    }
    if (pendingApproval.targetName) {
      parts.push(`Target: ${pendingApproval.targetName}.`);
    }
    if (pendingApproval.riskLevel) {
      parts.push(`Risk: ${pendingApproval.riskLevel}.`);
    }
    if (pendingApproval.expiresAt) {
      parts.push(`Expires: ${pendingApproval.expiresAt}.`);
    }
    if (pendingApproval.requestedAt) {
      parts.push(`Requested: ${pendingApproval.requestedAt}.`);
    }
    if (pendingApproval.actionRequestedBy) {
      parts.push(`Requested by: ${pendingApproval.actionRequestedBy}.`);
    }
    return parts.join(' ');
  }

  const remediationId = normalizeText(input.remediationId);
  if (remediationId) {
    return `Review governed remediation ${remediationId} before execution.`;
  }

  switch (normalizeText(input.investigationOutcome).toLowerCase()) {
    case 'fix_queued':
      return 'Recover or regenerate the governed approval before execution; do not execute from chat context.';
    case 'fix_executed':
      return 'Verify the execution result before closing or resolving the finding.';
    case 'fix_failed':
    case 'fix_verification_failed':
      return 'Review failed remediation evidence before retrying or escalating.';
    case 'fix_verification_unknown':
      return 'Gather verification evidence before closing or retrying remediation.';
    case 'needs_attention':
    case 'cannot_fix':
      return 'Operator intervention is required; use the evidence to choose the next manual step.';
    case 'timed_out':
      return 'Patrol timed out; rerun investigation or gather more evidence before remediation.';
  }

  const loopState = normalizeText(input.loopState).toLowerCase();
  if (loopState.includes('approval')) {
    return 'Review the governed approval flow before execution.';
  }
  if (loopState.includes('investigat')) {
    return 'Wait for Patrol to finish the investigation before approving remediation.';
  }
  const nextStepAction = normalizePatrolAssistantNextStepAction(input.nextStepAction);
  if (nextStepAction.label) {
    return `Use Patrol's next step: ${nextStepAction.label}; review the finding context before changing settings or rerunning Patrol.`;
  }
  if (normalizeText(input.findingStatus).toLowerCase() === 'active') {
    return 'Continue investigation or monitoring; no governed action reference is ready.';
  }
  return undefined;
}

function buildPatrolAssistantSafetyNote(
  proposedFix?: PatrolInvestigationRecordPresentation['proposedFix'],
  pendingApproval?: Required<PatrolAssistantApprovalBriefingInput>,
): string | undefined {
  const hasCommands = Boolean(proposedFix?.commandSummary);
  const isDestructive = Boolean(proposedFix?.destructive);
  if (hasCommands && isDestructive) {
    return 'Command details stay in approval context; destructive actions require governed approval.';
  }
  if (hasCommands && pendingApproval?.id) {
    return 'Command details stay in approval context; execution requires the governed approval flow.';
  }
  if (hasCommands) {
    return 'Command details stay in approval context.';
  }
  if (isDestructive) {
    return 'Destructive actions require governed approval.';
  }
  if (pendingApproval?.id) {
    return 'Execution requires the governed approval flow.';
  }
  return undefined;
}

function normalizeProposedFixBriefing(
  proposedFix?: PatrolAssistantProposedFixBriefingInput | null,
): PatrolInvestigationRecordPresentation['proposedFix'] | undefined {
  const commandSummary = formatCommandSummary(normalizeNonNegativeCount(proposedFix?.commandCount));
  const normalized = {
    description: normalizeText(proposedFix?.description),
    riskLabel: formatIdentifierLabel(proposedFix?.riskLevel),
    targetHost: normalizeText(proposedFix?.targetHost),
    rationale: normalizeText(proposedFix?.rationale),
    commandSummary,
    destructive: Boolean(proposedFix?.destructive),
  };

  if (
    !normalized.description &&
    !normalized.riskLabel &&
    !normalized.targetHost &&
    !normalized.rationale &&
    !normalized.commandSummary &&
    !normalized.destructive
  ) {
    return undefined;
  }

  return normalized;
}

function normalizePatrolAssistantNextStepAction(
  action?: PatrolAssistantNextStepInput | null,
): Required<PatrolAssistantNextStepInput> {
  return {
    label: normalizeText(action?.label),
    href: normalizeText(action?.href),
  };
}

function formatPatrolAssistantProposedFixDetail(
  proposedFix?: PatrolInvestigationRecordPresentation['proposedFix'],
): string | undefined {
  if (!proposedFix) return undefined;
  const detail = formatBriefingStringList(
    [
      proposedFix.description,
      proposedFix.targetHost ? `target ${proposedFix.targetHost}` : undefined,
      proposedFix.riskLabel ? `${proposedFix.riskLabel.toLowerCase()} risk` : undefined,
      proposedFix.commandSummary,
      proposedFix.destructive ? 'destructive proposed fix' : undefined,
      proposedFix.rationale ? `rationale ${proposedFix.rationale}` : undefined,
    ],
    6,
    'proposed-fix facts',
  );
  return detail ? `Proposed fix: ${detail}` : undefined;
}

function normalizeApprovalBriefing(
  approval?: PatrolAssistantApprovalBriefingInput | null,
): Required<PatrolAssistantApprovalBriefingInput> {
  return {
    id: normalizeText(approval?.id),
    status: normalizeText(approval?.status).toLowerCase(),
    riskLevel: normalizeText(approval?.riskLevel).toLowerCase(),
    requestedAt: normalizeText(approval?.requestedAt),
    expiresAt: normalizeText(approval?.expiresAt),
    targetName: normalizeText(approval?.targetName),
    actionId: normalizeText(approval?.actionId),
    actionApprovalPolicy: normalizeText(approval?.actionApprovalPolicy),
    actionPlanExpiresAt: normalizeText(approval?.actionPlanExpiresAt),
    actionPlanMessage: normalizeText(approval?.actionPlanMessage),
    actionPreflight: normalizeText(approval?.actionPreflight),
    actionDryRunSummary: normalizeText(approval?.actionDryRunSummary),
    actionRequestedBy: normalizeText(approval?.actionRequestedBy),
  };
}

function normalizeCorrelationCount(correlations?: CorrelationsResponse | null): number {
  if (!correlations) return 0;
  if (typeof correlations.count === 'number' && Number.isFinite(correlations.count)) {
    return Math.max(0, Math.trunc(correlations.count));
  }
  if (Array.isArray(correlations.correlations)) {
    return correlations.correlations.length;
  }
  return 0;
}

function normalizeNonNegativeCount(value?: number | null): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.trunc(value));
}

function formatCommandSummary(count: number): string | undefined {
  if (!Number.isFinite(count) || count <= 0) return undefined;
  return count === 1
    ? '1 command recorded for approval context'
    : `${count} commands recorded for approval context`;
}

function formatPlanCommandSummary(plan: RemediationPlan): string | undefined {
  const steps = Array.isArray(plan.steps) ? plan.steps : [];
  const commandCount = steps.filter((step) => Boolean(step.command)).length;
  const rollbackCount = steps.filter((step) => Boolean(step.rollback_command)).length;
  if (commandCount === 0 && rollbackCount === 0) return undefined;
  const parts: string[] = [];
  if (commandCount > 0) {
    parts.push(
      commandCount === 1
        ? '1 command recorded for governed plan review'
        : `${commandCount} commands recorded for governed plan review`,
    );
  }
  if (rollbackCount > 0) {
    parts.push(
      rollbackCount === 1
        ? '1 rollback command recorded'
        : `${rollbackCount} rollback commands recorded`,
    );
  }
  return parts.join('; ');
}

function formatPatrolSuggestedPrompts(values: string[]): string[] {
  return Array.from(new Set(values.map(normalizeText).filter(isNonEmptyString))).slice(
    0,
    MAX_PATROL_BRIEFING_SUGGESTED_PROMPTS,
  );
}

function normalizeAssessmentRecommendedNextStep(
  input?: PatrolAssessmentRecommendedNextStepInput | null,
): NormalizedPatrolAssessmentRecommendedNextStep | undefined {
  if (!input) return undefined;

  const actionKind = normalizeAssessmentRecommendedNextStepActionKind(input.actionKind);
  const title = formatSafeAssessmentRecommendationText(input.title, 140);
  const description = formatSafeAssessmentRecommendationText(input.description, 260);
  const fallbackActionLabel = actionKind
    ? PATROL_ASSESSMENT_RECOMMENDED_NEXT_STEP_ACTION_LABELS[actionKind]
    : undefined;
  const safeActionLabel = formatSafeAssessmentRecommendationText(input.actionLabel, 80);
  const actionLabel =
    safeActionLabel === WITHHELD_RECOMMENDATION_TEXT && fallbackActionLabel
      ? fallbackActionLabel
      : safeActionLabel || fallbackActionLabel;
  const actionDisabledReason = formatSafeAssessmentRecommendationText(
    input.actionDisabledReason,
    140,
  );
  const effectiveTitle =
    title === WITHHELD_RECOMMENDATION_TEXT && actionLabel ? actionLabel : title || actionLabel;
  const actionSummary = formatAssessmentRecommendedNextStepActionSummary(actionLabel, actionKind);

  if (!effectiveTitle && !description && !actionSummary) {
    return undefined;
  }

  return {
    title: effectiveTitle || 'Review Patrol recommendation',
    description,
    actionLabel,
    actionKind,
    actionDisabledReason,
    actionSummary,
  };
}

function normalizeAssessmentRecommendedNextStepActionKind(
  value?: string | null,
): PatrolAssessmentRecommendedNextStepActionKind | undefined {
  const normalized = normalizeText(value).toLowerCase();
  if (!normalized) return undefined;
  return Object.prototype.hasOwnProperty.call(
    PATROL_ASSESSMENT_RECOMMENDED_NEXT_STEP_ACTION_LABELS,
    normalized,
  )
    ? (normalized as PatrolAssessmentRecommendedNextStepActionKind)
    : undefined;
}

function formatAssessmentRecommendedNextStepActionSummary(
  actionLabel?: string,
  actionKind?: PatrolAssessmentRecommendedNextStepActionKind,
): string | undefined {
  if (actionLabel && actionKind) return `${actionLabel} (${actionKind})`;
  if (actionLabel) return actionLabel;
  if (actionKind) return PATROL_ASSESSMENT_RECOMMENDED_NEXT_STEP_ACTION_LABELS[actionKind];
  return undefined;
}

function formatSafeAssessmentRecommendationText(
  value?: string | null,
  limit: number = 240,
): string | undefined {
  const normalized = truncateContextText(value, limit);
  if (!normalized) return undefined;
  if (assessmentRecommendationTextShouldBeWithheld(normalized)) {
    return WITHHELD_RECOMMENDATION_TEXT;
  }
  return normalized;
}

function assessmentRecommendationTextShouldBeWithheld(value: string): boolean {
  return (
    /(password|secret|token|api[_-]?key|credential|private[_-]?key)/i.test(value) ||
    /\b(systemctl|sudo|bash|sh\s+-c|curl|wget|kubectl)\b/i.test(value) ||
    /\b(docker|ssh)\s+\S+/i.test(value)
  );
}

function formatBriefingStringList(
  values: Array<string | undefined>,
  limit: number,
  itemName: string,
): string | undefined {
  if (limit <= 0 || values.length === 0) return undefined;
  const parts: string[] = [];
  let total = 0;
  for (const value of values) {
    const normalized = normalizeText(value);
    if (!normalized) continue;
    total += 1;
    if (parts.length < limit) {
      parts.push(normalized);
    }
  }
  if (parts.length === 0) return undefined;
  const remaining = total - parts.length;
  if (remaining > 0) {
    parts.push(`${remaining} more ${itemName || 'items'}`);
  }
  return parts.join('; ');
}

function formatIdentifierLabel(value?: string | null): string | undefined {
  const normalized = normalizeText(value);
  if (!normalized) return undefined;
  return normalized
    .replace(/[._-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function formatToolLabel(value?: string | null): string {
  const normalized = normalizeText(value);
  const knownLabel = PATROL_TOOL_LABELS[normalized];
  if (knownLabel) return knownLabel;
  return formatIdentifierLabel(normalized) || '';
}

function normalizeText(value?: string | null): string {
  if (typeof value !== 'string') return '';
  return value.trim();
}

function truncateContextText(value?: string | null, limit: number = 240): string {
  const normalized = normalizeText(value).replace(/\s+/g, ' ');
  if (!normalized || normalized.length <= limit) {
    return normalized;
  }
  return `${normalized.slice(0, Math.max(0, limit - 3)).trim()}...`;
}

function formatContextLine(label: string, value?: string | null): string | undefined {
  const normalized = truncateContextText(value, 500);
  if (!normalized) return undefined;
  return `${label}: ${normalized}`;
}

function isNonEmptyString(value: string | undefined): value is string {
  return typeof value === 'string' && value.trim().length > 0;
}

const PATROL_TOOL_LABELS: Record<string, string> = {
  'metrics.history': 'Metrics history',
  'ssh.exec': 'SSH exec',
};

const GOVERNED_ACTION_OUTCOMES = new Set([
  'fix_queued',
  'fix_executed',
  'fix_failed',
  'fix_verified',
  'fix_verification_failed',
  'fix_verification_unknown',
]);
