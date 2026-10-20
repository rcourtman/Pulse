import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { AlertsAPI } from '@/api/alerts';
import { setActiveLocale, DEFAULT_LOCALE } from '@/i18n';
import type { Alert, AlertDeliveryDiagnosis } from '@/types/api';
import { OverviewTab } from '../OverviewTab';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));
vi.mock('@/api/alerts', () => ({ AlertsAPI: { getDeliveryDiagnoses: vi.fn() } }));
vi.mock('@/api/notifications', () => ({
  NotificationsAPI: { getHealth: vi.fn().mockResolvedValue({ queue: { status: 'healthy' } }) },
}));
vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));

const diagnosis = (id: string, reason: string) =>
  ({
    alertIdentifier: id,
    status: 'suppressed',
    reason,
  }) as AlertDeliveryDiagnosis;
const alerts = Object.fromEntries(
  ['a1', 'a2'].map((id) => [
    id,
    {
      id,
      resourceId: id,
      resourceName: `VM ${id}`,
      type: 'cpu',
      level: 'warning',
      message: 'CPU high',
      value: 91,
      threshold: 80,
      node: 'node-1',
      startTime: '2026-10-20T00:00:00Z',
      acknowledged: false,
    } as Alert,
  ]),
);

function mount() {
  const [activeAlerts, setActiveAlerts] = createSignal(alerts);
  render(() => (
    <OverviewTab
      activeAlerts={activeAlerts()}
      overrides={[]}
      updateAlert={vi.fn()}
      showQuickTip={() => false}
      dismissQuickTip={vi.fn()}
      showAcknowledged={() => true}
      setShowAcknowledged={vi.fn()}
      alertsDisabled={() => false}
    />
  ));
  return setActiveAlerts;
}

describe('Overview notification observation freshness', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-10-20T12:00:00Z'));
    vi.mocked(AlertsAPI.getDeliveryDiagnoses).mockReset();
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('withdraws prior reasons, reports unknown once, and restores only current reasons', async () => {
    vi.mocked(AlertsAPI.getDeliveryDiagnoses)
      .mockResolvedValueOnce([diagnosis('a1', 'flapping'), diagnosis('a2', 'rate_limited')])
      .mockRejectedValueOnce(new Error('unavailable'))
      .mockResolvedValueOnce([diagnosis('a1', 'monitor_only'), diagnosis('a2', 'monitor_only')]);
    mount();
    // The polite live region exists before the result changes.
    const status = screen.getByRole('status', { name: 'Alert notification status' });
    expect(status).toBeEmptyDOMElement();
    await Promise.resolve();
    expect(screen.getByText('Flapping — notifications paused')).toBeInTheDocument();
    expect(screen.getByText('Hourly notification limit reached')).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(screen.queryByText('Flapping — notifications paused')).toBeNull();
    expect(screen.queryByText('Hourly notification limit reached')).toBeNull();
    expect(screen.getAllByText('Alert notification status is unavailable.')).toHaveLength(1);
    expect(status).toHaveTextContent('It will check again automatically.');
    expect(screen.getByText('VM a1')).toBeInTheDocument();
    expect(screen.getByText('VM a2')).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(screen.queryByText('Alert notification status is unavailable.')).toBeNull();
    expect(screen.getAllByText('Monitor-only — no notifications')).toHaveLength(2);
    expect(status).toBeEmptyDOMElement();
  });

  it('reports a first-read failure without inventing a delivery result, and clears for no active alerts', async () => {
    vi.mocked(AlertsAPI.getDeliveryDiagnoses).mockRejectedValue(new Error('unavailable'));
    const setActiveAlerts = mount();
    expect(screen.queryByText('Alert notification status is unavailable.')).toBeNull();
    await Promise.resolve();
    expect(screen.getByRole('status', { name: 'Alert notification status' })).toHaveTextContent(
      'Alert notification status is unavailable.',
    );
    setActiveAlerts({});
    expect(screen.getByRole('status', { name: 'Alert notification status' })).toBeEmptyDOMElement();
    expect(screen.getByText('No active alerts')).toBeInTheDocument();
    expect(AlertsAPI.getDeliveryDiagnoses).toHaveBeenCalledOnce();
  });
});
