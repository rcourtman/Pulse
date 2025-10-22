import {
  Show,
  For,
  createSignal,
  createContext,
  useContext,
  createEffect,
  createMemo,
  onCleanup,
  onMount,
  getOwner,
  runWithOwner,
} from 'solid-js';
import type { JSX } from 'solid-js';
import { Router, Route, Navigate, useNavigate, useLocation } from '@solidjs/router';
import { getGlobalWebSocketStore } from './stores/websocket-global';
import { Dashboard } from './components/Dashboard/Dashboard';
import StorageComponent from './components/Storage/Storage';
import Backups from './components/Backups/Backups';
import Settings from './components/Settings/Settings';
import { Alerts } from './pages/Alerts';
import { DockerHosts } from './components/Docker/DockerHosts';
import { ToastContainer } from './components/Toast/Toast';
import { ErrorBoundary } from './components/ErrorBoundary';
import { SecurityWarning } from './components/SecurityWarning';
import { Login } from './components/Login';
import { logger } from './utils/logger';
import { POLLING_INTERVALS, STORAGE_KEYS } from './constants';
import { UpdatesAPI } from './api/updates';
import type { VersionInfo } from './api/updates';
import { apiFetch } from './utils/apiClient';
import { SettingsAPI } from './api/settings';
import { eventBus } from './stores/events';
import { updateStore } from './stores/updates';
import { UpdateBanner } from './components/UpdateBanner';
import { LegacySSHBanner } from './components/LegacySSHBanner';
import { DemoBanner } from './components/DemoBanner';
import { createTooltipSystem } from './components/shared/Tooltip';
import type { State } from '@/types/api';
import MailGateway from './components/PMG/MailGateway';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { DockerIcon } from '@/components/icons/DockerIcon';
import { AlertsIcon } from '@/components/icons/AlertsIcon';
import { SettingsGearIcon } from '@/components/icons/SettingsGearIcon';
import { TokenRevealDialog } from './components/TokenRevealDialog';
import { useAlertsActivation } from './stores/alertsActivation';

// Enhanced store type with proper typing
type EnhancedStore = ReturnType<typeof getGlobalWebSocketStore>;

// Export WebSocket context for other components
export const WebSocketContext = createContext<EnhancedStore>();
export const useWebSocket = () => {
  const context = useContext(WebSocketContext);
  if (!context) {
    throw new Error('useWebSocket must be used within WebSocketContext.Provider');
  }
  return context;
};

// Dark mode context for reactive theme switching
export const DarkModeContext = createContext<() => boolean>(() => false);
export const useDarkMode = () => {
  const context = useContext(DarkModeContext);
  if (!context) {
    throw new Error('useDarkMode must be used within DarkModeContext.Provider');
  }
  return context;
};

// Docker route component that properly uses activeAlerts from useWebSocket
function DockerRoute() {
  const wsContext = useContext(WebSocketContext);
  if (!wsContext) {
    return <div>Loading...</div>;
  }
  const { state, activeAlerts } = wsContext;
  const hosts = createMemo(() => state.dockerHosts ?? []);
  return <DockerHosts hosts={hosts()} activeAlerts={activeAlerts} />;
}

function App() {
  const TooltipRoot = createTooltipSystem();
  const owner = getOwner();
  const acquireWsStore = (): EnhancedStore => {
    const store = owner
      ? runWithOwner(owner, () => getGlobalWebSocketStore())
      : getGlobalWebSocketStore();
    return store || getGlobalWebSocketStore();
  };
  const alertsActivation = useAlertsActivation();

  const fallbackState: State = {
    nodes: [],
    vms: [],
    containers: [],
    dockerHosts: [],
    storage: [],
    cephClusters: [],
    physicalDisks: [],
    pbs: [],
  pmg: [],
  metrics: [],
  pveBackups: {
    backupTasks: [],
    storageBackups: [],
    guestSnapshots: [],
  },
  pbsBackups: [],
  pmgBackups: [],
  backups: {
    pve: {
      backupTasks: [],
      storageBackups: [],
      guestSnapshots: [],
    },
    pbs: [],
    pmg: [],
  },
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
    lastUpdate: '',
  };

  // Simple auth state
  const [isLoading, setIsLoading] = createSignal(true);
  const [needsAuth, setNeedsAuth] = createSignal(false);
  const [hasAuth, setHasAuth] = createSignal(false);
  const [proxyAuthInfo, setProxyAuthInfo] = createSignal<{
    username?: string;
    logoutURL?: string;
  } | null>(null);

  // Don't initialize WebSocket until after auth check
  const [wsStore, setWsStore] = createSignal<EnhancedStore | null>(null);
  const state = (): State => wsStore()?.state || fallbackState;
  const connected = () => wsStore()?.connected() || false;
  const reconnecting = () => wsStore()?.reconnecting() || false;

  // Data update indicator
  const [dataUpdated, setDataUpdated] = createSignal(false);
  let updateTimeout: number;

  // Last update time formatting
  const [lastUpdateText, setLastUpdateText] = createSignal('');

  const formatLastUpdate = (timestamp: string) => {
    if (!timestamp) return '';
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', {
      hour: 'numeric',
      minute: '2-digit',
      second: '2-digit',
      hour12: true
    });
  };

  // Flash indicator when data updates
  createEffect(() => {
    // Watch for state changes
    const updateTime = state().lastUpdate;
    if (updateTime && updateTime !== '') {
      setDataUpdated(true);
      setLastUpdateText(formatLastUpdate(updateTime));
      window.clearTimeout(updateTimeout);
      updateTimeout = window.setTimeout(() => setDataUpdated(false), POLLING_INTERVALS.DATA_FLASH);
    }
  });

  onMount(() => {
    void alertsActivation.refreshConfig();
    void alertsActivation.refreshActiveAlerts();
  });

  // No longer need tab state management - using router now

  // Version info
  const [versionInfo, setVersionInfo] = createSignal<VersionInfo | null>(null);

  // Dark mode - initialize immediately from localStorage to prevent flash
  // This addresses issue #443 where dark mode wasn't persisting
  // Priority: 1. localStorage (user's last choice on this device)
  //           2. System preference
  //           3. Server preference (loaded later for cross-device sync)
  const savedDarkMode = localStorage.getItem(STORAGE_KEYS.DARK_MODE);
  const hasLocalPreference = savedDarkMode !== null;
  const initialDarkMode = hasLocalPreference
    ? savedDarkMode === 'true'
    : window.matchMedia('(prefers-color-scheme: dark)').matches;
  const [darkMode, setDarkMode] = createSignal(initialDarkMode);
  const [, setHasLoadedServerTheme] = createSignal(false);

  // Apply dark mode immediately on initialization
  if (initialDarkMode) {
    document.documentElement.classList.add('dark');
  } else {
    document.documentElement.classList.remove('dark');
  }

  // Toggle dark mode
  const toggleDarkMode = async () => {
    const newMode = !darkMode();
    setDarkMode(newMode);
    localStorage.setItem(STORAGE_KEYS.DARK_MODE, String(newMode));
    if (newMode) {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
    logger.info('Theme changed', { mode: newMode ? 'dark' : 'light' });

    // Save theme preference to server if authenticated
    if (!needsAuth()) {
      try {
        await SettingsAPI.updateSystemSettings({ theme: newMode ? 'dark' : 'light' });
        logger.info('Theme preference saved to server');
      } catch (error) {
        logger.error('Failed to save theme preference to server', error);
        // Don't show error to user - local change still works
      }
    }
  };

  // Don't initialize dark mode here - will be handled based on auth state

  // Listen for theme changes from other browser instances
  onMount(() => {
    const handleThemeChange = (theme?: string) => {
      if (!theme) return;
      logger.info('Received theme change from another browser instance', { theme });
      const isDark = theme === 'dark';

      // Update local state
      setDarkMode(isDark);
      localStorage.setItem(STORAGE_KEYS.DARK_MODE, String(isDark));

      // Update DOM
      if (isDark) {
        document.documentElement.classList.add('dark');
      } else {
        document.documentElement.classList.remove('dark');
      }
    };

    // Subscribe to theme change events
    eventBus.on('theme_changed', handleThemeChange);

    // Cleanup on unmount
    onCleanup(() => {
      eventBus.off('theme_changed', handleThemeChange);
    });
  });

  // Check auth on mount
  onMount(async () => {
    console.log('[App] Starting auth check...');

    // Check if we just logged out - if so, always show login page
    const justLoggedOut = localStorage.getItem('just_logged_out');
    if (justLoggedOut) {
      localStorage.removeItem('just_logged_out');
      console.log('[App] Just logged out, showing login page');
      setHasAuth(true); // Force showing login instead of setup
      setNeedsAuth(true);
      setIsLoading(false);
      return;
    }

    // First check security status to see if auth is configured
    try {
      const securityRes = await apiFetch('/api/security/status');
      const securityData = await securityRes.json();
      console.log('[App] Security status:', securityData);

      // Check if auth is disabled via DISABLE_AUTH
      if (securityData.disabled === true) {
        console.log('[App] Auth is disabled via DISABLE_AUTH, skipping authentication');
        setHasAuth(false);
        setNeedsAuth(false);
        // Initialize WebSocket immediately since no auth needed
        setWsStore(acquireWsStore());

        // Load theme preference from server for cross-device sync
        // Only use server preference if no local preference exists
        if (!hasLocalPreference) {
          try {
            const systemSettings = await SettingsAPI.getSystemSettings();
            if (systemSettings.theme && systemSettings.theme !== '') {
              const prefersDark = systemSettings.theme === 'dark';
              setDarkMode(prefersDark);
              localStorage.setItem(STORAGE_KEYS.DARK_MODE, String(prefersDark));
              if (prefersDark) {
                document.documentElement.classList.add('dark');
              } else {
                document.documentElement.classList.remove('dark');
              }
            }
            setHasLoadedServerTheme(true);
          } catch (error) {
            console.error('Failed to load theme from server:', error);
          }
        }

        // Load version info even when auth is disabled
        UpdatesAPI.getVersion()
          .then((version) => {
            setVersionInfo(version);
            // Check for updates after loading version info (non-blocking)
            updateStore.checkForUpdates();
          })
          .catch((error) => console.error('Failed to load version:', error));

        setIsLoading(false);
        return;
      }

      const authConfigured = securityData.hasAuthentication || false;
      setHasAuth(authConfigured);

      // Check for proxy auth
      if (securityData.hasProxyAuth && securityData.proxyAuthUsername) {
        console.log('[App] Proxy auth detected, user:', securityData.proxyAuthUsername);
        setProxyAuthInfo({
          username: securityData.proxyAuthUsername,
          logoutURL: securityData.proxyAuthLogoutURL,
        });
        setNeedsAuth(false);
        // Initialize WebSocket for proxy auth users
        setWsStore(acquireWsStore());

        // Load theme preference from server for cross-device sync
        // Only use server preference if no local preference exists
        if (!hasLocalPreference) {
          try {
            const systemSettings = await SettingsAPI.getSystemSettings();
            if (systemSettings.theme && systemSettings.theme !== '') {
              const prefersDark = systemSettings.theme === 'dark';
              setDarkMode(prefersDark);
              localStorage.setItem(STORAGE_KEYS.DARK_MODE, String(prefersDark));
              if (prefersDark) {
                document.documentElement.classList.add('dark');
              } else {
                document.documentElement.classList.remove('dark');
              }
            }
            setHasLoadedServerTheme(true);
          } catch (error) {
            console.error('Failed to load theme from server:', error);
          }
        }

        // Load version info
        UpdatesAPI.getVersion()
          .then((version) => {
            setVersionInfo(version);
            // Check for updates after loading version info (non-blocking)
            updateStore.checkForUpdates();
          })
          .catch((error) => console.error('Failed to load version:', error));

        setIsLoading(false);
        return;
      }

      // Check for OIDC session
      if (securityData.oidcEnabled && securityData.oidcUsername) {
        console.log('[App] OIDC session detected, user:', securityData.oidcUsername);
        setHasAuth(true); // OIDC is enabled, so auth is configured
        setProxyAuthInfo({
          username: securityData.oidcUsername,
          logoutURL: securityData.oidcLogoutURL, // OIDC logout URL from IdP
        });
        setNeedsAuth(false);
        // Initialize WebSocket for OIDC users
        setWsStore(acquireWsStore());

        // Load theme preference from server for cross-device sync
        // Only use server preference if no local preference exists
        if (!hasLocalPreference) {
          try {
            const systemSettings = await SettingsAPI.getSystemSettings();
            if (systemSettings.theme && systemSettings.theme !== '') {
              const prefersDark = systemSettings.theme === 'dark';
              setDarkMode(prefersDark);
              localStorage.setItem(STORAGE_KEYS.DARK_MODE, String(prefersDark));
              if (prefersDark) {
                document.documentElement.classList.add('dark');
              } else {
                document.documentElement.classList.remove('dark');
              }
            }
            setHasLoadedServerTheme(true);
          } catch (error) {
            console.error('Failed to load theme from server:', error);
          }
        }

        // Load version info
        UpdatesAPI.getVersion()
          .then((version) => {
            setVersionInfo(version);
            // Check for updates after loading version info (non-blocking)
            updateStore.checkForUpdates();
          })
          .catch((error) => console.error('Failed to load version:', error));

        setIsLoading(false);
        return;
      }

      // If no auth is configured, show FirstRunSetup
      if (!authConfigured) {
        console.log('[App] No auth configured, showing Login/FirstRunSetup');
        setNeedsAuth(true); // This will show the Login component which shows FirstRunSetup
        setIsLoading(false);
        return;
      }

      // If auth is configured, check if we're authenticated
      const stateRes = await apiFetch('/api/state', {
        headers: {
          'X-Requested-With': 'XMLHttpRequest',
          Accept: 'application/json',
        },
      });

      if (stateRes.status === 401) {
        setNeedsAuth(true);
      } else {
        setNeedsAuth(false);
        // Only initialize WebSocket after successful auth check
        setWsStore(acquireWsStore());

        // Load theme preference from server for cross-device sync
        // Only use server preference if no local preference exists
        if (!hasLocalPreference) {
          try {
            const systemSettings = await SettingsAPI.getSystemSettings();
            if (systemSettings.theme && systemSettings.theme !== '') {
              const prefersDark = systemSettings.theme === 'dark';
              setDarkMode(prefersDark);
              localStorage.setItem(STORAGE_KEYS.DARK_MODE, String(prefersDark));
              if (prefersDark) {
                document.documentElement.classList.add('dark');
              } else {
                document.documentElement.classList.remove('dark');
              }
            }
            setHasLoadedServerTheme(true);
          } catch (error) {
            console.error('Failed to load theme from server:', error);
          }
        } else {
          // We have a local preference, just mark that we've checked the server
          setHasLoadedServerTheme(true);
        }
      }
    } catch (error) {
      console.error('Auth check error:', error);
      // On error, try to proceed without auth
      setNeedsAuth(false);
      setWsStore(acquireWsStore());

      // Theme is already applied on initialization, no need to reapply
    } finally {
      setIsLoading(false);
    }

    // Load version info
    UpdatesAPI.getVersion()
      .then((version) => {
        setVersionInfo(version);
        // Check for updates after loading version info (non-blocking)
        updateStore.checkForUpdates();
      })
      .catch((error) => console.error('Failed to load version:', error));
  });

  const handleLogin = () => {
    window.location.reload();
  };

  const handleLogout = async () => {
    // Check if we're using proxy auth with a logout URL
    const proxyAuth = proxyAuthInfo();
    if (proxyAuth?.logoutURL) {
      // Redirect to proxy auth logout URL
      window.location.href = proxyAuth.logoutURL;
      return;
    }

    try {
      // Import the apiClient to get CSRF token support
      const { apiFetch, clearAuth } = await import('./utils/apiClient');

      // Clear any session data - this will include CSRF token
      const response = await apiFetch('/api/logout', {
        method: 'POST',
      });

      if (!response.ok) {
        console.error('Logout failed:', response.status);
      }

      // Clear auth from apiClient
      clearAuth();
    } catch (error) {
      console.error('Logout error:', error);
    }

    // Clear all local storage EXCEPT theme preference and logout flag
    const currentTheme = localStorage.getItem(STORAGE_KEYS.DARK_MODE);
    localStorage.clear();
    sessionStorage.clear();
    localStorage.setItem('just_logged_out', 'true');
    // Preserve theme preference across logout
    if (currentTheme) {
      localStorage.setItem(STORAGE_KEYS.DARK_MODE, currentTheme);
    }

    // Clear WebSocket connection
    if (wsStore()) {
      setWsStore(null);
    }

    // Force reload to login page
    window.location.href = '/';
  };

  // Pass through the store directly (only when initialized)
  const enhancedStore = () => wsStore();

  // Root layout component for Router
  const RootLayout = (props: { children?: JSX.Element }) => {
    return (
      <Show
        when={!isLoading()}
        fallback={
          <div class="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
            <div class="text-gray-600 dark:text-gray-400">Loading...</div>
          </div>
        }
      >
        <Show when={!needsAuth()} fallback={<Login onLogin={handleLogin} />}>
          <ErrorBoundary>
            <Show when={enhancedStore()} fallback={<div>Initializing...</div>}>
              <WebSocketContext.Provider value={enhancedStore()!}>
                <DarkModeContext.Provider value={darkMode}>
                  <SecurityWarning />
                  <DemoBanner />
                  <UpdateBanner />
                  <LegacySSHBanner />
                  <div class="min-h-screen bg-gray-100 dark:bg-gray-900 text-gray-800 dark:text-gray-200 font-sans py-4 sm:py-6">
                    <AppLayout
                      connected={connected}
                      reconnecting={reconnecting}
                      dataUpdated={dataUpdated}
                      lastUpdateText={lastUpdateText}
                      versionInfo={versionInfo}
                      hasAuth={hasAuth}
                      needsAuth={needsAuth}
                      proxyAuthInfo={proxyAuthInfo}
                      handleLogout={handleLogout}
                      state={state}
                    >
                      {props.children}
                    </AppLayout>
                  </div>
                  <ToastContainer />
                  <TokenRevealDialog />
                  <TooltipRoot />
                </DarkModeContext.Provider>
              </WebSocketContext.Provider>
            </Show>
          </ErrorBoundary>
        </Show>
      </Show>
    );
  };

  // Use Router with routes
  return (
    <Router root={RootLayout}>
      <Route path="/" component={() => <Navigate href="/proxmox/overview" />} />
      <Route path="/proxmox" component={() => <Navigate href="/proxmox/overview" />} />
      <Route
        path="/proxmox/overview"
        component={() => <Dashboard vms={state().vms} containers={state().containers} nodes={state().nodes} />}
      />
      <Route path="/proxmox/storage" component={StorageComponent} />
      <Route path="/proxmox/mail" component={MailGateway} />
      <Route path="/proxmox/backups" component={Backups} />
      <Route path="/storage" component={() => <Navigate href="/proxmox/storage" />} />
      <Route path="/backups" component={() => <Navigate href="/proxmox/backups" />} />
      <Route path="/docker" component={DockerRoute} />
      <Route path="/alerts/*" component={Alerts} />
      <Route
        path="/settings/*"
        component={() => <Settings darkMode={darkMode} toggleDarkMode={toggleDarkMode} />}
      />
    </Router>
  );
}

function AppLayout(props: {
  connected: () => boolean;
  reconnecting: () => boolean;
  dataUpdated: () => boolean;
  lastUpdateText: () => string;
  versionInfo: () => VersionInfo | null;
  hasAuth: () => boolean;
  needsAuth: () => boolean;
  proxyAuthInfo: () => { username?: string; logoutURL?: string } | null;
  handleLogout: () => void;
  state: () => State;
  children?: JSX.Element;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const PLATFORM_SEEN_STORAGE_KEY = 'pulse-platforms-seen';

  const readSeenPlatforms = (): Record<string, boolean> => {
    if (typeof window === 'undefined') return {};
    try {
      const stored = window.localStorage.getItem(PLATFORM_SEEN_STORAGE_KEY);
      if (stored) {
        const parsed = JSON.parse(stored) as Record<string, boolean>;
        if (parsed && typeof parsed === 'object') {
          return parsed;
        }
      }
    } catch (error) {
      console.warn('Failed to parse stored platform visibility preferences', error);
    }
    return {};
  };

  const [seenPlatforms, setSeenPlatforms] = createSignal<Record<string, boolean>>(readSeenPlatforms());

  const persistSeenPlatforms = (map: Record<string, boolean>) => {
    if (typeof window === 'undefined') return;
    try {
      window.localStorage.setItem(PLATFORM_SEEN_STORAGE_KEY, JSON.stringify(map));
    } catch (error) {
      console.warn('Failed to persist platform visibility preferences', error);
    }
  };

  const markPlatformSeen = (platformId: string) => {
    setSeenPlatforms((current) => {
      if (current[platformId]) {
        return current;
      }
      const updated = { ...current, [platformId]: true };
      persistSeenPlatforms(updated);
      return updated;
    });
  };

  // Determine active tab from current path
  const getActiveTab = () => {
    const path = location.pathname;
    if (path.startsWith('/proxmox')) return 'proxmox';
    if (path.startsWith('/docker')) return 'docker';
    if (path.startsWith('/alerts')) return 'alerts';
    if (path.startsWith('/settings')) return 'settings';
    return 'proxmox';
  };
  const hasDockerHosts = createMemo(() => (props.state().dockerHosts?.length ?? 0) > 0);
  const hasProxmoxHosts = createMemo(
    () =>
      (props.state().nodes?.length ?? 0) > 0 ||
      (props.state().vms?.length ?? 0) > 0 ||
      (props.state().containers?.length ?? 0) > 0,
  );

  createEffect(() => {
    if (hasDockerHosts()) {
      markPlatformSeen('docker');
    }
  });

  createEffect(() => {
    if (hasProxmoxHosts()) {
      markPlatformSeen('proxmox');
    }
  });

  const platformTabs = createMemo(() => {
    return [
      {
        id: 'proxmox' as const,
        label: 'Proxmox',
        route: '/proxmox/overview',
        settingsRoute: '/settings',
        tooltip: 'Monitor Proxmox clusters and nodes',
        enabled: hasProxmoxHosts() || !!seenPlatforms()['proxmox'],
        live: hasProxmoxHosts(),
        icon: (
          <ProxmoxIcon class="w-4 h-4 shrink-0" />
        ),
      },
      {
        id: 'docker' as const,
        label: 'Docker',
        route: '/docker',
        settingsRoute: '/settings/docker',
        tooltip: 'Monitor Docker hosts and containers',
        enabled: hasDockerHosts() || !!seenPlatforms()['docker'],
        live: hasDockerHosts(),
        icon: (
          <DockerIcon class="w-4 h-4 shrink-0" />
        ),
      },
    ];
  });

  const utilityTabs = createMemo(() => {
    const allAlerts = props.state().activeAlerts || [];
    const breakdown = allAlerts.reduce(
      (acc, alert: any) => {
        if (alert?.acknowledged) return acc;
        const level = String(alert?.level || '').toLowerCase();
        if (level === 'critical') {
          acc.critical += 1;
        } else {
          acc.warning += 1;
        }
        return acc;
      },
      { warning: 0, critical: 0 },
    );
    const activeAlertCount = breakdown.warning + breakdown.critical;
    return [
      {
        id: 'alerts' as const,
        label: 'Alerts',
        route: '/alerts',
        tooltip: 'Review active alerts and automation rules',
        badge: null as 'update' | null,
        count: activeAlertCount,
        breakdown,
        icon: <AlertsIcon class="w-4 h-4 shrink-0" />,
      },
      {
        id: 'settings' as const,
        label: 'Settings',
        route: '/settings',
        tooltip: 'Configure Pulse preferences and integrations',
        badge: updateStore.isUpdateVisible() ? ('update' as const) : null,
        count: undefined,
        breakdown: undefined,
        icon: <SettingsGearIcon class="w-4 h-4 shrink-0" />,
      },
    ];
  });

  const handlePlatformClick = (platform: ReturnType<typeof platformTabs>[number]) => {
    if (platform.enabled) {
      navigate(platform.route);
    } else {
      navigate(platform.settingsRoute);
    }
  };

  const handleUtilityClick = (tab: ReturnType<typeof utilityTabs>[number]) => {
    navigate(tab.route);
  };

  return (
    <div class="pulse-shell">
      {/* Header */}
      <div class="header flex flex-row justify-between items-center mb-2">
        <div class="hidden md:block md:flex-1"></div>
        <div class="flex items-center gap-1 flex-none">
          <svg
            width="20"
            height="20"
            viewBox="0 0 256 256"
            xmlns="http://www.w3.org/2000/svg"
            class={`pulse-logo ${props.connected() && props.dataUpdated() ? 'animate-pulse-logo' : ''}`}
          >
            <title>Pulse Logo</title>
            <circle
              class="pulse-bg fill-blue-600 dark:fill-blue-500"
              cx="128"
              cy="128"
              r="122"
            />
            <circle
              class="pulse-ring fill-none stroke-white stroke-[14] opacity-[0.92]"
              cx="128"
              cy="128"
              r="84"
            />
            <circle
              class="pulse-center fill-white dark:fill-[#dbeafe]"
              cx="128"
              cy="128"
              r="26"
            />
          </svg>
          <span class="text-lg font-medium text-gray-800 dark:text-gray-200">Pulse</span>
          <Show when={props.versionInfo()?.channel === 'rc'}>
            <span class="text-xs px-1.5 py-0.5 bg-orange-500 text-white rounded font-bold">
              RC
            </span>
          </Show>
        </div>
        <div class="header-controls flex justify-end items-center gap-4 md:flex-1">
          <div class="flex items-center gap-2">
            <div
              class={`group status text-xs rounded-full flex items-center justify-center transition-all duration-500 ease-in-out px-1.5 ${
                props.connected()
                  ? 'connected bg-green-200 dark:bg-green-700 text-green-700 dark:text-green-300 min-w-6 h-6 group-hover:px-3'
                  : props.reconnecting()
                    ? 'reconnecting bg-yellow-200 dark:bg-yellow-700 text-yellow-700 dark:text-yellow-300 py-1'
                    : 'disconnected bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 min-w-6 h-6 group-hover:px-3'
              }`}
            >
              <Show when={props.reconnecting()}>
                <svg class="animate-spin h-3 w-3 flex-shrink-0" fill="none" viewBox="0 0 24 24">
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
              </Show>
              <Show when={props.connected()}>
                <span class="h-2.5 w-2.5 rounded-full bg-green-600 dark:bg-green-400 flex-shrink-0"></span>
              </Show>
              <Show when={!props.connected() && !props.reconnecting()}>
                <span class="h-2.5 w-2.5 rounded-full bg-gray-600 dark:bg-gray-400 flex-shrink-0"></span>
              </Show>
              <span class={`whitespace-nowrap overflow-hidden transition-all duration-500 ${props.connected() || (!props.connected() && !props.reconnecting()) ? 'max-w-0 group-hover:max-w-[100px] group-hover:ml-2 group-hover:mr-1 opacity-0 group-hover:opacity-100' : 'max-w-[100px] ml-1 opacity-100'}`}>
                {props.connected()
                  ? 'Connected'
                  : props.reconnecting()
                    ? 'Reconnecting...'
                    : 'Disconnected'}
              </span>
            </div>
            <Show when={props.hasAuth() && !props.needsAuth()}>
              <Show when={props.proxyAuthInfo()?.username}>
                <span class="text-xs px-2 py-1 text-gray-600 dark:text-gray-400">
                  {props.proxyAuthInfo()?.username}
                </span>
              </Show>
              <button
                type="button"
                onClick={props.handleLogout}
                class="group relative flex h-7 items-center justify-center gap-1 rounded-full bg-gray-200 px-2 text-xs text-gray-700 transition-all duration-500 ease-in-out hover:bg-gray-300 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
                title="Logout"
                aria-label="Logout"
              >
                <svg
                  class="h-3 w-3 flex-shrink-0"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                  aria-hidden="true"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                  />
                </svg>
                <span
                  class="max-w-0 overflow-hidden whitespace-nowrap opacity-0 transition-all duration-500 ease-in-out group-hover:ml-1 group-hover:max-w-[80px] group-hover:opacity-100 group-focus-visible:ml-1 group-focus-visible:max-w-[80px] group-focus-visible:opacity-100"
                >
                  Logout
                </span>
              </button>
            </Show>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div
        class="tabs flex items-end gap-2 mb-2 border-b border-gray-300 dark:border-gray-700 overflow-x-auto overflow-y-hidden whitespace-nowrap scrollbar-hide"
        role="tablist"
        aria-label="Primary navigation"
      >
        <div class="flex items-end gap-1" role="group" aria-label="Infrastructure">
          <For each={platformTabs()}>
            {(platform) => {
              const isActive = () => getActiveTab() === platform.id;
              const disabled = () => !platform.enabled;
              const baseClasses =
                'tab relative px-2 sm:px-3 py-1.5 text-xs sm:text-sm font-medium flex items-center gap-1 sm:gap-1.5 rounded-t border border-transparent transition-colors whitespace-nowrap cursor-pointer';

              const className = () => {
                if (isActive()) {
                  return `${baseClasses} bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 border-gray-300 dark:border-gray-700 border-b border-b-white dark:border-b-gray-800 shadow-sm font-semibold`;
                }
                if (disabled()) {
                  return `${baseClasses} cursor-not-allowed text-gray-400 dark:text-gray-600 opacity-70 bg-gray-100/40 dark:bg-gray-800/40`;
                }
                return `${baseClasses} text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-200/60 dark:hover:bg-gray-700/60`;
              };

              const title = () =>
                disabled()
                  ? `${platform.label} is not configured yet. Click to open settings.`
                  : platform.tooltip;

              return (
                <div
                  class={className()}
                  role="tab"
                  aria-disabled={disabled()}
                  onClick={() => handlePlatformClick(platform)}
                  title={title()}
                >
                  {platform.icon}
                  <span>{platform.label}</span>
                  <Show when={!platform.live}>
                    <button
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation();
                        navigate(platform.settingsRoute);
                      }}
                      class="ml-1 text-[10px] uppercase tracking-wide text-gray-400 dark:text-gray-600 hover:text-blue-500 focus-visible:outline-none focus-visible:ring-0"
                    >
                      Add host
                    </button>
                  </Show>
                </div>
              );
            }}
          </For>
        </div>
        <div class="flex items-end gap-2 ml-auto" role="group" aria-label="System">
          <div class="flex items-end gap-1 pl-3 sm:pl-4">
            <For each={utilityTabs()}>
              {(tab) => {
                const isActive = () => getActiveTab() === tab.id;
                const baseClasses =
                  'tab relative px-2 sm:px-3 py-1.5 text-xs sm:text-sm font-medium flex items-center gap-1 sm:gap-1.5 rounded-t border border-transparent transition-colors whitespace-nowrap cursor-pointer';

              const className = () => {
                if (isActive()) {
                  return `${baseClasses} bg-white dark:bg-gray-800 text-blue-600 dark:text-blue-400 border-gray-300 dark:border-gray-700 border-b border-b-white dark:border-b-gray-800 shadow-sm font-semibold`;
                }
                return `${baseClasses} text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-200/60 dark:hover:bg-gray-700/60`;
              };

                return (
                  <div
                    class={className()}
                    role="tab"
                    aria-disabled={false}
                    onClick={() => handleUtilityClick(tab)}
                    title={tab.tooltip}
                  >
                    {tab.icon}
                    <span class="flex items-center gap-1.5">
                      <span>{tab.label}</span>
                      {tab.id === 'alerts' && (() => {
                        const total = tab.count ?? 0;
                        if (total <= 0) {
                          return null;
                        }
                        return (
                          <span class="inline-flex items-center gap-1">
                            {tab.breakdown?.critical > 0 && (
                              <span class="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-bold text-white bg-red-600 dark:bg-red-500 rounded-full">
                                {tab.breakdown.critical}
                              </span>
                            )}
                            {tab.breakdown?.warning > 0 && (
                              <span class="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-semibold text-amber-900 dark:text-amber-100 bg-amber-200 dark:bg-amber-500/80 rounded-full">
                                {tab.breakdown.warning}
                              </span>
                            )}
                          </span>
                        );
                      })()}
                    </span>
                    <Show when={tab.badge === 'update'}>
                      <span class="ml-1 w-2 h-2 bg-red-500 rounded-full animate-pulse"></span>
                    </Show>
                  </div>
                );
              }}
            </For>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <main
        id="main"
        class="tab-content block bg-white dark:bg-gray-800 rounded-b rounded-tr shadow mb-2"
      >
        <div class="pulse-panel">
          {props.children}
        </div>
      </main>

      {/* Footer */}
      <footer class="text-center text-xs text-gray-500 dark:text-gray-400 py-4">
        Pulse | Version:{' '}
        <a
          href="https://github.com/rcourtman/Pulse/releases"
          target="_blank"
          rel="noopener noreferrer"
          class="text-blue-600 dark:text-blue-400 hover:underline"
        >
          {props.versionInfo()?.version || 'loading...'}
        </a>
        {props.versionInfo()?.isDevelopment && ' (Development)'}
        {props.versionInfo()?.isDocker && ' - Docker'}
        <Show when={props.lastUpdateText()}>
          <span class="mx-2">|</span>
          <span>Last refresh: {props.lastUpdateText()}</span>
        </Show>
      </footer>
    </div>
  );
}

export default App; // Test hot-reload comment $(date)
