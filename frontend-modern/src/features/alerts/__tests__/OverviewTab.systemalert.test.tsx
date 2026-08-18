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

// Exactly the payload internal/alerts marshals for a raised system alert:
// no resourceId, no node, resourceName "Pulse". Captured from the Go side
// rather than hand-written, because the point of this test is that the card
// copes with an alert that has no monitored resource behind it.
function systemAlert(): Alert {
  return {
    id: 'pulse-system-notification-delivery',
    type: 'notification-delivery',
    level: 'warning',
    resourceId: '',
    resourceName: 'Pulse',
    node: '',
    instance: '',
    message:
      'Alert notifications are not reaching their destinations. 4 failed deliveries were not delivered. Check destination credentials and settings under Alerts, Notifications.',
    startTime: new Date(Date.now() - 90_000).toISOString(),
    acknowledged: false,
    metadata: {
      systemAlert: true,
      systemAlertType: 'notification-delivery',
      deliveryStatus: 'degraded',
      failedDeliveries: 4,
    },
  } as unknown as Alert;
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

describe('OverviewTab system-scoped alerts', () => {
  beforeEach(() => {
    setActiveLocale(DEFAULT_LOCALE);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('renders a resource-less system alert without inventing a resource', () => {
    const alert = systemAlert();
    render(
      () => OverviewTab(defaultProps({ activeAlerts: { [alert.id]: alert } }) as never) as never,
    );

    // Named as Pulse itself rather than a monitored resource.
    expect(screen.getByText('Pulse')).toBeTruthy();
    // The hyphenated type is title-cased for display.
    expect(screen.getByText(/Notification Delivery/i)).toBeTruthy();
    // The operator has, by definition, not been notified, so the message has to
    // stand on its own and say where to go.
    expect(screen.getByText(/not reaching their destinations/i)).toBeTruthy();
    expect(screen.getByText(/Alerts, Notifications/i)).toBeTruthy();
  });

  it('does not render a node line for an alert with no node', () => {
    const alert = systemAlert();
    const { container } = render(
      () => OverviewTab(defaultProps({ activeAlerts: { [alert.id]: alert } }) as never) as never,
    );

    // Resource alerts render an "on <node>" line. A system alert has no node,
    // so that affordance must be skipped rather than rendered empty.
    expect(container.textContent).not.toMatch(/\bon\s*$/m);
    expect(container.textContent).not.toContain('on undefined');
    expect(container.textContent).not.toContain('undefined');
  });
});
