import { fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';
import { AlertHistoryMobileList } from '../AlertHistoryMobileList';
import type { AlertHistoryState } from '../useAlertHistoryState';

// Open rows offer the Assistant investigation, which needs the router.
vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));

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

    expect(container.querySelector('[data-testid="alert-history-mobile-list"]')).not.toHaveClass(
      'hidden',
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

    expect(screen.getByRole('button', { name: /^Timeline for / })).toHaveClass('min-h-11');
    expect(screen.getByRole('button', { name: /^Resource incidents for / })).toHaveClass(
      'min-h-11',
    );

    await fireEvent.click(screen.getByRole('button', { name: /^Timeline for / }));
    expect(toggleIncidentTimeline).toHaveBeenCalledWith(
      'alert-1-row',
      'alert-1',
      '2026-08-04T14:30:00.000Z',
    );
    expect(
      screen.getByRole('dialog', { name: 'Incident timeline for pve-production-01' }),
    ).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Close incident timeline' }));

    await fireEvent.click(screen.getByRole('button', { name: /^Resource incidents for / }));
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

  it('leads an open threshold alert with its live reading and keeps the breach visible', () => {
    const { state, setGroupedAlerts } = createState();
    const observedAt = new Date(Date.now() - 10_000).toISOString();
    setGroupedAlerts([
      {
        label: 'Today (August 4th)',
        fullLabel: 'Today, August 4th 2026',
        alerts: [
          {
            id: 'minipc::metric-threshold:temperature',
            source: 'alert',
            resourceId: 'node-1',
            resourceName: 'minipc',
            resourceType: 'node',
            title: 'Temperature',
            description: 'Node temperature at 95.0°C',
            severity: 'warning',
            status: 'active',
            duration: '30m',
            startTime: '2026-08-04T14:30:00.000Z',
            node: 'minipc',
            nodeDisplayName: 'minipc',
            rawAlertType: 'temperature',
            liveAlert: {
              type: 'temperature',
              value: 95,
              lastSeen: new Date(Date.now() - 20 * 60 * 1000).toISOString(),
              metricStatus: {
                phase: 'latched',
                value: 78,
                unit: '°C',
                observedAt,
                trigger: 80,
                recovery: 75,
                recoveryDelaySeconds: 300,
              },
            },
          } as never,
        ],
      },
    ]);

    render(() => <AlertHistoryMobileList state={state} />);

    expect(screen.getByText('Temperature 78°C now, back under the 80°C alert level')).toBeVisible();
    expect(
      screen.getByText('Stays open until it reaches 75°C or lower and stays there for 5 minutes.'),
    ).toBeVisible();
    // A phone has no hover, so the last breach stays on the card.
    expect(screen.getByText(/^Last reading at or above 80°C: 95°C, /)).toBeVisible();
    expect(screen.queryByText('Node temperature at 95.0°C')).toBeNull();
  });

  it('keeps resource investigation scrolling outside the virtualized history row', async () => {
    const { state } = createState();

    const { container } = render(() => <AlertHistoryMobileList state={state} />);
    await fireEvent.click(screen.getByRole('button', { name: /^Resource incidents for / }));

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
    const resourceButton = screen.getByRole('button', { name: /^Resource incidents for / });
    resourceButton.focus();
    await fireEvent.click(resourceButton);
    expect(screen.getByRole('dialog')).toBeInTheDocument();

    await fireEvent.keyDown(document, { key: 'Escape' });
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(closeResourceIncidentPanel).toHaveBeenCalled();
    expect(screen.getByRole('button', { name: /^Resource incidents for / })).toHaveFocus();
  });
});

describe('AlertHistoryMobileList changing history', () => {
  it.each(['timeline', 'resource'] as const)(
    'preserves an open %s across table and card layouts',
    async (kind) => {
      const { state } = createState();
      // The table section shows the cards when its card is too narrow for
      // the table; `active` is that decision.
      const [cardsShown, setCardsShown] = createSignal(false);
      if (kind === 'timeline') await state.toggleIncidentTimeline('alert-1-row', 'alert-1');
      else await state.openResourceIncidentPanel('node-1', 'pve-production-01', 'alert-1-row');
      const mounted = render(() => <AlertHistoryMobileList state={state} active={cardsShown} />);
      try {
        expect(screen.queryByRole('dialog')).toBeNull();
        expect(screen.getByRole('button', { name: /^Timeline for / })).toBeVisible();
        setCardsShown(true);
        expect(
          screen.getByRole('dialog', {
            name:
              kind === 'timeline'
                ? 'Incident timeline for pve-production-01'
                : 'Resource incidents for pve-production-01',
          }),
        ).toBeVisible();
        setCardsShown(false);
        expect(screen.queryByRole('dialog')).toBeNull();
        expect(
          kind === 'timeline'
            ? state.expandedIncidents().has('alert-1-row')
            : state.resourceIncidentPanel()?.rowKey === 'alert-1-row',
        ).toBe(true);
        setCardsShown(true);
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
        mounted.unmount();
      }
    },
  );

  // The table section derives `active` from the card's width, so the accessor
  // changes on every resized pixel; only a change of layout may move the open
  // expansions into the drawer, or a drawer the operator closed reopens.
  it('moves expansions into the drawer only when the layout changes', async () => {
    const { state, setGroupedAlerts } = createState();
    setGroupedAlerts((groups) => {
      const [group] = groups;
      const first = group.alerts[0];
      return [
        {
          ...group,
          alerts: [
            first,
            { ...first, id: 'alert-2', resourceId: 'node-2', resourceName: 'pve-production-02' },
          ],
        },
      ];
    });
    const keyed = {
      ...state,
      getIncidentRowKey: (alert: { id: string }) => `${alert.id}-row`,
    } as AlertHistoryState;
    await keyed.toggleIncidentTimeline('alert-1-row', 'alert-1');
    await keyed.toggleIncidentTimeline('alert-2-row', 'alert-2');
    const [cardWidth, setCardWidth] = createSignal(1000);
    render(() => <AlertHistoryMobileList state={keyed} active={() => cardWidth() < 800} />);

    setCardWidth(602);
    expect(
      screen.getByRole('dialog', { name: 'Incident timeline for pve-production-02' }),
    ).toBeVisible();
    await fireEvent.click(screen.getByRole('button', { name: 'Close incident timeline' }));
    expect(screen.queryByRole('dialog')).toBeNull();

    setCardWidth(603);
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('returns focus to the list when the investigated row disappears', async () => {
    const { closeResourceIncidentPanel, setGroupedAlerts, state } = createState();
    render(() => <AlertHistoryMobileList state={state} />);

    const resourceButton = screen.getByRole('button', { name: /^Resource incidents for / });
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
