import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { FilterHeader } from './FilterToolbar';

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
});
