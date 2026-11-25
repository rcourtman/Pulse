import { Suspense } from 'solid-js';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import type { PMGInstance } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  ProxmoxMailGatewayMailFlow,
  describeMailGatewayDegradation,
} from '../ProxmoxMailGatewayDrawer';

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
  beforeEach(() => {
    apiFetchMock.mockReset();
    resetCreateNonSuspendingQueryCacheForTest();
  });
  afterEach(cleanup);

  it('does not attribute an aggregate warning from a quiet poll to the gateway', () => {
    const stale = {
      ...gateway,
      status: 'degraded',
      sourceStatus: { pmg: { status: 'stale' } },
      pmg: { ...gateway.pmg!, connectionHealth: 'healthy' },
    } satisfies Resource;
    expect(describeMailGatewayDegradation(stale)).toEqual([]);
    expect(
      describeMailGatewayDegradation(stale, { ...instance, status: 'warning', nodes: [] }),
    ).toEqual([]);
    expect(
      describeMailGatewayDegradation(stale, {
        ...instance,
        connectionHealth: 'degraded',
        nodes: [],
      }),
    ).toEqual([expect.stringContaining('PMG API reported degraded connection health')]);
  });

  it('shows every mail-flow figure as content cards under the drawer header', async () => {
    apiFetchMock.mockResolvedValue(respond({ data: [instance], meta: { total: 1 } }));

    render(() => <ProxmoxMailGatewayMailFlow resource={gateway} />);

    const flow = within(await screen.findByTestId('pmg-mail-flow-section'));
    expect(apiFetchMock).toHaveBeenCalledWith(
      '/api/pmg/instances?id=pmg-main',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(flow.getByText('Mail flow (day)')).toBeInTheDocument();
    expect(flow.getByText('1,200 / 300')).toBeInTheDocument();
    expect(flow.getByText('140 / 2')).toBeInTheDocument();
    expect(flow.getByText('412ms')).toBeInTheDocument();
    expect(flow.getByText('RBL rejects').closest('tr')).toHaveTextContent('77');
    expect(flow.getByRole('img', { name: 'Inbound mail disposition breakdown' })).toBeVisible();

    const quarantine = within(screen.getByTestId('pmg-quarantine-section'));
    expect(quarantine.getAllByText('Attachment').length).toBeGreaterThanOrEqual(2);
    expect(quarantine.getAllByText('21').length).toBeGreaterThanOrEqual(2);
    expect(quarantine.getByText('87.5%')).toBeInTheDocument();

    const spamSection = screen.getByTestId('pmg-spam-scores-section');
    expect(
      within(spamSection).getByRole('img', {
        name: 'Spam score distribution',
      }),
    ).toBeInTheDocument();
    const bucketWrapper = spamSection.querySelector('[data-spam-histogram-bucket="0"]');
    expect(bucketWrapper).toHaveClass('h-full', 'justify-end');
    const bar = spamSection.querySelector('[data-spam-histogram-bar="0"]') as HTMLElement | null;
    expect(bar?.style.height).toBe('100%');
    expect(
      within(screen.getByTestId('pmg-relay-domains-section')).getByText(
        'example.com (Primary), example.org',
      ),
    ).toBeInTheDocument();

    const nodes = within(screen.getByTestId('pmg-cluster-nodes'));
    expect(nodes.getByText('pmg-node-1')).toBeInTheDocument();
    expect(nodes.getByText('1/2/0/0')).toBeInTheDocument();
    expect(nodes.getByTestId('pmg-queue-legend')).toHaveTextContent(
      'Queue: active / deferred / hold / incoming',
    );

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

  it('renders inline loading without suspending a parent Suspense boundary', async () => {
    let resolveFetch!: (value: Response) => void;
    apiFetchMock.mockReturnValue(
      new Promise<Response>((resolve) => {
        resolveFetch = resolve;
      }),
    );

    render(() => (
      <Suspense fallback={<div>Loading view...</div>}>
        <ProxmoxMailGatewayMailFlow resource={gateway} />
      </Suspense>
    ));

    expect(screen.getByText('Loading mail flow')).toBeInTheDocument();
    expect(screen.queryByText('Loading view...')).toBeNull();

    resolveFetch(respond({ data: [instance], meta: { total: 1 } }));
    expect(await screen.findByTestId('pmg-mail-flow-section')).toBeInTheDocument();
    expect(screen.queryByText('Loading view...')).toBeNull();
  });

  it('explains concrete degradation causes for degraded gateways', async () => {
    const degradedGateway = {
      ...gateway,
      id: 'pmg-unified-us',
      name: 'mail-gateway-us',
      displayName: 'mail-gateway-us',
      status: 'degraded',
      pmg: {
        instanceId: 'pmg-us',
        version: '8.1.2',
        nodeCount: 2,
        connectionHealth: 'degraded',
        queueDeferred: 7,
        queueHold: 1,
        queueTotal: 121,
      },
    } as unknown as Resource;
    const degradedInstance: PMGInstance = {
      ...instance,
      id: 'pmg-us',
      name: 'mail-gateway-us',
      status: 'degraded',
      connectionHealth: 'degraded',
      nodes: [
        {
          ...instance.nodes![0],
          name: 'pmg-us-node-2',
          status: 'degraded',
          queueStatus: {
            active: 113,
            deferred: 7,
            hold: 1,
            incoming: 0,
            total: 121,
            oldestAge: 1800,
            updatedAt: '2026-10-22T10:00:00Z',
          },
        },
      ],
    };

    expect(describeMailGatewayDegradation(degradedGateway, degradedInstance)).toEqual([
      'Cluster node pmg-us-node-2 is degraded.',
      'Mail queue backlog: 7 deferred, 1 on hold (121 total queued).',
    ]);

    apiFetchMock.mockResolvedValue(respond({ data: [degradedInstance], meta: { total: 1 } }));
    render(() => <ProxmoxMailGatewayMailFlow resource={degradedGateway} />);

    const notice = await screen.findByTestId('pmg-degradation-notice');
    expect(notice).toHaveTextContent('Cluster node pmg-us-node-2 is degraded.');
    expect(notice).toHaveTextContent('7 deferred');
  });
});
