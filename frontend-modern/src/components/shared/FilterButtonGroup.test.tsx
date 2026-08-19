import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import Sun from 'lucide-solid/icons/sun';
import { FilterButtonGroup } from './FilterButtonGroup';
import filterButtonGroupSource from './FilterButtonGroup.tsx?raw';
import filterButtonGroupModelSource from './filterButtonGroupModel.ts?raw';
import filterButtonGroupStateSource from './useFilterButtonGroupState.ts?raw';
import { getFilterButtonGroupCompactLabel } from './filterButtonGroupModel';

describe('FilterButtonGroup', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps shell, runtime, and model owners split', () => {
    expect(filterButtonGroupSource).toContain('useFilterButtonGroupState');
    expect(filterButtonGroupSource).toContain('getFilterButtonGroupClass');
    expect(filterButtonGroupSource).toContain('getFilterButtonGroupButtonClass');
    expect(filterButtonGroupSource).toContain('getFilterButtonGroupCompactLabel');
    expect(filterButtonGroupSource).not.toContain("label.split(' ').pop()");
    expect(filterButtonGroupSource).not.toContain('props.onChange(option.value)');
    expect(filterButtonGroupSource).not.toContain('groupClassByVariant');

    expect(filterButtonGroupStateSource).toContain('export function useFilterButtonGroupState');
    expect(filterButtonGroupStateSource).toContain('createMemo');
    expect(filterButtonGroupStateSource).toContain('props.onChange(option.value)');
    expect(filterButtonGroupStateSource).toContain('props.disabled || option.disabled');

    expect(filterButtonGroupModelSource).toContain('resolveFilterButtonGroupVariant');
    expect(filterButtonGroupModelSource).toContain('getFilterButtonGroupClass');
    expect(filterButtonGroupModelSource).toContain('getFilterButtonGroupButtonClass');
    expect(filterButtonGroupModelSource).toContain('getFilterButtonGroupCompactLabel');
    expect(filterButtonGroupModelSource).toContain("prominent: 'grid grid-cols-1 gap-2'");
    expect(filterButtonGroupModelSource).toContain('segmented:');
    expect(filterButtonGroupModelSource).toContain('compact:');
    expect(filterButtonGroupModelSource).toContain('inline-flex items-center gap-1');
  });

  it('keeps all-scope options meaningful in compact layouts', () => {
    expect(getFilterButtonGroupCompactLabel({ label: 'All time' })).toBe('All');
    expect(getFilterButtonGroupCompactLabel({ label: 'Last 24h' })).toBe('24h');
    expect(getFilterButtonGroupCompactLabel({ label: 'All time', compactLabel: 'Any time' })).toBe(
      'Any time',
    );
  });

  it('renders the active option as pressed and routes selection changes', () => {
    const onChange = vi.fn();

    render(() => (
      <FilterButtonGroup
        options={[
          { value: 'light', label: 'Light', icon: Sun },
          { value: 'dark', label: 'Dark' },
        ]}
        value="light"
        onChange={onChange}
      />
    ));

    const lightButton = screen.getByRole('button', { name: /light/i });
    const darkButton = screen.getByRole('button', { name: /dark/i });

    expect(lightButton).toHaveAttribute('aria-pressed', 'true');
    expect(darkButton).toHaveAttribute('aria-pressed', 'false');

    fireEvent.click(darkButton);
    expect(onChange).toHaveBeenCalledWith('dark');
  });

  it('renders an optional count directly beside its label', () => {
    render(() => (
      <FilterButtonGroup
        options={[{ value: 'vm', label: 'VMs', count: 1234 }]}
        value="vm"
        onChange={() => undefined}
        variant="compact"
      />
    ));

    const button = screen.getByRole('button', { name: 'VMs, 1,234' });
    expect(button).toHaveTextContent('VMs1,234');
    const count = button.querySelector('[aria-hidden="true"]');
    expect(count).toHaveClass('tabular-nums', 'leading-4');
    expect(count?.parentElement).toHaveClass('items-baseline');
  });

  it('blocks disabled option changes in the runtime owner', () => {
    const onChange = vi.fn();

    render(() => (
      <FilterButtonGroup
        options={[
          { value: 'light', label: 'Light' },
          { value: 'dark', label: 'Dark', disabled: true },
        ]}
        value="light"
        onChange={onChange}
      />
    ));

    const darkButton = screen.getByRole('button', { name: /dark/i });
    expect(darkButton).toBeDisabled();

    fireEvent.click(darkButton);
    expect(onChange).not.toHaveBeenCalled();
  });

  it('supports the settings variant without default filter styling', () => {
    render(() => (
      <FilterButtonGroup
        options={[
          { value: 'celsius', label: 'Celsius' },
          { value: 'fahrenheit', label: 'Fahrenheit' },
        ]}
        value="celsius"
        onChange={() => undefined}
        variant="settings"
      />
    ));

    const activeButton = screen.getByRole('button', { name: /celsius/i });
    const inactiveButton = screen.getByRole('button', { name: /fahrenheit/i });

    expect(activeButton.className).toContain('bg-surface');
    expect(activeButton.className).toContain('shadow-sm');
    expect(activeButton.className).not.toContain('text-blue-600');

    expect(inactiveButton.className).toContain('text-muted');
    expect(inactiveButton.className).not.toContain('border-transparent');
  });

  it('supports the prominent variant for full-width segmented controls', () => {
    render(() => (
      <FilterButtonGroup
        options={[
          { value: '24h', label: 'Last 24 Hours' },
          { value: '7d', label: 'Last 7 Days' },
        ]}
        value="24h"
        onChange={() => undefined}
        variant="prominent"
      />
    ));

    const activeButton = screen.getByRole('button', { name: /last 24 hours/i });
    const inactiveButton = screen.getByRole('button', { name: /last 7 days/i });

    expect(activeButton.className).toContain('bg-blue-50');
    expect(activeButton.className).toContain('border-blue-500');
    expect(inactiveButton.className).toContain('border-border');
    expect(inactiveButton.className).not.toContain('text-muted');
  });

  it('supports compact labelled groups for inline filter bars', () => {
    render(() => (
      <FilterButtonGroup
        ariaLabel="Type"
        options={[
          { value: 'all', label: 'All' },
          { value: 'vm', label: 'VMs' },
        ]}
        value="all"
        onChange={() => undefined}
        variant="compact"
      />
    ));

    const group = screen.getByRole('group', { name: 'Type' });
    const activeButton = within(group).getByRole('button', { name: 'All' });
    const inactiveButton = within(group).getByRole('button', { name: 'VMs' });

    expect(activeButton.className).toContain('text-base-content');
    expect(activeButton.className).toContain('shadow-sm');
    expect(inactiveButton.className).toContain('text-muted');
  });

  it('keeps every filter option touch-sized on phones and compact above the phone breakpoint', () => {
    render(() => (
      <FilterButtonGroup
        ariaLabel="Range"
        options={[
          { value: '1d', label: '1d' },
          { value: '7d', label: '7d' },
        ]}
        value="1d"
        onChange={() => undefined}
        variant="compact"
      />
    ));

    const rangeButton = screen.getByRole('button', { name: '1d' });
    expect(rangeButton).toHaveClass('min-h-11', 'min-w-11', 'sm:min-h-0', 'sm:min-w-0');
    expect(filterButtonGroupModelSource).toContain('sm:min-h-9');
    expect(filterButtonGroupModelSource).toContain('sm:min-h-10');
    expect(filterButtonGroupModelSource).toContain('sm:min-h-8');
  });

  it('keeps visual icon labels on one compact line', () => {
    render(() => (
      <FilterButtonGroup
        ariaLabel="Display mode"
        options={[
          { value: 'grouped', label: 'Grouped', visualLabel: <>Grouped</> },
          { value: 'flat', label: 'List', visualLabel: <>List</> },
        ]}
        value="grouped"
        onChange={() => undefined}
        variant="compact"
      />
    ));

    const activeButton = screen.getByRole('button', { name: 'Grouped' });
    const visualLabel = activeButton.querySelector('span');

    expect(visualLabel).toHaveClass('inline-flex');
    expect(visualLabel).toHaveClass('whitespace-nowrap');
  });

  it('supports equal-width segmented controls for compact feature settings', () => {
    render(() => (
      <FilterButtonGroup
        ariaLabel="Patrol control"
        options={[
          { value: 'monitor', label: 'Monitor' },
          { value: 'approval', label: 'Investigate', disabled: true },
          { value: 'assisted', label: 'Remediate', disabled: true },
        ]}
        value="monitor"
        onChange={() => undefined}
        variant="segmented"
      />
    ));

    const group = screen.getByRole('group', { name: 'Patrol control' });
    const activeButton = within(group).getByRole('button', { name: 'Monitor' });
    const disabledButton = within(group).getByRole('button', { name: 'Investigate' });

    expect(activeButton.className).toContain('flex-1');
    expect(activeButton.className).toContain('bg-surface');
    expect(activeButton.className).toContain('text-blue-600');
    expect(disabledButton).toBeDisabled();
    expect(disabledButton.className).toContain('opacity-50');
  });
});
