import { cleanup, renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { eventBus } from '@/stores/events';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { useVMwareSettingsPanelState } from '../useVMwareSettingsPanelState';

// Keep the hook, VMware adapter, JSON/error body reader and API client real.
// Mocking VMwareAPI alone cannot detect lost signal/context/retry options.
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() } }));

const fetchMock = vi.fn<typeof fetch>();
const pendingBodies = new Set<() => void>();
const connection = (name = 'Original private source') => ({
  id: 'same-source-id',
  name,
  host: 'vc.example.invalid',
  username: 'monitor',
  password: '********',
  insecureSkipVerify: false,
  enabled: true,
  monitorVms: true,
  monitorHosts: true,
  monitorDatastores: true,
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

const ready = async (state: ReturnType<typeof useVMwareSettingsPanelState>) => {
  await waitFor(() => {
    expect(state.loading()).toBe(false);
    expect(state.connections()).toHaveLength(1);
  });
};

describe('VMware settings through the real API transport', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
    for (const [url, options] of fetchMock.mock.calls) {
      expect(String(url)).toMatch(/^\/api\/vmware\/connections(?:\/|$)/);
      expect(new Headers(options?.headers).get('X-Pulse-Org-ID')).toBeTruthy();
    }
    clearAuth();
    setOrgID(null);
    vi.unstubAllGlobals();
  });

  it.each(['organisation', 'user round trip', 'capability', 'disposal'] as const)(
    'ignores a delayed JSON body after %s withdrawal and aborts the actual fetch signal',
    async (change) => {
      const [canLoad, setAllowed] = createSignal(true);
      const [currentUser, setUser] = createSignal('reader-a');
      const { result, cleanup: unmount } = renderHook(() =>
        useVMwareSettingsPanelState({ canLoad, currentUser }),
      );
      await ready(result);
      const oldConnection = result.connections()[0];
      result.openEditDialog(oldConnection);
      result.updateForm({ password: 'synthetic-unsaved-key' });

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
      expect(result.form().password).toBe('');
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
    [429, { error: 'rate_limited' }, { 'Retry-After': '1' }],
  ] as const)(
    'does not replay a settings read or drop its organisation after HTTP %i',
    async (status, body, headers) => {
      const { result } = renderHook(() => useVMwareSettingsPanelState());
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
    const { result } = renderHook(() => useVMwareSettingsPanelState({ currentUser }));
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
    result.updateForm({ password: 'synthetic-new-reader-draft' });

    old.release({ error: 'obsolete-private-denial' });
    await reading;
    expect(result.connections()[0].name).toBe('Replacement source');
    expect(result.dialogOpen()).toBe(true);
    expect(result.form().password).toBe('synthetic-new-reader-draft');
    expect(result.loadingError()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(4);
  });
  it.each([
    'create',
    'update',
    'delete',
    'test',
    'saved-test',
    'preview',
    'saved-preview',
  ] as const)(
    'binds the actual %s request, never replays a denial and requires a fresh read',
    async (action) => {
      const { result } = renderHook(() => useVMwareSettingsPanelState());
      await ready(result);
      const saved = result.connections()[0];
      if (action === 'delete') result.openDeleteDialog(saved);
      else if (action !== 'saved-test') {
        if (['create', 'test', 'preview'].includes(action)) result.openCreateDialog();
        else result.openEditDialog(saved);
        result.updateForm({ host: saved.host, username: 'monitor', password: 'synthetic-draft' });
      }
      fetchMock.mockResolvedValueOnce(
        json({ error: 'private-denial-detail' }, 403, { 'X-CSRF-Token': 'synthetic-replacement' }),
      );
      if (action === 'delete') await result.deletePendingConnection();
      else if (action === 'saved-test') await result.testSavedConnection(saved);
      else if (['test', 'saved-test'].includes(action)) await result.testCurrentForm();
      else if (['preview', 'saved-preview'].includes(action)) await result.previewCurrentForm();
      else await result.saveCurrentForm();
      expect(fetchMock).toHaveBeenCalledTimes(2);
      const [url, options] = fetchMock.mock.calls[1];
      const suffix = {
        create: '',
        update: '/same-source-id',
        delete: '/same-source-id',
        test: '/test',
        'saved-test': '/same-source-id/test',
        preview: '/preview',
        'saved-preview': '/same-source-id/preview',
      }[action];
      expect(url).toBe(`/api/vmware/connections${suffix}`);
      expect(options?.method).toBe(
        action === 'update' ? 'PUT' : action === 'delete' ? 'DELETE' : 'POST',
      );
      expect(new Headers(options?.headers).get('X-Pulse-Org-ID')).toBe('north');
      expect(options?.signal?.aborted).toBe(true);
      expect(result.connections()).toEqual([]);
      expect(result.form().password).toBe('');
      expect(result.dialogOpen() || result.deleteDialogOpen()).toBe(false);
      expect(result.loadingError()).toContain('access could not be verified');
      await result.testSavedConnection(saved);
      await result.saveCurrentForm();
      expect(fetchMock).toHaveBeenCalledTimes(2);
      await result.loadConnections();
      await ready(result);
      result.openEditDialog(result.connections()[0]);
      expect(result.dialogOpen()).toBe(true);
      expect(result.form().hasStoredPassword).toBe(true);
    },
  );
});
