import { For, Show, Suspense, createMemo, createSignal, lazy, onCleanup } from 'solid-js';
import { A } from '@solidjs/router';
import { getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { hasFeature, runtimeCapabilitiesLoaded } from '@/stores/license';
import { Portal } from 'solid-js/web';
import { Card } from '@/components/shared/Card';
import { Dialog } from '@/components/shared/Dialog';
import { InlineNotice } from '@/components/shared/InlineNotice';
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
import { FormSelect } from '@/components/shared/FormSelect';
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

const METRIC_TYPE_PRESETS: readonly { value: string; label: string }[] = [
  { value: '', label: 'All metrics' },
  { value: 'cpu', label: 'CPU usage' },
  { value: 'memory', label: 'Memory usage' },
  { value: 'disk', label: 'Disk usage' },
  { value: 'temperature', label: 'Temperature' },
  { value: 'network', label: 'Network' },
];

const ReportScheduleEditor = lazy(() => import('./ReportScheduleEditor'));

export interface ReportingPanelProps {
  currentUser?: string;
  accessKey?: string;
  canRead?: boolean;
}

export function ReportingPanel(props: ReportingPanelProps) {
  const [org, setOrg] = createSignal(getOrgID() || 'default');
  const unsubscribe = eventBus.on('org_switched', () => setOrg(getOrgID() || 'default'));
  onCleanup(unsubscribe);
  // Key the whole workspace, including picker searches and lazy editors, by the
  // actual reader facts. A copied security status is not a new reader.
  const reader = createMemo(
    (previous?: {
      org: string;
      user?: string;
      access?: string;
      read: boolean;
      enabled: boolean;
      resolved: boolean;
    }) => {
      org(); // Also refresh when the organization event has no capability delta.
      const next = {
        org: getOrgID() || 'default',
        user: props.currentUser,
        access: props.accessKey,
        read: props.canRead !== false,
        enabled: hasFeature('advanced_reporting'),
        resolved: runtimeCapabilitiesLoaded(),
      };
      return previous &&
        Object.keys(next).every(
          (key) => next[key as keyof typeof next] === previous[key as keyof typeof next],
        )
        ? previous
        : next;
    },
  );
  return (
    <Show
      when={reader().read}
      fallback={
        <OperationsPanel title="Reporting">
          <p class="p-4 text-sm text-muted">Reporting is not available to this session.</p>
        </OperationsPanel>
      }
    >
      <Show when={reader()} keyed>
        {(accepted) => <ReportingWorkspace isCurrentReader={() => reader() === accepted} />}
      </Show>
    </Show>
  );
}

function ReportingWorkspace(props: { isCurrentReader: () => boolean }) {
  const state = useReportingPanelState(props.isCurrentReader);
  const {
    actionError,
    successNotice,
    clearFeedback,
    canEditSchedules,
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
      <Show when={actionError() || successNotice()}>
        <Portal>
          <div class="fixed inset-x-3 top-28 z-50 rounded-lg bg-surface sm:left-auto sm:right-4 sm:w-96">
            <InlineNotice
              role={actionError() ? 'alert' : 'status'}
              aria-live={actionError() ? 'assertive' : 'polite'}
              tone={actionError() ? 'warning' : 'success'}
              class="shadow-lg"
              onDismiss={clearFeedback}
              dismissLabel="Dismiss reporting feedback"
            >
              {actionError() || successNotice()}
            </InlineNotice>
          </div>
        </Portal>
      </Show>
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
        <Card
          data-settings-panel
          padding="none"
          border={false}
          class="overflow-hidden border border-border"
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
                      <FormSelect
                        id="metric-type"
                        label="Metric Type (Optional)"
                        help="Filter by specific metric type"
                        value={metricType()}
                        onChange={(e) => setMetricType(e.currentTarget.value)}
                      >
                        <For each={METRIC_TYPE_PRESETS}>
                          {(option) => <option value={option.value}>{option.label}</option>}
                        </For>
                        <Show
                          when={
                            metricType() &&
                            !METRIC_TYPE_PRESETS.some((option) => option.value === metricType())
                          }
                        >
                          <option value={metricType()}>{metricType()}</option>
                        </Show>
                      </FormSelect>
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

            <section class="space-y-4 border-t border-border pt-6">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div class="space-y-1">
                  <h4 class="text-base font-semibold text-base-content">Scheduled reports</h4>
                  <p class="text-sm text-muted">
                    Email reports on a schedule or save them on this server.
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
                  <Button
                    variant="primary"
                    size="sm"
                    class="gap-2"
                    disabled={!canEditSchedules()}
                    onClick={startCreateSchedule}
                  >
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
                    <div class="flex flex-col gap-3 rounded-md border border-dashed border-border p-4 sm:flex-row sm:items-center sm:justify-between">
                      <p class="text-sm text-muted">No scheduled reports are configured yet.</p>
                      <Button
                        variant="secondary"
                        size="sm"
                        class="gap-2"
                        disabled={!canEditSchedules()}
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
                  class="divide-y divide-border rounded-md border border-border text-sm"
                  aria-label="Scheduled reports"
                >
                  <For each={reportSchedules()}>
                    {(schedule) => (
                      <li class="space-y-3 p-3">
                        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                          <div class="min-w-0 flex-1 whitespace-normal break-words">
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
                          <div class="flex flex-wrap items-center gap-2 shrink-0">
                            <label class="inline-flex min-h-11 sm:min-h-9 cursor-pointer items-center gap-2 rounded-md border border-border bg-surface-alt px-2.5 text-xs text-muted">
                              <input
                                type="checkbox"
                                class="h-4 w-4"
                                disabled={!canEditSchedules()}
                                checked={schedule.enabled}
                                aria-label={`Enable ${schedule.name}`}
                                onChange={() => toggleReportSchedule(schedule)}
                              />
                              <span>Enabled</span>
                            </label>
                            <Button
                              variant="ghost"
                              size="sm"
                              class="min-h-11 sm:min-h-9 gap-1 px-2"
                              aria-label={`${reportScheduleRunActionLabel(schedule)} ${schedule.name} now`}
                              isLoading={runningScheduleID() === schedule.id}
                              disabled={
                                !canEditSchedules() ||
                                runningScheduleID() !== '' ||
                                schedule.last_run_status === 'running'
                              }
                              onClick={() => runReportScheduleNow(schedule)}
                            >
                              <Show when={runningScheduleID() !== schedule.id}>
                                <Play size={15} />
                              </Show>
                              {reportScheduleRunActionLabel(schedule)}
                            </Button>
                            <Button
                              variant="secondary"
                              size="sm"
                              class="min-h-11 sm:min-h-9 gap-1.5"
                              aria-label={`Edit ${schedule.name}`}
                              disabled={!canEditSchedules()}
                              onClick={() => startEditSchedule(schedule)}
                            >
                              <Pencil size={14} />
                              Edit
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              class="min-h-11 sm:min-h-9 gap-1.5 text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/30"
                              aria-label={`Delete ${schedule.name}`}
                              isLoading={deletingScheduleID() === schedule.id}
                              disabled={!canEditSchedules() || deletingScheduleID() !== ''}
                              onClick={() => deleteReportSchedule(schedule)}
                            >
                              <Trash2 size={14} />
                              Delete
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
                              <A href="/alerts/notifications" class="underline">
                                Email settings
                              </A>
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
                          <Show
                            when={
                              schedule.kind === 'backup_health' && schedule.last_report_data_message
                            }
                          >
                            <p
                              class={
                                schedule.last_report_data_state === 'complete'
                                  ? 'text-muted'
                                  : 'text-amber-700 dark:text-amber-300'
                              }
                            >
                              Last report data: {schedule.last_report_data_message}
                            </p>
                          </Show>
                          <details>
                            <summary
                              class="cursor-pointer py-1.5"
                              aria-label={`Details for ${schedule.name}`}
                            >
                              Report details
                            </summary>
                            <div class="space-y-1.5 pb-1 pt-1.5">
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
              <section class="space-y-4 rounded-xl border border-border bg-surface-alt p-4 sm:p-5">
                <div class="space-y-2">
                  <h4 class="text-base font-semibold text-base-content">
                    {inventoryDefinition()?.title}
                  </h4>
                  <p class="text-sm text-muted">{inventoryDefinition()?.description}</p>
                </div>

                <Show when={inventoryDefinition()?.columns.length}>
                  <details class="rounded-md border border-border bg-surface p-3">
                    <summary class="cursor-pointer text-xs font-medium text-base-content">
                      Included columns ({inventoryDefinition()?.columns.length})
                    </summary>
                    <div class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                      <For each={inventoryDefinition()?.columns ?? []}>
                        {(column) => (
                          <div class="space-y-1 rounded-lg border border-border bg-surface-alt p-3">
                            <div class="text-xs font-semibold uppercase tracking-wide text-base-content">
                              {column.label}
                            </div>
                            <p class="text-xs leading-relaxed text-muted">{column.description}</p>
                          </div>
                        )}
                      </For>
                    </div>
                  </details>
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
        </Card>

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

        <Dialog
          isOpen={Boolean(state.pendingDeleteSchedule?.())}
          onClose={() => state.cancelDeleteReportSchedule?.()}
          panelClass="max-w-md"
          ariaLabel="Delete report schedule"
        >
          <Show when={state.pendingDeleteSchedule?.()}>
            {(schedule) => (
              <div class="w-full">
                <div class="border-b border-border px-6 py-4">
                  <h3 class="text-base font-semibold text-base-content">
                    Delete {schedule().name}?
                  </h3>
                </div>
                <div class="px-6 py-4 text-sm text-muted">
                  This removes the scheduled report and stops future runs. Previously saved files on
                  disk are not deleted.
                </div>
                <div class="flex items-center justify-end gap-3 border-t border-border bg-surface-alt px-6 py-4">
                  <Button
                    type="button"
                    variant="ghost"
                    size="md"
                    onClick={() => state.cancelDeleteReportSchedule?.()}
                  >
                    Cancel
                  </Button>
                  <Button
                    type="button"
                    variant="danger"
                    size="md"
                    onClick={() => void state.confirmDeleteReportSchedule?.()}
                  >
                    Delete schedule
                  </Button>
                </div>
              </div>
            )}
          </Show>
        </Dialog>

        <Dialog
          isOpen={Boolean(state.pendingRetrySchedule?.())}
          onClose={() => state.cancelRetryReportSchedule?.()}
          panelClass="max-w-md"
          ariaLabel="Retry unconfirmed report delivery"
        >
          <Show when={state.pendingRetrySchedule?.()}>
            {(schedule) => (
              <div class="w-full">
                <div class="border-b border-border px-6 py-4">
                  <h3 class="text-base font-semibold text-base-content">
                    Retry {schedule().name}?
                  </h3>
                </div>
                <div class="px-6 py-4 text-sm text-muted">
                  The previous email may have been accepted. Check the recipient first. Retry
                  anyway, with a risk of a duplicate?
                </div>
                <div class="flex items-center justify-end gap-3 border-t border-border bg-surface-alt px-6 py-4">
                  <Button
                    type="button"
                    variant="ghost"
                    size="md"
                    onClick={() => state.cancelRetryReportSchedule?.()}
                  >
                    Cancel
                  </Button>
                  <Button
                    type="button"
                    variant="primary"
                    size="md"
                    onClick={() => void state.confirmRetryReportSchedule?.()}
                  >
                    Retry anyway
                  </Button>
                </div>
              </div>
            )}
          </Show>
        </Dialog>
      </Show>
    </div>
  );
}
