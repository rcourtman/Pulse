import { describe, expect, it } from 'vitest';
import appSource from '@/App.tsx?raw';
import appLayoutSource from '@/AppLayout.tsx?raw';
import appRuntimeStateSource from '@/useAppRuntimeState.ts?raw';

describe('App architecture', () => {
  it('keeps App as the entry shell that delegates runtime and chrome ownership', () => {
    expect(appSource).toContain('DASHBOARD_PATH,');
    expect(appSource).toContain("import { AppLayout } from '@/AppLayout';");
    expect(appSource).toContain(
      "import { GlobalResourceContextProvider } from '@/features/globalResourceContext/GlobalResourceContext';",
    );
    expect(appSource).toContain("import { useAppRuntimeState } from '@/useAppRuntimeState';");
    expect(appSource).toContain('const runtime = useAppRuntimeState();');
    expect(appSource).toContain('const ROOT_DASHBOARD_PATH = DASHBOARD_PATH;');
    expect(appSource).toContain('<Route path={ROOT_DASHBOARD_PATH} component={DashboardPage} />');
    expect(appSource).toContain('<Route path="/" component={() => <Navigate href={ROOT_DASHBOARD_PATH} />} />');
    expect(appSource).toContain("const StoragePage = lazy(() => import('./pages/Storage'));");
    expect(appSource).toContain("const OperationsPage = lazy(() => import('./pages/Operations'));");
    expect(appSource).not.toContain(
      "const StorageComponent = lazy(() => import('./components/Storage/Storage'));",
    );
    expect(appSource).not.toContain('function ConnectionStatusBadge(');
    expect(appSource).not.toContain('function AppLayout(');
    expect(appSource).not.toContain('ActiveUseTrialNudge');
    expect(appSource).not.toContain('const [organizations, setOrganizations] = createSignal(');
    expect(appSource).not.toContain('const [themePreference, setThemePreference] =');
    expect(appSource).not.toContain('const [activeOrgID, setActiveOrgID] = createSignal(');
    expect(appSource).toContain('<GlobalResourceContextProvider>');
    expect(appSource).toContain('</GlobalResourceContextProvider>');
  });

  it('keeps authenticated chrome in AppLayout and hosted bootstrap in useAppRuntimeState', () => {
    expect(appLayoutSource).toContain('export function AppLayout(props: AppLayoutProps)');
    expect(appLayoutSource).toContain(
      "import { useGlobalResourceContext } from '@/features/globalResourceContext/GlobalResourceContext';",
    );
    expect(appLayoutSource).toContain(
      "import { GlobalResourceContextBar } from '@/features/globalResourceContext/GlobalResourceContextBar';",
    );
    expect(appLayoutSource).toContain('<OrgSwitcher');
    expect(appLayoutSource).toContain('const status = () => props.connectionStatus();');
    expect(appLayoutSource).toContain("status().kind === 'sync-reconnecting' || status().kind === 'reconnecting'");
    expect(appLayoutSource).toContain(
      "props.connectionStatus().kind === 'connected' && props.dataUpdated()",
    );
    expect(appLayoutSource).toContain("props.versionInfo()?.channel === 'rc'");
    expect(appLayoutSource).toContain('Preview');
    expect(appLayoutSource).not.toContain('props.connected()');
    expect(appLayoutSource).toContain('const utilityTabs = createMemo(() =>');
    expect(appSource).not.toContain("eventBus.on('theme_changed'");
    expect(appSource).not.toContain("eventBus.on('websocket_reconnected'");
    expect(appSource).not.toContain("apiFetch('/api/security/status')");
    expect(appLayoutSource).not.toContain("eventBus.on('theme_changed'");
    expect(appLayoutSource).not.toContain("apiFetch('/api/security/status')");
    expect(appLayoutSource).toContain('const globalContext = useGlobalResourceContext();');
    expect(appLayoutSource).toContain("route: globalContext.buildPlatformRoute('infrastructure')");
    expect(appLayoutSource).toContain(
      "route: buildPathWithGlobalResourceContext('/alerts', globalContext.contextResourceId())",
    );
    expect(appLayoutSource).toContain(
      '<GlobalResourceContextBar class="border-b border-border px-4 py-2" />',
    );
    expect(appRuntimeStateSource).toContain('export const useAppRuntimeState = () =>');
    expect(appRuntimeStateSource).toContain('const connectionStatus = createMemo<AppConnectionStatus>(() => {');
    expect(appRuntimeStateSource).toContain('const beginAuthenticatedRuntime = async () =>');
    expect(appRuntimeStateSource).toContain("const [backendHealthy, setBackendHealthy] = createSignal(false);");
    expect(appRuntimeStateSource).toContain("const checkBackendHealth = async () => {");
    expect(appRuntimeStateSource).toContain('const loadOrganizations = async () =>');
    expect(appRuntimeStateSource).toContain('const handleOrgSwitch = (nextOrgID: string) =>');
    expect(appRuntimeStateSource).toContain(
      'const [activeOrgID, setActiveOrgID] = createSignal(',
    );
    expect(appRuntimeStateSource).toContain('onMount(() => {');
    expect(appRuntimeStateSource).toContain('onMount(async () => {');
    expect(appRuntimeStateSource).toContain("eventBus.on('theme_changed', handleRemoteThemeChange);");
    expect(appRuntimeStateSource).toContain(
      "eventBus.on('websocket_reconnected', handleWebSocketReconnected);",
    );
    expect(appRuntimeStateSource).not.toContain("import { startMetricsCollector } from '@/stores/metricsCollector';");
    expect(appRuntimeStateSource).not.toContain('startMetricsCollector();');
    expect(appRuntimeStateSource).not.toContain('function AppLayout(');
  });
});
