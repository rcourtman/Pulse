import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { For, createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { FilterHeader, FilterSegmentedControl, LabeledFilterSelect } from './FilterToolbar';
import toggleSource from './Toggle.tsx?raw';
import toggleModelSource from './toggleModel.ts?raw';
import toggleStateSource from './useToggleState.ts?raw';

describe('FilterHeader', () => {
  afterEach(() => {
    cleanup();
  });

  it('gives the stacked search row full width by default', () => {
    const { container } = render(() => (
      <FilterHeader search={<div data-testid="search">Search</div>} showFilters={false}>
        <div>Filters</div>
      </FilterHeader>
    ));

    expect(screen.getByTestId('search')).toBeInTheDocument();
    expect(container.querySelector('.flex.w-full.items-center.gap-2')).not.toBeNull();
  });

  it('keeps segmented controls on value callbacks while forwarding div attributes', async () => {
    const onChange = vi.fn();
    render(() => (
      <FilterSegmentedControl
        value="all"
        onChange={onChange}
        options={[
          { value: 'all', label: 'All' },
          { value: 'warnings', label: 'Warnings' },
        ]}
        data-testid="segmented-control"
      />
    ));

    expect(screen.getByTestId('segmented-control')).toBeInTheDocument();
    screen.getByRole('button', { name: 'Warnings' }).click();
    expect(onChange).toHaveBeenCalledWith('warnings');
  });

  it('keeps shared toggle behavior on shell, runtime, and model owners', () => {
    expect(toggleSource).toContain('useToggleState');
    expect(toggleSource).toContain('getToggleTrackClass');
    expect(toggleSource).toContain('getToggleKnobClass');
    expect(toggleSource).not.toContain('defaultPrevented');
    expect(toggleSource).not.toContain('toggleSizeConfig');
    expect(toggleSource).not.toContain('handleClick =');

    expect(toggleStateSource).toContain('export function useToggleState');
    expect(toggleStateSource).toContain('defaultPrevented');
    expect(toggleStateSource).toContain('currentTarget: { checked: next }');
    expect(toggleStateSource).toContain('props.onChange?.(event)');
    expect(toggleStateSource).toContain('props.onToggle?.()');

    expect(toggleModelSource).toContain('toggleSizeConfig');
    expect(toggleModelSource).toContain('resolveToggleSize');
    expect(toggleModelSource).toContain('getToggleTrackClass');
    expect(toggleModelSource).toContain('getToggleKnobClass');
    expect(toggleModelSource).toContain('ToggleChangeEvent');
  });

  it('keeps controlled select state when options materialize asynchronously', async () => {
    const [options, setOptions] = createSignal<{ value: string; label: string }[]>([]);

    render(() => (
      <>
        <button
          type="button"
          onClick={() =>
            setOptions([
              { value: 'all', label: 'All platforms' },
              { value: 'truenas', label: 'TrueNAS' },
            ])
          }
        >
          Load options
        </button>
        <LabeledFilterSelect label="Platform" value="truenas" data-testid="platform-filter">
          <For each={options()}>{(option) => <option value={option.value}>{option.label}</option>}</For>
        </LabeledFilterSelect>
      </>
    ));

    const select = screen.getByTestId('platform-filter') as HTMLSelectElement;
    expect(select.value).toBe('');

    screen.getByRole('button', { name: 'Load options' }).click();

    await waitFor(() => expect(select.value).toBe('truenas'));
    expect(screen.getByRole('option', { name: 'TrueNAS' }).selected).toBe(true);
  });
});
