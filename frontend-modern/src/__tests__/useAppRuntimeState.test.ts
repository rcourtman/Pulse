import { createRoot } from 'solid-js';
import { waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import useAppRuntimeStateSource from '@/useAppRuntimeState.ts?raw';
import type { State } from '@/types/api';
import type { Resource } from '@/types/resource';

type UseAppRuntimeStateModule = typeof import('@/useAppRuntimeState');

const flushAsync = async () => {
  for (let i = 0; i < 8; i += 1) {
    await Promise.resolve();
  }
};

const makeWebSocketState = (overrides: Partial<State> = {}): State => ({
  connectedInfrastructure: [],
  metrics: [],
  performance: {
    apiCallDuration: {},
    lastPollDuration: 0,
    pollingStartTime: '',
    totalApiCalls: 0,
    failedApiCalls: 0,
    cacheHits: 0,
    cacheMisses: 0,
  },
  connectionHealth: {},
  stats: {
    startTime: new Date().toISOString(),
    uptime: 0,
    pollingCycles: 0,
    webSocketClients: 0,
    version: '0.0.0',
  },
  activeAlerts: [],
  recentlyResolved: [],
  lastUpdate: 0,
  pveTagColors: {},
  resources: [],
  ...overrides,
});

const makeTrueNASResource = (id = 'truenas-1'): Resource => ({
  id,
  name: id,
  displayName: id,
  type: 'agent',
  platformId: id,
  platformType: 'truenas',
  sourceType: 'api',
  sources: ['truenas'],
  status: 'online',
  lastSeen: 1_700_000_000_000,
});

describe('useAppRuntimeState', () => {
  let useAppRuntimeState: UseAppRuntimeStateModule['useAppRuntimeState'];
  let apiFetchMock: ReturnType<typeof vi.fn>;
  let orgsListMock: ReturnType<typeof vi.fn>;
  let loadLicenseStatusMock: ReturnType<typeof vi.fn>;
  let loadCommercialPostureMock: ReturnType<typeof vi.fn>;
  let isMultiTenantEnabledMock: ReturnType<typeof vi.fn>;
  let isHostedModeEnabledMock: ReturnType<typeof vi.fn>;
  let getOrgIDMock: ReturnType<typeof vi.fn>;
  let hasStoredAuthSessionMock: ReturnType<typeof vi.fn>;
  let setOrgIDMock: ReturnType<typeof vi.fn>;
  let showToastMock: ReturnType<typeof vi.fn>;
  let aiChatSetEnabledMock: ReturnType<typeof vi.fn>;
  let getSystemSettingsMock: ReturnType<typeof vi.fn>;
  let getRuntimeDisplayMock: ReturnType<typeof vi.fn>;
  let applyServerModeMock: ReturnType<typeof vi.fn>;
  let updateRuntimeDisplayFromResponseMock: ReturnType<typeof vi.fn>;
  let markSystemSettingsLoadedWithDefaultsMock: ReturnType<typeof vi.fn>;
  let apiFetchJSONMock: ReturnType<typeof vi.fn>;
  let websocketState: State;
  let websocketConnected: boolean;
  let websocketReconnecting: boolean;
  let websocketInitialDataReceived: boolean;

  beforeEach(async () => {
    vi.resetModules();
    window.history.replaceState({}, '', '/');
    window.sessionStorage.clear();

    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      configurable: true,
      value: vi.fn().mockReturnValue({
        matches: false,
        media: '(prefers-color-scheme: dark)',
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }),
    });

    apiFetchMock = vi.fn(async (url: string) => {
      if (url === '/api/state/summary') {
        return new Response(
          JSON.stringify({ activeAlerts: 0, nodes: 0, vms: 0, containers: 0, dockerHosts: [] }),
          { status: 200 },
        );
      }
      if (url === '/api/security/status') {
        return new Response(JSON.stringify({ hasAuthentication: true }), { status: 200 });
      }
      if (url === '/api/state') {
        return new Response('{}', { status: 200 });
      }
      if (url === '/api/health') {
        return new Response('{}', { status: 200 });
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });
    apiFetchJSONMock = vi.fn(async (url: string) => {
      if (url.startsWith('/api/resources')) {
        return {
          aggregations: {
            platformAdmission: {
              proxmox: true,
              docker: false,
              kubernetes: false,
              truenas: false,
              vmware: false,
              standalone: false,
            },
          },
        };
      }
      throw new Error(`Unhandled apiFetchJSON URL: ${url}`);
    });
    orgsListMock = vi.fn().mockResolvedValue([{ id: 'acme', displayName: 'Acme' }]);
    loadLicenseStatusMock = vi.fn().mockResolvedValue(undefined);
    loadCommercialPostureMock = vi.fn().mockResolvedValue(undefined);
    isMultiTenantEnabledMock = vi.fn().mockReturnValue(false);
    isHostedModeEnabledMock = vi.fn().mockReturnValue(false);
    getOrgIDMock = vi.fn().mockReturnValue('default');
    hasStoredAuthSessionMock = vi.fn().mockReturnValue(true);
    setOrgIDMock = vi.fn();
    showToastMock = vi.fn();
    aiChatSetEnabledMock = vi.fn();
    getSystemSettingsMock = vi.fn().mockResolvedValue({ theme: '' });
    getRuntimeDisplayMock = vi.fn().mockResolvedValue({ theme: '', fullWidthMode: false });
    applyServerModeMock = vi.fn();
    updateRuntimeDisplayFromResponseMock = vi.fn();
    markSystemSettingsLoadedWithDefaultsMock = vi.fn();
    websocketState = makeWebSocketState();
    websocketConnected = false;
    websocketReconnecting = false;
    websocketInitialDataReceived = false;

    vi.doMock('@/stores/websocket-global', () => ({
      getGlobalWebSocketStore: () => ({
        state: websocketState,
        connected: () => websocketConnected,
        reconnecting: () => websocketReconnecting,
        initialDataReceived: () => websocketInitialDataReceived,
        resourceSnapshotReceived: () => websocketInitialDataReceived,
        reconnect: vi.fn(),
        switchUrl: vi.fn(),
      }),
    }));

    vi.doMock('@/utils/logger', () => ({
      logger: {
        debug: vi.fn(),
        info: vi.fn(),
        warn: vi.fn(),
        error: vi.fn(),
      },
    }));

    vi.doMock('@/constants', () => ({
      POLLING_INTERVALS: { DATA_FLASH: 50 },
    }));

    vi.doMock('@/utils/localStorage', () => ({
      STORAGE_KEYS: {
        AUTH: 'auth',
        AUTH_USER: 'pulse_auth_user',
        REMEMBERED_LOGIN_USERNAME: 'pulse_remembered_login_username',
        ORG_ID: 'org_id',
        GUEST_METADATA: 'guest_metadata',
        DOCKER_METADATA: 'docker_metadata',
      },
    }));

    vi.doMock('@/api/orgs', () => ({
      OrgsAPI: {
        list: orgsListMock,
      },
    }));

    vi.doMock('@/api/settings', () => ({
      SettingsAPI: {
        getSystemSettings: getSystemSettingsMock,
        getRuntimeDisplay: getRuntimeDisplayMock,
        updateSystemSettings: vi.fn(),
      },
    }));

    vi.doMock('@/utils/apiClient', () => ({
      apiFetch: apiFetchMock,
      apiFetchJSON: apiFetchJSONMock,
      getOrgID: getOrgIDMock,
      hasAuth: hasStoredAuthSessionMock,
      setOrgID: setOrgIDMock,
    }));

    vi.doMock('@/stores/events', () => ({
      eventBus: {
        on: vi.fn(),
        off: vi.fn(),
        emit: vi.fn(),
      },
    }));

    vi.doMock('@/utils/toast', () => ({
      showToast: showToastMock,
    }));

    vi.doMock('@/stores/updates', () => ({
      updateStore: {
        versionInfo: vi.fn().mockReturnValue(null),
        checkForUpdates: vi.fn().mockResolvedValue(undefined),
      },
    }));

    vi.doMock('@/stores/alertsActivation', () => ({
      useAlertsActivation: () => ({
        refreshConfig: vi.fn().mockResolvedValue(undefined),
        refreshActiveAlerts: vi.fn().mockResolvedValue(undefined),
      }),
    }));

    vi.doMock('@/utils/theme', () => ({
      applyThemeClass: vi.fn(),
      computeIsDark: vi.fn().mockReturnValue(false),
      getStoredThemePreference: vi.fn().mockReturnValue('system'),
      hasStoredThemePreference: vi.fn().mockReturnValue(false),
      normalizeThemePreference: vi.fn((value: string) => value),
      persistThemePreference: vi.fn(),
    }));

    vi.doMock('@/utils/url', () => ({
      initKioskMode: vi.fn(),
      getPulseWebSocketUrl: vi.fn().mockReturnValue('ws://127.0.0.1/ws'),
    }));

    vi.doMock('@/hooks/useKioskMode', () => ({
      syncKioskMode: vi.fn(),
    }));

    vi.doMock('@/stores/license', () => ({
      isHostedModeEnabled: isHostedModeEnabledMock,
      isMultiTenantEnabled: isMultiTenantEnabledMock,
      runtimeCapabilitiesLoaded: vi.fn().mockReturnValue(true),
      loadRuntimeCapabilities: loadLicenseStatusMock,
    }));

    vi.doMock('@/stores/aiChat', () => ({
      aiChatStore: {
        setEnabled: aiChatSetEnabledMock,
      },
    }));

    vi.doMock('@/stores/licenseCommercial', () => ({
      loadCommercialPosture: loadCommercialPostureMock,
    }));

    vi.doMock('@/utils/layout', () => ({
      layoutStore: {
        applyServerMode: applyServerModeMock,
      },
    }));

    vi.doMock('@/stores/systemSettings', () => ({
      loadRuntimeBranding: vi.fn().mockResolvedValue(undefined),
      markSystemSettingsLoadedWithDefaults: markSystemSettingsLoadedWithDefaultsMock,
      updateRuntimeDisplayFromResponse: updateRuntimeDisplayFromResponseMock,
    }));

    ({ useAppRuntimeState } = await import('@/useAppRuntimeState'));

    // Reset the freshly-imported sessionPresentationPolicy signal to its
    // defaults so a prior test's demo-mode mutation (in this file or in a
    // sibling that imports the same module) cannot leak into the
    // loadOrganizations branch selection inside useAppRuntimeState.
    const policyModule = await import('@/stores/sessionPresentationPolicy');
    policyModule.syncSessionPresentationPolicy(null);
  });

  afterEach(async () => {
    vi.clearAllMocks();
    const policyModule = await import('@/stores/sessionPresentationPolicy');
    policyModule.syncSessionPresentationPolicy(null);
    vi.resetModules();
  });

  const mountHook = () => {
    let dispose = () => {};
    let hookState: ReturnType<UseAppRuntimeStateModule['useAppRuntimeState']>;

    createRoot((d) => {
      dispose = d;
      hookState = useAppRuntimeState();
    });

    return { dispose, hookState: hookState! };
  };

  it('loads bootstrap display settings from the authenticated runtime projection', async () => {
    getRuntimeDisplayMock.mockResolvedValue({
      theme: 'dark',
      fullWidthMode: true,
      disableDockerUpdateActions: true,
      reduceProUpsellNoise: false,
    });
    const { dispose } = mountHook();

    await waitFor(() => {
      expect(getRuntimeDisplayMock).toHaveBeenCalled();
    });

    expect(getSystemSettingsMock).not.toHaveBeenCalled();
    expect(updateRuntimeDisplayFromResponseMock).toHaveBeenCalledWith(
      expect.objectContaining({ disableDockerUpdateActions: true }),
    );
    expect(applyServerModeMock).toHaveBeenCalledWith(true);
    expect(markSystemSettingsLoadedWithDefaultsMock).not.toHaveBeenCalled();

    dispose();
  });

  it('falls back to client defaults when runtime display settings are unavailable', async () => {
    getRuntimeDisplayMock.mockRejectedValue(new Error('404'));
    const { dispose } = mountHook();

    await waitFor(() => {
      expect(markSystemSettingsLoadedWithDefaultsMock).toHaveBeenCalled();
    });

    expect(getSystemSettingsMock).not.toHaveBeenCalled();

    dispose();
  });

  it('stays on the default organization path when multi-tenant is not enabled', async () => {
    isMultiTenantEnabledMock.mockReturnValue(false);
    const { hookState, dispose } = mountHook();

    await waitFor(() => {
      expect(setOrgIDMock).toHaveBeenCalledWith('default');
    });

    expect(orgsListMock).not.toHaveBeenCalled();
    expect(hookState.organizations()).toEqual([
      { id: 'default', displayName: 'Default Organization' },
    ]);
    expect(hookState.activeOrgID()).toBe('default');
    expect(hookState.showOrgSwitcher()).toBe(false);
    expect(showToastMock).not.toHaveBeenCalledWith(
      'error',
      'Failed to load organizations. Using default.',
    );

    dispose();
  });

  it('loads organizations from the org API when multi-tenant is enabled', async () => {
    isMultiTenantEnabledMock.mockReturnValue(true);
    getOrgIDMock.mockReturnValue('acme');
    const { hookState, dispose } = mountHook();

    await waitFor(() => {
      expect(orgsListMock).toHaveBeenCalledOnce();
    });

    expect(setOrgIDMock).toHaveBeenCalledWith('acme');
    expect(hookState.organizations()).toEqual([{ id: 'acme', displayName: 'Acme' }]);
    expect(hookState.activeOrgID()).toBe('acme');
    expect(hookState.showOrgSwitcher()).toBe(true);

    dispose();
  });

  it('syncs demo mode from security status session capabilities during bootstrap', async () => {
    isMultiTenantEnabledMock.mockReturnValue(true);
    apiFetchMock.mockImplementation(async (url: string) => {
      if (url === '/api/state/summary') {
        return new Response(
          JSON.stringify({ activeAlerts: 0, nodes: 0, vms: 0, containers: 0, dockerHosts: [] }),
          { status: 200 },
        );
      }
      if (url === '/api/security/status') {
        return new Response(
          JSON.stringify({
            hasAuthentication: true,
            sessionCapabilities: { demoMode: true, assistantEnabled: true },
          }),
          { status: 200 },
        );
      }
      if (url === '/api/state') {
        return new Response('{}', { status: 200 });
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    const presentationPolicyModule = await import('@/stores/sessionPresentationPolicy');
    const { hookState, dispose } = mountHook();

    await flushAsync();
    await flushAsync();

    expect(presentationPolicyModule.sessionPresentationPolicyResolved()).toBe(true);
    expect(presentationPolicyModule.presentationPolicyIsDemoMode()).toBe(true);
    expect(aiChatSetEnabledMock).toHaveBeenCalledWith(true);
    expect(orgsListMock).not.toHaveBeenCalled();
    expect(hookState.organizations()).toEqual([
      { id: 'default', displayName: 'Default Organization' },
    ]);
    expect(hookState.showOrgSwitcher()).toBe(false);

    dispose();
  });

  it('uses the SSO display name while bootstrapping TrueNAS state before websocket data', async () => {
    const principal = 'sso:oidc:test-oidc:stable-principal';
    const bootstrapResource = makeTrueNASResource('truenas-sso');
    apiFetchMock.mockImplementation(async (url: string) => {
      if (url === '/api/state/summary') {
        return new Response(
          JSON.stringify({ activeAlerts: 0, nodes: 0, vms: 0, containers: 0, dockerHosts: [] }),
          { status: 200 },
        );
      }
      if (url === '/api/security/status') {
        return new Response(
          JSON.stringify({
            hasAuthentication: true,
            ssoEnabled: true,
            ssoSessionUsername: principal,
            ssoSessionDisplayName: 'alice@example.com',
            ssoLogoutURL: '/api/oidc/test-oidc/logout',
          }),
          { status: 200 },
        );
      }
      if (url === '/api/state') {
        return new Response(
          JSON.stringify({
            resources: [bootstrapResource],
            lastUpdate: 1_700_000_000_000,
          }),
          { status: 200 },
        );
      }
      if (url === '/api/health') {
        return new Response('{}', { status: 200 });
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    const { hookState, dispose } = mountHook();

    await waitFor(() => {
      expect(hookState.proxyAuthInfo()).toEqual({
        username: 'alice@example.com',
        logoutURL: '/api/oidc/test-oidc/logout',
      });
      expect(hookState.platformAdmission()).not.toBeNull();
    });

    expect(hookState.securityStatus()?.ssoSessionUsername).toBe(principal);
    expect(hookState.hasAuth()).toBe(true);
    expect(hookState.needsAuth()).toBe(false);
    expect(hookState.enhancedStore()?.initialDataReceived()).toBe(false);
    // The shell carries no estate before the socket connects; navigation comes
    // from the admission facet instead of a full-state payload.
    expect(hookState.state().resources).toEqual([]);
    expect(hookState.runtimeStateResolved()).toBe(false);
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/state/summary')).toHaveLength(1);
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/state')).toHaveLength(0);

    dispose();
  });

  it('bootstraps proxy-auth TrueNAS state before websocket data', async () => {
    const bootstrapResource = makeTrueNASResource('truenas-proxy');
    apiFetchMock.mockImplementation(async (url: string) => {
      if (url === '/api/state/summary') {
        return new Response(
          JSON.stringify({ activeAlerts: 0, nodes: 0, vms: 0, containers: 0, dockerHosts: [] }),
          { status: 200 },
        );
      }
      if (url === '/api/security/status') {
        return new Response(
          JSON.stringify({
            hasAuthentication: true,
            hasProxyAuth: true,
            proxyAuthUsername: 'proxy-operator',
            proxyAuthLogoutURL: '/proxy/logout',
          }),
          { status: 200 },
        );
      }
      if (url === '/api/state') {
        return new Response(
          JSON.stringify({
            resources: [bootstrapResource],
            lastUpdate: 1_700_000_000_000,
          }),
          { status: 200 },
        );
      }
      if (url === '/api/health') {
        return new Response('{}', { status: 200 });
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    const { hookState, dispose } = mountHook();

    await waitFor(() => {
      expect(hookState.platformAdmission()).not.toBeNull();
    });

    // No estate before the socket connects; navigation comes from the facet.
    expect(hookState.state().resources).toEqual([]);
    expect(hookState.runtimeStateResolved()).toBe(false);
    expect(hookState.proxyAuthInfo()).toEqual({
      username: 'proxy-operator',
      logoutURL: '/proxy/logout',
    });
    expect(hookState.needsAuth()).toBe(false);
    expect(hookState.enhancedStore()?.initialDataReceived()).toBe(false);
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/state/summary')).toHaveLength(1);
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/state')).toHaveLength(0);

    dispose();
  });

  it('does not start the proxy-auth runtime when protected state rejects the session', async () => {
    apiFetchMock.mockImplementation(async (url: string) => {
      if (url === '/api/state/summary') {
        return new Response('{}', { status: 401 });
      }
      if (url === '/api/security/status') {
        return new Response(
          JSON.stringify({
            hasAuthentication: true,
            hasProxyAuth: true,
            proxyAuthUsername: 'proxy-operator',
          }),
          { status: 200 },
        );
      }
      if (url === '/api/state') {
        return new Response('{}', { status: 401 });
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    const { hookState, dispose } = mountHook();

    await waitFor(() => {
      expect(hookState.isLoading()).toBe(false);
    });

    expect(hookState.needsAuth()).toBe(true);
    expect(hookState.enhancedStore()).toBeNull();
    expect(orgsListMock).not.toHaveBeenCalled();
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/state/summary')).toHaveLength(1);
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/state')).toHaveLength(0);

    dispose();
  });

  it('resolves navigation before websocket data without pulling the full state', async () => {
    const bootstrapResource: Resource = {
      id: 'pve-1',
      name: 'pve-1',
      displayName: 'pve-1',
      type: 'agent',
      platformId: 'pve-1',
      platformType: 'proxmox-pve',
      sourceType: 'api',
      sources: ['proxmox'],
      status: 'online',
      lastSeen: 1_700_000_000_000,
    };
    apiFetchMock.mockImplementation(async (url: string) => {
      if (url === '/api/state/summary') {
        return new Response(
          JSON.stringify({ activeAlerts: 0, nodes: 0, vms: 0, containers: 0, dockerHosts: [] }),
          { status: 200 },
        );
      }
      if (url === '/api/security/status') {
        return new Response(JSON.stringify({ hasAuthentication: true }), { status: 200 });
      }
      if (url === '/api/state') {
        return new Response(
          JSON.stringify({
            resources: [bootstrapResource],
            lastUpdate: 1_700_000_000_000,
          }),
          { status: 200 },
        );
      }
      if (url === '/api/health') {
        return new Response('{}', { status: 200 });
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    const { hookState, dispose } = mountHook();

    await waitFor(() => {
      expect(hookState.platformAdmission()).not.toBeNull();
    });
    // The shell used to render this window from a full-state payload. It now
    // resolves navigation from the admission facet and holds no estate of its
    // own until the socket delivers one.
    expect(hookState.state().resources).toEqual([]);
    expect(hookState.runtimeStateResolved()).toBe(false);
    expect(hookState.enhancedStore()?.initialDataReceived()).toBe(false);
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/state/summary')).toHaveLength(1);
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/state')).toHaveLength(0);

    dispose();
  });

  it('keeps retained server resource state available during a transient WebSocket reconnect', async () => {
    const retainedResource: Resource = {
      id: 'truenas-1',
      name: 'truenas-1',
      displayName: 'truenas-1',
      type: 'agent',
      platformId: 'truenas-1',
      platformType: 'truenas',
      sourceType: 'api',
      sources: ['truenas'],
      status: 'online',
      lastSeen: 1_700_000_000_000,
    };
    websocketState = makeWebSocketState({
      resources: [retainedResource],
      lastUpdate: 1_700_000_000_000,
    });
    websocketConnected = false;
    websocketReconnecting = true;
    websocketInitialDataReceived = false;
    apiFetchMock.mockImplementation(async (url: string) => {
      if (url === '/api/state/summary') {
        return new Response(
          JSON.stringify({ activeAlerts: 0, nodes: 0, vms: 0, containers: 0, dockerHosts: [] }),
          { status: 200 },
        );
      }
      if (url === '/api/security/status') {
        return new Response(
          JSON.stringify({
            hasAuthentication: true,
            ssoEnabled: true,
            ssoSessionUsername: 'sso:oidc:test:operator',
          }),
          { status: 200 },
        );
      }
      if (url === '/api/state') {
        return new Response('{}', { status: 200 });
      }
      if (url === '/api/health') {
        return new Response('{}', { status: 200 });
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    const { hookState, dispose } = mountHook();

    await waitFor(() => {
      expect(hookState.needsAuth()).toBe(false);
      expect(hookState.reconnecting()).toBe(true);
    });

    expect(hookState.enhancedStore()?.initialDataReceived()).toBe(false);
    expect(hookState.runtimeStateResolved()).toBe(true);
    expect(hookState.state().resources).toEqual([retainedResource]);
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/state/summary')).toHaveLength(1);
    expect(apiFetchMock.mock.calls.filter(([url]) => url === '/api/state')).toHaveLength(0);

    dispose();
  });

  it('does not mistake alert-only reconnect recovery for an empty resource snapshot', async () => {
    websocketState = makeWebSocketState({
      activeAlerts: [{ id: 'recovered-alert' } as State['activeAlerts'][number]],
    });
    websocketConnected = false;
    websocketReconnecting = true;
    websocketInitialDataReceived = false;
    const { hookState, dispose } = mountHook();
    await waitFor(() => expect(hookState.enhancedStore()).not.toBeNull());
    expect(hookState.state().activeAlerts).toHaveLength(1);
    expect(hookState.runtimeStateResolved()).toBe(false);
    dispose();
  });

  it('distinguishes an evidence-free first load from an authenticated empty estate', async () => {
    // The distinction still matters: an estate with nothing in it must resolve
    // to "no platform pages", not sit unresolved forever. It is now answered by
    // the admission facet rather than by a full-state payload arriving.
    let resolveAdmission: ((value: unknown) => void) | undefined;
    const pendingAdmission = new Promise((resolve) => {
      resolveAdmission = resolve;
    });
    apiFetchJSONMock.mockImplementation(async (url: string) => {
      if (url.startsWith('/api/resources')) {
        return pendingAdmission;
      }
      throw new Error(`Unhandled apiFetchJSON URL: ${url}`);
    });

    const { hookState, dispose } = mountHook();

    await waitFor(() => {
      expect(apiFetchMock.mock.calls.some(([url]) => url === '/api/state/summary')).toBe(true);
    });
    // Nothing has answered yet: no estate, and no admission to navigate by.
    expect(hookState.runtimeStateResolved()).toBe(false);
    expect(hookState.platformAdmission()).toBeNull();

    resolveAdmission?.({
      aggregations: {
        platformAdmission: {
          proxmox: false,
          docker: false,
          kubernetes: false,
          truenas: false,
          vmware: false,
          standalone: false,
        },
      },
    });

    await waitFor(() => {
      expect(hookState.platformAdmission()).not.toBeNull();
      expect(hookState.isLoading()).toBe(false);
    });
    // An authenticated empty estate: admitted nothing, which is a resolved
    // answer rather than an absent one.
    expect(Object.values(hookState.platformAdmission()!)).toEqual([
      false,
      false,
      false,
      false,
      false,
      false,
    ]);
    expect(hookState.state().resources).toEqual([]);

    dispose();
  });

  it('skips commercial posture bootstrap when upgrade prompts are hidden', async () => {
    apiFetchMock.mockImplementation(async (url: string) => {
      if (url === '/api/state/summary') {
        return new Response(
          JSON.stringify({ activeAlerts: 0, nodes: 0, vms: 0, containers: 0, dockerHosts: [] }),
          { status: 200 },
        );
      }
      if (url === '/api/security/status') {
        return new Response(
          JSON.stringify({
            hasAuthentication: true,
            presentationPolicy: {
              demoMode: false,
              readOnly: false,
              hideCommercial: false,
              hideUpgrade: true,
            },
          }),
          { status: 200 },
        );
      }
      if (url === '/api/state') {
        return new Response('{}', { status: 200 });
      }
      if (url === '/api/health') {
        return new Response('{}', { status: 200 });
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    const { dispose } = mountHook();

    await flushAsync();
    await flushAsync();

    expect(loadCommercialPostureMock).not.toHaveBeenCalled();

    dispose();
  });

  it('loads commercial posture during bootstrap when upgrade prompts are allowed', async () => {
    apiFetchMock.mockImplementation(async (url: string) => {
      if (url === '/api/state/summary') {
        return new Response(
          JSON.stringify({ activeAlerts: 0, nodes: 0, vms: 0, containers: 0, dockerHosts: [] }),
          { status: 200 },
        );
      }
      if (url === '/api/security/status') {
        return new Response(
          JSON.stringify({
            hasAuthentication: true,
            presentationPolicy: {
              demoMode: false,
              readOnly: false,
              hideCommercial: false,
              hideUpgrade: false,
            },
          }),
          { status: 200 },
        );
      }
      if (url === '/api/state') {
        return new Response('{}', { status: 200 });
      }
      if (url === '/api/health') {
        return new Response('{}', { status: 200 });
      }
      throw new Error(`Unhandled apiFetch URL: ${url}`);
    });

    const { dispose } = mountHook();

    await waitFor(() => {
      expect(loadCommercialPostureMock).toHaveBeenCalledOnce();
    });

    dispose();
  });

  it('reads platform admission from the canonical resource contract during bootstrap', async () => {
    const { dispose, hookState } = mountHook();

    await waitFor(() => {
      expect(hookState.platformAdmission()).not.toBeNull();
    });

    // A one-resource request answers it: navigation must not need an
    // estate-sized payload to know which platform pages exist.
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/resources?page=1&limit=1');
    expect(hookState.platformAdmission()).toMatchObject({ proxmox: true, standalone: false });

    dispose();
  });

  it('rejects a partial admission payload rather than hiding platforms', async () => {
    apiFetchJSONMock.mockResolvedValue({
      aggregations: { platformAdmission: { proxmox: true, docker: false } },
    });
    const { dispose, hookState } = mountHook();

    await waitFor(() => {
      expect(apiFetchJSONMock).toHaveBeenCalled();
    });
    await flushAsync();

    // A missing flag would read as a hidden platform, which is
    // indistinguishable from a real absence, so the facet is discarded.
    expect(hookState.platformAdmission()).toBeNull();

    dispose();
  });

  it('drops the previous tenant admission when the organization changes', async () => {
    isMultiTenantEnabledMock.mockReturnValue(true);
    const { dispose, hookState } = mountHook();

    await waitFor(() => {
      expect(hookState.platformAdmission()).not.toBeNull();
    });

    apiFetchJSONMock.mockImplementation(async () => ({
      aggregations: {
        platformAdmission: {
          proxmox: false,
          docker: true,
          kubernetes: false,
          truenas: false,
          vmware: false,
          standalone: false,
        },
      },
    }));

    await waitFor(() => {
      expect(hookState.organizations().some((org) => org.id === 'acme')).toBe(true);
    });

    // Switch to whichever tenant is not already active, so this exercises a
    // real switch rather than the no-op early return.
    const nextOrg = hookState.activeOrgID() === 'acme' ? 'default' : 'acme';
    hookState.handleOrgSwitch(nextOrg);

    // Cleared synchronously, before the replacement can arrive: anything that
    // renders between the switch and the refetch must not see the outgoing
    // tenant's platform tabs.
    expect(hookState.platformAdmission()).toBeNull();

    await waitFor(() => {
      expect(hookState.platformAdmission()).toMatchObject({ docker: true, proxmox: false });
    });

    dispose();
  });

  it('retains valid admission after a failed reconnect refresh and accepts later empty admission', async () => {
    const eventsModule = await import('@/stores/events');
    const { dispose, hookState } = mountHook();
    await waitFor(() => expect(hookState.platformAdmission()).not.toBeNull());
    const admitted = hookState.platformAdmission();
    const reconnect = (eventsModule.eventBus.on as ReturnType<typeof vi.fn>).mock.calls.find(
      ([event]) => event === 'websocket_reconnected',
    )![1] as () => void;
    apiFetchJSONMock.mockRejectedValue(new Error('503 admission unavailable'));
    reconnect();
    await flushAsync();
    expect(hookState.platformAdmission()).toEqual(admitted);

    const empty = {
      proxmox: false,
      docker: false,
      kubernetes: false,
      truenas: false,
      vmware: false,
      standalone: false,
    };
    apiFetchJSONMock.mockResolvedValue({ aggregations: { platformAdmission: empty } });
    reconnect();
    await waitFor(() => expect(hookState.platformAdmission()).toEqual(empty));
    dispose();
  });

  it('does not retain outgoing tenant admission if the new tenant request fails', async () => {
    isMultiTenantEnabledMock.mockReturnValue(true);
    const { dispose, hookState } = mountHook();
    await waitFor(() => {
      expect(hookState.platformAdmission()).not.toBeNull();
      expect(hookState.organizations().some((org) => org.id === 'acme')).toBe(true);
    });
    apiFetchJSONMock.mockRejectedValue(new Error('503 admission unavailable'));
    hookState.handleOrgSwitch(hookState.activeOrgID() === 'acme' ? 'default' : 'acme');
    expect(hookState.platformAdmission()).toBeNull();
    await flushAsync();
    expect(hookState.platformAdmission()).toBeNull();
    dispose();
  });

  it.each(['failed', 'successful', 'pending'] as const)(
    'ignores an outgoing tenant refresh after a %s tenant switch request',
    async (completion) => {
      isMultiTenantEnabledMock.mockReturnValue(true);
      const eventsModule = await import('@/stores/events');
      const { dispose, hookState } = mountHook();
      await waitFor(() => {
        expect(hookState.platformAdmission()).not.toBeNull();
        expect(hookState.organizations().some((org) => org.id === 'acme')).toBe(true);
      });
      const outgoing = hookState.platformAdmission();
      let finishOutgoing!: (value: unknown) => void;
      apiFetchJSONMock.mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finishOutgoing = resolve;
          }),
      );
      const reconnect = (eventsModule.eventBus.on as ReturnType<typeof vi.fn>).mock.calls.find(
        ([event]) => event === 'websocket_reconnected',
      )![1] as () => void;
      reconnect();
      const emptyAdmission = Object.fromEntries(Object.keys(outgoing!).map((key) => [key, false]));
      let finishIncoming!: (value: unknown) => void;
      if (completion === 'failed') {
        apiFetchJSONMock.mockRejectedValue(new Error('503 admission unavailable'));
      } else if (completion === 'successful') {
        apiFetchJSONMock.mockResolvedValue({ aggregations: { platformAdmission: emptyAdmission } });
      } else {
        apiFetchJSONMock.mockImplementation(
          () =>
            new Promise((resolve) => {
              finishIncoming = resolve;
            }),
        );
      }
      hookState.handleOrgSwitch(hookState.activeOrgID() === 'acme' ? 'default' : 'acme');
      await flushAsync();
      expect(hookState.platformAdmission()).toEqual(
        completion === 'successful' ? emptyAdmission : null,
      );
      finishOutgoing({ aggregations: { platformAdmission: outgoing } });
      await flushAsync();
      expect(hookState.platformAdmission()).toEqual(
        completion === 'successful' ? emptyAdmission : null,
      );
      if (completion === 'pending') {
        finishIncoming({ aggregations: { platformAdmission: emptyAdmission } });
        await flushAsync();
        expect(hookState.platformAdmission()).toEqual(emptyAdmission);
      }
      dispose();
    },
  );

  it('leaves admission unresolved when the first request fails', async () => {
    apiFetchJSONMock.mockRejectedValue(new Error('503 admission unavailable'));
    const { dispose, hookState } = mountHook();
    await waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalled());
    await flushAsync();
    expect(hookState.platformAdmission()).toBeNull();
    dispose();
  });

  it('refreshes admission when the websocket reconnects', async () => {
    const eventsModule = await import('@/stores/events');
    const { dispose } = mountHook();

    await waitFor(() => {
      expect(apiFetchJSONMock).toHaveBeenCalled();
    });

    const reconnectHandler = (eventsModule.eventBus.on as ReturnType<typeof vi.fn>).mock.calls.find(
      ([event]) => event === 'websocket_reconnected',
    )?.[1] as (() => void) | undefined;
    expect(reconnectHandler).toBeTypeOf('function');

    const before = apiFetchJSONMock.mock.calls.length;
    // The estate can gain or lose a platform while the socket is down.
    reconnectHandler!();
    await waitFor(() => {
      expect(apiFetchJSONMock.mock.calls.length).toBeGreaterThan(before);
    });

    dispose();
  });

  it('keeps retired chart cache prewarm out of the authenticated app shell', () => {
    expect(useAppRuntimeStateSource).not.toContain('fetchInfrastructureSummaryAndCache');
    expect(useAppRuntimeStateSource).not.toContain('fetchWorkloadsSummaryAndCache');
    expect(useAppRuntimeStateSource).not.toContain('requestIdleCallback');
    expect(useAppRuntimeStateSource).not.toContain('App prewarm');
  });

  it('skips the protected state bootstrap probe on the login route until a local auth hint exists', async () => {
    window.history.replaceState({}, '', '/login');
    hasStoredAuthSessionMock.mockReturnValue(false);

    const { hookState, dispose } = mountHook();

    await flushAsync();
    await flushAsync();

    expect(apiFetchMock.mock.calls.some(([url]) => url === '/api/state/summary')).toBe(false);
    expect(hookState.needsAuth()).toBe(true);

    dispose();
  });

  it('skips the protected state bootstrap probe on the root login entrypoint until a local auth hint exists', async () => {
    window.history.replaceState({}, '', '/');
    hasStoredAuthSessionMock.mockReturnValue(false);

    const { hookState, dispose } = mountHook();

    await flushAsync();
    await flushAsync();

    expect(apiFetchMock.mock.calls.some(([url]) => url === '/api/state/summary')).toBe(false);
    expect(hookState.needsAuth()).toBe(true);

    dispose();
  });

  it('keeps the protected state bootstrap probe on the login route once a local auth hint exists', async () => {
    window.history.replaceState({}, '', '/login');
    hasStoredAuthSessionMock.mockReturnValue(false);
    window.sessionStorage.setItem('pulse_auth_user', 'demo');

    const { dispose } = mountHook();

    await flushAsync();
    await flushAsync();

    expect(apiFetchMock.mock.calls.some(([url]) => url === '/api/state/summary')).toBe(true);

    dispose();
  });
});
