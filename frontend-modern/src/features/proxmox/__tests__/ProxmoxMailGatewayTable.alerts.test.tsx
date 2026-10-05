import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { ProxmoxMailGatewayTable } from '../ProxmoxMailGatewayTable';

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

  it('marks an online gateway with an open alert and says why, keyed by its instance id', async () => {
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

    const reason = document.querySelector('[data-mail-gateway-alert-reason]');
    expect(reason?.textContent).toBe('Message Age');
    expect(reason?.getAttribute('title')).toContain('queued for 41 minutes');
    expect(document.querySelectorAll('[data-mail-gateway-alert-reason]')).toHaveLength(1);

    fireEvent.click(screen.getByRole('button', { name: /mail-gateway-eu/ }));
    expect(
      await screen.findByText(/queued for 41 minutes \(threshold: 30 minutes\)/),
    ).toBeVisible();
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

    expect(document.querySelector('[data-mail-gateway-alert-reason]')).toBeNull();
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

    expect(document.querySelector('[data-mail-gateway-alert-reason]')).toBeNull();
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

    const reason = document.querySelector('[data-mail-gateway-alert-reason]');
    expect(reason?.className).toContain('text-red-600');
    const row = reason?.closest('tr');
    expect(row?.querySelector('[class*="bg-red"]')).not.toBeNull();
  });
});
