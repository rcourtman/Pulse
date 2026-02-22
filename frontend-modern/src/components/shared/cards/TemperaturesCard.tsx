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
    <div class="rounded border border-border bg-surface p-3 shadow-sm">
      <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">{props.title || 'Temperatures'}</div>
      <div class="space-y-1.5 text-[11px]">
        <For each={props.rows}>
          {(row) => (
            <div class="flex items-center justify-between gap-2 min-w-0">
              <span class="text-muted shrink-0">{row.label}</span>
              <span class="font-medium text-base-content truncate" title={row.valueTitle || row.value}>{row.value}</span>
            </div>
          )}
        </For>
      </div>
    </div>
  );
};
