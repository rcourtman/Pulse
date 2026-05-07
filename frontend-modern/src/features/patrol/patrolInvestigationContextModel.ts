import type {
  CorrelationsResponse,
  IntelligencePolicyPostureSummary,
  ResourceCorrelation,
} from '@/types/aiIntelligence';
import type { InvestigationRecord, RemediationPlan } from '@/api/ai';
import type { AIChatContext, AIChatContextBriefing, AIChatHandoffResource } from '@/stores/aiChat';
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
  investigationRecord?: InvestigationRecord | null;
}

export interface PatrolAssistantApprovalBriefingInput {
  id?: string | null;
  status?: string | null;
  riskLevel?: string | null;
  requestedAt?: string | null;
  expiresAt?: string | null;
  targetName?: string | null;
}

export interface PatrolAssistantProposedFixBriefingInput {
  description?: string | null;
  riskLevel?: string | null;
  targetHost?: string | null;
  rationale?: string | null;
  commandCount?: number | null;
  destructive?: boolean | null;
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

export interface PatrolAssessmentAssistantHandoffInput {
  assessment?: {
    title?: string | null;
    description?: string | null;
    eyebrow?: string | null;
  } | null;
  overallHealth?: {
    grade?: string | null;
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
  activeFindings?: PatrolAssessmentAssistantFindingInput[] | null;
}

export interface PatrolAssessmentAssistantHandoff {
  prompt: string;
  context: Omit<AIChatContext, 'initialPrompt'>;
}

const MAX_ASSESSMENT_FINDINGS = 5;
const MAX_ASSESSMENT_RECENT_CHANGES = 3;
const MAX_ASSESSMENT_CORRELATIONS = 3;
const MAX_ASSESSMENT_RESOURCES = 8;
const MAX_PATROL_BRIEFING_SUGGESTED_PROMPTS = 3;

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

export function buildPatrolAssistantFindingPrompt(
  input: PatrolAssistantFindingPromptInput,
): string {
  const title = normalizeText(input.title) || 'Patrol finding';
  const subject = normalizeText(input.subject) || 'the affected resource';
  const description = normalizeText(input.description);
  const hasRecord = Boolean(input.investigationRecord?.id);

  let prompt = `I'd like to discuss this Patrol finding: "${title}" on ${subject}.`;
  if (hasRecord) {
    prompt +=
      '\n\nPulse Patrol has a structured investigation record for this finding. Use that record as the main context before suggesting next actions.';
  }
  if (description) {
    prompt += `\n\n${description}`;
  }
  return prompt;
}

export function buildPatrolAssessmentAssistantHandoff(
  input: PatrolAssessmentAssistantHandoffInput,
): PatrolAssessmentAssistantHandoff {
  const title = normalizeText(input.assessment?.title) || 'Pulse Patrol assessment';
  const description = normalizeText(input.assessment?.description);
  const handoffContext = buildPatrolAssessmentAssistantModelContext(input);
  const recentChanges = normalizeAssessmentRecentChanges(input.supportingEvidence?.recentChanges);
  const correlations = normalizeAssessmentCorrelations(input.supportingEvidence?.correlations);

  return {
    prompt: [
      `Discuss the current Pulse Patrol assessment: ${title}.`,
      description,
      'Use the attached model-only Patrol assessment context before suggesting next actions. Help me understand priority, risk, and safe next steps.',
      'Do not infer, repeat, or execute raw command text from this handoff.',
    ]
      .filter(isNonEmptyString)
      .join('\n\n'),
    context: {
      targetType: 'dashboard',
      targetId: 'pulse-patrol-assessment',
      autonomousMode: false,
      handoffContext,
      handoffResources: buildPatrolAssessmentHandoffResources(input),
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
      },
    },
  };
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
  const findings = normalizeAssessmentFindings(input.activeFindings);
  const recentChanges = normalizeAssessmentRecentChanges(input.supportingEvidence?.recentChanges);
  const correlations = normalizeAssessmentCorrelations(input.supportingEvidence?.correlations);
  const findingEvidence = findings.map(formatAssessmentFindingEvidence).filter(isNonEmptyString);
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
    detailLines: [description, verification, latestRun, contextSummary]
      .filter(isNonEmptyString)
      .slice(0, 4),
    evidence: [...findingEvidence.slice(0, 3), ...supportingEvidence].slice(0, 5),
    actionLabel: 'Discuss Patrol assessment',
    safetyNote: 'Diagnostics and remediation require governed approval.',
    suggestedPrompts: buildPatrolAssessmentSuggestedPrompts(input, {
      findings,
      recentChanges,
      correlations,
    }),
  };
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
  const hasSupportingEvidence =
    normalized.recentChanges.length > 0 || normalized.correlations.length > 0;
  const hasGovernedAction = normalized.findings.some(assessmentFindingHasGovernedAction);

  if (activeFindingCount > 0) {
    prompts.push('Prioritize findings and safest next step');
  } else {
    prompts.push('Explain current health and what to watch');
  }

  if (hasSupportingEvidence) {
    prompts.push('Explain recent changes and correlations');
  }

  if (hasGovernedAction) {
    prompts.push('Summarize governed remediation risks');
  } else if (activeFindingCount > 0) {
    prompts.push('List evidence to verify before action');
  } else if (hasSupportingEvidence) {
    prompts.push('Identify early warning signals');
  }

  return formatPatrolSuggestedPrompts(prompts);
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
  ).slice(0, MAX_ASSESSMENT_RECENT_CHANGES);
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

  return {
    sourceLabel: 'Pulse Patrol',
    title: 'Operator briefing attached',
    subject: `${title} on ${subject}`,
    statusLabel: statusParts.join(' · ') || undefined,
    detailLines,
    evidence: [...record.evidenceSummaries, ...verificationLines].slice(0, 4),
    actionLabel:
      proposedFix?.description ||
      (pendingApproval.id ? `Approval ${pendingApproval.id}` : undefined),
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

  if (requiresApproval) {
    prompts.push('Review approval risk and next step');
  } else if (record.hasRecord) {
    prompts.push('Prioritize finding and safest next step');
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
