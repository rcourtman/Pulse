import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';

import { AlertHistoryMobileList } from '../AlertHistoryMobileList';
import type { AlertHistoryState } from '../useAlertHistoryState';

function createState() {
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
    expect(screen.getByText('Production cluster')).toBeVisible();
    expect(screen.getByText('warning')).toBeVisible();
    expect(screen.getByText('resolved')).toBeVisible();

    await fireEvent.click(screen.getByRole('button', { name: 'Timeline' }));
    expect(toggleIncidentTimeline).toHaveBeenCalledWith(
      'alert-1-row',
      'alert-1',
      '2026-08-04T14:30:00.000Z',
    );

    await fireEvent.click(screen.getByRole('button', { name: 'Resource' }));
    expect(openResourceIncidentPanel).toHaveBeenCalledWith('node-1', 'pve-production-01');
  });
});
