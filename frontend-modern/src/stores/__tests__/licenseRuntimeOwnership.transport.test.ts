import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { waitFor } from '@solidjs/testing-library';
import type { LicenseRuntimeCapabilities } from '@/api/license';

// Keep the singleton, LicenseAPI normalization, event bus and API client real.
// Only the final network boundary is synthetic: no account or grant is changed.
const log = vi.hoisted(() => ({ debug: vi.fn(), error: vi.fn(), warn: vi.fn() }));
vi.mock('@/utils/logger', () => ({ logger: log }));
const fetchMock = vi.fn<typeof fetch>();
let store: typeof import('@/stores/license');
let api: typeof import('@/utils/apiClient');
let events: typeof import('@/stores/events');
let releases: (() => void)[];
let loads: Promise<void>[];
const endpoint = '/api/license/runtime-capabilities';
const payload = (enabled: boolean, days = enabled ? 90 : 7): LicenseRuntimeCapabilities => ({
  capabilities: enabled ? ['rbac', 'multi_tenant'] : [],
  limits: [],
  hosted_mode: enabled,
  max_history_days: days,
  runtime: { build: enabled ? 'pro' : 'community', label: 'Synthetic runtime' },
  blocked_capabilities: [],
});
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
const turn = async () => {
  for (let index = 0; index < 12; index++) await Promise.resolve();
};
function deferred<T>(cleanupValue: T) {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  releases.push(() => resolve(cleanupValue));
  return { promise, resolve };
}
function queued() {
  const pending = deferred(json(payload(false)));
  fetchMock.mockImplementationOnce(() => pending.promise);
  return pending;
}
function load(force = false) {
  const promise = store.loadRuntimeCapabilities(force);
  loads.push(promise);
  return promise;
}
function switchOrg(org: string) {
  api.setOrgID(org);
  events.eventBus.emit('org_switched', org);
}
function expectCurrent(value: LicenseRuntimeCapabilities) {
  expect(store.runtimeCapabilities()).toEqual(value);
  expect(store.runtimeCapabilitiesLoaded()).toBe(true);
  expect(store.runtimeCapabilitiesLoading()).toBe(false);
  expect(store.runtimeCapabilitiesLoadError()).toBeNull();
  expect(store.hasFeature('rbac')).toBe(value.capabilities.includes('rbac'));
  expect(store.maxHistoryDays()).toBe(value.max_history_days);
}

beforeEach(async () => {
  vi.resetModules();
  vi.clearAllMocks();
  releases = [];
  loads = [];
  fetchMock.mockReset().mockImplementation(async () => {
    throw new Error('Unexpected extra capability request');
  });
  vi.stubGlobal('fetch', fetchMock);
  sessionStorage.clear();
  document.cookie = 'pulse_org_id=; Max-Age=0; Path=/';
  api = await import('@/utils/apiClient');
  api.clearAuth();
  api.setApiToken('synthetic-capability-reader');
  api.setOrgID('north');
  store = await import('@/stores/license');
  events = await import('@/stores/events');
});

afterEach(async () => {
  for (const release of releases) release();
  await Promise.allSettled(loads);
  await turn();
  api.clearAuth();
  vi.unstubAllGlobals();
});

describe('runtime capabilities keep the latest read owner through the real transport', () => {
  it('shares one cold startup read among simultaneous consumers', async () => {
    const response = queued();
    const first = load();
    const second = load();
    const third = load();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(store.runtimeCapabilitiesLoaded()).toBe(false);
    expect(store.runtimeCapabilitiesLoading()).toBe(true);
    response.resolve(json(payload(true)));
    await Promise.all([first, second, third]);
    expectCurrent(payload(true));
    expect(fetchMock.mock.calls[0]).toMatchObject([
      endpoint,
      { credentials: 'include', headers: { 'X-Pulse-Org-ID': 'north' } },
    ]);
  });

  it('joins an active forced refresh instead of returning an already loaded snapshot', async () => {
    fetchMock.mockResolvedValueOnce(json(payload(false)));
    await load();
    const response = queued();
    const forced = load(true);
    let joinedFinished = false;
    const joined = load().then(() => {
      joinedFinished = true;
    });
    await turn();
    expect(joinedFinished).toBe(false);
    expect(store.runtimeCapabilities()).toEqual(payload(false));
    expect(store.runtimeCapabilitiesLoading()).toBe(true);
    response.resolve(json(payload(true)));
    await Promise.all([forced, joined]);
    expectCurrent(payload(true));
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it.each(['success', 'error'] as const)(
    'ignores an older %s while the current read is still pending, including its flags and awaiters',
    async (outcome) => {
      const olderResponse = queued();
      const currentResponse = queued();
      let olderFinished = false;
      const older = load().then(() => {
        olderFinished = true;
      });
      const current = load(true);
      olderResponse.resolve(
        outcome === 'success'
          ? json(payload(true))
          : json({ error: 'Retired capability read failed' }, 503),
      );
      await turn();
      expect(store.runtimeCapabilities()).toBeNull();
      expect(store.runtimeCapabilitiesLoaded()).toBe(false);
      expect(store.runtimeCapabilitiesLoading()).toBe(true);
      expect(store.runtimeCapabilitiesLoadError()).toBeNull();
      expect(olderFinished).toBe(false);
      expect(log.error).not.toHaveBeenCalled();
      currentResponse.resolve(json(payload(false)));
      await Promise.all([older, current]);
      expectCurrent(payload(false));
    },
  );

  it.each(['grant', 'withdrawal', 'error'] as const)(
    'does not replace a newer accepted result with a late old %s',
    async (outcome) => {
      const olderResponse = queued();
      const currentResponse = queued();
      const older = load();
      const current = load(true);
      const accepted = payload(outcome !== 'grant', 30);
      currentResponse.resolve(json(accepted));
      await current;
      expectCurrent(accepted);
      olderResponse.resolve(
        outcome === 'error'
          ? json({ error: 'Retired capability read failed' }, 503)
          : json(payload(outcome === 'grant')),
      );
      await older;
      expectCurrent(accepted);
      expect(log.error).not.toHaveBeenCalled();
      expect(fetchMock).toHaveBeenCalledTimes(2);
    },
  );

  it('keeps the current error and Community fallback when an older success arrives afterwards', async () => {
    const olderResponse = queued();
    const currentResponse = queued();
    const older = load();
    const current = load(true);
    currentResponse.resolve(json({ error: 'Current capability read failed' }, 503));
    await current;
    const fallback = store.runtimeCapabilities();
    const error = store.runtimeCapabilitiesLoadError();
    expect(fallback?.capabilities).toEqual(['update_alerts', 'sso', 'ai_patrol']);
    expect(error?.message).toBe('Current capability read failed');
    expect(store.runtimeCapabilitiesLoading()).toBe(false);
    olderResponse.resolve(json(payload(true)));
    await older;
    expect(store.runtimeCapabilities()).toBe(fallback);
    expect(store.runtimeCapabilitiesLoadError()).toBe(error);
    expect(store.maxHistoryDays()).toBe(7);
    expect(log.error).toHaveBeenCalledTimes(1);
  });

  it('keeps every superseded awaiter pending through a third forced generation', async () => {
    const firstResponse = queued();
    const secondResponse = queued();
    const thirdResponse = queued();
    let finished = 0;
    const first = load().then(() => finished++);
    const second = load(true).then(() => finished++);
    const third = load(true);
    firstResponse.resolve(json(payload(true, 90)));
    await turn();
    secondResponse.resolve(json(payload(true, 30)));
    await turn();
    expect(finished).toBe(0);
    expect(store.runtimeCapabilities()).toBeNull();
    expect(store.runtimeCapabilitiesLoading()).toBe(true);
    thirdResponse.resolve(json(payload(false)));
    await Promise.all([first, second, third]);
    expect(finished).toBe(2);
    expectCurrent(payload(false));
  });

  it('keeps A/B/A organisation generations distinct even when the first A completes last', async () => {
    const firstA = queued();
    const orgB = queued();
    const secondA = queued();
    const first = load();
    switchOrg('south');
    switchOrg('north');
    const current = load();
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(
      fetchMock.mock.calls.map(
        ([, init]) => (init?.headers as Record<string, string>)['X-Pulse-Org-ID'],
      ),
    ).toEqual(['north', 'south', 'north']);
    expect(store.runtimeCapabilities()).toBeNull();
    secondA.resolve(json(payload(false)));
    await current;
    orgB.resolve(json(payload(true, 30)));
    firstA.resolve(json(payload(true, 90)));
    await first;
    await turn();
    expectCurrent(payload(false));
    expect(api.getOrgID()).toBe('north');
    expect(api.getApiToken()).toBe('synthetic-capability-reader');
  });

  it('rejects a stale body that finishes decoding after an organisation switch', async () => {
    const body = deferred(JSON.stringify(payload(false)));
    const response = json(null);
    const readBody = vi.spyOn(response, 'text').mockReturnValueOnce(body.promise);
    fetchMock.mockResolvedValueOnce(response);
    const next = queued();
    const older = load();
    await waitFor(() => expect(readBody).toHaveBeenCalledOnce());
    switchOrg('south');
    const current = load();
    next.resolve(json(payload(false)));
    await current;
    body.resolve(JSON.stringify(payload(true)));
    await older;
    expectCurrent(payload(false));
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('preserves the real invalid_org startup recovery and saved authentication', async () => {
    api.setOrgID('deleted-synthetic-workspace');
    fetchMock
      .mockResolvedValueOnce(json({ error: 'invalid_org' }, 400))
      .mockResolvedValueOnce(json(payload(true)));
    await load();
    expectCurrent(payload(true));
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[0][1]?.headers).toMatchObject({
      'X-Pulse-Org-ID': 'deleted-synthetic-workspace',
      'X-API-Token': 'synthetic-capability-reader',
    });
    expect(fetchMock.mock.calls[1][1]?.headers).not.toHaveProperty('X-Pulse-Org-ID');
    expect(api.getOrgID()).toBeNull();
    expect(api.getApiToken()).toBe('synthetic-capability-reader');
    expect(log.error).not.toHaveBeenCalled();
  });
});
