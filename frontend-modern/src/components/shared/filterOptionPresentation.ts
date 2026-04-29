const collapseWhitespace = (value: string): string => value.trim().replace(/\s+/g, ' ');

export const FILTER_OPTION_ALL_LABEL = 'All';

export function getAllFilterOptionLabel(scopeLabel: string): string {
  const normalizedScope = collapseWhitespace(scopeLabel);
  return normalizedScope
    ? `${FILTER_OPTION_ALL_LABEL} ${normalizedScope}`
    : FILTER_OPTION_ALL_LABEL;
}
