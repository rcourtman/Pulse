import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Keep the bootstrap helper and apiClient real: the rendered bootstrap suite
// mocks apiFetch, so it cannot detect a broken recovery/no-retry composition.
let bootstrap: typeof import('@/stores/organizationAccessBootstrap');
let client: typeof import('@/utils/apiClient');
const fetchMock = vi.fn<typeof fetch>();
const token = 'synthetic-bootstrap-token';
const username = 'synthetic-invited-reader';
const namedOrg = 'synthetic-client-east';

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });

const deferred = () => {
  let resolve!: (response: Response) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<Response>((done, fail) => {
    resolve = done;
    reject = fail;
  });
  return { promise, resolve, reject };
};

const expectCredentialsRetained = () => {
  expect(client.getApiToken()).toBe(token);
  expect(sessionStorage.getItem('pulse_auth_user')).toBe(JSON.stringify({ username }));
  expect(localStorage.getItem('just_logged_out')).toBeNull();
};

beforeEach(async () => {
  vi.resetModules();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  sessionStorage.clear();
  localStorage.clear();
  document.cookie = 'pulse_org_id=; Path=/; Max-Age=0';
  document.cookie = 'pulse_csrf=; Path=/; Max-Age=0';
  client = await import('@/utils/apiClient');
  bootstrap = await import('@/stores/organizationAccessBootstrap');
  client.setApiToken(token);
  sessionStorage.setItem('pulse_auth_user', JSON.stringify({ username }));
});

afterEach(() => {
  client.clearAuth();
  document.cookie = 'pulse_csrf=; Path=/; Max-Age=0';
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('no-organization response classification', () => {
  it.each([
    { status: 403, body: { error: 'default_org_access_denied' }, expected: true },
    { status: 200, body: { error: 'default_org_access_denied' }, expected: false },
    { status: 401, body: { error: 'default_org_access_denied' }, expected: false },
    { status: 503, body: { error: 'default_org_access_denied' }, expected: false },
    { status: 403, body: { error: 'access_denied' }, expected: false },
    { status: 403, body: { error: 'default_org_access_denied_later' }, expected: false },
    { status: 403, body: { message: 'default_org_access_denied' }, expected: false },
    { status: 403, body: null, expected: false },
  ])('classifies $status / $body without consuming the caller body', async (testCase) => {
    const response = json(testCase.status, testCase.body);

    expect(await bootstrap.isNoOrganizationAccessResponse(response)).toBe(testCase.expected);
    expect(response.bodyUsed).toBe(false);
    expect(await response.json()).toEqual(testCase.body);
    expect(fetchMock).not.toHaveBeenCalled();
    expectCredentialsRetained();
  });

  it('does not treat an unreadable or proxy response as a membership decision', async () => {
    const proxyResponse = new Response('<html>Unavailable</html>', { status: 403 });
    expect(await bootstrap.isNoOrganizationAccessResponse(proxyResponse)).toBe(false);
    expect(await proxyResponse.text()).toBe('<html>Unavailable</html>');

    const consumedResponse = json(403, { error: 'default_org_access_denied' });
    await consumedResponse.text();
    expect(await bootstrap.isNoOrganizationAccessResponse(consumedResponse)).toBe(false);
    expectCredentialsRetained();
  });
});

describe('no-organization probe through the real API transport', () => {
  it.each([
    { name: 'usable default', status: 200, body: {}, expected: false },
    {
      name: 'confirmed denial',
      status: 403,
      body: { error: 'default_org_access_denied' },
      expected: true,
    },
    {
      name: 'unavailable default',
      status: 503,
      body: { error: 'default_org_access_denied' },
      expected: false,
    },
  ])('recovers a stale default once before classifying $name', async (testCase) => {
    client.setOrgID('default');
    // Recovery must precede the ordinary CSRF retry, which would otherwise
    // repeat the obsolete selection and return before the unbound read.
    document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
    fetchMock
      .mockResolvedValueOnce(json(403, { error: 'default_org_access_denied' }))
      .mockResolvedValueOnce(json(testCase.status, testCase.body));

    expect(await bootstrap.probeNoOrganizationAccess()).toBe(testCase.expected);

    expect(fetchMock).toHaveBeenCalledTimes(2);
    const [bound, unbound] = fetchMock.mock.calls;
    for (const request of [bound, unbound]) {
      expect(request[0]).toBe('/api/state/summary');
      expect(request[1]).toMatchObject({
        credentials: 'include',
        headers: {
          'X-Requested-With': 'XMLHttpRequest',
          Accept: 'application/json',
          'X-API-Token': token,
        },
      });
      expect(request[1]?.method ?? 'GET').toBe('GET');
      expect(request[1]).not.toHaveProperty('recoverDefaultOrg');
    }
    expect(bound[1]?.headers).toHaveProperty('X-Pulse-Org-ID', 'default');
    expect(unbound[1]?.headers).not.toHaveProperty('X-Pulse-Org-ID');
    expect(client.getOrgID()).toBeNull();
    expect(sessionStorage.getItem('pulse_org_id')).toBeNull();
    expect(document.cookie).not.toContain('pulse_org_id=');
    expectCredentialsRetained();
  });

  it.each([
    { name: 'named membership refusal', status: 403, body: { error: 'access_denied' } },
    { name: 'expired authentication', status: 401, body: { error: 'unauthorized' } },
    { name: 'service failure', status: 503, body: { error: 'default_org_access_denied' } },
    { name: 'incomplete response', status: 403, body: {} },
  ])('preserves a selected organization and credentials after $name', async (testCase) => {
    client.setOrgID(namedOrg);
    fetchMock.mockResolvedValueOnce(json(testCase.status, testCase.body));

    expect(await bootstrap.probeNoOrganizationAccess()).toBe(false);

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock.mock.calls[0][1]?.headers).toHaveProperty('X-Pulse-Org-ID', namedOrg);
    expect(client.getOrgID()).toBe(namedOrg);
    expect(sessionStorage.getItem('pulse_org_id')).toBe(namedOrg);
    expect(document.cookie).toContain(`pulse_org_id=${namedOrg}`);
    expectCredentialsRetained();
  });

  it('keeps saved access when discovery rejects without an HTTP response', async () => {
    client.setOrgID(namedOrg);
    fetchMock.mockRejectedValueOnce(new TypeError('Synthetic connection failure'));

    expect(await bootstrap.probeNoOrganizationAccess()).toBe(false);
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(client.getOrgID()).toBe(namedOrg);
    expectCredentialsRetained();
  });
});

describe('deliberate sign-out without an organization', () => {
  const proxyLogout = '/oauth2/sign_out?rd=%2F';
  let assign: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    client.setOrgID(namedOrg);
    sessionStorage.setItem('synthetic-unrelated-state', 'keep');
    localStorage.setItem('pulseThemePreference', 'dark');
    document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
    assign = vi.fn(() => {
      // Assert the state at navigation, not merely after the helper returns.
      expect(client.getApiToken()).toBeNull();
      expect(client.getOrgID()).toBeNull();
      expect(sessionStorage.getItem('pulse_auth')).toBeNull();
      expect(sessionStorage.getItem('pulse_auth_user')).toBeNull();
      expect(sessionStorage.getItem('pulse_org_id')).toBeNull();
      expect(document.cookie).not.toContain('pulse_org_id=');
    });
    // Intercept only navigation; use the real session store and document.
    // jsdom does not implement navigation to the configured proxy logout page.
    vi.stubGlobal('window', {
      sessionStorage: window.sessionStorage,
      location: { protocol: 'http:', assign },
    });
    vi.spyOn(console, 'warn').mockImplementation(() => undefined);
  });

  it.each([200, 401, 403, 429, 503, 'network'] as const)(
    'settles one logout (%s) before clearing local auth and leaving through the proxy',
    async (outcome) => {
      const status = deferred();
      const logout = deferred();
      fetchMock.mockReturnValueOnce(status.promise).mockReturnValueOnce(logout.promise);

      const pending = bootstrap.signOutWithoutOrganization();
      expect(fetchMock).toHaveBeenCalledOnce();
      expectCredentialsRetained();
      expect(client.getOrgID()).toBe(namedOrg);
      expect(assign).not.toHaveBeenCalled();

      status.resolve(json(200, { proxyAuthLogoutURL: proxyLogout }));
      await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
      expectCredentialsRetained();
      expect(client.getOrgID()).toBe(namedOrg);
      expect(assign).not.toHaveBeenCalled();
      const [read, write] = fetchMock.mock.calls;
      expect(read[0]).toBe('/api/security/status');
      expect(write[0]).toBe('/api/logout');
      for (const request of [read, write]) {
        expect(request[1]).toMatchObject({
          credentials: 'include',
          headers: { 'X-API-Token': token, 'X-Pulse-Org-ID': 'default' },
        });
        expect(request[1]).not.toHaveProperty('skipOrgContext');
        expect(request[1]).not.toHaveProperty('retry');
      }
      expect(read[1]?.method ?? 'GET').toBe('GET');
      expect(write[1]).toMatchObject({
        method: 'POST',
        headers: { 'X-CSRF-Token': 'synthetic-csrf' },
      });

      if (outcome === 'network') {
        logout.reject(new TypeError('Synthetic logout connection failure'));
      } else {
        logout.resolve(json(outcome, { success: outcome === 200 }));
      }
      await pending;

      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(assign).toHaveBeenCalledExactlyOnceWith(proxyLogout);
      expect(sessionStorage.getItem('synthetic-unrelated-state')).toBe('keep');
      expect(localStorage.getItem('pulseThemePreference')).toBe('dark');
    },
  );

  it.each(['missing destination', 'empty destination', 'invalid JSON', 'network failure'])(
    'still signs out locally to the app root after status has %s',
    async (outcome) => {
      if (outcome === 'network failure') {
        fetchMock.mockRejectedValueOnce(new TypeError('Synthetic status failure'));
      } else if (outcome === 'invalid JSON') {
        fetchMock.mockResolvedValueOnce(new Response('<html>Unavailable</html>', { status: 503 }));
      } else {
        fetchMock.mockResolvedValueOnce(
          json(200, outcome === 'empty destination' ? { proxyAuthLogoutURL: '' } : {}),
        );
      }
      fetchMock.mockResolvedValueOnce(json(200, { success: true }));

      await bootstrap.signOutWithoutOrganization();

      expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
        '/api/security/status',
        '/api/logout',
      ]);
      expect(assign).toHaveBeenCalledExactlyOnceWith('/');
    },
  );
});
