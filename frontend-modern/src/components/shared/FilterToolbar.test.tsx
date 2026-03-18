import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { FilterHeader, FilterSegmentedControl } from './FilterToolbar';

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
});
