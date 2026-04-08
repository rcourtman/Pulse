import {
  Show,
  lazy,
  createSignal,
  createEffect,
  createMemo,
  onCleanup,
  onMount,
} from 'solid-js';
import type { JSX } from 'solid-js';
import { Router, Route, Navigate, useNavigate, useLocation } from '@solidjs/router';
import { ToastContainer } from './components/Toast/Toast';
import { ErrorBoundary } from './components/ErrorBoundary';
import { SecurityWarning } from './components/SecurityWarning';
import { Login } from './components/Login';
import { logger } from './utils/logger';
import { UpdateBanner } from './components/UpdateBanner';
import { DemoBanner } from './components/DemoBanner';
import { GitHubStarBanner } from './components/GitHubStarBanner';
import { TrialBanner } from './components/shared/TrialBanner';
import { MonitoredSystemLimitWarningBanner } from './components/shared/MonitoredSystemLimitWarningBanner';
import { WhatsNewModal } from './components/shared/WhatsNewModal';
import { KeyboardShortcutsModal } from './components/shared/KeyboardShortcutsModal';
import { CommandPaletteModal } from './components/shared/CommandPaletteModal';
import { createTooltipSystem } from './components/shared/Tooltip';
import { TokenRevealDialog } from './components/TokenRevealDialog';
import { UpdateProgressModal } from './components/UpdateProgressModal';
import { UpdatesAPI, type UpdateStatus } from './api/updates';
import { AIChat } from './components/AI/Chat';
import { aiChatStore } from './stores/aiChat';
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts';
import { useKioskMode } from '@/hooks/useKioskMode';
import {
  DASHBOARD_PATH,
  buildRecoveryPath,
  buildInfrastructurePath,
  buildStoragePath,
  buildWorkloadsPath,
} from './routing/resourceLinks';
import { AppLayout } from '@/AppLayout';
import { useAppRuntimeState } from '@/useAppRuntimeState';
import {
  clearPendingAppShellRestoreTop,
  readPendingAppShellRestoreTop,
} from '@/utils/appShellScrollRestoration';
import { DarkModeContext, WebSocketContext, useWebSocket } from '@/contexts/appRuntime';

function isPublicRoutePath(pathname: string): boolean {
  // Public routes must be viewable without authentication.
  // Keep the list small and explicit.
  return (
    pathname === '/pricing' ||
    pathname === '/cloud' ||
    pathname === '/cloud/signup' ||
    pathname === '/preview/setup-complete'
  );
}

const Dashboard = lazy(() =>
  import('./components/Dashboard/Dashboard').then((module) => ({ default: module.Dashboard })),
);
const StoragePage = lazy(() => import('./pages/Storage'));
const RecoveryRoute = lazy(() => import('./pages/RecoveryRoute'));
const CephPage = lazy(() => import('./pages/Ceph'));
const AlertsPage = lazy(() =>
  import('./pages/Alerts').then((module) => ({ default: module.Alerts })),
);
const SettingsPage = lazy(() => import('./components/Settings/Settings'));
const InfrastructurePage = lazy(() => import('./pages/Infrastructure'));
const DashboardPage = lazy(() => import('./pages/Dashboard'));
const AIIntelligencePage = lazy(() =>
  import('./pages/AIIntelligence').then((module) => ({ default: module.AIIntelligence })),
);
const NotFoundPage = lazy(() => import('./pages/NotFound'));
const PricingHandoffPage = lazy(() => import('./pages/PricingHandoff'));
const CloudPricingPage = lazy(() => import('./pages/CloudPricing'));
const HostedSignupPage = lazy(() => import('./pages/HostedSignup'));
const OperationsPage = lazy(() => import('./pages/Operations'));
const SetupCompletionPreviewPage = lazy(() =>
  import('./components/SetupWizard/SetupCompletionPreview').then((module) => ({
    default: module.SetupCompletionPreview,
  })),
);
const ROOT_DASHBOARD_PATH = DASHBOARD_PATH;
const INFRASTRUCTURE_ROUTE_PATH = buildInfrastructurePath();
const ROOT_WORKLOADS_PATH = buildWorkloadsPath();
const STORAGE_PATH = buildStoragePath();
const RECOVERY_ROUTE_PATH = buildRecoveryPath();

// Helper to detect if an update is actively in progress (not just checking for updates)
function isUpdateInProgress(status: string | undefined): boolean {
  if (!status) return false;
  const inProgressStates = ['downloading', 'verifying', 'extracting', 'installing', 'restarting'];
  return inProgressStates.includes(status);
}

// Global update progress watcher - shows modal in ALL tabs when an update is running
function GlobalUpdateProgressWatcher() {
  const wsContext = useWebSocket();
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
        navigate('/settings/system-updates');
      }}
      connected={wsContext?.connected}
      reconnecting={wsContext?.reconnecting}
    />
  );
}

function App() {
  const LegacyOperationsSettingsRedirect = () => {
    const location = useLocation();
    const canonicalPath =
      location.pathname.replace(/^\/settings\/operations(?=\/|$)/, '/operations') || '/operations';
    return <Navigate href={`${canonicalPath}${location.search ?? ''}`} />;
  };
  const kioskMode = useKioskMode();
  const TooltipRoot = createTooltipSystem();
  const runtime = useAppRuntimeState();

  const WorkloadsView = () => <Dashboard vms={[]} containers={[]} nodes={[]} useWorkloads />;

  // V2 is GA - always serve V2 at canonical routes.

  const SettingsRoute = () => (
    <SettingsPage
      darkMode={runtime.darkMode}
      themePreference={runtime.themePreference}
      setThemePreference={runtime.handleThemeChange}
    />
  );

  // Root layout component for Router
  const RootLayout = (props: { children?: JSX.Element }) => {
    const [shortcutsOpen, setShortcutsOpen] = createSignal(false);
    const [commandPaletteOpen, setCommandPaletteOpen] = createSignal(false);
    const [appScrollShellRef, setAppScrollShellRef] = createSignal<HTMLDivElement | undefined>(
      undefined,
    );
    const [pendingAppShellRestoreTop, setPendingAppShellRestoreTop] = createSignal<number | null>(
      null,
    );
    const location = useLocation();
    const isPublicRoute = createMemo(() => isPublicRoutePath(location.pathname));

    createEffect(() => {
      location.pathname;
      location.search;
      const pendingRestoreTop = readPendingAppShellRestoreTop();
      if (pendingRestoreTop !== null) {
        setPendingAppShellRestoreTop(pendingRestoreTop);
      }
    });

    createEffect(() => {
      const shell = appScrollShellRef();
      const restoreTop = pendingAppShellRestoreTop();
      if (!shell || restoreTop === null) {
        return;
      }
      if (Math.abs(shell.scrollTop - restoreTop) <= 2) {
        clearPendingAppShellRestoreTop();
        setPendingAppShellRestoreTop(null);
        return;
      }

      let settled = false;
      let rafId: number | undefined;
      let timeoutId: number | undefined;

      const finish = () => {
        if (settled) return;
        settled = true;
        if (rafId !== undefined) {
          window.cancelAnimationFrame(rafId);
        }
        if (timeoutId !== undefined) {
          window.clearTimeout(timeoutId);
        }
      };

      const attemptRestore = (remainingFrames: number) => {
        if (settled) return;
        const maxScrollTop = Math.max(0, shell.scrollHeight - shell.clientHeight);
        if (restoreTop <= maxScrollTop) {
          shell.scrollTop = restoreTop;
          if (Math.abs(shell.scrollTop - restoreTop) <= 2) {
            clearPendingAppShellRestoreTop();
            setPendingAppShellRestoreTop(null);
            finish();
            return;
          }
        }
        if (remainingFrames <= 0) {
          clearPendingAppShellRestoreTop();
          setPendingAppShellRestoreTop(null);
          finish();
          return;
        }
        rafId = window.requestAnimationFrame(() => attemptRestore(remainingFrames - 1));
      };

      rafId = window.requestAnimationFrame(() => attemptRestore(90));
      timeoutId = window.setTimeout(() => {
        const maxScrollTop = Math.max(0, shell.scrollHeight - shell.clientHeight);
        shell.scrollTop = Math.min(restoreTop, maxScrollTop);
        clearPendingAppShellRestoreTop();
        setPendingAppShellRestoreTop(null);
        finish();
      }, 1500);

      onCleanup(finish);
    });

    useKeyboardShortcuts({
      enabled: () => !runtime.needsAuth(),
      isShortcutsOpen: shortcutsOpen,
      isCommandPaletteOpen: commandPaletteOpen,
      onToggleShortcuts: () => {
        setCommandPaletteOpen(false);
        setShortcutsOpen((prev) => !prev);
      },
      onCloseShortcuts: () => setShortcutsOpen(false),
      onToggleCommandPalette: () => {
        setShortcutsOpen(false);
        setCommandPaletteOpen((prev) => !prev);
      },
      onCloseCommandPalette: () => setCommandPaletteOpen(false),
    });

    // Setup escape handling for the assistant drawer.
    onMount(() => {
      const handleKeyDown = (e: KeyboardEvent) => {
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
        when={!runtime.isLoading()}
        fallback={
          <div class="min-h-screen flex items-center justify-center bg-base">
            <div class="text-muted">Loading...</div>
          </div>
        }
      >
        <Show
          when={isPublicRoute()}
          fallback={
            <Show
              when={!runtime.needsAuth()}
              fallback={
                <Login
                  onLogin={runtime.handleLogin}
                  hasAuth={runtime.hasAuth()}
                  securityStatus={runtime.securityStatus() ?? undefined}
                />
              }
            >
              <ErrorBoundary>
                <Show
                  when={runtime.enhancedStore()}
                  fallback={
                    <div class="min-h-screen flex items-center justify-center bg-base">
                      <div class="text-muted">Initializing...</div>
                    </div>
                  }
                >
                  <WebSocketContext.Provider value={runtime.enhancedStore()!}>
                    <DarkModeContext.Provider value={runtime.darkMode}>
                      <Show when={!kioskMode()}>
                        <SecurityWarning />
                        <DemoBanner />
                        <UpdateBanner />
                        <TrialBanner />
                        <MonitoredSystemLimitWarningBanner />
                        <GitHubStarBanner />
                        <WhatsNewModal />
                        <GlobalUpdateProgressWatcher />
                      </Show>
                      {/* Main layout container - flexbox to allow AI panel to push content */}
                      <div class="flex h-screen overflow-hidden">
                        {/* Main content area - shrinks when AI panel is open, scrolls independently */}
                        <div
                          ref={setAppScrollShellRef}
                          class={`app-scroll-shell flex-1 min-w-0 overflow-y-scroll bg-base text-base-content font-sans py-4 sm:py-6 transition-all duration-300`}
                        >
      <AppLayout
        connectionStatus={runtime.connectionStatus}
        dataUpdated={runtime.dataUpdated}
        lastUpdateText={runtime.lastUpdateText}
        versionInfo={runtime.versionInfo}
                            hasAuth={runtime.hasAuth}
                            needsAuth={runtime.needsAuth}
                            proxyAuthInfo={runtime.proxyAuthInfo}
                            handleLogout={runtime.handleLogout}
                            state={runtime.state}
                            tokenScopes={() => runtime.securityStatus()?.tokenScopes}
                            organizations={runtime.organizations}
                            activeOrgID={runtime.activeOrgID}
                            orgsLoading={runtime.orgsLoading}
                            onSwitchOrg={runtime.handleOrgSwitch}
                          >
                            {props.children}
                          </AppLayout>
                        </div>
                        {/* AI Panel - slides in from right, pushes content */}
                        <AIChat onClose={() => aiChatStore.close()} />
                      </div>
                      <KeyboardShortcutsModal
                        isOpen={shortcutsOpen()}
                        onClose={() => setShortcutsOpen(false)}
                      />
                      <CommandPaletteModal
                        isOpen={commandPaletteOpen()}
                        onClose={() => setCommandPaletteOpen(false)}
                      />
                      <TokenRevealDialog />
                      {/* AI Assistant Button moved to AppLayout to access kioskMode state */}
                      <TooltipRoot />
                    </DarkModeContext.Provider>
                  </WebSocketContext.Provider>
                </Show>
              </ErrorBoundary>
            </Show>
          }
        >
          <div class="min-h-screen bg-base text-base-content font-sans">
            <div class="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6">{props.children}</div>
          </div>
        </Show>
        <ToastContainer />
      </Show>
    );
  };

  // Use Router with routes
  return (
    <Router root={RootLayout}>
      <Route path="/pricing" component={PricingHandoffPage} />
      <Route path="/cloud" component={CloudPricingPage} />
      <Route path="/cloud/signup" component={HostedSignupPage} />
      <Route path="/preview/setup-complete" component={SetupCompletionPreviewPage} />
      <Route path={ROOT_DASHBOARD_PATH} component={DashboardPage} />
      <Route path="/" component={() => <Navigate href={ROOT_DASHBOARD_PATH} />} />
      <Route path={ROOT_WORKLOADS_PATH} component={WorkloadsView} />
      <Route path={STORAGE_PATH} component={StoragePage} />
      <Route path={RECOVERY_ROUTE_PATH} component={RecoveryRoute} />
      <Route path="/ceph" component={CephPage} />
      <Route path={INFRASTRUCTURE_ROUTE_PATH} component={InfrastructurePage} />

      <Route path="/alerts/*" component={AlertsPage} />
      <Route path="/ai/*" component={AIIntelligencePage} />
      <Route path="/settings/operations/*" component={LegacyOperationsSettingsRedirect} />
      <Route path="/settings/*" component={SettingsRoute} />
      <Route path="/operations/*" component={OperationsPage} />
      <Route path="*all" component={NotFoundPage} />
    </Router>
  );
}

export default App; // Test hot-reload comment $(date)
