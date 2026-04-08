import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type UseAppRuntimeStateModule = typeof import('@/useAppRuntimeState');

const flushAsync = async () => {
  for (let i = 0; i < 8; i += 1) {
    await Promise.resolve();
  }
};

describe('useAppRuntimeState', () => {
  let useAppRuntimeState: UseAppRuntimeStateModule['useAppRuntimeState'];
  let apiFetchMock: ReturnType<typeof vi.fn>;
  let orgsListMock: ReturnType<typeof vi.fn>;
  let loadLicenseStatusMock: ReturnType<typeof vi.fn>;
  let isMultiTenantEnabledMock: ReturnType<typeof vi.fn>;
  let isHostedModeEnabledMock: ReturnType<typeof vi.fn>;
  let getOrgIDMock: ReturnType<typeof vi.fn>;
  let setOrgIDMock: ReturnType<typeof vi.fn>;
  let showToastMock: ReturnType<typeof vi.fn>;
  let aiChatSetEnabledMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vi.resetModules();

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

    Object.defineProperty(window, 'requestIdleCallback', {
      writable: true,
      configurable: true,
      value: vi.fn((cb: IdleRequestCallback) =>
        window.setTimeout(() => cb({ didTimeout: false, timeRemaining: () => 50 } as IdleDeadline), 0),
      ),
    });

    Object.defineProperty(window, 'cancelIdleCallback', {
      writable: true,
      configurable: true,
      value: vi.fn((id: number) => window.clearTimeout(id)),
    });

    apiFetchMock = vi.fn(async (url: string) => {
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
    orgsListMock = vi.fn().mockResolvedValue([{ id: 'acme', displayName: 'Acme' }]);
    loadLicenseStatusMock = vi.fn().mockResolvedValue(undefined);
    isMultiTenantEnabledMock = vi.fn().mockReturnValue(false);
    isHostedModeEnabledMock = vi.fn().mockReturnValue(false);
    getOrgIDMock = vi.fn().mockReturnValue('default');
    setOrgIDMock = vi.fn();
    showToastMock = vi.fn();
    aiChatSetEnabledMock = vi.fn();

    vi.doMock('@/stores/websocket-global', () => ({
      getGlobalWebSocketStore: () => ({
        state: {
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
          resources: [],
        },
        connected: () => false,
        reconnecting: () => false,
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
        getSystemSettings: vi.fn().mockResolvedValue({ theme: '' }),
        updateSystemSettings: vi.fn(),
      },
    }));

    vi.doMock('@/utils/apiClient', () => ({
      apiFetch: apiFetchMock,
      getOrgID: getOrgIDMock,
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

    vi.doMock('@/utils/infrastructureSummaryCache', () => ({
      fetchInfrastructureSummaryAndCache: vi.fn().mockResolvedValue(undefined),
      hasFreshInfrastructureSummaryCache: vi.fn().mockReturnValue(false),
    }));

    vi.doMock('@/routing/resourceLinks', () => ({
      buildInfrastructurePath: () => '/infrastructure',
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

    vi.doMock('@/utils/layout', () => ({
      layoutStore: {
        loadFromServer: vi.fn(),
      },
    }));

    vi.doMock('@/stores/systemSettings', () => ({
      markSystemSettingsLoadedWithDefaults: vi.fn(),
      updateSystemSettingsFromResponse: vi.fn(),
    }));

    ({ useAppRuntimeState } = await import('@/useAppRuntimeState'));
  });

  afterEach(() => {
    vi.clearAllMocks();
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

  it('stays on the default organization path when multi-tenant is not enabled', async () => {
    isMultiTenantEnabledMock.mockReturnValue(false);
    const { hookState, dispose } = mountHook();

    await flushAsync();
    await flushAsync();

    expect(orgsListMock).not.toHaveBeenCalled();
    expect(setOrgIDMock).toHaveBeenCalledWith('default');
    expect(hookState.organizations()).toEqual([
      { id: 'default', displayName: 'Default Organization' },
    ]);
    expect(hookState.activeOrgID()).toBe('default');
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

    await flushAsync();
    await flushAsync();

    expect(orgsListMock).toHaveBeenCalledOnce();
    expect(setOrgIDMock).toHaveBeenCalledWith('acme');
    expect(hookState.organizations()).toEqual([{ id: 'acme', displayName: 'Acme' }]);
    expect(hookState.activeOrgID()).toBe('acme');

    dispose();
  });

  it('syncs demo mode from security status session capabilities during bootstrap', async () => {
    apiFetchMock.mockImplementation(async (url: string) => {
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

    const demoModeModule = await import('@/stores/demoMode');
    const { dispose } = mountHook();

    await flushAsync();
    await flushAsync();

    expect(demoModeModule.demoModeResolved()).toBe(true);
    expect(demoModeModule.demoModeEnabled()).toBe(true);
    expect(aiChatSetEnabledMock).toHaveBeenCalledWith(true);

    dispose();
  });
});
