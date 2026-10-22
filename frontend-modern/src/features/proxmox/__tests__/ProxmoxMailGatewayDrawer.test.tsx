import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { PMGInstance } from '@/types/api';
import type { Resource } from '@/types/resource';
import { ProxmoxMailGatewayMailFlow } from '../ProxmoxMailGatewayDrawer';

const apiFetchMock = vi.hoisted(() => vi.fn());

vi.mock('@/utils/apiClient', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/utils/apiClient')>()),
  apiFetch: (...args: unknown[]) => apiFetchMock(...args),
}));

const gateway = {
  id: 'pmg-unified-eu',
  type: 'pmg',
  name: 'mail-gateway-eu',
  displayName: 'mail-gateway-eu',
  platformType: 'proxmox-pmg',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  pmg: { instanceId: 'pmg-main', version: '8.1.2', nodeCount: 1 },
} as unknown as Resource;

const longDomain = 'customer-support-notifications.eu-west.example-holdings.co.uk';

const instance: PMGInstance = {
  id: 'pmg-main',
  name: 'mail-gateway-eu',
  host: 'https://pmg.example:8006',
  status: 'online',
  version: '8.1.2',
  connectionHealth: 'healthy',
  lastSeen: '2026-10-22T10:00:00Z',
  lastUpdated: '2026-10-22T10:00:00Z',
  mailStats: {
    timeframe: 'day',
    countTotal: 1500,
    countIn: 1200,
    countOut: 300,
    spamIn: 140,
    spamOut: 2,
    virusIn: 3,
    virusOut: 0,
    bouncesIn: 12,
    bouncesOut: 4,
    bytesIn: 52_428_800,
    bytesOut: 10_485_760,
    greylistCount: 40,
    junkIn: 9,
    averageProcessTimeMs: 412.4,
    rblRejects: 77,
    pregreetRejects: 5,
    updatedAt: '2026-10-22T10:00:00Z',
  },
  quarantine: { spam: 21, virus: 1, attachment: 2, blacklisted: 0 },
  spamDistribution: [
    { score: '0', count: 900 },
    { score: '5', count: 30 },
    { score: '10+', count: 8 },
  ],
  relayDomains: [{ domain: 'example.com', comment: 'Primary' }, { domain: 'example.org' }],
  domainStats: [{ domain: longDomain, mailCount: 800, spamCount: 90, virusCount: 1, bytes: 1024 }],
  nodes: [
    {
      name: 'pmg-node-1',
      status: 'online',
      role: 'master',
      uptime: 86_400,
      loadAvg: '0.12',
      queueStatus: {
        active: 1,
        deferred: 2,
        hold: 0,
        incoming: 0,
        total: 3,
        oldestAge: 1800,
        updatedAt: '2026-10-22T10:00:00Z',
      },
    },
  ],
};

const respond = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

describe('ProxmoxMailGatewayMailFlow', () => {
  beforeEach(() => apiFetchMock.mockReset());
  afterEach(cleanup);

  it('shows every mail-flow figure as content cards under the drawer header', async () => {
    apiFetchMock.mockResolvedValue(respond({ data: [instance], meta: { total: 1 } }));

    render(() => <ProxmoxMailGatewayMailFlow resource={gateway} />);

    const flow = within(await screen.findByTestId('pmg-mail-flow-section'));
    expect(apiFetchMock).toHaveBeenCalledWith('/api/pmg/instances?id=pmg-main');
    expect(flow.getByText('Mail flow (day)')).toBeInTheDocument();
    expect(flow.getByText('1,200 / 300')).toBeInTheDocument();
    expect(flow.getByText('140 / 2')).toBeInTheDocument();
    expect(flow.getByText('412ms')).toBeInTheDocument();
    expect(flow.getByText('RBL rejects').closest('tr')).toHaveTextContent('77');
    expect(flow.getByRole('img', { name: 'Inbound mail disposition breakdown' })).toBeVisible();

    const quarantine = within(screen.getByTestId('pmg-quarantine-section'));
    expect(quarantine.getByText('Attachment')).toBeInTheDocument();
    expect(quarantine.getByText('21')).toBeInTheDocument();

    expect(
      within(screen.getByTestId('pmg-spam-scores-section')).getByRole('img', {
        name: 'Spam score distribution',
      }),
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId('pmg-relay-domains-section')).getByText(
        'example.com (Primary), example.org',
      ),
    ).toBeInTheDocument();

    const nodes = within(screen.getByTestId('pmg-cluster-nodes'));
    expect(nodes.getByText('pmg-node-1')).toBeInTheDocument();
    expect(nodes.getByText('1/2/0/0')).toBeInTheDocument();

    // Domains are compared and copied, so they wrap rather than truncate.
    const domain = within(screen.getByTestId('pmg-top-domains')).getByText(longDomain);
    expect(domain).toHaveClass('whitespace-normal', 'wrap-anywhere');
    expect(domain).not.toHaveClass('truncate');

    expect(screen.getByTestId('pmg-mail-flow-facts')).toHaveTextContent('8.1.2');
    expect(screen.getByTestId('pmg-mail-flow-facts')).toHaveTextContent('https://pmg.example:8006');
    // The drawer around this body owns the heading, status and alerts.
    expect(screen.queryByRole('heading', { level: 2 })).toBeNull();
    expect(screen.queryByTestId('drawer-attention-section')).toBeNull();
  });

  it('shows a shared error notice whose retry reads the gateway again', async () => {
    apiFetchMock.mockResolvedValueOnce(respond({ error: 'boom' }, 502));

    render(() => <ProxmoxMailGatewayMailFlow resource={gateway} />);

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Could not load mail flow for mail-gateway-eu.');

    apiFetchMock.mockResolvedValueOnce(respond({ data: [instance], meta: { total: 1 } }));
    await fireEvent.click(
      within(alert).getByRole('button', { name: 'Retry loading mail flow for mail-gateway-eu' }),
    );

    expect(await screen.findByTestId('pmg-mail-flow-section')).toBeInTheDocument();
    expect(screen.queryByRole('alert')).toBeNull();
    expect(apiFetchMock).toHaveBeenCalledTimes(2);
  });

  it('says plainly when the gateway has not reported mail flow', async () => {
    apiFetchMock.mockResolvedValue(respond({ data: [], meta: { total: 0 } }));

    render(() => <ProxmoxMailGatewayMailFlow resource={gateway} />);

    expect(
      await screen.findByText('This gateway has not reported mail flow yet.'),
    ).toBeInTheDocument();
  });
});
