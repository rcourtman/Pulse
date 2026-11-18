import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { createAlertAcknowledgementRequests } from '@/stores/alertAcknowledgementRequests';
import type { Alert } from '@/types/api';

import { OverviewTab } from '../OverviewTab';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
  A: (props: Record<string, unknown>) => props.children,
}));
vi.mock('@/api/alerts', () => ({
  AlertsAPI: { getDeliveryDiagnoses: async () => [] },
}));
vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));

// Same resource id, independent spec ids, thresholds and incident start times.
const gateway: Alert = {
  id: 'pmg-lab::pmg-lab-oldest-message',
  canonicalSpecId: 'pmg-lab-oldest-message',
  resourceId: 'pmg-lab',
  resourceName: 'mail lab',
  node: 'https://pmg.example.invalid',
  instance: 'mail lab',
  type: 'message-age',
  level: 'warning',
  message: 'PMG mail lab has messages queued for 20 minutes (threshold: 15 minutes)',
  threshold: 15,
  value: 20,
  startTime: '2026-11-18T12:00:00Z',
  acknowledged: false,
  metadata: { resourceType: 'pmg' },
};
const peer: Alert = {
  ...gateway,
  id: 'pmg-lab::pmg-lab-peer-oldest-message',
  canonicalSpecId: 'pmg-lab-peer-oldest-message',
  node: 'peer',
  nodeDisplayName: 'Relay station',
  message: 'PMG node peer on mail lab has messages queued for 20 minutes (threshold: 9 min)',
  threshold: 9,
  startTime: '2026-11-18T12:01:00Z',
};

function renderOverview(...alerts: Alert[]) {
  const index = (rows: Alert[]) => Object.fromEntries(rows.map((alert) => [alert.id, alert]));
  const [activeAlerts, setActiveAlerts] = createSignal(index(alerts));
  const [showAcknowledged, setShowAcknowledged] = createSignal(false);
  render(() => (
    <OverviewTab
      activeAlerts={activeAlerts()}
      overrides={[]}
      updateAlert={vi.fn()}
      acknowledgementRequests={createAlertAcknowledgementRequests()}
      showQuickTip={() => false}
      dismissQuickTip={vi.fn()}
      showAcknowledged={showAcknowledged}
      setShowAcknowledged={setShowAcknowledged}
      alertsDisabled={() => false}
    />
  ));
  return { replace: (...rows: Alert[]) => setActiveAlerts(index(rows)) };
}

describe('OverviewTab PMG message age', () => {
  afterEach(cleanup);

  it.each([
    { lead: peer, companion: gateway, label: 'Gateway-wide' },
    {
      lead: { ...gateway, startTime: '2026-11-18T12:02:00Z' },
      companion: peer,
      label: 'Node-specific',
    },
  ])('keeps both scope readings when $lead.canonicalSpecId leads', ({ lead, companion, label }) => {
    renderOverview(lead, companion);

    expect(screen.getByText(lead.message).closest('[id^="alert-"]')?.id).toBe(`alert-${lead.id}`);
    const secondaryCard = screen.getByText(companion.message).closest('[id^="alert-"]')!;
    expect(secondaryCard.id).toBe(`alert-${companion.id}`);
    expect(within(secondaryCard as HTMLElement).getByText(label)).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Acknowledge' })).toHaveLength(2);
    expect(screen.getAllByRole('button', { name: 'Acknowledge all (2)' })).toHaveLength(2);
    expect(screen.queryByRole('button', { name: '+1 related' })).not.toBeInTheDocument();
  });

  it('does not surface acknowledged or closed incidents excluded by the current view', () => {
    const page = renderOverview({ ...gateway, acknowledged: true }, peer);
    expect(screen.queryByText(gateway.message)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Show acknowledged' }));
    const card = screen.getByText(gateway.message).closest<HTMLElement>('[id^="alert-"]')!;
    expect(within(card).getByText('Acknowledged')).toBeInTheDocument();

    page.replace(peer);
    expect(screen.queryByText(gateway.message)).not.toBeInTheDocument();
    expect(screen.getByText(peer.message)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '+1 related' })).not.toBeInTheDocument();
  });

  it('leaves other same-resource alerts collapsed', () => {
    const queue: Alert = { ...gateway, type: 'queue-total', id: 'pmg-lab-queue', message: 'Queue' };
    renderOverview(gateway, peer, queue);

    expect(screen.getByText(gateway.message)).toBeInTheDocument();
    expect(screen.queryByText(queue.message)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '+1 related' }));
    expect(screen.getByText(queue.message)).toBeInTheDocument();
  });

  it('still folds alerts for multiple devices on the same ZFS resource', () => {
    const disk: Alert = {
      ...gateway,
      type: 'zfs-device',
      id: 'pool-disk-1',
      resourceId: 'pool',
      message: 'First disk failed',
      metadata: { resourceType: 'storage' },
    };
    const otherDisk: Alert = { ...disk, id: 'pool-disk-2', message: 'Second disk failed' };
    renderOverview(disk, otherDisk);

    expect(screen.queryByText(otherDisk.message)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '+1 related' }));
    expect(screen.getByText(otherDisk.message)).toBeInTheDocument();
  });
});
