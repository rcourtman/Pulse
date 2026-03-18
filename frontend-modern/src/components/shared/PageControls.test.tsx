import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PageControls } from './PageControls';

describe('PageControls', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders canonical mobile controls, columns, and reset actions', () => {
    const onToggleColumn = vi.fn();
    const onReset = vi.fn();
    const onToggleFilters = vi.fn();

    render(() => (
      <PageControls
        search={<div data-testid="search">Search</div>}
        showFilters={true}
        mobileFilters={{ enabled: true, count: 2, onToggle: onToggleFilters }}
        columnVisibility={{
          availableToggles: () => [{ id: 'subject', label: 'Subject' }],
          isHiddenByUser: () => false,
          toggle: onToggleColumn,
          resetToDefaults: vi.fn(),
        }}
        resetAction={{ show: true, onClick: onReset, label: 'Reset all' }}
      >
        <div>Filters</div>
      </PageControls>
    ));

    expect(screen.getByTestId('search')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /filters/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /columns/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /reset all/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /filters/i }));
    expect(onToggleFilters).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /reset all/i }));
    expect(onReset).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /columns/i }));
    fireEvent.click(screen.getByLabelText('Subject'));
    expect(onToggleColumn).toHaveBeenCalledWith('subject');
  });
});
