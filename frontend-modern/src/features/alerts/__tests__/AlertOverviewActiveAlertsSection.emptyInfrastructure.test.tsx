import type { JSX } from 'solid-js';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AlertOverviewActiveAlertsSection } from '../AlertOverviewActiveAlertsSection';
import type { AlertIncidentTimelineState } from '../useAlertIncidentTimelineState';
import type { AlertOverviewState } from '../useAlertOverviewState';

vi.mock('@solidjs/router', () => ({
  A: (props: JSX.AnchorHTMLAttributes<HTMLAnchorElement>) => <a {...props} />,
}));

const emptyOverviewState = {
  alertStats: () => ({ total: 0, critical: 0, warning: 0, acknowledged: 0, active: 0 }),
  activeAlertCounts: () => ({ total: 0, critical: 0, warning: 0, acknowledged: 0 }),
  activeGroups: () => [],
  filteredAlerts: () => [],
  hasAcknowledgedAlerts: () => false,
  isAcknowledgingAll: () => false,
  acknowledgeAll: vi.fn(),
  acknowledgeGroup: vi.fn(),
  unacknowledgeGroup: vi.fn(),
  isGroupPending: () => false,
} as unknown as AlertOverviewState;

const emptyTimelineState = {
  getTimeline: () => undefined,
  ensureLoaded: vi.fn(),
} as unknown as AlertIncidentTimelineState;

describe('AlertOverviewActiveAlertsSection empty infrastructure state', () => {
  afterEach(() => cleanup());

  it('guides operators to add infrastructure when no systems are connected', () => {
    render(() => (
      <AlertOverviewActiveAlertsSection
        state={emptyOverviewState}
        timelineState={emptyTimelineState}
        activeAlerts={{}}
        alertsDisabled={false}
        showAcknowledged={false}
        setShowAcknowledged={vi.fn()}
        hasMonitoredResources={false}
        canAddInfrastructure={true}
      />
    ));

    expect(screen.getByText('No systems connected yet')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Add infrastructure' })).toHaveAttribute(
      'href',
      '/settings/infrastructure?add=pick',
    );
  });

  it('keeps the standard all-clear empty state once systems are connected', () => {
    render(() => (
      <AlertOverviewActiveAlertsSection
        state={emptyOverviewState}
        timelineState={emptyTimelineState}
        activeAlerts={{}}
        alertsDisabled={false}
        showAcknowledged={false}
        setShowAcknowledged={vi.fn()}
        hasMonitoredResources={true}
        canAddInfrastructure={true}
      />
    ));

    expect(screen.getByText('No active alerts')).toBeInTheDocument();
    expect(screen.queryByText('No systems connected yet')).not.toBeInTheDocument();
  });
});
