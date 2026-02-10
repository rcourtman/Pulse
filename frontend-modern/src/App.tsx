import {
  Show,
  For,
  Suspense,
  lazy,
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
import { ToastContainer } from './components/Toast/Toast';
import { ErrorBoundary } from './components/ErrorBoundary';
import { SecurityWarning } from './components/SecurityWarning';
import { Login } from './components/Login';
import { logger } from './utils/logger';
import { POLLING_INTERVALS } from './constants';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { layoutStore } from '@/utils/layout';
import { MONITORING_READ_SCOPE } from '@/constants/apiScopes';
import { UpdatesAPI } from './api/updates';
import type { VersionInfo } from './api/updates';
import { apiFetch } from './utils/apiClient';
import type { SecurityStatus } from '@/types/config';
import { SettingsAPI } from './api/settings';
import { eventBus } from './stores/events';
import { updateStore } from './stores/updates';
import { UpdateBanner } from './components/UpdateBanner';
import { DemoBanner } from './components/DemoBanner';
import { GitHubStarBanner } from './components/GitHubStarBanner';
import { createTooltipSystem } from './components/shared/Tooltip';
import type { State, Alert } from '@/types/api';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { startMetricsSampler } from './stores/metricsSampler';
import { seedFromBackend } from './stores/metricsHistory';
import { getMetricsViewMode } from './stores/metricsViewMode';
import BoxesIcon from 'lucide-solid/icons/boxes';
import MonitorIcon from 'lucide-solid/icons/monitor';
import BellIcon from 'lucide-solid/icons/bell';
import SettingsIcon from 'lucide-solid/icons/settings';
import NetworkIcon from 'lucide-solid/icons/network';
import Maximize2Icon from 'lucide-solid/icons/maximize-2';
import Minimize2Icon from 'lucide-solid/icons/minimize-2';
import { PulsePatrolLogo } from '@/components/Brand/PulsePatrolLogo';
import { TokenRevealDialog } from './components/TokenRevealDialog';
import { useAlertsActivation } from './stores/alertsActivation';
import { UpdateProgressModal } from './components/UpdateProgressModal';
import type { UpdateStatus } from './api/updates';
import { AIChat } from './components/AI/Chat';
import { aiChatStore } from './stores/aiChat';
import { useResourcesAsLegacy } from './hooks/useResources';
import { updateSystemSettingsFromResponse, markSystemSettingsLoadedWithDefaults } from './stores/systemSettings';
import { initKioskMode, isKioskMode, setKioskMode, subscribeToKioskMode, getKioskModePreference } from './utils/url';


const Dashboard = lazy(() =>
  import('./components/Dashboard/Dashboard').then((module) => ({ default: module.Dashboard })),
);
const StorageComponent = lazy(() => import('./components/Storage/Storage'));
const Backups = lazy(() => import('./components/Backups/Backups'));
const Replication = lazy(() => import('./components/Replication/Replication'));
const MailGateway = lazy(() => import('./components/PMG/MailGateway'));
const CephPage = lazy(() => import('./pages/Ceph'));
const AlertsPage = lazy(() =>
  import('./pages/Alerts').then((module) => ({ default: module.Alerts })),
);
const SettingsPage = lazy(() => import('./components/Settings/Settings'));
const DockerHosts = lazy(() =>
  import('./components/Docker/DockerHosts').then((module) => ({ default: module.DockerHosts })),
);
const KubernetesClusters = lazy(() =>
  import('./components/Kubernetes/KubernetesClusters').then((module) => ({
    default: module.KubernetesClusters,
  })),
);
const HostsOverview = lazy(() =>
  import('./components/Hosts/HostsOverview').then((module) => ({
    default: module.HostsOverview,
  })),
);
const AIIntelligencePage = lazy(() =>
  import('./pages/AIIntelligence').then((module) => ({ default: module.AIIntelligence })),
);


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

// Docker route component - uses unified resources via useResourcesAsLegacy hook
function DockerRoute() {
  const wsContext = useContext(WebSocketContext);
  if (!wsContext) {
    return <div>Loading...</div>;
  }
  const { activeAlerts } = wsContext;
  const { asDockerHosts } = useResourcesAsLegacy();

  return <DockerHosts hosts={asDockerHosts() as any} activeAlerts={activeAlerts} />;
}

// Hosts route component - HostsOverview uses useResourcesAsLegacy directly for proper reactivity
function HostsRoute() {
  return <HostsOverview />;
}

function KubernetesRoute() {
  const wsContext = useContext(WebSocketContext);
  if (!wsContext) {
    return <div>Loading...</div>;
  }
  return <KubernetesClusters clusters={wsContext.state.kubernetesClusters ?? []} />;
}

// Helper to detect if an update is actively in progress (not just checking for updates)
function isUpdateInProgress(status: string | undefined): boolean {
  if (!status) return false;
  const inProgressStates = ['downloading', 'verifying', 'extracting', 'installing', 'restarting'];
  return inProgressStates.includes(status);
}

// Global update progress watcher - shows modal in ALL tabs when an update is running
function GlobalUpdateProgressWatcher() {
  const wsContext = useContext(WebSocketContext);
  const navigate = useNavigate();
  const [showProgressModal, setShowProgressModal] = createSignal(false);
  const [hasAutoOpened, setHasAutoOpened] = createSignal(false);
  let pollInterval: number | undefined;

  // Fallback polling in case WebSocket events are missed
  const pollUpdateStatus = async () => {
    try {
      const status = await UpdatesAPI.getUpdateStatus();
      const inProgress = isUpdateInProgress(status.status);

      if (inProgress && !showProgressModal() && !hasAutoOpened()) {
        logger.info('Update in progress detected via polling fallback, showing progress modal', {
          status: status.status,
          message: status.message,
        });
        setShowProgressModal(true);
        setHasAutoOpened(true);
      } else if (!inProgress && hasAutoOpened()) {
        setHasAutoOpened(false);
      }
    } catch (_error) {
      // Silently ignore polling errors
    }
  };

  // Watch for update progress events from WebSocket (primary mechanism)
  createEffect(() => {
    const progress = wsContext?.updateProgress?.() as UpdateStatus | null;

    if (!progress) {
      // Reset when no progress data
      setHasAutoOpened(false);
      return;
    }

    const inProgress = isUpdateInProgress(progress.status);

    if (inProgress && !showProgressModal() && !hasAutoOpened()) {
      // Update is starting - auto-open the modal in this tab
      logger.info('Update in progress detected via WebSocket, showing progress modal', {
        status: progress.status,
        message: progress.message,
      });
      setShowProgressModal(true);
      setHasAutoOpened(true);
    } else if (!inProgress && hasAutoOpened()) {
      // Update finished - allow the modal to be dismissed
      setHasAutoOpened(false);
    }
  });

  // Start fallback polling on mount, stop on cleanup
  onMount(() => {
    // Poll every 5 seconds as a safety net
    pollInterval = setInterval(pollUpdateStatus, 5000) as unknown as number;
  });

  onCleanup(() => {
    if (pollInterval) {
      clearInterval(pollInterval);
    }
  });

  return (
    <UpdateProgressModal
      isOpen={showProgressModal()}
      onClose={() => setShowProgressModal(false)}
      onViewHistory={() => {
        setShowProgressModal(false);
        navigate('/settings/updates');
      }}
      connected={wsContext?.connected}
      reconnecting={wsContext?.reconnecting}
    />
  );
}

function App() {
  // Initialize kiosk mode from URL params immediately (persists to sessionStorage)
  // This must happen before any renders so kiosk state is available everywhere
  initKioskMode();

  // Reactive kiosk state for App-level components (banners, etc.)
  const [kioskMode, setKioskModeSignal] = createSignal(isKioskMode());
  onMount(() => {
    const unsubscribe = subscribeToKioskMode((enabled) => {
      setKioskModeSignal(enabled);
    });
    onCleanup(unsubscribe);
  });

  const TooltipRoot = createTooltipSystem();
  const owner = getOwner();
  const acquireWsStore = (): EnhancedStore => {
    const store = owner
      ? runWithOwner(owner, () => getGlobalWebSocketStore())
      : getGlobalWebSocketStore();
    return store || getGlobalWebSocketStore();
  };
  const alertsActivation = useAlertsActivation();

  // Start metrics sampler for sparklines
  onMount(() => {
    startMetricsSampler();

    // If user already has sparklines mode enabled, seed historical data immediately
    if (getMetricsViewMode() === 'sparklines') {
      seedFromBackend('1h').catch(() => {
        // Errors are already logged in seedFromBackend
      });
    }
  });

  let hasPreloadedRoutes = false;
  let hasFetchedVersionInfo = false;
  const preloadLazyRoutes = () => {
    if (hasPreloadedRoutes || typeof window === 'undefined') {
      return;
    }
    hasPreloadedRoutes = true;
    const loaders: Array<() => Promise<unknown>> = [
      () => import('./components/Storage/Storage'),
      () => import('./components/Backups/Backups'),
      () => import('./components/Replication/Replication'),
      () => import('./components/PMG/MailGateway'),
      () => import('./components/Hosts/HostsOverview'),

      () => import('./pages/Alerts'),
      () => import('./components/Settings/Settings'),
      () => import('./components/Docker/DockerHosts'),
    ];

    loaders.forEach((load) => {
      void load().catch((error) => {
        logger.warn('Preloading route module failed', error);
      });
    });
  };

  const fallbackState: State = {
    nodes: [],
    vms: [],
    containers: [],
    dockerHosts: [],
    removedDockerHosts: [],
    hosts: [],
    storage: [],
    cephClusters: [],
    physicalDisks: [],
    pbs: [],
    pmg: [],
    replicationJobs: [],
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
  // Store full security status for Login component (hideLocalLogin, oidcEnabled, etc.)
  // Store full security status for Login component (hideLocalLogin, oidcEnabled, etc.)
  const [securityStatus, setSecurityStatus] = createSignal<SecurityStatus | null>(null);
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

  createEffect(() => {
    if (!isLoading() && !needsAuth()) {
      if (typeof window === 'undefined') {
        return;
      }
      if (!hasPreloadedRoutes) {
        // Defer to the next tick so we don't contend with initial render
        window.setTimeout(preloadLazyRoutes, 0);
      }
    }
  });

  createEffect(() => {
    if (isLoading() || needsAuth() || hasFetchedVersionInfo) {
      return;
    }
    hasFetchedVersionInfo = true;

    UpdatesAPI.getVersion()
      .then((version) => {
        setVersionInfo(version);
        // Check for updates after loading version info (non-blocking)
        updateStore.checkForUpdates();
      })
      .catch((error) => {
        logger.error('Failed to load version', error);
      });
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

    // Handle WebSocket reconnection - refresh alert config to restore activation state
    // This fixes issue where alert toggle appears disabled after connection loss
    const handleWebSocketReconnected = () => {
      logger.info('WebSocket reconnected, refreshing alert configuration');
      void alertsActivation.refreshConfig();
      void alertsActivation.refreshActiveAlerts();
    };

    // Subscribe to events
    eventBus.on('theme_changed', handleThemeChange);
    eventBus.on('websocket_reconnected', handleWebSocketReconnected);

    // Cleanup on unmount
    onCleanup(() => {
      eventBus.off('theme_changed', handleThemeChange);
      eventBus.off('websocket_reconnected', handleWebSocketReconnected);
    });
  });


  // Check auth on mount
  onMount(async () => {
    logger.debug('[App] Starting auth check...');

    // Check if we just logged out - if so, always show login page
    const justLoggedOut = localStorage.getItem('just_logged_out');

    // First check security status to see if auth is configured
    // We need this for ALL paths to properly set hideLocalLogin, oidcEnabled, etc.
    try {
      const securityRes = await apiFetch('/api/security/status');

      if (securityRes.status === 401) {
        logger.warn(
          '[App] Security status request returned 401. Clearing stored credentials and showing login.',
        );
        try {
          const { clearAuth } = await import('./utils/apiClient');
          clearAuth();
        } catch (clearError) {
          logger.warn('[App] Failed to clear stored auth after 401', clearError);
        }
        // Still try to parse security data from 401 response for OIDC settings
        // If not available, Login component will fetch it on mount
        setHasAuth(false);
        setNeedsAuth(true);
        setIsLoading(false);
        return;
      }

      // Handle just_logged_out AFTER we have security status
      if (justLoggedOut) {
        localStorage.removeItem('just_logged_out');
        logger.debug('[App] User logged out, showing login page');
        // Parse security data to get hideLocalLogin, oidcEnabled, etc.
        if (securityRes.ok) {
          const securityData = await securityRes.json();
          setSecurityStatus(securityData as SecurityStatus);
        }
        setHasAuth(true); // Force showing login instead of setup
        setNeedsAuth(true);
        setIsLoading(false);
        return;
      }

      if (!securityRes.ok) {
        throw new Error(`Security status request failed with status ${securityRes.status}`);
      }

      const securityData = await securityRes.json();
      logger.debug('[App] Security status fetched', securityData);

      // Store full security status for Login component
      setSecurityStatus(securityData as SecurityStatus);

      // Detect legacy DISABLE_AUTH flag (now ignored) so we can surface a warning
      if (securityData.deprecatedDisableAuth === true) {
        logger.warn(
          '[App] Legacy DISABLE_AUTH flag detected; authentication remains enabled. Remove the flag and restart Pulse to silence this warning.',
        );
      }

      const authConfigured = securityData.hasAuthentication || false;
      setHasAuth(authConfigured);

      // Check for proxy auth
      if (securityData.hasProxyAuth && securityData.proxyAuthUsername) {
        logger.info('[App] Proxy auth detected', { user: securityData.proxyAuthUsername });
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
            // Update system settings store (for Docker update actions, etc.)
            updateSystemSettingsFromResponse(systemSettings);
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
            // Also load full-width mode from server
            layoutStore.loadFromServer();
          } catch (error) {
            logger.error('Failed to load theme from server', error);
            // Ensure settings are marked as loaded so UI doesn't stay in loading state
            markSystemSettingsLoadedWithDefaults();
          }
        } else {
          setHasLoadedServerTheme(true);
          // Still load system settings for other features (Docker update actions, etc.)
          SettingsAPI.getSystemSettings()
            .then((settings) => updateSystemSettingsFromResponse(settings))
            .catch((error) => {
              logger.warn('Failed to load system settings', error);
              markSystemSettingsLoadedWithDefaults();
            });
        }

        // Load version info
        UpdatesAPI.getVersion()
          .then((version) => {
            setVersionInfo(version);
            // Check for updates after loading version info (non-blocking)
            updateStore.checkForUpdates();
          })
          .catch((error) => logger.error('Failed to load version', error));

        setIsLoading(false);
        return;
      }

      // Check for OIDC session
      if (securityData.oidcEnabled && securityData.oidcUsername) {
        logger.info('[App] OIDC session detected', { user: securityData.oidcUsername });
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
            // Update system settings store (for Docker update actions, etc.)
            updateSystemSettingsFromResponse(systemSettings);
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
            // Also load full-width mode from server
            layoutStore.loadFromServer();
          } catch (error) {
            logger.error('Failed to load theme from server', error);
            // Ensure settings are marked as loaded so UI doesn't stay in loading state
            markSystemSettingsLoadedWithDefaults();
          }
        } else {
          setHasLoadedServerTheme(true);
          // Still load system settings for other features (Docker update actions, etc.)
          SettingsAPI.getSystemSettings()
            .then((settings) => updateSystemSettingsFromResponse(settings))
            .catch((error) => {
              logger.warn('Failed to load system settings', error);
              markSystemSettingsLoadedWithDefaults();
            });
        }

        // Load version info
        UpdatesAPI.getVersion()
          .then((version) => {
            setVersionInfo(version);
            // Check for updates after loading version info (non-blocking)
            updateStore.checkForUpdates();
          })
          .catch((error) => logger.error('Failed to load version', error));

        setIsLoading(false);
        return;
      }

      // If no auth is configured, show FirstRunSetup
      if (!authConfigured) {
        logger.info('[App] No auth configured, showing Login/FirstRunSetup');
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
            // Update system settings store (for Docker update actions, etc.)
            updateSystemSettingsFromResponse(systemSettings);
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
            // Also load full-width mode from server
            layoutStore.loadFromServer();
          } catch (error) {
            logger.error('Failed to load theme from server', error);
            // Ensure settings are marked as loaded so UI doesn't stay in loading state
            markSystemSettingsLoadedWithDefaults();
          }
        } else {
          // We have a local preference, just mark that we've checked the server
          setHasLoadedServerTheme(true);
          // Still load system settings for other features (Docker update actions, etc.)
          SettingsAPI.getSystemSettings()
            .then((settings) => updateSystemSettingsFromResponse(settings))
            .catch((error) => {
              logger.warn('Failed to load system settings', error);
              // Ensure settings are marked as loaded so UI doesn't stay in loading state
              markSystemSettingsLoadedWithDefaults();
            });
        }
      }
    } catch (error) {
      logger.error('Auth check error', error);
      try {
        const { clearAuth } = await import('./utils/apiClient');
        clearAuth();
      } catch (clearError) {
        logger.warn('[App] Failed to clear stored auth after auth check error', clearError);
      }
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
        logger.error('Logout failed', { status: response.status });
      }

      // Clear auth from apiClient
      clearAuth();
    } catch (error) {
      logger.error('Logout error', error);
    }

    // Clear only auth and session-specific storage, preserve user preferences
    // Keys to clear on logout (auth and per-session caches)
    const keysToRemove = [
      STORAGE_KEYS.AUTH,
      STORAGE_KEYS.LEGACY_TOKEN,
      STORAGE_KEYS.GUEST_METADATA,
      STORAGE_KEYS.DOCKER_METADATA,
      STORAGE_KEYS.DOCKER_METADATA + '_hosts',
    ];
    keysToRemove.forEach((key) => localStorage.removeItem(key));
    sessionStorage.clear();
    localStorage.setItem('just_logged_out', 'true');

    // Clear WebSocket connection
    if (wsStore()) {
      setWsStore(null);
    }

    // Force reload to login page
    window.location.href = '/';
  };

  // Pass through the store directly (only when initialized)
  const enhancedStore = () => wsStore();

  // Dashboard view - uses unified resources via useResourcesAsLegacy hook
  const DashboardView = () => {
    const { asVMs, asContainers, asNodes } = useResourcesAsLegacy();

    return (
      <Dashboard vms={asVMs() as any} containers={asContainers() as any} nodes={asNodes() as any} />
    );
  };

  const SettingsRoute = () => (
    <SettingsPage darkMode={darkMode} toggleDarkMode={toggleDarkMode} />
  );

  // Root layout component for Router
  const RootLayout = (props: { children?: JSX.Element }) => {
    // Check AI settings on mount and setup keyboard shortcut
    onMount(() => {
      // Only check AI settings if already authenticated (not on login screen)
      // Otherwise, the 401 response triggers a redirect loop
      if (!needsAuth()) {
        import('./api/ai').then(({ AIAPI }) => {
          AIAPI.getSettings()
            .then((settings) => {
              aiChatStore.setEnabled(settings.enabled && settings.configured);
              // Initialize chat session sync with server
              if (settings.enabled && settings.configured) {
                aiChatStore.initSync();
              }
            })
            .catch(() => {
              aiChatStore.setEnabled(false);
            });
        });
      }

      // Keyboard shortcut: Cmd/Ctrl+K to toggle AI
      const handleKeyDown = (e: KeyboardEvent) => {
        if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
          e.preventDefault();
          if (aiChatStore.enabled) {
            aiChatStore.toggle();
          }
        }
        // Escape to close
        if (e.key === 'Escape' && aiChatStore.isOpen) {
          aiChatStore.close();
        }
      };

      document.addEventListener('keydown', handleKeyDown);
      onCleanup(() => {
        document.removeEventListener('keydown', handleKeyDown);
      });
    });

    return (
      <Show
        when={!isLoading()}
        fallback={
          <div class="min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900">
            <div class="text-gray-600 dark:text-gray-400">Loading...</div>
          </div>
        }
      >
        <Show when={!needsAuth()} fallback={<Login onLogin={handleLogin} hasAuth={hasAuth()} securityStatus={securityStatus() ?? undefined} />}>
          <ErrorBoundary>
            <Show when={enhancedStore()} fallback={
              <div class="min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900">
                <div class="text-gray-600 dark:text-gray-400">Initializing...</div>
              </div>
            }>
              <WebSocketContext.Provider value={enhancedStore()!}>
                <DarkModeContext.Provider value={darkMode}>
                  <Show when={!kioskMode()}>
                    <SecurityWarning />
                    <DemoBanner />
                    <UpdateBanner />
                    <GitHubStarBanner />
                    <GlobalUpdateProgressWatcher />
                  </Show>
                  {/* Main layout container - flexbox to allow AI panel to push content */}
                  <div class="flex h-screen overflow-hidden">
                    {/* Main content area - shrinks when AI panel is open, scrolls independently */}
                    <div class={`flex-1 min-w-0 overflow-y-auto bg-gray-100 dark:bg-gray-900 text-gray-800 dark:text-gray-200 font-sans py-4 sm:py-6 transition-all duration-300`}>
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
                        tokenScopes={() => securityStatus()?.tokenScopes}
                      >
                        {props.children}
                      </AppLayout>
                    </div>
                    {/* AI Panel - slides in from right, pushes content */}
                    <AIChat onClose={() => aiChatStore.close()} />
                  </div>
                  <TokenRevealDialog />
                  {/* AI Assistant Button moved to AppLayout to access kioskMode state */}
                  <TooltipRoot />
                </DarkModeContext.Provider>
              </WebSocketContext.Provider>
            </Show>
          </ErrorBoundary>
        </Show>
        <ToastContainer />
      </Show>
    );
  };

  // Use Router with routes
  return (
    <Router root={RootLayout}>
      <Route path="/" component={() => <Navigate href="/proxmox/overview" />} />
      <Route path="/proxmox" component={() => <Navigate href="/proxmox/overview" />} />
      <Route path="/proxmox/overview" component={DashboardView} />
      <Route path="/proxmox/storage" component={StorageComponent} />
      <Route path="/proxmox/ceph" component={CephPage} />
      <Route path="/proxmox/replication" component={Replication} />
      <Route path="/proxmox/mail" component={MailGateway} />
      <Route path="/proxmox/backups" component={Backups} />
      <Route path="/storage" component={() => <Navigate href="/proxmox/storage" />} />
      <Route path="/backups" component={() => <Navigate href="/proxmox/backups" />} />
      <Route path="/docker" component={DockerRoute} />
      <Route path="/kubernetes" component={KubernetesRoute} />
      <Route path="/hosts" component={HostsRoute} />

      <Route path="/servers" component={() => <Navigate href="/hosts" />} />
      <Route path="/alerts/*" component={AlertsPage} />
      <Route path="/ai/*" component={AIIntelligencePage} />
      <Route path="/settings/*" component={SettingsRoute} />
    </Router>
  );
}

function ConnectionStatusBadge(props: {
  connected: () => boolean;
  reconnecting: () => boolean;
  class?: string;
}) {
  return (
    <div
      class={`group status text-xs rounded-full flex items-center justify-center transition-all duration-500 ease-in-out px-1.5 ${props.connected()
        ? 'connected bg-green-200 dark:bg-green-700 text-green-700 dark:text-green-300 min-w-6 h-6 group-hover:px-3'
        : props.reconnecting()
          ? 'reconnecting bg-yellow-200 dark:bg-yellow-700 text-yellow-700 dark:text-yellow-300 py-1'
          : 'disconnected bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 min-w-6 h-6 group-hover:px-3'
        } ${props.class ?? ''}`}
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
      <span
        class={`whitespace-nowrap overflow-hidden transition-all duration-500 ${props.connected() || (!props.connected() && !props.reconnecting())
          ? 'max-w-0 group-hover:max-w-[100px] group-hover:ml-2 group-hover:mr-1 opacity-0 group-hover:opacity-100'
          : 'max-w-[100px] ml-1 opacity-100'
          }`}
      >
        {props.connected()
          ? 'Connected'
          : props.reconnecting()
            ? 'Reconnecting...'
            : 'Disconnected'}
      </span>
    </div>
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
  tokenScopes: () => string[] | undefined;
  children?: JSX.Element;
}) {
  const navigate = useNavigate();
  const location = useLocation();

  const readSeenPlatforms = (): Record<string, boolean> => {
    if (typeof window === 'undefined') return {};
    try {
      const stored = window.localStorage.getItem(STORAGE_KEYS.PLATFORMS_SEEN);
      if (stored) {
        const parsed = JSON.parse(stored) as Record<string, boolean>;
        if (parsed && typeof parsed === 'object') {
          return parsed;
        }
      }
    } catch (error) {
      logger.warn('Failed to parse stored platform visibility preferences', error);
    }
    return {};
  };

  const [seenPlatforms, setSeenPlatforms] = createSignal<Record<string, boolean>>(readSeenPlatforms());

  // Reactive kiosk mode state
  const [kioskMode, setKioskModeSignal] = createSignal(isKioskMode());

  // Subscribe to kiosk mode changes from other sources (like URL params)
  onMount(() => {
    const unsubscribe = subscribeToKioskMode((enabled) => {
      setKioskModeSignal(enabled);
    });
    onCleanup(unsubscribe);
  });

  // Auto-enable kiosk mode for monitoring-only tokens (if no user preference is set)
  createEffect(() => {
    const scopes = props.tokenScopes();
    // Only proceed if scopes are loaded and equal exactly ['monitoring:read']
    if (scopes && scopes.length === 1 && scopes[0] === MONITORING_READ_SCOPE) {
      // Check if user has an explicit preference
      const pref = getKioskModePreference();
      // If preference is unset (null), default to Kiosk Mode
      if (pref === null) {
        setKioskMode(true);
      }
    }
  });

  const toggleKioskMode = () => {
    const newValue = !kioskMode();
    setKioskMode(newValue);
    setKioskModeSignal(newValue);
  };

  const persistSeenPlatforms = (map: Record<string, boolean>) => {
    if (typeof window === 'undefined') return;
    try {
      window.localStorage.setItem(STORAGE_KEYS.PLATFORMS_SEEN, JSON.stringify(map));
    } catch (error) {
      logger.warn('Failed to persist platform visibility preferences', error);
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
    if (path.startsWith('/kubernetes')) return 'kubernetes';
    if (path.startsWith('/hosts')) return 'hosts';
    if (path.startsWith('/servers')) return 'hosts'; // Legacy redirect
    if (path.startsWith('/alerts')) return 'alerts';
    if (path.startsWith('/ai')) return 'ai';
    if (path.startsWith('/settings')) return 'settings';
    return 'proxmox';
  };
  const hasDockerHosts = createMemo(() => (props.state().dockerHosts?.length ?? 0) > 0);
  const hasKubernetesClusters = createMemo(() => (props.state().kubernetesClusters?.length ?? 0) > 0);
  const hasHosts = createMemo(() => (props.state().hosts?.length ?? 0) > 0);
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
    if (hasKubernetesClusters()) {
      markPlatformSeen('kubernetes');
    }
  });

  createEffect(() => {
    if (hasProxmoxHosts()) {
      markPlatformSeen('proxmox');
    }
  });

  createEffect(() => {
    if (hasHosts()) {
      markPlatformSeen('hosts');
    }
  });

  const platformTabs = createMemo(() => {
    const allPlatforms = [
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
        alwaysShow: true, // Proxmox is the default, always show
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
          <BoxesIcon class="w-4 h-4 shrink-0" />
        ),
        alwaysShow: true, // Docker is commonly used, keep visible
      },
      {
        id: 'kubernetes' as const,
        label: 'Kubernetes',
        route: '/kubernetes',
        settingsRoute: '/settings/agents',
        tooltip: 'Monitor Kubernetes clusters and workloads',
        enabled: hasKubernetesClusters(),
        live: hasKubernetesClusters(),
        icon: (
          <NetworkIcon class="w-4 h-4 shrink-0" />
        ),
        alwaysShow: false, // Only show when clusters exist
      },
      {
        id: 'hosts' as const,
        label: 'Hosts',
        route: '/hosts',
        settingsRoute: '/settings/host-agents',
        tooltip: 'Monitor hosts with the host agent',
        enabled: hasHosts() || !!seenPlatforms()['hosts'],
        live: hasHosts(),
        icon: (
          <MonitorIcon class="w-4 h-4 shrink-0" />
        ),
        alwaysShow: true, // Hosts is commonly used, keep visible
      },
    ];

    // Filter out platforms that should be hidden when not configured
    return allPlatforms.filter(p => p.alwaysShow || p.enabled);
  });

  const utilityTabs = createMemo(() => {
    const allAlerts = props.state().activeAlerts || [];
    const breakdown = allAlerts.reduce(
      (acc, alert: Alert) => {
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

    // Check if settings should be shown based on token scopes
    // If no scopes (session auth), show settings
    // If scopes include '*' (wildcard) or 'settings:read', show settings
    const scopes = props.tokenScopes();
    const hasSettingsAccess = !scopes || scopes.length === 0 ||
      scopes.includes('*') || scopes.includes('settings:read');

    const tabs: Array<{
      id: 'alerts' | 'ai' | 'settings';
      label: string;
      route: string;
      tooltip: string;
      badge: 'update' | 'pro' | null;
      count: number | undefined;
      breakdown: { warning: number; critical: number } | undefined;
      icon: JSX.Element;
    }> = [
        {
          id: 'alerts',
          label: 'Alerts',
          route: '/alerts',
          tooltip: 'Review active alerts and automation rules',
          badge: null,
          count: activeAlertCount,
          breakdown,
          icon: <BellIcon class="w-4 h-4 shrink-0" />,
        },
        {
          id: 'ai',
          label: 'Patrol',
          route: '/ai',
          tooltip: 'Pulse Patrol monitoring and analysis',
          badge: null, // Patrol is free with BYOK; auto-fix is Pro
          count: undefined,
          breakdown: undefined,
          icon: <PulsePatrolLogo class="w-4 h-4 shrink-0" />,
        },
      ];

    // Only show settings tab if user has access
    if (hasSettingsAccess) {
      tabs.push({
        id: 'settings',
        label: 'Settings',
        route: '/settings',
        tooltip: 'Configure Pulse preferences and integrations',
        badge: updateStore.isUpdateVisible() ? 'update' : null,
        count: undefined,
        breakdown: undefined,
        icon: <SettingsIcon class="w-4 h-4 shrink-0" />,
      });
    }

    return tabs;
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
    <div class={`pulse-shell ${layoutStore.isFullWidth() || kioskMode() ? 'pulse-shell--full-width' : ''}`}>
      {/* Header - simplified in kiosk mode */}
      <div class={`header mb-3 flex items-center gap-2 ${kioskMode() ? 'justify-end' : 'justify-between sm:grid sm:grid-cols-[1fr_auto_1fr] sm:items-center sm:gap-0'}`}>
        <Show when={!kioskMode()}>
          <div class="flex items-center gap-2 sm:flex-initial sm:gap-2 sm:col-start-2 sm:col-end-3 sm:justify-self-center">
            <div class="flex items-center gap-2">
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
          </div>
        </Show>
        <div class={`header-controls flex items-center gap-2 ${kioskMode() ? '' : 'justify-end sm:col-start-3 sm:col-end-4 sm:w-auto sm:justify-end sm:justify-self-end'}`}>
          <Show when={props.hasAuth() && !props.needsAuth()}>
            <div class="flex items-center gap-2">
              {/* Kiosk Mode Toggle */}
              <button
                type="button"
                onClick={toggleKioskMode}
                class={`group relative flex h-7 w-7 items-center justify-center rounded-full text-xs transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 ${kioskMode()
                  ? 'bg-blue-100 text-blue-700 hover:bg-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:hover:bg-blue-800'
                  : 'bg-gray-200 text-gray-700 hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600'
                  }`}
                title={kioskMode() ? 'Exit kiosk mode (show navigation)' : 'Enter kiosk mode (hide navigation)'}
                aria-label={kioskMode() ? 'Exit kiosk mode' : 'Enter kiosk mode'}
                aria-pressed={kioskMode()}
              >
                <Show when={kioskMode()} fallback={<Maximize2Icon class="h-3 w-3 flex-shrink-0" />}>
                  <Minimize2Icon class="h-3 w-3 flex-shrink-0" />
                </Show>
              </button>
              <Show when={props.proxyAuthInfo()?.username}>
                <span class="text-xs px-2 py-1 text-gray-600 dark:text-gray-400">
                  {props.proxyAuthInfo()?.username}
                </span>
              </Show>
              <button
                type="button"
                onClick={props.handleLogout}
                class="group relative flex h-7 w-7 items-center justify-center rounded-full bg-gray-200 text-xs text-gray-700 transition hover:bg-gray-300 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
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
              </button>
            </div>
          </Show>
          <ConnectionStatusBadge
            connected={props.connected}
            reconnecting={props.reconnecting}
            class="flex-shrink-0"
          />
        </div>
      </div>

      {/* Tabs - hidden in kiosk mode */}
      <Show when={!kioskMode()}>
        <div
          class="tabs mb-2 flex items-end gap-2 overflow-x-auto overflow-y-hidden whitespace-nowrap border-b border-gray-300 dark:border-gray-700 scrollbar-hide"
          role="tablist"
          aria-label="Primary navigation"
        >
          <div class="flex items-end gap-1" role="group" aria-label="Infrastructure">
            <For each={platformTabs()}>
              {(platform) => {
                const isActive = () => getActiveTab() === platform.id;
                const disabled = () => !platform.enabled;
                const baseClasses =
                  'tab relative px-1.5 sm:px-3 py-1.5 text-xs sm:text-sm font-medium flex items-center gap-1 sm:gap-1.5 rounded-t border border-transparent transition-colors whitespace-nowrap cursor-pointer';

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
                    <span class="hidden xs:inline">{platform.label}</span>
                    <span class="xs:hidden">{platform.label.charAt(0)}</span>
                  </div>
                );
              }}
            </For>
          </div>
          <div class="flex items-end gap-1 ml-auto" role="group" aria-label="System">
            <div class="flex items-end gap-1 pl-1 sm:pl-4">
              <For each={utilityTabs()}>
                {(tab) => {
                  const isActive = () => getActiveTab() === tab.id;
                  const baseClasses =
                    'tab relative px-1.5 sm:px-3 py-1.5 text-xs sm:text-sm font-medium flex items-center gap-1 sm:gap-1.5 rounded-t border border-transparent transition-colors whitespace-nowrap cursor-pointer';

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
                      <span class="flex items-center gap-1">
                        <span class="hidden xs:inline">{tab.label}</span>
                        <span class="xs:hidden">{tab.label.charAt(0)}</span>
                        {tab.id === 'alerts' && (() => {
                          const total = tab.count ?? 0;
                          if (total <= 0) {
                            return null;
                          }
                          return (
                            <span class="inline-flex items-center gap-1">
                              {tab.breakdown && tab.breakdown.critical > 0 && (
                                <span class="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-bold text-white bg-red-600 dark:bg-red-500 rounded-full">
                                  {tab.breakdown.critical}
                                </span>
                              )}
                              {tab.breakdown && tab.breakdown.warning > 0 && (
                                <span class="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-semibold text-amber-900 dark:text-amber-100 bg-amber-200 dark:bg-amber-500/80 rounded-full">
                                  {tab.breakdown.warning}
                                </span>
                              )}
                            </span>
                          );
                        })()}
                      </span>
                      <Show when={tab.badge === 'update'}>
                        <span class="ml-1 flex items-center">
                          <span class="sr-only">Update available</span>
                          <span aria-hidden="true" class="block h-2 w-2 rounded-full bg-red-500 animate-pulse"></span>
                        </span>
                      </Show>
                      <Show when={tab.badge === 'pro'}>
                        <span class="ml-1.5 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900/50 rounded">
                          Pro
                        </span>
                      </Show>
                    </div>
                  );
                }}
              </For>
            </div>
          </div>
        </div>
      </Show>

      {/* Main Content */}
      <main
        id="main"
        class="tab-content block bg-white dark:bg-gray-800 rounded-b rounded-tr rounded-tl shadow mb-2"
      >
        <div class="pulse-panel">
          <Suspense fallback={<div class="p-6 text-sm text-gray-500 dark:text-gray-400">Loading view...</div>}>
            {props.children}
          </Suspense>
        </div>
      </main>

      {/* Footer - hidden in kiosk mode */}
      <Show when={!kioskMode()}>
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
      </Show>

      {/* Fixed AI Assistant Button - only shows when chat is CLOSED and NOT in kiosk mode */}
      <Show when={aiChatStore.enabled === true && !aiChatStore.isOpenSignal() && !kioskMode()}>
        <button
          type="button"
          onClick={() => aiChatStore.toggle()}
          class="fixed right-0 top-1/2 -translate-y-1/2 z-40 flex items-center gap-1.5 pl-2 pr-1.5 py-3 rounded-l-xl bg-blue-600 text-white shadow-lg hover:bg-blue-700 transition-colors duration-200 group sm:top-1/2 sm:translate-y-[-50%] top-auto bottom-20 translate-y-0"
          title={aiChatStore.context.context?.name ? `Pulse Assistant - ${aiChatStore.context.context.name}` : 'Pulse Assistant (⌘K)'}
          aria-label="Expand Pulse Assistant"
        >
          <svg
            class="h-5 w-5 flex-shrink-0"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456zM16.894 20.567L16.5 21.75l-.394-1.183a2.25 2.25 0 00-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 001.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 001.423 1.423l1.183.394-1.183.394a2.25 2.25 0 00-1.423 1.423z"
            />
          </svg>
        </button>
      </Show>
    </div>
  );
}

export default App; // Test hot-reload comment $(date)
