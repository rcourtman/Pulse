import { Component, For, Show } from 'solid-js';
import { InfoCardFrame, InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';

interface TemperatureRow {
  label: string;
  value: string;
  valueTitle?: string;
}

interface TemperaturesCardProps {
  rows?: TemperatureRow[];
  title?: string;
  lastKnownReason?: string;
  wrapValues?: boolean;
}

export const TemperaturesCard: Component<TemperaturesCardProps> = (props) => {
  return (
    <Show when={props.rows?.length}>
      <InfoCardFrame>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
          {props.title || 'Temperatures'}
        </div>
        <Show when={props.lastKnownReason}>
          <p class="mb-2 whitespace-normal wrap-break-word text-xs text-muted" role="status">
            Last known readings, not current: {props.lastKnownReason}.
          </p>
        </Show>
        <div class="space-y-1.5 text-[11px]">
          <For each={props.rows}>
            {(row) => (
              <InfoCardKeyValueRow
                label={row.label}
                value={row.value}
                labelClass={
                  props.wrapValues ? 'max-w-[55%] lg:max-w-none shrink wrap-anywhere' : undefined
                }
                valueClass={props.wrapValues ? 'whitespace-normal wrap-anywhere' : 'truncate'}
                valueTitle={row.valueTitle || row.value}
              />
            )}
          </For>
        </div>
      </InfoCardFrame>
    </Show>
  );
};
