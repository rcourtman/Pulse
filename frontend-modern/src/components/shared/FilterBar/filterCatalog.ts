import type { Accessor, JSX } from 'solid-js';
import type { FilterButtonGroupOptionTone } from '@/components/shared/filterButtonGroupModel';
import type { SearchTipsConfig } from '@/components/shared/useSearchInputEnhancements';
import type { SearchInputSuggestion } from '@/components/shared/searchInputModel';

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
  suggestions?: Accessor<readonly SearchInputSuggestion[]>;
  clearOnEscape?: boolean;
  onBeforeAutoFocus?: () => boolean;
}

export interface FilterBarProps {
  search: FilterBarSearch;
  filters: FilterDef[];
  /** Contextual, frequently used actions rendered immediately before View. */
  leadingControls?: JSX.Element;
  /** Low-frequency presentation controls; FilterBar owns the shared View popover. */
  viewOptions?: JSX.Element;
  /** Persistent orientation/status controls rendered immediately after View. */
  trailingControls?: JSX.Element;
  searchTrailing?: JSX.Element;
  isMobile: Accessor<boolean>;
  role?: string;
  ariaLabel?: string;
  onClearAll?: () => void;
  showClearAll?: Accessor<boolean>;
  /** Opt into a visible "Filter" label; the compact accessible-only label is the default. */
  showAddFilterLabel?: boolean;
}

export interface FilterSearchTerm {
  id: string;
  label: string;
  value: string;
}

export const getSearchSuggestionValues = (suggestion: SearchInputSuggestion): string[] =>
  [
    suggestion.value,
    suggestion.completion,
    suggestion.label,
    ...(suggestion.completions ?? []),
  ].filter((value): value is string => Boolean(value?.trim()));

export const toFilterSearchTerm = (suggestion: SearchInputSuggestion): FilterSearchTerm => ({
  id: suggestion.id,
  label: suggestion.label,
  value: (suggestion.value ?? suggestion.completion ?? suggestion.label).trim(),
});

export const serializeFilterSearch = (terms: readonly FilterSearchTerm[], draft: string): string =>
  [...terms.map((term) => term.value), draft.trim()].filter(Boolean).join(', ');

export const isFilterSet = (filter: FilterDef): boolean => filter.value() !== filter.defaultValue;

export const hasSelectableFilterOptions = (filter: FilterDef): boolean =>
  !isFilterSet(filter) && filter.options().some((option) => option.value !== filter.defaultValue);

export const hasAddableFilterOptions = (filters: FilterDef[]): boolean =>
  filters.some(hasSelectableFilterOptions);

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

export const buildFilterSearchSuggestions = (
  filters: readonly FilterDef[],
): SearchInputSuggestion[] =>
  filters.flatMap((filter) =>
    filter
      .options()
      .filter((option) => option.value !== filter.defaultValue && option.value !== filter.value())
      .map((option) => ({
        id: `filter:${filter.id}:${option.value}`,
        label: `${filter.label}: ${option.label}`,
        completion: option.label,
        description: `Apply ${filter.label.toLowerCase()} filter`,
        group: 'Filters',
        keywords: [filter.label, option.label, option.value],
        onSelect: () => filter.setValue(option.value),
      })),
  );
