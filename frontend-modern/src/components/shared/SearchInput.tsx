import { Component } from 'solid-js';
import { SearchField } from '@/components/shared/SearchField';
import {
  SearchInputHistoryDropdown,
  SearchInputTrailingControls,
} from '@/components/shared/SearchInputEnhancements';
import { type SearchInputProps } from './searchInputModel';
import { useSearchInputState } from './useSearchInputState';

export type { SearchInputKeyboardEvent, SearchInputProps } from './searchInputModel';

export const SearchInput: Component<SearchInputProps> = (props) => {
  const search = useSearchInputState(props);

  return (
    <div class={`relative w-full ${props.class ?? ''}`}>
      <SearchField
        value={props.value()}
        onChange={props.onChange}
        placeholder={props.placeholder}
        title={props.title}
        inputRef={search.setInputRef}
        inputClass={props.inputClass}
        disabled={props.disabled}
        clearOnFocusedEscape={props.clearOnFocusedEscape}
        shortcutHint={search.shortcutHint()}
        hasTrailingControls={search.showTrailingControls()}
        onClearMouseDown={search.enhancements.onClearMouseDown}
        onKeyDown={search.enhancements.onFieldKeyDown}
        onBlur={search.enhancements.onFieldBlur}
        trailingControls={
          <SearchInputTrailingControls state={search.enhancements} tips={props.tips} />
        }
      />
      <SearchInputHistoryDropdown state={search.enhancements} />
    </div>
  );
};
