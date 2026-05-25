import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import appSource from '@/App.tsx?raw';
import appLayoutSource from '@/AppLayout.tsx?raw';
import appRuntimeContextSource from '@/contexts/appRuntime.ts?raw';
import routePreloadSource from '@/routing/routePreload.ts?raw';
import appRuntimeStateSource from '@/useAppRuntimeState.ts?raw';

const appStylesSource = readFileSync(join(process.cwd(), 'src/index.css'), 'utf8');
const headerAuditSource = readFileSync(join(process.cwd(), 'scripts/header-audit.mjs'), 'utf8');

describe('App architecture', () => {
  it('keeps App as the entry shell that delegates runtime and chrome ownership', () => {
    expect(appSource).toContain("import { AppLayout } from '@/AppLayout';");
    expect(appSource).toContain("import { aiChatStore } from './stores/aiChat';");
    expect(appSource).toContain(
      "import { DarkModeContext, WebSocketContext, useWebSocket } from '@/contexts/appRuntime';",
    );
    expect(appSource).toContain("import { useAppRuntimeState } from '@/useAppRuntimeState';");
    expect(appSource).toContain(
      "import { dialogStackHasBlockingDialog } from './components/shared/useDialogState';",
    );
    expect(appSource).toContain('import {');
    expect(appSource).toContain("} from '@/utils/appShellScrollRestoration';");
    expect(appSource).toContain('const runtime = useAppRuntimeState();');
    expect(appSource).toContain('pendingAppShellRestoreTop');
    expect(appSource).toContain('setAppScrollShellRef');
    expect(appSource).toContain('readPendingAppShellRestoreTop');
    expect(appSource).toContain('clearPendingAppShellRestoreTop');
    expect(appSource).toContain("const ProxmoxPage = lazy(() => import('./pages/Proxmox'));");
    expect(appSource).toContain('const ROOT_PATROL_PATH = PATROL_PATH;');
    expect(appSource).toContain(
      "import { APP_SHELL_ROUTE_PRELOAD_PATHS, preloadRouteModule } from '@/routing/routePreload';",
    );
    expect(routePreloadSource).toContain('export const APP_SHELL_ROUTE_PRELOAD_PATHS = [');
    expect(routePreloadSource).toContain('ROOT_PROXMOX_PATH,');
    expect(routePreloadSource).toContain('PATROL_PATH,');
    // These aggregate pages never shipped as stable top-level routes, so they
    // must not remain wired as hidden compatibility or shell workspace surfaces.
    expect(routePreloadSource).not.toContain('ROOT_INFRASTRUCTURE_PATH');
    expect(routePreloadSource).not.toContain("id: 'workloads',");
    expect(routePreloadSource).not.toContain("id: 'storage',");
    expect(routePreloadSource).not.toContain("id: 'recovery',");
    expect(appSource).not.toContain('const InfrastructurePage = lazy(');
    expect(appSource).not.toContain('const WorkloadsPage = lazy(');
    expect(appSource).not.toContain('const StoragePage = lazy(');
    expect(appSource).not.toContain('const RecoveryPage = lazy(');
    expect(appSource).not.toContain(
      "import('./features/infrastructure/InfrastructurePageSurface')",
    );
    expect(appSource).not.toContain('component={InfrastructurePage}');
    expect(appSource).not.toContain('INFRASTRUCTURE_PATH');
    expect(appSource).not.toContain('<Route path={WORKLOADS_PATH}');
    expect(appSource).not.toContain('<Route path={STORAGE_PATH}');
    expect(appSource).not.toContain('<Route path={RECOVERY_PATH}');
    expect(appSource).not.toContain('<Route path="/ceph"');
    expect(appSource).not.toContain('<Route path="/ceph/*"');
    expect(appSource).not.toContain('Route, Navigate,');
    expect(appSource).not.toContain('<Navigate ');
    expect(appSource).toContain('await preloadRouteModule(route);');
    expect(appRuntimeStateSource).not.toContain('preloadLazyRoutes');
    expect(appRuntimeStateSource).not.toContain("import('@/pages/Alerts')");
    expect(appRuntimeStateSource).not.toContain("import('@/components/Settings/Settings')");
    expect(appSource).toContain('const timeoutId = window.setTimeout(() => {');
    expect(appSource).toContain('void preloadAppShellRoutes();');
    expect(appRuntimeStateSource).not.toContain('fetchInfrastructureSummaryAndCache');
    expect(appRuntimeStateSource).not.toContain('fetchWorkloadsSummaryAndCache');
    expect(appRuntimeStateSource).not.toContain('requestIdleCallback');
    expect(appSource).toContain("const StandalonePage = lazy(() => import('./pages/Standalone'));");
    expect(appSource).toContain('<Route path={STANDALONE_PATH} component={StandalonePage} />');
    expect(appSource).toContain(
      '<Route path={`${STANDALONE_PATH}/*`} component={StandalonePage} />',
    );
    expect(appSource).not.toContain("import('./pages/Agents')");
    expect(appSource).not.toContain('AGENTS_PATH');
    expect(appSource).toContain('<Route path={PROXMOX_PATH} component={ProxmoxPage} />');
    expect(appSource).toContain('<Route path={`${PROXMOX_PATH}/*`} component={ProxmoxPage} />');
    expect(appSource).toContain("const DockerPage = lazy(() => import('./pages/Docker'));");
    expect(appSource).toContain("const KubernetesPage = lazy(() => import('./pages/Kubernetes'));");
    expect(appSource).toContain("const TrueNASPage = lazy(() => import('./pages/TrueNAS'));");
    expect(appSource).toContain("const VmwarePage = lazy(() => import('./pages/Vmware'));");
    expect(appSource).toContain('<Route path={DOCKER_PATH} component={DockerPage} />');
    expect(appSource).toContain('<Route path={KUBERNETES_PATH} component={KubernetesPage} />');
    expect(appSource).toContain('<Route path={TRUENAS_PATH} component={TrueNASPage} />');
    expect(appSource).toContain('<Route path={VMWARE_PATH} component={VmwarePage} />');
    expect(routePreloadSource).toContain("id: 'standalone',");
    expect(routePreloadSource).toContain("id: 'docker',");
    expect(routePreloadSource).toContain("id: 'kubernetes',");
    expect(routePreloadSource).toContain("id: 'truenas',");
    expect(routePreloadSource).toContain("id: 'vmware',");
    expect(routePreloadSource.indexOf("id: 'proxmox',")).toBeLessThan(
      routePreloadSource.indexOf("id: 'standalone',"),
    );
    expect(appLayoutSource).toContain("id: 'standalone',");
    expect(appLayoutSource).toContain("id: 'docker',");
    expect(appLayoutSource).toContain("id: 'kubernetes',");
    expect(appLayoutSource).toContain("id: 'truenas',");
    expect(appLayoutSource).toContain("id: 'vmware',");
    expect(appLayoutSource).toContain(
      "tooltip: 'VMware vSphere hosts, virtual machines, datastores, and networks'",
    );
    expect(appLayoutSource).toContain(
      "tooltip: 'Pulse Agent machines, agentless computers, and availability checks'",
    );
    // Governed shell nav: Infrastructure, Workloads, Storage, and Recovery are
    // not standalone shell tabs; platform/runtime pages own those workflows.
    expect(appSource).toContain('getDefaultWorkspaceRoute');
    expect(appSource).toContain('infrastructureNavigationResolved');
    expect(appSource).toContain('buildPrimaryInfrastructureNavigationVisibility');
    expect(appLayoutSource).toContain('buildPrimaryInfrastructureNavigationVisibility');
    expect(appLayoutSource).toContain('primaryInfrastructureNavigationIsVisible');
    expect(appLayoutSource).toContain("label: 'Containers'");
    expect(appLayoutSource).toContain("'Docker / Podman runtime lens");
    expect(appLayoutSource).not.toContain("id: 'infrastructure',");
    expect(appLayoutSource).not.toContain("id: 'workloads',");
    expect(appLayoutSource).not.toContain("id: 'storage',");
    expect(appLayoutSource).not.toContain("id: 'recovery',");
    expect(appLayoutSource).not.toContain('aria-label="Workspaces"');
    expect(appLayoutSource).not.toContain('buildStorageRecoveryTabSpecs(');
    expect(appSource).not.toContain('DashboardPage');
    expect(headerAuditSource).not.toContain("['src/pages/Dashboard.tsx', 'PageHeader']");
    expect(appSource).toContain("import RuntimeHomePage from '@/pages/RuntimeHome';");
    expect(appSource).toContain('<Route path="/login" component={RuntimeHomePage} />');
    expect(appSource).toContain('<Route path="/" component={RuntimeHomePage} />');
    expect(appSource).toContain(
      '<Route path={`${ROOT_PATROL_PATH}/*`} component={AIIntelligencePage} />',
    );
    expect(appSource).not.toContain('LegacyPatrolRouteRedirect');
    expect(appSource).not.toContain('<Route path="/ai/*"');
    expect(appSource).toContain(
      "const PricingHandoffPage = lazy(() => import('./pages/PricingHandoff'));",
    );
    expect(appSource).toContain('<Route path="/pricing" component={PricingHandoffPage} />');
    expect(appSource).not.toContain(
      "const CloudPricingPage = lazy(() => import('./pages/CloudPricing'));",
    );
    expect(appSource).not.toContain(
      "const HostedSignupPage = lazy(() => import('./pages/HostedSignup'));",
    );
    expect(appSource).not.toContain('<Route path="/cloud" component=');
    expect(appSource).not.toContain('<Route path="/cloud/signup" component=');
    expect(appSource).toContain("const ProxmoxPage = lazy(() => import('./pages/Proxmox'));");
    expect(appSource).not.toContain("import('./pages/Operations')");
    expect(appSource).not.toContain('<Route path="/operations/*"');
    // Legacy page wrappers were deleted when primary nav moved to
    // platform-first; their tables are reused inside platform pages directly.
    expect(appSource).not.toContain("import('./pages/Storage')");
    expect(appSource).not.toContain("import('./pages/Workloads')");
    expect(appSource).not.toContain("import('./pages/Recovery')");
    expect(appSource).not.toContain("import('./pages/Infrastructure')");
    expect(appSource).not.toContain("import('./pages/Ceph')");
    expect(appSource).not.toContain(
      "const StorageComponent = lazy(() => import('./components/Storage/Storage'));",
    );
    expect(appSource).not.toContain(
      "const WorkloadsView = lazy(() => import('./components/Workloads/WorkloadsSurface'));",
    );
    expect(appSource).not.toContain(
      "const RecoveryRoute = lazy(() => import('./pages/RecoveryRoute'));",
    );
    expect(appSource).not.toContain("const PricingPage = lazy(() => import('./pages/PricingV6'));");
    expect(appSource).not.toContain('function ConnectionStatusBadge(');
    expect(appSource).not.toContain('function AppLayout(');
    expect(appSource).not.toContain('export const WebSocketContext = createContext<');
    expect(appSource).not.toContain('export const DarkModeContext = createContext<');
    expect(appSource).not.toContain('const [organizations, setOrganizations] = createSignal(');
    expect(appSource).not.toContain('const [themePreference, setThemePreference] =');
    expect(appSource).not.toContain('const [activeOrgID, setActiveOrgID] = createSignal(');
    expect(appSource).not.toContain("import('./api/ai')");
    expect(appSource).not.toContain('AIAPI.getSettings()');
    expect(appSource).toContain(
      'if (dialogStackHasBlockingDialog() && aiChatStore.isOpenSignal()) {',
    );
    expect(appSource).toContain("if (e.key === 'Escape' && aiChatStore.isOpen) {");
    expect(appSource).toContain('<AIChat onClose={() => aiChatStore.close()} />');
    expect(appSource).toContain('showOrgSwitcher={runtime.showOrgSwitcher}');
    expect(appSource).not.toContain('TrialBanner');
    expect(appSource).not.toContain('MonitoredSystemLimitWarningBanner');
    expect(appSource).not.toContain('monitoredSystemLimitWarningBanner');
  });

  it('keeps authenticated chrome in AppLayout and hosted bootstrap in useAppRuntimeState', () => {
    expect(appLayoutSource).toContain('export function AppLayout(props: AppLayoutProps)');
    expect(appLayoutSource).toContain(
      "import { preloadRouteModule } from '@/routing/routePreload';",
    );
    expect(appLayoutSource).toContain("import { aiChatStore } from '@/stores/aiChat';");
    expect(appLayoutSource).toContain(
      "import { dialogStackHasBlockingDialog } from '@/components/shared/useDialogState';",
    );
    expect(appLayoutSource).toContain(
      "import { buildInfrastructureWorkspacePath } from '@/components/Settings/infrastructureWorkspaceModel';",
    );
    expect(appLayoutSource).toContain(
      'const ROOT_INFRASTRUCTURE_SETTINGS_PATH = buildInfrastructureWorkspacePath();',
    );
    expect(appLayoutSource).toContain('settingsRoute: ROOT_INFRASTRUCTURE_SETTINGS_PATH');
    expect(appLayoutSource).not.toContain("settingsRoute: '/settings/workloads");
    expect(appLayoutSource).not.toContain("settingsRoute: '/settings/infrastructure/platforms");
    expect(appLayoutSource).toContain('<OrgSwitcher');
    expect(appLayoutSource).toContain('const status = () => props.connectionStatus();');
    expect(appLayoutSource).toContain(
      "status().kind === 'sync-reconnecting' || status().kind === 'reconnecting'",
    );
    expect(appLayoutSource).toContain("props.connectionStatus().tone === 'healthy'");
    expect(appLayoutSource).toContain('const brandMotionActive = createMemo(');
    expect(appLayoutSource).toContain('pulse-brand-lockup');
    expect(appLayoutSource).toContain('animate-pulse-brand');
    expect(appLayoutSource).toContain('pulse-brand-wordmark');
    expect(appLayoutSource).not.toContain('dataUpdated: () => boolean');
    expect(appLayoutSource).not.toContain('animate-pulse-logo');
    expect(appRuntimeStateSource).not.toContain('dataUpdated');
    expect(appRuntimeStateSource).not.toContain('DATA_FLASH');
    expect(appStylesSource).toContain('--pulse-brand-cycle: 3.4s;');
    expect(appStylesSource).toContain('@keyframes pulse-brand-wave');
    expect(appStylesSource).toContain(
      'animation: pulse-brand-wave var(--pulse-brand-cycle) ease-in-out infinite;',
    );
    expect(appStylesSource).toContain(
      'animation: pulse-brand-bg var(--pulse-brand-cycle) ease-in-out infinite;',
    );
    expect(appStylesSource).toContain(
      'animation: pulse-brand-logo var(--pulse-brand-cycle) ease-in-out infinite;',
    );
    expect(appStylesSource).toContain(
      'animation: pulse-brand-ring var(--pulse-brand-cycle) ease-in-out infinite;',
    );
    expect(appStylesSource).toContain('tr.grouped-table-row > td');
    expect(appStylesSource).toContain('--color-grouped-table-row-bg');
    expect(appStylesSource).toContain('--color-grouped-table-row-bg: rgba(226, 232, 240, 0.72);');
    expect(appStylesSource).toContain('--color-grouped-table-row-bg: rgba(51, 65, 85, 0.58);');
    expect(appStylesSource).toContain('.progress-fill-frame');
    expect(appStylesSource).toContain('.metric-fill-geometry');
    expect(appStylesSource).toContain('.animated-number');
    expect(appStylesSource).toContain('@media (prefers-reduced-motion: reduce)');
    expect(appStylesSource).not.toContain('--color-grouped-table-row-bg: theme(');
    expect(appStylesSource).not.toContain('@keyframes pulse-brand-wordmark');
    expect(appStylesSource).not.toContain('text-shadow');
    expect(appLayoutSource).toContain("props.versionInfo()?.channel === 'rc'");
    expect(appLayoutSource).toContain('Preview');
    expect(appLayoutSource).not.toContain(
      "import { ReleaseCandidateBanner } from '@/components/shared/ReleaseCandidateBanner';",
    );
    expect(appLayoutSource).not.toContain(
      '<ReleaseCandidateBanner version={props.versionInfo()?.version} />',
    );
    expect(appLayoutSource).toContain("const blockedPrefixes = ['/settings', '/patrol'];");
    expect(appLayoutSource).not.toContain("'/operations', '/patrol', '/ai'");
    expect(appLayoutSource).toContain("route: '/patrol',");
    expect(appLayoutSource).not.toContain("route: '/operations',");
    expect(appLayoutSource).not.toContain('props.connected()');
    expect(appLayoutSource).toContain('const utilityTabs = createMemo(() =>');
    expect(appLayoutSource).toContain(
      'type MobileNavBarPrimaryTab as PrimaryTab,\n  type MobileNavBarUtilityTab as UtilityTab,',
    );
    expect(appLayoutSource).toContain("const NAV_TAB_ICON_CLASS = 'w-4 h-4 shrink-0';");
    expect(appLayoutSource).toContain('function getDesktopUtilityTabAriaLabel(tab: UtilityTab)');
    expect(appLayoutSource).toContain('return `${count} ${tab.label}`;');
    expect(appLayoutSource).toContain('const primaryTabs = createMemo<PrimaryTab[]>(() =>');
    expect(appLayoutSource).toContain("id: 'proxmox',");
    expect(appLayoutSource).toContain('icon: ProxmoxIcon,');
    expect(appLayoutSource).toContain('const Icon = tab.icon;');
    expect(appLayoutSource).toContain('const Icon = tab.icon;');
    expect(appLayoutSource).toContain('aria-label={tab.label}');
    expect(appLayoutSource).toContain('aria-label={getDesktopUtilityTabAriaLabel(tab)}');
    expect(appLayoutSource).toContain(
      '<span aria-hidden="true" class="inline-flex items-center justify-center">',
    );
    expect(appLayoutSource).toContain('<Icon class={NAV_TAB_ICON_CLASS} />');
    expect(appLayoutSource).not.toContain('type PrimaryTab = {');
    expect(appLayoutSource).not.toContain('type UtilityTab = {');
    expect(appLayoutSource).not.toContain('const platformTabsDesktop = createMemo(() =>');
    expect(appLayoutSource).not.toContain('const platformTabsMobile = createMemo(() =>');
    expect(appLayoutSource).not.toContain(
      "import { isMultiTenantEnabled } from '@/stores/license';",
    );
    expect(appLayoutSource).not.toContain('loadCommercialPosture');
    expect(appLayoutSource).not.toContain('buildReleaseNotesUrl');
    expect(appLayoutSource).not.toContain('buildV6RcFeedbackUrl');
    expect(appLayoutSource).not.toContain('sessionPresentationPolicyResolved');
    expect(appLayoutSource).not.toContain('presentationPolicyHidesCommercialSurfaces');
    expect(appLayoutSource).not.toContain('presentationPolicyHidesOrganizationSurfaces');
    expect(appLayoutSource).not.toContain('presentationPolicyIsDemoMode');
    expect(appLayoutSource).toContain('await preloadRouteModule(targetRoute);');
    expect(appLayoutSource).toContain('await preloadRouteModule(tab.route);');
    expect(appLayoutSource).toContain('onMouseEnter={() => warmNavigationTarget(');
    expect(appLayoutSource).toContain('aiChatStore.enabled === true &&');
    expect(appLayoutSource).toContain('!dialogStackHasBlockingDialog()');
    expect(appLayoutSource).toContain('onClick={() => aiChatStore.toggle()}');
    expect(appLayoutSource).toContain('getAIChatLauncherTitle');
    expect(appLayoutSource).toContain('const AI_CHAT_LAUNCHER_BUTTON_CLASS =');
    expect(appLayoutSource).toContain('bottom-[calc(5rem+env(safe-area-inset-bottom,0px))]');
    expect(appLayoutSource).toContain('lg:top-1/2');
    expect(appLayoutSource).toContain('lg:bottom-auto');
    expect(appLayoutSource).not.toContain('sm:top-1/2');
    expect(appLayoutSource).not.toContain('Pulse Assistant (⌘K)');
    expect(appSource).not.toContain("eventBus.on('theme_changed'");
    expect(appSource).not.toContain("eventBus.on('websocket_reconnected'");
    expect(appSource).not.toContain("apiFetch('/api/security/status')");
    expect(appLayoutSource).not.toContain("eventBus.on('theme_changed'");
    expect(appLayoutSource).not.toContain("apiFetch('/api/security/status')");
    expect(appRuntimeStateSource).toContain('export const useAppRuntimeState = () =>');
    expect(appRuntimeStateSource).toContain("import { aiChatStore } from '@/stores/aiChat';");
    expect(appRuntimeStateSource).toContain(
      'const connectionStatus = createMemo<AppConnectionStatus>(() => {',
    );
    expect(appRuntimeStateSource).toContain('const showOrgSwitcher = createMemo(() => {');
    expect(appRuntimeStateSource).toContain('const beginAuthenticatedRuntime = async () =>');
    expect(appRuntimeStateSource).toContain(
      'const [backendHealthy, setBackendHealthy] = createSignal(false);',
    );
    expect(appRuntimeStateSource).toContain('const checkBackendHealth = async () => {');
    expect(appRuntimeStateSource).toContain('const loadOrganizations = async () =>');
    expect(appRuntimeStateSource).toContain('const handleOrgSwitch = (nextOrgID: string) =>');
    expect(appRuntimeStateSource).toContain('const handleOrganizationsChanged = () => {');
    expect(appRuntimeStateSource).toContain(
      "eventBus.on('organizations_changed', handleOrganizationsChanged);",
    );
    expect(appRuntimeStateSource).toContain(
      "eventBus.off('organizations_changed', handleOrganizationsChanged);",
    );
    expect(appRuntimeStateSource).toContain(
      "import {\n  isHostedModeEnabled,\n  isMultiTenantEnabled,\n  runtimeCapabilitiesLoaded,\n  loadRuntimeCapabilities,\n} from '@/stores/license';",
    );
    expect(appRuntimeStateSource).toContain(
      "import { loadCommercialPosture } from '@/stores/licenseCommercial';",
    );
    expect(appRuntimeStateSource).toContain('presentationPolicyHidesOrganizationSurfaces');
    expect(appRuntimeStateSource).toContain('presentationPolicyHidesUpgradePrompts');
    expect(appRuntimeStateSource).toContain('const [activeOrgID, setActiveOrgID] = createSignal(');
    expect(appRuntimeStateSource).toContain('onMount(() => {');
    expect(appRuntimeStateSource).toContain('onMount(async () => {');
    expect(appRuntimeStateSource).toContain('if (!presentationPolicyHidesUpgradePrompts()) {');
    expect(appRuntimeStateSource).toContain('void loadCommercialPosture();');
    expect(appRuntimeStateSource).toContain('const hasLocalAuthBootstrapHint = (): boolean => {');
    expect(appRuntimeStateSource).toContain(
      'const isPreAuthLoginBootstrapPath = (pathname: string): boolean =>',
    );
    expect(appRuntimeStateSource).toContain(
      'if (isPreAuthLoginBootstrapPath(window.location.pathname) && !hasLocalAuthBootstrapHint()) {',
    );
    expect(appRuntimeStateSource).toContain('aiChatStore.setEnabled(');
    expect(appRuntimeStateSource).toContain(
      "eventBus.on('theme_changed', handleRemoteThemeChange);",
    );
    expect(appRuntimeStateSource).toContain(
      "eventBus.on('websocket_reconnected', handleWebSocketReconnected);",
    );
    expect(appRuntimeStateSource).not.toContain('buildInfrastructurePath');
    expect(appRuntimeStateSource).not.toContain('buildWorkloadsPath');
    expect(appRuntimeStateSource).not.toContain("const ROOT_DASHBOARD_PATH = '/dashboard';");
    expect(appRuntimeStateSource).not.toContain(
      "import { startMetricsCollector } from '@/stores/metricsCollector';",
    );
    expect(appRuntimeStateSource).not.toContain('startMetricsCollector();');
    expect(appRuntimeStateSource).not.toContain('function AppLayout(');
    expect(routePreloadSource).toContain('const ROUTE_PRELOADERS: readonly RoutePreloader[] = [');
    expect(routePreloadSource).toContain('export const APP_SHELL_ROUTE_PRELOAD_PATHS = [');
    expect(routePreloadSource).toContain("id: 'standalone',");
    expect(routePreloadSource).toContain("id: 'proxmox',");
    expect(routePreloadSource).toContain("id: 'patrol',");
    expect(routePreloadSource).toContain(
      'const routePreloadCache = new Map<string, Promise<void>>();',
    );
    expect(routePreloadSource).toContain("import('@/pages/Standalone')");
    expect(routePreloadSource).toContain("import('@/pages/Proxmox')");
    expect(routePreloadSource).not.toContain("import('@/pages/Infrastructure')");
    expect(routePreloadSource).not.toContain("import('@/pages/Workloads')");
    expect(routePreloadSource).not.toContain("import('@/pages/Recovery')");
    expect(routePreloadSource).not.toContain("import('@/pages/Storage')");
    expect(routePreloadSource).not.toContain("import('@/pages/Ceph')");
    expect(routePreloadSource).not.toContain("import('@/components/Workloads/WorkloadsSurface')");
    expect(routePreloadSource).not.toContain("import('@/components/Storage/Storage')");
    expect(routePreloadSource).not.toContain("import('@/components/Recovery/Recovery')");
    expect(routePreloadSource).not.toContain("import('@/pages/RecoveryRoute')");
    expect(appRuntimeContextSource).toContain(
      "import { createContext, useContext } from 'solid-js';",
    );
    expect(appRuntimeContextSource).toContain(
      'export const WebSocketContext = createContext<WebSocketStore>();',
    );
    expect(appRuntimeContextSource).toContain('export const useWebSocket = () => {');
    expect(appRuntimeContextSource).toContain(
      'export const DarkModeContext = createContext<() => boolean>();',
    );
  });
});
