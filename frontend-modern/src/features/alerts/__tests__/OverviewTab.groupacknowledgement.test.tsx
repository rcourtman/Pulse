import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import type { Alert } from '@/types/api';
import { OverviewTab } from '../OverviewTab';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));
vi.mock('@/api/alerts', () => ({
  AlertsAPI: { getDeliveryDiagnoses: vi.fn().mockResolvedValue([]), bulkAcknowledge: vi.fn() },
}));
vi.mock('@/api/notifications', () => ({
  NotificationsAPI: { getHealth: vi.fn().mockResolvedValue({ queue: { status: 'healthy' } }) },
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));
vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));

const alert = (id: string, acknowledged = false): Alert => ({
  id,
  type: 'connection',
  level: 'warning',
  resourceId: id === 'unrelated' ? 'other-host' : 'host',
  resourceName: 'Same display name',
  node: '',
  instance: 'lab',
  message: `Synthetic observation for ${id}`,
  value: 0,
  threshold: 0,
  startTime: '2026-10-20T12:00:00Z',
  acknowledged,
});

describe('OverviewTab group acknowledgement', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-10-20T12:01:00Z'));
    vi.mocked(AlertsAPI.bulkAcknowledge).mockReset();
    vi.mocked(notificationStore.success).mockReset();
    vi.mocked(notificationStore.error).mockReset();
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('counts only pending members, disables overlapping controls, and keeps a rejected member actionable', async () => {
    type Response = Awaited<ReturnType<typeof AlertsAPI.bulkAcknowledge>>;
    let finish!: (value: Response) => void;
    vi.mocked(AlertsAPI.bulkAcknowledge).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    render(() => {
      const [alerts, setAlerts] = createSignal<Record<string, Alert>>({
        primary: alert('primary'),
        related: alert('related'),
        acknowledged: alert('acknowledged', true),
        unrelated: alert('unrelated'),
      });
      return (
        <OverviewTab
          activeAlerts={alerts()}
          updateAlert={(id, updates) =>
            setAlerts((old) => ({ ...old, [id]: { ...old[id], ...updates } }))
          }
          overrides={[]}
          showAcknowledged={() => true}
          setShowAcknowledged={vi.fn()}
          showQuickTip={() => false}
          dismissQuickTip={vi.fn()}
          alertsDisabled={() => false}
        />
      );
    });
    const groupButton = screen.getByRole('button', { name: 'Acknowledge all (2)' });
    const allButton = screen.getByRole('button', { name: 'Acknowledge all (3)' });
    await fireEvent.click(groupButton);
    expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['primary', 'related']);
    expect(groupButton).toBeDisabled();
    expect(groupButton).toHaveAttribute('aria-busy', 'true');
    expect(groupButton).toHaveTextContent('Acknowledging...');
    expect(allButton).toBeDisabled();
    await fireEvent.click(groupButton);
    await fireEvent.click(allButton);
    expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledTimes(1);
    finish({
      results: [
        { alertIdentifier: 'primary', success: true },
        { alertIdentifier: 'related', success: false },
      ],
    });
    await vi.advanceTimersByTimeAsync(1500);
    // The live grouped projection replaces its group when acknowledgement
    // changes. Inspect the newly rendered retry, not a detached old button.
    const retryButton = screen.getByRole('button', { name: 'Acknowledge all (1)' });
    expect(retryButton).toBeEnabled();
    expect(retryButton).toHaveAttribute('aria-busy', 'false');
    expect(notificationStore.success).toHaveBeenCalledWith('Acknowledged 1 alert.');
    expect(notificationStore.error).toHaveBeenCalledWith('Failed to acknowledge 1 alert.');
    vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValueOnce({
      results: [{ alertIdentifier: 'related', success: true }],
    });
    await fireEvent.click(retryButton);
    await vi.advanceTimersByTimeAsync(1500);
    expect(AlertsAPI.bulkAcknowledge).toHaveBeenLastCalledWith(['related']);
    expect(retryButton).not.toBeInTheDocument();
    // The independent same-name host remains active and is not a group member.
    expect(screen.getByRole('button', { name: 'Acknowledge all (1)' })).toBeInTheDocument();
  });
});
