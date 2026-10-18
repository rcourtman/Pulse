import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import type { Alert, AlertDeliveryDiagnosis } from '@/types/api';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { METRIC_ALERT_STATUS_STALE_MS } from '../metricAlertPresentation';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));

const getDeliveryDiagnoses = vi.fn<() => Promise<AlertDeliveryDiagnosis[]>>();

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    get getDeliveryDiagnoses() {
      return getDeliveryDiagnoses;
    },
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

const OPENED_AT = Date.parse('2026-10-07T12:00:00Z');
// Between the shared clock's first and second ticks, and before the overview's
// own minute.
const BETWEEN_TICKS_MS = RELATIVE_TIME_TICK_MS + 15_000;

function cpuAlert(id: string, startTime: number, observedAt: number): Alert {
  return {
    id,
    resourceId: `vm-${id}`,
    resourceName: `VM ${id}`,
    type: 'cpu',
    level: 'warning',
    message: `CPU on VM ${id} at 92%`,
    value: 92,
    threshold: 80,
    startTime: new Date(startTime).toISOString(),
    lastSeen: new Date(observedAt).toISOString(),
    acknowledged: false,
    node: 'node1',
    metricStatus: {
      phase: 'breaching',
      value: 91,
      unit: '%',
      observedAt: new Date(observedAt).toISOString(),
      trigger: 80,
      recovery: 75,
    },
  } as Alert;
}

function renderOverview() {
  const [activeAlerts, setActiveAlerts] = createSignal<Record<string, Alert>>({});
  render(() => (
    <OverviewTab
      overrides={[]}
      activeAlerts={activeAlerts()}
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

const triggered24h = () =>
  screen
    .getByText('Triggered (24h)')
    .closest('[data-alert-overview-stat]')
    ?.querySelector('[data-testid="alert-overview-stat-value"]')?.textContent;

describe('Alert overview card clock', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
    getDeliveryDiagnoses.mockReset();
    getDeliveryDiagnoses.mockResolvedValue([]);
    vi.useFakeTimers();
    vi.setSystemTime(OPENED_AT);
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('measures a card that mounts between ticks from the wall clock', () => {
    const setActiveAlerts = renderOverview();
    vi.advanceTimersByTime(BETWEEN_TICKS_MS);

    // Raised 70 seconds ago; last evaluated just past the stale cut-off. Read
    // from the time of the last tick, the card would call it "this minute"
    // and lead with the old reading as the live one.
    const now = Date.now();
    setActiveAlerts({
      late: cpuAlert('late', now - 70_000, now - METRIC_ALERT_STATUS_STALE_MS - 5_000),
    });

    expect(screen.getByText('1 min. ago')).toBeInTheDocument();
    expect(screen.getByText('Last reading: CPU 91%, 10 mins ago')).toBeInTheDocument();
  });

  it('turns stale at the next shared tick after the cut-off with no data change', () => {
    const setActiveAlerts = renderOverview();
    vi.advanceTimersByTime(BETWEEN_TICKS_MS);

    // The reading goes stale 5 seconds after the shared clock's second tick.
    const staleAt = OPENED_AT + 2 * RELATIVE_TIME_TICK_MS + 5_000;
    const now = Date.now();
    setActiveAlerts({ held: cpuAlert('held', now, staleAt - METRIC_ALERT_STATUS_STALE_MS) });
    expect(screen.getByText('CPU 91%, above the 80% alert level')).toBeInTheDocument();

    vi.advanceTimersByTime(OPENED_AT + 2 * RELATIVE_TIME_TICK_MS - Date.now());
    expect(screen.getByText('CPU 91%, above the 80% alert level')).toBeInTheDocument();

    // The next tick lands within one shared tick of the cut-off, not on the
    // overview's next minute.
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
    expect(screen.queryByText('CPU 91%, above the 80% alert level')).toBeNull();
    expect(screen.getByText('Last reading: CPU 91%, 10 mins ago')).toBeInTheDocument();
  });

  it('drops an alert from the 24h count at the next shared tick after it turns 24h old', () => {
    const setActiveAlerts = renderOverview();
    vi.advanceTimersByTime(BETWEEN_TICKS_MS);

    const leavesWindowAt = OPENED_AT + 2 * RELATIVE_TIME_TICK_MS + 5_000;
    setActiveAlerts({ old: cpuAlert('old', leavesWindowAt - 86_400_000, Date.now()) });
    expect(triggered24h()).toBe('1');

    vi.advanceTimersByTime(OPENED_AT + 3 * RELATIVE_TIME_TICK_MS - Date.now());
    expect(triggered24h()).toBe('0');
  });
});
