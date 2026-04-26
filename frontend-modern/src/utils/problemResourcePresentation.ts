import type { Resource } from '@/types/resource';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';
import type { StatusIndicatorVariant } from '@/utils/status';

function normalizeProblemResourceValue(value?: string | null): string {
  return (value ?? '').trim().toLowerCase().replace(/[-_]+/g, ' ');
}

function getProblemResourceTypeNoun(resource: Resource): string | null {
  const typeLabel = getResourceTypeLabel(resource.type)?.trim();
  if (!typeLabel) return null;
  return typeLabel === typeLabel.toUpperCase() ? typeLabel : typeLabel.toLowerCase();
}

function getGenericProblemResourceNames(resource: Resource, problems: string[]): Set<string> {
  const typeLabel = normalizeProblemResourceValue(getResourceTypeLabel(resource.type));
  const rawType = normalizeProblemResourceValue(resource.type);
  const normalizedProblems = problems.map(normalizeProblemResourceValue).filter(Boolean);

  return new Set(
    [typeLabel, rawType]
      .filter(Boolean)
      .flatMap((value) => [
        value,
        ...normalizedProblems.map((problem) => `${value} (${problem})`),
      ]),
  );
}

export function getProblemResourceStatusVariant(worstValue: number): StatusIndicatorVariant {
  return worstValue >= 150 ? 'danger' : 'warning';
}

export function getProblemResourceDisplayName(resource: Resource): string {
  return getPreferredResourceDisplayName(resource) || resource.id;
}

export function isGenericProblemResourceDisplayName(
  resource: Resource,
  problems: string[],
  displayName = getProblemResourceDisplayName(resource),
): boolean {
  return getGenericProblemResourceNames(resource, problems).has(
    normalizeProblemResourceValue(displayName),
  );
}

export function getProblemResourceIssueLabel(resource: Resource, problems: string[]): string {
  const displayName = getProblemResourceDisplayName(resource);
  const typeNoun = getProblemResourceTypeNoun(resource);
  return typeNoun && isGenericProblemResourceDisplayName(resource, problems, displayName)
    ? `the ${typeNoun} issue`
    : displayName;
}

export function getProblemResourceMemberLabel(resource: Resource, problems: string[]): string | null {
  const displayName = getProblemResourceDisplayName(resource);
  return isGenericProblemResourceDisplayName(resource, problems, displayName) ? null : displayName;
}
