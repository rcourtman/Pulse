import type { Accessor, JSX } from 'solid-js';

export interface FilterSelectOption {
  value: string;
  label: string;
  count?: number;
}

export type FilterGroupKey = 'scope' | 'status' | 'properties';

export interface FilterDef {
  id: string;
  label: string;
  group?: FilterGroupKey;
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
   * When set, render a "Saved views" menu next to "+ Filter" that persists
   * named filter combinations (URL query strings) to localStorage under
   * `pulse:filterbar:saved-views:<savedViewsKey>`. Pages opt in per surface
   * because the saved-view storage scope follows the page's URL state model.
   */
  savedViewsKey?: string;
}

export const isFilterSet = (filter: FilterDef): boolean =>
  filter.value() !== filter.defaultValue;

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
