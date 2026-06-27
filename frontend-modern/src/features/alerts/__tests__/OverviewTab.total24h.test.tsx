import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import type { Alert } from '@/types/api';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {},
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

function makeAlert(id: string, startTime: string, ack = false): Alert {
  return {
    id,
    resourceId: `vm-${id}`,
    resourceName: `VM ${id}`,
    type: 'cpu',
    level: 'warning',
    message: `High CPU on VM ${id}`,
    startTime,
    acknowledged: ack,
    node: 'node1',
  } as Alert;
}

function defaultProps(overrides: Record<string, unknown> = {}) {
  return {
    overrides: [] as never[],
    activeAlerts: {} as Record<string, Alert>,
    updateAlert: vi.fn(),
    showQuickTip: () => false,
    dismissQuickTip: vi.fn(),
    showAcknowledged: () => true,
    setShowAcknowledged: vi.fn(),
    alertsDisabled: () => false,
    ...overrides,
  };
}

describe('OverviewTab Last 24 Hours stat', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-06T12:00:00Z'));
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('counts only alerts with startTime within the last 24 hours', () => {
    const now = Date.now();
    const oneHourAgo = new Date(now - 3_600_000).toISOString();
    const twoDaysAgo = new Date(now - 2 * 86_400_000).toISOString();

    const activeAlerts: Record<string, Alert> = {
      recent: makeAlert('recent', oneHourAgo),
      old: makeAlert('old', twoDaysAgo),
    };

    render(() => <OverviewTab {...defaultProps({ activeAlerts })} />);

    const label = screen.getByText('Triggered (24h)');
    const statValue = label
      .closest('tr')
      ?.querySelector('[data-testid="alert-overview-stat-value"]');
    expect(statValue?.textContent).toBe('1');
  });

  it('shows 0 when all alerts are older than 24 hours', () => {
    const now = Date.now();
    const threeDaysAgo = new Date(now - 3 * 86_400_000).toISOString();

    const activeAlerts: Record<string, Alert> = {
      old1: makeAlert('old1', threeDaysAgo),
      old2: makeAlert('old2', threeDaysAgo),
    };

    render(() => <OverviewTab {...defaultProps({ activeAlerts })} />);

    const label = screen.getByText('Triggered (24h)');
    const statValue = label
      .closest('tr')
      ?.querySelector('[data-testid="alert-overview-stat-value"]');
    expect(statValue?.textContent).toBe('0');
  });

  it('counts all alerts when all are within 24 hours', () => {
    const now = Date.now();
    const recentTime = new Date(now - 1_800_000).toISOString();

    const activeAlerts: Record<string, Alert> = {
      a: makeAlert('a', recentTime),
      b: makeAlert('b', recentTime),
      c: makeAlert('c', recentTime),
    };

    render(() => <OverviewTab {...defaultProps({ activeAlerts })} />);

    const label = screen.getByText('Triggered (24h)');
    const statValue = label
      .closest('tr')
      ?.querySelector('[data-testid="alert-overview-stat-value"]');
    expect(statValue?.textContent).toBe('3');
  });

  it('excludes future-dated alerts (clock skew)', () => {
    const futureTime = new Date(Date.now() + 3_600_000).toISOString();

    const activeAlerts: Record<string, Alert> = {
      future: makeAlert('future', futureTime),
    };

    render(() => <OverviewTab {...defaultProps({ activeAlerts })} />);

    const label = screen.getByText('Triggered (24h)');
    const statValue = label
      .closest('tr')
      ?.querySelector('[data-testid="alert-overview-stat-value"]');
    expect(statValue?.textContent).toBe('0');
  });

  it('ages out alerts as the tick advances past 24h', () => {
    // Alert started 23h 59m ago — just inside the window
    const almostExpired = new Date(Date.now() - (86_400_000 - 60_000)).toISOString();

    const activeAlerts: Record<string, Alert> = {
      borderline: makeAlert('borderline', almostExpired),
    };

    render(() => <OverviewTab {...defaultProps({ activeAlerts })} />);

    const label = screen.getByText('Triggered (24h)');
    const statValue = label
      .closest('tr')
      ?.querySelector('[data-testid="alert-overview-stat-value"]');
    expect(statValue?.textContent).toBe('1');

    // Advance time by 2 minutes — alert is now 24h 1m old, outside the window
    vi.advanceTimersByTime(120_000);

    expect(statValue?.textContent).toBe('0');
  });

  it('renders acknowledged alert cards through the canonical overview presentation path', () => {
    const recentTime = new Date(Date.now() - 1_800_000).toISOString();

    const activeAlerts: Record<string, Alert> = {
      acknowledged: makeAlert('acknowledged', recentTime, true),
    };

    const { container } = render(() => <OverviewTab {...defaultProps({ activeAlerts })} />);

    expect(screen.getByText('Unacknowledge')).toBeInTheDocument();
    expect(screen.getAllByText('Acknowledged').length).toBeGreaterThan(0);
    expect(container.querySelector('#alert-acknowledged.bg-surface-alt')).toBeTruthy();
  });

  it('localizes stats and active alert card controls without translating source fields', () => {
    setActiveLocale('es');
    const recentTime = new Date(Date.now() - 1_800_000).toISOString();

    const activeAlerts: Record<string, Alert> = {
      recent: makeAlert('recent', recentTime),
    };

    render(() => <OverviewTab {...defaultProps({ activeAlerts })} />);

    expect(screen.getByText('Activadas (24h)')).toBeInTheDocument();
    expect(screen.getByText('Reconocidas')).toBeInTheDocument();
    expect(screen.getByText('Alertas activas')).toBeInTheDocument();
    expect(screen.getByText('Reconocer')).toBeInTheDocument();
    expect(screen.getByText('Linea de tiempo')).toBeInTheDocument();
    expect(screen.getByText('en node1')).toBeInTheDocument();
    expect(screen.getByText('VM recent')).toBeInTheDocument();
    expect(screen.getByText('High CPU on VM recent')).toBeInTheDocument();
  });
});
