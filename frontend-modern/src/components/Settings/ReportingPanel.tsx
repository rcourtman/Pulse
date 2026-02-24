import { createSignal, For, Show, JSX } from 'solid-js';
import FileText from 'lucide-solid/icons/file-text';
import Download from 'lucide-solid/icons/download';
import BarChart from 'lucide-solid/icons/bar-chart';
import OperationsPanel from '@/components/Settings/OperationsPanel';
import { formField, formLabel, formHelpText, formControl } from '@/components/shared/Form';
import { showSuccess, showWarning } from '@/utils/toast';
import { apiFetch } from '@/utils/apiClient';
import { ResourcePicker, type SelectedResource } from './ResourcePicker';

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
  const [range, setRange] = createSignal('24h');
  const [generating, setGenerating] = createSignal(false);
  const [title, setTitle] = createSignal('');

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
          resourceType: res.type,
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
            resourceType: r.type,
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
      <OperationsPanel
        title="Detailed Reporting"
        description="Generate reports across infrastructure, workloads, storage, and backup resources."
        icon={<BarChart class="w-5 h-5" strokeWidth={2} />}
      >
        <div class="space-y-6 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
          <FormField label="Resources" helpText="Select the resources to include in the report">
            <ResourcePicker selected={selectedResources} onSelectionChange={setSelectedResources} />
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
              <div class="grid grid-cols-1 sm:flex gap-2">
                <For each={['24h', '7d', '30d']}>
                  {(r) => (
                    <button
                      class={`w-full sm:w-auto min-h-10 sm:min-h-9 px-4 py-2.5 rounded-md border transition-all ${range() === r ? 'bg-blue-50 border-blue-500 text-blue-700 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-500' : ' border-border text-base-content hover:bg-surface-alt'}`}
                      onClick={() => setRange(r)}
                    >
                      {r === '24h' ? 'Last 24 Hours' : r === '7d' ? 'Last 7 Days' : 'Last 30 Days'}
                    </button>
                  )}
                </For>
              </div>
            </FormField>

            <FormField label="Export Format">
              <div class="grid grid-cols-1 sm:flex gap-2">
                <button
                  class={`w-full sm:w-auto min-h-10 sm:min-h-9 flex items-center justify-center gap-2 px-4 py-2.5 rounded-md border transition-all ${format() === 'pdf' ? 'bg-blue-50 border-blue-500 text-blue-700 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-500' : ' border-border text-base-content hover:bg-surface-alt'}`}
                  onClick={() => setFormat('pdf')}
                >
                  <FileText size={16} />
                  PDF Report
                </button>
                <button
                  class={`w-full sm:w-auto min-h-10 sm:min-h-9 flex items-center justify-center gap-2 px-4 py-2.5 rounded-md border transition-all ${format() === 'csv' ? 'bg-blue-50 border-blue-500 text-blue-700 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-500' : ' border-border text-base-content hover:bg-surface-alt'}`}
                  onClick={() => setFormat('csv')}
                >
                  <BarChart size={16} />
                  CSV Data
                </button>
              </div>
            </FormField>
          </div>
        </div>

        <div class="flex justify-end p-4 sm:p-6 hover:bg-surface-hover transition-colors">
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

      <div class="rounded-md border border-blue-200 bg-blue-50 p-6 dark:border-blue-800 dark:bg-blue-900">
        <div class="flex flex-col sm:flex-row gap-4">
          <div class="p-3 rounded-md h-fit text-blue-600 dark:text-blue-300 bg-blue-100 dark:bg-blue-900">
            <BarChart size={24} />
          </div>
          <div>
            <h3 class="text-lg font-semibold text-blue-900 dark:text-blue-100 mb-2">
              Advanced Insights
            </h3>
            <p class="text-sm text-blue-800 dark:text-blue-200 leading-relaxed">
              Reports are generated directly from the historical metrics store. PDF reports provide
              summarized trends (average, minimum, and maximum), while CSV exports provide raw
              time-series data for deeper analysis in spreadsheets or BI tools. Select multiple
              resources to generate a fleet-wide summary.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
