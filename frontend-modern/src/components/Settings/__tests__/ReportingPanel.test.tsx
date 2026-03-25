import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { ReportingPanel } from '../ReportingPanel';
import type { JSX } from 'solid-js';

const useReportingPanelStateMock = vi.fn();

vi.mock('@/components/Settings/useReportingPanelState', () => ({
  useReportingPanelState: () => useReportingPanelStateMock(),
}));

vi.mock('../ResourcePicker', () => ({
  ResourcePicker: (_props: { maxSelection?: number; selected: () => unknown[]; onSelectionChange: (items: unknown[]) => void }): JSX.Element => (
    <div>Mock Resource Picker</div>
  ),
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackUpgradeClicked: vi.fn(),
}));

const baseCatalog = {
  id: 'advanced_reporting',
  title: 'Detailed Reporting',
  description: 'Canonical reporting surfaces',
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
    defaultFormat: 'pdf' as const,
    ranges: [
      {
        key: '24h',
        label: 'Last 24 Hours',
        description: 'Daily review',
        windowHours: 24,
      },
    ],
    defaultRange: '24h',
    multiResourceMax: 50,
    supportsMetricFilter: true,
    supportsCustomTitle: true,
  },
  vmInventoryExport: {
    id: 'vm_inventory',
    title: 'VM Inventory Export',
    description: 'Current-state inventory',
    format: 'csv' as const,
    exportEndpoint: '/api/admin/reports/inventory/vms/export',
    filenamePrefix: 'vm-inventory',
    columns: [],
  },
};

function buildState(overrides: Record<string, unknown> = {}) {
  return {
    canStartTrial: () => false,
    exportingInventory: () => false,
    format: () => 'pdf' as const,
    handleExportVMInventory: vi.fn(),
    generating: () => false,
    handleGenerate: vi.fn(),
    handleStartTrial: vi.fn(),
    isLocked: () => false,
    isReportingEnabled: () => true,
    metricType: () => '',
    range: () => '24h',
    reportingCatalog: () => baseCatalog,
    reportingCatalogError: () => '',
    reportingCatalogLoading: () => false,
    selectedResources: () => [],
    setFormat: vi.fn(),
    setMetricType: vi.fn(),
    setRange: vi.fn(),
    setSelectedResources: vi.fn(),
    setTitle: vi.fn(),
    startingTrial: () => false,
    title: () => '',
    upgradeActionUrl: () => '/pricing',
    ...overrides,
  };
}

describe('ReportingPanel', () => {
  beforeEach(() => {
    useReportingPanelStateMock.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('shows metric filter and custom title controls when the catalog supports them', () => {
    useReportingPanelStateMock.mockReturnValue(buildState());

    render(() => <ReportingPanel />);

    expect(screen.getByText('Metric Type (Optional)')).toBeInTheDocument();
    expect(screen.getByText('Report Title')).toBeInTheDocument();
  });

  it('hides unsupported optional controls from the reporting surface', () => {
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        reportingCatalog: () => ({
          ...baseCatalog,
          performanceReport: {
            ...baseCatalog.performanceReport,
            supportsMetricFilter: false,
            supportsCustomTitle: false,
          },
        }),
      }),
    );

    render(() => <ReportingPanel />);

    expect(screen.queryByLabelText('Metric Type (Optional)')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Report Title')).not.toBeInTheDocument();
  });
});
