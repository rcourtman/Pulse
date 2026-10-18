import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { ProxmoxMailGatewayTable, mailGatewayAlertColumn } from '../ProxmoxMailGatewayTable';

const activeAlerts: Record<string, Alert> = {};

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ detectionEnabled: () => true }),
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetch: vi.fn(async () => new Response(JSON.stringify(null), { status: 200 })),
}));

const gateway = (id: string, name: string, instanceId: string): Resource =>
  ({
    id,
    type: 'pmg',
    name,
    displayName: name,
    platformType: 'proxmox-pmg',
    sourceType: 'api',
    status: 'online',
    lastSeen: Date.now(),
    canonicalIdentity: { primaryId: `agent:${instanceId}`, aliases: [instanceId, name] },
    pmg: { instanceId, nodeCount: 1, mailCountTotal: 10, queueTotal: 3, queueDeferred: 1 },
  }) as unknown as Resource;

describe('ProxmoxMailGatewayTable open alerts', () => {
  afterEach(() => {
    cleanup();
    for (const key of Object.keys(activeAlerts)) delete activeAlerts[key];
  });

  it('marks the alerting number on an online gateway, keyed by its instance id', async () => {
    activeAlerts['pmg-main::oldest'] = {
      id: 'pmg-main::oldest',
      type: 'message-age',
      level: 'warning',
      resourceId: 'pmg-main',
      resourceName: 'mail-gateway-eu',
      node: 'https://pmg.example:8006',
      message: 'PMG mail-gateway-eu has messages queued for 41 minutes (threshold: 30 minutes)',
      acknowledged: false,
    } as unknown as Alert;

    render(() => (
      <ProxmoxMailGatewayTable
        resources={[
          gateway('pmg-unified-eu', 'mail-gateway-eu', 'pmg-main'),
          gateway('pmg-unified-us', 'mail-gateway-us', 'pmg-edge'),
        ]}
        emptyTitle="No gateways"
        emptyDescription="No gateways"
      />
    ));

    // A message-age alert is about the queue, so the Queue number carries it
    // and the row stays single-line.
    const queue = document.querySelector('[data-mail-gateway-alert-column="queue"]');
    expect(queue?.getAttribute('title')).toContain('queued for 41 minutes');
    expect(queue?.className).toContain('text-amber-700');
    expect(queue?.querySelector('.sr-only')?.textContent).toContain('Message Age:');
    expect(document.querySelectorAll('[data-mail-gateway-alert-column]')).toHaveLength(1);
    const nameCell = screen.getByText('mail-gateway-eu').closest('td');
    expect(nameCell?.querySelector('[data-mail-gateway-alert-summary]')).toHaveClass('sr-only');
    expect(nameCell).not.toHaveTextContent('Message Age');

    fireEvent.click(screen.getByRole('button', { name: /mail-gateway-eu/ }));
    const drawerMatches = (
      await screen.findAllByText(/queued for 41 minutes \(threshold: 30 minutes\)/)
    ).filter((element) => !element.closest('.sr-only'));
    expect(drawerMatches.length).toBeGreaterThan(0);
    expect(drawerMatches[0]).toBeVisible();
    expect(screen.getByText('Needs attention')).toBeInTheDocument();
  });

  it('ignores acknowledged alerts', () => {
    activeAlerts['pmg-main::oldest'] = {
      id: 'pmg-main::oldest',
      type: 'message-age',
      level: 'warning',
      resourceId: 'pmg-main',
      resourceName: 'mail-gateway-eu',
      node: 'https://pmg.example:8006',
      message: 'queued',
      acknowledged: true,
    } as unknown as Alert;

    render(() => (
      <ProxmoxMailGatewayTable
        resources={[gateway('pmg-unified-eu', 'mail-gateway-eu', 'pmg-main')]}
        emptyTitle="No gateways"
        emptyDescription="No gateways"
      />
    ));

    expect(document.querySelector('[data-mail-gateway-alert-column]')).toBeNull();
    expect(document.querySelector('[data-mail-gateway-alert-summary]')).toBeNull();
  });

  it("does not pin another gateway's node alert on a row that shares its node name", () => {
    activeAlerts['pmg-edge::node'] = {
      id: 'pmg-edge::node',
      type: 'message-age',
      level: 'warning',
      resourceId: 'pmg-edge',
      resourceName: 'mail-gateway-us',
      node: 'mail-gateway-eu',
      message: 'queued on edge',
      acknowledged: false,
    } as unknown as Alert;

    render(() => (
      <ProxmoxMailGatewayTable
        resources={[gateway('pmg-unified-eu', 'mail-gateway-eu', 'pmg-main')]}
        emptyTitle="No gateways"
        emptyDescription="No gateways"
      />
    ));

    expect(document.querySelector('[data-mail-gateway-alert-column]')).toBeNull();
  });

  it('reads red for a critical open alert', () => {
    activeAlerts['pmg-main::queue'] = {
      id: 'pmg-main::queue',
      type: 'queue-total',
      level: 'critical',
      resourceId: 'pmg-main',
      resourceName: 'mail-gateway-eu',
      node: 'https://pmg.example:8006',
      message: 'Queue above critical',
      acknowledged: false,
    } as unknown as Alert;

    render(() => (
      <ProxmoxMailGatewayTable
        resources={[gateway('pmg-unified-eu', 'mail-gateway-eu', 'pmg-main')]}
        emptyTitle="No gateways"
        emptyDescription="No gateways"
      />
    ));

    const queue = document.querySelector('[data-mail-gateway-alert-column="queue"]');
    expect(queue?.className).toContain('text-red-600');
    const row = queue?.closest('tr');
    expect(row?.querySelector('[class*="bg-red"]')).not.toBeNull();
  });

  it('puts a deferred-queue alert on the Deferred number', () => {
    activeAlerts['pmg-main::deferred'] = {
      id: 'pmg-main::deferred',
      type: 'queue-deferred',
      level: 'warning',
      resourceId: 'pmg-main',
      resourceName: 'mail-gateway-eu',
      node: 'https://pmg.example:8006',
      message: 'Deferred queue above warning',
      acknowledged: false,
    } as unknown as Alert;

    render(() => (
      <ProxmoxMailGatewayTable
        resources={[gateway('pmg-unified-eu', 'mail-gateway-eu', 'pmg-main')]}
        emptyTitle="No gateways"
        emptyDescription="No gateways"
      />
    ));

    const deferred = document.querySelector('[data-mail-gateway-alert-column="deferred"]');
    expect(deferred?.getAttribute('title')).toBe('Deferred queue above warning');
    expect(document.querySelector('[data-mail-gateway-alert-column="queue"]')).toBeNull();
  });

  it('marks inbound spam anomalies on Spam but leaves outbound ones to the dot and drawer', () => {
    activeAlerts['pmg-main-anomaly-spamIn'] = {
      id: 'pmg-main-anomaly-spamIn',
      type: 'anomaly-spamIn',
      level: 'warning',
      resourceId: 'pmg-main',
      resourceName: 'mail-gateway-eu',
      message: 'Inbound spam is 4x its usual rate',
      acknowledged: false,
    } as unknown as Alert;
    activeAlerts['pmg-main-anomaly-spamOut'] = {
      id: 'pmg-main-anomaly-spamOut',
      type: 'anomaly-spamOut',
      level: 'warning',
      resourceId: 'pmg-main',
      resourceName: 'mail-gateway-eu',
      message: 'Outbound spam is 6x its usual rate',
      acknowledged: false,
    } as unknown as Alert;

    render(() => (
      <ProxmoxMailGatewayTable
        resources={[gateway('pmg-unified-eu', 'mail-gateway-eu', 'pmg-main')]}
        emptyTitle="No gateways"
        emptyDescription="No gateways"
      />
    ));

    // jsdom renders the compact layout, where Spam is hidden, so the column
    // mapping itself is asserted below and the row keeps the full summary.
    expect(document.querySelector('[data-mail-gateway-alert-summary]')?.textContent).toContain(
      'Outbound spam',
    );
  });

  it('maps every mail gateway alert type to the number it is about', () => {
    expect(mailGatewayAlertColumn('queue-total')).toBe('queue');
    expect(mailGatewayAlertColumn('queue-depth')).toBe('queue');
    expect(mailGatewayAlertColumn('queue')).toBe('queue');
    expect(mailGatewayAlertColumn('queue-hold')).toBe('queue');
    expect(mailGatewayAlertColumn('message-age')).toBe('queue');
    expect(mailGatewayAlertColumn('queue-deferred')).toBe('deferred');
    expect(mailGatewayAlertColumn('quarantine-spam')).toBe('quarantine');
    expect(mailGatewayAlertColumn('quarantine-virus')).toBe('quarantine');
    expect(mailGatewayAlertColumn('anomaly-spamIn')).toBe('spam');
    expect(mailGatewayAlertColumn('anomaly-virusIn')).toBe('virus');
    // Outbound anomalies and offline have no column of their own.
    expect(mailGatewayAlertColumn('anomaly-spamOut')).toBeNull();
    expect(mailGatewayAlertColumn('anomaly-virusOut')).toBeNull();
    expect(mailGatewayAlertColumn('offline')).toBeNull();
  });
});
