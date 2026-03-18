import { getSourcePlatformLabel, getSourcePlatformPresentation } from '@/utils/sourcePlatforms';

const BASE_BADGE =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap';

export interface SourcePlatformBadge {
  label: string;
  title: string;
  classes: string;
}

const DEFAULT_TONE = 'bg-surface-alt text-base-content';

export const getSourcePlatformBadge = (
  value: string | null | undefined,
): SourcePlatformBadge | null => {
  const presentation = getSourcePlatformPresentation(value);
  if (!presentation) {
    const raw = (value || '').toString().trim();
    if (!raw) return null;
    const label = getSourcePlatformLabel(raw);
    if (!label) return null;
    return {
      label,
      title: label,
      classes: `${BASE_BADGE} ${DEFAULT_TONE}`,
    };
  }

  return {
    label: presentation.label,
    title: presentation.label,
    classes: `${BASE_BADGE} ${presentation.tone}`,
  };
};
