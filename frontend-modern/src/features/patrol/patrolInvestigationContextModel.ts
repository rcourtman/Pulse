import type {
  CorrelationsResponse,
  IntelligencePolicyPostureSummary,
} from '@/types/aiIntelligence';
import type { InvestigationRecord, RemediationPlan } from '@/api/ai';
import type { AIChatContextBriefing } from '@/stores/aiChat';

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

export interface PatrolAssistantFindingBriefingInput {
  title: string;
  subject: string;
  severity?: string | null;
  findingStatus?: string | null;
  loopState?: string | null;
  timesRaised?: number | null;
  regressionCount?: number | null;
  lastRegressionAt?: string | null;
  remediationId?: string | null;
  pendingApproval?: PatrolAssistantApprovalBriefingInput | null;
  investigationRecord?: InvestigationRecord | null;
}

export interface PatrolRemediationPlanAssistantInput {
  title: string;
  subject: string;
  plan: RemediationPlan;
}

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

  const proposedFix = record.proposed_fix
    ? {
        description: normalizeText(record.proposed_fix.description),
        riskLabel: formatIdentifierLabel(record.proposed_fix.risk_level),
        targetHost: normalizeText(record.proposed_fix.target_host),
        rationale: normalizeText(record.proposed_fix.rationale),
        commandSummary: formatCommandSummary(record.proposed_fix.commands?.length ?? 0),
        destructive: Boolean(record.proposed_fix.destructive),
      }
    : undefined;

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
  };
}

export function buildPatrolAssistantFindingBriefing(
  input: PatrolAssistantFindingBriefingInput,
): AIChatContextBriefing | undefined {
  const record = buildPatrolInvestigationRecordPresentation(input.investigationRecord);
  const title = normalizeText(input.title) || 'Patrol finding';
  const subject = normalizeText(input.subject) || 'affected resource';
  const statusParts = [record.statusLabel, record.outcomeLabel, record.confidenceLabel].filter(
    isNonEmptyString,
  );
  const attentionReason = buildPatrolAssistantAttentionReason(input, record);
  const operatorDecision = buildPatrolAssistantOperatorDecision(input);
  if (!record.hasRecord && !attentionReason && !operatorDecision) {
    return undefined;
  }

  const detailLines = [
    attentionReason ? `Attention: ${attentionReason}` : undefined,
    record.conclusion,
    record.recommendedAction,
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
    actionLabel: record.proposedFix?.description,
    commandSummary: record.proposedFix?.commandSummary,
    safetyNote: buildPatrolAssistantSafetyNote(record),
  };
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

  switch (normalizeText(rawRecord?.outcome)) {
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
    if (pendingApproval.expiresAt) {
      parts.push(`Expires: ${pendingApproval.expiresAt}.`);
    }
    return parts.join(' ');
  }

  const remediationId = normalizeText(input.remediationId);
  if (remediationId) {
    return `Review governed remediation ${remediationId} before execution.`;
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
  record: PatrolInvestigationRecordPresentation,
): string | undefined {
  const hasCommands = Boolean(record.proposedFix?.commandSummary);
  const isDestructive = Boolean(record.proposedFix?.destructive);
  if (hasCommands && isDestructive) {
    return 'Command details stay in approval context; destructive actions require governed approval.';
  }
  if (hasCommands) {
    return 'Command details stay in approval context.';
  }
  if (isDestructive) {
    return 'Destructive actions require governed approval.';
  }
  return undefined;
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

function isNonEmptyString(value: string | undefined): value is string {
  return typeof value === 'string' && value.trim().length > 0;
}

const PATROL_TOOL_LABELS: Record<string, string> = {
  'metrics.history': 'Metrics history',
  'ssh.exec': 'SSH exec',
};
