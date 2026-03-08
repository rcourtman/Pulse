import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { createSignal } from 'solid-js';
import { CollapsibleSearchInput } from '@/components/shared/CollapsibleSearchInput';
import { SearchInput } from '@/components/shared/SearchInput';
import { focusActiveTypeToSearch } from '@/hooks/useTypeToSearch';

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

  it('expands a collapsible search and inserts typed characters through the shared registry', async () => {
    const CollapsibleHarness = () => {
      const [value, setValue] = createSignal('');

      return (
        <div>
          <button type="button">Outside</button>
          <CollapsibleSearchInput
            value={value}
            onChange={setValue}
            placeholder="Collapsed search"
          />
        </div>
      );
    };

    render(() => <CollapsibleHarness />);

    const outside = screen.getByRole('button', { name: 'Outside' });
    outside.focus();

    fireEvent.keyDown(document, { key: 'm' });

    await waitFor(() => {
      const input = screen.getByPlaceholderText('Collapsed search');
      expect(input).toHaveValue('m');
      expect(document.activeElement).toBe(input);
    });
  });

  it('focuses the active search through the shared focus helper', async () => {
    const CollapsibleHarness = () => {
      const [value, setValue] = createSignal('');

      return (
        <div>
          <button type="button">Outside</button>
          <CollapsibleSearchInput
            value={value}
            onChange={setValue}
            placeholder="Helper search"
          />
        </div>
      );
    };

    render(() => <CollapsibleHarness />);

    expect(focusActiveTypeToSearch({ selectText: true })).toBe(true);

    await waitFor(() => {
      const input = screen.getByPlaceholderText('Helper search');
      expect(document.activeElement).toBe(input);
    });
  });
});
