import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'solid-js';
import type { Alert } from '@/types/api';

// Keep both the canonical socket store and the API client real. The final
// HTTP/socket boundaries are synthetic, not a backend or a changed grant.
const notifications = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  warning: vi.fn(),
}));
vi.mock('@/stores/notifications', () => ({ notificationStore: notifications }));
vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), error: vi.fn(), warn: vi.fn() },
}));

const endpoint = '/api/alerts/active';
const token = 'synthetic-monitoring-reader';
const knownAlert: Alert = {
  id: 'agent:same-name-cpu',
  type: 'cpu',
  level: 'critical',
  resourceId: 'agent:same-name',
  resourceName: 'same-name',
  node: 'same-name',
  instance: 'same-name',
  message: 'Last-known CPU above threshold',
  value: 96,
  threshold: 90,
  startTime: '2026-11-11T02:00:00Z',
  acknowledged: false,
};
const json = (value: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });
const fetchMock = vi.fn<typeof fetch>();
const sockets: SyntheticSocket[] = [];
let api: typeof import('@/utils/apiClient');
let store: ReturnType<typeof import('@/stores/websocket').createWebSocketStore>;
let dispose: () => void;
let pending: Promise<boolean>[];
let releaseResponses: (() => void)[];

class SyntheticSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  readyState = SyntheticSocket.CONNECTING;
  send = vi.fn();
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(readonly url: string) {
    sockets.push(this);
  }
  open() {
    this.readyState = SyntheticSocket.OPEN;
    this.onopen?.({} as Event);
  }
  close(code = 1000, reason = '') {
    this.readyState = SyntheticSocket.CLOSED;
    this.onclose?.({ code, reason } as CloseEvent);
  }
  snapshot(alerts: Alert[]) {
    this.onmessage?.({
      data: JSON.stringify({
        type: 'initialState',
        data: { resources: [], activeAlerts: alerts, recentlyResolved: [], lastUpdate: Date.now() },
      }),
    } as MessageEvent);
  }
}

function queuedResponse() {
  let resolve!: (response: Response) => void;
  const response = new Promise<Response>((yes) => {
    resolve = yes;
  });
  releaseResponses.push(() => resolve(json([])));
  fetchMock.mockImplementationOnce(() => response);
  return { resolve };
}
function refresh() {
  const result = store.refreshActiveAlerts();
  pending.push(result);
  return result;
}
function storageSnapshot(storage: Storage) {
  return Object.fromEntries(
    Array.from({ length: storage.length }, (_, index) => {
      const key = storage.key(index)!;
      return [key, storage.getItem(key)];
    }),
  );
}
function expectSavedAuthority(org = 'north') {
  expect(api.getOrgID()).toBe(org);
  expect(api.getApiToken()).toBe(token);
  expect(sessionStorage.getItem('pulse_auth')).toBe(
    JSON.stringify({ type: 'token', value: token }),
  );
  expect(sessionStorage.getItem('pulse_auth_user')).toBe('synthetic-monitoring-user');
  expect(localStorage.getItem('just_logged_out')).toBeNull();
  expect(localStorage.getItem('unrelated-preference')).toBe('preserve');
  expect(document.cookie).toContain(`pulse_org_id=${org}`);
}
function expectRequests(orgs: string[], retryCSRF?: string) {
  expect(fetchMock).toHaveBeenCalledTimes(orgs.length);
  fetchMock.mock.calls.forEach(([url, options], index) => {
    expect(url).toBe(endpoint);
    expect(options).toMatchObject({
      credentials: 'include',
      headers: {
        'X-Pulse-Org-ID': orgs[index],
        'X-API-Token': token,
        'X-Requested-With': 'XMLHttpRequest',
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
    });
    expect((options?.headers as Record<string, string>)['X-CSRF-Token']).toBe(
      index > 0 ? retryCSRF : undefined,
    );
    expect(options?.method ?? 'GET').toBe('GET');
  });
}
function expectAlerts(alerts: Alert[], status: 'ready' | 'unavailable' | 'pending') {
  expect(store.activeAlertsHydrationStatus()).toBe(status);
  expect(Object.values(store.activeAlerts)).toEqual(alerts);
  expect(store.state.activeAlerts).toEqual(alerts);
}
const flush = async () => {
  for (let index = 0; index < 12; index++) await Promise.resolve();
};

beforeEach(async () => {
  vi.resetModules();
  vi.clearAllMocks();
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-11-11T03:00:00Z'));
  sockets.length = 0;
  pending = [];
  releaseResponses = [];
  dispose = () => {};
  fetchMock.mockReset().mockRejectedValue(new Error('Unexpected extra HTTP request'));
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('WebSocket', SyntheticSocket);
  sessionStorage.clear();
  localStorage.clear();
  localStorage.setItem('unrelated-preference', 'preserve');
  for (const name of ['pulse_org_id', 'pulse_csrf'])
    document.cookie = `${name}=; Max-Age=0; Path=/`;
  delete window.__pulseAlertsDetectionEnabled;
  api = await import('@/utils/apiClient');
  api.clearAuth();
  api.setApiToken(token);
  api.setOrgID('north');
  sessionStorage.setItem('pulse_auth_user', 'synthetic-monitoring-user');
  const { createWebSocketStore } = await import('@/stores/websocket');
  store = createRoot((cleanup) => {
    dispose = cleanup;
    return createWebSocketStore('ws://localhost/ws?org_id=north');
  });
  sockets[0].open();
});

afterEach(async () => {
  dispose();
  releaseResponses.forEach((release) => release());
  await Promise.allSettled(pending);
  await flush();
  api.clearAuth();
  sessionStorage.clear();
  localStorage.clear();
  for (const name of ['pulse_org_id', 'pulse_csrf'])
    document.cookie = `${name}=; Max-Age=0; Path=/`;
  delete window.__pulseAlertsDetectionEnabled;
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('monitoring access across the real HTTP client and socket store', () => {
  it('coalesces cold alert readers and admits real JSON without claiming resource hydration', async () => {
    const response = queuedResponse();
    const first = refresh();
    const joined = refresh();
    expect(joined).toBe(first);
    expectAlerts([], 'pending');
    expectRequests(['north']);
    response.resolve(json([knownAlert]));
    await expect(first).resolves.toBe(true);
    await expect(joined).resolves.toBe(true);
    expectAlerts([knownAlert], 'ready');
    expect(store.resourceSnapshotReceived()).toBe(false);
    expect(store.initialDataReceived()).toBe(false);
    expectSavedAuthority();
  });

  it.each([
    { name: 'object instead of alert array', response: () => json({ activeAlerts: [] }) },
    { name: 'null payload', response: () => json(null) },
    { name: 'invalid JSON', response: () => new Response('not-json', { status: 200 }) },
    { name: 'service failure', response: () => json({ error: 'Unavailable' }, 503) },
  ])('does not admit $name as a successful empty estate', async ({ response }) => {
    fetchMock.mockResolvedValueOnce(response());
    await expect(refresh()).resolves.toBe(false);
    expectAlerts([], 'unavailable');
    expect(store.resourceSnapshotReceived()).toBe(false);
    expectRequests(['north']);
    expectSavedAuthority();
  });

  it.each([
    { org: 'north', code: 'access_denied', csrf: false, known: false },
    { org: 'north', code: 'access_denied', csrf: true, known: true },
    { org: 'default', code: 'default_org_access_denied', csrf: false, known: true },
    { org: 'default', code: 'default_org_access_denied', csrf: true, known: false },
  ])(
    'keeps $org authority after membership403 (CSRF=$csrf, snapshot=$known)',
    async ({ org, code, csrf, known }) => {
      api.setOrgID(org);
      if (org !== 'north') {
        store.switchUrl(`ws://localhost/ws?org_id=${org}`);
        sockets.at(-1)!.open();
      }
      if (known) sockets.at(-1)!.snapshot([knownAlert]);
      // Exercise the real client's existing same-organisation CSRF retry too.
      // A retry is not permission to select the server's other organisation.
      if (csrf) document.cookie = 'pulse_csrf=synthetic-refreshed-csrf; Path=/';
      const before = storageSnapshot(sessionStorage);
      fetchMock.mockImplementation(async () => json({ error: code }, 403));
      await expect(refresh()).resolves.toBe(false);
      expectAlerts(known ? [knownAlert] : [], known ? 'ready' : 'unavailable');
      expectRequests(csrf ? [org, org] : [org], csrf ? 'synthetic-refreshed-csrf' : undefined);
      if (csrf)
        expect(
          (fetchMock.mock.calls[1][1]?.headers as Record<string, string>)['X-CSRF-Token'],
        ).toBe('synthetic-refreshed-csrf');
      expect(storageSnapshot(sessionStorage)).toEqual(before);
      expectSavedAuthority(org);
      expect(sockets).toHaveLength(org === 'north' ? 1 : 2);
    },
  );

  it.each([
    { name: 'empty', response: () => json([]), known: false },
    {
      name: 'nonempty',
      response: () => json([{ ...knownAlert, message: 'Late HTTP result' }]),
      known: false,
    },
    { name: 'membership403', response: () => json({ error: 'access_denied' }, 403), known: false },
    { name: 'empty', response: () => json([]), known: true },
    {
      name: 'nonempty',
      response: () => json([{ ...knownAlert, message: 'Late HTTP result' }]),
      known: true,
    },
    { name: 'membership403', response: () => json({ error: 'access_denied' }, 403), known: true },
  ])(
    'retires a late $name HTTP completion after4403 (snapshot=$known)',
    async ({ response, known }) => {
      if (known) sockets[0].snapshot([knownAlert]);
      const reply = queuedResponse();
      const read = refresh();
      const before = storageSnapshot(sessionStorage);
      sockets[0].close(4403, 'Organization access ended');
      reply.resolve(response());
      await expect(read).resolves.toBe(false);
      expectAlerts(known ? [knownAlert] : [], known ? 'ready' : 'unavailable');
      expect(store.connected()).toBe(false);
      expect(store.reconnecting()).toBe(false);
      expect(store.resourceSnapshotReceived()).toBe(known);
      await vi.advanceTimersByTimeAsync(60_000);
      await expect(refresh()).resolves.toBe(false);
      expectRequests(['north']);
      expect(sockets).toHaveLength(1);
      expect(storageSnapshot(sessionStorage)).toEqual(before);
      expectSavedAuthority();
      expect(notifications.warning).toHaveBeenCalledExactlyOnceWith(
        'Access to this organization ended. Select another organization or reload after access is restored.',
        10000,
      );
    },
  );

  it.each(['reconnect', 'switch organisation'] as const)(
    'admits intentional %s without letting a retired read or close take over',
    async (action) => {
      const oldSocket = sockets[0];
      const oldReply = queuedResponse();
      const oldRead = refresh();
      oldSocket.close(4403, 'Organization access ended');
      const nextOrg = action === 'reconnect' ? 'north' : 'south';
      if (action === 'reconnect') store.reconnect();
      else {
        api.setOrgID(nextOrg);
        store.switchUrl('ws://localhost/ws?org_id=south');
      }
      expect(sockets).toHaveLength(2);
      sockets[1].open();
      const currentReply = queuedResponse();
      const currentRead = refresh();
      expectRequests(['north', nextOrg]);
      const currentAlert = { ...knownAlert, message: 'Current admitted organization' };
      currentReply.resolve(json([currentAlert]));
      await expect(currentRead).resolves.toBe(true);
      oldReply.resolve(json([]));
      await expect(oldRead).resolves.toBe(false);
      // Retained callbacks from a retired socket must not affect its successor.
      oldSocket.close(4403, 'Late old close');
      expectAlerts([currentAlert], 'ready');
      expect(store.connected()).toBe(true);
      expect(store.reconnecting()).toBe(false);
      expectSavedAuthority(nextOrg);
      expect(notifications.warning).toHaveBeenCalledTimes(1);
    },
  );

  it('does not replace a newer socket snapshot with an older real HTTP empty array', async () => {
    const reply = queuedResponse();
    const read = refresh();
    sockets[0].snapshot([knownAlert]);
    reply.resolve(json([]));
    await expect(read).resolves.toBe(false);
    expectAlerts([knownAlert], 'ready');
    expectRequests(['north']);
    expectSavedAuthority();
  });

  it('distinguishes an actual HTTP401 from membership403 and clears only saved authentication', async () => {
    fetchMock.mockResolvedValueOnce(json({ error: 'Session expired' }, 401));
    await expect(refresh()).resolves.toBe(false);
    expectAlerts([], 'unavailable');
    expectRequests(['north']);
    expect(api.getApiToken()).toBeNull();
    expect(api.getOrgID()).toBeNull();
    expect(sessionStorage.getItem('pulse_auth')).toBeNull();
    expect(sessionStorage.getItem('pulse_auth_user')).toBeNull();
    expect(sessionStorage.getItem('pulse_org_id')).toBeNull();
    expect(document.cookie).not.toContain('pulse_org_id=');
    expect(localStorage.getItem('just_logged_out')).toBe('true');
    expect(localStorage.getItem('unrelated-preference')).toBe('preserve');
  });
});
