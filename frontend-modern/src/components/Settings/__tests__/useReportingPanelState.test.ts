import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { getPublicPricingUrl } from '@/utils/pricingHandoff';

type UseReportingPanelStateModule = typeof import('../useReportingPanelState');
type ReportingPanelState = ReturnType<UseReportingPanelStateModule['useReportingPanelState']>;

// The hook loads the catalog on mount and, once reporting is enabled, the
// schedules. Reading a fetch Response body takes a different number of
// microtasks on each Node release (8 on Node 20, 3 on Node 24), so tests wait
// for those loads to finish instead of flushing a fixed number of ticks.
const settleMountLoads = (hookState: ReportingPanelState) =>
  vi.waitFor(() => {
    expect(hookState.reportingCatalogLoading()).toBe(false);
    expect(hookState.reportingCatalog() !== null || hookState.reportingCatalogError() !== '').toBe(
      true,
    );
    expect(hookState.reportSchedulesLoading()).toBe(false);
  });

const catalogPayload = {
  id: 'advanced_reporting',
  title: 'Detailed Reporting',
  description: 'Canonical reporting surfaces',
  lockedState: {
    title: 'Advanced Reporting',
    description: 'Canonical locked reporting teaser',
  },
  guidance: {
    title: 'Advanced Insights',
    description: 'Catalog-owned reporting guidance',
  },
  performanceReport: {
    id: 'performance_reports',
    title: 'Performance Reports',
    description: 'Historical performance reporting',
    singleResourceEndpoint: '/api/admin/reports/generate',
    singleResourceMethod: 'POST',
    multiResourceEndpoint: '/api/admin/reports/generate-multi',
    singleFilenamePrefix: 'report',
    singleFilenameSubject: 'resource_id',
    multiFilenamePrefix: 'fleet-report',
    filenameDateStyle: 'utc_yyyymmdd',
    formats: [
      { value: 'pdf', label: 'PDF Report' },
      { value: 'csv', label: 'CSV Data' },
    ],
    defaultFormat: 'csv',
    ranges: [
      {
        key: '24h',
        label: 'Last 24 Hours',
        description: 'Daily review',
        windowHours: 24,
      },
      {
        key: '7d',
        label: 'Last 7 Days',
        description: 'Weekly review',
        windowHours: 168,
      },
    ],
    defaultRange: '7d',
    multiResourceMax: 50,
    supportsMetricFilter: true,
    supportsCustomTitle: true,
  },
  vmInventoryExport: {
    id: 'vm_inventory',
    title: 'VM Inventory Export',
    description: 'Current-state inventory',
    format: 'csv',
    exportEndpoint: '/api/admin/reports/inventory/vms/export',
    filenamePrefix: 'vm-inventory',
    filenameDateStyle: 'utc_yyyymmdd',
    columns: [
      {
        key: 'name',
        label: 'Name',
        description: 'VM display name.',
      },
    ],
  },
};

const schedulesPayload = { schedules: [] };

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), { status });
}

function buildCatalogAndSchedulesFetchMock(
  catalogResponse: () => Response = () => jsonResponse(catalogPayload),
) {
  return vi.fn((url: string) => {
    if (url === '/api/admin/reports/schedules') {
      return Promise.resolve(jsonResponse(schedulesPayload));
    }
    return Promise.resolve(catalogResponse());
  });
}

describe('useReportingPanelState', () => {
  let useReportingPanelState: UseReportingPanelStateModule['useReportingPanelState'];
  let apiFetchMock: ReturnType<typeof vi.fn>;
  let hasReportingFeature: boolean;
  let loadRuntimeLicenseStatusMock: ReturnType<typeof vi.fn>;
  let loadCommercialLicenseStatusMock: ReturnType<typeof vi.fn>;
  let aiEnabled: boolean | null;
  let showWarningMock: ReturnType<typeof vi.fn>;
  let showSuccessMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vi.resetModules();

    apiFetchMock = buildCatalogAndSchedulesFetchMock();
    hasReportingFeature = true;
    aiEnabled = null;
    showWarningMock = vi.fn();
    showSuccessMock = vi.fn();
    loadRuntimeLicenseStatusMock = vi.fn();
    loadCommercialLicenseStatusMock = vi.fn();

    vi.doMock('@/stores/aiAvailability', () => ({
      aiAvailabilityStore: {
        get enabled() {
          return aiEnabled;
        },
      },
    }));

    vi.doMock('@/utils/apiClient', async () => {
      const actual = await vi.importActual<typeof import('@/utils/apiClient')>('@/utils/apiClient');
      return {
        ...actual,
        apiFetch: apiFetchMock,
      };
    });

    vi.doMock('@/utils/toast', () => ({
      showSuccess: showSuccessMock,
      showWarning: showWarningMock,
    }));

    vi.doMock('@/stores/license', () => ({
      hasFeature: vi.fn(
        (feature: string) => feature === 'advanced_reporting' && hasReportingFeature,
      ),
      runtimeCapabilitiesLoaded: vi.fn(() => true),
      loadRuntimeCapabilities: loadRuntimeLicenseStatusMock,
    }));

    vi.doMock('@/stores/licenseCommercial', () => ({
      canOfferCommercialTrial: vi.fn(() => true),
      commercialPosture: vi.fn(() => ({ trial_eligible: true })),
      getUpgradeActionDestination: vi.fn((feature?: string) => ({
        href: getPublicPricingUrl(feature),
        external: true,
      })),
      getUpgradeActionUrlOrFallback: vi.fn((feature?: string) => getPublicPricingUrl(feature)),
      loadCommercialPosture: loadCommercialLicenseStatusMock,
      startProTrial: vi.fn(),
    }));

    vi.doMock('@/stores/sessionPresentationPolicy', () => ({
      presentationPolicyHidesUpgradePrompts: vi.fn(() => false),
    }));

    ({ useReportingPanelState } = await import('../useReportingPanelState'));
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.resetModules();
  });

  const mountHook = () => {
    let dispose = () => {};
    let hookState: ReturnType<UseReportingPanelStateModule['useReportingPanelState']>;

    createRoot((d) => {
      dispose = d;
      hookState = useReportingPanelState();
    });

    return { dispose, hookState: hookState! };
  };

  it('shows unavailable inventory guidance without downloading an empty fleet', async () => {
    const message =
      "VM inventory is unavailable. Check this organization's infrastructure connections before running again.";
    apiFetchMock.mockImplementation((url: string) => {
      if (url.startsWith('/api/admin/reports/inventory/vms/export?')) {
        return Promise.resolve(jsonResponse({ code: 'inventory_unavailable', error: message }, 503));
      }
      return Promise.resolve(
        jsonResponse(url.endsWith('/schedules') ? schedulesPayload : catalogPayload),
      );
    });
    const { hookState, dispose } = mountHook();
    await settleMountLoads(hookState);
    const createElement = vi.spyOn(document, 'createElement');
    try {
      await hookState.handleExportVMInventory();
      expect(showWarningMock).toHaveBeenCalledExactlyOnceWith(message);
      expect(showSuccessMock).not.toHaveBeenCalled();
      expect(createElement).not.toHaveBeenCalledWith('a');
      expect(hookState.exportingInventory()).toBe(false);
    } finally {
      createElement.mockRestore();
      dispose();
    }
  });

  it('seeds reporting selections from the backend catalog defaults', async () => {
    const { hookState, dispose } = mountHook();

    await settleMountLoads(hookState);

    expect(loadRuntimeLicenseStatusMock).toHaveBeenCalledOnce();
    expect(loadCommercialLicenseStatusMock).not.toHaveBeenCalled();
    expect(apiFetchMock).toHaveBeenCalledWith('/api/admin/reports/catalog');
    expect(hookState.reportingCatalog()?.performanceReport.defaultFormat).toBe('csv');
    expect(hookState.format()).toBe('csv');
    expect(hookState.range()).toBe('7d');

    dispose();
  });

  it('loads the reporting catalog even when the feature is locked', async () => {
    hasReportingFeature = false;
    const { hookState, dispose } = mountHook();

    await settleMountLoads(hookState);

    expect(apiFetchMock).toHaveBeenCalledWith('/api/admin/reports/catalog');
    expect(hookState.reportingCatalog()?.title).toBe('Detailed Reporting');
    expect(hookState.reportingCatalog()?.lockedState.title).toBe('Advanced Reporting');
    expect(hookState.isLocked()).toBe(true);

    dispose();
  });

  it('loads the reporting catalog before license readiness settles', async () => {
    vi.resetModules();

    apiFetchMock = buildCatalogAndSchedulesFetchMock();
    hasReportingFeature = false;
    loadRuntimeLicenseStatusMock = vi.fn();
    loadCommercialLicenseStatusMock = vi.fn();

    vi.doMock('@/utils/apiClient', async () => {
      const actual = await vi.importActual<typeof import('@/utils/apiClient')>('@/utils/apiClient');
      return {
        ...actual,
        apiFetch: apiFetchMock,
      };
    });

    vi.doMock('@/utils/toast', () => ({
      showSuccess: vi.fn(),
      showWarning: vi.fn(),
    }));

    vi.doMock('@/stores/license', () => ({
      hasFeature: vi.fn(() => false),
      runtimeCapabilitiesLoaded: vi.fn(() => false),
      loadRuntimeCapabilities: loadRuntimeLicenseStatusMock,
    }));

    vi.doMock('@/stores/licenseCommercial', () => ({
      canOfferCommercialTrial: vi.fn(() => true),
      commercialPosture: vi.fn(() => ({ trial_eligible: true })),
      getUpgradeActionDestination: vi.fn((feature?: string) => ({
        href: getPublicPricingUrl(feature),
        external: true,
      })),
      getUpgradeActionUrlOrFallback: vi.fn((feature?: string) => getPublicPricingUrl(feature)),
      loadCommercialPosture: loadCommercialLicenseStatusMock,
      startProTrial: vi.fn(),
    }));

    ({ useReportingPanelState } = await import('../useReportingPanelState'));

    const { hookState, dispose } = mountHook();

    await settleMountLoads(hookState);

    expect(loadRuntimeLicenseStatusMock).toHaveBeenCalledOnce();
    expect(loadCommercialLicenseStatusMock).not.toHaveBeenCalled();
    expect(apiFetchMock).toHaveBeenCalledWith('/api/admin/reports/catalog');
    expect(hookState.reportingCatalog()?.title).toBe('Detailed Reporting');
    expect(hookState.isLocked()).toBe(false);
    expect(hookState.isReportingEnabled()).toBe(false);

    dispose();
  });

  it('allows retrying the reporting catalog fetch after an initial failure', async () => {
    vi.resetModules();

    let catalogCalls = 0;
    apiFetchMock = vi.fn((url: string) => {
      if (url === '/api/admin/reports/schedules') {
        return Promise.resolve(jsonResponse(schedulesPayload));
      }
      catalogCalls += 1;
      if (catalogCalls === 1) {
        return Promise.resolve(new Response('temporary failure', { status: 500 }));
      }
      return Promise.resolve(jsonResponse(catalogPayload));
    });

    vi.doMock('@/utils/apiClient', async () => {
      const actual = await vi.importActual<typeof import('@/utils/apiClient')>('@/utils/apiClient');
      return {
        ...actual,
        apiFetch: apiFetchMock,
      };
    });

    vi.doMock('@/utils/toast', () => ({
      showSuccess: vi.fn(),
      showWarning: vi.fn(),
    }));

    vi.doMock('@/stores/license', () => ({
      hasFeature: vi.fn(
        (feature: string) => feature === 'advanced_reporting' && hasReportingFeature,
      ),
      runtimeCapabilitiesLoaded: vi.fn(() => true),
      loadRuntimeCapabilities: loadRuntimeLicenseStatusMock,
    }));

    vi.doMock('@/stores/licenseCommercial', () => ({
      commercialPosture: vi.fn(() => ({ trial_eligible: true })),
      getUpgradeActionDestination: vi.fn((feature?: string) => ({
        href: getPublicPricingUrl(feature),
        external: true,
      })),
      getUpgradeActionUrlOrFallback: vi.fn((feature?: string) => getPublicPricingUrl(feature)),
      loadCommercialPosture: loadCommercialLicenseStatusMock,
      startProTrial: vi.fn(),
    }));

    ({ useReportingPanelState } = await import('../useReportingPanelState'));
    const { hookState, dispose } = mountHook();

    await settleMountLoads(hookState);

    expect(hookState.reportingCatalog()).toBeNull();
    expect(hookState.reportingCatalogError()).toBe('temporary failure');

    hookState.reloadReportingCatalog();
    await settleMountLoads(hookState);

    expect(
      apiFetchMock.mock.calls.filter(([url]) => url === '/api/admin/reports/catalog'),
    ).toHaveLength(2);
    expect(hookState.reportingCatalog()?.title).toBe('Detailed Reporting');
    expect(hookState.reportSchedules()).toEqual([]);

    dispose();
  });

  it('extracts structured API error messages for reporting catalog failures', async () => {
    vi.resetModules();

    apiFetchMock = buildCatalogAndSchedulesFetchMock(
      () =>
        new Response(JSON.stringify({ error: 'Catalog is unavailable right now' }), {
          status: 503,
        }),
    );

    vi.doMock('@/utils/apiClient', async () => {
      const actual = await vi.importActual<typeof import('@/utils/apiClient')>('@/utils/apiClient');
      return {
        ...actual,
        apiFetch: apiFetchMock,
      };
    });

    vi.doMock('@/utils/toast', () => ({
      showSuccess: vi.fn(),
      showWarning: vi.fn(),
    }));

    vi.doMock('@/stores/license', () => ({
      hasFeature: vi.fn(
        (feature: string) => feature === 'advanced_reporting' && hasReportingFeature,
      ),
      runtimeCapabilitiesLoaded: vi.fn(() => true),
      loadRuntimeCapabilities: loadRuntimeLicenseStatusMock,
    }));

    vi.doMock('@/stores/licenseCommercial', () => ({
      commercialPosture: vi.fn(() => ({ trial_eligible: true })),
      getUpgradeActionDestination: vi.fn((feature?: string) => ({
        href: getPublicPricingUrl(feature),
        external: true,
      })),
      getUpgradeActionUrlOrFallback: vi.fn((feature?: string) => getPublicPricingUrl(feature)),
      loadCommercialPosture: loadCommercialLicenseStatusMock,
      startProTrial: vi.fn(),
    }));

    ({ useReportingPanelState } = await import('../useReportingPanelState'));
    const { hookState, dispose } = mountHook();

    await settleMountLoads(hookState);

    expect(hookState.reportingCatalog()).toBeNull();
    expect(hookState.reportingCatalogError()).toBe('Catalog is unavailable right now');

    dispose();
  });

  it('falls back to the legacy reporting transport when the catalog route is missing', async () => {
    vi.resetModules();

    apiFetchMock = buildCatalogAndSchedulesFetchMock(
      () => new Response('404 page not found', { status: 404 }),
    );

    vi.doMock('@/utils/apiClient', async () => {
      const actual = await vi.importActual<typeof import('@/utils/apiClient')>('@/utils/apiClient');
      return {
        ...actual,
        apiFetch: apiFetchMock,
      };
    });

    vi.doMock('@/utils/toast', () => ({
      showSuccess: vi.fn(),
      showWarning: vi.fn(),
    }));

    vi.doMock('@/stores/license', () => ({
      hasFeature: vi.fn(
        (feature: string) => feature === 'advanced_reporting' && hasReportingFeature,
      ),
      runtimeCapabilitiesLoaded: vi.fn(() => true),
      loadRuntimeCapabilities: loadRuntimeLicenseStatusMock,
    }));

    vi.doMock('@/stores/licenseCommercial', () => ({
      commercialPosture: vi.fn(() => ({ trial_eligible: true })),
      getUpgradeActionDestination: vi.fn((feature?: string) => ({
        href: getPublicPricingUrl(feature),
        external: true,
      })),
      getUpgradeActionUrlOrFallback: vi.fn((feature?: string) => getPublicPricingUrl(feature)),
      loadCommercialPosture: loadCommercialLicenseStatusMock,
      startProTrial: vi.fn(),
    }));

    ({ useReportingPanelState } = await import('../useReportingPanelState'));
    const { hookState, dispose } = mountHook();

    await settleMountLoads(hookState);

    expect(hookState.reportingCatalogError()).toBe('');
    expect(hookState.reportingCatalog()?.performanceReport.singleResourceEndpoint).toBe(
      '/api/reporting',
    );
    expect(hookState.reportingCatalog()?.vmInventoryExport).toBeNull();

    dispose();
  });

  it('offers the Patrol weekly summary only while the session reports Patrol can run', async () => {
    aiEnabled = null;
    const first = mountHook();
    await settleMountLoads(first.hookState);
    expect(first.hookState.patrolDigestAvailable()).toBe(false);
    first.dispose();

    aiEnabled = true;
    const second = mountHook();
    await settleMountLoads(second.hookState);
    expect(second.hookState.patrolDigestAvailable()).toBe(true);
    second.dispose();
  });

  it('pins a Patrol weekly summary to a weekly email and drops any picked resources', async () => {
    const { hookState, dispose } = mountHook();
    await settleMountLoads(hookState);

    hookState.startCreateSchedule();
    hookState.updateScheduleForm({ cadenceType: 'monthly', deliveryMethod: 'disk' });
    hookState.setScheduleResources([{ id: 'vm-1', type: 'vm', name: 'vm-1' }]);

    hookState.setScheduleKind('patrol_digest');

    expect(hookState.scheduleForm()).toMatchObject({
      kind: 'patrol_digest',
      cadenceType: 'weekly',
      deliveryMethod: 'email',
      attach: false,
      saveToDisk: false,
    });
    expect(hookState.scheduleResources()).toEqual([]);

    hookState.setScheduleKind('resources');
    expect(hookState.scheduleForm()).toMatchObject({ kind: 'resources', attach: true });

    dispose();
  });

  it('posts a single-resource report as a JSON body through apiFetch', async () => {
    apiFetchMock.mockImplementation((url: string) => {
      if (url === '/api/admin/reports/generate') {
        return Promise.resolve(new Response('%PDF', { status: 200 }));
      }
      if (url === '/api/admin/reports/schedules') {
        return Promise.resolve(jsonResponse(schedulesPayload));
      }
      return Promise.resolve(jsonResponse(catalogPayload));
    });
    const { createObjectURL, revokeObjectURL } = URL;
    URL.createObjectURL = vi.fn(() => 'blob:report');
    URL.revokeObjectURL = vi.fn();
    try {
      const { hookState, dispose } = mountHook();
      await settleMountLoads(hookState);

      hookState.setSelectedResources([{ id: 'vm-1', type: 'vm', name: 'vm-a' }]);
      await hookState.handleGenerate();

      const generate = apiFetchMock.mock.calls.filter(([url]) =>
        String(url).startsWith('/api/admin/reports/generate'),
      );
      expect(generate).toHaveLength(1);
      expect(generate[0][0]).toBe('/api/admin/reports/generate');
      expect(generate[0][1]).toMatchObject({
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      expect(JSON.parse(String(generate[0][1].body))).toMatchObject({
        resourceType: 'vm',
        resourceId: 'vm-1',
        format: catalogPayload.performanceReport.defaultFormat,
      });
      expect(showWarningMock).not.toHaveBeenCalled();
      dispose();
    } finally {
      URL.createObjectURL = createObjectURL;
      URL.revokeObjectURL = revokeObjectURL;
    }
  });

  it('saves a Patrol weekly summary without a resource scope', async () => {
    const savedSchedule = {
      id: 'digest-1',
      name: 'Weekly Patrol',
      kind: 'patrol_digest',
      enabled: true,
      cadence: { type: 'weekly', weekday: 'monday', time: '09:00', timezone: 'UTC' },
      scope: { resources: [], tags: [] },
      format: 'email',
      delivery: { method: 'email', to: [], attach: false, save_to_disk: false },
    };
    let scheduleSaved = false;
    apiFetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/admin/reports/schedules' && init?.method === 'POST') {
        scheduleSaved = true;
        return Promise.resolve(jsonResponse(savedSchedule));
      }
      if (url === '/api/admin/reports/schedules') {
        return Promise.resolve(jsonResponse({ schedules: scheduleSaved ? [savedSchedule] : [] }));
      }
      return Promise.resolve(jsonResponse(catalogPayload));
    });

    const { hookState, dispose } = mountHook();
    await settleMountLoads(hookState);

    hookState.startCreateSchedule();
    hookState.updateScheduleForm({ name: 'Weekly Patrol', timezone: 'UTC' });
    hookState.setScheduleKind('patrol_digest');
    await hookState.saveReportSchedule();

    expect(showWarningMock).not.toHaveBeenCalled();
    const post = apiFetchMock.mock.calls.find(
      ([url, init]) => url === '/api/admin/reports/schedules' && init?.method === 'POST',
    );
    expect(post).toBeDefined();
    const body = JSON.parse(String(post![1].body));
    expect(body.kind).toBe('patrol_digest');
    expect(body.format).toBe('email');
    expect(body.scope).toEqual({ resources: [], tags: [] });
    expect(body.delivery).toMatchObject({ method: 'email', attach: false, save_to_disk: false });
    expect(hookState.reportSchedules()).toHaveLength(1);
    expect(hookState.reportSchedules()[0].kind).toBe('patrol_digest');
    expect(hookState.scheduleFormOpen()).toBe(false);

    dispose();
  });

  it('saves an inventory CSV schedule without leaking old resources or tags', async () => {
    apiFetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/admin/reports/schedules' && init?.method === 'POST') {
        return Promise.resolve(jsonResponse({ ...JSON.parse(String(init.body)), id: 'inventory' }));
      }
      if (url === '/api/admin/reports/schedules')
        return Promise.resolve(jsonResponse(schedulesPayload));
      return Promise.resolve(jsonResponse(catalogPayload));
    });
    const { hookState, dispose } = mountHook();
    await settleMountLoads(hookState);
    hookState.startCreateSchedule();
    hookState.updateScheduleForm({ name: 'Inventory', tagFilter: 'old-client' });
    hookState.setScheduleResources([{ id: 'vm-old', name: 'Old VM', type: 'vm' }]);
    hookState.setScheduleKind('vm_inventory');
    expect(hookState.scheduleResources()).toEqual([]);
    await hookState.saveReportSchedule();
    expect(showWarningMock).not.toHaveBeenCalled();
    const post = apiFetchMock.mock.calls.find(
      ([url, init]) => url === '/api/admin/reports/schedules' && init?.method === 'POST',
    );
    expect(post).toBeDefined();
    expect(JSON.parse(String(post![1].body))).toMatchObject({
      kind: 'vm_inventory',
      format: 'csv',
      scope: { resources: [], tags: [] },
    });
    dispose();
  });

  it('still requires a scope for a performance report schedule', async () => {
    const { hookState, dispose } = mountHook();
    await settleMountLoads(hookState);

    hookState.startCreateSchedule();
    hookState.updateScheduleForm({ name: 'Monthly clients' });
    await hookState.saveReportSchedule();

    expect(showWarningMock).toHaveBeenCalledWith('Select resources or enter at least one tag');
    expect(
      apiFetchMock.mock.calls.some(
        ([, init]) => (init as RequestInit | undefined)?.method === 'POST',
      ),
    ).toBe(false);

    dispose();
  });
  const deliverySchedule = (status: 'failed' | 'retrying' | 'unconfirmed' = 'failed') => ({
    id: 'weekly',
    name: 'Client report',
    enabled: true,
    cadence: { type: 'weekly' as const, weekday: 'monday', time: '09:00', timezone: 'UTC' },
    scope: { resources: [], tags: [] },
    format: 'csv' as const,
    delivery: { method: 'email' as const, to: [], attach: true, save_to_disk: true },
    last_run_status: status,
    run_window_end: '2026-10-17T09:00:00Z',
    last_error: 'Mail server rejected this report.',
  });

  it('uses the retry endpoint and retains the actionable failed response', async () => {
    const failed = deliverySchedule();
    apiFetchMock.mockImplementation((url: string) => {
      if (url.includes('/run')) return Promise.resolve(jsonResponse({ schedule: failed }, 500));
      if (url === '/api/admin/reports/schedules')
        return Promise.resolve(jsonResponse({ schedules: [failed] }));
      return Promise.resolve(jsonResponse(catalogPayload));
    });
    const { hookState, dispose } = mountHook();
    await settleMountLoads(hookState);
    await hookState.runReportScheduleNow(failed);
    expect(apiFetchMock).toHaveBeenCalledWith(
      '/api/admin/reports/schedules/weekly/run?retry=true',
      { method: 'POST' },
    );
    expect(showWarningMock).toHaveBeenCalledWith(failed.last_error);
    expect(hookState.reportSchedules()[0].last_run_status).toBe('failed');
    dispose();
  });

  it('requires deliberate confirmation before retrying uncertain acceptance', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const { hookState, dispose } = mountHook();
    await settleMountLoads(hookState);
    apiFetchMock.mockClear();
    await hookState.runReportScheduleNow(deliverySchedule('unconfirmed'));
    expect(confirm).toHaveBeenCalledWith(expect.stringContaining('risk of a duplicate'));
    expect(apiFetchMock).not.toHaveBeenCalled();
    confirm.mockRestore();
    dispose();
  });

  it('refreshes a pending delivery after returning to the page and cleans up', async () => {
    const pending = deliverySchedule('retrying');
    apiFetchMock.mockImplementation((url: string) =>
      Promise.resolve(
        jsonResponse(
          url === '/api/admin/reports/schedules' ? { schedules: [pending] } : catalogPayload,
        ),
      ),
    );
    const { hookState, dispose } = mountHook();
    await settleMountLoads(hookState);
    apiFetchMock.mockClear();
    document.dispatchEvent(new Event('visibilitychange'));
    await vi.waitFor(() =>
      expect(apiFetchMock).toHaveBeenCalledWith('/api/admin/reports/schedules'),
    );
    await settleMountLoads(hookState);
    dispose();
    apiFetchMock.mockClear();
    document.dispatchEvent(new Event('visibilitychange'));
    expect(apiFetchMock).not.toHaveBeenCalled();
  });

  it('keeps row identity through refreshes so open details and keyboard focus survive', async () => {
    let outcome = 'retrying';
    apiFetchMock.mockImplementation((url: string) =>
      Promise.resolve(
        jsonResponse(
          url === '/api/admin/reports/schedules'
            ? { schedules: [{ ...deliverySchedule('retrying'), last_run_status: outcome }] }
            : catalogPayload,
        ),
      ),
    );
    const { hookState, dispose } = mountHook();
    await settleMountLoads(hookState);
    const row = hookState.reportSchedules()[0];
    hookState.reloadReportSchedules();
    await settleMountLoads(hookState);
    expect(hookState.reportSchedules()[0]).toBe(row);
    outcome = 'ok';
    hookState.reloadReportSchedules();
    await vi.waitFor(() => expect(row.last_run_status).toBe('ok'));
    expect(hookState.reportSchedules()[0]).toBe(row);
    dispose();
  });

  it('does not send the transient configuration check back when toggling', async () => {
    const failed = { ...deliverySchedule(), delivery_check: { ready: false, message: 'Disabled' } };
    let scheduleEnabled = true;
    apiFetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (init?.method === 'PUT') {
        scheduleEnabled = false;
        expect(JSON.parse(String(init.body))).not.toHaveProperty('delivery_check');
        return Promise.resolve(jsonResponse({ ...failed, enabled: false }));
      }
      if (url === '/api/admin/reports/schedules')
        return Promise.resolve(
          jsonResponse({ schedules: [{ ...failed, enabled: scheduleEnabled }] }),
        );
      return Promise.resolve(jsonResponse(catalogPayload));
    });
    const { hookState, dispose } = mountHook();
    await settleMountLoads(hookState);
    await hookState.toggleReportSchedule(hookState.reportSchedules()[0]);
    expect(hookState.reportSchedules()[0].enabled).toBe(false);
    expect(showSuccessMock).toHaveBeenCalledWith('Report schedule disabled');
    dispose();
  });
});
