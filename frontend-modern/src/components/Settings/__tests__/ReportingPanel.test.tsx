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
    closeScheduleForm: vi.fn(),
    deleteReportSchedule: vi.fn(),
    deletingScheduleID: () => '',
    exportingInventory: () => false,
    format: () => 'pdf' as const,
    handleExportVMInventory: vi.fn(),
    generating: () => false,
    handleGenerate: vi.fn(),
    isLocked: () => false,
    isReportingEnabled: () => true,
    metricType: () => '',
    patrolDigestAvailable: () => false,
    range: () => '24h',
    reportSchedules: () => [],
    reportSchedulesError: () => '',
    reportSchedulesLoading: () => false,
    reportingCatalog: () => baseCatalog,
    reportingCatalogError: () => '',
    reportingCatalogLoading: () => false,
    reloadReportingCatalog: vi.fn(),
    reloadReportSchedules: vi.fn(),
    runReportScheduleNow: vi.fn(),
    runningScheduleID: () => '',
    saveReportSchedule: vi.fn(),
    savingSchedule: () => false,
    scheduleForm: () => ({
      id: '',
      name: '',
      kind: 'resources',
      enabled: true,
      cadenceType: 'monthly',
      dayOfMonth: 1,
      weekday: 'monday',
      time: '09:00',
      timezone: 'UTC',
      format: 'pdf',
      deliveryMethod: 'email',
      recipients: '',
      attach: true,
      saveToDisk: true,
      tagFilter: '',
      retentionCount: 12,
    }),
    scheduleFormOpen: () => false,
    scheduleResources: () => [],
    selectedResources: () => [],
    setFormat: vi.fn(),
    setMetricType: vi.fn(),
    setRange: vi.fn(),
    setScheduleKind: vi.fn(),
    setScheduleResources: vi.fn(),
    setSelectedResources: vi.fn(),
    setTitle: vi.fn(),
    showUpgradePrompts: () => true,
    startCreateSchedule: vi.fn(),
    startEditSchedule: vi.fn(),
    title: () => '',
    toggleReportSchedule: vi.fn(),
    updateScheduleForm: vi.fn(),
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

  it('shows the scheduled reports table surface with an empty state', () => {
    useReportingPanelStateMock.mockReturnValue(buildState());

    render(() => <ReportingPanel />);

    expect(screen.getByText('Scheduled reports')).toBeInTheDocument();
    expect(screen.getByText('No scheduled reports are configured yet.')).toBeInTheDocument();
  });

  it('keeps the schedule form free of the report type choice while Patrol cannot run', () => {
    useReportingPanelStateMock.mockReturnValue(
      buildState({ scheduleFormOpen: () => true, patrolDigestAvailable: () => false }),
    );

    render(() => <ReportingPanel />);

    expect(screen.queryByLabelText('Report type')).not.toBeInTheDocument();
    expect(screen.queryByText(/Patrol weekly summary can also be emailed/)).not.toBeInTheDocument();
    expect(screen.getByLabelText('Cadence')).toBeInTheDocument();
    // One picker for on-demand reports, one in the schedule form.
    expect(screen.getAllByText('Mock Resource Picker')).toHaveLength(2);
  });

  it('offers the Patrol weekly summary as a report type when Patrol can run', () => {
    const setScheduleKind = vi.fn();
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        scheduleFormOpen: () => true,
        patrolDigestAvailable: () => true,
        setScheduleKind,
      }),
    );

    render(() => <ReportingPanel />);

    expect(screen.getByText(/Patrol weekly summary can also be emailed/)).toBeInTheDocument();
    const select = screen.getByLabelText('Report type') as HTMLSelectElement;
    expect(select.value).toBe('resources');
    expect(Array.from(select.options).map((option) => option.textContent)).toEqual([
      'Performance report',
      'Patrol weekly summary',
    ]);
    expect(
      screen.getByText('A performance report for the resources you pick, as a PDF or CSV.'),
    ).toBeInTheDocument();

    select.value = 'patrol_digest';
    select.dispatchEvent(new Event('change', { bubbles: true }));
    expect(setScheduleKind).toHaveBeenCalledWith('patrol_digest');
  });

  it('reduces the form to weekday, time and recipients for a Patrol weekly summary', () => {
    const base = buildState();
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        scheduleFormOpen: () => true,
        patrolDigestAvailable: () => false,
        scheduleForm: () => ({
          ...base.scheduleForm(),
          kind: 'patrol_digest',
          cadenceType: 'weekly',
          weekday: 'friday',
          deliveryMethod: 'email',
          attach: false,
          saveToDisk: false,
        }),
      }),
    );

    render(() => <ReportingPanel />);

    // An existing digest schedule stays editable even when Patrol is off now.
    const select = screen.getByLabelText('Report type') as HTMLSelectElement;
    expect(select.value).toBe('patrol_digest');
    expect(
      screen.getByText(/Emails what Patrol checked, found, fixed, and spent/),
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Weekday')).toBeInTheDocument();
    expect(screen.getByLabelText('Time', { selector: 'input' })).toBeInTheDocument();
    expect(screen.getByLabelText('Email recipients', { selector: 'input' })).toBeInTheDocument();
    expect(screen.getByLabelText('Schedule name', { selector: 'input' })).toHaveAttribute(
      'placeholder',
      'Patrol weekly summary',
    );
    expect(screen.getByText('Enabled')).toBeInTheDocument();

    expect(screen.queryByLabelText('Cadence')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Day of month')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Format')).not.toBeInTheDocument();
    // Only the on-demand report picker remains.
    expect(screen.getAllByText('Mock Resource Picker')).toHaveLength(1);
    expect(screen.queryByLabelText('Tag filter')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Delivery')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Retention')).not.toBeInTheDocument();
    expect(screen.queryByText('Attach')).not.toBeInTheDocument();
    expect(screen.queryByText('Save copy')).not.toBeInTheDocument();
  });

  it('describes a digest schedule by its Patrol window in the schedules table', () => {
    useReportingPanelStateMock.mockReturnValue(
      buildState({
        reportSchedules: () => [
          {
            id: 'digest-1',
            name: 'Weekly Patrol',
            kind: 'patrol_digest',
            enabled: true,
            cadence: { type: 'weekly', weekday: 'monday', time: '09:00', timezone: 'UTC' },
            scope: { resources: [], tags: [] },
            format: 'email',
            delivery: { method: 'email', to: [], attach: false, save_to_disk: false },
          },
        ],
      }),
    );

    render(() => <ReportingPanel />);

    expect(screen.getByText('Weekly Patrol')).toBeInTheDocument();
    expect(screen.getByText('Patrol activity, last 7 days')).toBeInTheDocument();
    expect(screen.getByText('Email config recipients')).toBeInTheDocument();
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
