import type { PlatformType, SourceType, ResourceType } from '@/types/resource';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import { normalizeSourcePlatformKey, type KnownSourcePlatform } from '@/utils/sourcePlatforms';
import { getSourceTypePresentation } from '@/utils/sourceTypePresentation';
import {
  canonicalResourceTypeForDisplay,
  getResourceTypePresentation,
} from '@/utils/resourceTypePresentation';

export interface ResourceBadge {
  label: string;
  classes: string;
  title?: string;
}

const baseBadge =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap';

const typeClasses = 'bg-surface-alt text-base-content';

export function getPlatformBadge(platformType?: PlatformType): ResourceBadge | null {
  if (!platformType) return null;
  const sharedBadge = getSourcePlatformBadge(platformType);
  if (!sharedBadge) return null;
  return {
    label: sharedBadge.label,
    classes: sharedBadge.classes,
    title: sharedBadge.title,
  };
}

export function getSourceBadge(sourceType?: SourceType): ResourceBadge | null {
  if (!sourceType) return null;
  const presentation = getSourceTypePresentation(sourceType);
  return {
    label: presentation?.label ?? sourceType,
    classes: `${baseBadge} ${presentation?.badgeClasses ?? typeClasses}`,
    title: sourceType,
  };
}

export function getTypeBadge(resourceType?: ResourceType | string): ResourceBadge | null {
  if (!resourceType) return null;
  const normalizedType = canonicalResourceTypeForDisplay(resourceType);
  const presentation = getResourceTypePresentation(resourceType);
  if (!presentation) return null;
  return {
    label: presentation.label,
    classes: `${baseBadge} ${presentation.badgeClasses || typeClasses}`,
    title: normalizedType,
  };
}

export function getUnifiedSourceBadges(sources?: string[] | null): ResourceBadge[] {
  if (!sources || sources.length === 0) return [];
  const normalized = sources
    .map((source) => normalizeSourcePlatformKey(source))
    .filter((source): source is KnownSourcePlatform => Boolean(source));
  const unique = Array.from(new Set(normalized));
  return unique.map((source) => {
    const sharedBadge = getSourcePlatformBadge(source);
    return {
      label: sharedBadge?.label ?? source,
      classes: sharedBadge?.classes ?? `${baseBadge} ${typeClasses}`,
      title: sharedBadge?.title ?? source,
    };
  });
}

export function getContainerRuntimeBadge(
  platformType?: PlatformType,
  platformData?: Record<string, unknown> | null,
): ResourceBadge | null {
  if (platformType !== 'docker' || !platformData) return null;

  const docker = (platformData as { docker?: { runtime?: string } } | undefined)?.docker;
  const raw = (docker?.runtime || '').trim();
  if (!raw) return null;

  const normalized = raw.toLowerCase();
  const label = normalized === 'podman' ? 'Podman' : normalized === 'docker' ? 'Docker' : raw;

  return {
    label,
    classes: `${baseBadge} ${typeClasses}`,
    title: `Runtime: ${label}`,
  };
}
