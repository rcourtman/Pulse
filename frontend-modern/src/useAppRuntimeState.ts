import {
  createEffect,
  createMemo,
  createSignal,
  getOwner,
  onCleanup,
  onMount,
  runWithOwner,
} from 'solid-js';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';
import { logger } from '@/utils/logger';
import { POLLING_INTERVALS } from '@/constants';
import { STORAGE_KEYS } from '@/utils/localStorage';
import type { TimeRange } from '@/api/charts';
import type { VersionInfo } from '@/api/updates';
import type { Organization } from '@/api/orgs';
import { OrgsAPI } from '@/api/orgs';
import type { SecurityStatus } from '@/types/config';
import type { State } from '@/types/api';
import { SettingsAPI } from '@/api/settings';
import {
  apiFetch,
  getOrgID as getSelectedOrgID,
  hasAuth as hasStoredAuthSession,
  setOrgID as setSelectedOrgID,
} from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { showToast } from '@/utils/toast';
import { updateStore } from '@/stores/updates';
import {
  fetchInfrastructureSummaryAndCache,
  hasFreshInfrastructureSummaryCache,
} from '@/utils/infrastructureSummaryCache';
import { buildInfrastructurePath } from '@/routing/resourceLinks';
import { useAlertsActivation } from '@/stores/alertsActivation';
import {
  applyThemeClass,
  computeIsDark,
  getStoredThemePreference,
  hasStoredThemePreference,
  normalizeThemePreference,
  persistThemePreference,
  type ThemePreference,
} from '@/utils/theme';
import {
  initKioskMode,
  getPulseWebSocketUrl,
} from '@/utils/url';
import { syncKioskMode } from '@/hooks/useKioskMode';
import {
  isHostedModeEnabled,
  isMultiTenantEnabled,
  runtimeCapabilitiesLoaded,
  loadRuntimeCapabilities,
} from '@/stores/license';
import { aiChatStore } from '@/stores/aiChat';
import { loadCommercialPosture } from '@/stores/licenseCommercial';
import {
  presentationPolicyHidesOrganizationSurfaces,
  syncSessionPresentationPolicy,
} from '@/stores/sessionPresentationPolicy';
import { layoutStore } from '@/utils/layout';
import {
  markSystemSettingsLoadedWithDefaults,
  updateSystemSettingsFromResponse,
} from '@/stores/systemSettings';

type EnhancedStore = ReturnType<typeof getGlobalWebSocketStore>;

export type AppConnectionStatus = {
  kind:
    | 'connected'
    | 'sync-reconnecting'
    | 'backend-healthy'
    | 'reconnecting'
    | 'disconnected';
  label: string;
  detail: string;
  tone: 'healthy' | 'warning' | 'offline';
};

const ROOT_INFRASTRUCTURE_PATH = buildInfrastructurePath();
const ROOT_DASHBOARD_PATH = '/dashboard';

export const useAppRuntimeState = () => {
  initKioskMode();
  syncKioskMode();

  const owner = getOwner();
  const acquireWsStore = (): EnhancedStore => {
    const store = owner
      ? runWithOwner(owner, () => getGlobalWebSocketStore())
      : getGlobalWebSocketStore();
    return store || getGlobalWebSocketStore();
  };

  const alertsActivation = useAlertsActivation();
  let hasPreloadedRoutes = false;
  let hasFetchedVersionInfo = false;
  let hasPrewarmedInfrastructureCharts = false;

  const getInfrastructureTrendRangeForPrewarm = (): TimeRange => '1h';

  const hasLocalAuthBootstrapHint = (): boolean => {
    if (hasStoredAuthSession()) {
      return true;
    }

    if (typeof window === 'undefined') {
      return false;
    }

    try {
      return Boolean(window.sessionStorage.getItem('pulse_auth_user'));
    } catch {
      return false;
    }
  };

  const shouldPrewarmInfrastructure = (): boolean => {
    if (typeof window === 'undefined') return false;

    const connection = (
      navigator as unknown as { connection?: { saveData?: boolean; effectiveType?: string } }
    ).connection;
    if (connection?.saveData) return false;
    const effectiveType = connection?.effectiveType;
    if (
      typeof effectiveType === 'string' &&
      (effectiveType === 'slow-2g' || effectiveType === '2g')
    ) {
      return false;
    }

    const pathname = window.location.pathname;
    if (!pathname) return true;
    if (pathname === ROOT_INFRASTRUCTURE_PATH) return false;
    if (pathname === ROOT_DASHBOARD_PATH) return false;
    return true;
  };

  const prewarmInfrastructureCharts = () => {
    if (hasPrewarmedInfrastructureCharts || !shouldPrewarmInfrastructure()) {
      return;
    }

    const range = getInfrastructureTrendRangeForPrewarm();
    if (hasFreshInfrastructureSummaryCache(range)) {
      hasPrewarmedInfrastructureCharts = true;
      return;
    }

    hasPrewarmedInfrastructureCharts = true;
    void fetchInfrastructureSummaryAndCache(range, { caller: 'App prewarm' }).catch(() => {
      // Non-blocking prewarm; ignore failures.
    });
  };

  const preloadLazyRoutes = () => {
    if (hasPreloadedRoutes || typeof window === 'undefined') {
      return;
    }
    hasPreloadedRoutes = true;
    const loaders: Array<() => Promise<unknown>> = [
      () => import('@/pages/Alerts'),
      () => import('@/components/Settings/Settings'),
    ];

    loaders.forEach((load) => {
      void load().catch((error) => {
        logger.warn('Preloading route module failed', error);
      });
    });
  };

  const fallbackState: State = {
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
  };

  const [isLoading, setIsLoading] = createSignal(true);
  const [needsAuth, setNeedsAuth] = createSignal(false);
  const [hasAuth, setHasAuth] = createSignal(false);
  const [organizations, setOrganizations] = createSignal<Organization[]>([
    { id: 'default', displayName: 'Default Organization' },
  ]);
  const [activeOrgID, setActiveOrgID] = createSignal(getSelectedOrgID() || 'default');
  const [orgsLoading, setOrgsLoading] = createSignal(false);
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
  const [proxyAuthInfo, setProxyAuthInfo] = createSignal<{
    username?: string;
    logoutURL?: string;
  } | null>(null);
  const [wsStore, setWsStore] = createSignal<EnhancedStore | null>(null);
  const [backendHealthy, setBackendHealthy] = createSignal(false);
  const state = (): State => wsStore()?.state || fallbackState;
  const connected = () => wsStore()?.connected() || false;
  const reconnecting = () => wsStore()?.reconnecting() || false;
  const [dataUpdated, setDataUpdated] = createSignal(false);
  const [lastUpdateText, setLastUpdateText] = createSignal('');
  const [versionInfo, setVersionInfo] = createSignal<VersionInfo | null>(null);
  const connectionStatus = createMemo<AppConnectionStatus>(() => {
    if (connected()) {
      return {
        kind: 'connected',
        label: 'Connected',
        detail: 'Backend and live data stream are connected.',
        tone: 'healthy',
      };
    }

    if (backendHealthy() && reconnecting()) {
      return {
        kind: 'sync-reconnecting',
        label: 'Sync reconnecting',
        detail: 'Backend is healthy. Live updates are reconnecting.',
        tone: 'warning',
      };
    }

    if (backendHealthy()) {
      return {
        kind: 'backend-healthy',
        label: 'Backend healthy',
        detail: 'Backend is healthy, but the live data stream is not connected.',
        tone: 'warning',
      };
    }

    if (reconnecting()) {
      return {
        kind: 'reconnecting',
        label: 'Reconnecting...',
        detail: 'Attempting to reconnect to the backend and live data stream.',
        tone: 'offline',
      };
    }

    return {
      kind: 'disconnected',
      label: 'Disconnected',
      detail: 'Backend and live data stream are unavailable.',
      tone: 'offline',
    };
  });
  const showOrgSwitcher = createMemo(() => {
    if (!isMultiTenantEnabled()) {
      return false;
    }
    return !presentationPolicyHidesOrganizationSurfaces();
  });

  const initialThemePreference = getStoredThemePreference();
  const [themePreference, setThemePreference] =
    createSignal<ThemePreference>(initialThemePreference);
  const [, setHasLoadedServerTheme] = createSignal(false);
  const [darkMode, setDarkMode] = createSignal(computeIsDark(initialThemePreference));
  applyThemeClass(darkMode());

  const applyThemePreferenceLocally = (preference: ThemePreference) => {
    setThemePreference(preference);
    persistThemePreference(preference);
    const isDark = computeIsDark(preference);
    setDarkMode(isDark);
    applyThemeClass(isDark);
  };

  const applyServerThemeIfAllowed = (theme?: string) => {
    if (!hasStoredThemePreference() && theme && theme !== '') {
      applyThemePreferenceLocally(normalizeThemePreference(theme));
    }
  };

  const loadSystemSettingsAndLayout = async () => {
    try {
      const systemSettings = await SettingsAPI.getSystemSettings();
      updateSystemSettingsFromResponse(systemSettings);
      applyServerThemeIfAllowed(systemSettings.theme);
      setHasLoadedServerTheme(true);
      layoutStore.loadFromServer();
    } catch (error) {
      logger.error('Failed to load system settings from server', error);
      markSystemSettingsLoadedWithDefaults();
    }
  };

  const applySecurityStatus = (securityData: SecurityStatus | null) => {
    if (securityData) {
      syncSessionPresentationPolicy(securityData);
    }
    setSecurityStatus(securityData);
    aiChatStore.setEnabled(securityData?.sessionCapabilities?.assistantEnabled === true);
  };

  const beginAuthenticatedRuntime = async () => {
    setNeedsAuth(false);
    await loadOrganizations();
    setWsStore(acquireWsStore());
    setBackendHealthy(true);
    await loadSystemSettingsAndLayout();
    // Shared commercial posture bootstraps once per authenticated app shell.
    void loadCommercialPosture();
  };

  const checkBackendHealth = async () => {
    try {
      const response = await apiFetch('/api/health', { cache: 'no-store' });
      setBackendHealthy(response.ok);
      return response.ok;
    } catch {
      setBackendHealthy(false);
      return false;
    }
  };

  const loadOrganizations = async () => {
    setOrgsLoading(true);
    try {
      if (!runtimeCapabilitiesLoaded()) {
        await loadRuntimeCapabilities();
      }

      if (presentationPolicyHidesOrganizationSurfaces()) {
        const storedOrgID = getSelectedOrgID();
        const hiddenOrgID =
          isHostedModeEnabled() && storedOrgID && storedOrgID !== 'default'
            ? storedOrgID
            : 'default';
        setOrganizations([
          {
            id: hiddenOrgID,
            displayName: hiddenOrgID === 'default' ? 'Default Organization' : hiddenOrgID,
          },
        ]);
        setSelectedOrgID(hiddenOrgID);
        setActiveOrgID(hiddenOrgID);
        return;
      }

      if (!isMultiTenantEnabled()) {
        const storedOrgID = getSelectedOrgID();
        const hostedOrgID =
          isHostedModeEnabled() && storedOrgID && storedOrgID !== 'default'
            ? storedOrgID
            : 'default';
        setOrganizations([
          {
            id: hostedOrgID,
            displayName: hostedOrgID === 'default' ? 'Default Organization' : hostedOrgID,
          },
        ]);
        setSelectedOrgID(hostedOrgID);
        setActiveOrgID(hostedOrgID);
        return;
      }

      const fetched = await OrgsAPI.list();
      const organizationList =
        fetched.length > 0 ? fetched : [{ id: 'default', displayName: 'Default Organization' }];
      setOrganizations(organizationList);

      const storedOrgID = getSelectedOrgID();
      const selectedOrg =
        (storedOrgID && organizationList.some((org) => org.id === storedOrgID)
          ? storedOrgID
          : null) ||
        organizationList[0]?.id ||
        'default';

      setSelectedOrgID(selectedOrg);
      setActiveOrgID(selectedOrg);
    } catch (error) {
      logger.warn('Failed to load organizations, falling back to default org', error);
      showToast('error', 'Failed to load organizations. Using default.');
      const fallback = [{ id: 'default', displayName: 'Default Organization' }];
      setOrganizations(fallback);
      setSelectedOrgID('default');
      setActiveOrgID('default');
    } finally {
      setOrgsLoading(false);
    }
  };

  const handleOrgSwitch = (nextOrgID: string) => {
    const target = nextOrgID?.trim() || 'default';
    if (target === activeOrgID()) {
      return;
    }

    if (target !== 'default' && !organizations().some((org) => org.id === target)) {
      showToast('error', 'Organization no longer exists');
      return;
    }

    setSelectedOrgID(target);
    setActiveOrgID(target);
    eventBus.emit('org_switched', target);

    try {
      localStorage.removeItem(STORAGE_KEYS.GUEST_METADATA);
      localStorage.removeItem(STORAGE_KEYS.DOCKER_METADATA);
      localStorage.removeItem(STORAGE_KEYS.DOCKER_METADATA + '_hosts');
    } catch {
      // Ignore storage errors.
    }

    try {
      const store = wsStore();
      if (store && typeof store.switchUrl === 'function') {
        store.switchUrl(getPulseWebSocketUrl());
      } else {
        store?.reconnect();
      }
      showToast('success', 'Organization switched');
    } catch (error) {
      logger.error('Failed to switch organization', error);
      showToast('error', 'Failed to switch organization');
    }
  };

  const formatLastUpdate = (timestamp: number) => {
    if (!timestamp) return '';
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', {
      hour: 'numeric',
      minute: '2-digit',
      second: '2-digit',
      hour12: true,
    });
  };

  let updateTimeout: number | undefined;
  createEffect(() => {
    const updateTime = state().lastUpdate;
    if (updateTime > 0) {
      setDataUpdated(true);
      setLastUpdateText(formatLastUpdate(updateTime));
      if (updateTimeout !== undefined) {
        window.clearTimeout(updateTimeout);
      }
      updateTimeout = window.setTimeout(() => setDataUpdated(false), POLLING_INTERVALS.DATA_FLASH);
    }
  });

  onCleanup(() => {
    if (updateTimeout !== undefined) {
      window.clearTimeout(updateTimeout);
    }
  });

  createEffect(() => {
    if (!isLoading() && !needsAuth()) {
      if (typeof window === 'undefined') {
        return;
      }
      if (!hasPreloadedRoutes) {
        window.setTimeout(preloadLazyRoutes, 0);
      }
    }
  });

  createEffect(() => {
    if (isLoading() || needsAuth() || hasPrewarmedInfrastructureCharts) {
      return;
    }
    if (typeof window === 'undefined') {
      return;
    }

    if (typeof window.requestIdleCallback === 'function') {
      const id = window.requestIdleCallback(() => {
        prewarmInfrastructureCharts();
      }, { timeout: 2_000 });
      onCleanup(() => {
        window.cancelIdleCallback(id);
      });
      return;
    }

    const timeout = window.setTimeout(() => {
      prewarmInfrastructureCharts();
    }, 500);
    onCleanup(() => {
      window.clearTimeout(timeout);
    });
  });

  const syncVersionInfoFromUpdateStore = async () => {
    const cachedVersion = updateStore.versionInfo();
    if (cachedVersion) {
      setVersionInfo(cachedVersion);
    }

    await updateStore.checkForUpdates();

    const resolvedVersion = updateStore.versionInfo();
    if (resolvedVersion) {
      setVersionInfo(resolvedVersion);
    }
  };

  createEffect(() => {
    if (isLoading() || needsAuth() || hasFetchedVersionInfo) {
      return;
    }
    hasFetchedVersionInfo = true;
    void syncVersionInfoFromUpdateStore();
  });

  let alertsInitialized = false;
  createEffect(() => {
    const ready = !isLoading() && !needsAuth();
    if (ready && !alertsInitialized) {
      alertsInitialized = true;
      void alertsActivation.refreshConfig();
      void alertsActivation.refreshActiveAlerts();
    }
    if (!ready) {
      alertsInitialized = false;
    }
  });

  createEffect(() => {
    if (!hasAuth()) {
      setBackendHealthy(false);
      return;
    }

    if (connected()) {
      setBackendHealthy(true);
      return;
    }

    void checkBackendHealth();
    const interval = window.setInterval(() => {
      void checkBackendHealth();
    }, reconnecting() ? 5000 : 15000);

    onCleanup(() => {
      window.clearInterval(interval);
    });
  });

  const handleThemeChange = async (newPreference: ThemePreference) => {
    applyThemePreferenceLocally(newPreference);
    logger.info('Theme changed', { pref: newPreference, active: computeIsDark(newPreference) ? 'dark' : 'light' });

    if (!needsAuth()) {
      try {
        await SettingsAPI.updateSystemSettings({ theme: newPreference });
        logger.info('Theme preference saved to server');
      } catch (error) {
        logger.error('Failed to save theme preference to server', error);
      }
    }
  };

  onMount(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const systemThemeListener = (event: MediaQueryListEvent) => {
      if (themePreference() === 'system') {
        const isDark = event.matches;
        setDarkMode(isDark);
        applyThemeClass(isDark);
      }
    };
    mediaQuery.addEventListener('change', systemThemeListener);

    const handleRemoteThemeChange = (theme?: string) => {
      if (!theme) return;
      logger.info('Received theme change from another browser instance', { theme });
      applyThemePreferenceLocally(normalizeThemePreference(theme));
    };

    const handleWebSocketReconnected = () => {
      logger.info('WebSocket reconnected, refreshing alert configuration');
      void alertsActivation.refreshConfig();
      void alertsActivation.refreshActiveAlerts();
    };

    eventBus.on('theme_changed', handleRemoteThemeChange);
    eventBus.on('websocket_reconnected', handleWebSocketReconnected);

    onCleanup(() => {
      mediaQuery.removeEventListener('change', systemThemeListener);
      eventBus.off('theme_changed', handleRemoteThemeChange);
      eventBus.off('websocket_reconnected', handleWebSocketReconnected);
    });
  });

  onMount(async () => {
    logger.debug('[App] Starting auth check...');
    const justLoggedOut = localStorage.getItem('just_logged_out');

    try {
      const securityResponse = await apiFetch('/api/security/status');

      if (securityResponse.status === 401) {
        logger.warn(
          '[App] Security status request returned 401. Clearing stored credentials and showing login.',
        );
        try {
          const { clearAuth } = await import('@/utils/apiClient');
          clearAuth();
        } catch (clearError) {
          logger.warn('[App] Failed to clear stored auth after 401', clearError);
        }
        aiChatStore.setEnabled(false);
        setHasAuth(false);
        setNeedsAuth(true);
        setIsLoading(false);
        return;
      }

      if (justLoggedOut) {
        localStorage.removeItem('just_logged_out');
        logger.debug('[App] User logged out, showing login page');
        if (securityResponse.ok) {
          const securityData = (await securityResponse.json()) as SecurityStatus;
          applySecurityStatus(securityData);
        }
        setHasAuth(true);
        setNeedsAuth(true);
        setIsLoading(false);
        return;
      }

      if (!securityResponse.ok) {
        throw new Error(`Security status request failed with status ${securityResponse.status}`);
      }

      const securityData = (await securityResponse.json()) as SecurityStatus & {
        hasAuthentication?: boolean;
        hasProxyAuth?: boolean;
        proxyAuthUsername?: string;
        proxyAuthLogoutURL?: string;
        ssoEnabled?: boolean;
        ssoSessionUsername?: string;
        ssoLogoutURL?: string;
      };
      logger.debug('[App] Security status fetched', securityData);
      applySecurityStatus(securityData);

      const authConfigured = securityData.hasAuthentication || false;
      setHasAuth(authConfigured);

      if (securityData.hasProxyAuth && securityData.proxyAuthUsername) {
        logger.info('[App] Proxy auth detected', { user: securityData.proxyAuthUsername });
        setProxyAuthInfo({
          username: securityData.proxyAuthUsername,
          logoutURL: securityData.proxyAuthLogoutURL,
        });
        await beginAuthenticatedRuntime();
        void syncVersionInfoFromUpdateStore();
        setIsLoading(false);
        return;
      }

      if (securityData.ssoEnabled && securityData.ssoSessionUsername) {
        logger.info('[App] SSO session detected', { user: securityData.ssoSessionUsername });
        setHasAuth(true);
        setProxyAuthInfo({
          username: securityData.ssoSessionUsername,
          logoutURL: securityData.ssoLogoutURL,
        });
        await beginAuthenticatedRuntime();
        void syncVersionInfoFromUpdateStore();
        setIsLoading(false);
        return;
      }

      if (!authConfigured) {
        logger.info('[App] No auth configured, showing Login/FirstRunSetup');
        setNeedsAuth(true);
        setIsLoading(false);
        return;
      }

      if (window.location.pathname === '/login' && !hasLocalAuthBootstrapHint()) {
        logger.debug('[App] Login route bootstrap has no local auth hint yet');
        setNeedsAuth(true);
        return;
      }

      const stateResponse = await apiFetch('/api/state', {
        headers: {
          'X-Requested-With': 'XMLHttpRequest',
          Accept: 'application/json',
        },
      });

      if (stateResponse.status === 401) {
        setNeedsAuth(true);
      } else {
        await beginAuthenticatedRuntime();
      }
    } catch (error) {
      logger.error('Auth check error', error);
      try {
        const { clearAuth } = await import('@/utils/apiClient');
        clearAuth();
      } catch (clearError) {
        logger.warn('[App] Failed to clear stored auth after auth check error', clearError);
      }
      aiChatStore.setEnabled(false);
      setHasAuth(false);
      setNeedsAuth(true);
    } finally {
      setIsLoading(false);
    }
  });

  const handleLogin = () => {
    window.location.reload();
  };

  const handleLogout = async () => {
    const proxyAuth = proxyAuthInfo();
    if (proxyAuth?.logoutURL) {
      window.location.href = proxyAuth.logoutURL;
      return;
    }

    try {
      const { apiFetch: runtimeApiFetch, clearAuth } = await import('@/utils/apiClient');
      const response = await runtimeApiFetch('/api/logout', {
        method: 'POST',
      });

      if (!response.ok) {
        logger.error('Logout failed', { status: response.status });
      }
      clearAuth();
    } catch (error) {
      logger.error('Logout error', error);
    }

    const keysToRemove = [
      STORAGE_KEYS.AUTH,
      STORAGE_KEYS.GUEST_METADATA,
      STORAGE_KEYS.DOCKER_METADATA,
      STORAGE_KEYS.DOCKER_METADATA + '_hosts',
    ];
    keysToRemove.forEach((key) => localStorage.removeItem(key));
    sessionStorage.clear();
    localStorage.setItem('just_logged_out', 'true');
    aiChatStore.setEnabled(false);

    if (wsStore()) {
      setWsStore(null);
    }

    window.location.href = '/';
  };

  const enhancedStore = () => wsStore();

  return {
    isLoading,
    needsAuth,
    hasAuth,
    organizations,
    activeOrgID,
    orgsLoading,
    securityStatus,
    proxyAuthInfo,
    state,
    connected,
    backendHealthy,
    connectionStatus,
    reconnecting,
    dataUpdated,
    lastUpdateText,
    versionInfo,
    showOrgSwitcher,
    themePreference,
    darkMode,
    handleThemeChange,
    handleOrgSwitch,
    handleLogin,
    handleLogout,
    enhancedStore,
  };
};
