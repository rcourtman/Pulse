import { fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';

import { aiChatStore } from '@/stores/aiChat';
import { AlertHistoryMobileList } from '../AlertHistoryMobileList';
import type { AlertHistoryState } from '../useAlertHistoryState';

function createState() {
  const alert = {
    id: 'alert-1',
    source: 'alert',
    resourceId: 'node-1',
    resourceName: 'pve-production-01',
    resourceType: 'node',
    title: 'Backup',
    description: 'Backup failed after the target became unavailable.',
    severity: 'warning',
    status: 'resolved',
    duration: '14m',
    startTime: '2026-08-04T14:30:00.000Z',
    node: 'pve-production-01',
    nodeDisplayName: 'Production cluster',
    rawAlertType: 'backup_failed',
  };
  const [expandedIncidents, setExpandedIncidents] = createSignal(new Set<string>());
  const [resourceIncidentPanel, setResourceIncidentPanel] = createSignal<{
    resourceId: string;
    resourceName: string;
    rowKey: string;
  } | null>(null);
  const toggleIncidentTimeline = vi.fn((rowKey: string) => {
    setExpandedIncidents((current) => {
      const next = new Set(current);
      if (next.has(rowKey)) next.delete(rowKey);
      else next.add(rowKey);
      return next;
    });
  });
  const openResourceIncidentPanel = vi.fn(
    (resourceId: string, resourceName: string, rowKey: string) => {
      setResourceIncidentPanel({ resourceId, resourceName, rowKey });
    },
  );
  const closeResourceIncidentPanel = vi.fn(() => setResourceIncidentPanel(null));

  const [groupedAlerts, setGroupedAlerts] = createSignal([
    {
      label: 'Today (August 4th)',
      fullLabel: 'Today, August 4th 2026',
      alerts: [alert],
    },
  ]);

  const state = {
    groupedAlerts,
    getIncidentRowKey: () => 'alert-1-row',
    expandedIncidents,
    incidentLoading: () => ({}),
    incidentErrors: () => ({}),
    incidentTimelines: () => ({}),
    historyIncidentEventFilters: () => new Set<string>(),
    setHistoryIncidentEventFilters: vi.fn(),
    incidentNoteDrafts: () => ({}),
    setIncidentNoteDraft: vi.fn(),
    incidentNoteSaving: () => new Set<string>(),
    saveIncidentNote: vi.fn(),
    loadIncidentTimeline: vi.fn(),
    toggleIncidentTimeline,
    openResourceIncidentPanel,
    resourceIncidentPanel,
    resourceIncidents: () => ({ 'node-1': [] }),
    resourceIncidentLoading: () => ({}),
    resourceIncidentError: () => ({}),
    resourceIncidentEventFilters: () => new Set<string>(),
    setResourceIncidentEventFilters: vi.fn(),
    refreshResourceIncidentPanel: vi.fn(),
    setResourceIncidentPanel: closeResourceIncidentPanel,
    getResource: () => undefined,
    formatAlertRowTime: () => '14:30',
    formatAlertRowTimestamp: () => 'Tuesday, 4 August 2026 at 14:30:00',
  } as unknown as AlertHistoryState;

  return {
    closeResourceIncidentPanel,
    openResourceIncidentPanel,
    state,
    setGroupedAlerts,
    toggleIncidentTimeline,
  };
}

describe('AlertHistoryMobileList', () => {
  it('keeps the operator context and actions together in a phone-sized card', async () => {
    const { openResourceIncidentPanel, state, toggleIncidentTimeline } = createState();

    const { container } = render(() => <AlertHistoryMobileList state={state} />);

    expect(container.querySelector('[data-testid="alert-history-mobile-list"]')).toHaveClass(
      'md:hidden',
    );
    expect(screen.getByRole('heading', { name: 'pve-production-01' })).toBeInTheDocument();
    expect(screen.getByText('Backup failed after the target became unavailable.')).toBeVisible();
    expect(screen.getByText('14m')).toBeVisible();
    // #1687: the row shows clock time, but the absolute date has to be
    // reachable without scrolling back to the day header.
    expect(screen.getByText('14:30')).toHaveAttribute(
      'title',
      'Tuesday, 4 August 2026 at 14:30:00',
    );
    expect(screen.getByText('Production cluster')).toBeVisible();
    expect(screen.getByText('warning')).toBeVisible();
    expect(screen.getByText('resolved')).toBeVisible();

    expect(screen.getByRole('button', { name: 'Timeline' })).toHaveClass('min-h-11');
    expect(screen.getByRole('button', { name: 'Resource' })).toHaveClass('min-h-11');

    await fireEvent.click(screen.getByRole('button', { name: 'Timeline' }));
    expect(toggleIncidentTimeline).toHaveBeenCalledWith(
      'alert-1-row',
      'alert-1',
      '2026-08-04T14:30:00.000Z',
    );
    expect(
      screen.getByRole('dialog', { name: 'Incident timeline for pve-production-01' }),
    ).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Close incident timeline' }));

    await fireEvent.click(screen.getByRole('button', { name: 'Resource' }));
    expect(openResourceIncidentPanel).toHaveBeenCalledWith(
      'node-1',
      'pve-production-01',
      'alert-1-row',
    );
    expect(
      screen.getByRole('dialog', { name: 'Resource incidents for pve-production-01' }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole('heading', { name: 'Resource incidents' })).toHaveLength(1);
  });

  it('keeps resource investigation scrolling outside the virtualized history row', async () => {
    const { state } = createState();

    const { container } = render(() => <AlertHistoryMobileList state={state} />);
    await fireEvent.click(screen.getByRole('button', { name: 'Resource' }));

    const card = container.querySelector('article');
    const dialog = screen.getByRole('dialog', {
      name: 'Resource incidents for pve-production-01',
    });
    expect(card).not.toBeNull();
    expect(card?.textContent).not.toContain('No incidents recorded for this resource yet.');
    expect(dialog).not.toBeNull();
    expect(card?.contains(dialog)).toBe(false);
    expect(screen.getByTestId('mobile-alert-investigation-scroll')).toHaveClass(
      'overflow-y-auto',
      'overscroll-contain',
    );
    expect(screen.getByText('No incidents recorded for this resource yet.')).toBeVisible();
  });

  it('dismisses the mobile investigation with Escape and restores the history surface', async () => {
    const { closeResourceIncidentPanel, state } = createState();

    render(() => <AlertHistoryMobileList state={state} />);
    const resourceButton = screen.getByRole('button', { name: 'Resource' });
    resourceButton.focus();
    await fireEvent.click(resourceButton);
    expect(screen.getByRole('dialog')).toBeInTheDocument();

    await fireEvent.keyDown(document, { key: 'Escape' });
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(closeResourceIncidentPanel).toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Resource' })).toHaveFocus();
  });
});

describe('AlertHistoryMobileList changing history', () => {
  it.each(['timeline', 'resource'] as const)(
    'preserves an open %s across desktop and phone layouts',
    async (kind) => {
      const { state } = createState();
      let resize = () => {};
      const viewport = {
        matches: true,
        addEventListener: vi.fn((_event: string, listener: () => void) => {
          resize = listener;
        }),
        removeEventListener: vi.fn(),
      };
      vi.stubGlobal('matchMedia', () => viewport);
      if (kind === 'timeline') await state.toggleIncidentTimeline('alert-1-row', 'alert-1');
      else await state.openResourceIncidentPanel('node-1', 'pve-production-01', 'alert-1-row');
      const mounted = render(() => <AlertHistoryMobileList state={state} />);
      const assistantOpen = vi.spyOn(aiChatStore, 'isOpen', 'get').mockReturnValue(false);
      try {
        expect(screen.queryByRole('dialog')).toBeNull();
        expect(screen.getByRole('button', { name: 'Timeline' })).toBeVisible();
        assistantOpen.mockReturnValue(true);
        viewport.matches = false;
        resize();
        expect(screen.queryByRole('dialog')).toBeNull();
        viewport.matches = true;
        resize();
        assistantOpen.mockReturnValue(false);
        viewport.matches = false;
        resize();
        expect(
          screen.getByRole('dialog', {
            name:
              kind === 'timeline'
                ? 'Incident timeline for pve-production-01'
                : 'Resource incidents for pve-production-01',
          }),
        ).toBeVisible();
        viewport.matches = true;
        resize();
        expect(screen.queryByRole('dialog')).toBeNull();
        expect(
          kind === 'timeline'
            ? state.expandedIncidents().has('alert-1-row')
            : state.resourceIncidentPanel()?.rowKey === 'alert-1-row',
        ).toBe(true);
        viewport.matches = false;
        resize();
        expect(screen.getByRole('dialog')).toBeVisible();
        await fireEvent.click(
          screen.getByRole('button', {
            name: kind === 'timeline' ? 'Close incident timeline' : 'Close resource incidents',
          }),
        );
        expect(screen.queryByRole('dialog')).toBeNull();
        expect(
          kind === 'timeline'
            ? state.expandedIncidents().has('alert-1-row')
            : state.resourceIncidentPanel() !== null,
        ).toBe(false);
      } finally {
        assistantOpen.mockRestore();
        mounted.unmount();
        vi.unstubAllGlobals();
      }
      expect(viewport.removeEventListener).toHaveBeenCalledWith('change', resize);
    },
  );

  it('returns focus to the list when the investigated row disappears', async () => {
    const { closeResourceIncidentPanel, setGroupedAlerts, state } = createState();
    render(() => <AlertHistoryMobileList state={state} />);

    const resourceButton = screen.getByRole('button', { name: 'Resource' });
    resourceButton.focus();
    await fireEvent.click(resourceButton);
    expect(
      screen.getByRole('dialog', {
        name: 'Resource incidents for pve-production-01',
      }),
    ).toBeInTheDocument();

    // A refresh or filter change can remove the originating history card.
    setGroupedAlerts([]);
    expect(resourceButton).not.toBeInTheDocument();
    await fireEvent.keyDown(document, { key: 'Escape' });
    await new Promise<void>((resolve) =>
      requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
    );

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(closeResourceIncidentPanel).toHaveBeenCalledOnce();
    expect(screen.getByTestId('alert-history-mobile-list')).toHaveFocus();
  });
});

describe('history handoff focus ownership', () => {
  it('leaves focus with Assistant after the source drawer closes', async () => {
    const { state } = createState();
    state.incidentTimelines = () =>
      ({
        'alert-1-row': {
          id: 'incident-focus',
          alertType: 'backup',
          level: 'warning',
          openedAt: '2026-09-07T12:00:00Z',
          resourceId: 'node-1',
          resourceName: 'Backup',
          status: 'resolved',
          events: [],
        },
      }) as any;
    aiChatStore.setEnabled(true);
    const composer = document.createElement('textarea');
    document.body.append(composer);
    const open = vi.spyOn(aiChatStore, 'open').mockImplementation(() => composer.focus());
    try {
      render(() => <AlertHistoryMobileList state={state} />);
      await fireEvent.click(screen.getByRole('button', { name: 'Timeline' }));
      await fireEvent.click(
        screen.getByRole('button', {
          name: 'Discuss incident incident-focus with Pulse Assistant',
        }),
      );
      await new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      );
      expect(screen.queryByRole('dialog')).toBeNull();
      expect(open).toHaveBeenCalledOnce();
      expect(composer).toHaveFocus();
    } finally {
      open.mockRestore();
      composer.remove();
      aiChatStore.setEnabled(false);
    }
  });
});
