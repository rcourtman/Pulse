import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { ReportingPanel } from '../ReportingPanel';
import type { JSX } from 'solid-js';
import { getPublicPricingUrl } from '@/utils/pricingHandoff';

const useReportingPanelStateMock = vi.fn();

vi.mock('@/components/Settings/useReportingPanelState', () => ({
  useReportingPanelState: () => useReportingPanelStateMock(),
}));

vi.mock('../ResourcePicker', () => ({
  ResourcePicker: (_props: {
    maxSelection?: number;
    selected: () => unknown[];
    onSelectionChange: (items: unknown[]) => void;
  }): JSX.Element => <div>Mock Resource Picker</div>,
}));

const baseCatalog = {
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
    filenameDateStyle: 'utc_yyyymmdd',
    columns: [],
  },
};

function buildState(overrides: Record<string, unknown> = {}) {
  return {
    exportingInventory: () => false,
    format: () => 'pdf' as const,
    handleExportVMInventory: vi.fn(),
    generating: () => false,
    handleGenerate: vi.fn(),
    isLocked: () => false,
    isReportingEnabled: () => true,
    metricType: () => '',
    range: () => '24h',
    reportingCatalog: () => baseCatalog,
    reportingCatalogError: () => '',
    reportingCatalogLoading: () => false,
    reloadReportingCatalog: vi.fn(),
    selectedResources: () => [],
    setFormat: vi.fn(),
    setMetricType: vi.fn(),
    setRange: vi.fn(),
    setSelectedResources: vi.fn(),
    setTitle: vi.fn(),
    showUpgradePrompts: () => true,
    title: () => '',
    upgradeDestination: () => ({
      href: getPublicPricingUrl('advanced_reporting'),
      external: true,
    }),
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

  it('uses catalog-owned locked teaser copy for the paywalled shell', () => {
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        isLocked: () => true,
        isReportingEnabled: () => false,
        reportingCatalog: () => ({
          ...baseCatalog,
          lockedState: {
            title: 'Reporting for Paid Workflows',
            description: 'Catalog-owned locked teaser copy',
          },
        }),
      }),
    );

    render(() => <ReportingPanel />);

    expect(screen.getByText('Reporting for Paid Workflows')).toBeInTheDocument();
    expect(screen.getByText('Catalog-owned locked teaser copy')).toBeInTheDocument();
  });

  it('uses neutral locked copy when upgrade prompts are hidden', () => {
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        isLocked: () => true,
        isReportingEnabled: () => false,
        showUpgradePrompts: () => false,
      }),
    );

    render(() => <ReportingPanel />);

    expect(screen.getByText('Advanced Reporting unavailable')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Reporting is locked for this session. The report builder appears when advanced reporting is available.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Advanced Reporting (Pro)')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();
  });

  it('uses catalog-owned guidance copy for the enabled explainer callout', () => {
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        reportingCatalog: () => ({
          ...baseCatalog,
          guidance: {
            title: 'Inventory Versus Trends',
            description: 'Catalog-owned explainer for when to use each reporting surface.',
          },
        }),
      }),
    );

    render(() => <ReportingPanel />);

    expect(screen.getByText('Inventory Versus Trends')).toBeInTheDocument();
    expect(
      screen.getByText('Catalog-owned explainer for when to use each reporting surface.'),
    ).toBeInTheDocument();
  });

  it('renders performance reports when the catalog only exposes the legacy report surface', () => {
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        reportingCatalog: () => ({
          ...baseCatalog,
          guidance: {
            title: 'Advanced Insights',
            description: 'Performance reports remain available on older backends.',
          },
          vmInventoryExport: null,
        }),
      }),
    );

    render(() => <ReportingPanel />);

    expect(screen.getByText('Performance Reports')).toBeInTheDocument();
    expect(screen.queryByText('VM Inventory Export')).not.toBeInTheDocument();
    expect(
      screen.getByText('Performance reports remain available on older backends.'),
    ).toBeInTheDocument();
  });

  it('shows a generic loading shell before the reporting catalog arrives', () => {
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        isReportingEnabled: () => false,
        reportingCatalog: () => null,
        reportingCatalogLoading: () => true,
      }),
    );

    render(() => <ReportingPanel />);

    expect(screen.getByText('Reporting')).toBeInTheDocument();
    expect(screen.getAllByText('Loading reporting surfaces...').length).toBeGreaterThan(0);
  });

  it('offers a retry action when the reporting catalog fails to load', () => {
    const reloadReportingCatalog = vi.fn();
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        isReportingEnabled: () => false,
        reportingCatalog: () => null,
        reportingCatalogError: () => 'Reporting unavailable',
        reloadReportingCatalog,
      }),
    );

    render(() => <ReportingPanel />);

    screen.getByRole('button', { name: 'Retry' }).click();
    expect(reloadReportingCatalog).toHaveBeenCalledOnce();
  });

  it('routes reporting command actions through shared button variants', () => {
    const handleGenerate = vi.fn();
    const handleExportVMInventory = vi.fn();
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        handleExportVMInventory,
        handleGenerate,
      }),
    );

    render(() => <ReportingPanel />);

    const generateButton = screen.getByRole('button', { name: 'Generate Report' });
    expect(generateButton).toHaveClass('bg-blue-600');
    expect(generateButton).toHaveClass('px-6');
    generateButton.click();
    expect(handleGenerate).toHaveBeenCalledOnce();

    const exportButton = screen.getByRole('button', { name: 'Export VM Inventory' });
    expect(exportButton).toHaveClass('bg-emerald-600');
    expect(exportButton).toHaveClass('px-6');
    exportButton.click();
    expect(handleExportVMInventory).toHaveBeenCalledOnce();
  });
});
