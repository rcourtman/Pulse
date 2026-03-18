import { createSignal, createEffect, Show, JSX } from 'solid-js';
import FileText from 'lucide-solid/icons/file-text';
import Download from 'lucide-solid/icons/download';
import BarChart from 'lucide-solid/icons/bar-chart';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { formField, formLabel, formHelpText, formControl } from '@/components/shared/Form';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { showSuccess, showWarning } from '@/utils/toast';
import { apiFetch } from '@/utils/apiClient';
import { ResourcePicker, type SelectedResource } from './ResourcePicker';
import {
  hasFeature,
  licenseLoaded,
  loadLicenseStatus,
  getUpgradeActionUrlOrFallback,
  startProTrial,
  entitlements,
} from '@/stores/license';
import { trackPaywallViewed, trackUpgradeClicked } from '@/utils/upgradeMetrics';
import { toReportingResourceType } from '@/utils/reportingResourceTypes';
import { REPORTING_RANGE_OPTIONS, type ReportingRangeOption } from '@/utils/reportingPresentation';
import {
  getProTrialStartedMessage,
  getTrialAlreadyUsedMessage,
  getTrialStartErrorMessage,
  getUpgradeActionButtonClass,
  UPGRADE_ACTION_LABEL,
  UPGRADE_TRIAL_LABEL,
  UPGRADE_TRIAL_LINK_CLASS,
} from '@/utils/upgradePresentation';

type ReportingRangeValue = ReportingRangeOption['value'];

const REPORTING_RANGE_FILTER_OPTIONS: FilterOption<ReportingRangeValue>[] = REPORTING_RANGE_OPTIONS.map(
  (option) => ({
    label: option.label,
    value: option.value,
  }),
);

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
  const [selectedResources, setSelectedResources] = createSignal<SelectedResource[]>([]);
  const [metricType, setMetricType] = createSignal('');
  const [format, setFormat] = createSignal<'pdf' | 'csv'>('pdf');
  const [range, setRange] = createSignal<ReportingRangeValue>('24h');
  const [generating, setGenerating] = createSignal(false);
  const [title, setTitle] = createSignal('');
  const [startingTrial, setStartingTrial] = createSignal(false);

  loadLicenseStatus();

  const isLocked = () => licenseLoaded() && !hasFeature('advanced_reporting');
  const canStartTrial = () => entitlements()?.trial_eligible !== false;

  createEffect((wasVisible: boolean) => {
    const visible = isLocked();
    if (visible && !wasVisible) {
      trackPaywallViewed('advanced_reporting', 'settings_reporting_panel');
    }
    return visible;
  }, false);

  const handleStartTrial = async () => {
    if (startingTrial()) return;
    setStartingTrial(true);
    try {
      const result = await startProTrial();
      if (result?.outcome === 'redirect') {
        window.location.href = result.actionUrl;
        return;
      }
      showSuccess(getProTrialStartedMessage());
    } catch (err) {
      const statusCode = (err as { status?: number } | null)?.status;
      if (statusCode === 409) {
        showWarning(getTrialAlreadyUsedMessage());
      } else {
        showWarning(getTrialStartErrorMessage(err instanceof Error ? err.message : undefined));
      }
    } finally {
      setStartingTrial(false);
    }
  };

  const handleGenerate = async () => {
    const resources = selectedResources();
    if (resources.length === 0) {
      showWarning('Please select at least one resource');
      return;
    }

    setGenerating(true);
    try {
      const end = new Date().toISOString();
      let start = new Date();
      if (range() === '24h') start.setHours(start.getHours() - 24);
      else if (range() === '7d') start.setDate(start.getDate() - 7);
      else if (range() === '30d') start.setDate(start.getDate() - 30);

      const startStr = start.toISOString();

      let response: Response;
      let filename: string;

      if (resources.length === 1) {
        const res = resources[0];
        const params = new URLSearchParams({
          resourceType: toReportingResourceType(res.type),
          resourceId: res.id,
          format: format(),
          start: startStr,
          end: end,
          title: title() || `Pulse Report - ${res.name}`,
        });

        if (metricType()) {
          params.append('metricType', metricType());
        }

        response = await apiFetch(`/api/admin/reports/generate?${params.toString()}`);
        filename = `report-${res.name}-${new Date().toISOString().split('T')[0]}.${format()}`;
      } else {
        const body = {
          resources: resources.map((r) => ({
            resourceType: toReportingResourceType(r.type),
            resourceId: r.id,
          })),
          format: format(),
          start: startStr,
          end: end,
          title: title() || 'Pulse Fleet Report',
          metricType: metricType() || undefined,
        };

        response = await apiFetch('/api/admin/reports/generate-multi', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
        filename = `fleet-report-${new Date().toISOString().split('T')[0]}.${format()}`;
      }

      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || 'Failed to generate report');
      }

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);

      showSuccess('Report generated successfully');
    } catch (err) {
      console.error('Report generation error:', err);
      showWarning(err instanceof Error ? err.message : 'Failed to generate report');
    } finally {
      setGenerating(false);
    }
  };

  return (
    <div class="space-y-6">
      <Show when={isLocked()}>
        <OperationsPanel
          title="Detailed Reporting"
          description="Generate reports across infrastructure, workloads, storage, and backup resources."
          icon={<BarChart class="w-5 h-5" strokeWidth={2} />}
        >
          <div class="p-4 sm:p-6">
            <div class="flex flex-col sm:flex-row items-center gap-4">
              <div class="flex-1 text-center sm:text-left">
                <h4 class="text-base font-semibold text-base-content">Advanced Reporting (Pro)</h4>
                <p class="text-sm text-muted mt-1">
                  Generate PDF and CSV reports across infrastructure, workloads, storage, and backup
                  resources. Includes trend summaries, fleet-wide reporting, and raw data export.
                </p>
              </div>
              <div class="flex flex-col sm:flex-row items-center gap-2">
                <a
                  href={getUpgradeActionUrlOrFallback('advanced_reporting')}
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

      <Show when={licenseLoaded() && hasFeature('advanced_reporting')}>
        <OperationsPanel
          title="Detailed Reporting"
          description="Generate reports across infrastructure, workloads, storage, and backup resources."
          icon={<BarChart class="w-5 h-5" strokeWidth={2} />}
        >
          <div class="space-y-6 p-4 sm:p-6">
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
          </div>

          <div class="flex justify-end p-4 sm:p-6">
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
        </OperationsPanel>

        <CalloutCard
          icon={<BarChart size={24} />}
          title="Advanced Insights"
          description="Reports are generated directly from the historical metrics store. PDF reports provide summarized trends (average, minimum, and maximum), while CSV exports provide raw time-series data for deeper analysis in spreadsheets or BI tools. Select multiple resources to generate a fleet-wide summary."
          padding="lg"
        />
      </Show>
    </div>
  );
}
