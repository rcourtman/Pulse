import { Show } from 'solid-js';
import { ProgressBar } from '@/components/shared/ProgressBar';
import { type MetricBarProps } from './metricBarModel';
import { useMetricBarState } from './useMetricBarState';

export function MetricBar(props: MetricBarProps) {
  const state = useMetricBarState(props);
  const presentation = state.presentation;

  return (
    <div
      ref={state.setContainerRef}
      class="metric-text w-full h-4 flex items-center justify-center min-w-0"
    >
      <ProgressBar
        value={presentation().width}
        class={`h-full ${props.class || ''}`}
        fillClass={presentation().progressColorClass}
        label={
          <Show when={presentation().showLabel}>
            <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content leading-none min-w-0 overflow-hidden">
              <span class="max-w-full min-w-0 whitespace-nowrap overflow-hidden text-ellipsis px-0.5 text-center">
                <span>{props.label}</span>
                <Show when={presentation().showSublabel}>
                  <span class="metric-sublabel font-normal text-muted"> ({props.sublabel})</span>
                </Show>
              </span>
            </span>
          </Show>
        }
      />
    </div>
  );
}
