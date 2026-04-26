import type { DashboardRecoverySummary } from '@/hooks/useDashboardRecovery';
import type { DashboardOverview, ProblemResource } from '@/hooks/useDashboardOverview';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';
import type { DashboardEstateSummary } from './estateSummaryModel';

export type DashboardPulseBriefTone = 'healthy' | 'attention' | 'critical';

export interface DashboardPulseBriefInput {
  estate: DashboardEstateSummary;
  overview: DashboardOverview;
  storageCapacityPercent: number;
  recovery: DashboardRecoverySummary;
  pendingApprovalCount: number;
  patrolFindingCount: number;
}

export interface DashboardPulseBrief {
  tone: DashboardPulseBriefTone;
  title: string;
  body: string;
  evidence: string[];
  assistantPrompt: string;
  assistantContext: Record<string, unknown>;
}

const pluralize = (count: number, singular: string, plural = `${singular}s`): string =>
  `${count} ${count === 1 ? singular : plural}`;

function joinNatural(items: string[]): string {
  if (items.length <= 1) return items[0] ?? '';
  if (items.length === 2) return `${items[0]} and ${items[1]}`;
  return `${items.slice(0, -1).join(', ')}, and ${items[items.length - 1]}`;
}

function normalizeCount(value: number | undefined): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.trunc(value ?? 0));
}

function problemResourceLabel(problem: ProblemResource | undefined): string | null {
  if (!problem) return null;
  const name = getPreferredResourceDisplayName(problem.resource).trim();
  if (!name) return null;
  const normalizedName = name.toLowerCase();
  const typeLabel = getResourceTypeLabel(problem.resource.type)?.trim();
  const typeNoun =
    typeLabel && typeLabel === typeLabel.toUpperCase() ? typeLabel : typeLabel?.toLowerCase();
  const normalizedTypeLabel = typeLabel?.toLowerCase();
  const normalizedRawType = problem.resource.type.trim().toLowerCase().replace(/[-_]+/g, ' ');
  const genericStatusNames = new Set(
    [normalizedTypeLabel, normalizedRawType]
      .filter((value): value is string => Boolean(value))
      .flatMap((value) => [
        value,
        ...problem.problems.map((reason) => `${value} (${reason.toLowerCase()})`),
      ]),
  );
  const displayName =
    typeNoun && genericStatusNames.has(normalizedName) ? `the ${typeNoun} issue` : name;
  const normalizedDisplayName = displayName.toLowerCase();
  const reasons = problem.problems
    .filter((reason) => {
      const normalizedReason = reason.toLowerCase();
      return (
        !normalizedName.includes(normalizedReason) &&
        !normalizedDisplayName.includes(normalizedReason)
      );
    })
    .slice(0, 2)
    .join(', ')
    .trim();
  return reasons ? `${displayName} (${reasons})` : displayName;
}

function buildAttentionParts(input: DashboardPulseBriefInput): string[] {
  const parts: string[] = [];
  const resourceIssues = input.overview.problemResources.length;
  const activeAlerts = normalizeCount(input.overview.alerts.total);
  const storageWarnings = normalizeCount(input.overview.storage.warningCount);
  const storageCritical = normalizeCount(input.overview.storage.criticalCount);
  const recoveryFailures =
    normalizeCount(input.recovery.byOutcome.failed) +
    normalizeCount(input.recovery.byOutcome.error);

  if (resourceIssues > 0) parts.push(pluralize(resourceIssues, 'resource issue'));
  if (activeAlerts > 0) parts.push(pluralize(activeAlerts, 'active alert'));
  if (storageCritical > 0) parts.push(pluralize(storageCritical, 'critical storage signal'));
  if (storageWarnings > 0) parts.push(pluralize(storageWarnings, 'storage warning'));
  if (recoveryFailures > 0) parts.push(pluralize(recoveryFailures, 'recovery failure'));
  if (input.pendingApprovalCount > 0) {
    parts.push(pluralize(input.pendingApprovalCount, 'pending approval'));
  }
  if (input.patrolFindingCount > 0) {
    parts.push(pluralize(input.patrolFindingCount, 'Patrol finding'));
  }

  return parts;
}

function buildBody(input: DashboardPulseBriefInput, attentionParts: string[]): string {
  const systems = input.estate.totalSystems;
  const storagePercent = Math.round(input.storageCapacityPercent);
  const topProblem = problemResourceLabel(input.overview.problemResources[0]);
  const activeAlerts = normalizeCount(input.overview.alerts.total);
  const criticalAlerts = normalizeCount(input.overview.alerts.activeCritical);
  const warningAlerts = normalizeCount(input.overview.alerts.activeWarning);
  const recoveryFailures =
    normalizeCount(input.recovery.byOutcome.failed) +
    normalizeCount(input.recovery.byOutcome.error);

  const opening =
    attentionParts.length > 0
      ? `${input.estate.headline}. Pulse sees ${joinNatural(attentionParts)} in the current dashboard signals.`
      : `All ${pluralize(systems, 'monitored system')} ${
          systems === 1 ? 'is' : 'are'
        } reporting cleanly across the current dashboard signals.`;

  const review =
    topProblem !== null
      ? `Review ${topProblem} first; it is the top-ranked problem resource.`
      : criticalAlerts > 0
        ? `Start with ${pluralize(criticalAlerts, 'critical alert')} before reviewing lower-severity work.`
        : activeAlerts > 0
          ? `There ${activeAlerts === 1 ? 'is' : 'are'} ${pluralize(warningAlerts || activeAlerts, 'warning alert')} to review.`
          : input.pendingApprovalCount > 0
            ? `There ${input.pendingApprovalCount === 1 ? 'is' : 'are'} ${pluralize(
                input.pendingApprovalCount,
                'Assistant approval',
              )} waiting for a decision.`
            : input.patrolFindingCount > 0
              ? `Pulse Patrol has ${pluralize(input.patrolFindingCount, 'finding')} waiting for review.`
              : 'There are no pending approvals, active alerts, or Patrol findings waiting in the dashboard.';

  const supporting =
    input.overview.storage.criticalCount > 0 || input.overview.storage.warningCount > 0
      ? `Storage is at ${storagePercent}% capacity with storage health signals present.`
      : recoveryFailures > 0
        ? `Recovery has ${pluralize(recoveryFailures, 'failed outcome')} in the latest rollup.`
        : input.recovery.hasData
          ? `Recovery is tracking ${pluralize(input.recovery.totalProtected, 'protected item')}.`
          : 'Recovery has no protected-item rollup yet.';

  return `${opening} ${review} ${supporting}`;
}

function buildEvidence(input: DashboardPulseBriefInput, attentionParts: string[]): string[] {
  const evidence = [
    pluralize(input.estate.totalSystems, 'system'),
    pluralize(input.overview.workloads.total, 'workload'),
  ];

  if (attentionParts.length > 0) {
    evidence.push(...attentionParts.slice(0, 3));
  } else {
    evidence.push('No active dashboard issues');
  }

  if (input.recovery.hasData) {
    evidence.push(pluralize(input.recovery.totalProtected, 'protected item'));
  }

  return evidence.slice(0, 5);
}

function buildAssistantPrompt(input: DashboardPulseBriefInput, body: string): string {
  const topProblems = input.overview.problemResources
    .slice(0, 3)
    .map(problemResourceLabel)
    .filter((label): label is string => label !== null);

  return [
    'Summarize the current Pulse dashboard for an operator. Use only these dashboard facts unless you need to ask for more context, and do not run commands or change anything unless the operator explicitly asks for a follow-up action.',
    '',
    `Current brief: ${body}`,
    `Systems: ${input.estate.totalSystems} total, ${input.estate.healthySystems} healthy, ${input.estate.attentionSystems} needing attention.`,
    `Alerts: ${input.overview.alerts.total} active, ${input.overview.alerts.activeCritical} critical, ${input.overview.alerts.activeWarning} warning.`,
    `Storage: ${Math.round(input.storageCapacityPercent)}% capacity, ${input.overview.storage.criticalCount} critical signals, ${input.overview.storage.warningCount} warnings.`,
    `Recovery: ${input.recovery.hasData ? `${input.recovery.totalProtected} protected items` : 'no rollup available'}.`,
    topProblems.length > 0
      ? `Top problem resources: ${topProblems.join('; ')}.`
      : 'Top problem resources: none.',
  ].join('\n');
}

export function buildDashboardPulseBrief(input: DashboardPulseBriefInput): DashboardPulseBrief {
  const attentionParts = buildAttentionParts(input);
  const body = buildBody(input, attentionParts);
  const hasCriticalSignals =
    input.estate.offlineSystems > 0 ||
    input.overview.alerts.activeCritical > 0 ||
    input.overview.storage.criticalCount > 0 ||
    input.overview.problemResources.some((problem) => problem.worstValue >= 200);
  const tone: DashboardPulseBriefTone =
    hasCriticalSignals || input.pendingApprovalCount > 0
      ? 'critical'
      : attentionParts.length > 0
        ? 'attention'
        : 'healthy';

  return {
    tone,
    title: tone === 'healthy' ? 'Estate looks steady' : 'Review recommended',
    body,
    evidence: buildEvidence(input, attentionParts),
    assistantPrompt: buildAssistantPrompt(input, body),
    assistantContext: {
      dashboardBrief: body,
      estate: {
        totalSystems: input.estate.totalSystems,
        healthySystems: input.estate.healthySystems,
        attentionSystems: input.estate.attentionSystems,
      },
      alerts: input.overview.alerts,
      storage: {
        capacityPercent: Math.round(input.storageCapacityPercent),
        warningCount: input.overview.storage.warningCount,
        criticalCount: input.overview.storage.criticalCount,
      },
      recovery: input.recovery,
      problemResources: input.overview.problemResources.slice(0, 3).map((problem) => ({
        id: problem.resource.id,
        name: getPreferredResourceDisplayName(problem.resource),
        problems: problem.problems,
      })),
      patrol: {
        findingCount: input.patrolFindingCount,
        pendingApprovalCount: input.pendingApprovalCount,
      },
    },
  };
}
