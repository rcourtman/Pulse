import type {
  CorrelationsResponse,
  IntelligencePolicyPostureSummary,
} from '@/types/aiIntelligence';
import type { InvestigationRecord } from '@/api/ai';

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

const PATROL_TOOL_LABELS: Record<string, string> = {
  'metrics.history': 'Metrics history',
  'ssh.exec': 'SSH exec',
};
