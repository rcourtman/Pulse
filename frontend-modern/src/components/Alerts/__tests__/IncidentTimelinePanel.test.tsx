import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { IncidentTimelinePanel } from '../IncidentTimelinePanel';
import { aiChatStore } from '@/stores/aiChat';
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
    aiChatStore.close();
    aiChatStore.clearAllContext();
    aiChatStore.setEnabled(false);
    cleanup();
    vi.restoreAllMocks();
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
        loading={() => state().loading}
        error={() => state().error}
        timeline={() => state().timeline}
        filters={filters}
        setFilters={setFilters}
        filterVariant="panel"
        eventCardVariant="alt"
        noteDraft={() => ''}
        onNoteDraftChange={vi.fn()}
        noteSaving={() => false}
        onSaveNote={vi.fn()}
        onRetry={vi.fn()}
      />
    ));

    expect(screen.getByRole('status')).toHaveTextContent('Loading timeline...');

    setState({ loading: false, error: true, timeline: undefined });

    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load timeline.');
    expect(screen.getByRole('button', { name: 'Retry' })).toHaveClass('min-h-11', 'focus:ring-2');

    setState({ loading: false, error: false, timeline: undefined });

    expect(screen.getByRole('status')).toHaveTextContent('No incident timeline available.');
  });

  it('keeps failure recovery operable and out of implicit form submission', () => {
    const [filters, setFilters] = createSignal(new Set<string>());
    const onRetry = vi.fn();

    render(() => (
      <IncidentTimelinePanel
        loading={() => false}
        error={() => true}
        timeline={() => undefined}
        filters={filters}
        setFilters={setFilters}
        filterVariant="panel"
        eventCardVariant="alt"
        noteDraft={() => ''}
        onNoteDraftChange={vi.fn()}
        noteSaving={() => false}
        onSaveNote={vi.fn()}
        onRetry={onRetry}
      />
    ));

    const retry = screen.getByRole('button', { name: 'Retry' });
    expect(retry).toHaveAttribute('type', 'button');
    fireEvent.click(retry);
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it('renders shared timeline content and note handling', () => {
    const [filters, setFilters] = createSignal(new Set(['command']));
    const handleNoteDraftChange = vi.fn();
    const handleSave = vi.fn();

    render(() => (
      <IncidentTimelinePanel
        loading={() => false}
        error={() => false}
        timeline={() => makeTimeline()}
        filters={filters}
        setFilters={setFilters}
        filterVariant="compact"
        eventCardVariant="surface"
        noteDraft={() => 'operator note'}
        onNoteDraftChange={handleNoteDraftChange}
        noteSaving={() => false}
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

    const saveNote = screen.getByRole('button', { name: 'Save Note' });
    expect(saveNote).toHaveAttribute('type', 'button');
    expect(saveNote).toHaveClass('min-h-11', 'focus:ring-2');
    fireEvent.click(saveNote);
    expect(handleSave).toHaveBeenCalledTimes(1);
  });

  it('opens Assistant with a sanitized incident briefing from the loaded timeline', () => {
    const [filters, setFilters] = createSignal(new Set(['command']));
    const openSpy = vi.spyOn(aiChatStore, 'open');
    aiChatStore.setEnabled(true);

    render(() => (
      <IncidentTimelinePanel
        loading={() => false}
        error={() => false}
        timeline={() => makeTimeline()}
        filters={filters}
        setFilters={setFilters}
        filterVariant="compact"
        eventCardVariant="surface"
        noteDraft={() => ''}
        onNoteDraftChange={vi.fn()}
        noteSaving={() => false}
        onSaveNote={vi.fn()}
        onRetry={vi.fn()}
      />
    ));

    fireEvent.click(
      screen.getByRole('button', {
        name: 'Discuss incident incident-1 with Pulse Assistant',
      }),
    );

    expect(openSpy).toHaveBeenCalledTimes(1);
    const [context] = openSpy.mock.calls[0] as [Record<string, unknown>];
    expect(context).toMatchObject({
      autonomousMode: false,
      briefing: {
        sourceLabel: 'Pulse Alerts',
        title: 'Incident timeline attached',
        actionLabel: 'Discuss incident incident-1',
      },
    });
    expect(JSON.stringify(context)).not.toContain('systemctl status pulse');
  });
});
