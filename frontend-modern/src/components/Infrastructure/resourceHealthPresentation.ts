import type { Resource, ResourceStorageRisk } from '@/types/resource';

export interface ResourceHealthIssuePresentation {
  primary: string;
  compactLabel: string;
  details: string[];
  title: string;
}

type AgentStorageLike = NonNullable<Resource['agent']> & {
  unraid?: {
    risk?: ResourceStorageRisk;
    riskSummary?: string;
    postureSummary?: string;
    protectionSummary?: string;
    rebuildSummary?: string;
  };
};

const ATTENTION_STATUSES = new Set([
  'degraded',
  'warning',
  'critical',
  'faulted',
  'failed',
  'error',
  'unhealthy',
  'offline',
  'down',
  'unavailable',
]);

const trimSummary = (value: string | undefined | null): string => (value || '').trim();

const pushUnique = (items: string[], value: string | undefined | null) => {
  const trimmed = trimSummary(value);
  if (!trimmed) return;
  if (!items.some((item) => item.toLowerCase() === trimmed.toLowerCase())) {
    items.push(trimmed);
  }
};

const pushRiskReasons = (items: string[], risk: ResourceStorageRisk | undefined) => {
  for (const reason of risk?.reasons ?? []) {
    pushUnique(items, reason.summary);
  }
};

const compactHealthLabel = (summary: string): string => {
  const normalized = summary.toLowerCase();
  if (normalized.includes('without parity protection')) return 'No parity';
  if (normalized.includes('parity protection') && normalized.includes('unavailable')) {
    return 'Parity unavailable';
  }
  if (normalized.includes('parity') && normalized.includes('missing')) return 'Parity missing';
  if (normalized.includes('array is running check')) return 'Array check running';
  if (summary.length <= 34) return summary;
  return `${summary.slice(0, 31).trimEnd()}...`;
};

export const getResourceHealthIssuePresentation = (
  resource: Resource,
): ResourceHealthIssuePresentation | null => {
  const status = (resource.status || '').trim().toLowerCase();
  const summaries: string[] = [];

  pushUnique(summaries, resource.incidentSummary);
  pushUnique(summaries, resource.incidentLabel);

  pushUnique(summaries, resource.storage?.postureSummary);
  pushUnique(summaries, resource.storage?.riskSummary);
  pushUnique(summaries, resource.storage?.protectionSummary);
  pushUnique(summaries, resource.storage?.rebuildSummary);
  pushRiskReasons(summaries, resource.storage?.risk);

  const agent = resource.agent as AgentStorageLike | undefined;
  pushUnique(summaries, agent?.storagePostureSummary);
  pushUnique(summaries, agent?.storageRiskSummary);
  pushUnique(summaries, agent?.protectionSummary);
  pushUnique(summaries, agent?.rebuildSummary);
  pushRiskReasons(summaries, agent?.storageRisk);
  pushUnique(summaries, agent?.unraid?.postureSummary);
  pushUnique(summaries, agent?.unraid?.riskSummary);
  pushUnique(summaries, agent?.unraid?.protectionSummary);
  pushUnique(summaries, agent?.unraid?.rebuildSummary);
  pushRiskReasons(summaries, agent?.unraid?.risk);

  if (summaries.length === 0 || !ATTENTION_STATUSES.has(status)) {
    return null;
  }

  const [primary, ...details] = summaries;
  const compactLabel = compactHealthLabel(primary);
  return {
    primary,
    compactLabel,
    details: details.filter((detail) => detail.toLowerCase() !== primary.toLowerCase()),
    title: [primary, ...details].join(' · '),
  };
};
