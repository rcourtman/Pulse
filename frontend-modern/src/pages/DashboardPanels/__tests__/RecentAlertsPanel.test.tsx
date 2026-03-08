import { describe, expect, it } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import { RecentAlertsPanel } from '../RecentAlertsPanel';
import type { Alert } from '@/types/api';

function makeAlert(overrides: Partial<Alert> = {}): Alert {
  return {
    id: 'alert-1',
    level: 'critical',
    resourceName: 'tower',
    message: 'Disk failure detected',
    acknowledged: false,
    startTime: '2026-03-08T10:00:00Z',
    type: 'disk-health',
    resourceId: 'host-1',
    resourceType: 'agent',
    source: 'agent',
    ...overrides,
  } as Alert;
}

describe('RecentAlertsPanel', () => {
  it('renders summary counts when alerts exist', () => {
    render(() => (
      <RecentAlertsPanel
        alerts={[makeAlert(), makeAlert({ id: 'alert-2', level: 'warning' })]}
        criticalCount={1}
        warningCount={1}
        totalCount={2}
      />
    ));

    expect(screen.getByText(/critical ·/)).toBeInTheDocument();
    expect(screen.getAllByText('tower')).toHaveLength(2);
  });

  it('renders empty state when there are no active alerts', () => {
    render(() => (
      <RecentAlertsPanel alerts={[]} criticalCount={0} warningCount={0} totalCount={0} />
    ));

    expect(screen.getByText('No active alerts')).toBeInTheDocument();
  });
});
