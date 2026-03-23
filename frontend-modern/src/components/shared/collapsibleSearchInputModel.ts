import type { SearchInputProps } from './SearchInput';

export interface CollapsibleSearchInputProps extends Omit<SearchInputProps, 'typeToSearch'> {
  triggerLabel?: string;
  fullWidthWhenExpanded?: boolean;
}

export const getCollapsibleSearchTriggerLabel = (triggerLabel?: string) =>
  triggerLabel ?? 'Search';

export const shouldShowCollapsibleSearchExpanded = (isExpanded: boolean, value: string) =>
  isExpanded || value.trim().length > 0;

export const getCollapsibleSearchRootClass = (options: {
  className?: string;
  fullWidthWhenExpanded?: boolean;
  showExpanded: boolean;
}) => {
  const baseClass = options.className ?? '';
  if (!options.fullWidthWhenExpanded) return baseClass;
  const layoutClass = options.showExpanded
    ? 'order-last basis-full w-full'
    : 'shrink-0 md:ml-auto';
  return `${baseClass} ${layoutClass}`.trim();
};
