import { For, JSX, Show } from 'solid-js';
import FileText from 'lucide-solid/icons/file-text';
import Download from 'lucide-solid/icons/download';
import BarChart from 'lucide-solid/icons/bar-chart';
import TableProperties from 'lucide-solid/icons/table-properties';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { formControl, formField, formHelpText, formLabel } from '@/components/shared/Form';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { useReportingPanelState } from '@/components/Settings/useReportingPanelState';
import type { ReportingFormat } from '@/components/Settings/reportingCatalogModel';
import { type ReportingRangeValue } from '@/components/Settings/reportingPanelModel';
import { trackUpgradeClicked } from '@/utils/upgradeMetrics';
import {
  getUpgradeActionButtonClass,
  UPGRADE_ACTION_LABEL,
  UPGRADE_TRIAL_LABEL,
  UPGRADE_TRIAL_LINK_CLASS,
} from '@/utils/upgradePresentation';
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

export function ReportingPanel() {
  const {
    canStartTrial,
    exportingInventory,
    format,
    handleExportVMInventory,
    generating,
    handleGenerate,
    handleStartTrial,
    isLocked,
    isReportingEnabled,
    metricType,
    range,
    reportingCatalog,
    reportingCatalogError,
    reportingCatalogLoading,
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

  const performanceReport = () => reportingCatalog()?.performanceReport ?? null;
  const inventoryDefinition = () => reportingCatalog()?.vmInventoryExport ?? null;
  const supportsMetricFilter = () => performanceReport()?.supportsMetricFilter ?? false;
  const supportsCustomTitle = () => performanceReport()?.supportsCustomTitle ?? false;
  const selectedRange = (): ReportingRangeValue => range() ?? performanceReport()!.defaultRange;
  const selectedFormat = (): ReportingFormat => format() ?? performanceReport()!.defaultFormat;
  const optionalFieldCount = () =>
    Number(supportsMetricFilter()) + Number(supportsCustomTitle());
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
      <Show when={isLocked()}>
        <OperationsPanel
          title={reportingCatalog()?.title ?? 'Detailed Reporting'}
          description={
            reportingCatalog()?.description ??
            'Generate performance reports and current-state exports across infrastructure and workloads.'
          }
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
          title={reportingCatalog()?.title ?? 'Detailed Reporting'}
          description={
            reportingCatalog()?.description ??
            'Generate performance reports and current-state exports across infrastructure and workloads.'
          }
          icon={<BarChart class="w-5 h-5" strokeWidth={2} />}
        >
          <div class="space-y-6 p-4 sm:p-6">
            <Show when={reportingCatalogLoading()}>
              <p class="text-sm text-muted">Loading reporting surfaces...</p>
            </Show>

            <Show when={reportingCatalogError()}>
              <p class="text-sm text-warning">{reportingCatalogError()}</p>
            </Show>

            <Show when={performanceReport() && inventoryDefinition()}>
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
                  <button
                    class={`flex w-full items-center justify-center gap-2 rounded-md px-6 py-3 font-semibold transition-all sm:w-auto ${
                      generating()
                        ? 'cursor-not-allowed bg-slate-300 text-slate-500'
                        : 'bg-blue-600 text-white hover:bg-blue-700'
                    }`}
                    disabled={generating()}
                    onClick={handleGenerate}
                  >
                    <Show when={generating()} fallback={<Download size={20} />}>
                      <div class="h-5 w-5 animate-spin rounded-full border-2 border-t-white border-white" />
                    </Show>
                    {generating()
                      ? 'Generating...'
                      : selectedResources().length > 0
                        ? `Generate Report (${selectedResources().length} resource${selectedResources().length !== 1 ? 's' : ''})`
                        : 'Generate Report'}
                  </button>
                </div>
              </section>
            </Show>

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
                  <button
                    class={`flex w-full items-center justify-center gap-2 rounded-md px-6 py-3 font-semibold transition-all sm:w-auto ${
                      exportingInventory()
                        ? 'cursor-not-allowed bg-slate-300 text-slate-500'
                        : 'bg-emerald-600 text-white hover:bg-emerald-700'
                    }`}
                    disabled={exportingInventory()}
                    onClick={handleExportVMInventory}
                  >
                    <Show when={exportingInventory()} fallback={<TableProperties size={20} />}>
                      <div class="h-5 w-5 animate-spin rounded-full border-2 border-t-white border-white" />
                    </Show>
                    {exportingInventory() ? 'Exporting...' : 'Export VM Inventory'}
                  </button>
                </div>
              </section>
            </Show>
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
