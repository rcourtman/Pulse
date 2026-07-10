import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { For, createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  ChartVisibilityToggleButton,
  FilterHeader,
  FilterToolbarPanel,
  FilterSegmentedControl,
  LabeledFilterSelect,
  LabeledFilterToggleGroup,
  isCompactFilterToggleGroupEligible,
  filterPanelDefaultWidthClass,
  filterPanelClass,
  resolveFilterSelectDomValue,
} from './FilterToolbar';
import toggleSource from './Toggle.tsx?raw';
import toggleModelSource from './toggleModel.ts?raw';
import toggleStateSource from './useToggleState.ts?raw';
import filterToolbarSource from './FilterToolbar.tsx?raw';

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

  it('uses labeled toggle groups for compact stable filter choices', () => {
    const onChange = vi.fn();
    render(() => (
      <LabeledFilterToggleGroup
        id="status-filter"
        label="Status"
        value="all"
        onChange={onChange}
        options={[
          { value: 'all', label: 'All' },
          { value: 'running', label: 'Running' },
          { value: 'stopped', label: 'Stopped' },
        ]}
      />
    ));

    const statusGroup = screen.getByRole('group', { name: 'Status' });
    expect(statusGroup).toBeInTheDocument();
    expect(statusGroup).toHaveClass('hidden');
    expect(statusGroup).toHaveClass('xl:inline-flex');
    const fallbackSelect = screen.getByLabelText('Status', { selector: 'select' });
    expect(fallbackSelect).toBeInTheDocument();
    expect(fallbackSelect.parentElement).toHaveClass('xl:hidden');
    const runningButton = screen.getByRole('button', { name: 'Running' });
    expect(runningButton).toHaveAttribute('aria-pressed', 'false');

    runningButton.click();

    expect(onChange).toHaveBeenCalledWith('running');
    expect(
      isCompactFilterToggleGroupEligible([
        { value: 'all', label: 'All' },
        { value: 'running', label: 'Running' },
        { value: 'degraded', label: 'Degraded' },
        { value: 'stopped', label: 'Stopped' },
      ]),
    ).toBe(true);
    expect(
      isCompactFilterToggleGroupEligible([
        { value: 'all', label: 'All' },
        { value: 'one', label: 'One' },
        { value: 'two', label: 'Two' },
        { value: 'three', label: 'Three' },
        { value: 'four', label: 'Four' },
      ]),
    ).toBe(true);
    expect(
      isCompactFilterToggleGroupEligible([
        { value: 'all', label: 'All' },
        { value: 'one', label: 'One' },
        { value: 'two', label: 'Two' },
        { value: 'three', label: 'Three' },
        { value: 'four', label: 'Four' },
        { value: 'five', label: 'Five' },
      ]),
    ).toBe(false);
  });

  it('keeps chart visibility as an explicit display toggle action', () => {
    const [collapsed, setCollapsed] = createSignal(false);
    const onToggle = vi.fn(() => setCollapsed((current) => !current));

    render(() => (
      <ChartVisibilityToggleButton
        collapsed={collapsed()}
        onToggle={onToggle}
        data-testid="charts-toggle"
      />
    ));

    const hideButton = screen.getByRole('button', { name: 'Hide charts' });
    expect(hideButton).toHaveTextContent('Charts');
    expect(hideButton).toHaveAttribute('aria-pressed', 'true');
    expect(hideButton).toHaveAttribute('title', 'Hide charts');
    expect(hideButton).toHaveClass('hidden');
    expect(hideButton).toHaveClass('lg:inline-flex');

    hideButton.click();

    const showButton = screen.getByRole('button', { name: 'Show charts' });
    expect(onToggle).toHaveBeenCalledTimes(1);
    expect(showButton).toHaveTextContent('Charts');
    expect(showButton).toHaveAttribute('aria-pressed', 'false');
    expect(showButton).toHaveAttribute('title', 'Show charts');
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

  it('keeps labelled filter selects on the shared FormSelect primitive', () => {
    expect(filterToolbarSource).toContain("import { FormSelect } from './FormSelect';");
    expect(filterToolbarSource).toContain('<FormSelect');
    expect(filterToolbarSource).not.toContain('<select');
    expect(filterToolbarSource).not.toContain('MutationObserver');
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
          <For each={options()}>
            {(option) => <option value={option.value}>{option.label}</option>}
          </For>
        </LabeledFilterSelect>
      </>
    ));

    const select = screen.getByTestId('platform-filter') as HTMLSelectElement;
    expect(select.value).toBe('');

    screen.getByRole('button', { name: 'Load options' }).click();

    await waitFor(() => expect(select.value).toBe('truenas'));
    expect((screen.getByRole('option', { name: 'TrueNAS' }) as HTMLOptionElement).selected).toBe(
      true,
    );
  });

  it('keeps stale controlled select values out of the native DOM value', () => {
    expect(resolveFilterSelectDomValue('proxmox-pve', ['', 'docker'])).toBe('');
    expect(resolveFilterSelectDomValue('docker', ['', 'docker'])).toBe('docker');
    expect(resolveFilterSelectDomValue(undefined, ['', 'docker'])).toBeUndefined();
  });

  it('keeps the label association current when a dynamic select id changes', async () => {
    const [mode, setMode] = createSignal<'node' | 'k8s'>('node');
    const filterConfig = () =>
      mode() === 'node'
        ? {
            id: 'workloads-node-filter',
            label: 'Node',
            options: [{ value: '', label: 'All nodes' }],
          }
        : {
            id: 'workloads-k8s-context-filter',
            label: 'K8s cluster',
            options: [{ value: '', label: 'All K8s clusters' }],
          };

    render(() => (
      <>
        <button type="button" onClick={() => setMode('k8s')}>
          Show pods
        </button>
        <LabeledFilterSelect
          id={filterConfig().id}
          label={filterConfig().label}
          value=""
          data-testid="dynamic-filter"
        >
          <For each={filterConfig().options}>
            {(option) => <option value={option.value}>{option.label}</option>}
          </For>
        </LabeledFilterSelect>
      </>
    ));

    expect(screen.getByLabelText('Node')).toBe(screen.getByTestId('dynamic-filter'));

    screen.getByRole('button', { name: 'Show pods' }).click();

    await waitFor(() =>
      expect(screen.getByLabelText('K8s cluster')).toBe(screen.getByTestId('dynamic-filter')),
    );
  });

  it('keeps shared filter popovers above nested table and card shells', () => {
    expect(filterPanelClass).toContain('absolute');
    expect(filterPanelClass).toContain('z-[80]');
    expect(filterPanelClass).not.toContain('w-[min(40rem,calc(100vw-2rem))]');
    expect(filterPanelDefaultWidthClass).toContain('w-[min(40rem,calc(100vw-2rem))]');
    expect(filterToolbarSource).toContain("'flex min-h-10 items-center gap-1.5");
  });

  it('allows narrow shared popovers to opt out of the default wide panel width', () => {
    render(() => (
      <FilterToolbarPanel widthClass="w-56 max-w-[calc(100vw-2rem)]" data-testid="panel">
        Panel
      </FilterToolbarPanel>
    ));

    const panel = screen.getByTestId('panel');
    expect(panel).toHaveClass('w-56');
    expect(panel).toHaveClass('max-w-[calc(100vw-2rem)]');
    expect(panel).not.toHaveClass('w-[min(40rem,calc(100vw-2rem))]');
  });
});
