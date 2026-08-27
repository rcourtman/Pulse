import { Component, For } from 'solid-js';
import { InfoCardFrame, InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';

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
    <InfoCardFrame>
      <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
        {props.title || 'Temperatures'}
      </div>
      <div class="space-y-1.5 text-[11px]">
        <For each={props.rows}>
          {(row) => (
            <InfoCardKeyValueRow
              label={row.label}
              value={row.value}
              valueClass="truncate"
              valueTitle={row.valueTitle || row.value}
            />
          )}
        </For>
      </div>
    </InfoCardFrame>
  );
};
