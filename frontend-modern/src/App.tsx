import {
  Show,
  createSignal,
  createContext,
  useContext,
  createEffect,
  onMount,
  onCleanup,
  getOwner,
  runWithOwner,
} from 'solid-js';
import { Router, Route, useNavigate, useLocation } from '@solidjs/router';
import { getGlobalWebSocketStore } from './stores/websocket-global';
import { Dashboard } from './components/Dashboard/Dashboard';
import StorageComponent from './components/Storage/Storage';
import Backups from './components/Backups/Backups';
import Settings from './components/Settings/Settings';
import { Alerts } from './pages/Alerts';
import { ToastContainer } from './components/Toast/Toast';
import { ErrorBoundary } from './components/ErrorBoundary';
import NotificationContainer from './components/NotificationContainer';
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
import { DemoBanner } from './components/DemoBanner';

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

function App() {
  const owner = getOwner();
  const acquireWsStore = (): EnhancedStore => {
    const store = owner
      ? runWithOwner(owner, () => getGlobalWebSocketStore())
      : getGlobalWebSocketStore();
    return store || getGlobalWebSocketStore();
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
  const state = () =>
    wsStore()?.state || { vms: [], containers: [], nodes: [], pbs: [], lastUpdate: '' };
  const connected = () => wsStore()?.connected() || false;
  const reconnecting = () => wsStore()?.reconnecting() || false;

  // Data update indicator
  const [dataUpdated, setDataUpdated] = createSignal(false);
  let updateTimeout: number;

  // Flash indicator when data updates
  createEffect(() => {
    // Watch for state changes
    const updateTime = state().lastUpdate;
    if (updateTime && updateTime !== '') {
      setDataUpdated(true);
      window.clearTimeout(updateTimeout);
      updateTimeout = window.setTimeout(() => setDataUpdated(false), POLLING_INTERVALS.DATA_FLASH);
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
                  <div class="min-h-screen bg-gray-100 dark:bg-gray-900 text-gray-800 dark:text-gray-200 p-2 font-sans">
                    <div class="container w-[95%] max-w-screen-xl mx-auto">
                      <AppLayout
                        connected={connected}
                        reconnecting={reconnecting}
                        dataUpdated={dataUpdated}
                        versionInfo={versionInfo}
                        darkMode={darkMode}
                        toggleDarkMode={toggleDarkMode}
                        hasAuth={hasAuth}
                        needsAuth={needsAuth}
                        proxyAuthInfo={proxyAuthInfo}
                        handleLogout={handleLogout}
                        state={state}
                      >
                        {props.children}
                      </AppLayout>
                    </div>
                  </div>
                  <ToastContainer />
                  <NotificationContainer />
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
      <Route path="/" component={() => <Dashboard vms={state().vms} containers={state().containers} nodes={state().nodes} />} />
      <Route path="/storage" component={StorageComponent} />
      <Route path="/backups" component={Backups} />
      <Route path="/alerts" component={Alerts} />
      <Route path="/settings/*" component={Settings} />
    </Router>
  );
}

// Layout component that wraps all routes
function AppLayout(props: {
  connected: () => boolean;
  reconnecting: () => boolean;
  dataUpdated: () => boolean;
  versionInfo: () => VersionInfo | null;
  darkMode: () => boolean;
  toggleDarkMode: () => void;
  hasAuth: () => boolean;
  needsAuth: () => boolean;
  proxyAuthInfo: () => { username?: string; logoutURL?: string } | null;
  handleLogout: () => void;
  state: () => { vms: unknown[]; containers: unknown[]; nodes: unknown[] };
  children?: JSX.Element;
}) {
  const navigate = useNavigate();
  const location = useLocation();

  // Determine active tab from current path
  const getActiveTab = () => {
    const path = location.pathname;
    if (path === '/storage') return 'storage';
    if (path === '/backups') return 'backups';
    if (path === '/alerts') return 'alerts';
    if (path === '/settings') return 'settings';
    return 'main';
  };

  return (
    <>
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
          <button
            onClick={props.toggleDarkMode}
            class="p-2 rounded-md text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 focus:outline-none transition-colors"
            title={props.darkMode() ? 'Switch to light mode' : 'Switch to dark mode'}
          >
            <Show
              when={props.darkMode()}
              fallback={
                <svg
                  class="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"
                  />
                </svg>
              }
            >
              <svg
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"
                />
              </svg>
            </Show>
          </button>
          <div class="flex items-center gap-2">
            <div
              class={`status text-xs px-2 py-1 rounded-full flex items-center gap-1 ${
                props.connected()
                  ? 'connected bg-green-200 dark:bg-green-700 text-green-700 dark:text-green-300'
                  : props.reconnecting()
                    ? 'reconnecting bg-yellow-200 dark:bg-yellow-700 text-yellow-700 dark:text-yellow-300'
                    : 'disconnected bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
              }`}
            >
              <Show when={props.reconnecting()}>
                <svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
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
              {props.connected()
                ? 'Connected'
                : props.reconnecting()
                  ? 'Reconnecting...'
                  : 'Disconnected'}
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
                class="text-xs px-2 py-1 rounded-full bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors flex items-center gap-1"
                title="Logout"
              >
                <svg
                  class="h-3 w-3"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                  />
                </svg>
                <span>Logout</span>
              </button>
            </Show>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div
        class="tabs flex mb-2 border-b border-gray-300 dark:border-gray-700 overflow-x-auto overflow-y-hidden whitespace-nowrap scrollbar-hide"
        role="tablist"
      >
        <div
          class={`tab px-2 sm:px-3 py-1.5 cursor-pointer text-xs sm:text-sm rounded-t flex items-center gap-1 sm:gap-1.5 transition-colors ${
            getActiveTab() === 'main'
              ? 'active bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 border-b-0 -mb-px text-blue-600 dark:text-blue-500'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 border-transparent'
          }`}
          onClick={() => navigate('/')}
          role="tab"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"></path>
            <polyline points="9 22 9 12 15 12 15 22"></polyline>
          </svg>
          <span>Main</span>
        </div>
        <div
          class={`tab px-2 sm:px-3 py-1.5 cursor-pointer text-xs sm:text-sm rounded-t flex items-center gap-1 sm:gap-1.5 transition-colors ${
            getActiveTab() === 'storage'
              ? 'active bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 border-b-0 -mb-px text-blue-600 dark:text-blue-500'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 border-transparent'
          }`}
          onClick={() => navigate('/storage')}
          role="tab"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <ellipse cx="12" cy="5" rx="9" ry="3"></ellipse>
            <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"></path>
            <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"></path>
          </svg>
          <span>Storage</span>
        </div>
        <div
          class={`tab px-2 sm:px-3 py-1.5 cursor-pointer text-xs sm:text-sm rounded-t flex items-center gap-1 sm:gap-1.5 transition-colors ${
            getActiveTab() === 'backups'
              ? 'active bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 border-b-0 -mb-px text-blue-600 dark:text-blue-500'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 border-transparent'
          }`}
          onClick={() => navigate('/backups')}
          role="tab"
          title="PVE backups, PBS backups, and VM/CT snapshots"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
            <line x1="3" y1="9" x2="21" y2="9"></line>
            <line x1="9" y1="21" x2="9" y2="9"></line>
          </svg>
          <span>Backups</span>
        </div>
        <div
          class={`tab px-2 sm:px-3 py-1.5 cursor-pointer text-xs sm:text-sm rounded-t flex items-center gap-1 sm:gap-1.5 transition-colors ${
            getActiveTab() === 'alerts'
              ? 'active bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 border-b-0 -mb-px text-blue-600 dark:text-blue-500'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 border-transparent'
          }`}
          onClick={() => navigate('/alerts')}
          role="tab"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
            <line x1="12" y1="9" x2="12" y2="13"></line>
            <line x1="12" y1="17" x2="12.01" y2="17"></line>
          </svg>
          <span>Alerts</span>
        </div>
        <div
          class={`tab px-2 sm:px-3 py-1.5 cursor-pointer text-xs sm:text-sm rounded-t flex items-center gap-1 sm:gap-1.5 transition-colors relative ${
            getActiveTab() === 'settings'
              ? 'active bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 border-b-0 -mb-px text-blue-600 dark:text-blue-500'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 border-transparent'
          }`}
          onClick={() => navigate('/settings')}
          role="tab"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M12.22 2h-.44a2 2 0 00-2 2v.18a2 2 0 01-1 1.73l-.43.25a2 2 0 01-2 0l-.15-.08a2 2 0 00-2.73.73l-.22.38a2 2 0 00.73 2.73l.15.1a2 2 0 011 1.72v.51a2 2 0 01-1 1.74l-.15.09a2 2 0 00-.73 2.73l.22.38a2 2 0 002.73.73l.15-.08a2 2 0 012 0l.43.25a2 2 0 011 1.73V20a2 2 0 002 2h.44a2 2 0 002-2v-.18a2 2 0 011-1.73l.43-.25a2 2 0 012 0l.15.08a2 2 0 002.73-.73l.22-.39a2 2 0 00-.73-2.73l-.15-.08a2 2 0 01-1-1.74v-.5a2 2 0 011-1.74l.15-.09a2 2 0 00.73-2.73l-.22-.38a2 2 0 00-2.73-.73l-.15.08a2 2 0 01-2 0l-.43-.25a2 2 0 01-1-1.73V4a2 2 0 00-2-2z"></path>
            <circle cx="12" cy="12" r="3"></circle>
          </svg>
          <span>Settings</span>
          <Show when={updateStore.isUpdateVisible()}>
            <span class="absolute -top-1 -right-1 w-2 h-2 bg-red-500 rounded-full animate-pulse"></span>
          </Show>
        </div>
      </div>

      {/* Main Content */}
      <main
        id="main"
        class="tab-content block bg-white dark:bg-gray-800 rounded-b rounded-tr shadow mb-2"
      >
        <div class="p-3">
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
      </footer>
    </>
  );
}

export default App; // Test hot-reload comment $(date)
