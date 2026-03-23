import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import searchFieldSource from '@/components/shared/SearchField.tsx?raw';
import searchFieldModelSource from '@/components/shared/searchFieldModel.ts?raw';
import searchFieldStateSource from '@/components/shared/useSearchFieldState.ts?raw';
import { SearchField } from '@/components/shared/SearchField';

describe('SearchField', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps search field on shell, runtime, and model owners', () => {
    expect(searchFieldSource).toContain('useSearchFieldState');
    expect(searchFieldSource).not.toContain('let inputEl: HTMLInputElement');
    expect(searchFieldSource).not.toContain("if (props.hasTrailingControls) return 'pr-14 sm:pr-20'");
    expect(searchFieldSource).not.toContain("if (e.key === 'Escape'");

    expect(searchFieldStateSource).toContain('export function useSearchFieldState');
    expect(searchFieldStateSource).toContain('let inputEl: HTMLInputElement');
    expect(searchFieldStateSource).toContain("if (event.key === 'Escape'");
    expect(searchFieldStateSource).toContain('inputEl?.blur()');
    expect(searchFieldStateSource).toContain('getSearchFieldInputPaddingRightClass');

    expect(searchFieldModelSource).toContain('shouldShowSearchFieldShortcutHint');
    expect(searchFieldModelSource).toContain('shouldShowSearchFieldClearButton');
    expect(searchFieldModelSource).toContain('getSearchFieldInputPaddingRightClass');
    expect(searchFieldModelSource).toContain("return 'pr-14 sm:pr-20'");
  });

  it('renders the shortcut hint when empty', () => {
    render(() => (
      <SearchField value="" onChange={vi.fn()} placeholder="Search field" shortcutHint="Cmd+K" />
    ));

    expect(screen.getByText('Cmd+K')).toBeInTheDocument();
  });

  it('clears and blurs on focused Escape by default', async () => {
    const Harness = () => {
      const [value, setValue] = createSignal('alpha');
      return <SearchField value={value()} onChange={setValue} placeholder="Escape field" />;
    };

    render(() => <Harness />);

    const input = screen.getByPlaceholderText('Escape field');
    input.focus();
    await fireEvent.keyDown(input, { key: 'Escape' });

    expect(input).toHaveValue('');
    expect(document.activeElement).not.toBe(input);
  });

  it('preserves value on focused Escape when disabled', async () => {
    const Harness = () => {
      const [value, setValue] = createSignal('alpha');
      return (
        <SearchField
          value={value()}
          onChange={setValue}
          placeholder="Stable field"
          clearOnFocusedEscape={false}
        />
      );
    };

    render(() => <Harness />);

    const input = screen.getByPlaceholderText('Stable field');
    input.focus();
    await fireEvent.keyDown(input, { key: 'Escape' });

    expect(input).toHaveValue('alpha');
    expect(document.activeElement).toBe(input);
  });

  it('renders trailing controls alongside the clear button', () => {
    render(() => (
      <SearchField
        value="alpha"
        onChange={vi.fn()}
        placeholder="Trailing field"
        hasTrailingControls
        trailingControls={<button type="button">Extra</button>}
      />
    ));

    expect(screen.getByRole('button', { name: 'Clear search' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Extra' })).toBeInTheDocument();
  });

  it('invokes explicit keyboard and blur handlers with the input event target', async () => {
    const onKeyDown = vi.fn();
    const onBlur = vi.fn();

    render(() => (
      <SearchField
        value="alpha"
        onChange={vi.fn()}
        placeholder="Handler field"
        onKeyDown={onKeyDown}
        onBlur={onBlur}
      />
    ));

    const input = screen.getByPlaceholderText('Handler field');
    await fireEvent.keyDown(input, { key: 'Enter' });
    await fireEvent.blur(input);

    expect(onKeyDown).toHaveBeenCalledTimes(1);
    expect(onKeyDown.mock.calls[0][0].currentTarget).toBe(input);
    expect(onBlur).toHaveBeenCalledTimes(1);
    expect(onBlur.mock.calls[0][0].currentTarget).toBe(input);
  });
});
