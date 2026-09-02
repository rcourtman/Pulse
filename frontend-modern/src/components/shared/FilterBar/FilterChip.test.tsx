import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { FilterChip, shouldPlaceFilterChipPopoverAbove } from './FilterChip';
import type { FilterDef } from './filterCatalog';

afterEach(cleanup);

const Harness = () => {
  const [value, setValue] = createSignal('pve1');
  const filter: FilterDef = {
    id: 'node',
    label: 'Node',
    value,
    setValue,
    defaultValue: '',
    options: () => [
      { value: '', label: 'All nodes' },
      { value: 'pve1', label: 'pve1' },
      { value: 'pve2', label: 'pve2', ariaLabel: 'Node pve2' },
    ],
  };

  return <FilterChip filter={filter} />;
};

describe('FilterChip', () => {
  it('connects the searchable combobox to its active listbox option', async () => {
    render(() => <Harness />);

    const trigger = screen.getByRole('button', { name: 'Node: pve1' });
    fireEvent.click(trigger);
    await Promise.resolve();

    const combobox = screen.getByRole('combobox', { name: 'Filter Node values' });
    const listbox = screen.getByRole('listbox', { name: 'Node' });
    expect(combobox).toHaveFocus();
    expect(listbox.parentElement).toHaveClass('top-[calc(100%+0.25rem)]');
    expect(combobox).toHaveAttribute('aria-controls', listbox.id);
    expect(combobox).toHaveAttribute(
      'aria-activedescendant',
      within(listbox).getByRole('option', { name: 'pve1' }).id,
    );
    const nextOption = within(listbox).getByRole('option', { name: 'Node pve2' });
    const scrollIntoView = vi.fn();
    nextOption.scrollIntoView = scrollIntoView;

    fireEvent.keyDown(combobox, { key: 'ArrowDown' });
    expect(combobox).toHaveAttribute('aria-activedescendant', nextOption.id);
    expect(within(listbox).getByRole('option', { name: 'pve1' })).toHaveAttribute(
      'aria-selected',
      'false',
    );
    expect(nextOption).toHaveAttribute('aria-selected', 'true');
    await Promise.resolve();
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'nearest', inline: 'nearest' });
  });

  it('commits the active option and restores focus to the chip trigger', async () => {
    render(() => <Harness />);

    const trigger = screen.getByRole('button', { name: 'Node: pve1' });
    fireEvent.click(trigger);
    await Promise.resolve();
    const combobox = screen.getByRole('combobox', { name: 'Filter Node values' });

    fireEvent.keyDown(combobox, { key: 'ArrowDown' });
    fireEvent.keyDown(combobox, { key: 'Enter' });
    await Promise.resolve();

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Node: pve2' })).toHaveFocus();
  });

  it('dismisses on Escape and returns focus without changing the filter', async () => {
    render(() => <Harness />);

    const trigger = screen.getByRole('button', { name: 'Node: pve1' });
    fireEvent.click(trigger);
    await Promise.resolve();
    fireEvent.keyDown(screen.getByRole('combobox'), { key: 'Escape' });
    await Promise.resolve();

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
    expect(trigger).toHaveTextContent('pve1');
  });

  it('dismisses on Tab without changing the filter or overriding sequential focus', async () => {
    render(() => (
      <>
        <Harness />
        <button type="button">Next control</button>
      </>
    ));

    const trigger = screen.getByRole('button', { name: 'Node: pve1' });
    fireEvent.click(trigger);
    await Promise.resolve();
    const combobox = screen.getByRole('combobox', { name: 'Filter Node values' });

    fireEvent.keyDown(combobox, { key: 'ArrowDown' });
    fireEvent.keyDown(combobox, { key: 'Tab' });
    const nextControl = screen.getByRole('button', { name: 'Next control' });
    nextControl.focus();
    await Promise.resolve();

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Node: pve1' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(nextControl).toHaveFocus();
  });

  it('announces an empty search and keeps every chip target at least 24px high', async () => {
    render(() => <Harness />);

    const trigger = screen.getByRole('button', { name: 'Node: pve1' });
    const remove = screen.getByRole('button', { name: 'Remove Node filter' });
    expect(trigger).toHaveClass('min-h-6');
    expect(remove).toHaveClass('min-h-6', 'min-w-6');

    fireEvent.click(trigger);
    await Promise.resolve();
    fireEvent.input(screen.getByRole('combobox'), { target: { value: 'missing' } });

    expect(screen.getByRole('status')).toHaveTextContent('No values match.');
    expect(screen.getByRole('combobox')).not.toHaveAttribute('aria-activedescendant');
  });
});

describe('shouldPlaceFilterChipPopoverAbove', () => {
  it('uses the preferred upper side when it fits on a narrow viewport', () => {
    expect(
      shouldPlaceFilterChipPopoverAbove({
        availableAbove: 400,
        availableBelow: 400,
        popoverHeight: 300,
        preferAbove: true,
      }),
    ).toBe(true);
  });

  it('opens upward when the lower side clips and the upper side fits', () => {
    expect(
      shouldPlaceFilterChipPopoverAbove({
        availableAbove: 700,
        availableBelow: 100,
        popoverHeight: 300,
        preferAbove: false,
      }),
    ).toBe(true);
  });

  it('falls back to the side with more space when neither side fits', () => {
    expect(
      shouldPlaceFilterChipPopoverAbove({
        availableAbove: 250,
        availableBelow: 150,
        popoverHeight: 300,
        preferAbove: false,
      }),
    ).toBe(true);
  });
});
