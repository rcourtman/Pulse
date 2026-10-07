import type { ApprovalRequest, InvestigationRecord, RemediationPlan } from '@/api/ai';
import type { PatrolRunRecord } from '@/api/patrol';
import type { AttentionItemDetail } from '@/api/patrolAttention';
import type { UnifiedFinding } from '@/stores/aiIntelligence';
import type {
  AIChatContext,
  AIChatContextBriefing,
  AIChatHandoffAction,
  AIChatHandoffResource,
} from '@/stores/aiChat';
import {
  getFindingSubjectPresentation,
  getFindingTitlePresentation,
} from '@/utils/aiFindingPresentation';
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
import { formatRelativeTime } from '@/utils/format';

// formatBriefingTimestamp renders a timestamp in a way that's readable for
// both the operator (who sees the briefing copy in the Assistant drawer)
// and the LLM (which receives the same string as part of the handoff
// context). Relative time satisfies both: humans read it naturally, and
// LLMs interpret "22 mins ago" just as well as a raw ISO timestamp. The
// precise timestamp is still available in the structured handoff fields
// that flow alongside the briefing copy for any caller that needs it.
function formatBriefingTimestamp(value: string | undefined): string {
  if (!value) return '';
  return formatRelativeTime(value, { compact: false, emptyText: '' });
}

export interface PatrolInvestigationRecordPresentation {
  hasRecord: boolean;
  statusLabel: string;
  outcomeLabel?: string;
  confidenceLabel?: string;
  conclusion?: string;
  impact?: string;
  recommendedAction?: string;
  evidenceSummaries: string[];
  verificationSummaries: string[];
  rollbackSummaries: string[];
  toolsUsed: string[];
  proposedFix?: {
    description: string;
    riskLabel?: string;
    targetHost?: string;
    rationale?: string;
    commandSummary?: string;
    destructive?: boolean;
  };
  error?: string;
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

export interface PatrolAssistantFindingHandoff {
  context: AIChatContext;
}

export interface PatrolRunAssistantHandoff {
  context: AIChatContext;
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
const MAX_PATROL_RUN_HANDOFF_RESOURCES = 8;

export function buildPatrolInvestigationRecordPresentation(
  record?: InvestigationRecord | null,
): PatrolInvestigationRecordPresentation {
  if (!record) {
    return {
      hasRecord: false,
      statusLabel: '',
      evidenceSummaries: [],
      verificationSummaries: [],
      rollbackSummaries: [],
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
    impact: normalizeText(record.impact),
    recommendedAction: normalizeText(record.recommended_action),
    evidenceSummaries: (record.evidence || [])
      .map((item) => normalizeText(item.summary || item.kind || item.id))
      .filter(Boolean)
      .slice(0, 3),
    verificationSummaries: (record.verification || [])
      .map(normalizeText)
      .filter(Boolean)
      .slice(0, 3),
    rollbackSummaries: (record.rollback || []).map(normalizeText).filter(Boolean).slice(0, 3),
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

export function buildPatrolAssistantProposedFixBriefingInputFromApproval(
  approval?: ApprovalRequest | null,
): PatrolAssistantProposedFixBriefingInput | undefined {
  return buildPatrolAssistantProposedFixBriefingInput(
    approval
      ? {
          description: approval.context,
          riskLevel: approval.riskLevel,
          targetHost: approval.targetName,
          commandCount: approval.command ? 1 : 0,
        }
      : null,
  );
}

export interface PatrolUnifiedFindingHandoffOptions {
  pendingApproval?: PatrolAssistantApprovalBriefingInput | null;
  proposedFix?: PatrolAssistantProposedFixBriefingInput | null;
}

export function buildPatrolAssistantFindingHandoffInputFromUnifiedFinding(
  finding: UnifiedFinding,
  options: PatrolUnifiedFindingHandoffOptions = {},
): PatrolAssistantFindingHandoffInput {
  const title = getFindingTitlePresentation(finding).label;
  const subject = getFindingSubjectPresentation(finding).label;
  return {
    id: finding.id,
    title,
    subject,
    description: finding.description,
    severity: finding.severity,
    findingStatus: finding.status,
    investigationStatus: finding.investigationStatus,
    investigationOutcome: finding.investigationOutcome,
    loopState: finding.loopState,
    timesRaised: finding.timesRaised,
    regressionCount: finding.regressionCount,
    lastRegressionAt: finding.lastRegressionAt,
    remediationId: finding.remediationPlanId,
    resourceId: finding.resourceId,
    resourceName: finding.resourceName,
    resourceType: finding.resourceType,
    detectedAt: finding.detectedAt,
    lastSeenAt: finding.lastSeenAt,
    pendingApproval: options.pendingApproval,
    proposedFix: options.proposedFix,
    investigationRecord: finding.investigationRecord,
  };
}

export function buildPatrolAssistantFindingHandoffFromUnifiedFinding(
  finding: UnifiedFinding,
  options: PatrolUnifiedFindingHandoffOptions = {},
): PatrolAssistantFindingHandoff {
  return buildPatrolAssistantFindingHandoff(
    buildPatrolAssistantFindingHandoffInputFromUnifiedFinding(finding, options),
  );
}

/** Keep the main attention journey on the same finding/action handoff as records. */
export function buildPatrolAttentionAssistantHandoff(
  detail: AttentionItemDetail,
  linkedFindings: readonly UnifiedFinding[] = [],
): PatrolAssistantFindingHandoff {
  const item = detail.item;
  // Never choose an arbitrary finding when several findings explain one issue.
  const finding = linkedFindings.length === 1 ? linkedFindings[0] : undefined;
  const canonical = finding
    ? buildPatrolAssistantFindingHandoffFromUnifiedFinding(finding).context
    : {};
  const evidence = [...detail.evidence]
    .sort((a, b) => Date.parse(b.observedAt) - Date.parse(a.observedAt))
    .slice(0, 5)
    .map((entry) =>
      [
        `Evidence ${entry.id}: ${entry.source.provider}/${entry.source.collector}`,
        `${entry.completeness}, ${entry.confidence}, permissions ${entry.permissions}`,
        `observed ${entry.observedAt}`,
        entry.reason?.message || entry.reason?.code,
      ]
        .filter(Boolean)
        .join(' | '),
    );
  const actionReferences: AIChatHandoffAction[] = item.availableActions
    .filter((action) => Boolean(action.actionId))
    .slice(0, 5)
    .map((action) => ({
      actionId: action.actionId,
      targetResourceId: action.targetResourceId,
      actionCapability: action.capability,
      actionRequiresApproval: action.requiresApproval,
    }));
  return {
    context: {
      ...canonical,
      targetType: item.subjectResourceType || 'resource',
      targetId: item.subjectResourceId,
      autonomousMode: false,
      handoffResources: canonical.handoffResources ?? [
        {
          id: item.subjectResourceId,
          name: item.subjectResourceName,
          type: item.subjectResourceType,
        },
      ],
      handoffActions: canonical.handoffActions?.length
        ? canonical.handoffActions
        : actionReferences,
      handoffContext: [
        canonical.handoffContext,
        '[Patrol Attention Context]',
        `Attention Item: ${item.id}`,
        `Operational Record: ${item.operationalRecordId}`,
        `Resource: ${item.subjectResourceName} (${item.subjectResourceId})`,
        `State: ${item.state}`,
        `Severity: ${item.severity}`,
        `Summary: ${item.plainLanguageSummary}`,
        `Evidence: ${item.evidenceFreshness}/${item.evidenceCompleteness}`,
        item.impact ? `Impact: ${item.impact}` : '',
        item.recommendedNextStep ? `Recorded next step: ${item.recommendedNextStep}` : '',
        ...evidence,
      ]
        .filter(Boolean)
        .join('\n'),
      briefing: {
        sourceLabel: 'Pulse Patrol',
        title: item.title,
        subject: item.subjectResourceName,
        statusLabel: `${item.severity} · ${item.state}`,
      },
      context: {
        ...canonical.context,
        attentionItemId: item.id,
        operationalRecordId: item.operationalRecordId,
        lifecycleState: item.state,
        evidenceFreshness: item.evidenceFreshness,
        evidenceCompleteness: item.evidenceCompleteness,
        protectionPosture: item.protectionPosture,
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

  return {
    context: {
      targetType: resource?.type,
      targetId: resource?.id,
      findingId: findingId || undefined,
      autonomousMode: false,
      handoffContext: buildPatrolAssistantFindingModelContext(input),
      handoffResources: handoffResources.length > 0 ? handoffResources : undefined,
      handoffActions: handoffActions.length > 0 ? handoffActions : undefined,
      handoffMetadata: {
        kind: 'patrol_finding',
      },
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
      }),
      context: {
        source: 'pulse-patrol-finding',
        findingId: findingId || undefined,
        investigationRecordId: normalizeText(input.investigationRecord?.id) || undefined,
        resourceId: resource?.id,
        resourceName: resource?.name,
        resourceType: resource?.type,
        pendingApprovalId: normalizeApprovalBriefing(input.pendingApproval).id || undefined,
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

export function buildPatrolAssistantFindingHandoffActions(
  finding: PatrolAssessmentAssistantFindingInput,
): AIChatHandoffAction[] {
  const action = buildPatrolFindingHandoffAction(finding);
  return action ? [action] : [];
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
      'Assistant can explain the Patrol run context. Retries, configuration changes, and remediation remain operator-controlled.',
  };
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
    approvalStatus: approvalId ? pendingApproval.status || undefined : undefined,
    actionState: pendingApproval.actionId ? pendingApproval.status || undefined : undefined,
    approvalRequestedAt: pendingApproval.requestedAt || undefined,
    approvalExpiresAt: pendingApproval.expiresAt || undefined,
    actionId: pendingApproval.actionId || undefined,
    actionRequestedBy: pendingApproval.actionRequestedBy || undefined,
    actionApprovalPolicy: pendingApproval.actionApprovalPolicy || undefined,
    actionRequiresApproval: Boolean(
      approvalId ||
      pendingApproval.status === 'pending_approval' ||
      (pendingApproval.actionApprovalPolicy && pendingApproval.actionApprovalPolicy !== 'none'),
    ),
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
    destructive: proposedFix?.destructive ?? recordFix?.destructive,
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
    formatBriefingTimestamp(normalizeText(input.lastRegressionAt))
      ? `last regression ${formatBriefingTimestamp(normalizeText(input.lastRegressionAt))}`
      : undefined,
  ].filter(isNonEmptyString);
  const actionArtifactFacts = proposedFix
    ? formatBriefingStringList(
        [
          proposedFix.description,
          proposedFix.targetHost ? `target ${proposedFix.targetHost}` : undefined,
          proposedFix.riskLabel ? `${proposedFix.riskLabel.toLowerCase()} risk` : undefined,
          proposedFix.commandSummary,
          proposedFix.destructive ? 'destructive action artifact' : undefined,
          proposedFix.rationale ? `rationale ${proposedFix.rationale}` : undefined,
        ],
        6,
        'action-artifact facts',
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
    formatContextLine('Recurrence', raisedParts.join(' · ')),
    formatContextLine('Description', input.description),
    formatContextLine('Investigation Record', input.investigationRecord?.id),
    formatContextLine('Investigation Status', record.statusLabel),
    formatContextLine('Investigation Outcome', record.outcomeLabel),
    formatContextLine('Investigation Confidence', record.confidenceLabel),
    formatContextLine('Conclusion', record.conclusion),
    formatContextLine(
      'Impact',
      record.hasRecord ? record.impact || 'Impact not assessed' : record.impact,
    ),
    formatContextLine('Recorded Action Note', record.recommendedAction),
    ...record.evidenceSummaries.map((summary, index) =>
      formatContextLine(`Evidence ${index + 1}`, summary),
    ),
    ...record.verificationSummaries.map((summary, index) =>
      formatContextLine(`Verification ${index + 1}`, summary),
    ),
    ...(record.rollbackSummaries.length > 0
      ? record.rollbackSummaries.map((summary, index) =>
          formatContextLine(`Rollback ${index + 1}`, summary),
        )
      : record.hasRecord
        ? [formatContextLine('Rollback', 'Rollback not specified')]
        : []),
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
    formatContextLine('Existing Action Artifact', actionArtifactFacts),
    'Command Boundary: Command details stay in governed approval or remediation context. This model-only handoff may include command counts but not raw command text.',
    'Model Boundary: This Patrol finding handoff is model-only context for explanation and review. Use available diagnostic tools within current permissions. This handoff grants no new action authority or permission to retry a refused provider path.',
  ]
    .filter(isNonEmptyString)
    .join('\n');
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

export function buildPatrolRemediationPlanAssistantModelContext(
  input: PatrolRemediationPlanAssistantInput,
): string {
  const title = normalizeText(input.title) || 'Patrol finding';
  const subject = normalizeText(input.subject) || 'the affected resource';
  const plan = input.plan;
  const planTitle = normalizeText(plan.title);
  const planDescription = normalizeText(plan.description);
  const riskLabel = formatIdentifierLabel(plan.risk_level)?.toLowerCase();
  const statusLabel = formatIdentifierLabel(plan.status)?.toLowerCase();
  const commandSummary = formatPlanCommandSummary(plan);

  return [
    '[Patrol Finding Action Context]',
    'Pulse is attaching observed finding context and any existing governed action artifact. The selected language model should decide whether remediation is appropriate and what should happen next.',
    formatContextLine('Finding', `${title} on ${subject}`),
    formatContextLine('Existing Action Artifact', planTitle),
    formatContextLine('Artifact Status', statusLabel),
    formatContextLine('Recorded Risk', riskLabel),
    formatContextLine('Attached Action Context', planDescription),
    commandSummary
      ? `Governed Action Context: ${commandSummary}. Treat this as approval state, not remediation guidance.`
      : undefined,
    'Model Boundary: Do not assume any Patrol-authored action is correct. Use the finding evidence, available tools, and current operational state to decide the remediation. Any state-changing command must go through governed approval. Do not infer, repeat, or execute raw command text from this chat handoff.',
  ]
    .filter(isNonEmptyString)
    .join('\n');
}

export function buildPatrolRemediationPlanAssistantBriefing(
  input: PatrolRemediationPlanAssistantInput,
): AIChatContextBriefing {
  const title = normalizeText(input.title) || 'Patrol finding';
  const subject = normalizeText(input.subject) || 'affected resource';
  const plan = input.plan;
  const statusParts = [
    formatIdentifierLabel(plan.status),
    formatIdentifierLabel(plan.risk_level)
      ? `${formatIdentifierLabel(plan.risk_level)} risk`
      : undefined,
  ].filter(isNonEmptyString);
  const planTitle = normalizeText(plan.title);
  const planDescription = normalizeText(plan.description);
  const commandSummary = formatPlanCommandSummary(plan);

  return {
    sourceLabel: 'Pulse Patrol',
    title: 'Patrol finding attached',
    subject: `${title} on ${subject}`,
    statusLabel: statusParts.join(' · ') || undefined,
    detailLines: [
      planTitle ? `Existing action artifact: ${planTitle}` : undefined,
      planDescription,
    ].filter(isNonEmptyString),
    commandSummary,
    safetyNote: commandSummary
      ? 'Assistant should decide remediation from evidence. Command execution requires governed approval.'
      : 'Assistant should decide remediation from evidence before any governed action.',
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
        pendingApproval.status
          ? `${formatIdentifierLabel(pendingApproval.status)} ${pendingApproval.actionId ? 'action' : 'approval'}`
          : '',
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
  const hasFindingFacts = [
    input.severity,
    input.findingStatus,
    input.investigationOutcome,
    input.loopState,
    normalizeNonNegativeCount(input.timesRaised) > 0 ? String(input.timesRaised) : '',
    normalizeNonNegativeCount(input.regressionCount) > 0 ? String(input.regressionCount) : '',
    input.lastRegressionAt,
  ].some((value) => normalizeText(value).length > 0);
  if (!record.hasRecord && !hasFindingFacts && !pendingApproval.id && !proposedFix) {
    return undefined;
  }
  const actionArtifactDetail = formatPatrolAssistantActionArtifactDetail(proposedFix);

  const detailLines = [
    record.conclusion,
    record.impact ? `Impact: ${record.impact}` : undefined,
    actionArtifactDetail,
  ]
    .filter(isNonEmptyString)
    .slice(0, 4);
  const verificationLines = record.verificationSummaries.map((summary) => `Verified: ${summary}`);

  return {
    sourceLabel: 'Pulse Patrol',
    title: 'Patrol finding attached',
    subject: `${title} on ${subject}`,
    statusLabel: statusParts.join(' · ') || undefined,
    detailLines,
    evidence: [...record.evidenceSummaries, ...verificationLines].slice(0, 4),
    actionLabel: undefined,
    commandSummary: proposedFix?.commandSummary,
    safetyNote: buildPatrolAssistantSafetyNote(proposedFix, pendingApproval),
  };
}

function buildPatrolAssistantSafetyNote(
  proposedFix?: PatrolInvestigationRecordPresentation['proposedFix'],
  pendingApproval?: Required<PatrolAssistantApprovalBriefingInput>,
): string | undefined {
  const hasCommands = Boolean(proposedFix?.commandSummary);
  const isDestructive = Boolean(proposedFix?.destructive);
  if (hasCommands && isDestructive) {
    return 'Command details stay in approval context. Destructive actions require governed approval.';
  }
  if (hasCommands && pendingApproval?.id) {
    return 'Command details stay in approval context. Execution requires the governed approval flow.';
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
    destructive:
      typeof proposedFix?.destructive === 'boolean' ? proposedFix.destructive : undefined,
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

function formatPatrolAssistantActionArtifactDetail(
  proposedFix?: PatrolInvestigationRecordPresentation['proposedFix'],
): string | undefined {
  if (!proposedFix) return undefined;
  const detail = formatBriefingStringList(
    [
      proposedFix.description,
      proposedFix.targetHost ? `target ${proposedFix.targetHost}` : undefined,
      proposedFix.riskLabel ? `${proposedFix.riskLabel.toLowerCase()} risk` : undefined,
      proposedFix.commandSummary,
      proposedFix.destructive ? 'destructive action artifact' : undefined,
      proposedFix.rationale ? `rationale ${proposedFix.rationale}` : undefined,
    ],
    6,
    'action-artifact facts',
  );
  return detail ? `Existing action artifact: ${detail}` : undefined;
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
  return parts.join(' · ');
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
  return parts.join(' · ');
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
  'fix_rejected',
  'fix_verified',
  'fix_verification_failed',
  'fix_verification_unknown',
]);
