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

  it('opens Assistant with a sanitized incident briefing from the loaded timeline', () => {
    const [filters, setFilters] = createSignal(new Set(['command']));
    const openWithPromptSpy = vi.spyOn(aiChatStore, 'openWithPrompt');
    aiChatStore.setEnabled(true);

    render(() => (
      <IncidentTimelinePanel
        loading={false}
        error={false}
        timeline={makeTimeline()}
        filters={filters}
        setFilters={setFilters}
        filterVariant="compact"
        eventCardVariant="surface"
        noteDraft=""
        onNoteDraftChange={vi.fn()}
        noteSaving={false}
        onSaveNote={vi.fn()}
        onRetry={vi.fn()}
      />
    ));

    fireEvent.click(
      screen.getByRole('button', {
        name: 'Discuss incident incident-1 with Pulse Assistant',
      }),
    );

    expect(openWithPromptSpy).toHaveBeenCalledTimes(1);
    const [prompt, context] = openWithPromptSpy.mock.calls[0] as [string, Record<string, unknown>];
    expect(prompt).toContain('Discuss this Warning alert incident from Pulse Alerts.');
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
