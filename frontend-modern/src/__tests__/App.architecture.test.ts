import { describe, expect, it } from 'vitest';
import appSource from '@/App.tsx?raw';
import appLayoutSource from '@/AppLayout.tsx?raw';
import appRuntimeContextSource from '@/contexts/appRuntime.ts?raw';
import appRuntimeStateSource from '@/useAppRuntimeState.ts?raw';

describe('App architecture', () => {
  it('keeps App as the entry shell that delegates runtime and chrome ownership', () => {
    expect(appSource).toContain('DASHBOARD_PATH,');
    expect(appSource).toContain("import { AppLayout } from '@/AppLayout';");
    expect(appSource).toContain("import { aiChatStore } from './stores/aiChat';");
    expect(appSource).toContain(
      "import { DarkModeContext, WebSocketContext, useWebSocket } from '@/contexts/appRuntime';",
    );
    expect(appSource).toContain("import { useAppRuntimeState } from '@/useAppRuntimeState';");
    expect(appSource).toContain("import {");
    expect(appSource).toContain("} from '@/utils/appShellScrollRestoration';");
    expect(appSource).toContain('const runtime = useAppRuntimeState();');
    expect(appSource).toContain('pendingAppShellRestoreTop');
    expect(appSource).toContain('setAppScrollShellRef');
    expect(appSource).toContain('readPendingAppShellRestoreTop');
    expect(appSource).toContain('clearPendingAppShellRestoreTop');
    expect(appSource).toContain('const ROOT_DASHBOARD_PATH = DASHBOARD_PATH;');
    expect(appSource).toContain('const ROOT_PATROL_PATH = PATROL_PATH;');
    expect(appSource).toContain('<Route path={ROOT_DASHBOARD_PATH} component={DashboardPage} />');
    expect(appSource).toContain('<Route path="/login" component={() => <Navigate href={ROOT_DASHBOARD_PATH} />} />');
    expect(appSource).toContain('<Route path="/" component={() => <Navigate href={ROOT_DASHBOARD_PATH} />} />');
    expect(appSource).toContain('<Route path={`${ROOT_PATROL_PATH}/*`} component={AIIntelligencePage} />');
    expect(appSource).toContain('<Route path="/ai/*" component={LegacyPatrolRouteRedirect} />');
    expect(appSource).toContain(
      "const PricingHandoffPage = lazy(() => import('./pages/PricingHandoff'));",
    );
    expect(appSource).toContain('<Route path="/pricing" component={PricingHandoffPage} />');
    expect(appSource).toContain("const StoragePage = lazy(() => import('./pages/Storage'));");
    expect(appSource).toContain("const OperationsPage = lazy(() => import('./pages/Operations'));");
    expect(appSource).not.toContain(
      "const StorageComponent = lazy(() => import('./components/Storage/Storage'));",
    );
    expect(appSource).not.toContain("const PricingPage = lazy(() => import('./pages/PricingV6'));");
    expect(appSource).not.toContain('function ConnectionStatusBadge(');
    expect(appSource).not.toContain('function AppLayout(');
    expect(appSource).not.toContain('export const WebSocketContext = createContext<');
    expect(appSource).not.toContain('export const DarkModeContext = createContext<');
    expect(appSource).not.toContain('ActiveUseTrialNudge');
    expect(appSource).not.toContain('const [organizations, setOrganizations] = createSignal(');
    expect(appSource).not.toContain('const [themePreference, setThemePreference] =');
    expect(appSource).not.toContain('const [activeOrgID, setActiveOrgID] = createSignal(');
    expect(appSource).not.toContain("import('./api/ai')");
    expect(appSource).not.toContain('AIAPI.getSettings()');
    expect(appSource).toContain("if (e.key === 'Escape' && aiChatStore.isOpen) {");
    expect(appSource).toContain('<AIChat onClose={() => aiChatStore.close()} />');
    expect(appSource).toContain('showOrgSwitcher={runtime.showOrgSwitcher}');
  });

  it('keeps authenticated chrome in AppLayout and hosted bootstrap in useAppRuntimeState', () => {
    expect(appLayoutSource).toContain('export function AppLayout(props: AppLayoutProps)');
    expect(appLayoutSource).toContain("import { aiChatStore } from '@/stores/aiChat';");
    expect(appLayoutSource).toContain(
      "import { ReleaseCandidateBanner } from '@/components/shared/ReleaseCandidateBanner';",
    );
    expect(appLayoutSource).toContain('<OrgSwitcher');
    expect(appLayoutSource).toContain('const status = () => props.connectionStatus();');
    expect(appLayoutSource).toContain("status().kind === 'sync-reconnecting' || status().kind === 'reconnecting'");
    expect(appLayoutSource).toContain(
      "props.connectionStatus().kind === 'connected' && props.dataUpdated()",
    );
    expect(appLayoutSource).toContain("props.versionInfo()?.channel === 'rc'");
    expect(appLayoutSource).toContain('Preview');
    expect(appLayoutSource).toContain(
      '<ReleaseCandidateBanner version={props.versionInfo()?.version} />',
    );
    expect(appLayoutSource).toContain("const blockedPrefixes = ['/settings', '/operations', '/patrol', '/ai'];");
    expect(appLayoutSource).toContain("route: '/patrol',");
    expect(appLayoutSource).not.toContain('props.connected()');
    expect(appLayoutSource).toContain('const utilityTabs = createMemo(() =>');
    expect(appLayoutSource).not.toContain("import { isMultiTenantEnabled } from '@/stores/license';");
    expect(appLayoutSource).not.toContain('loadCommercialPosture');
    expect(appLayoutSource).not.toContain('buildReleaseNotesUrl');
    expect(appLayoutSource).not.toContain('buildV6RcFeedbackUrl');
    expect(appLayoutSource).not.toContain('sessionPresentationPolicyResolved');
    expect(appLayoutSource).not.toContain('presentationPolicyHidesCommercialSurfaces');
    expect(appLayoutSource).not.toContain('presentationPolicyHidesOrganizationSurfaces');
    expect(appLayoutSource).toContain('presentationPolicyIsDemoMode');
    expect(appLayoutSource).toContain("if (!presentationPolicyIsDemoMode()) {");
    expect(appLayoutSource).toContain(
      'aiChatStore.enabled === true && !aiChatStore.isOpenSignal() && !kioskMode()',
    );
    expect(appLayoutSource).toContain('onClick={() => aiChatStore.toggle()}');
    expect(appSource).not.toContain("eventBus.on('theme_changed'");
    expect(appSource).not.toContain("eventBus.on('websocket_reconnected'");
    expect(appSource).not.toContain("apiFetch('/api/security/status')");
    expect(appLayoutSource).not.toContain("eventBus.on('theme_changed'");
    expect(appLayoutSource).not.toContain("apiFetch('/api/security/status')");
    expect(appRuntimeStateSource).toContain('export const useAppRuntimeState = () =>');
    expect(appRuntimeStateSource).toContain("import { aiChatStore } from '@/stores/aiChat';");
    expect(appRuntimeStateSource).toContain('const connectionStatus = createMemo<AppConnectionStatus>(() => {');
    expect(appRuntimeStateSource).toContain('const showOrgSwitcher = createMemo(() => {');
    expect(appRuntimeStateSource).toContain('const beginAuthenticatedRuntime = async () =>');
    expect(appRuntimeStateSource).toContain("const [backendHealthy, setBackendHealthy] = createSignal(false);");
    expect(appRuntimeStateSource).toContain("const checkBackendHealth = async () => {");
    expect(appRuntimeStateSource).toContain('const loadOrganizations = async () =>');
    expect(appRuntimeStateSource).toContain('const handleOrgSwitch = (nextOrgID: string) =>');
    expect(appRuntimeStateSource).toContain(
      "import {\n  isHostedModeEnabled,\n  isMultiTenantEnabled,\n  runtimeCapabilitiesLoaded,\n  loadRuntimeCapabilities,\n} from '@/stores/license';",
    );
    expect(appRuntimeStateSource).toContain(
      "import { loadCommercialPosture } from '@/stores/licenseCommercial';",
    );
    expect(appRuntimeStateSource).toContain('presentationPolicyHidesOrganizationSurfaces');
    expect(appRuntimeStateSource).toContain(
      'const [activeOrgID, setActiveOrgID] = createSignal(',
    );
    expect(appRuntimeStateSource).toContain('onMount(() => {');
    expect(appRuntimeStateSource).toContain('onMount(async () => {');
    expect(appRuntimeStateSource).toContain('void loadCommercialPosture();');
    expect(appRuntimeStateSource).toContain('const hasLocalAuthBootstrapHint = (): boolean => {');
    expect(appRuntimeStateSource).toContain(
      "const isPreAuthLoginBootstrapPath = (pathname: string): boolean =>",
    );
    expect(appRuntimeStateSource).toContain(
      "if (isPreAuthLoginBootstrapPath(window.location.pathname) && !hasLocalAuthBootstrapHint()) {",
    );
    expect(appRuntimeStateSource).toContain('aiChatStore.setEnabled(');
    expect(appRuntimeStateSource).toContain("eventBus.on('theme_changed', handleRemoteThemeChange);");
    expect(appRuntimeStateSource).toContain(
      "eventBus.on('websocket_reconnected', handleWebSocketReconnected);",
    );
    expect(appRuntimeStateSource).toContain("const ROOT_DASHBOARD_PATH = '/dashboard';");
    expect(appRuntimeStateSource).toContain('if (pathname === ROOT_DASHBOARD_PATH) return false;');
    expect(appRuntimeStateSource).not.toContain("import { startMetricsCollector } from '@/stores/metricsCollector';");
    expect(appRuntimeStateSource).not.toContain('startMetricsCollector();');
    expect(appRuntimeStateSource).not.toContain('function AppLayout(');
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
