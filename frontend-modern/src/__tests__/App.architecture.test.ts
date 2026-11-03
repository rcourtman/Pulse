import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  getDefaultWorkspaceRoute,
  resolvePlatformNavigationAdmission,
  sessionCanReadUpdateStatus,
} from '@/App';
import appSource from '@/App.tsx?raw';
import appRuntimeStateSource from '@/useAppRuntimeState.ts?raw';
import type { Resource } from '@/types/resource';

const appStylesSource = readFileSync(join(process.cwd(), 'src/index.css'), 'utf8');
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
  it('hands current authenticated reader facts to the existing Alerts route', () => {
    expect(appSource).toContain(
      'const AlertsRoute = () => <AlertsPage securityStatus={runtime.securityStatus()} />;',
    );
    expect(appSource).toContain('<Route path="/alerts/*" component={AlertsRoute} />');
    expect(appSource).not.toContain('<Route path="/alerts/*" component={AlertsPage} />');
  });

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

  it('does not let retained reconnect admission override an explicit empty resource snapshot', () => {
    const retainedFacet = {
      proxmox: true,
      docker: true,
      kubernetes: false,
      truenas: false,
      vmware: false,
      standalone: false,
    };
    expect(resolvePlatformNavigationAdmission([], false, retainedFacet).visibility.docker).toBe(
      true,
    );
    const authoritativeEmpty = resolvePlatformNavigationAdmission([], true, retainedFacet);
    expect(authoritativeEmpty.resolved).toBe(true);
    expect(Object.values(authoritativeEmpty.visibility)).toEqual([
      false,
      false,
      false,
      false,
      false,
      false,
    ]);
    expect(getDefaultWorkspaceRoute(authoritativeEmpty.visibility, false)).toBe('/alerts');
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
  it('gates Patrol navigation on the AI availability session capability (#905)', () => {
    // The tab itself is rendered in AppLayout.test.tsx; these are the App-level
    // and bootstrap wiring that no render test reaches.
    expect(appSource).toContain('patrolVisible: () => aiAvailabilityStore.enabled === true');
    expect(appSource).toContain('patrolVisible={() => aiAvailabilityStore.enabled === true}');
    expect(appRuntimeStateSource).toContain(
      'aiAvailabilityStore.setEnabled(securityData?.sessionCapabilities?.assistantEnabled === true)',
    );
  });

  it('keeps first-load connection progress understandable and retryable', () => {
    expect(appSource).toContain('(!runtime.bootstrapFailed() || isPublicRoute())');
    expect(appSource).toContain(
      'fallback={<AppBootstrapStatus failed={runtime.bootstrapFailed()} />}',
    );
    expect(appSource).not.toContain('<div class="text-muted">Loading...</div>');
  });

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

  it('keeps the desired replica count on screen in the Deployments table', () => {
    // Desired returns with the 72rem tier, so Ready's own /desired total hides
    // only there, after its base rule so the container rule wins.
    const base = appStylesSource.indexOf('.k8s-deployment-ready-total {\n    display: inline;');
    const hide = appStylesSource.indexOf(
      '@container (min-width: 72rem) {\n    .k8s-deployment-ready-total {\n      display: none;',
    );
    expect(base).toBeGreaterThan(-1);
    expect(hide).toBeGreaterThan(base);
    expect(appStylesSource.match(/\.k8s-deployment-ready-total \{\s*display: none;/g)).toHaveLength(
      1,
    );
  });

  it('keeps the identity anchor between the phone projection and the md column widths', () => {
    // Column visibility is container-led but md:w-[...] widths are viewport
    // media queries, so a 544-767px band showed every column with no widths and
    // split them equally (a Kubernetes node name got 83px). The shared shell
    // keeps the 30% name anchor there.
    const band = appStylesSource.slice(appStylesSource.indexOf('@media (width < 768px)'));
    expect(band).toMatch(
      /^@media \(width < 768px\) \{\s*@container \(min-width: 34rem\) \{\s*\.table-scroll-shell > \.table-fixed\.platform-table th\.platform-table-name-column \{\s*width: 30%;/,
    );
    // A table that needs unequal room in the band declares each header's
    // width, and the shell applies it only there (after the name anchor).
    expect(band).toMatch(
      /width: 30%;\s*\}[\s\S]*?th\[style\*='--platform-table-band-width'\] \{\s*width: var\(--platform-table-band-width\);/,
    );
    // A header whose full label outgrows its band column keeps its compact
    // label there; these rules sit after the 34rem label switch so they win.
    expect(band).toMatch(
      /--platform-table-band-width\);\s*\}[\s\S]*?\.platform-table-label-compact\.platform-table-label-band-compact \{\s*display: inline;\s*\}\s*\.platform-table-label-full\.platform-table-label-band-compact \{\s*display: none;/,
    );
    const labelSwitch = appStylesSource.indexOf(
      '@container (min-width: 34rem) {\n    .platform-table-label-compact',
    );
    expect(labelSwitch).toBeGreaterThan(0);
    expect(appStylesSource.indexOf('.platform-table-label-band-compact')).toBeGreaterThan(
      labelSwitch,
    );
  });

  it('draws the shared select arrow that appearance-none removes', () => {
    // FormSelect's default chrome sets appearance-none to match text inputs;
    // without this utility every default select read as a text field.
    expect(appStylesSource).toMatch(
      /@utility form-select-arrow \{\s*background-image:\s*linear-gradient\(45deg, transparent 50%, var\(--color-text-muted\) 50%\)/,
    );
    // Forced-colors mode drops background images, so the native arrow returns.
    expect(appStylesSource).toMatch(
      /@media \(forced-colors: active\) \{\s*appearance: auto !important;\s*background-image: none;/,
    );
  });

  it('keeps compact phone settings spacing scoped to the settings content shell', () => {
    expect(appStylesSource).toContain('@media (max-width: 39.999rem)');
    expect(appStylesSource).toContain(
      '[data-settings-content-body] > .space-y-6 > :not([hidden]) ~ :not([hidden])',
    );
    expect(appStylesSource).toContain('margin-top: 0.75rem;');
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

  it('keeps summary row emphasis to the shared active-row rules', () => {
    expect(appStylesSource).toContain("tr[data-summary-row-active='true'] > td");
    // Group-member emphasis went with the Storage group pin; no table sets it.
    expect(appStylesSource).not.toContain('data-summary-group-member-active');
    expect(appStylesSource).not.toContain('--color-summary-group-member-');
  });

  it('keeps infrastructure platforms marked as dense data surfaces', () => {
    platformSurfaceSources.forEach((source) => {
      expect(source).toMatch(/data-testid="[^"]+-page" class="pulse-wide-data-surface /);
    });
  });

  it('mounts every platform page, the workspace entry and Patrol under the app shell', () => {
    // Retired aggregate routes are gone from the source; these are the pages a
    // user reaches. No render test mounts App, so the registrations are pinned.
    for (const route of [
      '<Route path="/pricing" component={PricingHandoffPage} />',
      '<Route path="/login" component={RuntimeHomePage} />',
      '<Route path="/" component={RuntimeHomePage} />',
      '<Route path="/infrastructure" component={RuntimeHomePage} />',
      '<Route path={PROXMOX_PATH} component={ProxmoxPage} />',
      '<Route path={`${PROXMOX_PATH}/*`} component={ProxmoxPage} />',
      '<Route path={DOCKER_PATH} component={DockerPage} />',
      '<Route path={KUBERNETES_PATH} component={KubernetesPage} />',
      '<Route path={TRUENAS_PATH} component={TrueNASPage} />',
      '<Route path={VMWARE_PATH} component={VmwarePage} />',
      '<Route path={STANDALONE_PATH} component={StandalonePage} />',
      '<Route path={`${STANDALONE_PATH}/*`} component={StandalonePage} />',
      '<Route path={`${ROOT_PATROL_PATH}/*`} component={AIIntelligencePage} />',
    ]) {
      expect(appSource).toContain(route);
    }
  });

  it('Issue1650 keeps kiosk-scoped sessions off settings chrome in the shell', () => {
    // Escape handling and the blocked-route redirect are rendered in
    // AppLayout.test.tsx. Banners in the shell deep-link into settings, so they
    // follow the same rule; no render test mounts App itself.
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

  it('keeps the compact organization audit explicitly entitled before it creates an organization', () => {
    const audit = readFileSync(
      join(integrationTestsDir, '05-settings-mobile-audit.spec.ts'),
      'utf8',
    );
    const preparation = audit.slice(
      audit.indexOf('const prepareOrganizationAuditFixture'),
      audit.indexOf('const test = base.extend'),
    );
    const activation = preparation.indexOf('await activateOfflineOrganization(page)');
    expect(activation).toBeGreaterThan(
      preparation.indexOf('await ensureSessionAuthenticated(page)'),
    );
    expect(activation).toBeLessThan(preparation.indexOf('await createOrg(page'));
    expect(activation).toBeGreaterThan(0);
    expect(preparation).toContain('process.env.PULSE_E2E_OFFLINE_ACTIVATION_KEY');
  });

  it('owns mounted platform pages inside the confirmed-sign-out authentication gate', () => {
    // This gate disposes Workloads metadata without relying on provider
    // navigation. Runtime tests exercise successful and partial cleanup;
    // the metadata lifetime tests exercise late results after disposal.
    expect(appSource).toContain('when={!runtime.needsAuth()}');
  });

  it('keeps the shell stylesheet contract for app chrome, brand motion and platform tables', () => {
    expect(appStylesSource).toContain('@utility pb-safe-or-14');
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
    expect(appStylesSource).toContain('th.platform-table-mobile-w-38');
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
  });

  it('keeps alert hydration separate from tenant resource admission', () => {
    const start = appRuntimeStateSource.indexOf('const runtimeStateResolved');
    const predicate = appRuntimeStateSource.slice(
      start,
      appRuntimeStateSource.indexOf('const state =', start),
    );
    expect(start).toBeGreaterThan(-1);
    expect(predicate).toContain('resourceSnapshotReceived()');
    expect(predicate).not.toContain('initialDataReceived()');
    expect(predicate).not.toContain('hasRuntimeStatePayload');
    const storeSource = readFileSync(join(process.cwd(), 'src/stores/websocket.ts'), 'utf8');
    const reset = storeSource.slice(storeSource.indexOf('setResourceSnapshotReceived(false)'));
    expect(reset).toContain("setActiveAlertsHydrationStatus('pending')");
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
  });
});

function collectFrontendSources(dir: string): Array<{ path: string; source: string }> {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const entryPath = join(dir, entry.name);
    if (entry.isDirectory()) {
      return collectFrontendSources(entryPath);
    }
    // Only runtime sources matter; a test may quote an old shape in order to
    // assert against it, as the guards below do.
    if (!/\.(css|ts|tsx)$/.test(entry.name)) {
      return [];
    }
    if (entryPath.includes('__tests__') || /\.(test|spec)\./.test(entry.name)) {
      return [];
    }
    return [{ path: entryPath, source: readFileSync(entryPath, 'utf8') }];
  });
}

describe('mobile bottom navigation clearance', () => {
  const NAV_HEIGHT_VARIABLE = '--pulse-mobile-nav-height';
  // Matches the shape every drifted copy used, spaced or not. The lookbehind
  // keeps the declared 2.5rem fallback from matching its own guard.
  const HARDCODED_BAR_HEIGHT = /(?<![\d.])5rem\s*\+\s*env\(\s*safe-area-inset-bottom/;

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

describe('page background utility', () => {
  // Tailwind 4 emits nothing for a colour it does not know, so `bg-base`
  // rendered transparent without any error: command blocks, the alert
  // selection bar and segmented-control tracks all lost their backgrounds.
  // A theme colour named `base` is not the fix either, because it also gives
  // the `text-base` font-size utility a colour and paints text in the page
  // colour. The page backdrop is the bg-page utility.
  const DEAD_BASE_BACKGROUND = /(?<![\w-])bg-base(?![\w-])/;
  // A comment may name the old class to explain a fix, as toggleModel does.
  const COMMENTS = /\/\*[\s\S]*?\*\/|(^|[^:])\/\/.*$/gm;

  it('declares the page backdrop as a utility, never as a colour named base', () => {
    expect(appStylesSource).toMatch(
      /@utility bg-page \{\s*background-color: var\(--color-bg-base\);\s*\}/,
    );
    expect(appStylesSource).not.toMatch(/--color-base\s*:/);
  });

  it('keeps every runtime class off the bg-base utility that renders nothing', () => {
    const offenders = collectFrontendSources(join(process.cwd(), 'src'))
      .filter(({ source }) => DEAD_BASE_BACKGROUND.test(source.replace(COMMENTS, '$1')))
      .map(({ path }) => path.slice(process.cwd().length + 1));

    expect(offenders).toEqual([]);
  });
});

describe('Docker phone update reflow boundary', () => {
  it('scopes label wrapping to narrow Docker cells without restoring a nested scrollport', () => {
    const phoneStyles = appStylesSource.slice(appStylesSource.indexOf('/* Phone update states'));
    expect(phoneStyles).toContain('@container (max-width: 33.999rem)');
    expect(phoneStyles).toMatch(/\.docker-container-update-cell\s*\{\s*padding-inline: 0.125rem;/);
    expect(phoneStyles).toContain('max-width: 100%');
    expect(phoneStyles).toContain('white-space: normal');
    expect(phoneStyles).toContain('overflow-wrap: anywhere');
    expect(phoneStyles).toContain('flex-wrap: wrap');
    expect(phoneStyles).not.toMatch(/overflow(?:-x|-y)?:\s*(auto|scroll)/);
    expect(appStylesSource).toMatch(
      /\.table-scroll-shell\.table-scroll-shell-phone-page\s*\{\s*overflow: clip;/,
    );
  });
});

describe('Alert History column widths', () => {
  // Hidden tracks are collapsed and zero-width so the table is always the
  // card's width: WebKit keeps a collapsed column's width in a fixed layout,
  // so widening the table to compensate made Safari scroll sideways. The base
  // density (an unmeasured card under 800px) gives every visible track a
  // percentage summing to 100%. From an 800px card the fixed-content columns
  // take exact pixels, resource and node a plain share of the card, and
  // message has no width so it takes the rest.
  const columns = [
    'timestamp',
    'resource',
    'type',
    'severity',
    'message',
    'duration',
    'status',
    'node',
    'actions',
  ] as const;
  type Column = (typeof columns)[number];
  const densities: Array<{ name: string; visible: readonly Column[] }> = [
    {
      name: 'base',
      visible: ['timestamp', 'resource', 'severity', 'message', 'status', 'actions'],
    },
    {
      name: '50rem',
      visible: ['timestamp', 'resource', 'severity', 'message', 'duration', 'status', 'actions'],
    },
    { name: '64rem', visible: columns },
    { name: '80rem', visible: columns },
  ];
  const blocks = [
    ...appStylesSource.matchAll(
      /table\.alert-history-responsive-table \{\s*((?:--alert-history-[a-z]+-width: [^;]+;\s*)+)width: ([^;]+);/g,
    ),
  ];
  const parse = (block: RegExpMatchArray) =>
    Object.fromEntries(
      [...block[1].matchAll(/--alert-history-([a-z]+)-width: ([^;]+);/g)].map((match) => [
        match[1],
        match[2].trim(),
      ]),
    ) as Record<Column, string>;
  const percent = (value: string) => {
    const match = /^(\d+(?:\.\d+)?)%$/.exec(value);
    if (!match) throw new Error(`expected a percentage, got ${value}`);
    return Number(match[1]);
  };
  const pixels = (value: string) => {
    const match = /^(\d+(?:\.\d+)?)px$/.exec(value);
    if (!match) throw new Error(`expected pixels, got ${value}`);
    return Number(match[1]);
  };

  // Each density is sized for the narrowest card it serves, from the natural
  // widths measured in a real browser (padding included): "08:27 AM" in a 14px
  // monospace, the Severity/Duration/Resource headers, the compact 11px
  // "moved to agent" status badge (108px), and the three 24px icon row actions
  // on one line (100px; worded buttons needed 177px). The base density only
  // needs the actions' width at its percentage.
  // Message, the column that says what happened, may never fall under 18% of
  // the card. Type and Node, which appear from 1024px, get a floor too: the
  // longest type label, "Container Health", needs 119px with its padding.
  const wideNeeds = {
    timestamp: 84,
    resource: 93,
    severity: 86,
    duration: 91,
    status: 108,
    actions: 100,
  } as const;
  const pixelNeeds: Array<{
    name: string;
    cardWidth: number;
    needs: Partial<Record<Column, number>>;
  }> = [
    {
      name: 'base',
      cardWidth: 600,
      needs: { timestamp: 84, resource: 93, severity: 86, status: 108, actions: 100 },
    },
    { name: '50rem', cardWidth: 800, needs: wideNeeds },
    { name: '64rem', cardWidth: 1024, needs: { ...wideNeeds, type: 120, node: 68 } },
    { name: '80rem', cardWidth: 1280, needs: { ...wideNeeds, type: 120, node: 68 } },
  ];

  // The rendered width of each visible track at a given card width.
  const trackWidths = (index: number, cardWidth: number): Record<string, number> => {
    const widths = parse(blocks[index]);
    const visible = densities[index].visible;
    if (index === 0) {
      return Object.fromEntries(
        visible.map((column) => [column, (percent(widths[column]) / 100) * cardWidth]),
      );
    }
    const tableWidth = cardWidth;
    const resolved: Record<string, number> = {};
    for (const column of visible) {
      if (column === 'message') continue;
      resolved[column] = widths[column].endsWith('%')
        ? (percent(widths[column]) / 100) * tableWidth
        : pixels(widths[column]);
    }
    resolved.message = cardWidth - Object.values(resolved).reduce((sum, width) => sum + width, 0);
    return resolved;
  };

  it('declares every track width at each density', () => {
    expect(blocks).toHaveLength(densities.length);
    for (const column of columns) {
      expect(appStylesSource).toContain(
        `col.alert-history-${column}-track {\n    width: var(--alert-history-${column}-width);`,
      );
    }
  });

  it('keeps the table at the card width with zero-width hidden tracks', () => {
    blocks.forEach((block, index) => {
      const widths = parse(block);
      const { name, visible } = densities[index];
      expect(block[2].trim(), `${name}: table width`).toBe('100%');
      for (const column of columns.filter((candidate) => !visible.includes(candidate))) {
        expect(widths[column], `${name}: hidden ${column}`).toMatch(/^0(?:px|%)$/);
      }
    });
  });

  it('fills the narrow card exactly with the base density percentages', () => {
    const widths = parse(blocks[0]);
    const visibleSum = densities[0].visible.reduce(
      (sum, column) => sum + percent(widths[column]),
      0,
    );
    expect(visibleSum).toBeCloseTo(100, 6);
  });

  it('lets message take what the fixed tracks leave from an 800px card', () => {
    blocks.slice(1).forEach((block, offset) => {
      const index = offset + 1;
      const widths = parse(block);
      expect(widths.message, `${densities[index].name}: message`).toBe('auto');
      // Content with a fixed size takes pixels. Resource and node, which hold
      // truncating names, may take a plain share of the card; never a
      // max() or calc() on a column, which Chromium lays out as auto.
      for (const column of columns) {
        if (column === 'message') continue;
        const shared = column === 'resource' || column === 'node';
        const read = () =>
          shared && widths[column].endsWith('%') ? percent(widths[column]) : pixels(widths[column]);
        expect(read, `${densities[index].name}: ${column}`).not.toThrow();
      }
    });
  });

  it('declares each density block inside its 50rem, 64rem and 80rem container query', () => {
    const enclosing = blocks.slice(1).map((block) => {
      const start = block.index as number;
      const queries = [
        ...appStylesSource.slice(0, start).matchAll(/@container \(min-width: ([\d.]+)rem\) \{/g),
      ];
      const query = queries[queries.length - 1];
      // Walk the braces from the query to the end of its body: the density
      // block only counts if it sits before that closing brace.
      let depth = 0;
      let close = -1;
      for (
        let index = (query.index as number) + query[0].length - 1;
        index < appStylesSource.length;
        index += 1
      ) {
        const char = appStylesSource[index];
        if (char === '{') depth += 1;
        if (char === '}') depth -= 1;
        if (depth === 0) {
          close = index;
          break;
        }
      }
      return { rem: Number(query[1]), enclosed: start + block[0].length <= close };
    });
    expect(enclosing).toEqual([
      { rem: 50, enclosed: true },
      { rem: 64, enclosed: true },
      { rem: 80, enclosed: true },
    ]);
  });

  it('gives every column the pixels its content needs at the narrowest card of each density', () => {
    pixelNeeds.forEach(({ name, cardWidth, needs }, index) => {
      const widths = trackWidths(index, cardWidth);
      for (const [column, need] of Object.entries(needs)) {
        expect(widths[column], `${name}: ${column} track at ${cardWidth}px`).toBeGreaterThanOrEqual(
          need - 0.5,
        );
      }
    });
  });

  it('keeps Message at least 18% of the card at the narrowest card of every density', () => {
    pixelNeeds.forEach(({ name, cardWidth }, index) => {
      expect(
        trackWidths(index, cardWidth).message / cardWidth,
        `${name}: message share`,
      ).toBeGreaterThanOrEqual(0.18);
    });
  });

  // Percentages handed most of every extra pixel to a badge and three buttons
  // that already fit: at a 982px card message held 182px. Pixel tracks give
  // the extra width to message.
  it('gives message most of the extra width of a wider card', () => {
    for (const [index, narrow, wide] of [
      [1, 800, 1023],
      [2, 1024, 1279],
      [3, 1280, 1600],
    ] as const) {
      const growth = trackWidths(index, wide).message - trackWidths(index, narrow).message;
      expect(growth, densities[index].name).toBeGreaterThanOrEqual(0.75 * (wide - narrow));
    }
    expect(trackWidths(1, 982).message).toBeGreaterThan(300);
  });
});
