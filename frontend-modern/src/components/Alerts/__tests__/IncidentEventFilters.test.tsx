import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { IncidentEventFilters } from '../IncidentEventFilters';

describe('IncidentEventFilters', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('renders compact filters with quick selection controls', () => {
    const [filters, setFilters] = createSignal(new Set(['alert_fired', 'note']));

    render(() => (
      <IncidentEventFilters
        filters={filters}
        setFilters={setFilters}
        variant="compact"
        showQuickSelection
      />
    ));

    expect(screen.getByText('Filters')).toBeInTheDocument();
    expect(screen.getByText('All')).toBeInTheDocument();
    expect(screen.getByText('None')).toBeInTheDocument();

    fireEvent.click(screen.getByText('None'));
    expect(filters().size).toBe(0);

    fireEvent.click(screen.getByText('All'));
    expect(filters().has('alert_fired')).toBe(true);
    expect(filters().has('note')).toBe(true);
    expect(filters().has('runbook')).toBe(true);
  });

  it('renders panel filters without quick selection controls', () => {
    const [filters, setFilters] = createSignal(new Set(['alert_fired']));

    render(() => (
      <IncidentEventFilters filters={filters} setFilters={setFilters} variant="panel" />
    ));

    expect(screen.getByText('Filter events:')).toBeInTheDocument();
    expect(screen.queryByText('All')).not.toBeInTheDocument();
    expect(screen.queryByText('None')).not.toBeInTheDocument();

    fireEvent.click(screen.getByText('Ack'));
    expect(filters().has('alert_acknowledged')).toBe(true);
  });
});
