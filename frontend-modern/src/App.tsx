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
import { UpdatesAPI } from './api/updates';
import type { VersionInfo } from './api/updates';
import { apiFetch } from './utils/apiClient';
import { SettingsAPI } from './api/settings';
import { eventBus } from './stores/events';
import { updateStore } from './stores/updates';
import { UpdateBanner } from './components/UpdateBanner';
import { DemoBanner } from './components/DemoBanner';
import { createTooltipSystem } from './components/shared/Tooltip';
import type { State } from '@/types/api';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { startMetricsSampler } from './stores/metricsSampler';
import { seedFromBackend } from './stores/metricsHistory';
import { getMetricsViewMode } from './stores/metricsViewMode';
import BoxesIcon from 'lucide-solid/icons/boxes';
import MonitorIcon from 'lucide-solid/icons/monitor';
import BellIcon from 'lucide-solid/icons/bell';
import SettingsIcon from 'lucide-solid/icons/settings';
import { TokenRevealDialog } from './components/TokenRevealDialog';
import { useAlertsActivation } from './stores/alertsActivation';
import { UpdateProgressModal } from './components/UpdateProgressModal';
import type { UpdateStatus } from './api/updates';
import { AIChat } from './components/AI/AIChat';
import { aiChatStore } from './stores/aiChat';

const Dashboard = lazy(() =>
  import('./components/Dashboard/Dashboard').then((module) => ({ default: module.Dashboard })),
);
const StorageComponent = lazy(() => import('./components/Storage/Storage'));
const Backups = lazy(() => import('./components/Backups/Backups'));
const Replication = lazy(() => import('./components/Replication/Replication'));
const MailGateway = lazy(() => import('./components/PMG/MailGateway'));
const AlertsPage = lazy(() =>
  import('./pages/Alerts').then((module) => ({ default: module.Alerts })),
);
const SettingsPage = lazy(() => import('./components/Settings/Settings'));
const DockerHosts = lazy(() =>
  import('./components/Docker/DockerHosts').then((module) => ({ default: module.DockerHosts })),
);
const HostsOverview = lazy(() =>
  import('./components/Hosts/HostsOverview').then((module) => ({
    default: module.HostsOverview,
  })),
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

// Docker route component that properly uses activeAlerts from useWebSocket
function DockerRoute() {
  const wsContext = useContext(WebSocketContext);
  if (!wsContext) {
    return <div>Loading...</div>;
  }
  const { state, activeAlerts } = wsContext;

  // Use unified resources if available, fall back to legacy state.dockerHosts
  const hosts = createMemo(() => {
    const dockerHostResources = state.resources?.filter(r => r.type === 'docker-host') ?? [];
    const dockerContainerResources = state.resources?.filter(r => r.type === 'docker-container') ?? [];

    // If we have unified resources, convert and reconstruct hierarchy
    if (dockerHostResources.length > 0) {
      return dockerHostResources.map(h => {
        const platformData = h.platformData as Record<string, unknown> | undefined;

        // Find containers belonging to this host
        const hostContainers = dockerContainerResources
          .filter(c => c.parentId === h.id)
          .map(c => {
            const cPlatform = c.platformData as Record<string, unknown> | undefined;
            return {
              id: c.id,
              name: c.name,
              image: cPlatform?.image as string ?? '',
              state: c.status === 'running' ? 'running' : 'exited',
              status: c.status,
              health: cPlatform?.health as string | undefined,
              cpuPercent: c.cpu?.current ?? 0,
              memoryUsageBytes: c.memory?.used ?? 0,
              memoryLimitBytes: c.memory?.total ?? 0,
              memoryPercent: c.memory?.current ?? 0,
              uptimeSeconds: c.uptime ?? 0,
              restartCount: cPlatform?.restartCount as number ?? 0,
              exitCode: cPlatform?.exitCode as number ?? 0,
              createdAt: cPlatform?.createdAt as number ?? 0,
              startedAt: cPlatform?.startedAt as number | undefined,
              finishedAt: cPlatform?.finishedAt as number | undefined,
              ports: cPlatform?.ports,
              labels: cPlatform?.labels as Record<string, string> | undefined,
              networks: cPlatform?.networks,
            };
          });

        return {
          id: h.id,
          agentId: platformData?.agentId as string ?? h.id,
          hostname: h.identity?.hostname ?? h.name,
          displayName: h.displayName || h.name,
          customDisplayName: platformData?.customDisplayName as string | undefined,
          machineId: h.identity?.machineId,
          os: platformData?.os as string | undefined,
          kernelVersion: platformData?.kernelVersion as string | undefined,
          architecture: platformData?.architecture as string | undefined,
          runtime: platformData?.runtime as string ?? 'docker',
          runtimeVersion: platformData?.runtimeVersion as string | undefined,
          dockerVersion: platformData?.dockerVersion as string | undefined,
          cpus: platformData?.cpus as number ?? 0,
          totalMemoryBytes: h.memory?.total ?? 0,
          uptimeSeconds: h.uptime ?? 0,
          cpuUsagePercent: h.cpu?.current,
          loadAverage: platformData?.loadAverage as number[] | undefined,
          memory: h.memory ? {
            total: h.memory.total ?? 0,
            used: h.memory.used ?? 0,
            free: h.memory.free ?? 0,
            usage: h.memory.current,
          } : undefined,
          disks: platformData?.disks,
          networkInterfaces: platformData?.networkInterfaces,
          status: h.status === 'online' || h.status === 'running' ? 'online' : h.status,
          lastSeen: h.lastSeen,
          intervalSeconds: platformData?.intervalSeconds as number ?? 30,
          agentVersion: platformData?.agentVersion as string | undefined,
          containers: hostContainers,
          services: platformData?.services,
          tasks: platformData?.tasks,
          swarm: platformData?.swarm,
          tokenId: platformData?.tokenId as string | undefined,
          tokenName: platformData?.tokenName as string | undefined,
          tokenHint: platformData?.tokenHint as string | undefined,
          tokenLastUsedAt: platformData?.tokenLastUsedAt as number | undefined,
          hidden: platformData?.hidden as boolean | undefined,
          pendingUninstall: platformData?.pendingUninstall as boolean | undefined,
          command: platformData?.command,
          isLegacy: platformData?.isLegacy as boolean | undefined,
        };
      });
    }

    // Return empty array if no unified resources available
    return [];
  });

  return <DockerHosts hosts={hosts() as any} activeAlerts={activeAlerts} />;
}

function HostsRoute() {
  const wsContext = useContext(WebSocketContext);
  if (!wsContext) {
    return <div>Loading...</div>;
  }
  const { state } = wsContext;

  // Use unified resources if available, fall back to legacy state.hosts
  // During migration: resources may be empty initially, so we need the fallback
  const hosts = createMemo(() => {
    const unifiedHosts = state.resources?.filter(r => r.type === 'host') ?? [];

    // If we have unified resources, convert them to legacy Host format
    // (This will be simplified once HostsOverview is migrated to use resources directly)
    if (unifiedHosts.length > 0) {
      return unifiedHosts.map(r => {
        const platformData = r.platformData as Record<string, unknown> | undefined;
        return {
          id: r.id,
          hostname: r.identity?.hostname ?? r.name,
          displayName: r.displayName || r.name,
          platform: platformData?.platform as string | undefined,
          osName: platformData?.osName as string | undefined,
          osVersion: platformData?.osVersion as string | undefined,
          kernelVersion: platformData?.kernelVersion as string | undefined,
          architecture: platformData?.architecture as string | undefined,
          cpuCount: platformData?.cpuCount as number | undefined,
          cpuUsage: r.cpu?.current,
          loadAverage: platformData?.loadAverage as number[] | undefined,
          memory: r.memory ? {
            total: r.memory.total ?? 0,
            used: r.memory.used ?? 0,
            free: r.memory.free ?? 0,
            usage: r.memory.current,
          } : { total: 0, used: 0, free: 0, usage: 0 },
          disks: platformData?.disks as Array<{
            total: number;
            used: number;
            free: number;
            usage: number;
            mountpoint?: string;
          }> | undefined,
          diskIO: platformData?.diskIO,
          networkInterfaces: platformData?.networkInterfaces,
          sensors: platformData?.sensors,
          raid: platformData?.raid,
          status: r.status === 'online' || r.status === 'running' ? 'online' : r.status,
          uptimeSeconds: r.uptime,
          lastSeen: r.lastSeen,
          intervalSeconds: platformData?.intervalSeconds as number | undefined,
          agentVersion: platformData?.agentVersion as string | undefined,
          tokenId: platformData?.tokenId as string | undefined,
          tokenName: platformData?.tokenName as string | undefined,
          tags: r.tags,
        };
      });
    }

    // Return empty array if no unified resources available
    return [];
  });

  return (
    <HostsOverview hosts={hosts() as any} connectionHealth={state.connectionHealth ?? {}} />
  );
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

    // Subscribe to theme change events
    eventBus.on('theme_changed', handleThemeChange);

    // Cleanup on unmount
    onCleanup(() => {
      eventBus.off('theme_changed', handleThemeChange);
    });
  });

  // Check auth on mount
  onMount(async () => {
    logger.debug('[App] Starting auth check...');

    // Check if we just logged out - if so, always show login page
    const justLoggedOut = localStorage.getItem('just_logged_out');
    if (justLoggedOut) {
      localStorage.removeItem('just_logged_out');
      logger.debug('[App] User logged out, showing login page');
      setHasAuth(true); // Force showing login instead of setup
      setNeedsAuth(true);
      setIsLoading(false);
      return;
    }

    // First check security status to see if auth is configured
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
        setHasAuth(false);
        setNeedsAuth(true);
        return;
      }

      if (!securityRes.ok) {
        throw new Error(`Security status request failed with status ${securityRes.status}`);
      }

      const securityData = await securityRes.json();
      logger.debug('[App] Security status fetched', securityData);

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
            logger.error('Failed to load theme from server', error);
          }
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
            logger.error('Failed to load theme from server', error);
          }
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
            logger.error('Failed to load theme from server', error);
          }
        } else {
          // We have a local preference, just mark that we've checked the server
          setHasLoadedServerTheme(true);
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

  // Dashboard view using unified resources with fallback
  const DashboardView = () => {
    // Use unified resources if available, fall back to legacy data
    const vms = createMemo(() => {
      const vmResources = state().resources?.filter(r => r.type === 'vm') ?? [];
      if (vmResources.length > 0) {
        return vmResources.map(r => {
          const platformData = r.platformData as Record<string, unknown> | undefined;
          return {
            id: r.id,
            vmid: platformData?.vmid as number ?? 0,
            name: r.name,
            node: platformData?.node as string ?? '',
            instance: platformData?.instance as string ?? r.platformId,
            status: r.status === 'running' ? 'running' : 'stopped',
            type: 'qemu',
            cpu: r.cpu?.current ?? 0,
            cpus: platformData?.cpus as number ?? 1,
            memory: r.memory ? {
              total: r.memory.total ?? 0,
              used: r.memory.used ?? 0,
              free: r.memory.free ?? 0,
              usage: r.memory.current,
            } : { total: 0, used: 0, free: 0, usage: 0 },
            disk: r.disk ? {
              total: r.disk.total ?? 0,
              used: r.disk.used ?? 0,
              free: r.disk.free ?? 0,
              usage: r.disk.current,
            } : { total: 0, used: 0, free: 0, usage: 0 },
            networkIn: r.network?.rxBytes ?? 0,
            networkOut: r.network?.txBytes ?? 0,
            diskRead: 0,
            diskWrite: 0,
            uptime: r.uptime ?? 0,
            template: platformData?.template as boolean ?? false,
            lastBackup: platformData?.lastBackup as number ?? 0,
            tags: r.tags ?? [],
            lock: platformData?.lock as string ?? '',
            lastSeen: new Date(r.lastSeen).toISOString(),
          };
        });
      }
      // Return empty array if no unified resources available
      return [];
    });

    const containers = createMemo(() => {
      const containerResources = state().resources?.filter(r => r.type === 'container') ?? [];
      if (containerResources.length > 0) {
        return containerResources.map(r => {
          const platformData = r.platformData as Record<string, unknown> | undefined;
          return {
            id: r.id,
            vmid: platformData?.vmid as number ?? 0,
            name: r.name,
            node: platformData?.node as string ?? '',
            instance: platformData?.instance as string ?? r.platformId,
            status: r.status === 'running' ? 'running' : 'stopped',
            type: 'lxc',
            cpu: r.cpu?.current ?? 0,
            cpus: platformData?.cpus as number ?? 1,
            memory: r.memory ? {
              total: r.memory.total ?? 0,
              used: r.memory.used ?? 0,
              free: r.memory.free ?? 0,
              usage: r.memory.current,
            } : { total: 0, used: 0, free: 0, usage: 0 },
            disk: r.disk ? {
              total: r.disk.total ?? 0,
              used: r.disk.used ?? 0,
              free: r.disk.free ?? 0,
              usage: r.disk.current,
            } : { total: 0, used: 0, free: 0, usage: 0 },
            networkIn: r.network?.rxBytes ?? 0,
            networkOut: r.network?.txBytes ?? 0,
            diskRead: 0,
            diskWrite: 0,
            uptime: r.uptime ?? 0,
            template: platformData?.template as boolean ?? false,
            lastBackup: platformData?.lastBackup as number ?? 0,
            tags: r.tags ?? [],
            lock: platformData?.lock as string ?? '',
            lastSeen: new Date(r.lastSeen).toISOString(),
          };
        });
      }
      // Return empty array if no unified resources available
      return [];
    });

    const nodes = createMemo(() => {
      const nodeResources = state().resources?.filter(r => r.type === 'node') ?? [];
      if (nodeResources.length > 0) {
        return nodeResources.map(r => {
          const platformData = r.platformData as Record<string, unknown> | undefined;
          return {
            id: r.id,
            name: r.name,
            displayName: r.displayName,
            instance: r.platformId,
            host: platformData?.host as string ?? '',
            status: r.status,
            type: 'node',
            cpu: r.cpu?.current ?? 0,
            memory: r.memory ? {
              total: r.memory.total ?? 0,
              used: r.memory.used ?? 0,
              free: r.memory.free ?? 0,
              usage: r.memory.current,
            } : { total: 0, used: 0, free: 0, usage: 0 },
            disk: r.disk ? {
              total: r.disk.total ?? 0,
              used: r.disk.used ?? 0,
              free: r.disk.free ?? 0,
              usage: r.disk.current,
            } : { total: 0, used: 0, free: 0, usage: 0 },
            uptime: r.uptime ?? 0,
            loadAverage: platformData?.loadAverage as number[] ?? [],
            kernelVersion: platformData?.kernelVersion as string ?? '',
            pveVersion: platformData?.pveVersion as string ?? '',
            cpuInfo: platformData?.cpuInfo ?? { model: '', cores: 0, sockets: 0, mhz: '' },
            temperature: platformData?.temperature,
            lastSeen: new Date(r.lastSeen).toISOString(),
            connectionHealth: platformData?.connectionHealth as string ?? 'unknown',
            isClusterMember: platformData?.isClusterMember as boolean | undefined,
            clusterName: platformData?.clusterName as string | undefined,
          };
        });
      }
      // Return empty array if no unified resources available
      return [];
    });

    return (
      <Dashboard vms={vms() as any} containers={containers() as any} nodes={nodes() as any} />
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
        <Show when={!needsAuth()} fallback={<Login onLogin={handleLogin} hasAuth={hasAuth()} />}>
          <ErrorBoundary>
            <Show when={enhancedStore()} fallback={
              <div class="min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900">
                <div class="text-gray-600 dark:text-gray-400">Initializing...</div>
              </div>
            }>
              <WebSocketContext.Provider value={enhancedStore()!}>
                <DarkModeContext.Provider value={darkMode}>
                  <SecurityWarning />
                  <DemoBanner />
                  <UpdateBanner />
                  <GlobalUpdateProgressWatcher />
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
                      >
                        {props.children}
                      </AppLayout>
                    </div>
                    {/* AI Panel - slides in from right, pushes content */}
                    <AIChat onClose={() => aiChatStore.close()} />
                  </div>
                  <ToastContainer />
                  <TokenRevealDialog />
                  {/* Fixed AI Assistant Button - always visible on the side when AI is enabled */}
                  <Show when={aiChatStore.enabled !== false && !aiChatStore.isOpen}>
                    {/* This component only shows when chat is closed */}
                    <button
                      type="button"
                      onClick={() => aiChatStore.toggle()}
                      class="fixed right-0 top-1/2 -translate-y-1/2 z-40 flex items-center gap-1.5 pl-2 pr-1.5 py-3 rounded-l-xl bg-gradient-to-r from-purple-600 to-purple-700 text-white shadow-lg hover:from-purple-700 hover:to-purple-800 transition-all duration-200 group"
                      title={aiChatStore.context.context?.name ? `AI Assistant - ${aiChatStore.context.context.name}` : 'AI Assistant (⌘K)'}
                      aria-label="Expand AI Assistant"
                    >
                      {/* Double chevron left - expand */}
                      <svg
                        class="h-4 w-4 flex-shrink-0"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                        stroke-width="2"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M11 19l-7-7 7-7M18 19l-7-7 7-7"
                        />
                      </svg>
                      {/* Context indicator - shows count when items are in context */}
                      <Show when={aiChatStore.contextItems.length > 0}>
                        <span class="min-w-[18px] h-[18px] px-1 flex items-center justify-center text-[10px] font-bold bg-green-500 text-white rounded-full">
                          {aiChatStore.contextItems.length}
                        </span>
                      </Show>
                    </button>
                  </Show>
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
      <Route path="/proxmox/overview" component={DashboardView} />
      <Route path="/proxmox/storage" component={StorageComponent} />
      <Route path="/proxmox/replication" component={Replication} />
      <Route path="/proxmox/mail" component={MailGateway} />
      <Route path="/proxmox/backups" component={Backups} />
      <Route path="/storage" component={() => <Navigate href="/proxmox/storage" />} />
      <Route path="/backups" component={() => <Navigate href="/proxmox/backups" />} />
      <Route path="/docker" component={DockerRoute} />
      <Route path="/hosts" component={HostsRoute} />

      <Route path="/servers" component={() => <Navigate href="/hosts" />} />
      <Route path="/alerts/*" component={AlertsPage} />
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
    if (path.startsWith('/hosts')) return 'hosts';
    if (path.startsWith('/servers')) return 'hosts'; // Legacy redirect
    if (path.startsWith('/alerts')) return 'alerts';
    if (path.startsWith('/settings')) return 'settings';
    return 'proxmox';
  };
  const hasDockerHosts = createMemo(() => (props.state().dockerHosts?.length ?? 0) > 0);
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
          <BoxesIcon class="w-4 h-4 shrink-0" />
        ),
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
        icon: <BellIcon class="w-4 h-4 shrink-0" />,
      },
      {
        id: 'settings' as const,
        label: 'Settings',
        route: '/settings',
        tooltip: 'Configure Pulse preferences and integrations',
        badge: updateStore.isUpdateVisible() ? ('update' as const) : null,
        count: undefined,
        breakdown: undefined,
        icon: <SettingsIcon class="w-4 h-4 shrink-0" />,
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
      <div class="header mb-3 flex items-center justify-between gap-2 sm:grid sm:grid-cols-[1fr_auto_1fr] sm:items-center sm:gap-0">
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
        <div class="header-controls flex items-center gap-2 justify-end sm:col-start-3 sm:col-end-4 sm:w-auto sm:justify-end sm:justify-self-end">
          <Show when={props.hasAuth() && !props.needsAuth()}>
            <div class="flex items-center gap-2">
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
            </div>
          </Show>
          <ConnectionStatusBadge
            connected={props.connected}
            reconnecting={props.reconnecting}
            class="flex-shrink-0"
          />
        </div>
      </div>

      {/* Tabs */}
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
                      <span class="ml-1 flex items-center">
                        <span class="sr-only">Update available</span>
                        <span aria-hidden="true" class="block h-2 w-2 rounded-full bg-red-500 animate-pulse"></span>
                      </span>
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
        class="tab-content block bg-white dark:bg-gray-800 rounded-b rounded-tr rounded-tl shadow mb-2"
      >
        <div class="pulse-panel">
          <Suspense fallback={<div class="p-6 text-sm text-gray-500 dark:text-gray-400">Loading view...</div>}>
            {props.children}
          </Suspense>
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
