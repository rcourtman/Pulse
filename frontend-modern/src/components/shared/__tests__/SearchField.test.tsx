import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { SearchField } from '@/components/shared/SearchField';

describe('SearchField', () => {
  afterEach(() => {
    cleanup();
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
});
