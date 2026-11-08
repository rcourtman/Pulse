import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import type { APITokenRecord } from '@/api/security';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import { APITokenManager } from '../APITokenManager';

// Preserve the production panel, state, SecurityAPI and authenticated client.
// Only unrelated runtime/resource context is supplied by this fixture.
vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    markAgentsTokenRevoked: vi.fn(),
    markDockerRuntimesTokenRevoked: vi.fn(),
  }),
}));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ resources: () => [], byType: () => [] }),
}));
vi.mock('@/api/agentCapabilities', () => ({
  fetchAgentCapabilitiesManifest: () => Promise.resolve({ requiredScopes: [] }),
}));

const now = Date.parse('2026-11-08T21:30:00Z');
const original: APITokenRecord = {
  id: 'token/future',
  name: 'Same-name collector',
  prefix: 'test',
  suffix: '1234',
  createdAt: '2026-11-01T00:00:00Z',
  lastUsedAt: '2026-11-08T20:30:00Z',
  scopes: ['monitoring:read'],
  expiresAt: '2026-11-08T23:30:10+02:00',
};
const fetchMock = vi.fn<typeof fetch>();
const changed = vi.fn();
let records: APITokenRecord[];
let deny = false;
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const writes = () =>
  fetchMock.mock.calls.filter(([, init]) => init?.method && init.method !== 'GET');
const expiryIn = (root: HTMLElement) => root.querySelector(`[data-token-expiry="${original.id}"]`)!;
const surfaces = async () => {
  const cards = await screen.findAllByRole('article', { name: `API token ${original.name}` });
  const row = screen
    .getAllByText('test…1234')
    .map((e) => e.closest('tr'))
    .find((e) => e !== null)!;
  return [cards[0], row];
};

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] });
  vi.setSystemTime(now);
  clearAuth();
  setOrgID('fixture/estate');
  setApiToken('synthetic-expiry-token');
  records = [structuredClone(original)];
  deny = false;
  changed.mockClear();
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    expect(String(input)).toBe('/api/security/tokens');
    expect(init.method ?? 'GET').toBe('GET');
    expect(init).toMatchObject({
      credentials: 'include',
      signal: expect.any(AbortSignal),
      headers: { 'X-API-Token': 'synthetic-expiry-token', 'X-Pulse-Org-ID': 'fixture/estate' },
    });
    return deny ? json({ error: 'forbidden' }, 403) : json({ tokens: records });
  });
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  cleanup();
  clearAuth();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('saved expiry through the token inventory transport', () => {
  it.each([true, false])(
    'keeps both layouts accurate and read-only metadata with management=%s',
    async (canManage) => {
      records.push({ ...original, id: 'other', suffix: '5678', expiresAt: '2026-11-01T00:00:00Z' });
      render(() => (
        <APITokenManager
          canManage={canManage}
          currentUser="fixture-user"
          onTokensChanged={changed}
        />
      ));
      const [card, row] = await surfaces();
      for (const surface of [card, row]) {
        const expiry = expiryIn(surface);
        expect(expiry).toHaveTextContent('Expires 2026-11-08 21:30:10 UTC');
        expect(expiry.querySelector('time')).toHaveAttribute(
          'datetime',
          '2026-11-08T21:30:10.000Z',
        );
        expect(within(surface).getByTitle('monitoring:read')).toBeInTheDocument();
        for (const action of ['Edit scopes', 'Rename', 'Revoke'])
          expect(within(surface).getByRole('button', { name: action })).toHaveProperty(
            'disabled',
            !canManage,
          );
      }
      expect(screen.getAllByText('Expired 2026-11-01 00:00:00 UTC')).toHaveLength(2);
      expect(screen.queryByText(/Active credentials/)).not.toBeInTheDocument();
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(writes()).toEqual([]);
      expect(changed).not.toHaveBeenCalled();
    },
  );

  it.each([
    [undefined, 'No expiry'],
    [null, 'No expiry'],
    ['', 'Unavailable'],
    ['not-a-timestamp', 'Unavailable'],
    ['2026-02-30T00:00:00Z', 'Unavailable'],
  ])('keeps absent or unreadable saved metadata distinct: %s', async (expiresAt, label) => {
    records = [{ ...original, expiresAt } as APITokenRecord];
    render(() => <APITokenManager canManage currentUser="fixture-user" />);
    for (const surface of await surfaces()) {
      expect(expiryIn(surface)).toHaveTextContent(label);
      expect(expiryIn(surface).querySelector('time')).toBeNull();
    }
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(writes()).toEqual([]);
  });

  it('updates an open page at the shared clock tick without refetching or mutating lifetime', async () => {
    render(() => <APITokenManager canManage currentUser="fixture-user" />);
    const roots = await surfaces();
    for (const root of roots) expect(expiryIn(root)).toHaveTextContent('Expires');
    await vi.advanceTimersByTimeAsync(30_000);
    for (const root of roots)
      expect(expiryIn(root)).toHaveTextContent('Expired 2026-11-08 21:30:10 UTC');
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(writes()).toEqual([]);
    expect(records).toEqual([original]);
  });

  it('removes saved expiry with a withdrawn inventory, never replacing it with no expiry', async () => {
    const [user, setUser] = createSignal('fixture-user');
    render(() => <APITokenManager canManage currentUser={user()} />);
    await surfaces();
    deny = true;
    setUser('different-user');
    await waitFor(() =>
      expect(
        screen.queryByRole('article', { name: `API token ${original.name}` }),
      ).not.toBeInTheDocument(),
    );
    expect(document.querySelector('[data-token-expiry]')).toBeNull();
    expect(screen.queryByText('No expiry')).not.toBeInTheDocument();
    expect(screen.queryByText(/No tokens yet/)).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(writes()).toEqual([]);
  });
});
