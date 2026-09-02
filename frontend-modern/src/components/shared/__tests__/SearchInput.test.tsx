import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { SearchInput } from '@/components/shared/SearchInput';
import searchInputSource from '@/components/shared/SearchInput.tsx?raw';
import searchInputModelSource from '@/components/shared/searchInputModel.ts?raw';
import searchInputEnhancementsSource from '@/components/shared/SearchInputEnhancements.tsx?raw';
import searchInputEnhancementsModelSource from '@/components/shared/searchInputEnhancementsModel.ts?raw';
import searchInputEnhancementsStateSource from '@/components/shared/useSearchInputEnhancements.ts?raw';
import searchInputStateSource from '@/components/shared/useSearchInputState.ts?raw';

const SearchHarness = (props: {
  typeToSearch?: boolean;
  clearOnEscape?: boolean;
  focusOnShortcut?: boolean;
  captureBackspace?: boolean;
  onBeforeAutoFocus?: () => boolean;
}) => {
  const [value, setValue] = createSignal('');

  return (
    <div>
      <button type="button">Outside</button>
      <SearchInput
        value={value}
        onChange={setValue}
        typeToSearch={props.typeToSearch}
        clearOnEscape={props.clearOnEscape}
        focusOnShortcut={props.focusOnShortcut}
        captureBackspace={props.captureBackspace}
        onBeforeAutoFocus={props.onBeforeAutoFocus}
      />
    </div>
  );
};

describe('SearchInput', () => {
  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });

  it('keeps search input on shell, runtime, and model owners', () => {
    expect(searchInputSource).toContain('useSearchInputState');
    expect(searchInputSource).not.toContain('let searchInputEl: HTMLInputElement');
    expect(searchInputSource).not.toContain('useTypeToSearch');
    expect(searchInputSource).not.toContain('useSearchInputEnhancements');
    expect(searchInputSource).not.toContain(
      'enhancements.isSimple() ? props.shortcutHint : undefined',
    );

    expect(searchInputStateSource).toContain('export function useSearchInputState');
    expect(searchInputStateSource).toContain('let searchInputEl: HTMLInputElement');
    expect(searchInputStateSource).toContain('useTypeToSearch');
    expect(searchInputStateSource).toContain('useSearchInputEnhancements');
    expect(searchInputStateSource).toContain('getSearchInputShortcutHint');
    expect(searchInputStateSource).toContain('shouldSearchInputShowTrailingControls');

    expect(searchInputModelSource).toContain('getSearchInputShortcutHint');
    expect(searchInputModelSource).toContain('shouldSearchInputShowTrailingControls');
    expect(searchInputModelSource).toContain('export interface SearchInputProps');

    expect(searchInputEnhancementsSource).toContain('getSearchHistoryToggleButtonClass');
    expect(searchInputEnhancementsSource).toContain('getSearchHistoryToggleTitle');
    expect(searchInputEnhancementsSource).toContain('SEARCH_HISTORY_CLEAR_LABEL');
    expect(searchInputEnhancementsSource).not.toContain('Show recent searches');
    expect(searchInputEnhancementsSource).not.toContain('No recent searches yet');
    expect(searchInputEnhancementsSource).not.toContain('Clear history');
    expect(searchInputEnhancementsSource).not.toContain('hover:bg-blue-50');

    expect(searchInputStateSource).toContain('useSearchInputEnhancements');

    expect(searchInputEnhancementsStateSource).toContain('createSearchHistoryManager');
    expect(searchInputEnhancementsStateSource).toContain(
      "options.history?.emptyMessage ?? 'Searches you run will appear here.'",
    );
    expect(searchInputEnhancementsStateSource).not.toContain('Show recent searches');

    expect(searchInputEnhancementsModelSource).toContain('getSearchHistoryToggleButtonClass');
    expect(searchInputEnhancementsModelSource).toContain('getSearchHistoryToggleTitle');
    expect(searchInputEnhancementsModelSource).toContain('SEARCH_HISTORY_CLEAR_LABEL');
    expect(searchInputEnhancementsModelSource).toContain('SEARCH_HISTORY_MENU_CLASS');
    expect(searchInputEnhancementsModelSource).toContain('w-full max-w-lg');
    expect(searchInputEnhancementsModelSource).not.toContain('left-0 right-0 top-full');
    expect(searchInputEnhancementsModelSource).toContain('h-11 w-11');
    expect(searchInputEnhancementsModelSource).toContain('sm:h-7 sm:w-7');
  });

  it('caps the history menu on wide search surfaces without narrowing mobile layouts', () => {
    const HistoryHarness = () => {
      const [value, setValue] = createSignal('');

      return (
        <SearchInput
          value={value}
          onChange={setValue}
          history={{ storageKey: 'pulse:test:search-history-width' }}
        />
      );
    };

    render(() => <HistoryHarness />);

    fireEvent.click(screen.getByRole('button', { name: 'Show search history' }));

    const historyMenu = screen.getByRole('menu', { name: 'Recent searches' });
    expect(historyMenu).toHaveClass('w-full', 'max-w-lg');
    expect(historyMenu).not.toHaveClass('right-0');
  });

  it('exposes recent-search actions as a keyboard-operated menu', async () => {
    const storageKey = 'pulse:test:search-history-accessibility';
    window.localStorage.setItem(storageKey, JSON.stringify(['alpha', 'beta']));

    const HistoryHarness = () => {
      const [value, setValue] = createSignal('');
      return <SearchInput value={value} onChange={setValue} history={{ storageKey }} />;
    };

    render(() => <HistoryHarness />);

    const toggle = screen.getByRole('button', { name: 'Show search history' });
    expect(toggle).toHaveAttribute('aria-haspopup', 'menu');

    fireEvent.keyDown(toggle, { key: 'ArrowDown' });

    const menu = screen.getByRole('menu', { name: 'Recent searches' });
    expect(toggle).toHaveAttribute('aria-controls', menu.id);
    const items = within(menu).getAllByRole('menuitem');
    expect(items.map((item) => item.textContent?.trim())).toEqual([
      'alpha',
      '',
      'beta',
      '',
      'Clear history',
    ]);
    expect(within(menu).getByRole('menuitem', { name: 'Remove alpha from history' })).toBe(
      items[1],
    );

    await waitFor(() => expect(items[0]).toHaveFocus());
    fireEvent.keyDown(items[0], { key: 'ArrowDown' });
    expect(items[1]).toHaveFocus();
    fireEvent.keyDown(items[1], { key: 'End' });
    expect(items[4]).toHaveFocus();
    fireEvent.keyDown(items[4], { key: 'Escape' });
    expect(screen.queryByRole('menu', { name: 'Recent searches' })).not.toBeInTheDocument();
    await waitFor(() => expect(toggle).toHaveFocus());
  });

  it('keeps menu focus stable when a recent search is removed', async () => {
    const storageKey = 'pulse:test:search-history-delete-focus';
    window.localStorage.setItem(storageKey, JSON.stringify(['alpha', 'beta']));

    const HistoryHarness = () => {
      const [value, setValue] = createSignal('');
      return <SearchInput value={value} onChange={setValue} history={{ storageKey }} />;
    };

    render(() => <HistoryHarness />);
    fireEvent.click(screen.getByRole('button', { name: 'Show search history' }));

    const menu = screen.getByRole('menu', { name: 'Recent searches' });
    const removeAlpha = within(menu).getByRole('menuitem', {
      name: 'Remove alpha from history',
    });
    removeAlpha.focus();
    fireEvent.click(removeAlpha);

    expect(within(menu).queryByText('alpha')).not.toBeInTheDocument();
    await waitFor(() =>
      expect(
        within(menu).getByRole('menuitem', { name: 'Remove beta from history' }),
      ).toHaveFocus(),
    );
  });

  it('captures typed characters by default when focus is outside the input', async () => {
    render(() => <SearchHarness />);

    const outside = screen.getByRole('button', { name: 'Outside' });
    const input = screen.getByPlaceholderText('Search...');
    outside.focus();

    fireEvent.keyDown(document, { key: 'a' });

    await waitFor(() => {
      expect(input).toHaveValue('a');
      expect(document.activeElement).toBe(input);
    });
  });

  it('does not capture typed characters when disabled', async () => {
    render(() => <SearchHarness typeToSearch={false} />);

    const outside = screen.getByRole('button', { name: 'Outside' });
    const input = screen.getByPlaceholderText('Search...');
    outside.focus();

    fireEvent.keyDown(document, { key: 'a' });

    await waitFor(() => {
      expect(input).toHaveValue('');
      expect(document.activeElement).toBe(outside);
    });
  });

  it('respects onBeforeAutoFocus guards', async () => {
    render(() => <SearchHarness onBeforeAutoFocus={() => true} />);

    const outside = screen.getByRole('button', { name: 'Outside' });
    const input = screen.getByPlaceholderText('Search...');
    outside.focus();

    fireEvent.keyDown(document, { key: 'a' });

    await waitFor(() => {
      expect(input).toHaveValue('');
      expect(document.activeElement).toBe(outside);
    });
  });

  it('clears the query on Escape when enabled and focus is outside the input', async () => {
    render(() => <SearchHarness clearOnEscape />);

    const outside = screen.getByRole('button', { name: 'Outside' });
    const input = screen.getByPlaceholderText('Search...');

    fireEvent.input(input, { target: { value: 'alpha' } });
    outside.focus();
    fireEvent.keyDown(document, { key: 'Escape' });

    await waitFor(() => {
      expect(input).toHaveValue('');
      expect(document.activeElement).toBe(outside);
    });
  });

  it('can keep focused Escape from clearing when requested', async () => {
    const FocusedEscapeHarness = () => {
      const [value, setValue] = createSignal('alpha');

      return (
        <SearchInput
          value={value}
          onChange={setValue}
          clearOnFocusedEscape={false}
          placeholder="Focused escape"
        />
      );
    };

    render(() => <FocusedEscapeHarness />);

    const input = screen.getByPlaceholderText('Focused escape');
    input.focus();
    fireEvent.keyDown(input, { key: 'Escape' });

    await waitFor(() => {
      expect(input).toHaveValue('alpha');
      expect(document.activeElement).toBe(input);
    });
  });

  it('focuses the input on Ctrl/Cmd+F when enabled', async () => {
    render(() => <SearchHarness focusOnShortcut />);

    const outside = screen.getByRole('button', { name: 'Outside' });
    const input = screen.getByPlaceholderText('Search...');
    outside.focus();

    fireEvent.keyDown(document, { key: 'f', ctrlKey: true });

    await waitFor(() => {
      expect(document.activeElement).toBe(input);
    });
  });

  it('captures Backspace through the shared handler when enabled', async () => {
    render(() => <SearchHarness captureBackspace />);

    const outside = screen.getByRole('button', { name: 'Outside' });
    const input = screen.getByPlaceholderText('Search...');

    fireEvent.input(input, { target: { value: 'alpha' } });
    outside.focus();
    fireEvent.keyDown(document, { key: 'Backspace' });

    await waitFor(() => {
      expect(input).toHaveValue('alph');
      expect(document.activeElement).toBe(input);
    });
  });

  it('routes type-to-search to the most recently mounted visible search input', async () => {
    const SearchPair = () => {
      const [first, setFirst] = createSignal('');
      const [second, setSecond] = createSignal('');

      return (
        <div>
          <button type="button">Outside</button>
          <SearchInput value={first} onChange={setFirst} placeholder="First search" />
          <SearchInput value={second} onChange={setSecond} placeholder="Second search" />
        </div>
      );
    };

    render(() => <SearchPair />);

    const outside = screen.getByRole('button', { name: 'Outside' });
    const first = screen.getByPlaceholderText('First search');
    const second = screen.getByPlaceholderText('Second search');
    outside.focus();

    fireEvent.keyDown(document, { key: 'z' });

    await waitFor(() => {
      expect(first).toHaveValue('');
      expect(second).toHaveValue('z');
      expect(document.activeElement).toBe(second);
    });
  });

  it('offers one inline completion and applies an exact structured match on Enter', async () => {
    const applyNode = vi.fn();
    const InlineCompletionHarness = () => {
      const [value, setValue] = createSignal('');
      return (
        <SearchInput
          value={value}
          onChange={setValue}
          placeholder="Infrastructure search"
          suggestions={{
            items: () => [
              {
                id: 'filter:node:pve1',
                label: 'Node: pve1',
                completion: 'pve1',
                keywords: ['node', 'pve1'],
                onSelect: applyNode,
              },
            ],
          }}
        />
      );
    };

    const { container } = render(() => <InlineCompletionHarness />);
    const input = screen.getByPlaceholderText('Infrastructure search');
    input.focus();
    fireEvent.input(input, { target: { value: 'pv' } });

    expect(container.querySelector('[data-search-completion-suffix]')).toHaveTextContent('e1');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();

    fireEvent.keyDown(input, { key: 'Tab' });
    expect(input).toHaveValue('pve1');
    expect(applyNode).not.toHaveBeenCalled();

    fireEvent.keyDown(input, { key: 'Enter' });
    expect(applyNode).toHaveBeenCalledTimes(1);
    expect(input).toHaveValue('');
  });

  it('accepts inline completion with Right Arrow but preserves unmatched free text', () => {
    const InlineCompletionHarness = () => {
      const [value, setValue] = createSignal('');
      return (
        <SearchInput
          value={value}
          onChange={setValue}
          placeholder="Mixed search"
          suggestions={{
            items: () => [{ id: 'resource:alpha', label: 'alpha-host', value: 'alpha-host' }],
          }}
        />
      );
    };

    render(() => <InlineCompletionHarness />);
    const input = screen.getByPlaceholderText('Mixed search') as HTMLInputElement;
    input.focus();
    fireEvent.input(input, { target: { value: 'alp' } });
    input.setSelectionRange(3, 3);
    fireEvent.keyDown(input, { key: 'ArrowRight' });
    expect(input).toHaveValue('alpha-host');

    fireEvent.input(input, { target: { value: 'arbitrary phrase' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(input).toHaveValue('arbitrary phrase');
  });
});
