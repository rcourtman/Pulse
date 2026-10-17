import { For, Show, Suspense, lazy } from 'solid-js';
import FileText from 'lucide-solid/icons/file-text';
import Download from 'lucide-solid/icons/download';
import BarChart from 'lucide-solid/icons/bar-chart';
import TableProperties from 'lucide-solid/icons/table-properties';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Play from 'lucide-solid/icons/play';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Trash2 from 'lucide-solid/icons/trash-2';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import { Button } from '@/components/shared/Button';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { formControl } from '@/components/shared/Form';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { FeatureGateSection } from '@/components/shared/FeatureGateSection';
import { useReportingPanelState } from '@/components/Settings/useReportingPanelState';
import type { ReportingFormat } from '@/components/Settings/reportingCatalogModel';
import { type ReportingRangeValue } from '@/components/Settings/reportingPanelModel';
import {
  formatReportScheduleTime,
  reportScheduleCadenceLabel,
  reportScheduleDeliveryLabel,
  reportScheduleLastRunLabel,
  reportScheduleOutcomeDetail,
  reportScheduleRunActionLabel,
  reportScheduleScopeLabel,
  type ReportSchedule,
} from '@/components/Settings/reportingSchedulesModel';
import { ResourcePicker } from './ResourcePicker';
import { ReportingFormField as FormField } from './ReportingFormField';

const REPORTING_FORMAT_ICONS: Record<ReportingFormat, typeof FileText> = {
  csv: BarChart,
  pdf: FileText,
};

const ReportScheduleEditor = lazy(() => import('./ReportScheduleEditor'));

export function ReportingPanel() {
  const state = useReportingPanelState();
  const {
    deleteReportSchedule,
    deletingScheduleID,
    exportingInventory,
    format,
    handleExportVMInventory,
    generating,
    handleGenerate,
    isLocked,
    isReportingEnabled,
    metricType,
    patrolDigestAvailable,
    range,
    reportSchedules,
    reportSchedulesError,
    reportSchedulesLoading,
    reportingCatalog,
    reportingCatalogError,
    reportingCatalogLoading,
    reloadReportingCatalog,
    reloadReportSchedules,
    runReportScheduleNow,
    runningScheduleID,
    scheduleFormOpen,
    selectedResources,
    setFormat,
    setMetricType,
    setRange,
    setSelectedResources,
    setTitle,
    showUpgradePrompts,
    startCreateSchedule,
    startEditSchedule,
    title,
    toggleReportSchedule,
    upgradeDestination,
  } = state;
  const visibleOutcome = (schedule: ReportSchedule): ReportSchedule =>
    runningScheduleID() === schedule.id
      ? { ...schedule, last_run_status: 'running', last_error: '' }
      : schedule;

  const performanceReport = () => reportingCatalog()?.performanceReport ?? null;
  const inventoryDefinition = () => reportingCatalog()?.vmInventoryExport ?? null;
  const lockedState = () => {
    const catalog = reportingCatalog();
    const state = catalog?.lockedState ?? null;
    if (!catalog || !state || showUpgradePrompts()) {
      return state;
    }
    return {
      title: `${state.title} unavailable`,
      description:
        'Reporting is locked for this session. The report builder appears when advanced reporting is available.',
    };
  };
  const guidance = () => reportingCatalog()?.guidance ?? null;
  const catalogReady = () => reportingCatalog() !== null;
  const supportsMetricFilter = () => performanceReport()?.supportsMetricFilter ?? false;
  const supportsCustomTitle = () => performanceReport()?.supportsCustomTitle ?? false;
  const selectedRange = (): ReportingRangeValue => range() ?? performanceReport()!.defaultRange;
  const selectedFormat = (): ReportingFormat => format() ?? performanceReport()!.defaultFormat;
  const optionalFieldCount = () => Number(supportsMetricFilter()) + Number(supportsCustomTitle());
  const optionalFieldGridClass = () =>
    optionalFieldCount() > 1 ? 'grid grid-cols-1 gap-6 md:grid-cols-2' : 'grid grid-cols-1 gap-6';

  const rangeFilterOptions = (): FilterOption<ReportingRangeValue>[] =>
    (performanceReport()?.ranges ?? []).map((option) => ({
      label: option.label,
      value: option.key,
    }));

  const formatFilterOptions = (): FilterOption<ReportingFormat>[] =>
    (performanceReport()?.formats ?? []).map((option) => ({
      value: option.value,
      label: option.label,
      icon: REPORTING_FORMAT_ICONS[option.value],
    }));

  return (
    <div class="space-y-6">
      <Show when={reportingCatalogLoading() && !catalogReady()}>
        <OperationsPanel title="Reporting" description="Loading reporting surfaces...">
          <div class="p-4 sm:p-6">
            <p class="text-sm text-muted">Loading reporting surfaces...</p>
          </div>
        </OperationsPanel>
      </Show>

      <Show when={reportingCatalogError() && !catalogReady()}>
        <OperationsPanel
          title="Reporting"
          description="Reporting surfaces are currently unavailable."
        >
          <div class="space-y-4 p-4 sm:p-6">
            <p class="text-sm text-warning">{reportingCatalogError()}</p>
            <div class="flex justify-end">
              <Button variant="secondary" size="md" onClick={reloadReportingCatalog}>
                Retry
              </Button>
            </div>
          </div>
        </OperationsPanel>
      </Show>

      <Show when={isLocked()}>
        <OperationsPanel
          title={reportingCatalog()!.title}
          description={reportingCatalog()!.description}
        >
          <div class="p-4 sm:p-6">
            <Show when={lockedState()}>
              {(state) => (
                <FeatureGateSection
                  title={state().title}
                  body={state().description}
                  upgradeDestination={upgradeDestination()}
                  showUpgradePrompts={showUpgradePrompts()}
                />
              )}
            </Show>
          </div>
        </OperationsPanel>
      </Show>

      <Show when={isReportingEnabled()}>
        <OperationsPanel
          title={reportingCatalog()!.title}
          description={reportingCatalog()!.description}
        >
          <div class="space-y-6 p-4 sm:p-6">
            <Show when={reportingCatalogLoading()}>
              <p class="text-sm text-muted">Loading reporting surfaces...</p>
            </Show>

            <Show when={reportingCatalogError()}>
              <p class="text-sm text-warning">{reportingCatalogError()}</p>
            </Show>

            <Show when={performanceReport()}>
              <section class="space-y-6">
                <div class="space-y-2">
                  <h4 class="text-base font-semibold text-base-content">
                    {performanceReport()?.title}
                  </h4>
                  <p class="text-sm text-muted">{performanceReport()?.description}</p>
                </div>

                <FormField
                  label="Resources"
                  helpText="Select the resources to include in the report"
                >
                  <ResourcePicker
                    maxSelection={performanceReport()?.multiResourceMax}
                    selected={selectedResources}
                    onSelectionChange={setSelectedResources}
                  />
                </FormField>

                <Show when={optionalFieldCount() > 0}>
                  <div class={optionalFieldGridClass()}>
                    <Show when={supportsMetricFilter()}>
                      <FormField
                        label="Metric Type (Optional)"
                        helpText="Filter by specific metric type"
                      >
                        <input
                          id="metric-type"
                          aria-label="Metric Type (Optional)"
                          type="text"
                          class={formControl}
                          placeholder="e.g. cpu, memory, disk, temperature (leave empty for all)"
                          value={metricType()}
                          onInput={(e) => setMetricType(e.currentTarget.value)}
                        />
                      </FormField>
                    </Show>

                    <Show when={supportsCustomTitle()}>
                      <FormField label="Report Title" helpText="Custom title for the PDF report">
                        <input
                          id="report-title"
                          aria-label="Report Title"
                          type="text"
                          class={formControl}
                          placeholder="Auto-generated if empty"
                          value={title()}
                          onInput={(e) => setTitle(e.currentTarget.value)}
                        />
                      </FormField>
                    </Show>
                  </div>
                </Show>

                <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
                  <FormField label="Time Range">
                    <FilterButtonGroup
                      class="sm:grid-cols-3"
                      options={rangeFilterOptions()}
                      value={selectedRange()}
                      onChange={setRange}
                      variant="prominent"
                    />
                  </FormField>

                  <FormField label="Export Format">
                    <FilterButtonGroup
                      class="sm:grid-cols-2"
                      options={formatFilterOptions()}
                      value={selectedFormat()}
                      onChange={setFormat}
                      variant="prominent"
                    />
                  </FormField>
                </div>

                <div class="flex justify-end">
                  <Button
                    variant="primary"
                    size="lg"
                    class="w-full gap-2 font-semibold sm:w-auto"
                    isLoading={generating()}
                    disabled={generating()}
                    onClick={handleGenerate}
                  >
                    <Show when={!generating()}>
                      <Download size={20} />
                    </Show>
                    {generating()
                      ? 'Generating...'
                      : selectedResources().length > 0
                        ? `Generate Report (${selectedResources().length} resource${selectedResources().length !== 1 ? 's' : ''})`
                        : 'Generate Report'}
                  </Button>
                </div>
              </section>
            </Show>

            <section class="space-y-4 border-t border-base-300/80 pt-6">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="space-y-1">
                  <h4 class="text-base font-semibold text-base-content">Scheduled reports</h4>
                  <p class="text-sm text-muted">
                    Send recurring client performance reports using the same resource scope and
                    branding as generated reports.
                    <Show when={patrolDigestAvailable()}>
                      {' '}
                      A Patrol weekly summary can also be emailed: what Patrol checked, found,
                      fixed, and spent in the last seven days.
                    </Show>
                  </p>
                </div>
                <div class="flex flex-wrap gap-2">
                  <Button
                    variant="secondary"
                    size="sm"
                    class="gap-2"
                    onClick={reloadReportSchedules}
                  >
                    <RefreshCw size={16} />
                    Refresh
                  </Button>
                  <Button variant="primary" size="sm" class="gap-2" onClick={startCreateSchedule}>
                    <Plus size={16} />
                    Create schedule
                  </Button>
                </div>
              </div>

              <Show when={scheduleFormOpen()}>
                <Suspense fallback={<p class="text-sm text-muted">Loading schedule editor...</p>}>
                  <ReportScheduleEditor state={state} />
                </Suspense>
              </Show>

              <Show when={reportSchedulesLoading()}>
                <p class="text-sm text-muted">Loading report schedules...</p>
              </Show>
              <Show when={reportSchedulesError()}>
                <p class="text-sm text-warning">{reportSchedulesError()}</p>
              </Show>

              <Show
                when={reportSchedules().length > 0}
                fallback={
                  <Show when={!reportSchedulesLoading() && !reportSchedulesError()}>
                    <div class="flex flex-col gap-3 border border-dashed border-base-300 p-4 sm:flex-row sm:items-center sm:justify-between">
                      <p class="text-sm text-muted">No scheduled reports are configured yet.</p>
                      <Button
                        variant="secondary"
                        size="sm"
                        class="gap-2"
                        onClick={startCreateSchedule}
                      >
                        <Plus size={16} />
                        Create schedule
                      </Button>
                    </div>
                  </Show>
                }
              >
                <ul
                  class="divide-y divide-base-300 rounded-md border border-base-300 text-sm"
                  aria-label="Scheduled reports"
                >
                  <For each={reportSchedules()}>
                    {(schedule) => (
                      <li class="space-y-3 p-3">
                        <div class="grid grid-cols-[minmax(0,1fr)_auto_auto] items-start gap-3">
                          <div class="min-w-0 whitespace-normal break-words">
                            <div class="font-medium text-base-content">{schedule.name}</div>
                            <div class="mt-1 text-xs text-muted">
                              {reportScheduleCadenceLabel(schedule)}
                            </div>
                            <div class="mt-1 text-sm">
                              {reportScheduleLastRunLabel(visibleOutcome(schedule))}
                            </div>
                            <Show when={schedule.last_run_at}>
                              <div class="mt-0.5 text-xs text-muted">
                                {formatReportScheduleTime(schedule.last_run_at)}
                              </div>
                            </Show>
                          </div>
                          <label class="flex min-h-11 cursor-pointer flex-col items-center gap-2 text-xs text-muted">
                            Enabled
                            <input
                              type="checkbox"
                              class="h-4 w-4"
                              checked={schedule.enabled}
                              aria-label={`Enable ${schedule.name}`}
                              onChange={() => toggleReportSchedule(schedule)}
                            />
                          </label>
                          <div class="w-20">
                            <Button
                              variant="ghost"
                              size="sm"
                              class="min-h-11 w-full gap-1 px-1"
                              aria-label={`${reportScheduleRunActionLabel(schedule)} ${schedule.name} now`}
                              isLoading={runningScheduleID() === schedule.id}
                              disabled={
                                runningScheduleID() !== '' || schedule.last_run_status === 'running'
                              }
                              onClick={() => runReportScheduleNow(schedule)}
                            >
                              <Show when={runningScheduleID() !== schedule.id}>
                                <Play size={15} />
                              </Show>
                              {reportScheduleRunActionLabel(schedule)}
                            </Button>
                          </div>
                        </div>
                        <div
                          class="space-y-1 whitespace-normal break-words text-xs text-muted"
                          aria-live="polite"
                        >
                          <Show when={reportScheduleOutcomeDetail(visibleOutcome(schedule))}>
                            <p
                              class={
                                visibleOutcome(schedule).last_run_status === 'failed' ||
                                visibleOutcome(schedule).last_run_status === 'unconfirmed'
                                  ? 'text-warning'
                                  : ''
                              }
                            >
                              {reportScheduleOutcomeDetail(visibleOutcome(schedule))}
                            </p>
                          </Show>
                          <Show when={schedule.delivery_check && !schedule.delivery_check.ready}>
                            <p class="text-warning">
                              {schedule.delivery_check?.message}{' '}
                              <a href="/alerts/notifications" class="underline">
                                Email settings
                              </a>
                            </p>
                          </Show>
                          <Show when={schedule.retry_at && schedule.enabled}>
                            <p>
                              Automatic retry: {formatReportScheduleTime(schedule.retry_at)}.{' '}
                              {schedule.run_attempts} of 3 attempts used.
                            </p>
                          </Show>
                          <Show when={schedule.enabled && schedule.next_run_at}>
                            <p>
                              Next scheduled report:{' '}
                              {formatReportScheduleTime(schedule.next_run_at)}
                            </p>
                          </Show>
                          <details>
                            <summary
                              class="cursor-pointer py-2"
                              aria-label={`Details for ${schedule.name}`}
                            >
                              Report details
                            </summary>
                            <div class="space-y-2 pb-1 pt-2">
                              <p>
                                <span>{reportScheduleScopeLabel(schedule)}</span> ·{' '}
                                <span>{reportScheduleDeliveryLabel(schedule)}</span> ·{' '}
                                {schedule.format.toUpperCase()}
                              </p>
                              <Show when={(schedule.delivery.to?.length ?? 0) > 0}>
                                <p class="break-all">
                                  Recipients: {schedule.delivery.to?.join(', ')}
                                </p>
                              </Show>
                              <Show when={schedule.last_success_at}>
                                <p>
                                  Last successful run:{' '}
                                  {formatReportScheduleTime(schedule.last_success_at)}
                                </p>
                              </Show>
                              <Show when={schedule.last_report_path}>
                                <p class="break-all">
                                  Last saved report on this server: {schedule.last_report_path}
                                </p>
                              </Show>
                              <div class="flex flex-wrap gap-2 pt-1">
                                <Button
                                  variant="secondary"
                                  size="sm"
                                  class="min-h-11 gap-2"
                                  aria-label={`Edit ${schedule.name}`}
                                  onClick={() => startEditSchedule(schedule)}
                                >
                                  <Pencil size={15} />
                                  Edit schedule
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  class="min-h-11 gap-2"
                                  aria-label={`Delete ${schedule.name}`}
                                  isLoading={deletingScheduleID() === schedule.id}
                                  disabled={deletingScheduleID() !== ''}
                                  onClick={() => deleteReportSchedule(schedule)}
                                >
                                  <Trash2 size={15} />
                                  Delete schedule
                                </Button>
                              </div>
                            </div>
                          </details>
                        </div>
                      </li>
                    )}
                  </For>
                </ul>
              </Show>
            </section>

            <Show when={inventoryDefinition()}>
              <section class="space-y-4 rounded-xl border border-base-300/80 bg-base-200/30 p-4 sm:p-5">
                <div class="space-y-2">
                  <h4 class="text-base font-semibold text-base-content">
                    {inventoryDefinition()?.title}
                  </h4>
                  <p class="text-sm text-muted">{inventoryDefinition()?.description}</p>
                </div>

                <Show when={inventoryDefinition()?.columns.length}>
                  <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                    <For each={inventoryDefinition()?.columns ?? []}>
                      {(column) => (
                        <div class="space-y-1 rounded-lg border border-base-300/70 bg-base-100/70 p-3">
                          <div class="text-xs font-semibold uppercase tracking-wide">
                            {column.label}
                          </div>
                          <p class="text-xs leading-relaxed text-muted">{column.description}</p>
                        </div>
                      )}
                    </For>
                  </div>
                </Show>

                <div class="flex justify-end">
                  <Button
                    variant="success"
                    size="lg"
                    class="w-full gap-2 font-semibold sm:w-auto"
                    isLoading={exportingInventory()}
                    disabled={exportingInventory()}
                    onClick={handleExportVMInventory}
                  >
                    <Show when={!exportingInventory()}>
                      <TableProperties size={20} />
                    </Show>
                    {exportingInventory() ? 'Exporting...' : 'Export VM Inventory'}
                  </Button>
                </div>
              </section>
            </Show>
          </div>
        </OperationsPanel>

        <Show when={guidance()}>
          {(card) => (
            <CalloutCard
              icon={<BarChart size={24} />}
              title={card().title}
              description={card().description}
              padding="lg"
            />
          )}
        </Show>
      </Show>
    </div>
  );
}
