import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { getPublicPricingUrl } from '@/utils/pricingHandoff';

type UseReportingPanelStateModule = typeof import('../useReportingPanelState');

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
};

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
  catalogResponse: Response = jsonResponse(catalogPayload),
) {
  return vi.fn((url: string) => {
    if (url === '/api/admin/reports/schedules') {
      return Promise.resolve(jsonResponse(schedulesPayload));
    }
    return Promise.resolve(catalogResponse);
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

  beforeEach(async () => {
    vi.resetModules();

    apiFetchMock = buildCatalogAndSchedulesFetchMock();
    hasReportingFeature = true;
    aiEnabled = null;
    showWarningMock = vi.fn();
    loadRuntimeLicenseStatusMock = vi.fn();
    loadCommercialLicenseStatusMock = vi.fn();

    vi.doMock('@/stores/aiChat', () => ({
      aiChatStore: {
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
      showSuccess: vi.fn(),
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

  it('seeds reporting selections from the backend catalog defaults', async () => {
    const { hookState, dispose } = mountHook();

    await flushAsync();
    await flushAsync();

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

    await flushAsync();
    await flushAsync();

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

    await flushAsync();
    await flushAsync();

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

    await flushAsync();
    await flushAsync();

    expect(hookState.reportingCatalog()).toBeNull();
    expect(hookState.reportingCatalogError()).toBe('temporary failure');

    hookState.reloadReportingCatalog();
    await flushAsync();
    await flushAsync();

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

    await flushAsync();
    await flushAsync();

    expect(hookState.reportingCatalog()).toBeNull();
    expect(hookState.reportingCatalogError()).toBe('Catalog is unavailable right now');

    dispose();
  });

  it('falls back to the legacy reporting transport when the catalog route is missing', async () => {
    vi.resetModules();

    apiFetchMock = buildCatalogAndSchedulesFetchMock(
      new Response('404 page not found', { status: 404 }),
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

    await flushAsync();
    await flushAsync();

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
    await flushAsync();
    expect(first.hookState.patrolDigestAvailable()).toBe(false);
    first.dispose();

    aiEnabled = true;
    const second = mountHook();
    await flushAsync();
    expect(second.hookState.patrolDigestAvailable()).toBe(true);
    second.dispose();
  });

  it('pins a Patrol weekly summary to a weekly email and drops any picked resources', async () => {
    const { hookState, dispose } = mountHook();
    await flushAsync();

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
    apiFetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (url === '/api/admin/reports/schedules' && init?.method === 'POST') {
        return Promise.resolve(jsonResponse(savedSchedule));
      }
      if (url === '/api/admin/reports/schedules') {
        return Promise.resolve(jsonResponse(schedulesPayload));
      }
      return Promise.resolve(jsonResponse(catalogPayload));
    });

    const { hookState, dispose } = mountHook();
    await flushAsync();

    hookState.startCreateSchedule();
    hookState.updateScheduleForm({ name: 'Weekly Patrol', timezone: 'UTC' });
    hookState.setScheduleKind('patrol_digest');
    await hookState.saveReportSchedule();
    await flushAsync();

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

  it('still requires a scope for a performance report schedule', async () => {
    const { hookState, dispose } = mountHook();
    await flushAsync();

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
});
