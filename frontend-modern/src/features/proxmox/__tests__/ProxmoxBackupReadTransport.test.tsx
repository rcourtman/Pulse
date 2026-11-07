import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { eventBus } from '@/stores/events';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { protectionPolicy, protectionPosture } from '@/utils/__fixtures__/protectionPosture';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
} from '../__fixtures__/backupIdentity';

// Keep the page, independent query owners, parsers, posture batching, apiClient
// and org events real. Isolated query tests cannot exercise an actual HTTP
// Retry-After wait, and API-client mocks hide headers and transport cancellation.
// These are DOM/transport regressions, not browser or installed-guest proof.
// Unrelated runtime-capability refreshes have their own global org listener.
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

type Source = 'pve' | 'pbs' | 'posture';
const sources: Source[] = ['pve', 'pbs', 'posture'];
const north = 'north/one';
const south = 'south/two';
const token = 'synthetic-monitoring-read-token';
// The same resource ID in both synthetic orgs deliberately collides with the
// posture-cache key. Only a new org's accepted reads can repopulate that key.
const guest = identityGuest('same-resource-id', 'east', 'pve-1');
const pathFor = (source: Source) =>
  source === 'posture' ? '/api/recovery/postures' : `/api/backups/${source}`;
const marker = (org: string, revision: string) => `${org.replace('/', '-')}-${revision}`;
const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });

function payload(source: Source, org = north, revision = 'saved') {
  const id = marker(org, revision);
  if (source === 'pve') {
    return {
      data: {
        storageBackups: [
          identityArchive({
            id: `file-${id}`,
            storage: `pve-${id}`,
            volid: `local:backup/${id}.vma.zst`,
          }),
        ],
        guestSnapshots: [],
        backupTasks: [],
      },
    };
  }
  if (source === 'pbs') {
    return {
      data: {
        backups: [identityPBS({ id: `pbs-${id}`, datastore: `pbs-${id}` })],
      },
    };
  }
  return {
    data: [
      {
        ...protectionPosture(guest.id),
        evaluatedAt: new Date(BACKUP_IDENTITY_NOW).toISOString(),
        state: revision === 'saved' ? 'protected' : 'attention',
        reason: revision === 'saved' ? 'recent-verified' : 'source-failed',
        explanation: `Assessment ${id}`,
      },
    ],
    policy: protectionPolicy,
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}

const fetchMock = vi.fn<typeof fetch>();
let reply: (source: Source, org: string, options: RequestInit) => Promise<Response>;
const requests = (source: Source, org?: string) =>
  fetchMock.mock.calls.filter(([url, options]) => {
    const headers = new Headers(options?.headers);
    return (
      new URL(String(url), 'https://pulse.invalid').pathname === pathFor(source) &&
      (org === undefined || headers.get('X-Pulse-Org-ID') === org)
    );
  });
const rows = () => document.querySelectorAll('[data-proxmox-backup-row="recoverable"]');
const verdict = () => document.querySelector('[data-backup-verdict]');
const notice = (source: Source) =>
  source === 'posture'
    ? /Protection status is unavailable/
    : new RegExp(`${source.toUpperCase()} backup inventory is unavailable`);
const retryName = (source: Source) =>
  source === 'posture' ? 'Retry protection status' : `Retry ${source.toUpperCase()} inventory`;
const settle = () => vi.advanceTimersByTimeAsync(0);

function mount() {
  return render(() => (
    <Router>
      <Route
        path="/*"
        component={() => <ProxmoxBackupsTable emptyIcon={<span />} workloads={[guest]} />}
      />
    </Router>
  ));
}

function expectAccepted(source: Source, org: string, revision: string) {
  if (source === 'posture') {
    expect(verdict()).toHaveAttribute(
      'data-backup-verdict',
      revision === 'saved' ? 'protected' : 'attention',
    );
  } else {
    expect(document.body).toHaveTextContent(`${source}-${marker(org, revision)}`);
  }
}

function expectReadOnlyTransport() {
  for (const [url, options] of fetchMock.mock.calls) {
    expect(options?.method ?? 'GET').toBe('GET');
    expect(options, String(url)).toMatchObject({
      credentials: 'include',
      signal: expect.any(AbortSignal),
    });
    expect(new Headers(options?.headers).get('X-API-Token')).toBe(token);
    expect(options?.body).toBeUndefined();
    const parsed = new URL(String(url), 'https://pulse.invalid');
    expect(sources.map(pathFor)).toContain(parsed.pathname);
    if (parsed.pathname === pathFor('posture')) {
      expect(parsed.searchParams.getAll('resourceId')).toEqual([guest.id]);
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
  window.history.replaceState({}, '', '/proxmox/backups/date');
  reply = async (source, org) => json(payload(source, org));
  fetchMock.mockImplementation(async (input, options = {}) => {
    const pathname = new URL(String(input), 'https://pulse.invalid').pathname;
    const source = sources.find((candidate) => pathFor(candidate) === pathname);
    if (!source) throw new Error(`Unexpected synthetic backup read: ${pathname}`);
    return reply(source, new Headers(options.headers).get('X-Pulse-Org-ID') ?? '', options);
  });
});

afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  clearAuth();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('backup reads through the real HTTP and consuming page boundary', () => {
  it('reads exactly the selected org inventory pair and bounded assessment without any write', async () => {
    mount();
    await settle();
    expect(rows()).toHaveLength(2);
    expect(verdict()).toHaveAttribute('data-backup-verdict', 'protected');
    for (const source of sources) {
      expect(requests(source, north)).toHaveLength(1);
      expectAccepted(source, north, 'saved');
    }
    expect(getOrgID()).toBe(north);
    expectReadOnlyTransport();
  });

  it.each(sources)(
    'lets a rate-limited %s poll settle across two other poll ticks',
    async (source) => {
      mount();
      await settle();
      const retryAfter =
        source === 'pbs' ? new Date(BACKUP_IDENTITY_NOW + 91_000).toUTCString() : '61';
      let limited = false;
      reply = async (candidate, org) => {
        if (candidate === source && !limited) {
          limited = true;
          return json({}, 429, { 'Retry-After': retryAfter });
        }
        return json(payload(candidate, org, candidate === source ? 'confirmed' : 'saved'));
      };
      await vi.advanceTimersByTimeAsync(30_000);
      const pending = requests(source)[1][1]?.signal;
      expect(pending).toBeInstanceOf(AbortSignal);
      expectAccepted(source, north, 'saved');

      await vi.advanceTimersByTimeAsync(60_000);
      expect(requests(source)).toHaveLength(2);
      expect(pending?.aborted).toBe(false);
      expectAccepted(source, north, 'saved');
      for (const other of sources.filter((candidate) => candidate !== source)) {
        expect(requests(other)).toHaveLength(4);
      }
      await vi.advanceTimersByTimeAsync(999);
      expect(requests(source)).toHaveLength(2);
      await vi.advanceTimersByTimeAsync(1);
      expect(requests(source)).toHaveLength(3);
      expect(requests(source)[2][1]?.signal).toBe(pending);
      expectAccepted(source, north, 'confirmed');
      expect(screen.queryByText(notice(source))).not.toBeInTheDocument();
      expect(rows()).toHaveLength(2);
      expectReadOnlyTransport();
    },
  );

  it.each(sources)(
    'cancels %s Retry-After on org change without replaying the retired read',
    async (source) => {
      mount();
      await settle();
      reply = async (candidate, org) =>
        candidate === source && org === north
          ? json({}, 429, { 'Retry-After': '61' })
          : json(payload(candidate, org, 'confirmed'));
      await vi.advanceTimersByTimeAsync(30_000);
      const retired = requests(source, north)[1][1]?.signal;
      expect(retired?.aborted).toBe(false);
      setOrgID(south);
      eventBus.emit('org_switched', south);
      await settle();
      expect(retired?.aborted).toBe(true);
      for (const candidate of sources) {
        expect(requests(candidate, south)).toHaveLength(1);
        expectAccepted(candidate, south, 'confirmed');
      }
      expect(document.body).not.toHaveTextContent(marker(north, 'saved'));
      await vi.advanceTimersByTimeAsync(61_000);
      expect(requests(source, north)).toHaveLength(2);
      expect(getOrgID()).toBe(south);
      expectReadOnlyTransport();
    },
  );

  it.each(sources)('cancels %s backoff and every mounted reader on disposal', async (source) => {
    const view = mount();
    await settle();
    reply = async (candidate, org) =>
      candidate === source ? json({}, 429, { 'Retry-After': '61' }) : json(payload(candidate, org));
    await vi.advanceTimersByTimeAsync(30_000);
    const retired = requests(source)[1][1]?.signal;
    view.unmount();
    expect(retired?.aborted).toBe(true);
    const before = fetchMock.mock.calls.length;
    setOrgID(south);
    eventBus.emit('org_switched', south);
    await vi.advanceTimersByTimeAsync(120_000);
    expect(fetchMock).toHaveBeenCalledTimes(before);
    expectReadOnlyTransport();
  });

  it.each(sources)(
    'ignores a late %s body after org replacement, including the shared posture cache',
    async (source) => {
      const oldBody = deferred<string>();
      let response!: Response;
      reply = async (candidate, org) => {
        if (candidate === source && org === north) {
          response = json({});
          // Model response headers already received but an old body still pending.
          // Abort can lose this race; the query generation must reject it too.
          vi.spyOn(response, 'text').mockImplementation(() => oldBody.promise);
          vi.spyOn(response, 'json').mockImplementation(async () =>
            JSON.parse(await oldBody.promise),
          );
          return response;
        }
        return json(payload(candidate, org, 'confirmed'));
      };
      const view = mount();
      await settle();
      const retired = requests(source, north)[0][1]?.signal;
      setOrgID(south);
      eventBus.emit('org_switched', south);
      await settle();
      expect(retired?.aborted).toBe(true);
      oldBody.resolve(JSON.stringify(payload(source, north)));
      await settle();
      expect(rows()).toHaveLength(2);
      for (const candidate of sources) expectAccepted(candidate, south, 'confirmed');
      expect(document.body).not.toHaveTextContent(marker(north, 'saved'));
      view.unmount();
      const remountedPosture = deferred<Response>();
      reply = async (candidate, org) =>
        candidate === 'posture'
          ? remountedPosture.promise
          : json(payload(candidate, org, 'confirmed'));
      mount();
      await settle();
      // Hold the fresh assessment: a quick response must not hide an old
      // body's accidental cache write during the remount.
      expect(verdict()).toHaveAttribute('data-backup-verdict', 'attention');
      remountedPosture.resolve(json(payload('posture', south, 'confirmed')));
      await settle();
      expectReadOnlyTransport();
    },
  );

  it.each(sources)(
    'withdraws a denied %s result and retries only its read owner',
    async (source) => {
      mount();
      await settle();
      reply = async (candidate, org) =>
        candidate === source ? json({ error: 'forbidden' }, 403) : json(payload(candidate, org));
      await vi.advanceTimersByTimeAsync(30_000);
      expect(screen.getByText(notice(source))).toBeInTheDocument();
      expect(rows()).toHaveLength(source === 'posture' ? 2 : 1);
      if (source === 'posture') expect(verdict()).toHaveAttribute('data-backup-verdict', 'unknown');
      else expect(document.body).not.toHaveTextContent(`${source}-${marker(north, 'saved')}`);
      const counts = sources.map((candidate) => requests(candidate).length);
      reply = async (candidate, org) => json(payload(candidate, org, 'confirmed'));
      fireEvent.click(screen.getByRole('button', { name: retryName(source) }));
      await settle();
      expect(screen.queryByText(notice(source))).not.toBeInTheDocument();
      expect(rows()).toHaveLength(2);
      sources.forEach((candidate, index) => {
        expect(requests(candidate)).toHaveLength(counts[index] + (candidate === source ? 1 : 0));
      });
      expectAccepted(source, north, 'confirmed');
      expect(getOrgID()).toBe(north);
      expectReadOnlyTransport();
    },
  );
});
