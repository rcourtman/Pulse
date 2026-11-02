import { cleanup, renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { DIAGNOSTICS_PANEL_COPY } from '@/utils/diagnosticsPresentation';
import type { DiagnosticsData } from '@/components/Settings/diagnosticsModel';

type UseDiagnosticsPanelStateModule = typeof import('../useDiagnosticsPanelState');
type URLStaticWithBlobMethods = {
  createObjectURL?: typeof URL.createObjectURL;
  revokeObjectURL?: typeof URL.revokeObjectURL;
};

const createDiagnosticsData = (): DiagnosticsData =>
  ({
    version: '6.0.0',
    runtime: 'go',
    uptime: 3600,
    nodes: [
      {
        id: 'node-a',
        name: 'pve-01',
        host: '10.0.0.5',
        type: 'pve',
        authMethod: 'token',
        connected: false,
        error: 'dial tcp 10.0.0.5:8006: connect: connection refused',
      },
    ],
    pbs: [],
    system: {
      os: 'linux',
      arch: 'amd64',
      goVersion: 'go1.25',
      numCPU: 8,
      numGoroutine: 32,
      memoryMB: 128,
    },
    discovery: {
      enabled: true,
      configuredSubnet: '10.0.0.0/24',
      activeSubnet: '10.0.1.0/24',
      environmentOverride: 'PULSE_DISCOVERY_SUBNET=10.0.2.0/24',
      subnetAllowlist: ['10.0.0.0/24'],
      subnetBlocklist: [],
    },
    errors: ['probe failed for 10.0.0.10 after timeout'],
    commercialFunnel: {
      enabled: true,
      summary: { pricing_viewed: 4, checkout_clicked: 1 },
    },
    infrastructureOnboarding: {
      enabled: true,
      summary: { opened: 4, credentials_opened: 1 },
      platforms: [{ key: 'truenas', catalog_selected: 2, credentials_opened: 1 }],
    },
  }) as DiagnosticsData;

const readBlobText = async (blob: Blob): Promise<string> =>
  await new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ''));
    reader.onerror = () => reject(reader.error ?? new Error('Failed to read exported diagnostics'));
    reader.readAsText(blob);
  });

const currentExportDate = (): string => new Date().toISOString().split('T')[0];

describe('useDiagnosticsPanelState', () => {
  let useDiagnosticsPanelState: UseDiagnosticsPanelStateModule['useDiagnosticsPanelState'];
  let apiFetchJSONMock: ReturnType<typeof vi.fn<(...args: any[]) => any>>;
  let currentOrg: string;
  let eventBus: (typeof import('@/stores/events'))['eventBus'];
  let createObjectURLMock: ReturnType<typeof vi.fn<(...args: any[]) => any>>;
  let revokeObjectURLMock: ReturnType<typeof vi.fn<(...args: any[]) => any>>;
  let anchorClickMock: ReturnType<typeof vi.fn<(...args: any[]) => any>>;
  let createdAnchor: HTMLAnchorElement | null;
  let originalCreateObjectURL: typeof URL.createObjectURL | undefined;
  let originalRevokeObjectURL: typeof URL.revokeObjectURL | undefined;

  beforeEach(async () => {
    vi.resetModules();

    apiFetchJSONMock = vi.fn();
    currentOrg = 'diagnostics-a';
    createObjectURLMock = vi.fn(() => 'blob:diagnostics-export');
    revokeObjectURLMock = vi.fn();
    anchorClickMock = vi.fn();
    createdAnchor = null;
    originalCreateObjectURL = URL.createObjectURL;
    originalRevokeObjectURL = URL.revokeObjectURL;

    vi.doMock('@/utils/apiClient', () => ({
      apiFetchJSON: apiFetchJSONMock,
      getOrgID: () => currentOrg,
    }));

    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURLMock,
    });
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURLMock,
    });

    const createElement = document.createElement.bind(document);
    vi.spyOn(document, 'createElement').mockImplementation(((tagName: string) => {
      const element = createElement(tagName);
      if (tagName.toLowerCase() === 'a') {
        createdAnchor = element as HTMLAnchorElement;
        createdAnchor.click = anchorClickMock;
      }
      return element;
    }) as typeof document.createElement);

    ({ useDiagnosticsPanelState } = await import('../useDiagnosticsPanelState'));
    ({ eventBus } = await import('@/stores/events'));
  });

  afterEach(() => {
    cleanup();
    if (originalCreateObjectURL) {
      Object.defineProperty(URL, 'createObjectURL', {
        configurable: true,
        value: originalCreateObjectURL,
      });
    } else {
      delete (URL as unknown as URLStaticWithBlobMethods).createObjectURL;
    }
    if (originalRevokeObjectURL) {
      Object.defineProperty(URL, 'revokeObjectURL', {
        configurable: true,
        value: originalRevokeObjectURL,
      });
    } else {
      delete (URL as unknown as URLStaticWithBlobMethods).revokeObjectURL;
    }
    vi.restoreAllMocks();
    vi.resetModules();
  });

  it('exports full diagnostics without internal analytics fields', async () => {
    const diagnosticsData = createDiagnosticsData();
    apiFetchJSONMock.mockResolvedValue(diagnosticsData);

    const { result } = renderHook(() => useDiagnosticsPanelState());

    await result.runDiagnostics();

    await waitFor(() => expect(result.diagnosticsData()).not.toBeNull());
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/diagnostics', {
      signal: expect.any(AbortSignal),
      expectedOrgID: 'diagnostics-a',
      retry: false,
    });
    expect(result.notice()).toBe('Diagnostics completed');
    expect(result.diagnosticsData()).not.toHaveProperty('commercialFunnel');
    expect(result.diagnosticsData()).not.toHaveProperty('infrastructureOnboarding');

    await result.exportDiagnostics(false);

    expect(createObjectURLMock).toHaveBeenCalledOnce();
    expect(anchorClickMock).toHaveBeenCalledOnce();
    expect(createdAnchor?.download).toBe(`pulse-diagnostics-full-${currentExportDate()}.json`);
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:diagnostics-export');
    expect(result.notice()).toBe('Full diagnostics downloaded — keep this file private');

    const payload = JSON.parse(
      await readBlobText(createObjectURLMock.mock.calls[0][0] as Blob),
    ) as DiagnosticsData & Record<string, unknown>;

    expect(payload.commercialFunnel).toBeUndefined();
    expect(payload.infrastructureOnboarding).toBeUndefined();
    expect(payload.nodes[0].host).toBe('10.0.0.5');
  });

  it('exports sanitized diagnostics without internal analytics fields', async () => {
    apiFetchJSONMock.mockResolvedValue(createDiagnosticsData());

    const { result } = renderHook(() => useDiagnosticsPanelState());

    await result.runDiagnostics();
    await waitFor(() => expect(result.diagnosticsData()).not.toBeNull());

    await result.exportDiagnostics(true);

    expect(anchorClickMock).toHaveBeenCalledOnce();
    expect(createdAnchor?.download).toBe(`pulse-diagnostics-sanitized-${currentExportDate()}.json`);
    expect(result.notice()).toBe('Sanitised diagnostics downloaded — review before sharing');

    const payload = JSON.parse(
      await readBlobText(createObjectURLMock.mock.calls[0][0] as Blob),
    ) as DiagnosticsData & Record<string, unknown>;

    expect(payload.nodes[0]).toEqual(
      expect.objectContaining({
        id: 'node-1',
        name: 'node-1',
        host: 'node-1',
      }),
    );
    expect(payload.discovery).toEqual(
      expect.objectContaining({
        configuredSubnet: '[REDACTED_SUBNET]',
        activeSubnet: '[REDACTED_SUBNET]',
      }),
    );
    expect(payload.errors).toEqual(['probe failed for [REDACTED_IP] after timeout']);
    expect(payload.commercialFunnel).toBeUndefined();
    expect(payload.infrastructureOnboarding).toBeUndefined();
  });

  it('opens with no result and makes no request just to prepare an export', async () => {
    const { result } = renderHook(() => useDiagnosticsPanelState());

    expect(result.diagnosticsData()).toBeNull();
    expect(result.loading()).toBe(false);
    expect(apiFetchJSONMock).not.toHaveBeenCalled();

    await result.exportDiagnostics(true);

    expect(result.error()).toBe(DIAGNOSTICS_PANEL_COPY.exportRequiresResult);
    expect(createObjectURLMock).not.toHaveBeenCalled();
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
  });

  const pendingResult = () => {
    let resolve!: (value: DiagnosticsData) => void;
    let reject!: (reason: Error) => void;
    const promise = new Promise<DiagnosticsData>((done, fail) => {
      resolve = done;
      reject = fail;
    });
    return { promise, resolve, reject };
  };

  it('withdraws the prior result at refresh start and keeps a denied refresh unknown', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(createDiagnosticsData());
    const { result } = renderHook(() => useDiagnosticsPanelState());
    await result.runDiagnostics();
    const pending = pendingResult();
    apiFetchJSONMock.mockReturnValueOnce(pending.promise);
    const request = result.runDiagnostics();
    expect(result.diagnosticsData()).toBeNull();
    expect(result.loading()).toBe(true);
    await result.exportDiagnostics(false);
    await result.runDiagnostics();
    expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
    expect(createObjectURLMock).not.toHaveBeenCalled();
    pending.reject(new Error('Denied for private.example.test?token=private-fixture'));
    await request;
    expect(result.loading()).toBe(false);
    expect(result.diagnosticsData()).toBeNull();
    expect(result.error()).toBe(DIAGNOSTICS_PANEL_COPY.runError);
    expect(result.notice()).toBe('');
    await result.exportDiagnostics(true);
    expect(createObjectURLMock).not.toHaveBeenCalled();
    expect(apiFetchJSONMock.mock.calls.every(([, options]) => options.retry === false)).toBe(true);
  });

  it.each(['user', 'access', 'permission', 'organization', 'same organization event'])(
    'erases accepted data and local feedback on %s change without live probes',
    async (change) => {
      const [user, setUser] = createSignal('reader-a');
      const [access, setAccess] = createSignal('token-a');
      const [allowed, setAllowed] = createSignal(true);
      apiFetchJSONMock.mockResolvedValue(createDiagnosticsData());
      const { result } = renderHook(() =>
        useDiagnosticsPanelState({
          get currentUser() {
            return user();
          },
          get accessKey() {
            return access();
          },
          get canRead() {
            return allowed();
          },
        }),
      );
      await result.runDiagnostics();
      expect(result.diagnosticsData()).not.toBeNull();
      expect(result.notice()).toBeTruthy();
      if (change === 'user') setUser('reader-b');
      if (change === 'access') setAccess('token-b');
      if (change === 'permission') setAllowed(false);
      if (change === 'organization') currentOrg = 'diagnostics-b';
      if (change.includes('organization')) eventBus.emit('org_switched', currentOrg);
      await waitFor(() => expect(result.diagnosticsData()).toBeNull());
      expect(result.notice()).toBe('');
      expect(result.error()).toBe('');
      expect(apiFetchJSONMock).toHaveBeenCalledOnce();
      await result.exportDiagnostics(false);
      expect(createObjectURLMock).not.toHaveBeenCalled();
      // Returning to the old identifiers never restores the old result.
      setUser('reader-a');
      setAccess('token-a');
      setAllowed(true);
      currentOrg = 'diagnostics-a';
      eventBus.emit('org_switched', currentOrg);
      expect(result.diagnosticsData()).toBeNull();
      expect(apiFetchJSONMock).toHaveBeenCalledOnce();
      await result.runDiagnostics();
      expect(result.diagnosticsData()).not.toBeNull();
      expect(apiFetchJSONMock).toHaveBeenCalledTimes(2);
    },
  );

  it('cancels obsolete work, ignores a late success and lets only an explicit new run recover', async () => {
    const obsolete = pendingResult();
    const fresh = pendingResult();
    apiFetchJSONMock.mockReturnValueOnce(obsolete.promise).mockReturnValueOnce(fresh.promise);
    const { result } = renderHook(() => useDiagnosticsPanelState());
    const oldRequest = result.runDiagnostics();
    const oldSignal = apiFetchJSONMock.mock.calls[0][1].signal as AbortSignal;
    currentOrg = 'diagnostics-b';
    eventBus.emit('org_switched', currentOrg);
    expect(oldSignal.aborted).toBe(true);
    expect(result.loading()).toBe(false);
    const newRequest = result.runDiagnostics();
    expect(apiFetchJSONMock.mock.calls[1][1].expectedOrgID).toBe('diagnostics-b');
    obsolete.resolve(createDiagnosticsData());
    await oldRequest;
    expect(result.diagnosticsData()).toBeNull();
    expect(result.loading()).toBe(true);
    expect(result.notice()).toBe('');
    fresh.resolve({ ...createDiagnosticsData(), version: 'current-reader' });
    await newRequest;
    expect(result.diagnosticsData()?.version).toBe('current-reader');
    expect(result.loading()).toBe(false);
  });

  it('ignores a late failure when the original reader returns during a new run', async () => {
    const old = pendingResult();
    const fresh = pendingResult();
    apiFetchJSONMock.mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
    const { result } = renderHook(() => useDiagnosticsPanelState());
    const oldRequest = result.runDiagnostics();
    currentOrg = 'diagnostics-b';
    eventBus.emit('org_switched', currentOrg);
    currentOrg = 'diagnostics-a';
    eventBus.emit('org_switched', currentOrg);
    const newRequest = result.runDiagnostics();
    old.reject(new Error('old private failure'));
    await oldRequest;
    expect(result.error()).toBe('');
    expect(result.loading()).toBe(true);
    fresh.resolve(createDiagnosticsData());
    await newRequest;
    expect(result.notice()).toBe(DIAGNOSTICS_PANEL_COPY.runSuccess);
  });

  it.each(['resolve', 'reject'] as const)(
    'cancels disposal and ignores late %s',
    async (completion) => {
      const pending = pendingResult();
      apiFetchJSONMock.mockReturnValueOnce(pending.promise);
      const { result, cleanup: dispose } = renderHook(() => useDiagnosticsPanelState());
      const request = result.runDiagnostics();
      const signal = apiFetchJSONMock.mock.calls[0][1].signal as AbortSignal;
      dispose();
      expect(signal.aborted).toBe(true);
      if (completion === 'resolve') pending.resolve(createDiagnosticsData());
      else pending.reject(new Error('obsolete private failure'));
      await request;
      expect(result.diagnosticsData()).toBeNull();
      expect(result.error()).toBe('');
      expect(result.notice()).toBe('');
      await result.exportDiagnostics(false);
      await result.runDiagnostics();
      expect(createObjectURLMock).not.toHaveBeenCalled();
      expect(apiFetchJSONMock).toHaveBeenCalledOnce();
    },
  );

  it('makes no request while read permission is absent or after restoration until Run', async () => {
    const [allowed, setAllowed] = createSignal(false);
    const { result } = renderHook(() =>
      useDiagnosticsPanelState({
        get canRead() {
          return allowed();
        },
      }),
    );
    await result.runDiagnostics();
    await result.exportDiagnostics(false);
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
    expect(createObjectURLMock).not.toHaveBeenCalled();
    setAllowed(true);
    await waitFor(() => expect(result.loading()).toBe(false));
    expect(apiFetchJSONMock).not.toHaveBeenCalled();
    apiFetchJSONMock.mockResolvedValue(createDiagnosticsData());
    await result.runDiagnostics();
    expect(result.diagnosticsData()).not.toBeNull();
  });

  it('preserves a successful result on an unchanged reader refresh', async () => {
    const [props, setProps] = createSignal({
      currentUser: 'same-reader',
      accessKey: 'same-key',
      canRead: true,
    });
    const { result } = renderHook(() =>
      useDiagnosticsPanelState({
        get currentUser() {
          return props().currentUser;
        },
        get accessKey() {
          return props().accessKey;
        },
        get canRead() {
          return props().canRead;
        },
      }),
    );
    apiFetchJSONMock.mockResolvedValue(createDiagnosticsData());
    await result.runDiagnostics();
    const accepted = result.diagnosticsData();
    setProps({ ...props() });
    expect(result.diagnosticsData()).toBe(accepted);
    expect(apiFetchJSONMock).toHaveBeenCalledOnce();
  });

  it.each([
    null,
    {},
    { nodes: [] },
    { ...createDiagnosticsData(), nodes: null },
    { ...createDiagnosticsData(), system: null },
    { ...createDiagnosticsData(), errors: null },
  ])(
    'keeps a malformed response unknown instead of displaying a successful empty inventory: %s',
    async (data) => {
      apiFetchJSONMock.mockResolvedValue(data);
      const { result } = renderHook(() => useDiagnosticsPanelState());
      await result.runDiagnostics();
      expect(result.diagnosticsData()).toBeNull();
      expect(result.notice()).toBe('');
      expect(result.error()).toBe(DIAGNOSTICS_PANEL_COPY.runError);
      await result.exportDiagnostics(false);
      expect(createObjectURLMock).not.toHaveBeenCalled();
    },
  );

  it('cleans up a failed local download and permits a download retry without probing', async () => {
    apiFetchJSONMock.mockResolvedValue(createDiagnosticsData());
    const { result } = renderHook(() => useDiagnosticsPanelState());
    await result.runDiagnostics();
    anchorClickMock.mockImplementationOnce(() => {
      throw new Error('Download blocked');
    });
    await result.exportDiagnostics(false);
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:diagnostics-export');
    expect(document.querySelector('a[download]')).toBeNull();
    expect(result.error()).toBe(DIAGNOSTICS_PANEL_COPY.exportError);
    expect(result.notice()).toBe('');
    expect(result.exportLoading()).toBe(false);
    await result.exportDiagnostics(false);
    expect(result.error()).toBe('');
    expect(result.notice()).toBe(DIAGNOSTICS_PANEL_COPY.exportFullSuccess);
    expect(apiFetchJSONMock).toHaveBeenCalledOnce();
  });
});
