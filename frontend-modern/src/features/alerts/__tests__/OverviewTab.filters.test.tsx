import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { Alert } from '@/types/api';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    getConfig: vi.fn(),
    activate: vi.fn(),
    getDeliveryDiagnoses: vi.fn().mockResolvedValue([]),
  },
}));

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getHealth: vi.fn(),
    retryTerminalFailures: vi.fn(),
    dismissTerminalFailures: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn() },
}));

vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));

import { OverviewTab } from '../OverviewTab';
import { createAlertAcknowledgementRequests } from '@/stores/alertAcknowledgementRequests';

const makeAlert = (overrides: Partial<Alert>): Alert => ({
  id: 'alert-1',
  type: 'cpu',
  level: 'warning',
  resourceId: 'pve:vm:101',
  resourceName: 'web-01',
  node: 'pve1',
  instance: 'pve1',
  message: 'CPU usage high',
  value: 92,
  threshold: 80,
  startTime: '2026-11-23T18:00:00Z',
  acknowledged: false,
  ...overrides,
});

describe('OverviewTab active alert search and severity filters', () => {
  afterEach(() => {
    cleanup();
  });

  const renderWithAlerts = () => {
    const activeAlerts: Record<string, Alert> = {
      'alert-crit': makeAlert({
        id: 'alert-crit',
        level: 'critical',
        resourceId: 'docker:ops-postgres',
        resourceName: 'ops-postgres',
        message: 'Container is stopped',
      }),
      'alert-warn': makeAlert({
        id: 'alert-warn',
        level: 'warning',
        resourceId: 'pve:vm:102',
        resourceName: 'media-worker',
        message: 'Memory usage elevated',
      }),
    };

    return render(() => (
      <OverviewTab
        overrides={[]}
        activeAlerts={activeAlerts}
        updateAlert={vi.fn()}
        acknowledgementRequests={createAlertAcknowledgementRequests()}
        showQuickTip={() => false}
        dismissQuickTip={vi.fn()}
        showAcknowledged={() => false}
        setShowAcknowledged={vi.fn()}
        alertsDisabled={() => false}
      />
    ));
  };

  it('filters active alerts with clickable severity chips', () => {
    renderWithAlerts();

    expect(screen.getByText('ops-postgres')).toBeInTheDocument();
    expect(screen.getByText('media-worker')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Critical/i }));
    expect(screen.getByText('ops-postgres')).toBeInTheDocument();
    expect(screen.queryByText('media-worker')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Warning/i }));
    expect(screen.queryByText('ops-postgres')).not.toBeInTheDocument();
    expect(screen.getByText('media-worker')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /^All\b/i }));
    expect(screen.getByText('ops-postgres')).toBeInTheDocument();
    expect(screen.getByText('media-worker')).toBeInTheDocument();
  });

  it('filters active alerts by search query and lets users clear empty matches', () => {
    renderWithAlerts();

    const searchInput = screen.getByPlaceholderText('Search alerts...');
    fireEvent.input(searchInput, { target: { value: 'postgres' } });

    expect(screen.getByText('ops-postgres')).toBeInTheDocument();
    expect(screen.queryByText('media-worker')).not.toBeInTheDocument();

    fireEvent.input(searchInput, { target: { value: 'does-not-exist' } });
    expect(screen.getByText('No alerts found')).toBeInTheDocument();

    const clearButtons = screen.getAllByRole('button', { name: 'Clear filters' });
    fireEvent.click(clearButtons[clearButtons.length - 1]);
    expect(screen.getByText('ops-postgres')).toBeInTheDocument();
    expect(screen.getByText('media-worker')).toBeInTheDocument();
  });
});
