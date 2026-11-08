import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { eventBus } from '@/stores/events';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import type { ProtectionPosturesResponse } from '@/types/recovery';
import { protectionPolicy, protectionPosture } from '@/utils/__fixtures__/protectionPosture';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
} from '../__fixtures__/backupIdentity';

// Keep the page, fleet batching/parser/cache, apiClient and org events real.
// Mocked hook transports and a one-guest page cannot protect their composition
// past either the 200-ID request limit or the four-request concurrency boundary.
// Safe HTTP fixtures are not browser, provider or installed-guest acceptance.
vi.mock('@/api/license', () => ({
  LicenseAPI: {
    getRuntimeCapabilities: async () => ({
      capabilities: [],
      limits: [],
      blocked_capabilities: [],
      hosted_mode: false,
      max_history_days: 7,
      runtime: { build: 'community', label: 'Synthetic Community runtime' },
    }),
  },
}));
vi.mock('@/hooks/useObservedElementWidth', () => ({
  useObservedElementWidth: () => ({ setElement: () => {}, width: () => 1200 }),
}));
vi.mock('@/utils/logger', () => ({
  logger: { warn: vi.fn(), debug: vi.fn(), error: vi.fn() },
}));

const north = 'fixture-north';
const south = 'fixture-south';
const token = 'synthetic-fleet-read-token';
const posturePath = '/api/recovery/postures';
const fetchMock = vi.fn<typeof fetch>();
const fleet = (count = 801) =>
  Array.from({ length: count }, (_, index) =>
    identityGuest(`fleet-${String(index).padStart(4, '0')}`, 'east', 'pve-1', {
      vmid: 100 + index,
    }),
  );
const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });

interface BatchRead {
  ids: string[];
  org: string | null;
  signal: AbortSignal;
}
interface HeldBody {
  request: BatchRead;
  finish: (body?: unknown) => void;
}
let batches: BatchRead[];
let reply: (request: BatchRead) => Response;

function assessment(
  request: BatchRead,
  state: 'protected' | 'attention' = 'protected',
  days = 7,
): ProtectionPosturesResponse {
  return {
    // Deliberately reverse the server's order: the page must join by subject ID.
    data: request.ids
      .slice()
      .reverse()
      .map((id) => ({
        ...protectionPosture(id),
        state,
        reason: state === 'protected' ? 'recent-verified' : 'stale',
        evaluatedAt: new Date(BACKUP_IDENTITY_NOW).toISOString(),
      })),
    policy: { ...protectionPolicy, freshnessWindowSeconds: days * 86_400 },
  };
}

function holdBodies(status = 200) {
  const held: HeldBody[] = [];
  return {
    held,
    reply: (request: BatchRead) => {
      let finish!: (body: string) => void;
      const body = new Promise<string>((resolve) => {
        finish = resolve;
      });
      const response = json({}, status);
      // Headers have arrived. Keep body consumption pending independently of
      // abort, so a late body tests generation/queue protection as well as I/O.
      vi.spyOn(response, 'text').mockImplementation(() => body);
      held.push({
        request,
        finish: (payload = assessment(request)) => finish(JSON.stringify(payload)),
      });
      return response;
    },
  };
}

const settle = () => vi.advanceTimersByTimeAsync(0);
const verdict = () => document.querySelector('[data-backup-verdict]');
const rule = () => document.querySelector('[data-coverage-posture-rule]');
const restoreRows = () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]');
const inventoryCalls = () =>
  fetchMock.mock.calls.filter(([url]) => String(url).startsWith('/api/backups/'));
const expectHealthy = (count = 801) => {
  expect(verdict()).toHaveAttribute('data-backup-verdict', 'protected');
  expect(verdict()).toHaveTextContent(`All ${count} protected`);
  expect(rule()).toHaveTextContent('older than 7 days');
};
const expectUnavailable = (count = 801) => {
  expect(screen.getByText(/Protection status is unavailable/)).toBeInTheDocument();
  expect(verdict()).toHaveAttribute('data-backup-verdict', 'unknown');
  expect(verdict()).toHaveTextContent(`0 of ${count} protected`);
  expect(rule()).toBeNull();
  expect(document.querySelector('[data-backup-protected-row]')).toBeNull();
  expect(restoreRows()).toHaveLength(2);
};

function mount(count = 801) {
  return render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={fleet(count)} />}
      />
    </Router>
  ));
}

function expectReadOnlyTransport() {
  for (const [url, options] of fetchMock.mock.calls) {
    expect(options?.method ?? 'GET').toBe('GET');
    expect(options?.body).toBeUndefined();
    expect(options?.credentials).toBe('include');
    expect(options?.signal).toBeInstanceOf(AbortSignal);
    expect(new Headers(options?.headers).get('X-API-Token')).toBe(token);
    const parsed = new URL(String(url), 'https://pulse.invalid');
    expect(['/api/backups/pve', '/api/backups/pbs', posturePath]).toContain(parsed.pathname);
    if (parsed.pathname === posturePath) {
      const ids = parsed.searchParams.getAll('resourceId');
      expect(ids.length).toBeGreaterThan(0);
      expect(ids.length).toBeLessThanOrEqual(200);
      expect(parsed.searchParams.get('limit')).toBe('200');
    }
  }
  expect(getApiToken()).toBe(token);
}

beforeEach(() => {
  vi.useFakeTimers({ now: BACKUP_IDENTITY_NOW });
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('scrollTo', vi.fn());
  sessionStorage.clear();
  clearAuth();
  setApiToken(token);
  setOrgID(north);
  resetCreateNonSuspendingQueryCacheForTest();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  // By date renders only the two independent restore points while the real
  // health model still rates every guest; no 801-row DOM performance claim.
  window.history.replaceState({}, '', '/proxmox/backups/date');
  batches = [];
  reply = (request) => json(assessment(request));
  fetchMock.mockImplementation(async (input, options = {}) => {
    const url = new URL(String(input), 'https://pulse.invalid');
    if (url.pathname === '/api/backups/pve')
      return json({
        data: {
          storageBackups: [identityArchive()],
          guestSnapshots: [],
          backupTasks: [],
        },
      });
    if (url.pathname === '/api/backups/pbs') return json({ data: { backups: [identityPBS()] } });
    if (url.pathname !== posturePath)
      throw new Error(`Unexpected synthetic fleet read: ${url.pathname}`);
    const request = {
      ids: url.searchParams.getAll('resourceId'),
      org: new Headers(options.headers).get('X-Pulse-Org-ID'),
      signal: options.signal as AbortSignal,
    };
    batches.push(request);
    return reply(request);
  });
});

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  clearAuth();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('fleet backup health through real HTTP and the consuming page', () => {
  it('rates all 801 targets only after every bounded response body, including the queued fifth batch', async () => {
    const pending = holdBodies();
    reply = pending.reply;
    mount();
    await settle();
    expect(batches).toHaveLength(4);
    expect(pending.held).toHaveLength(4);
    expect(verdict()).toHaveAttribute('data-backup-verdict', 'checking');
    expect(rule()).toBeNull();
    expect(restoreRows()).toHaveLength(2);

    pending.held[3].finish();
    await settle();
    expect(batches).toHaveLength(5);
    expect(batches.map((request) => request.ids.length)).toEqual([200, 200, 200, 200, 1]);
    // Completing the small queued batch must not publish a one-guest verdict.
    pending.held[4].finish(assessment(pending.held[4].request, 'attention'));
    pending.held[0].finish();
    pending.held[2].finish();
    await settle();
    expect(verdict()).toHaveAttribute('data-backup-verdict', 'checking');
    expect(rule()).toBeNull();
    pending.held[1].finish();
    await settle();
    expect(verdict()).toHaveAttribute('data-backup-verdict', 'attention');
    expect(verdict()).toHaveTextContent('800 of 801 protected');
    expect(rule()).toHaveTextContent('older than 7 days');
    expect(restoreRows()).toHaveLength(2);
    // The only exception came from the queued last subject, not an array
    // position in an earlier reversed batch. Follow it into real Coverage.
    fireEvent.click(screen.getByRole('button', { name: '1 no backup in 7 days' }));
    await settle();
    const coverage = document.querySelectorAll('[data-proxmox-backup-row="coverage"]');
    expect(coverage).toHaveLength(1);
    expect(coverage[0]).toHaveTextContent('fleet-0800');
    expect(window.location.pathname).toBe('/proxmox/backups/coverage');
    expect(new URLSearchParams(window.location.search).get('reason')).toBe('stale');
    expect(batches.flatMap((request) => request.ids)).toEqual(fleet().map((guest) => guest.id));
    expect(
      batches.every((request) => request.org === north && request.signal === batches[0].signal),
    ).toBe(true);
    expect(batches[0].signal.aborted).toBe(false);
    expect(inventoryCalls()).toHaveLength(2);
    expectReadOnlyTransport();
  });

  it('cancels sibling HTTP backoff and queued work after one denied batch, then retries the entire fleet', async () => {
    mount();
    await settle();
    expectHealthy();
    const blocked = holdBodies();
    const denied = holdBodies(403);
    reply = (request) => {
      if (request.ids[0] === 'fleet-0000') return json({}, 429, { 'Retry-After': '61' });
      return request.ids[0] === 'fleet-0200' ? denied.reply(request) : blocked.reply(request);
    };
    await vi.advanceTimersByTimeAsync(30_000);
    const retired = batches.slice(5);
    expect(retired).toHaveLength(4);
    expectHealthy();
    denied.held[0].finish({ error: 'synthetic-private-provider-detail' });
    await settle();
    expectUnavailable();
    expect(retired.every((request) => request.signal.aborted)).toBe(true);
    for (const body of blocked.held) body.finish();
    await settle();
    expect(batches.filter((request) => request.signal === retired[0].signal)).toHaveLength(4);
    expect(document.body).not.toHaveTextContent('synthetic-private-provider-detail');

    const retry = holdBodies();
    reply = retry.reply;
    const button = screen.getByRole('button', { name: 'Retry protection status' });
    fireEvent.click(button);
    fireEvent.click(button);
    await settle();
    expect(button).toHaveAttribute('aria-disabled', 'true');
    expect(retry.held).toHaveLength(4);
    expectUnavailable();
    retry.held[0].finish();
    await settle();
    expect(retry.held).toHaveLength(5);
    for (const index of [4, 3, 2]) retry.held[index].finish();
    await settle();
    expectUnavailable();
    retry.held[1].finish();
    await settle();
    expectHealthy();
    expect(screen.queryByText(/Protection status is unavailable/)).toBeNull();
    expect(retry.held.flatMap(({ request }) => request.ids)).toEqual(
      fleet().map((guest) => guest.id),
    );
    expect(inventoryCalls()).toHaveLength(4);
    reply = (request) => json(assessment(request));
    await vi.advanceTimersByTimeAsync(61_000);
    expect(batches.filter((request) => request.signal === retired[0].signal)).toHaveLength(4);
    expectReadOnlyTransport();
  });

  it.each(['row', 'policy'] as const)(
    'rejects a bad final-batch %s without publishing a partial count or the old rule',
    async (invalid) => {
      mount(201);
      await settle();
      expectHealthy(201);
      const pending = holdBodies();
      reply = pending.reply;
      await vi.advanceTimersByTimeAsync(30_000);
      expect(pending.held).toHaveLength(2);
      pending.held[0].finish();
      await settle();
      expectHealthy(201);
      const body = assessment(pending.held[1].request);
      pending.held[1].finish(
        invalid === 'row'
          ? { ...body, data: [{ ...body.data[0], state: 'synthetic-private-invalid-state' }] }
          : { ...body, policy: { ...body.policy, freshnessWindowSeconds: 86_400 } },
      );
      await settle();
      expectUnavailable(201);
      expect(document.body).not.toHaveTextContent('synthetic-private-invalid-state');

      const retry = holdBodies();
      reply = retry.reply;
      fireEvent.click(screen.getByRole('button', { name: 'Retry protection status' }));
      await settle();
      retry.held[1].finish(assessment(retry.held[1].request, 'attention', 2));
      await settle();
      expectUnavailable(201);
      retry.held[0].finish(assessment(retry.held[0].request, 'attention', 2));
      await settle();
      expect(verdict()).toHaveAttribute('data-backup-verdict', 'attention');
      expect(verdict()).toHaveTextContent('0 of 201 protected');
      expect(screen.getByRole('button', { name: '201 no backup in 2 days' })).toBeInTheDocument();
      expect(rule()).toHaveTextContent('older than 2 days');
      expect(rule()).not.toHaveTextContent('7 days');
      expect(screen.queryByText(/Protection status is unavailable/)).toBeNull();
      expect(restoreRows()).toHaveLength(2);
      expect(inventoryCalls()).toHaveLength(4);
      expectReadOnlyTransport();
    },
  );

  it.each(['organisation', 'disposal'] as const)(
    'retires backoff, late bodies and the fifth queued batch on %s change',
    async (retirement) => {
      const pending = holdBodies();
      reply = (request) =>
        request.ids[0] === 'fleet-0000'
          ? json({}, 429, { 'Retry-After': '61' })
          : pending.reply(request);
      const view = mount();
      await settle();
      expect(batches).toHaveLength(4);
      const retired = batches.slice();
      reply = (request) => json(assessment(request, 'attention', 2));
      if (retirement === 'disposal') view.unmount();
      setOrgID(south);
      eventBus.emit('org_switched', south);
      await settle();
      expect(retired.every((request) => request.signal.aborted)).toBe(true);
      // Even a remount must see the replacement organisation, not late old health.
      if (retirement === 'disposal') {
        mount();
        await settle();
      }
      for (const body of pending.held) body.finish();
      await settle();
      expect(batches.filter((request) => request.signal === retired[0].signal)).toHaveLength(4);
      expect(
        batches.filter((request) => request.org === south).flatMap((request) => request.ids),
      ).toEqual(fleet().map((guest) => guest.id));
      expect(verdict()).toHaveAttribute('data-backup-verdict', 'attention');
      expect(verdict()).toHaveTextContent('0 of 801 protected');
      expect(screen.getByRole('button', { name: '801 no backup in 2 days' })).toBeInTheDocument();
      expect(rule()).toHaveTextContent('older than 2 days');
      await vi.advanceTimersByTimeAsync(61_000);
      expect(batches.filter((request) => request.org === north)).toHaveLength(4);
      expect(getOrgID()).toBe(south);
      expect(restoreRows()).toHaveLength(2);
      expectReadOnlyTransport();
    },
  );
});
