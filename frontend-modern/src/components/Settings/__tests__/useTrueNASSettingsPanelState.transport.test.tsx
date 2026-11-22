import { cleanup, renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { eventBus } from '@/stores/events';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { useTrueNASSettingsPanelState } from '../useTrueNASSettingsPanelState';

// Keep the hook, TrueNAS adapter, JSON/error body reader and API client real.
// Mocking TrueNASAPI alone cannot detect lost signal/context/retry options.
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() } }));

const fetchMock = vi.fn<typeof fetch>();
let expectedActions: [string, string][];
const pendingBodies = new Set<() => void>();
const destinationRefusal = {
  code: 'destination_not_allowed',
  error: 'Synthetic upstream explanation that should not be displayed',
};
const connection = (name = 'Original private source') => ({
  id: 'same-source-id',
  name,
  host: 'nas.example.invalid',
  username: 'monitor',
  apiKey: '********',
  useHttps: true,
  insecureSkipVerify: false,
  fingerprint: 'SHA256:synthetic-pin',
  enabled: true,
  monitorDatasets: true,
  monitorPools: true,
  monitorReplication: true,
});
const json = (value: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });

// A real Response whose headers arrive before its body. The fetch fixture
// deliberately ignores abort, so authority must still be checked at completion.
const delayedBody = (status = 200) => {
  let controller!: ReadableStreamDefaultController<Uint8Array>;
  const body = new ReadableStream<Uint8Array>({
    start(stream) {
      controller = stream;
    },
  });
  const response = new Response(body, { status });
  const release = (value: unknown) => {
    if (!pendingBodies.delete(discard)) return;
    controller.enqueue(new TextEncoder().encode(JSON.stringify(value)));
    controller.close();
  };
  const discard = () => release([]);
  pendingBodies.add(discard);
  return { response, release };
};

const ready = async (state: ReturnType<typeof useTrueNASSettingsPanelState>) => {
  await waitFor(() => {
    expect(state.loading()).toBe(false);
    expect(state.connections()).toHaveLength(1);
  });
};

describe('TrueNAS settings through the real API transport', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    expectedActions = [];
    fetchMock.mockReset().mockImplementation(async () => json([connection('Replacement source')]));
    fetchMock.mockResolvedValueOnce(json([connection()]));
    vi.stubGlobal('fetch', fetchMock);
    sessionStorage.clear();
    localStorage.clear();
    document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
    clearAuth();
    setOrgID('north');
    setApiToken('synthetic-settings-token');
  });

  afterEach(() => {
    cleanup();
    for (const discard of pendingBodies) discard();
    const actions = fetchMock.mock.calls.flatMap(([url, options]) => {
      const method = options?.method ?? 'GET';
      return url === '/api/truenas/connections' && method === 'GET' ? [] : [[method, String(url)]];
    });
    clearAuth();
    setOrgID(null);
    vi.unstubAllGlobals();
    expect(actions).toEqual(expectedActions);
  });

  it.each([
    ['new', 'test', 'POST', '/api/truenas/connections/test'],
    ['saved', 'test', 'POST', '/api/truenas/connections/same-source-id/test'],
    ['new', 'save', 'POST', '/api/truenas/connections'],
    ['saved', 'save', 'PUT', '/api/truenas/connections/same-source-id'],
  ] as const)(
    'preserves the %s draft after a %s destination refusal without replaying',
    async (mode, action, method, endpoint) => {
      const { result } = renderHook(() => useTrueNASSettingsPanelState());
      await ready(result);
      if (mode === 'saved') result.openEditDialog(result.connections()[0]);
      else result.openCreateDialog();
      result.updateForm({
        name: 'Retained draft',
        host: 'nas.example.invalid',
        username: 'monitor',
        ...(mode === 'new' ? { apiKey: 'fixture-unsaved-key' } : {}),
        monitorDatasets: false,
      });
      const draft = result.form();
      expectedActions = [[method, endpoint]];
      fetchMock.mockResolvedValueOnce(
        json(destinationRefusal, 403, { 'X-CSRF-Token': 'fixture-no-replay' }),
      );

      if (action === 'test') expect(await result.testCurrentForm()).toBe(false);
      else await result.saveCurrentForm();

      expect(result.connectionFeedback()).toEqual({ kind: 'destination-refused' });
      expect(result.dialogOpen()).toBe(true);
      expect(result.form()).toEqual(draft);
      expect(result.connections().map((item) => item.name)).toEqual(['Original private source']);
      expect(result.loadingError()).toBeNull();
      expect(fetchMock).toHaveBeenCalledTimes(2);
      const options = fetchMock.mock.calls[1][1];
      expect(new Headers(options?.headers).get('X-Pulse-Org-ID')).toBe('north');
      expect(JSON.parse(String(options?.body))).toMatchObject({
        apiKey: mode === 'saved' ? '********' : 'fixture-unsaved-key',
        useHttps: true,
        insecureSkipVerify: false,
        monitorDatasets: false,
      });
      result.updateForm({ host: 'corrected.example.invalid' });
      expect(result.connectionFeedback()).toBeNull();
      expect(fetchMock).toHaveBeenCalledTimes(2);
    },
  );

  it('does not replace the shared sign-in recovery on Test HTTP 401', async () => {
    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await ready(result);
    result.openEditDialog(result.connections()[0]);
    expectedActions = [['POST', '/api/truenas/connections/same-source-id/test']];
    fetchMock.mockResolvedValueOnce(json(destinationRefusal, 401));

    expect(await result.testCurrentForm()).toBe(false);
    // The real API client clears auth and starts navigation before the hook
    // reads the error. JSDOM does not unmount the route on navigation.
    expect(getOrgID()).toBeNull();
    expect(localStorage.getItem('just_logged_out')).toBe('true');
    expect(result.connectionFeedback()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('still withdraws on a permission denial mentioning the code only as text', async () => {
    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await ready(result);
    result.openEditDialog(result.connections()[0]);
    result.updateForm({ apiKey: 'fixture-unsaved-key' });
    expectedActions = [['POST', '/api/truenas/connections/same-source-id/test']];
    fetchMock.mockResolvedValueOnce(
      json({ code: 'permission_denied', error: 'destination_not_allowed' }, 403),
    );

    expect(await result.testCurrentForm()).toBe(false);
    expect(result.dialogOpen()).toBe(false);
    expect(result.form().apiKey).toBe('');
    expect(result.connections()).toEqual([]);
    expect(result.connectionFeedback()).toBeNull();
    expect(result.loadingError()).toContain('access could not be verified');
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('does not make the Test and Save exception on a preview request', async () => {
    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await ready(result);
    result.openEditDialog(result.connections()[0]);
    expectedActions = [['POST', '/api/truenas/connections/same-source-id/preview']];
    fetchMock.mockResolvedValueOnce(json(destinationRefusal, 403));

    expect(await result.previewCurrentForm()).toBeNull();
    expect(result.dialogOpen()).toBe(false);
    expect(result.connectionFeedback()).toBeNull();
    expect(result.loadingError()).toContain('access could not be verified');
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('does not show a delayed destination refusal after changing the tested address', async () => {
    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await ready(result);
    result.openEditDialog(result.connections()[0]);
    const old = delayedBody(403);
    fetchMock.mockResolvedValueOnce(old.response);
    expectedActions = [['POST', '/api/truenas/connections/same-source-id/test']];
    const testing = result.testCurrentForm();
    await waitFor(() => expect(old.response.body?.locked).toBe(true));
    result.updateForm({ host: 'corrected.example.invalid' });
    old.release(destinationRefusal);

    expect(await testing).toBe(false);
    expect(result.connectionFeedback()).toBeNull();
    expect(result.loadingError()).toBeNull();
    expect(result.dialogOpen()).toBe(true);
    expect(result.form().host).toBe('corrected.example.invalid');
    expect(result.testing()).toBe(false);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it.each(['organisation', 'user round trip', 'capability', 'disposal'] as const)(
    'ignores a delayed JSON body after %s withdrawal and aborts the actual fetch signal',
    async (change) => {
      const [canLoad, setAllowed] = createSignal(true);
      const [currentUser, setUser] = createSignal('reader-a');
      const { result, cleanup: unmount } = renderHook(() =>
        useTrueNASSettingsPanelState({ canLoad, currentUser }),
      );
      await ready(result);
      const oldConnection = result.connections()[0];
      result.openEditDialog(oldConnection);
      result.updateForm({ apiKey: 'synthetic-unsaved-key' });

      const old = delayedBody();
      fetchMock.mockResolvedValueOnce(old.response);
      const reading = result.loadConnections();
      await waitFor(() => expect(old.response.body?.locked).toBe(true));
      const options = fetchMock.mock.calls[1][1]!;
      expect(options.signal).toBeDefined();
      expect(options.signal?.aborted).toBe(false);
      expect(options.credentials).toBe('include');
      expect(new Headers(options.headers).get('X-Pulse-Org-ID')).toBe('north');
      expect(options).not.toHaveProperty('expectedOrgID');
      expect(options).not.toHaveProperty('retry');

      if (change === 'organisation') {
        setOrgID('south');
        eventBus.emit('org_switched', 'south');
        await ready(result);
        expect(new Headers(fetchMock.mock.calls[2][1]?.headers).get('X-Pulse-Org-ID')).toBe(
          'south',
        );
      } else if (change === 'user round trip') {
        setUser('reader-b');
        await ready(result);
        setUser('reader-a');
        await ready(result);
      } else if (change === 'capability') {
        setAllowed(false);
      } else {
        unmount();
      }

      expect(options.signal?.aborted).toBe(true);
      expect(result.dialogOpen()).toBe(false);
      expect(result.form().apiKey).toBe('');
      old.release([connection('Obsolete private source')]);
      await reading;

      expect(result.connections().map((item) => item.name)).toEqual(
        change === 'organisation' || change === 'user round trip' ? ['Replacement source'] : [],
      );
      expect(result.loadingError()).toBeNull();
      // A same-ID connection from the previous reader is still not actionable.
      const requests = fetchMock.mock.calls.length;
      result.openEditDialog(oldConnection);
      await result.testSavedConnection(oldConnection);
      expect(result.dialogOpen()).toBe(false);
      expect(fetchMock).toHaveBeenCalledTimes(requests);
    },
  );

  it.each([
    [400, { error: 'invalid_org' }, {}],
    [403, { error: 'private-denial-detail' }, { 'X-CSRF-Token': 'synthetic-replacement' }],
    [403, destinationRefusal, {}],
    [429, { error: 'rate_limited' }, { 'Retry-After': '1' }],
  ] as const)(
    'does not replay a settings read or drop its organisation after HTTP %i',
    async (status, body, headers) => {
      const { result } = renderHook(() => useTrueNASSettingsPanelState());
      await ready(result);
      fetchMock.mockResolvedValueOnce(json(body, status, headers));
      await result.loadConnections();
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(getOrgID()).toBe('north');
      expect(result.connections()).toEqual([]);
      expect(result.loadingError()).toBeTruthy();
      if (status === 403) {
        expect(result.loadingError()).toContain('access could not be verified');
        expect(result.loadingError()).not.toContain('private-denial-detail');
      }
    },
  );

  it('does not apply an old delayed denial to a new same-user reader', async () => {
    const [currentUser, setUser] = createSignal('reader-a');
    const { result } = renderHook(() => useTrueNASSettingsPanelState({ currentUser }));
    await ready(result);
    const old = delayedBody(403);
    fetchMock.mockResolvedValueOnce(old.response);
    const reading = result.loadConnections();
    await waitFor(() => expect(old.response.body?.locked).toBe(true));
    setUser('reader-b');
    await ready(result);
    setUser('reader-a');
    await ready(result);
    result.openEditDialog(result.connections()[0]);
    result.updateForm({ apiKey: 'synthetic-new-reader-draft' });

    old.release({ error: 'obsolete-private-denial' });
    await reading;
    expect(result.connections()[0].name).toBe('Replacement source');
    expect(result.dialogOpen()).toBe(true);
    expect(result.form().apiKey).toBe('synthetic-new-reader-draft');
    expect(result.loadingError()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(4);
  });
});
