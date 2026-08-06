import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';

import { AlertHistoryMobileList } from '../AlertHistoryMobileList';
import type { AlertHistoryState } from '../useAlertHistoryState';

function createState(openRowKey: string | null = null) {
  const toggleIncidentTimeline = vi.fn();
  const openResourceIncidentPanel = vi.fn();
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

  const state = {
    groupedAlerts: () => [
      {
        label: 'Today (August 4th)',
        fullLabel: 'Today, August 4th 2026',
        alerts: [alert],
      },
    ],
    getIncidentRowKey: () => 'alert-1-row',
    expandedIncidents: () => new Set<string>(),
    incidentLoading: () => ({}),
    incidentErrors: () => ({}),
    incidentTimelines: () => ({}),
    historyIncidentEventFilters: () => [],
    setHistoryIncidentEventFilters: vi.fn(),
    incidentNoteDrafts: () => ({}),
    setIncidentNoteDraft: vi.fn(),
    incidentNoteSaving: () => new Set<string>(),
    saveIncidentNote: vi.fn(),
    loadIncidentTimeline: vi.fn(),
    toggleIncidentTimeline,
    openResourceIncidentPanel,
    resourceIncidentPanel: () =>
      openRowKey
        ? { resourceId: 'node-1', resourceName: 'pve-production-01', rowKey: openRowKey }
        : null,
    resourceIncidents: () => ({ 'node-1': [] }),
    resourceIncidentLoading: () => ({}),
    resourceIncidentEventFilters: () => new Set<string>(),
    setResourceIncidentEventFilters: vi.fn(),
    refreshResourceIncidentPanel: vi.fn(),
    setResourceIncidentPanel: vi.fn(),
    getResource: () => undefined,
    formatAlertRowTime: () => '14:30',
    formatAlertRowTimestamp: () => 'Tuesday, 4 August 2026 at 14:30:00',
  } as unknown as AlertHistoryState;

  return { openResourceIncidentPanel, state, toggleIncidentTimeline };
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

    await fireEvent.click(screen.getByRole('button', { name: 'Resource' }));
    expect(openResourceIncidentPanel).toHaveBeenCalledWith(
      'node-1',
      'pve-production-01',
      'alert-1-row',
    );
  });

  // #1687: the panel used to render at page level, so on a phone it opened far
  // above whatever card the reader had tapped and the button looked inert.
  it('renders the resource incidents panel inside the card that opened it', () => {
    const { state } = createState('alert-1-row');

    const { container } = render(() => <AlertHistoryMobileList state={state} />);

    const card = container.querySelector('article');
    expect(card).not.toBeNull();
    expect(card?.textContent).toContain('No incidents recorded for this resource yet.');
  });

  it('leaves the card alone while a different row owns the panel', () => {
    const { state } = createState('some-other-row');

    const { container } = render(() => <AlertHistoryMobileList state={state} />);

    expect(container.textContent).not.toContain('No incidents recorded for this resource yet.');
  });
});
