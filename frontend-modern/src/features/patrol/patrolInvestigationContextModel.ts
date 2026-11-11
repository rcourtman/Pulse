import type { InvestigationRecord, InvestigationRecordFix } from '@/api/ai';
import { getInvestigationOutcomeLabel } from '@/utils/aiFindingPresentation';

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

  const proposedFix = presentProposedFix(record.proposed_fix);

  return {
    hasRecord: true,
    statusLabel: formatIdentifierLabel(record.status) || 'Investigation recorded',
    outcomeLabel: formatOutcomeLabel(record.outcome),
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

export interface PatrolInvestigationVerdict {
  /** The investigation's own one-line decision, without its outcome marker. */
  text: string;
}

const SUMMARY_HEADING = /^\s{0,3}#{1,6}\s+(.*?)\s*#*\s*$/;
const OUTCOME_MARKER = /^\s*(?:\*\*)?(?:RESOLVED|CANNOT_FIX|NEEDS_ATTENTION)(?:\*\*)?\s*:\s*/i;
// Identifiers the model repeats beside a name, such as
// "nextcloud-db (app-container-f678dd3b3f36f8ee, action_id act_a6336838...)".
// The name already says which resource it is, the button opens the action,
// and the full text stays in the expansion.
const ACTION_ID_MENTION = /,?\s*\baction[_ ]id\s+act_[0-9a-f]+/gi;
const REGISTRY_ID = /\b(?:[a-z][a-z0-9]*-)+[0-9a-f]{16}\b/g;
const EMPTY_ASIDE = /\s*\(\s*[,;]?\s*\)/g;

/**
 * The line an investigation ends on: its Conclusion section, which the
 * investigation prompt asks for as one outcome line. A finding review leads
 * with it, because it is the answer the rest of the summary argues for.
 * Returns null for a summary without that section, such as a finding nobody
 * investigated, whose record carries the first-look description instead.
 */
export function getPatrolInvestigationVerdict(
  summary?: string | null,
): PatrolInvestigationVerdict | null {
  if (typeof summary !== 'string') return null;
  let inConclusion = false;
  const lines: string[] = [];
  for (const raw of summary.split('\n')) {
    const heading = SUMMARY_HEADING.exec(raw);
    if (heading) {
      inConclusion = heading[1].trim().toLowerCase() === 'conclusion';
      continue;
    }
    if (inConclusion && raw.trim()) lines.push(raw.trim());
  }
  const text = lines
    .join(' ')
    .replace(OUTCOME_MARKER, '')
    .replace(ACTION_ID_MENTION, '')
    .replace(REGISTRY_ID, '')
    .replace(EMPTY_ASIDE, '')
    .replace(/\s+([,.;:])/g, '$1')
    .replace(/\s+/g, ' ')
    .trim();
  return text ? { text } : null;
}

function presentProposedFix(
  fix?: InvestigationRecordFix | null,
): PatrolInvestigationRecordPresentation['proposedFix'] | undefined {
  if (!fix) return undefined;
  const normalized = {
    description: normalizeText(fix.description),
    riskLabel: formatIdentifierLabel(fix.risk_level),
    targetHost: normalizeText(fix.target_host),
    rationale: normalizeText(fix.rationale),
    commandSummary: formatCommandSummary(Array.isArray(fix.commands) ? fix.commands.length : 0),
    destructive: typeof fix.destructive === 'boolean' ? fix.destructive : undefined,
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

// The record's outcome badge sits beside the investigation's own outcome
// badge, so both use the shared outcome labels.
function formatOutcomeLabel(value?: string | null): string | undefined {
  const normalized = normalizeText(value);
  return normalized ? getInvestigationOutcomeLabel(normalized) : undefined;
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
