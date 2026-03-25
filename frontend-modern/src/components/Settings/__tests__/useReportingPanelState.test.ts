import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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
    multiFilenamePrefix: 'fleet-report',
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
  let loadLicenseStatusMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vi.resetModules();

    apiFetchMock = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify(catalogPayload), { status: 200 }));
    hasReportingFeature = true;
    loadLicenseStatusMock = vi.fn();

    vi.doMock('@/utils/apiClient', () => ({
      apiFetch: apiFetchMock,
    }));

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
      entitlements: vi.fn(() => ({ trial_eligible: true })),
      getUpgradeActionUrlOrFallback: vi.fn(() => '/pricing'),
      hasFeature: vi.fn((feature: string) => feature === 'advanced_reporting' && hasReportingFeature),
      licenseLoaded: vi.fn(() => true),
      loadLicenseStatus: loadLicenseStatusMock,
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

    expect(loadLicenseStatusMock).toHaveBeenCalledOnce();
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
});
