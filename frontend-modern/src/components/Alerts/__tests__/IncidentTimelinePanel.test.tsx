import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { IncidentTimelinePanel } from '../IncidentTimelinePanel';
import type { Incident } from '@/types/api';

function makeTimeline(overrides: Partial<Incident> = {}): Incident {
  return {
    id: 'incident-1',
    alertIdentifier: 'alert-1',
    alertType: 'cpu',
    level: 'warning',
    resourceId: 'vm-100',
    resourceName: 'test-vm',
    status: 'open',
    openedAt: '2026-03-20T10:00:00Z',
    acknowledged: false,
    events: [
      {
        id: 'event-1',
        type: 'command',
        timestamp: '2026-03-20T10:05:00Z',
        summary: 'Command executed',
        details: {
          command: 'systemctl status pulse',
          output_excerpt: 'Active: active (running)',
        },
      },
    ],
    ...overrides,
  };
}

describe('IncidentTimelinePanel', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('renders loading, error, and unavailable states through shared copy', () => {
    const [filters, setFilters] = createSignal(new Set(['command']));
    const [state, setState] = createSignal<{
      loading: boolean;
      error: boolean;
      timeline?: Incident | null;
    }>({
      loading: true,
      error: false,
      timeline: undefined,
    });

    render(() => (
      <IncidentTimelinePanel
        loading={state().loading}
        error={state().error}
        timeline={state().timeline}
        filters={filters}
        setFilters={setFilters}
        filterVariant="panel"
        eventCardVariant="alt"
        noteDraft=""
        onNoteDraftChange={vi.fn()}
        noteSaving={false}
        onSaveNote={vi.fn()}
        onRetry={vi.fn()}
      />
    ));

    expect(screen.getByText('Loading timeline...')).toBeInTheDocument();

    setState({ loading: false, error: true, timeline: undefined });

    expect(screen.getByText('Failed to load timeline.')).toBeInTheDocument();

    setState({ loading: false, error: false, timeline: undefined });

    expect(screen.getByText('No incident timeline available.')).toBeInTheDocument();
  });

  it('renders shared timeline content and note handling', () => {
    const [filters, setFilters] = createSignal(new Set(['command']));
    const handleNoteDraftChange = vi.fn();
    const handleSave = vi.fn();

    render(() => (
      <IncidentTimelinePanel
        loading={false}
        error={false}
        timeline={makeTimeline()}
        filters={filters}
        setFilters={setFilters}
        filterVariant="compact"
        eventCardVariant="surface"
        noteDraft="operator note"
        onNoteDraftChange={handleNoteDraftChange}
        noteSaving={false}
        onSaveNote={handleSave}
        onRetry={vi.fn()}
      />
    ));

    expect(screen.getByText('Incident')).toBeInTheDocument();
    expect(screen.getByText('Command executed')).toBeInTheDocument();
    expect(screen.getByText('systemctl status pulse')).toBeInTheDocument();
    expect(screen.getByText('Active: active (running)')).toBeInTheDocument();
    expect(screen.getByText('All')).toBeInTheDocument();
    expect(screen.getByText('None')).toBeInTheDocument();

    fireEvent.input(screen.getByPlaceholderText('Add a note for this incident...'), {
      target: { value: 'updated note' },
      currentTarget: { value: 'updated note' },
    });
    expect(handleNoteDraftChange).toHaveBeenCalledWith('updated note');

    fireEvent.click(screen.getByText('Save Note'));
    expect(handleSave).toHaveBeenCalledTimes(1);
  });
});
