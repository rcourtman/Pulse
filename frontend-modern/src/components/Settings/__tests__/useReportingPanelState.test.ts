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
    title: 'Advanced Reporting (Pro)',
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

describe('useReportingPanelState', () => {
  let useReportingPanelState: UseReportingPanelStateModule['useReportingPanelState'];
  let apiFetchMock: ReturnType<typeof vi.fn>;
  let hasReportingFeature: boolean;
  let loadRuntimeLicenseStatusMock: ReturnType<typeof vi.fn>;
  let loadCommercialLicenseStatusMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vi.resetModules();

    apiFetchMock = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify(catalogPayload), { status: 200 }));
    hasReportingFeature = true;
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

    vi.doMock('@/utils/trialStartAction', () => ({
      runStartProTrialAction: vi.fn(),
    }));

    vi.doMock('@/utils/upgradeMetrics', () => ({
      trackPaywallViewed: vi.fn(),
    }));

    vi.doMock('@/stores/license', () => ({
      hasFeature: vi.fn((feature: string) => feature === 'advanced_reporting' && hasReportingFeature),
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
    expect(loadCommercialLicenseStatusMock).toHaveBeenCalledOnce();
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
    expect(hookState.reportingCatalog()?.lockedState.title).toBe('Advanced Reporting (Pro)');
    expect(hookState.isLocked()).toBe(true);

    dispose();
  });

  it('loads the reporting catalog before license readiness settles', async () => {
    vi.resetModules();

    apiFetchMock = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify(catalogPayload), { status: 200 }));
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

    vi.doMock('@/utils/trialStartAction', () => ({
      runStartProTrialAction: vi.fn(),
    }));

    vi.doMock('@/utils/upgradeMetrics', () => ({
      trackPaywallViewed: vi.fn(),
    }));

    vi.doMock('@/stores/license', () => ({
      hasFeature: vi.fn(() => false),
      runtimeCapabilitiesLoaded: vi.fn(() => false),
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

    expect(loadRuntimeLicenseStatusMock).toHaveBeenCalledOnce();
    expect(loadCommercialLicenseStatusMock).toHaveBeenCalledOnce();
    expect(apiFetchMock).toHaveBeenCalledWith('/api/admin/reports/catalog');
    expect(hookState.reportingCatalog()?.title).toBe('Detailed Reporting');
    expect(hookState.isLocked()).toBe(false);
    expect(hookState.isReportingEnabled()).toBe(false);

    dispose();
  });

  it('allows retrying the reporting catalog fetch after an initial failure', async () => {
    vi.resetModules();

    apiFetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response('temporary failure', { status: 500 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(catalogPayload), { status: 200 }));

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

    vi.doMock('@/utils/trialStartAction', () => ({
      runStartProTrialAction: vi.fn(),
    }));

    vi.doMock('@/utils/upgradeMetrics', () => ({
      trackPaywallViewed: vi.fn(),
    }));

    vi.doMock('@/stores/license', () => ({
      hasFeature: vi.fn((feature: string) => feature === 'advanced_reporting' && hasReportingFeature),
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

    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    expect(hookState.reportingCatalog()?.title).toBe('Detailed Reporting');

    dispose();
  });

  it('extracts structured API error messages for reporting catalog failures', async () => {
    vi.resetModules();

    apiFetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'Catalog is unavailable right now' }), { status: 503 }),
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

    vi.doMock('@/utils/trialStartAction', () => ({
      runStartProTrialAction: vi.fn(),
    }));

    vi.doMock('@/utils/upgradeMetrics', () => ({
      trackPaywallViewed: vi.fn(),
    }));

    vi.doMock('@/stores/license', () => ({
      hasFeature: vi.fn((feature: string) => feature === 'advanced_reporting' && hasReportingFeature),
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

    apiFetchMock = vi.fn().mockResolvedValueOnce(new Response('404 page not found', { status: 404 }));

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

    vi.doMock('@/utils/trialStartAction', () => ({
      runStartProTrialAction: vi.fn(),
    }));

    vi.doMock('@/utils/upgradeMetrics', () => ({
      trackPaywallViewed: vi.fn(),
    }));

    vi.doMock('@/stores/license', () => ({
      hasFeature: vi.fn((feature: string) => feature === 'advanced_reporting' && hasReportingFeature),
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
    expect(hookState.reportingCatalog()?.performanceReport.singleResourceEndpoint).toBe('/api/reporting');
    expect(hookState.reportingCatalog()?.vmInventoryExport).toBeNull();

    dispose();
  });
});
