import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createComponent, createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { apiFetch } from '@/utils/apiClient';
import { ProxmoxMailGatewayDrawer } from '../ProxmoxMailGatewayDrawer';

import { getPlatformTableWeightedColumnWidthStyle } from '@/features/platformPage/sharedPlatformPage';
import {
  MAIL_GATEWAY_COLUMN_WEIGHTS,
  type MailGatewayColumn,
  MAIL_GATEWAY_NARROW_PHONE_COLUMNS,
  MAIL_GATEWAY_NARROW_PHONE_COLUMN_WIDTHS,
  MAIL_GATEWAY_PHONE_COLUMNS,
  MAIL_GATEWAY_PHONE_COLUMN_WIDTHS,
} from '../ProxmoxMailGatewayTable';
import mailGatewayDrawerSource from '../ProxmoxMailGatewayDrawer.tsx?raw';
import mailGatewayTableSource from '../ProxmoxMailGatewayTable.tsx?raw';

describe('ProxmoxMailGatewayTable phone presentation', () => {
  it('keeps compact drawer statistics on the shared responsive row contract', () => {
    expect(mailGatewayDrawerSource).toContain('InfoCardKeyValueRow');
    expect(mailGatewayDrawerSource).not.toMatch(
      /class="flex items-baseline justify-between[^\"]*">\s*<span class="text-muted">/,
    );
    expect(
      mailGatewayDrawerSource.match(/class="col-span-2 text-\[11px\] sm:col-span-1"/g),
    ).toHaveLength(3);
  });

  it('keeps gateway names distinguishable with five phone tracks', () => {
    // A sixth track (node count) truncated "mail-gateway-eu" and
    // "mail-gateway-us" to the same text; the count lives in the expansion.
    expect(MAIL_GATEWAY_PHONE_COLUMNS).toEqual(['instance', 'uptime', 'mail', 'queue', 'deferred']);
    expect(MAIL_GATEWAY_PHONE_COLUMN_WIDTHS.instance).toBe(40);
    expect(
      MAIL_GATEWAY_PHONE_COLUMNS.reduce(
        (total, column) => total + MAIL_GATEWAY_PHONE_COLUMN_WIDTHS[column],
        0,
      ),
    ).toBe(100);
  });

  it('demotes node count below 360px while keeping five mail-flow fields', () => {
    expect(MAIL_GATEWAY_NARROW_PHONE_COLUMNS).toEqual([
      'instance',
      'uptime',
      'mail',
      'queue',
      'deferred',
    ]);
    expect(
      MAIL_GATEWAY_NARROW_PHONE_COLUMNS.reduce(
        (total, column) => total + MAIL_GATEWAY_NARROW_PHONE_COLUMN_WIDTHS[column],
        0,
      ),
    ).toBe(100);
  });

  it('gives every desktop column a weighted share instead of the remainder', () => {
    const columns = Object.keys(MAIL_GATEWAY_COLUMN_WEIGHTS) as MailGatewayColumn[];
    expect(columns).toHaveLength(10);
    const widths = columns.map((column) =>
      Number.parseFloat(
        String(
          getPlatformTableWeightedColumnWidthStyle(column, MAIL_GATEWAY_COLUMN_WEIGHTS, columns)
            .width,
        ),
      ),
    );
    // Version, Spam, Virus, and Quarantine used to split ~14% between them,
    // which clipped a three-digit spam count to "3…" on a 1440px screen.
    expect(Math.min(...widths)).toBeGreaterThan(7);
    expect(widths.reduce((total, width) => total + width, 0)).toBeCloseTo(100, 1);
  });

  it('matches gateway alerts across identity aliases and shows the problem in the drawer', () => {
    // Alerts are keyed by the PMG instance id, rows by the unified id.
    expect(mailGatewayTableSource).toContain('getPMGAlertResourceIds(resource)');
    expect(mailGatewayTableSource).toContain(
      'filterPlatformResources(resources, search, status, effectiveStatus)',
    );
    expect(mailGatewayDrawerSource).toContain('<DrawerAttentionSection');
  });

  it('keeps alerting rows single-line by marking the metric the alert is about', () => {
    // Platform table rows stay on one line, so an alert colours its own number
    // rather than stacking a reason under the gateway name.
    expect(mailGatewayTableSource).toContain('<AlertedMetric alerts={columnAlerts(');
    expect(mailGatewayTableSource).not.toContain('data-mail-gateway-alert-reason');
  });
});

vi.mock('@/utils/apiClient', () => ({ apiFetch: vi.fn() }));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});

const INITIAL = '2026-10-15T12:00:00Z';
const NEXT = '2026-10-15T12:00:30Z';
const healthy = {
  id: 'unified-gateway',
  type: 'pmg',
  name: 'gateway',
  status: 'online',
  lastSeen: Date.parse(INITIAL),
  pmg: { instanceId: 'pmg-gateway', lastUpdated: INITIAL, connectionHealth: 'healthy' },
} as Resource;
const failed = {
  ...healthy,
  status: 'warning',
  pmg: { ...healthy.pmg, connectionHealth: 'degraded' },
} as Resource;
const critical = {
  id: 'connection',
  type: 'connection-degraded',
  level: 'critical',
  message: 'Collection failed',
  acknowledged: false,
} as Alert;

function response(status: string, total = 5, node = 'mail-a') {
  return new Response(
    JSON.stringify({
      data: [
        {
          id: 'pmg-gateway',
          name: 'gateway',
          status,
          connectionHealth: status === 'online' ? 'healthy' : 'degraded',
          nodes: [{ name: node, status: 'online', queueStatus: { total } }],
        },
      ],
      meta: { total: 1 },
    }),
    { status: 200 },
  );
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((done, fail) => {
    resolve = done;
    reject = fail;
  });
  return { promise, resolve, reject };
}

function queue(name = 'mail-a') {
  return within(screen.getByText(name).closest('tr')!).getAllByRole('cell')[4].textContent?.trim();
}

function mount(row = healthy, initialAlerts: Alert[] = []) {
  const [state, setState] = createSignal(row);
  const [alerts, setAlerts] = createSignal(initialAlerts);
  render(() =>
    createComponent(ProxmoxMailGatewayDrawer, {
      get instanceRow() {
        return state();
      },
      get alerts() {
        return alerts();
      },
    }),
  );
  return { setState, setAlerts };
}

describe('PMG detail follows accepted resource observations while open', () => {
  it('refreshes a failed full detail when the same open gateway recovers', async () => {
    const recovery = deferred<Response>();
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(response('degraded'))
      .mockReturnValueOnce(recovery.promise);
    const { setState, setAlerts } = mount(failed, [critical]);
    await screen.findByText('mail-a');
    const section = screen.getByRole('region', { name: /gateway/ });
    expect(screen.getByRole('img', { name: 'Needs attention' })).toHaveClass('bg-red-500');
    setState({ ...healthy, pmg: { ...healthy.pmg, lastUpdated: NEXT } });
    setAlerts([]);
    await waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2));
    // A new slim row alone must not erase the adverse detail still being replaced.
    expect(screen.getByRole('img', { name: 'Degraded' })).toHaveClass('bg-amber-500');
    recovery.resolve(response('online', 0));
    await screen.findByRole('img', { name: 'Healthy' });
    expect(screen.getByRole('region', { name: /gateway/ })).toBe(section);
    expect(screen.queryByTestId('drawer-attention-section')).toBeNull();
    expect(queue()).toBe('0—/—/—/—');
    expect(apiFetch).toHaveBeenNthCalledWith(2, '/api/pmg/instances?id=pmg-gateway');
  });

  it('refreshes readings when collection advances without any health change', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(response('online', 5))
      .mockResolvedValueOnce(response('online', 9));
    const { setState } = mount();
    await screen.findByText('mail-a');
    expect(queue()).toBe('5—/—/—/—');
    setState({ ...healthy, pmg: { ...healthy.pmg, lastUpdated: NEXT } });
    await waitFor(() => expect(queue()).toBe('9—/—/—/—'));
    expect(apiFetch).toHaveBeenCalledTimes(2);
  });

  it('deduplicates equal observations, unrelated row changes and alert-only changes', async () => {
    vi.mocked(apiFetch).mockResolvedValue(response('online'));
    const { setState, setAlerts } = mount();
    await screen.findByText('mail-a');
    setState({ ...healthy, name: 'renamed', lastSeen: Date.parse(NEXT), pmg: { ...healthy.pmg } });
    setAlerts([critical]);
    expect(screen.getByRole('img', { name: 'Needs attention' })).toBeInTheDocument();
    setAlerts([]);
    await screen.findByRole('img', { name: 'Healthy' });
    expect(apiFetch).toHaveBeenCalledTimes(1);
  });

  it('uses lastSeen for a legacy row with no PMG collection timestamp', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(response('online'))
      .mockResolvedValueOnce(response('online', 7));
    const legacy = { ...healthy, pmg: { ...healthy.pmg, lastUpdated: undefined } };
    const { setState } = mount(legacy);
    await screen.findByText('mail-a');
    setState({ ...legacy, lastSeen: Date.parse(NEXT) });
    await waitFor(() => expect(queue()).toBe('7—/—/—/—'));
    expect(apiFetch).toHaveBeenCalledTimes(2);
  });

  it('refreshes a health change even without an advancing collection timestamp', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(response('online'))
      .mockResolvedValueOnce(response('degraded', 8));
    const { setState } = mount();
    await screen.findByText('mail-a');
    setState(failed);
    await waitFor(() => expect(queue()).toBe('8—/—/—/—'));
    expect(screen.getByRole('img', { name: 'Degraded' })).toBeInTheDocument();
    expect(apiFetch).toHaveBeenCalledTimes(2);
  });

  it('ignores an older in-flight failure that completes after accepted recovery', async () => {
    const old = deferred<Response>();
    const current = deferred<Response>();
    vi.mocked(apiFetch).mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise);
    const { setState } = mount(failed);
    setState({ ...healthy, pmg: { ...healthy.pmg, lastUpdated: NEXT } });
    await waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2));
    current.resolve(response('online', 7, 'current-node'));
    await screen.findByText('current-node');
    const obsolete = response('degraded', 100, 'obsolete-node');
    const decoded = deferred<void>();
    const json = obsolete.json.bind(obsolete);
    vi.spyOn(obsolete, 'json').mockImplementation(async () => {
      const body = await json();
      decoded.resolve();
      return body;
    });
    old.resolve(obsolete);
    await decoded.promise;
    // Wait through the completed body decode and the resource's continuations,
    // not two guesses at how many microtasks fetch/JSON parsing takes.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByText('obsolete-node')).toBeNull();
    expect(screen.getByRole('img', { name: 'Healthy' })).toBeInTheDocument();
    expect(queue('current-node')).toBe('7—/—/—/—');
  });

  it('keeps detail failure and Retry usable and recovers without closing the drawer', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(response('degraded'))
      .mockResolvedValueOnce(new Response('', { status: 503 }))
      .mockResolvedValueOnce(response('online', 0));
    const { setState } = mount(failed);
    await screen.findByText('mail-a');
    setState({ ...healthy, pmg: { ...healthy.pmg, lastUpdated: NEXT } });
    await screen.findByText('Failed to load Mail Gateway detail.');
    expect(screen.queryByText('mail-a')).toBeNull();
    expect(screen.getByRole('heading', { name: /gateway/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await screen.findByText('mail-a');
    expect(screen.queryByText('Failed to load Mail Gateway detail.')).toBeNull();
    expect(screen.getByRole('img', { name: 'Healthy' })).toBeInTheDocument();
    expect(queue()).toBe('0—/—/—/—');
    expect(apiFetch).toHaveBeenCalledTimes(3);
  });

  it('keeps an initial transport failure inside the existing error and Retry UI', async () => {
    vi.mocked(apiFetch)
      .mockRejectedValueOnce(new Error('synthetic upstream diagnostic'))
      .mockResolvedValueOnce(response('online', 0));
    mount(healthy, [critical]);
    await screen.findByText('Failed to load Mail Gateway detail.');
    const section = screen.getByRole('region', { name: /gateway/ });
    expect(screen.getByRole('heading', { name: /gateway/ })).toBeInTheDocument();
    expect(screen.getByTestId('drawer-attention-section')).toHaveTextContent('Collection failed');
    expect(screen.queryByText('synthetic upstream diagnostic')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await screen.findByText('mail-a');
    expect(screen.getByRole('region', { name: /gateway/ })).toBe(section);
    expect(queue()).toBe('0—/—/—/—');
    // Stored detail recovery does not clear an independently open alert.
    expect(screen.getByRole('img', { name: 'Needs attention' })).toBeInTheDocument();
  });

  it('recovers a failed detail on the next observation without requiring Retry', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(new Response('', { status: 503 }))
      .mockResolvedValueOnce(response('online', 9));
    const { setState } = mount();
    await screen.findByText('Failed to load Mail Gateway detail.');
    const section = screen.getByRole('region', { name: /gateway/ });
    setState({ ...healthy, pmg: { ...healthy.pmg, lastUpdated: NEXT } });
    await screen.findByText('mail-a');
    expect(screen.getByRole('region', { name: /gateway/ })).toBe(section);
    expect(screen.queryByText('Failed to load Mail Gateway detail.')).toBeNull();
    expect(queue()).toBe('9—/—/—/—');
    expect(apiFetch).toHaveBeenCalledTimes(2);
  });

  it('does not let an obsolete transport rejection replace accepted recovery', async () => {
    const old = deferred<Response>();
    vi.mocked(apiFetch)
      .mockReturnValueOnce(old.promise)
      .mockResolvedValueOnce(response('online', 7, 'current-node'));
    const { setState } = mount(failed);
    setState({ ...healthy, pmg: { ...healthy.pmg, lastUpdated: NEXT } });
    await screen.findByText('current-node');
    old.reject(new Error('obsolete synthetic failure'));
    await old.promise.catch(() => undefined);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByText('Failed to load Mail Gateway detail.')).toBeNull();
    expect(screen.getByRole('img', { name: 'Healthy' })).toBeInTheDocument();
    expect(queue('current-node')).toBe('7—/—/—/—');
  });

  it('admits only one pending Retry and keeps it usable after a repeated failure', async () => {
    const retry = deferred<void>();
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(new Response('', { status: 503 }))
      .mockImplementation(async () => {
        await retry.promise;
        return new Response('', { status: 503 });
      });
    mount();
    await screen.findByText('Failed to load Mail Gateway detail.');
    const button = screen.getByRole('button', { name: 'Retry' }) as HTMLButtonElement;
    try {
      button.click();
      await waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2));
      // A later task must not bypass createResource's same-microtask coalescing.
      await new Promise((resolve) => setTimeout(resolve, 0));
      button.click();
      await new Promise((resolve) => setTimeout(resolve, 0));
      button.click();
      expect(apiFetch).toHaveBeenCalledTimes(2);
      expect(button).toBeDisabled();
      expect(button).toHaveAttribute('aria-busy', 'true');
    } finally {
      retry.resolve();
    }
    await waitFor(() => expect(button).not.toBeDisabled());
    expect(button).toHaveAttribute('aria-busy', 'false');
    expect(screen.getByText('Failed to load Mail Gateway detail.')).toBeInTheDocument();
    vi.mocked(apiFetch).mockImplementation(async () => response('online', 0));
    button.click();
    await screen.findByText('mail-a');
    expect(queue()).toBe('0—/—/—/—');
    expect(apiFetch).toHaveBeenCalledTimes(3);
  });
});
