import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { AlertOverviewState } from '../useAlertOverviewState';
import type { AlertIncidentTimelineState } from '../useAlertIncidentTimelineState';
import { makeSystemAlert, SYSTEM_ALERT_TYPES } from '../__fixtures__/systemAlerts';

vi.mock('@solidjs/router', () => ({
  A: (props: any) => (
    <a href={props.href} title={props.title}>
      {props.children}
    </a>
  ),
}));
vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));
vi.mock('../ResourceMonitoringPolicyAction', () => ({
  ResourceMonitoringPolicyAction: () => <button>Resource monitoring policy</button>,
}));
vi.mock('../AlertSnoozeAction', () => ({ AlertSnoozeAction: () => null }));
import { AlertOverviewAlertCard } from '../AlertOverviewAlertCard';

const acknowledge = vi.fn();
const toggleTimeline = vi.fn();
const state = {
  processingAlerts: () => new Set(),
  snoozeProcessingAlerts: () => new Set(),
  deliveryDiagnoses: () => ({}),
  handleAlertAcknowledgement: acknowledge,
} as unknown as AlertOverviewState;
const timelineState = {
  expandedIncidents: () => new Set(),
  toggleIncidentTimeline: toggleTimeline,
} as unknown as AlertIncidentTimelineState;
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('Pulse system-alert overview scope', () => {
  it.each(SYSTEM_ALERT_TYPES)('does not link %s to a fictional Proxmox resource', (type) => {
    const alert = makeSystemAlert(type);
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByText('Pulse').closest('a')).toBeNull();
    expect(screen.getByText(alert.message)).toBeInTheDocument();
    expect(screen.queryByText('Resource monitoring policy')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Acknowledge' }));
    expect(acknowledge).toHaveBeenCalledWith(alert);
    fireEvent.click(screen.getByRole('button', { name: 'Timeline' }));
    expect(toggleTimeline).toHaveBeenCalledWith(alert.id, alert.id, alert.startTime);
  });

  it('ignores conflicting metric and resource hints on a marked system alert', () => {
    const alert = makeSystemAlert('future-system-condition', {
      id: 'legacy-id',
      resourceId: 'vm-wrong',
      threshold: 80,
      metadata: { systemAlert: true, resourceType: 'vm' },
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByText('Pulse').closest('a')).toBeNull();
    expect(screen.queryByText(/limit:/)).toBeNull();
    expect(screen.queryByText('Resource monitoring policy')).toBeNull();
  });

  it('preserves the actual link and metric limit for a monitored resource called Pulse', () => {
    const alert = makeSystemAlert('cpu', {
      id: 'resource-cpu',
      resourceId: 'vm-pulse',
      value: 92,
      threshold: 80,
      metadata: undefined,
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByRole('link', { name: 'Pulse' })).toHaveAttribute(
      'href',
      '/proxmox/overview',
    );
    expect(screen.getByText('limit: 80%')).toBeInTheDocument();
    expect(screen.getByText('Resource monitoring policy')).toBeInTheDocument();
  });
});
