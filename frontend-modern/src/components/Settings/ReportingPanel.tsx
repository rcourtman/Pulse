import { For, Show, JSX } from 'solid-js';
import FileText from 'lucide-solid/icons/file-text';
import Download from 'lucide-solid/icons/download';
import BarChart from 'lucide-solid/icons/bar-chart';
import TableProperties from 'lucide-solid/icons/table-properties';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { formField, formLabel, formHelpText, formControl } from '@/components/shared/Form';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { ResourcePicker } from './ResourcePicker';
import { trackUpgradeClicked } from '@/utils/upgradeMetrics';
import { REPORTING_RANGE_OPTIONS } from '@/utils/reportingPresentation';
import {
  getUpgradeActionButtonClass,
  UPGRADE_ACTION_LABEL,
  UPGRADE_TRIAL_LABEL,
  UPGRADE_TRIAL_LINK_CLASS,
} from '@/utils/upgradePresentation';
import { useReportingPanelState } from '@/components/Settings/useReportingPanelState';
import { type ReportingRangeValue } from '@/components/Settings/reportingPanelModel';

const REPORTING_RANGE_FILTER_OPTIONS: FilterOption<ReportingRangeValue>[] =
  REPORTING_RANGE_OPTIONS.map((option) => ({
    label: option.label,
    value: option.value,
  }));

const REPORTING_FORMAT_FILTER_OPTIONS: FilterOption<'pdf' | 'csv'>[] = [
  { value: 'pdf', label: 'PDF Report', icon: FileText },
  { value: 'csv', label: 'CSV Data', icon: BarChart },
];

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

export function ReportingPanel() {
  const {
    canStartTrial,
    exportingInventory,
    format,
    handleExportVMInventory,
    generating,
    handleGenerate,
    handleStartTrial,
    inventoryDefinition,
    inventoryDefinitionError,
    inventoryDefinitionLoading,
    isLocked,
    isReportingEnabled,
    metricType,
    range,
    selectedResources,
    setFormat,
    setMetricType,
    setRange,
    setSelectedResources,
    setTitle,
    startingTrial,
    title,
    upgradeActionUrl,
  } = useReportingPanelState();

  return (
    <div class="space-y-6">
      <Show when={isLocked()}>
        <OperationsPanel
          title="Detailed Reporting"
          description="Generate performance reports and current-state exports across infrastructure and workloads."
          icon={<BarChart class="w-5 h-5" strokeWidth={2} />}
        >
          <div class="p-4 sm:p-6">
            <div class="flex flex-col sm:flex-row items-center gap-4">
              <div class="flex-1 text-center sm:text-left">
                <h4 class="text-base font-semibold text-base-content">Advanced Reporting (Pro)</h4>
                <p class="text-sm text-muted mt-1">
                  Generate PDF and CSV performance reports plus current-state VM inventory exports
                  across infrastructure and workload resources.
                </p>
              </div>
              <div class="flex flex-col sm:flex-row items-center gap-2">
                <a
                  href={upgradeActionUrl()}
                  target="_blank"
                  rel="noopener noreferrer"
                  class={getUpgradeActionButtonClass()}
                  onClick={() =>
                    trackUpgradeClicked('settings_reporting_panel', 'advanced_reporting')
                  }
                >
                  {UPGRADE_ACTION_LABEL}
                </a>
                <Show when={canStartTrial()}>
                  <button
                    type="button"
                    onClick={handleStartTrial}
                    disabled={startingTrial()}
                    class={UPGRADE_TRIAL_LINK_CLASS}
                  >
                    {UPGRADE_TRIAL_LABEL}
                  </button>
                </Show>
              </div>
            </div>
          </div>
        </OperationsPanel>
      </Show>

      <Show when={isReportingEnabled()}>
        <OperationsPanel
          title="Detailed Reporting"
          description="Generate performance reports and current-state exports across infrastructure and workloads."
          icon={<BarChart class="w-5 h-5" strokeWidth={2} />}
        >
          <div class="space-y-6 p-4 sm:p-6">
            <section class="space-y-6">
              <div class="space-y-2">
                <h4 class="text-base font-semibold text-base-content">Performance Reports</h4>
                <p class="text-sm text-muted">
                  Generate PDF summaries or CSV metric exports from historical monitoring data for
                  one or more selected resources.
                </p>
              </div>

              <FormField label="Resources" helpText="Select the resources to include in the report">
                <ResourcePicker
                  selected={selectedResources}
                  onSelectionChange={setSelectedResources}
                />
              </FormField>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                <FormField label="Metric Type (Optional)" helpText="Filter by specific metric type">
                  <input
                    id="metric-type"
                    type="text"
                    class={formControl}
                    placeholder="e.g. cpu, memory, disk, temperature (leave empty for all)"
                    value={metricType()}
                    onInput={(e) => setMetricType(e.currentTarget.value)}
                  />
                </FormField>

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
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                <FormField label="Time Range">
                  <FilterButtonGroup
                    class="sm:grid-cols-3"
                    options={REPORTING_RANGE_FILTER_OPTIONS}
                    value={range()}
                    onChange={setRange}
                    variant="prominent"
                  />
                </FormField>

                <FormField label="Export Format">
                  <FilterButtonGroup
                    class="sm:grid-cols-2"
                    options={REPORTING_FORMAT_FILTER_OPTIONS}
                    value={format()}
                    onChange={setFormat}
                    variant="prominent"
                  />
                </FormField>
              </div>

              <div class="flex justify-end">
                <button
                  class={`w-full sm:w-auto flex items-center justify-center gap-2 px-6 py-3 rounded-md font-semibold transition-all ${
                    generating()
                      ? 'bg-slate-300 text-slate-500 cursor-not-allowed'
                      : 'bg-blue-600 hover:bg-blue-700 text-white'
                  }`}
                  disabled={generating()}
                  onClick={handleGenerate}
                >
                  <Show when={generating()} fallback={<Download size={20} />}>
                    <div class="w-5 h-5 border-2 border-white border-t-white rounded-full animate-spin" />
                  </Show>
                  {generating()
                    ? 'Generating...'
                    : selectedResources().length > 0
                      ? `Generate Report (${selectedResources().length} resource${selectedResources().length !== 1 ? 's' : ''})`
                      : 'Generate Report'}
                </button>
              </div>
            </section>

            <section class="rounded-xl border border-base-300/80 bg-base-200/30 p-4 sm:p-5 space-y-4">
              <div class="space-y-2">
                <h4 class="text-base font-semibold text-base-content">VM Inventory Export</h4>
                <p class="text-sm text-muted">
                  {inventoryDefinition()?.description ??
                    'Export the current fleet-wide VM inventory as CSV using the canonical runtime model.'}
                </p>
              </div>

              <Show when={inventoryDefinitionLoading()}>
                <p class="text-xs text-muted">Loading export column definition...</p>
              </Show>

              <Show when={inventoryDefinitionError()}>
                <p class="text-xs text-warning">{inventoryDefinitionError()}</p>
              </Show>

              <Show when={inventoryDefinition()?.columns.length}>
                <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
                  <For each={inventoryDefinition()?.columns ?? []}>
                    {(column) => (
                      <div class="rounded-lg border border-base-300/70 bg-base-100/70 p-3 space-y-1">
                        <div class="text-xs font-semibold uppercase tracking-wide text-base-content/80">
                          {column.label}
                        </div>
                        <p class="text-xs text-muted leading-relaxed">{column.description}</p>
                      </div>
                    )}
                  </For>
                </div>
              </Show>

              <div class="flex justify-end">
                <button
                  class={`w-full sm:w-auto flex items-center justify-center gap-2 px-6 py-3 rounded-md font-semibold transition-all ${
                    exportingInventory()
                      ? 'bg-slate-300 text-slate-500 cursor-not-allowed'
                      : 'bg-emerald-600 hover:bg-emerald-700 text-white'
                  }`}
                  disabled={exportingInventory()}
                  onClick={handleExportVMInventory}
                >
                  <Show when={exportingInventory()} fallback={<TableProperties size={20} />}>
                    <div class="w-5 h-5 border-2 border-white border-t-white rounded-full animate-spin" />
                  </Show>
                  {exportingInventory() ? 'Exporting...' : 'Export VM Inventory'}
                </button>
              </div>
            </section>
          </div>
        </OperationsPanel>

        <CalloutCard
          icon={<BarChart size={24} />}
          title="Advanced Insights"
          description="Performance reports come from the historical metrics store, while VM inventory export captures the current runtime state for spreadsheet-friendly fleet reviews. Use reports for trends and the inventory export for current allocation and usage snapshots."
          padding="lg"
        />
      </Show>
    </div>
  );
}
