import { createSignal, For, Show, JSX } from 'solid-js';
import FileText from 'lucide-solid/icons/file-text';
import Download from 'lucide-solid/icons/download';
import BarChart from 'lucide-solid/icons/bar-chart';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { formField, formLabel, formHelpText, formControl, formSelect } from '@/components/shared/Form';
import { showSuccess, showWarning } from '@/utils/toast';
import { apiFetch } from '@/utils/apiClient';

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
    const [resourceType, setResourceType] = createSignal('node');
    const [resourceId, setResourceId] = createSignal('');
    const [metricType, setMetricType] = createSignal('');
    const [format, setFormat] = createSignal<'pdf' | 'csv'>('pdf');
    const [range, setRange] = createSignal('24h');
    const [generating, setGenerating] = createSignal(false);
    const [title, setTitle] = createSignal('');

    const handleGenerate = async () => {
        if (!resourceId()) {
            showWarning('Please enter a Resource ID');
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

            const params = new URLSearchParams({
                resourceType: resourceType(),
                resourceId: resourceId(),
                format: format(),
                start: startStr,
                end: end,
                title: title() || `Infrastructure Report - ${resourceId()}`,
            });

            if (metricType()) {
                params.append('metricType', metricType());
            }

            const response = await apiFetch(`/api/admin/reports/generate?${params.toString()}`);
            if (!response.ok) {
                const text = await response.text();
                throw new Error(text || 'Failed to generate report');
            }

            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `report-${resourceId()}-${new Date().toISOString().split('T')[0]}.${format()}`;
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
            <SectionHeader
                title={<>Advanced Reporting</>}
                description={<>Generate detailed infrastructure reports in PDF or CSV format.</>}
            />

            <Card>
                <div class="p-6 space-y-6">
                    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                        <FormField label="Resource Type" helpText="Select the type of infrastructure resource">
                            <select
                                id="resource-type"
                                class={formSelect}
                                value={resourceType()}
                                onChange={(e) => setResourceType(e.currentTarget.value)}
                            >
                                <option value="node">Proxmox Node</option>
                                <option value="vm">Virtual Machine (QEMU)</option>
                                <option value="container">Container (LXC)</option>
                                <option value="storage">Storage</option>
                            </select>
                        </FormField>

                        <FormField label="Resource ID" helpText="Enter the full ID of the resource">
                            <input
                                id="resource-id"
                                type="text"
                                class={formControl}
                                placeholder="e.g. pve1:node1 or pve1:node1:100"
                                value={resourceId()}
                                onInput={(e) => setResourceId(e.currentTarget.value)}
                            />
                        </FormField>
                    </div>

                    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                        <FormField label="Metric Type (Optional)" helpText="Filter by specific metric type">
                            <input
                                id="metric-type"
                                type="text"
                                class={formControl}
                                placeholder="e.g. cpu, memory, netin (leave empty for all)"
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
                            <div class="flex gap-2">
                                <For each={['24h', '7d', '30d']}>
                                    {(r) => (
                                        <button
                                            class={`px-4 py-2 rounded-lg border transition-all ${range() === r
                                                ? 'bg-blue-600/20 border-blue-500 text-blue-400'
                                                : 'bg-slate-800/50 border-slate-700 text-slate-400 hover:border-slate-500'
                                                }`}
                                            onClick={() => setRange(r)}
                                        >
                                            {r === '24h' ? 'Last 24 Hours' : r === '7d' ? 'Last 7 Days' : 'Last 30 Days'}
                                        </button>
                                    )}
                                </For>
                            </div>
                        </FormField>

                        <FormField label="Export Format">
                            <div class="flex gap-2">
                                <button
                                    class={`flex items-center gap-2 px-4 py-2 rounded-lg border transition-all ${format() === 'pdf'
                                        ? 'bg-blue-600/20 border-blue-500 text-blue-400'
                                        : 'bg-slate-800/50 border-slate-700 text-slate-400 hover:border-slate-500'
                                        }`}
                                    onClick={() => setFormat('pdf')}
                                >
                                    <FileText size={16} />
                                    PDF Report
                                </button>
                                <button
                                    class={`flex items-center gap-2 px-4 py-2 rounded-lg border transition-all ${format() === 'csv'
                                        ? 'bg-blue-600/20 border-blue-500 text-blue-400'
                                        : 'bg-slate-800/50 border-slate-700 text-slate-400 hover:border-slate-500'
                                        }`}
                                    onClick={() => setFormat('csv')}
                                >
                                    <BarChart size={16} />
                                    CSV Data
                                </button>
                            </div>
                        </FormField>
                    </div>

                    <div class="flex justify-end pt-4 border-t border-slate-800">
                        <button
                            class={`flex items-center gap-2 px-6 py-3 rounded-xl font-semibold transition-all ${generating()
                                ? 'bg-slate-700 text-slate-400 cursor-not-allowed'
                                : 'bg-blue-600 hover:bg-blue-500 text-white shadow-lg shadow-blue-900/20'
                                }`}
                            disabled={generating()}
                            onClick={handleGenerate}
                        >
                            <Show when={generating()} fallback={<Download size={20} />}>
                                <div class="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                            </Show>
                            {generating() ? 'Generating...' : 'Generate Report'}
                        </button>
                    </div>
                </div>
            </Card>

            <div class="bg-blue-900/10 border border-blue-900/20 rounded-xl p-6 mt-8">
                <div class="flex gap-4">
                    <div class="p-3 bg-blue-600/20 rounded-lg h-fit text-blue-400">
                        <BarChart size={24} />
                    </div>
                    <div>
                        <h3 class="text-lg font-bold text-white mb-2">Enterprise Insights</h3>
                        <p class="text-slate-400 leading-relaxed">
                            Reports are generated directly from the historical metrics store. PDF reports provide a summarized view with average, minimum, and maximum values, while CSV exports provide raw granular data for external analysis in tools like Excel or BI suites.
                        </p>
                    </div>
                </div>
            </div>
        </div>
    );
}
