import type { Accessor } from 'solid-js';
import type {
  SearchHistoryConfig,
  SearchTipsConfig,
} from '@/components/shared/useSearchInputEnhancements';

export type SearchInputKeyboardEvent = KeyboardEvent & {
  currentTarget: HTMLInputElement;
  target: Element;
};

export interface SearchInputSuggestion {
  id: string;
  label: string;
  /** Preferred text completed inline after the current query. Defaults to the label. */
  completion?: string;
  /** Alternate canonical identities that can be completed inline. */
  completions?: readonly string[];
  value?: string;
  description?: string;
  group?: string;
  keywords?: readonly string[];
  onSelect?: () => void;
}

export interface SearchInputSuggestionsConfig {
  items: Accessor<readonly SearchInputSuggestion[]>;
  minQueryLength?: number;
  maxResults?: number;
  /** Commit a non-exact query when it still resolves to known suggestions. */
  onCommitQuery?: (query: string, matches: readonly SearchInputSuggestion[]) => void;
}

export interface SearchInputProps {
  value: () => string;
  onChange: (value: string) => void;
  placeholder?: string;
  title?: string;
  history?: SearchHistoryConfig;
  tips?: SearchTipsConfig;
  suggestions?: SearchInputSuggestionsConfig;
  inputRef?: (el: HTMLInputElement) => void;
  class?: string;
  inputClass?: string;
  disabled?: boolean;
  onKeyDown?: (event: SearchInputKeyboardEvent) => void;
  typeToSearch?: boolean;
  clearOnEscape?: boolean;
  clearOnFocusedEscape?: boolean;
  focusOnShortcut?: boolean;
  captureBackspace?: boolean;
  shortcutHint?: string;
  onBeforeAutoFocus?: () => boolean;
}

export const getSearchInputShortcutHint = (isSimple: boolean, shortcutHint?: string) =>
  isSimple ? shortcutHint : undefined;

export const shouldSearchInputShowTrailingControls = (isSimple: boolean) => !isSimple;

const normalizedSearchSuggestionFields = (suggestion: SearchInputSuggestion): string[] =>
  [
    suggestion.label,
    suggestion.completion,
    ...(suggestion.completions ?? []),
    suggestion.value,
    ...(suggestion.keywords ?? []),
  ]
    .filter((value): value is string => Boolean(value?.trim()))
    .map((value) => value.trim().toLowerCase());

export const getSearchInputSuggestionCompletions = (suggestion: SearchInputSuggestion): string[] =>
  [suggestion.completion, suggestion.label, suggestion.value, ...(suggestion.completions ?? [])]
    .filter((value): value is string => Boolean(value?.trim()))
    .map((value) => value.trim());

export interface SearchInputInlineCompletion {
  suggestion: SearchInputSuggestion;
  text: string;
}

export const resolveSearchInputInlineCompletion = (
  suggestions: readonly SearchInputSuggestion[],
  query: string,
): SearchInputInlineCompletion | undefined => {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery || query !== query.trim()) return undefined;

  const candidates = suggestions.flatMap((suggestion) =>
    getSearchInputSuggestionCompletions(suggestion)
      .filter((completion) => completion.toLowerCase().startsWith(normalizedQuery))
      .map((completion) => ({ suggestion, text: completion })),
  );
  if (candidates.length === 0) return undefined;

  const uniqueCandidates = Array.from(
    new Map(candidates.map((candidate) => [candidate.text.toLowerCase(), candidate])).values(),
  );
  let commonLength = uniqueCandidates[0].text.length;
  for (let index = 1; index < uniqueCandidates.length; index += 1) {
    const left = uniqueCandidates[0].text.toLowerCase();
    const right = uniqueCandidates[index].text.toLowerCase();
    commonLength = Math.min(commonLength, right.length);
    let position = normalizedQuery.length;
    while (position < commonLength && left[position] === right[position]) position += 1;
    commonLength = position;
  }

  if (commonLength <= query.length) return undefined;
  return {
    suggestion: uniqueCandidates[0].suggestion,
    text: uniqueCandidates[0].text.slice(0, commonLength),
  };
};

const isSubsequence = (query: string, candidate: string): boolean => {
  let queryIndex = 0;
  for (const character of candidate) {
    if (character === query[queryIndex]) queryIndex += 1;
    if (queryIndex === query.length) return true;
  }
  return false;
};

export const scoreSearchInputSuggestion = (
  suggestion: SearchInputSuggestion,
  query: string,
): number | null => {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) return null;

  const fields = normalizedSearchSuggestionFields(suggestion);
  if (fields.some((field) => field === normalizedQuery)) return 0;
  if (fields.some((field) => field.startsWith(normalizedQuery))) return 1;
  if (
    fields.some((field) =>
      field.split(/[^a-z0-9]+/).some((token) => token.startsWith(normalizedQuery)),
    )
  ) {
    return 2;
  }
  if (fields.some((field) => field.includes(normalizedQuery))) return 3;
  if (
    normalizedQuery.length >= 3 &&
    fields.some((field) => isSubsequence(normalizedQuery, field))
  ) {
    return 4;
  }
  return null;
};

export const rankSearchInputSuggestions = (
  suggestions: readonly SearchInputSuggestion[],
  query: string,
  maxResults = 10,
): SearchInputSuggestion[] => {
  const groupOrder = new Map<string, number>();
  for (const suggestion of suggestions) {
    const group = suggestion.group ?? '';
    if (!groupOrder.has(group)) groupOrder.set(group, groupOrder.size);
  }

  return suggestions
    .map((suggestion, sourceIndex) => ({
      suggestion,
      sourceIndex,
      score: scoreSearchInputSuggestion(suggestion, query),
    }))
    .filter(
      (
        candidate,
      ): candidate is {
        suggestion: SearchInputSuggestion;
        sourceIndex: number;
        score: number;
      } => candidate.score !== null,
    )
    .sort((left, right) => {
      const leftGroup = groupOrder.get(left.suggestion.group ?? '') ?? 0;
      const rightGroup = groupOrder.get(right.suggestion.group ?? '') ?? 0;
      return (
        leftGroup - rightGroup || left.score - right.score || left.sourceIndex - right.sourceIndex
      );
    })
    .slice(0, Math.max(1, maxResults))
    .map((candidate) => candidate.suggestion);
};
