import { Show, lazy, createSignal, createEffect, createMemo, onCleanup, onMount } from 'solid-js';
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
// Modals are only mounted when opened, so their code can stay out of the
// entry bundle until first use (same pattern as AIChat below).
const KeyboardShortcutsModal = lazy(() => import('./components/shared/KeyboardShortcutsModal').then((m) => ({ default: m.KeyboardShortcutsModal })));
const CommandPaletteModal = lazy(() => import('./components/shared/CommandPaletteModal').then((m) => ({ default: m.CommandPaletteModal })));
import { dialogStackHasBlockingDialog } from './components/shared/useDialogState';
import { createTooltipSystem } from './components/shared/Tooltip';
const TokenRevealDialog = lazy(() => import('./components/TokenRevealDialog').then((m) => ({ default: m.TokenRevealDialog })));
import { tokenRevealStore } from './stores/tokenReveal';
const UpdateProgressModal = lazy(() => import('./components/UpdateProgressModal').then((m) => ({ default: m.UpdateProgressModal })));
import { UpdatesAPI, type UpdateStatus } from './api/updates';
// AIChat is the side-panel chat UI plus its store deps (markdown rendering,
// tool-call formatting, prompt scaffolding). Lazy-load behind aiChatStore.isOpen
// so the entry bundle stays out of "everything users might ever click."
const AIChat = lazy(() => import('./components/AI/Chat').then((m) => ({ default: m.AIChat })));
import { aiChatStore } from './stores/aiChat';
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts';
import { useKioskMode } from '@/hooks/useKioskMode';
import {
  PATROL_PATH,
  buildRecoveryPath,
  buildInfrastructurePath,
  buildStoragePath,
  buildWorkloadsPath,
} from './routing/resourceLinks';
import { APP_SHELL_ROUTE_PRELOAD_PATHS, preloadRouteModule } from '@/routing/routePreload';
import { AppLayout } from '@/AppLayout';
import RuntimeHomePage from '@/pages/RuntimeHome';
import { useAppRuntimeState } from '@/useAppRuntimeState';
import {
  clearPendingAppShellRestoreTop,
  readPendingAppShellRestoreTop,
} from '@/utils/appShellScrollRestoration';
import { DarkModeContext, WebSocketContext, useWebSocket } from '@/contexts/appRuntime';

function isPublicRoutePath(pathname: string): boolean {
  // Public routes must be viewable without authentication.
  // Keep the list small and explicit.
  return pathname === '/pricing' || pathname === '/preview/setup-complete';
}

const StoragePage = lazy(() => import('./pages/Storage'));
const RecoveryPage = lazy(() => import('./pages/Recovery'));
const CephPage = lazy(() => import('./pages/Ceph'));
const AlertsPage = lazy(() =>
  import('./pages/Alerts').then((module) => ({ default: module.Alerts })),
);
const SettingsPage = lazy(() => import('./components/Settings/Settings'));
const InfrastructurePage = lazy(() => import('./pages/Infrastructure'));
const WorkloadsPage = lazy(() => import('./pages/Workloads'));
const AIIntelligencePage = lazy(() =>
  import('./pages/AIIntelligence').then((module) => ({ default: module.AIIntelligence })),
);
const NotFoundPage = lazy(() => import('./pages/NotFound'));
const PricingHandoffPage = lazy(() => import('./pages/PricingHandoff'));
const OperationsPage = lazy(() => import('./pages/Operations'));
const SetupCompletionPreviewPage = lazy(() =>
  import('./components/SetupWizard/SetupCompletionPreview').then((module) => ({
    default: module.SetupCompletionPreview,
  })),
);
const INFRASTRUCTURE_ROUTE_PATH = buildInfrastructurePath();
const ROOT_WORKLOADS_PATH = buildWorkloadsPath();
const STORAGE_PATH = buildStoragePath();
const RECOVERY_ROUTE_PATH = buildRecoveryPath();
const ROOT_PATROL_PATH = PATROL_PATH;

async function preloadAppShellRoutes() {
  await Promise.all(
    APP_SHELL_ROUTE_PRELOAD_PATHS.map(async (route) => {
      try {
        await preloadRouteModule(route);
      } catch (error) {
        logger.warn('Failed to preload app shell route', {
          route,
          error: error instanceof Error ? error.message : String(error),
        });
      }
    }),
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
    <Show when={showProgressModal()}>
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
    </Show>
  );
}

function App() {
  const LegacyPatrolRouteRedirect = () => {
    const location = useLocation();
    const canonicalPath =
      location.pathname.replace(/^\/ai(?=\/|$)/, ROOT_PATROL_PATH) || ROOT_PATROL_PATH;
    return <Navigate href={`${canonicalPath}${location.search ?? ''}`} />;
  };
  const kioskMode = useKioskMode();
  const TooltipRoot = createTooltipSystem();
  const runtime = useAppRuntimeState();

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
    let appShellRoutePreloadCleanup: (() => void) | undefined;
    let appShellRoutesPreloadScheduled = false;

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

    createEffect(() => {
      if (
        runtime.isLoading() ||
        runtime.needsAuth() ||
        isPublicRoute() ||
        appShellRoutesPreloadScheduled
      ) {
        return;
      }

      appShellRoutesPreloadScheduled = true;
      const startPreload = () => {
        appShellRoutePreloadCleanup = undefined;
        void preloadAppShellRoutes();
      };

      const timeoutId = window.setTimeout(() => {
        startPreload();
      }, 150);
      appShellRoutePreloadCleanup = () => window.clearTimeout(timeoutId);
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

    createEffect(() => {
      if (dialogStackHasBlockingDialog() && aiChatStore.isOpenSignal()) {
        aiChatStore.close();
      }
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

    onCleanup(() => {
      appShellRoutePreloadCleanup?.();
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
                        <GitHubStarBanner />
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
                            showOrgSwitcher={runtime.showOrgSwitcher}
                            onSwitchOrg={runtime.handleOrgSwitch}
                          >
                            {props.children}
                          </AppLayout>
                        </div>
                        {/* AI Panel - slides in from right, pushes content.
                            Mounted only when open so the lazy chunk only
                            downloads on first chat-open click. */}
                        <Show when={aiChatStore.isOpenSignal()}>
                          <AIChat onClose={() => aiChatStore.close()} />
                        </Show>
                      </div>
                      <Show when={shortcutsOpen()}>
                        <KeyboardShortcutsModal
                          isOpen={shortcutsOpen()}
                          onClose={() => setShortcutsOpen(false)}
                        />
                      </Show>
                      <Show when={commandPaletteOpen()}>
                        <CommandPaletteModal
                          isOpen={commandPaletteOpen()}
                          onClose={() => setCommandPaletteOpen(false)}
                        />
                      </Show>
                      <Show when={tokenRevealStore.state() !== null}>
                        <TokenRevealDialog />
                      </Show>
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
      <Route path="/preview/setup-complete" component={SetupCompletionPreviewPage} />
      <Route path="/login" component={RuntimeHomePage} />
      <Route path="/" component={RuntimeHomePage} />
      <Route path={ROOT_WORKLOADS_PATH} component={WorkloadsPage} />
      <Route path={STORAGE_PATH} component={StoragePage} />
      <Route path={RECOVERY_ROUTE_PATH} component={RecoveryPage} />
      <Route path="/ceph" component={CephPage} />
      <Route path={INFRASTRUCTURE_ROUTE_PATH} component={InfrastructurePage} />

      <Route path="/alerts/*" component={AlertsPage} />
      <Route path={`${ROOT_PATROL_PATH}/*`} component={AIIntelligencePage} />
      <Route path="/ai/*" component={LegacyPatrolRouteRedirect} />
      <Route path="/settings/*" component={SettingsRoute} />
      <Route path="/operations/*" component={OperationsPage} />
      <Route path="*all" component={NotFoundPage} />
    </Router>
  );
}

export default App; // Test hot-reload comment $(date)
