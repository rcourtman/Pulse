import { Component, For } from 'solid-js';

interface TemperatureRow {
  label: string;
  value: string;
  valueTitle?: string;
}

interface TemperaturesCardProps {
  rows?: TemperatureRow[];
  title?: string;
}

export const TemperaturesCard: Component<TemperaturesCardProps> = (props) => {
  if (!props.rows || props.rows.length === 0) return null;

  return (
    <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600 dark:bg-slate-800">
      <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">{props.title || 'Temperatures'}</div>
      <div class="space-y-1.5 text-[11px]">
        <For each={props.rows}>
          {(row) => (
            <div class="flex items-center justify-between gap-2 min-w-0">
              <span class="text-muted shrink-0">{row.label}</span>
              <span class="font-medium text-slate-700 dark:text-slate-200 truncate" title={row.valueTitle || row.value}>{row.value}</span>
            </div>
          )}
        </For>
      </div>
    </div>
  );
};
