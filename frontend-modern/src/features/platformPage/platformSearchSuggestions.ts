import type { SearchInputSuggestion } from '@/components/shared/SearchInput';
import type { Resource } from '@/types/resource';
import { getCanonicalStatusLabel } from '@/utils/status';
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';

const clean = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

export interface PlatformSearchSuggestionCandidate {
  id: string;
  label: string;
  value?: string;
  description?: string;
  keywords?: readonly string[];
  completions?: readonly string[];
}

export const buildPlatformSearchSuggestions = (
  candidates: readonly PlatformSearchSuggestionCandidate[],
  idNamespace = 'infrastructure',
): SearchInputSuggestion[] => {
  const labelCounts = new Map<string, number>();
  for (const candidate of candidates) {
    const label = clean(candidate.label);
    if (!label) continue;
    const key = label.toLowerCase();
    labelCounts.set(key, (labelCounts.get(key) ?? 0) + 1);
  }

  const seen = new Set<string>();
  const suggestions: SearchInputSuggestion[] = [];
  for (const candidate of candidates) {
    const id = clean(candidate.id);
    const label = clean(candidate.label);
    if (!id || !label || seen.has(id)) continue;
    seen.add(id);
    const duplicateLabel = (labelCounts.get(label.toLowerCase()) ?? 0) > 1;
    suggestions.push({
      id: `${idNamespace}:${id}`,
      label,
      value: clean(candidate.value) || (duplicateLabel ? id : label),
      description: clean(candidate.description) || undefined,
      group: 'Infrastructure',
      keywords: [id, ...(candidate.keywords ?? [])].map(clean).filter(Boolean),
      completions: candidate.completions?.map(clean).filter(Boolean),
    });
  }

  return suggestions.sort((left, right) => left.label.localeCompare(right.label));
};

const resourceDisplayName = (resource: Resource): string =>
  clean(resource.canonicalIdentity?.displayName) ||
  clean(resource.displayName) ||
  clean(resource.name) ||
  resource.id;

const resourceSearchKeywords = (resource: Resource): string[] =>
  [
    resource.id,
    resource.name,
    resource.displayName,
    resource.type,
    resource.technology,
    resource.status,
    resource.parentName,
    resource.platformId,
    resource.platformType,
    resource.clusterId,
    resource.agent?.hostname,
    resource.identity?.hostname,
    resource.identity?.clusterName,
    resource.canonicalIdentity?.displayName,
    resource.canonicalIdentity?.hostname,
    resource.canonicalIdentity?.platformId,
    resource.canonicalIdentity?.primaryId,
    ...(resource.canonicalIdentity?.aliases ?? []),
    ...(resource.tags ?? []),
  ].filter((value): value is string => Boolean(clean(value)));

const resourceSuggestionDescription = (resource: Resource): string => {
  const type = getResourceTypeLabel(resource.type) ?? 'Resource';
  const parent = clean(resource.parentName) || clean(resource.identity?.clusterName);
  const status = getCanonicalStatusLabel(resource.status);
  return [type, parent, status].filter(Boolean).join(' · ');
};

export const isPlatformSearchSuggestionResource = (value: unknown): value is Resource => {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Partial<Resource>;
  return (
    typeof candidate.id === 'string' &&
    typeof candidate.type === 'string' &&
    typeof candidate.name === 'string'
  );
};

export const buildPlatformResourceSearchSuggestions = (
  resources: readonly Resource[],
): SearchInputSuggestion[] =>
  buildPlatformSearchSuggestions(
    resources.map((resource) => ({
      id: resource.id,
      label: resourceDisplayName(resource),
      description: resourceSuggestionDescription(resource),
      keywords: resourceSearchKeywords(resource),
      completions: [
        resource.name,
        resource.displayName,
        resource.id,
        resource.agent?.hostname,
        resource.identity?.hostname,
        resource.canonicalIdentity?.displayName,
        resource.canonicalIdentity?.hostname,
        resource.canonicalIdentity?.primaryId,
        ...(resource.canonicalIdentity?.aliases ?? []),
      ].filter((value): value is string => Boolean(clean(value))),
    })),
    'resource',
  );
