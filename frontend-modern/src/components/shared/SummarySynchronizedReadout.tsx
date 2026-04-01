import type { Component } from 'solid-js';

export interface SummarySynchronizedReadoutProps {
  empty?: boolean;
  timestamp: number;
  value: string;
}

export const formatSummarySynchronizedReadoutTime = (timestamp: number): string =>
  new Date(timestamp).toLocaleString([], {
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });

export const SummarySynchronizedReadout: Component<
  SummarySynchronizedReadoutProps
> = (props) => (
  <span
    data-summary-sync-readout="true"
    data-summary-sync-empty={props.empty === true ? 'true' : 'false'}
    data-summary-sync-timestamp={props.timestamp}
    class={`whitespace-nowrap text-[10px] leading-none ${
      props.empty === true ? 'text-muted' : 'font-semibold text-base-content tabular-nums'
    }`.trim()}
    title={`Synced value at ${formatSummarySynchronizedReadoutTime(props.timestamp)}`}
  >
    {props.value}
  </span>
);

export default SummarySynchronizedReadout;
