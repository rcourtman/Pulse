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
    const columnsButton = screen.getByRole('button', { name: /columns/i });
    const resetButton = screen.getByRole('button', { name: /reset all/i });
    expect(columnsButton).toBeInTheDocument();
    expect(resetButton).toBeInTheDocument();
    const trailingActions = columnsButton.closest('.page-controls-toolbar-actions');
    expect(trailingActions).not.toBeNull();
    expect(trailingActions!).toContainElement(resetButton);
    expect(trailingActions!).toHaveClass('ml-auto');
    expect(trailingActions!).toHaveClass('justify-end');
    expect(trailingActions!).not.toHaveClass('2xl:ml-auto');

    fireEvent.click(screen.getByRole('button', { name: /filters/i }));
    expect(onToggleFilters).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /reset all/i }));
    expect(onReset).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /columns/i }));
    fireEvent.click(screen.getByLabelText('Subject'));
    expect(onToggleColumn).toHaveBeenCalledWith('subject');
  });

  it('renders at most one scope utility slot for the active breakpoint', () => {
    const { unmount } = render(() => (
      <PageControls
        search={<div data-testid="search">Search</div>}
        showFilters={true}
        mobileFilters={{ enabled: false, count: 0, onToggle: vi.fn() }}
        mobileTrailing={<div data-testid="scope-slot">mobile scope</div>}
        utilityActions={<div data-testid="scope-slot">desktop scope</div>}
      >
        <div>Filters</div>
      </PageControls>
    ));

    expect(screen.getAllByTestId('scope-slot')).toHaveLength(1);
    expect(screen.getByTestId('scope-slot')).toHaveTextContent('desktop scope');

    unmount();

    render(() => (
      <PageControls
        search={<div data-testid="search">Search</div>}
        showFilters={true}
        mobileFilters={{ enabled: true, count: 0, onToggle: vi.fn() }}
        mobileTrailing={<div data-testid="scope-slot">mobile scope</div>}
        utilityActions={<div data-testid="scope-slot">desktop scope</div>}
      >
        <div>Filters</div>
      </PageControls>
    ));

    expect(screen.getAllByTestId('scope-slot')).toHaveLength(1);
    expect(screen.getByTestId('scope-slot')).toHaveTextContent('mobile scope');
  });

  it('keeps toolbar trailing actions grouped with columns in collapsed filter layouts', () => {
    render(() => (
      <PageControls
        search={<div data-testid="search">Search</div>}
        showFilters={true}
        mobileFilters={{ enabled: true, count: 1, onToggle: vi.fn() }}
        toolbarTrailing={<div data-testid="toolbar-trailing">Grouped List</div>}
        columnVisibility={{
          availableToggles: () => [{ id: 'subject', label: 'Subject' }],
          isHiddenByUser: () => false,
          toggle: vi.fn(),
          resetToDefaults: vi.fn(),
        }}
        utilityActions={<div data-testid="desktop-utility">Desktop utility</div>}
      >
        <div>Filters</div>
      </PageControls>
    ));

    const trailing = screen.getByTestId('toolbar-trailing');
    const columnsButton = screen.getByRole('button', { name: /columns/i });
    const trailingActions = columnsButton.closest('.page-controls-toolbar-actions');

    expect(trailingActions).not.toBeNull();
    expect(trailingActions!).toContainElement(trailing);
    expect(trailingActions!).toContainElement(columnsButton);
    expect(trailingActions!).toHaveClass('ml-auto');
    expect(trailingActions!).toHaveClass('justify-end');
    expect(screen.queryByTestId('desktop-utility')).not.toBeInTheDocument();
  });
});
