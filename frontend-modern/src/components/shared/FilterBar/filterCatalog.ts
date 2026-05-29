import type { Accessor, JSX } from 'solid-js';
import type { FilterButtonGroupOptionTone } from '@/components/shared/filterButtonGroupModel';
import type { SearchTipsConfig } from '@/components/shared/useSearchInputEnhancements';

export interface FilterSelectOption {
  value: string;
  label: string;
  ariaLabel?: string;
  title?: string;
  compactLabel?: string;
  leading?: JSX.Element;
  visualLabel?: JSX.Element;
  icon?: (props: { class?: string }) => JSX.Element;
  tone?: FilterButtonGroupOptionTone;
  count?: number;
}

export type FilterGroupKey = 'scope' | 'status' | 'properties';

export interface FilterDef {
  id: string;
  label: string;
  group?: FilterGroupKey;
  /**
   * Primary filters with a small, fixed option set can opt into an inline
   * segmented control instead of forcing operators through the add-filter menu.
   */
  inline?: boolean;
  options: Accessor<FilterSelectOption[]>;
  value: Accessor<string>;
  setValue: (value: string) => void;
  defaultValue: string;
  formatChipValue?: (value: string, options: FilterSelectOption[]) => string;
}

export interface FilterBarSearch {
  value: Accessor<string>;
  setValue: (value: string) => void;
  placeholder: string;
  historyKey?: string;
  emptyMessage?: string;
  tips?: SearchTipsConfig;
  clearOnEscape?: boolean;
  onBeforeAutoFocus?: () => boolean;
}

export interface FilterBarProps {
  search: FilterBarSearch;
  filters: FilterDef[];
  viewOptionsTrailing?: JSX.Element;
  searchTrailing?: JSX.Element;
  isMobile: Accessor<boolean>;
  role?: string;
  ariaLabel?: string;
  onClearAll?: () => void;
  showClearAll?: Accessor<boolean>;
  /**
   * When set, render a "Saved views" menu next to the Filter selector that persists
   * named filter combinations (URL query strings) to localStorage under
   * `pulse:filterbar:saved-views:<savedViewsKey>`. Pages opt in per surface
   * because the saved-view storage scope follows the page's URL state model.
   */
  savedViewsKey?: string;
}

export const isFilterSet = (filter: FilterDef): boolean => filter.value() !== filter.defaultValue;

export const clearFilter = (filter: FilterDef): void => {
  filter.setValue(filter.defaultValue);
};

export const formatFilterChipValue = (filter: FilterDef): string => {
  const value = filter.value();
  const options = filter.options();
  if (filter.formatChipValue) return filter.formatChipValue(value, options);
  const match = options.find((option) => option.value === value);
  return match?.label ?? value;
};
