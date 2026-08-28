import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  getDefaultWorkspaceRoute,
  resolvePlatformNavigationAdmission,
  sessionCanReadUpdateStatus,
} from '@/App';
import appSource from '@/App.tsx?raw';
import appLayoutSource from '@/AppLayout.tsx?raw';
import appRuntimeContextSource from '@/contexts/appRuntime.ts?raw';
import runtimeHomeSource from '@/pages/RuntimeHome.tsx?raw';
import routePreloadSource from '@/routing/routePreload.ts?raw';
import appRuntimeStateSource from '@/useAppRuntimeState.ts?raw';
import type { Resource } from '@/types/resource';

const appStylesSource = readFileSync(join(process.cwd(), 'src/index.css'), 'utf8');
const headerAuditSource = readFileSync(join(process.cwd(), 'scripts/header-audit.mjs'), 'utf8');
const integrationTestsDir = join(process.cwd(), '..', 'tests', 'integration', 'tests');
const platformSurfaceSources = [
  'features/proxmox/ProxmoxPageSurface.tsx',
  'features/docker/DockerPageSurface.tsx',
  'features/kubernetes/KubernetesPageSurface.tsx',
  'features/truenas/TrueNASPageSurface.tsx',
  'features/vmware/VmwarePageSurface.tsx',
  'features/standalone/StandalonePageSurface.tsx',
].map((path) => readFileSync(join(process.cwd(), 'src', path), 'utf8'));

function readIntegrationTestSources(dir: string): Array<{ path: string; source: string }> {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      return readIntegrationTestSources(path);
    }
    if (!entry.isFile() || !path.endsWith('.ts')) {
      return [];
    }
    return [{ path, source: readFileSync(path, 'utf8') }];
  });
}

const makeResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'resource-1',
    name: overrides.name ?? overrides.id ?? 'resource-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'resource-1',
    type: overrides.type ?? 'agent',
    platformId: overrides.platformId ?? 'platform-1',
    platformType: overrides.platformType ?? 'agent',
    sourceType: overrides.sourceType ?? 'api',
    status: overrides.status ?? 'online',
    lastSeen: overrides.lastSeen ?? 1_700_000_000_000,
    ...overrides,
  }) as Resource;

describe('App platform navigation admission', () => {
  const authenticatedResources = [
    makeResource({ id: 'machine-1', platformType: 'agent' }),
    makeResource({ id: 'docker-1', type: 'docker-host', platformType: 'docker' }),
    makeResource({ id: 'truenas-1', platformType: 'truenas', sources: ['truenas'] }),
  ];

  it('admits current authenticated REST resources before a WebSocket initial payload', () => {
    const admission = resolvePlatformNavigationAdmission(authenticatedResources, true);

    expect(admission.resolved).toBe(true);
    expect(admission.visibility).toMatchObject({
      docker: true,
      truenas: true,
      standalone: true,
    });
    expect(getDefaultWorkspaceRoute(admission.visibility, true)).toBe('/docker/overview');
  });

  it('retains platform visibility through a transient WebSocket disconnect', () => {
    const beforeDisconnect = resolvePlatformNavigationAdmission(authenticatedResources, true);
    const duringReconnect = resolvePlatformNavigationAdmission(authenticatedResources, true);

    expect(duringReconnect).toEqual(beforeDisconnect);
    expect(getDefaultWorkspaceRoute(duringReconnect.visibility, true)).toBe('/docker/overview');
  });

  it('keeps an evidence-free first load unresolved despite stale browser-local metadata', () => {
    window.localStorage.setItem('guest_metadata', JSON.stringify({ id: 'stale-machine' }));
    window.localStorage.setItem('docker_metadata', JSON.stringify({ id: 'stale-docker' }));

    try {
      const admission = resolvePlatformNavigationAdmission([], false);

      expect(admission.resolved).toBe(false);
      expect(Object.values(admission.visibility)).toEqual([
        false,
        false,
        false,
        false,
        false,
        false,
      ]);
    } finally {
      window.localStorage.removeItem('guest_metadata');
      window.localStorage.removeItem('docker_metadata');
    }
  });

  it('resolves navigation from the canonical admission facet before runtime state arrives', () => {
    // The facet is what lets the shell render navigation without first
    // downloading and classifying an estate-sized runtime payload.
    const admission = resolvePlatformNavigationAdmission([], false, {
      proxmox: true,
      docker: false,
      kubernetes: false,
      truenas: true,
      vmware: false,
      standalone: false,
    });

    expect(admission.resolved).toBe(true);
    expect(admission.visibility).toMatchObject({ proxmox: true, truenas: true, standalone: false });
  });

  it('does not admit the standalone page for a provider-owned estate', () => {
    // A TrueNAS or Proxmox host reports through the agent source, so a
    // count-based derivation would show a Machines tab for an estate that has
    // no Pulse agent in it. The facet reports ownership, so this stays hidden.
    const admission = resolvePlatformNavigationAdmission([], false, {
      proxmox: false,
      docker: false,
      kubernetes: false,
      truenas: true,
      vmware: false,
      standalone: false,
    });

    expect(admission.visibility.standalone).toBe(false);
    expect(getDefaultWorkspaceRoute(admission.visibility, true)).toBe('/truenas/overview');
  });

  it('keeps live runtime state authoritative once it arrives', () => {
    // The estate can gain a platform after the facet was read, so the live
    // payload wins rather than freezing navigation at the bootstrap answer.
    const staleFacet = {
      proxmox: false,
      docker: false,
      kubernetes: false,
      truenas: false,
      vmware: false,
      standalone: false,
    };
    const admission = resolvePlatformNavigationAdmission(authenticatedResources, true, staleFacet);

    expect(admission.resolved).toBe(true);
    expect(admission.visibility).toMatchObject({ docker: true, truenas: true, standalone: true });
  });

  it('stays unresolved when neither the facet nor runtime state is available', () => {
    const admission = resolvePlatformNavigationAdmission([], false, null);

    expect(admission.resolved).toBe(false);
    expect(Object.values(admission.visibility)).toEqual([false, false, false, false, false, false]);
  });

  it('resolves an authenticated empty estate without inventing platform visibility', () => {
    const admission = resolvePlatformNavigationAdmission([], true);

    expect(admission.resolved).toBe(true);
    expect(Object.values(admission.visibility)).toEqual([false, false, false, false, false, false]);
    expect(getDefaultWorkspaceRoute(admission.visibility, true)).toBe('/settings/infrastructure');
    expect(getDefaultWorkspaceRoute(admission.visibility, false)).toBe('/alerts');
  });
});

describe('Global update progress authorization', () => {
  it('keeps the watcher available when update routes are open without authentication', () => {
    expect(
      sessionCanReadUpdateStatus({
        requiresAuth: false,
        settingsCapabilities: undefined,
      }),
    ).toBe(true);
  });

  it('follows the served system-settings capability on authenticated installations', () => {
    expect(
      sessionCanReadUpdateStatus({
        requiresAuth: true,
        settingsCapabilities: {
          systemSettingsRead: true,
        },
      }),
    ).toBe(true);
    expect(
      sessionCanReadUpdateStatus({
        requiresAuth: true,
        settingsCapabilities: {
          systemSettingsRead: false,
        },
      }),
    ).toBe(false);
  });

  it('fails closed when an authenticated status omits the route capability', () => {
    expect(
      sessionCanReadUpdateStatus({
        requiresAuth: true,
        settingsCapabilities: undefined,
      }),
    ).toBe(false);
  });
});

describe('App architecture', () => {
  it('keeps every top-level destination on the same wide shell contract', () => {
    expect(appStylesSource).toContain('--pulse-shell-max-width: min(97vw, 1920px)');
    expect(appStylesSource).not.toContain('--pulse-shell-max-width: min(97vw, 1560px)');
    expect(appStylesSource).not.toContain(
      '.pulse-shell:has(.pulse-wide-data-surface):not(.pulse-shell--full-width)',
    );
  });

  it('keeps narrow platform tables readable through the global app-shell contract', () => {
    expect(appStylesSource).toContain('@container (max-width: 33.999rem)');
    expect(appStylesSource).toContain(':is(th, td).platform-table-phone-hidden');
    expect(appStylesSource).toContain(':is(th, td).platform-table-phone-only');
    expect(appStylesSource).toContain('.platform-table-phone-hidden-inline');
    expect(appStylesSource).toContain('.platform-table-phone-only-inline');
    expect(appStylesSource).toContain('@container (max-width: 22.499rem)');
    expect(appStylesSource).toContain(
      '.table-scroll-shell > .table-fixed.platform-table th.platform-table-name-column',
    );
    expect(appStylesSource).toContain('width: 40%;');
    expect(appStylesSource).toContain(':is(th, td).platform-table-narrow-hidden');
    expect(appStylesSource).toContain('display: none;');
    expect(appStylesSource).not.toContain('white-space: normal;\n    -webkit-box-orient');
  });

  it('keeps manual workload widths inside the existing horizontal table shell', () => {
    expect(appStylesSource).toContain('.table-scroll-shell > table.workload-table--manual-widths');
    expect(appStylesSource).toContain('.workload-col-resizer');
    expect(appStylesSource).toContain('cursor: col-resize');
    expect(appStylesSource).toContain('touch-action: none');
    expect(appStylesSource).toContain('@media (prefers-reduced-motion: reduce)');
  });

  it('keeps native form and browser autofill paint on semantic theme tokens', () => {
    expect(appStylesSource).toContain('color-scheme: light');
    expect(appStylesSource).toContain('color-scheme: dark');
    expect(appStylesSource).toContain('input:-webkit-autofill');
    expect(appStylesSource).toContain(
      '-webkit-box-shadow: 0 0 0 1000px var(--color-bg-surface) inset;',
    );
    expect(appStylesSource).toContain('-webkit-text-fill-color: var(--color-text-base);');
  });

  it('keeps infrastructure platforms marked as dense data surfaces', () => {
    platformSurfaceSources.forEach((source) => {
      expect(source).toMatch(/data-testid="[^"]+-page" class="pulse-wide-data-surface /);
    });
  });

  it('keeps App as the entry shell that delegates runtime and chrome ownership', () => {
    expect(appSource).toContain(
      "import { AppLayout, sessionHasSettingsAccess } from '@/AppLayout';",
    );
    expect(appSource).toContain("import { aiChatStore } from './stores/aiChat';");
    expect(appSource).toContain(
      "import { DarkModeContext, WebSocketContext, useWebSocket } from '@/contexts/appRuntime';",
    );
    expect(appSource).toContain("import { useAppRuntimeState } from '@/useAppRuntimeState';");
    expect(appSource).toContain(
      "import { dialogStackHasBlockingDialog } from './components/shared/useDialogState';",
    );
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
    expect(routePreloadSource).toContain(
      'export const APP_SHELL_ROUTE_PRELOAD_PATHS = [ACTIONS_PATH] as const;',
    );
    expect(routePreloadSource).not.toContain('ROOT_PROXMOX_PATH');
    expect(routePreloadSource).not.toContain('ROOT_STANDALONE_PATH');
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
    expect(appRuntimeStateSource).toContain('ssoSessionDisplayName?: string;');
    expect(appRuntimeStateSource).toContain(
      'securityData.ssoSessionDisplayName || securityData.ssoSessionUsername',
    );
    expect(appRuntimeStateSource).toContain('username: ssoDisplayName');
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
    expect(appSource).toContain('platformNavigationResolved');
    expect(appSource).toContain('resolvePlatformNavigationAdmission');
    expect(appSource).toContain('runtime.runtimeStateResolved()');
    expect(appSource).not.toContain('store?.initialDataReceived?.()');
    expect(appSource).toContain('if (!platformNavigationResolved()) return;');
    expect(appSource).toContain('buildPrimaryPlatformNavigationVisibility');
    expect(appLayoutSource).toContain('buildPrimaryPlatformNavigationVisibility');
    expect(appLayoutSource).toContain('primaryPlatformNavigationIsVisible');
    expect(appLayoutSource).toContain("label: 'Docker'");
    expect(appLayoutSource).toContain("'Docker / Podman runtime lens");
    expect(appLayoutSource).not.toContain("id: 'infrastructure',");
    expect(appLayoutSource).not.toContain("id: 'workloads',");
    expect(appLayoutSource).not.toContain("id: 'storage',");
    expect(appLayoutSource).not.toContain("id: 'recovery',");
    expect(appLayoutSource).not.toContain('aria-label="Workspaces"');
    expect(appLayoutSource).not.toContain('buildStorageRecoveryTabSpecs(');
    // Nav tab arrays are rebuilt from live store reads (activeAlerts is
    // replaced wholesale by state frames), so both tab lists must route
    // through the shared identity stabilizer or every reference-keyed <For>
    // consumer recreates all nav buttons and drops in-flight taps.
    expect(appLayoutSource).toContain(
      'const primaryTabs = createStableTabList(primaryTabsSource, primaryNavTabEquals);',
    );
    expect(appLayoutSource).toContain(
      'const utilityTabs = createStableTabList(utilityTabsSource, utilityNavTabEquals);',
    );
    expect(appSource).not.toContain('DashboardPage');
    expect(headerAuditSource).not.toContain("['src/pages/Dashboard.tsx', 'PageHeader']");
    expect(appSource).toContain("import RuntimeHomePage from '@/pages/RuntimeHome';");
    expect(appSource).toContain('<Route path="/login" component={RuntimeHomePage} />');
    expect(appSource).toContain('<Route path="/" component={RuntimeHomePage} />');
    expect(appSource).toContain('<Route path="/infrastructure" component={RuntimeHomePage} />');
    expect(appSource).toContain('function isWorkspaceEntryRoutePath(pathname: string): boolean');
    expect(appSource).toContain("normalizedPath === '/'");
    expect(appSource).toContain("normalizedPath === '/login'");
    expect(appSource).toContain("normalizedPath === '/infrastructure'");
    expect(appSource).toContain('let workspaceRedirectPending = false');
    expect(appSource).toContain('if (workspaceRedirectPending) return');
    expect(appSource).toContain('workspaceRedirectPending = true');
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
    expect(appSource).toContain('window.setTimeout(() => {');
    expect(appSource).toContain("closest('[data-ai-model-picker]')");
    expect(appSource).toContain(
      'if (!e.defaultPrevented && !isModelPickerEscape && aiChatStore.isOpen) {',
    );
    expect(appSource).toContain('<AIChat onClose={() => aiChatStore.close()} />');
    expect(appSource).toContain('showOrgSwitcher={runtime.showOrgSwitcher}');
    expect(appSource).toContain("import { WhatsNewCard } from './components/WhatsNewCard';");
    expect(appSource).toContain('<WhatsNewCard />');
    expect(appSource).not.toContain('TrialBanner');
    expect(appSource).not.toContain('MonitoredSystemLimitWarningBanner');
    expect(appSource).not.toContain('monitoredSystemLimitWarningBanner');
  });

  it('Issue1650 keeps kiosk-scoped sessions off settings chrome in the shell', () => {
    // ESC used to call toggleKioskMode() unconditionally, and setKioskMode(false)
    // is sticky for the rest of the session, so a kiosk display could be walked
    // out of kiosk and into settings-linked chrome it has no authority to use.
    expect(appLayoutSource).toContain('export function sessionHasSettingsAccess(');
    expect(appLayoutSource).toContain(
      'const hasSettingsAccess = createMemo(() => sessionHasSettingsAccess(props.tokenScopes()));',
    );
    expect(appLayoutSource).toContain("if (event.key !== 'Escape') return;");
    expect(appLayoutSource).toContain('if (!hasSettingsAccess()) {');
    expect(appLayoutSource).toContain('scheduleHideHeader(3000);');
    expect(appLayoutSource).not.toContain(
      "if (event.key === 'Escape') {\n        toggleKioskMode();",
    );
    // The blocked-route guard is an authority rule, not kiosk decoration.
    expect(appLayoutSource).toContain('if (!kioskMode() && hasSettingsAccess()) return;');
    expect(appLayoutSource).toContain("const blockedPrefixes = ['/settings', '/patrol'];");
    // Banners in the shell deep-link into settings, so they follow the same rule.
    expect(appSource).toContain('<Show when={!kioskMode() && hasSettingsAccess()}>');
    expect(appSource).toContain(
      'const hasSettingsAccess = createMemo(() =>\n      sessionHasSettingsAccess(runtime.securityStatus()?.tokenScopes),\n    );',
    );
    expect(appSource).not.toContain("scopes.includes('settings:read')");
  });

  it('keeps the update progress watcher aligned with the backend updater stages', () => {
    // The in-progress stage list must mirror internal/updates/manager.go
    // updateStatus emissions, including the rollback path's restoring stage,
    // so the progress modal auto-opens for every real update or rollback.
    expect(appSource).toContain("'downloading',");
    expect(appSource).toContain("'verifying',");
    expect(appSource).toContain("'extracting',");
    expect(appSource).toContain("'backing-up',");
    expect(appSource).toContain("'applying',");
    expect(appSource).toContain("'restoring',");
    expect(appSource).toContain("'restarting',");
    // 'checking' is a probe, not an apply: it must never pop the modal, and
    // the never-emitted legacy 'installing' stage must not return.
    expect(appSource).not.toContain("'checking',");
    expect(appSource).not.toContain("'installing'");
  });

  it('mounts the update progress watcher only behind its route capability', () => {
    expect(appSource).toContain('sessionCanReadUpdateStatus(runtime.securityStatus())');
    expect(appSource).toContain('return status.settingsCapabilities?.systemSettingsRead === true;');
    expect(appSource).toContain('if (!status || !status.requiresAuth) return true;');

    const watcherGate = appSource.indexOf('<Show when={canReadUpdateStatus()}>');
    const watcherMount = appSource.indexOf('<GlobalUpdateProgressWatcher />');
    expect(watcherGate).toBeGreaterThan(-1);
    expect(watcherMount).toBeGreaterThan(watcherGate);
    expect(appSource.split('<GlobalUpdateProgressWatcher />')).toHaveLength(2);
  });

  it('keeps integration browser proofs off the retired AI route', () => {
    const routesSource = readFileSync(join(integrationTestsDir, 'routes.ts'), 'utf8');
    const retiredRouteNavigations = readIntegrationTestSources(integrationTestsDir).flatMap(
      ({ path, source }) =>
        source
          .split('\n')
          .flatMap((line, index) =>
            /page\.goto\(\s*(['"])\/ai\1/.test(line) ? [`${path}:${index + 1}`] : [],
          ),
    );

    expect(routesSource).toContain('export const PATROL_ROUTE = "/patrol";');
    expect(retiredRouteNavigations).toEqual([]);
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
    expect(appLayoutSource).toContain('type PrimaryRouteMemory = Partial');
    expect(appLayoutSource).toContain('let primaryRouteMemory: PrimaryRouteMemory = {};');
    expect(appLayoutSource).toContain('function resolvePrimaryNavigationRoute(');
    expect(appLayoutSource).toContain('routeBelongsToPrimaryTab(remembered');
    expect(appLayoutSource).not.toContain('primaryRouteMemory[props.activeOrgID');
    expect(appLayoutSource).not.toContain('primaryRouteMemory[activeOrgID');
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
    expect(appLayoutSource).toContain("'pb-safe-or-14 xl:pb-0'");
    expect(appStylesSource).toContain('.pb-safe-or-14');
    expect(appStylesSource).toContain('.pulse-shell--full-width');
    expect(appStylesSource).toContain('.pulse-wide-data-surface.space-y-3');
    expect(appStylesSource).toContain('.filter-bar > div > div:first-child button');
    expect(appStylesSource).toContain(
      'grid-template-columns: repeat(auto-fit, minmax(min(11rem, 100%), 11rem))',
    );
    expect(appStylesSource).toContain(".view-options-grid > * [role='group'] > button");
    expect(appStylesSource).toContain('flex: 1 1 0%');
    expect(appStylesSource).toContain("button[aria-expanded='true']");
    expect(appStylesSource).not.toContain('.proxmox-nodes-card > :first-child');
    expect(appStylesSource).toContain(
      '.table-fixed.platform-table > tbody > tr:not([data-inline-detail-for])',
    );
    expect(appStylesSource).not.toContain(
      '.pulse-wide-data-surface .host-row,\n    .pulse-wide-data-surface .workload-row',
    );
    expect(appStylesSource).toContain('.pulse-footer > div');
    expect(appLayoutSource).toContain('tabs mb-2 hidden xl:flex');
    expect(appLayoutSource).toContain('xl:px-2 2xl:px-3');
    expect(appLayoutSource).toContain('gap-1 2xl:gap-1.5');
    expect(appLayoutSource).not.toContain('tabs mb-2 hidden lg:flex');
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
    expect(appStylesSource).toContain('.table-scroll-shell');
    expect(appStylesSource).toContain('container-type: inline-size');
    expect(appStylesSource).toContain('.table-fixed.platform-table th.platform-table-name-column');
    expect(appStylesSource).toContain('th.platform-table-mobile-w-10');
    expect(appStylesSource).toMatch(/th\.platform-table-name-column\s*\{\s*width:\s*30%;\s*\}/);
    expect(appStylesSource).toContain('@container (min-width: 72rem)');
    expect(appStylesSource).toContain('.hidden.md\\:table-cell');
    expect(appStylesSource).toContain('contain: paint');
    expect(appStylesSource).toContain('overflow-y: hidden');
    expect(appStylesSource).toContain('overscroll-behavior-x: contain');
    expect(appStylesSource).toContain('overscroll-behavior-y: chain');
    expect(appStylesSource).toContain('.table-scroll-shell.table-scroll-shell-phone-page');
    expect(appStylesSource).toContain('overflow: clip');
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
    expect(appLayoutSource).toContain("label: 'Patrol'");
    expect(appLayoutSource).toContain("id: 'actions',");
    expect(appLayoutSource).toContain("label: 'Actions',");
    expect(appLayoutSource).toContain("route: '/actions',");
    expect(appLayoutSource).toContain(
      'const getNavigationActiveTab = () => getActiveTabForPath(location.pathname);',
    );
    expect(appLayoutSource).toContain(
      "tooltip: 'Review active operational attention and recent Patrol checks'",
    );
    expect(appLayoutSource).toContain('const patrolAttentionCount = createMemo(');
    expect(appLayoutSource).toContain('countLabel: patrolAttentionCountLabel()');
    expect(appLayoutSource).toContain('countLabel: actionApprovalBadge()?.label');
    expect(appLayoutSource).not.toContain("label: 'Needs Attention'");
    expect(appLayoutSource).not.toContain("route: '/operations',");
    expect(appLayoutSource).not.toContain('props.connected()');
    expect(appLayoutSource).toContain('const utilityTabsSource = createMemo<UtilityTab[]>(() =>');
    expect(appLayoutSource).toContain(
      'type MobileNavBarPrimaryTab as PrimaryTab,\n  type MobileNavBarUtilityTab as UtilityTab,',
    );
    expect(appLayoutSource).toContain("const NAV_TAB_ICON_CLASS = 'w-4 h-4 shrink-0';");
    expect(appLayoutSource).toContain('function getDesktopUtilityTabAriaLabel(tab: UtilityTab)');
    expect(appLayoutSource).toContain('return `${count} ${tab.label}`;');
    expect(appLayoutSource).toContain('const primaryTabsSource = createMemo<PrimaryTab[]>(() =>');
    expect(appLayoutSource).toContain("id: 'proxmox',");
    expect(appLayoutSource).toContain("icon: getPlatformIcon('proxmox'),");
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
    expect(appLayoutSource).toContain(
      'warmNavigationTarget(targetRoute);\n    navigate(targetRoute);',
    );
    expect(appLayoutSource).toContain('warmNavigationTarget(tab.route);\n    navigate(tab.route);');
    expect(appLayoutSource).not.toContain('await preloadRouteModule(targetRoute);');
    expect(appLayoutSource).not.toContain('await preloadRouteModule(tab.route);');
    expect(appLayoutSource).toContain('onMouseEnter={() => warmNavigationTarget(');
    expect(appLayoutSource).toContain('aiChatStore.enabled === true &&');
    expect(appLayoutSource).toContain('!dialogStackHasBlockingDialog()');
    expect(appLayoutSource).toContain('onClick={openAssistantFromLauncher}');
    expect(appLayoutSource).toContain('getAssistantPageContext');
    expect(appLayoutSource).toContain('const AI_CHAT_MOBILE_LAUNCHER_BUTTON_CLASS =');
    expect(appLayoutSource).toContain('const AI_CHAT_DESKTOP_LAUNCHER_BUTTON_CLASS =');
    expect(appLayoutSource).toContain("viewport.isBelow('lg')");
    expect(appLayoutSource).toContain("viewport.isAtLeast('lg')");
    expect(appLayoutSource).toContain('fixed right-0 top-1/2');
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
    expect(appRuntimeStateSource).toContain(
      'const loadAuthenticatedBootstrapState = async (): Promise<boolean> => {',
    );
    expect(appRuntimeStateSource).toContain(
      'const beginAuthenticatedRuntime = async (): Promise<boolean> => {',
    );
    expect(appRuntimeStateSource).toContain('if (!(await loadAuthenticatedBootstrapState())) {');
    // The bootstrap probes the session once, and never through the full state
    // payload: `/api/state` stays the oversized-snapshot recovery path only.
    expect(appRuntimeStateSource.match(/apiFetch\('\/api\/state\/summary'/g)).toHaveLength(1);
    expect(appRuntimeStateSource).not.toContain("apiFetch('/api/state',");
    expect(appRuntimeStateSource).toContain('SettingsAPI.getRuntimeDisplay()');
    expect(appRuntimeStateSource).not.toContain('SettingsAPI.getSystemSettings');
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
    expect(appRuntimeStateSource).toContain('const hasSSOCallbackSuccessHint = (): boolean => {');
    expect(appRuntimeStateSource).toContain(
      'isPreAuthLoginBootstrapPath(window.location.pathname) &&',
    );
    expect(appRuntimeStateSource).toContain('!hasLocalAuthBootstrapHint() &&');
    expect(appRuntimeStateSource).toContain('!hasSSOCallbackSuccessHint()');
    // Remember-me sessions ride an HttpOnly cookie with no per-tab artifacts,
    // so the remembered-username hint must keep the /api/state probe reachable
    // after a tab close (issue #1531).
    expect(appRuntimeStateSource).toContain(
      'window.localStorage.getItem(STORAGE_KEYS.REMEMBERED_LOGIN_USERNAME)',
    );
    expect(appRuntimeStateSource).toContain('aiChatStore.setEnabled(');
    expect(appRuntimeStateSource).toContain('aiIntelligenceStore.loadPatrolFindings()');
    expect(appRuntimeStateSource).toContain('aiIntelligenceStore.loadPendingApprovals()');
    expect(appRuntimeStateSource).toContain('actionInboxStore.loadPendingActionCount()');
    expect(appRuntimeStateSource).toContain('window.setInterval(refreshOpenWorkBadges, 30000)');
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

  it('keeps dashboard and Explore boundaries out of the runtime home shell', () => {
    const shellEntrySources = [
      { name: 'App.tsx', source: appSource },
      { name: 'AppLayout.tsx', source: appLayoutSource },
      { name: 'RuntimeHome.tsx', source: runtimeHomeSource },
      { name: 'routePreload.ts', source: routePreloadSource },
    ];

    for (const { name, source } of shellEntrySources) {
      expect(source, name).not.toContain('/dashboard');
      expect(source, name).not.toContain("'/explore'");
      expect(source, name).not.toContain('"/explore"');
      expect(source, name).not.toContain('DashboardPage');
      expect(source, name).not.toContain('ExplorePage');
      expect(source, name).not.toContain('getPatrolProtectionPosture');
      expect(source, name).not.toContain('getMonitorContextPatrolProtectionPosture');
      expect(source, name).not.toContain('Patrol protection posture');
      expect(source, name).not.toContain('Proxmox Patrol coverage');
      expect(source, name).not.toContain('Protection current');
    }

    expect(runtimeHomeSource).toContain('runtimeHome.openingWorkspace');
    expect(runtimeHomeSource).not.toContain('aiIntelligenceStore');
    expect(runtimeHomeSource).not.toContain('patrolOpenWork');
    expect(appLayoutSource).toContain("label: 'Patrol'");
    expect(appLayoutSource).toContain('countLabel: patrolAttentionCountLabel()');
    expect(appLayoutSource).toContain("label: 'Actions'");
  });

  it('drops platform admission when the tenant changes', () => {
    const switchStart = appRuntimeStateSource.indexOf('const handleOrgSwitch');
    expect(switchStart).toBeGreaterThan(-1);
    const orgSwitch = appRuntimeStateSource.slice(switchStart, switchStart + 1200);

    // Admission is tenant-scoped. Carrying the outgoing tenant's answer across
    // a switch would render their platform tabs to the incoming tenant, so it
    // is cleared before the replacement is requested.
    const clearIndex = orgSwitch.indexOf('setPlatformAdmission(null)');
    const refetchIndex = orgSwitch.indexOf('loadPlatformAdmission()');
    expect(clearIndex).toBeGreaterThan(-1);
    expect(refetchIndex).toBeGreaterThan(clearIndex);
  });

  it('keeps licensed application branding inside the authenticated shell bootstrap', () => {
    // Asserted on the bootstrap's contents rather than one formatted line, so
    // a prettier line-wrap cannot read as branding leaving the bootstrap.
    const authenticatedBootstrap = appRuntimeStateSource
      .slice(appRuntimeStateSource.indexOf('await Promise.all(['))
      .slice(
        0,
        appRuntimeStateSource
          .slice(appRuntimeStateSource.indexOf('await Promise.all(['))
          .indexOf(']);') + 3,
      );
    expect(authenticatedBootstrap).toContain('loadRuntimeDisplayAndLayout()');
    expect(authenticatedBootstrap).toContain('loadRuntimeBranding()');
    expect(authenticatedBootstrap).toContain('loadPlatformAdmission()');
    expect(appLayoutSource).toContain("import { runtimeBranding } from '@/stores/systemSettings';");
    expect(appLayoutSource).toContain(
      "const browserBrandName = createMemo(() => customBrandName() || 'Pulse');",
    );
    expect(appLayoutSource).toContain('data-testid="custom-brand-logo"');
    expect(appRuntimeStateSource).not.toContain('/api/license/commercial-posture');
    expect(appRuntimeStateSource).not.toContain('/api/license/entitlements');
  });
});

describe('mobile bottom navigation clearance', () => {
  const NAV_HEIGHT_VARIABLE = '--pulse-mobile-nav-height';
  // Matches the shape every drifted copy used, spaced or not. The lookbehind
  // keeps the declared 2.5rem fallback from matching its own guard.
  const HARDCODED_BAR_HEIGHT = /(?<![\d.])5rem\s*\+\s*env\(\s*safe-area-inset-bottom/;

  function collectFrontendSources(dir: string): Array<{ path: string; source: string }> {
    return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
      const entryPath = join(dir, entry.name);
      if (entry.isDirectory()) {
        return collectFrontendSources(entryPath);
      }
      // Only runtime sources matter; a test may quote the old shape in order
      // to assert against it, as this one does.
      if (!/\.(css|ts|tsx)$/.test(entry.name)) {
        return [];
      }
      if (entryPath.includes('__tests__') || /\.(test|spec)\./.test(entry.name)) {
        return [];
      }
      return [{ path: entryPath, source: readFileSync(entryPath, 'utf8') }];
    });
  }

  it('declares the bar height as a single custom property', () => {
    expect(appStylesSource).toContain(NAV_HEIGHT_VARIABLE);
  });

  it('keeps every consumer reading that property instead of its own copy', () => {
    // The Assistant panel, its backdrop, the star banner and both halves of
    // the former filter-popover docking rule each carried their own
    // calc(5rem + env(safe-area-inset-bottom)) for a bar that measures ~45px.
    // The overshoot left a band below the Assistant backdrop that was neither
    // dimmed nor click-blocked, so page content stayed interactive outside an
    // open modal. Read the published height rather than adding a sixth copy.
    const offenders = collectFrontendSources(join(process.cwd(), 'src'))
      .filter(({ source }) => HARDCODED_BAR_HEIGHT.test(source))
      .map(({ path }) => path.slice(process.cwd().length + 1));

    expect(offenders).toEqual([]);
  });
});
