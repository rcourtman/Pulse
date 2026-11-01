import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const fetchMock = vi.fn();
let api: typeof import('../security');
let client: typeof import('@/utils/apiClient');
beforeEach(async () => {
  vi.resetModules(); fetchMock.mockReset(); vi.stubGlobal('fetch', fetchMock);
  sessionStorage.clear(); document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  client = await import('@/utils/apiClient'); api = await import('../security');
  client.setOrgID('org-a'); client.setApiToken('synthetic-only-auth');
});
afterEach(() => { client.clearAuth(); vi.unstubAllGlobals(); vi.unstubAllEnvs(); });
const invoke = (operation: string, options: { signal?: AbortSignal; expectedOrgID?: string }) => {
  if (operation === 'create') return api.SecurityAPI.createToken('Test', ['monitoring:read'], options);
  if (operation === 'rename') return api.SecurityAPI.renameToken('token/1', 'Renamed', options);
  if (operation === 'scope') return api.SecurityAPI.updateTokenScopes('token/1', ['monitoring:read'], options);
  return api.SecurityAPI.deleteToken('token/1', options);
};
describe('API token real transport', () => {
  it.each(['create', 'rename', 'scope', 'revoke'])('does not dispatch an aborted %s after CSRF preparation', async (operation) => {
    vi.stubEnv('MODE', 'production');
    let resolve!: (response: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise<Response>((yes) => { resolve = yes; }));
    const controller = new AbortController();
    const writing = invoke(operation, { expectedOrgID: 'org-a', signal: controller.signal });
    const result = expect(writing).rejects.toMatchObject({ name: 'AbortError' });
    expect(fetchMock.mock.calls[0][0]).toBe('/api/health');
    controller.abort(); resolve(new Response('{}', { status: 200 })); await result;
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('does not dispatch a confirmation into a different organisation after CSRF preparation', async () => {
    vi.stubEnv('MODE', 'production');
    let resolve!: (response: Response) => void;
    fetchMock.mockImplementationOnce(() => new Promise<Response>((yes) => { resolve = yes; }));
    const writing = invoke('create', { expectedOrgID: 'org-a' });
    const result = expect(writing).rejects.toThrow('Organisation changed.');
    client.setOrgID('org-b'); resolve(new Response('{}', { status: 200 })); await result;
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it.each(['create', 'rename', 'scope', 'revoke'].flatMap((operation) => [403, 429, 503].map((status) => ({ operation, status }))))('does not replay $operation after HTTP $status', async ({ operation, status }) => {
    fetchMock.mockResolvedValueOnce(new Response('{}', { status, headers: { 'X-CSRF-Token': 'new-synthetic-csrf', 'Retry-After': '0' } }));
    await expect(invoke(operation, { expectedOrgID: 'org-a' })).rejects.toMatchObject({ status });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ headers: { 'X-Pulse-Org-ID': 'org-a', 'X-API-Token': 'synthetic-only-auth' }, credentials: 'include' });
    expect(client.getOrgID()).toBe('org-a');
  });

  it('keeps a refused inventory unavailable instead of silently falling back to default', async () => {
    fetchMock.mockResolvedValueOnce(new Response('{"error":"invalid_org"}', { status: 400 }));
    await expect(api.SecurityAPI.listTokens({ expectedOrgID: 'org-a' })).rejects.toMatchObject({ status: 400 });
    expect(fetchMock).toHaveBeenCalledTimes(1); expect(client.getOrgID()).toBe('org-a');
  });
});
