import { For, JSX, Show } from 'solid-js';
import FileText from 'lucide-solid/icons/file-text';
import Download from 'lucide-solid/icons/download';
import BarChart from 'lucide-solid/icons/bar-chart';
import TableProperties from 'lucide-solid/icons/table-properties';
import Plus from 'lucide-solid/icons/plus';
import Pencil from 'lucide-solid/icons/pencil';
import Play from 'lucide-solid/icons/play';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Trash2 from 'lucide-solid/icons/trash-2';
import Save from 'lucide-solid/icons/save';
import X from 'lucide-solid/icons/x';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import { Button } from '@/components/shared/Button';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { formControl, formField, formHelpText, formLabel } from '@/components/shared/Form';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { FormSelect } from '@/components/shared/FormSelect';
import { FeatureGateSection } from '@/components/shared/FeatureGateSection';
import { useReportingPanelState } from '@/components/Settings/useReportingPanelState';
import type { ReportingFormat } from '@/components/Settings/reportingCatalogModel';
import { type ReportingRangeValue } from '@/components/Settings/reportingPanelModel';
import {
  formatReportScheduleTime,
  reportScheduleCadenceLabel,
  reportScheduleDeliveryLabel,
  reportScheduleLastRunLabel,
  reportScheduleScopeLabel,
} from '@/components/Settings/reportingSchedulesModel';
import { ResourcePicker } from './ResourcePicker';

const REPORTING_FORMAT_ICONS: Record<ReportingFormat, typeof FileText> = {
  csv: BarChart,
  pdf: FileText,
};

interface FormFieldProps {
  label: string;
  helpText?: string;
  children: JSX.Element;
}

function FormField(props: FormFieldProps) {
  return (
    <div class={formField}>
      <label class={formLabel}>{props.label}</label>
      {props.children}
      {props.helpText && <span class={formHelpText}>{props.helpText}</span>}
    </div>
  );
}

const WEEKDAY_OPTIONS = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'];

export function ReportingPanel() {
  const {
    closeScheduleForm,
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
    saveReportSchedule,
    savingSchedule,
    scheduleForm,
    scheduleFormOpen,
    scheduleResources,
    selectedResources,
    setFormat,
    setMetricType,
    setRange,
    setScheduleResources,
    setSelectedResources,
    setTitle,
    showUpgradePrompts,
    startCreateSchedule,
    startEditSchedule,
    title,
    toggleReportSchedule,
    updateScheduleForm,
    upgradeDestination,
  } = useReportingPanelState();

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
                    Send recurring client performance reports using the same resource scope and branding as generated reports.
                  </p>
                </div>
                <div class="flex flex-wrap gap-2">
                  <Button variant="secondary" size="sm" class="gap-2" onClick={reloadReportSchedules}>
                    <RefreshCw size={16} />
                    Refresh
                  </Button>
                  <Button variant="primary" size="sm" class="gap-2" onClick={startCreateSchedule}>
                    <Plus size={16} />
                    Create schedule
                  </Button>
                </div>
              </div>

              <Show when={reportSchedulesLoading()}>
                <p class="text-sm text-muted">Loading report schedules...</p>
              </Show>
              <Show when={reportSchedulesError()}>
                <p class="text-sm text-warning">{reportSchedulesError()}</p>
              </Show>

              <Show
                when={reportSchedules().length > 0}
                fallback={
                  <div class="flex flex-col gap-3 border border-dashed border-base-300 p-4 sm:flex-row sm:items-center sm:justify-between">
                    <p class="text-sm text-muted">
                      No scheduled reports are configured yet.
                    </p>
                    <Button variant="secondary" size="sm" class="gap-2" onClick={startCreateSchedule}>
                      <Plus size={16} />
                      Create schedule
                    </Button>
                  </div>
                }
              >
                <div class="overflow-x-auto rounded-md border border-base-300">
                  <table class="w-full min-w-[980px] table-fixed text-left text-sm">
                    <colgroup>
                      <col class="w-[18%]" />
                      <col class="w-[16%]" />
                      <col class="w-[14%]" />
                      <col class="w-[14%]" />
                      <col class="w-[16%]" />
                      <col class="w-[8%]" />
                      <col class="w-[14%]" />
                    </colgroup>
                    <thead class="border-b border-base-300 bg-base-200/50 text-xs uppercase text-muted">
                      <tr>
                        <th class="px-3 py-2 font-semibold">Name</th>
                        <th class="px-3 py-2 font-semibold">Cadence</th>
                        <th class="px-3 py-2 font-semibold">Scope</th>
                        <th class="px-3 py-2 font-semibold">Delivery</th>
                        <th class="px-3 py-2 font-semibold">Last run</th>
                        <th class="px-3 py-2 font-semibold">Enabled</th>
                        <th class="px-3 py-2 text-right font-semibold">Actions</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-base-300">
                      <For each={reportSchedules()}>
                        {(schedule) => (
                          <tr>
                            <td class="truncate px-3 py-2 font-medium text-base-content" title={schedule.name}>
                              {schedule.name}
                            </td>
                            <td class="truncate px-3 py-2 text-muted" title={reportScheduleCadenceLabel(schedule)}>
                              {reportScheduleCadenceLabel(schedule)}
                            </td>
                            <td class="truncate px-3 py-2 text-muted" title={reportScheduleScopeLabel(schedule)}>
                              {reportScheduleScopeLabel(schedule)}
                            </td>
                            <td class="truncate px-3 py-2 text-muted" title={reportScheduleDeliveryLabel(schedule)}>
                              {reportScheduleDeliveryLabel(schedule)}
                            </td>
                            <td class="truncate px-3 py-2 text-muted" title={schedule.last_error || ''}>
                              <div class="truncate">{reportScheduleLastRunLabel(schedule)}</div>
                              <Show when={schedule.last_run_at}>
                                <div class="truncate text-xs text-muted">
                                  {formatReportScheduleTime(schedule.last_run_at)}
                                </div>
                              </Show>
                            </td>
                            <td class="px-3 py-2">
                              <label class="inline-flex items-center">
                                <input
                                  type="checkbox"
                                  class="h-4 w-4"
                                  checked={schedule.enabled}
                                  onChange={() => toggleReportSchedule(schedule)}
                                />
                              </label>
                            </td>
                            <td class="px-3 py-2">
                              <div class="flex justify-end gap-1">
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  class="gap-1 px-2"
                                  title="Run now"
                                  isLoading={runningScheduleID() === schedule.id}
                                  disabled={runningScheduleID() !== ''}
                                  onClick={() => runReportScheduleNow(schedule)}
                                >
                                  <Show when={runningScheduleID() !== schedule.id}>
                                    <Play size={15} />
                                  </Show>
                                  Run
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  class="gap-1 px-2"
                                  title="Edit"
                                  onClick={() => startEditSchedule(schedule)}
                                >
                                  <Pencil size={15} />
                                  Edit
                                </Button>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  class="gap-1 px-2"
                                  title="Delete"
                                  isLoading={deletingScheduleID() === schedule.id}
                                  disabled={deletingScheduleID() !== ''}
                                  onClick={() => deleteReportSchedule(schedule)}
                                >
                                  <Trash2 size={15} />
                                  Delete
                                </Button>
                              </div>
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>

              <Show when={scheduleFormOpen()}>
                <section class="space-y-4 border-y border-base-300 py-4">
                  <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                    <FormField label="Schedule name">
                      <input
                        type="text"
                        class={formControl}
                        value={scheduleForm().name}
                        onInput={(e) => updateScheduleForm({ name: e.currentTarget.value })}
                        placeholder="Monthly client report"
                      />
                    </FormField>
                    <FormField label="Timezone">
                      <input
                        type="text"
                        class={formControl}
                        value={scheduleForm().timezone}
                        onInput={(e) => updateScheduleForm({ timezone: e.currentTarget.value })}
                        placeholder="Europe/London"
                      />
                    </FormField>
                  </div>

                  <div class="grid grid-cols-1 gap-4 md:grid-cols-4">
                    <FormSelect
                      label="Cadence"
                      value={scheduleForm().cadenceType}
                      onChange={(e) => updateScheduleForm({ cadenceType: e.currentTarget.value as 'monthly' | 'weekly' })}
                    >
                      <option value="monthly">Monthly</option>
                      <option value="weekly">Weekly</option>
                    </FormSelect>
                    <Show
                      when={scheduleForm().cadenceType === 'monthly'}
                      fallback={
                        <FormSelect
                          label="Weekday"
                          value={scheduleForm().weekday}
                          onChange={(e) => updateScheduleForm({ weekday: e.currentTarget.value })}
                        >
                          <For each={WEEKDAY_OPTIONS}>
                            {(day) => <option value={day}>{day[0].toUpperCase() + day.slice(1)}</option>}
                          </For>
                        </FormSelect>
                      }
                    >
                      <FormField label="Day of month">
                        <input
                          type="number"
                          min="1"
                          max="28"
                          class={formControl}
                          value={scheduleForm().dayOfMonth}
                          onInput={(e) => updateScheduleForm({ dayOfMonth: Number(e.currentTarget.value) })}
                        />
                      </FormField>
                    </Show>
                    <FormField label="Time">
                      <input
                        type="time"
                        class={formControl}
                        value={scheduleForm().time}
                        onInput={(e) => updateScheduleForm({ time: e.currentTarget.value })}
                      />
                    </FormField>
                    <FormSelect
                      label="Format"
                      value={scheduleForm().format}
                      onChange={(e) => updateScheduleForm({ format: e.currentTarget.value as ReportingFormat })}
                    >
                      <option value="pdf">PDF</option>
                      <option value="csv">CSV</option>
                    </FormSelect>
                  </div>

                  <FormField
                    label="Resources"
                    helpText="Use explicit resources, tags, or both. Scheduled reports use the previous reporting boundary."
                  >
                    <ResourcePicker
                      maxSelection={performanceReport()?.multiResourceMax}
                      selected={scheduleResources}
                      onSelectionChange={setScheduleResources}
                    />
                  </FormField>

                  <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
                    <FormField label="Tag filter" helpText="Comma-separated tags">
                      <input
                        type="text"
                        class={formControl}
                        value={scheduleForm().tagFilter}
                        onInput={(e) => updateScheduleForm({ tagFilter: e.currentTarget.value })}
                        placeholder="production, customer-facing"
                      />
                    </FormField>
                    <FormSelect
                      label="Delivery"
                      value={scheduleForm().deliveryMethod}
                      onChange={(e) => updateScheduleForm({ deliveryMethod: e.currentTarget.value as 'email' | 'disk' })}
                    >
                      <option value="email">Email recipients</option>
                      <option value="disk">Save to disk</option>
                    </FormSelect>
                    <FormField label="Retention">
                      <input
                        type="number"
                        min="1"
                        max="120"
                        class={formControl}
                        value={scheduleForm().retentionCount}
                        onInput={(e) => updateScheduleForm({ retentionCount: Number(e.currentTarget.value) })}
                      />
                    </FormField>
                  </div>

                  <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                    <FormField label="Email recipients" helpText="Blank uses the existing email notification recipients">
                      <input
                        type="text"
                        class={formControl}
                        value={scheduleForm().recipients}
                        onInput={(e) => updateScheduleForm({ recipients: e.currentTarget.value })}
                        placeholder="client@example.com, ops@example.com"
                        disabled={scheduleForm().deliveryMethod !== 'email'}
                      />
                    </FormField>
                    <div class="grid grid-cols-1 gap-3 pt-6 sm:grid-cols-3">
                      <label class="inline-flex items-center gap-2 text-sm text-muted">
                        <input
                          type="checkbox"
                          checked={scheduleForm().enabled}
                          onChange={(e) => updateScheduleForm({ enabled: e.currentTarget.checked })}
                        />
                        Enabled
                      </label>
                      <label class="inline-flex items-center gap-2 text-sm text-muted">
                        <input
                          type="checkbox"
                          checked={scheduleForm().attach}
                          onChange={(e) => updateScheduleForm({ attach: e.currentTarget.checked })}
                          disabled={scheduleForm().deliveryMethod !== 'email'}
                        />
                        Attach
                      </label>
                      <label class="inline-flex items-center gap-2 text-sm text-muted">
                        <input
                          type="checkbox"
                          checked={scheduleForm().saveToDisk}
                          onChange={(e) => updateScheduleForm({ saveToDisk: e.currentTarget.checked })}
                        />
                        Save copy
                      </label>
                    </div>
                  </div>

                  <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
                    <Button variant="secondary" size="md" class="gap-2" onClick={closeScheduleForm}>
                      <X size={16} />
                      Cancel
                    </Button>
                    <Button
                      variant="primary"
                      size="md"
                      class="gap-2"
                      isLoading={savingSchedule()}
                      disabled={savingSchedule()}
                      onClick={saveReportSchedule}
                    >
                      <Save size={16} />
                      Save schedule
                    </Button>
                  </div>
                </section>
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
                          <div class="text-xs font-semibold uppercase tracking-wide text-base-content/80">
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
