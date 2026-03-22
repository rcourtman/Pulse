import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';
import { notificationStore } from '@/stores/notifications';
import { RecentAlertsPanel } from '../RecentAlertsPanel';
import type { Alert } from '@/types/api';

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    acknowledge: vi.fn(),
    bulkAcknowledge: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

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
  beforeEach(() => {
    vi.mocked(AlertsAPI.acknowledge).mockReset();
    vi.mocked(AlertsAPI.bulkAcknowledge).mockReset();
    vi.mocked(notificationStore.success).mockReset();
    vi.mocked(notificationStore.error).mockReset();
  });

  it('renders summary counts when alerts exist', () => {
    render(() => (
      <RecentAlertsPanel alerts={[makeAlert(), makeAlert({ id: 'alert-2', level: 'warning' })]} />
    ));

    expect(screen.getByText(/critical ·/)).toBeInTheDocument();
    expect(screen.getAllByText('tower')).toHaveLength(2);
  });

  it('renders empty state when there are no active alerts', () => {
    render(() => <RecentAlertsPanel alerts={[]} />);

    expect(screen.getByText('No active alerts')).toBeInTheDocument();
  });

  it('routes single acknowledge actions through the shared alert acknowledgement owner', async () => {
    vi.mocked(AlertsAPI.acknowledge).mockResolvedValue(undefined as never);

    render(() => (
      <RecentAlertsPanel alerts={[makeAlert(), makeAlert({ id: 'alert-2', message: 'Memory high' })]} />
    ));

    await fireEvent.click(screen.getAllByText('Ack')[0]);

    await waitFor(() => {
      expect(AlertsAPI.acknowledge).toHaveBeenCalledWith('alert-1');
    });
    expect(notificationStore.success).toHaveBeenCalledWith('Alert acknowledged');
  });

  it('routes bulk acknowledge actions through the shared alert acknowledgement owner', async () => {
    vi.mocked(AlertsAPI.bulkAcknowledge).mockResolvedValue({
      results: [
        { alertIdentifier: 'alert-1', success: true },
        { alertIdentifier: 'alert-2', success: true },
      ],
    } as never);

    render(() => (
      <RecentAlertsPanel alerts={[makeAlert(), makeAlert({ id: 'alert-2', message: 'Memory high' })]} />
    ));

    await fireEvent.click(screen.getByText('Ack All'));

    await waitFor(() => {
      expect(AlertsAPI.bulkAcknowledge).toHaveBeenCalledWith(['alert-1', 'alert-2']);
    });
    expect(notificationStore.success).toHaveBeenCalledWith('Acknowledged 2 alerts.');
  });
});
