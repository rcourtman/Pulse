import { fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { describe, expect, it, vi } from 'vitest';

import { AlertHistoryItemActions } from '../AlertHistoryItemActions';
import type { AlertHistoryState } from '../useAlertHistoryState';

vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));

type HistoryAlert = Parameters<typeof AlertHistoryItemActions>[0]['alert'];

const occurrence = (id: string, startTime: string) =>
  ({
    id,
    source: 'alert',
    resourceId: 'vm-101',
    resourceName: 'db-01',
    resourceType: 'vm',
    title: 'CPU usage',
    severity: 'warning',
    status: 'resolved',
    startTime,
  }) as unknown as HistoryAlert;

// The History table repeats Timeline, Resource and the Assistant on every
// row; on the mock estate 91 buttons were all named "Timeline".
describe('AlertHistoryItemActions accessible names', () => {
  it('names each row control after its alert, resource and time', async () => {
    const [expanded, setExpanded] = createSignal(new Set<string>());
    const state = {
      getIncidentRowKey: (alert: HistoryAlert) => `${alert.id}-row`,
      expandedIncidents: expanded,
      toggleIncidentTimeline: vi.fn((rowKey: string) => setExpanded(new Set([rowKey]))),
      openResourceIncidentPanel: vi.fn(),
      formatAlertRowTimestamp: (startTime: string) =>
        startTime.startsWith('2026-10-14') ? '14 Oct 2026, 09:00' : '15 Oct 2026, 11:44',
    } as unknown as AlertHistoryState;

    render(() => (
      <>
        <AlertHistoryItemActions alert={occurrence('a-1', '2026-10-14T09:00:00Z')} state={state} />
        <AlertHistoryItemActions alert={occurrence('a-2', '2026-10-15T11:44:00Z')} state={state} />
      </>
    ));

    const earlier = screen.getByRole('button', {
      name: 'Timeline for CPU usage on db-01, 14 Oct 2026, 09:00',
    });
    expect(
      screen.getByRole('button', { name: 'Timeline for CPU usage on db-01, 15 Oct 2026, 11:44' }),
    ).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getAllByRole('button', { name: 'Resource incidents for db-01' })).toHaveLength(2);

    await fireEvent.click(earlier);
    expect(earlier).toHaveTextContent('Hide');
    expect(earlier).toHaveAttribute('aria-expanded', 'true');
    expect(earlier).toHaveAccessibleName(
      'Hide timeline for CPU usage on db-01, 14 Oct 2026, 09:00',
    );
  });
});
