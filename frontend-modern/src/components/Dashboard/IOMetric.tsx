import { createMemo, Show, createEffect, createSignal } from 'solid-js';
import { formatSpeed } from '@/utils/format';
import { AnimatedMetric } from '@/components/shared/AnimatedMetric';

interface IOMetricProps {
  value: (() => number) | number;
  disabled?: boolean;
}

export function IOMetric(props: IOMetricProps) {
  // Handle both accessor functions and direct values
  const getValue = () => {
    return typeof props.value === 'function' ? props.value() : props.value;
  };
  
  // Create a local signal that tracks the value
  const [currentValue, setCurrentValue] = createSignal(getValue() || 0);
  
  // Update the signal when value changes
  createEffect(() => {
    const newValue = getValue() || 0;
    const oldValue = currentValue();
    if (newValue !== oldValue) {
      setCurrentValue(newValue);
    }
  });

  // Color based on speed (MB/s) - matching current dashboard
  const colorClass = createMemo(() => {
    if (props.disabled) return 'text-gray-400 dark:text-gray-500';
    
    const mbps = currentValue() / (1024 * 1024);
    if (mbps < 1) return 'text-gray-300 dark:text-gray-400';
    if (mbps < 10) return 'text-green-600 dark:text-green-400';
    if (mbps < 50) return 'text-yellow-600 dark:text-yellow-400';
    return 'text-red-600 dark:text-red-400';
  });

  return (
    <Show when={!props.disabled} fallback={<span class="text-sm text-gray-400">-</span>}>
      <div class={`text-sm font-mono ${colorClass()} overflow-visible relative`} style="min-height: 24px;">
        <AnimatedMetric 
          value={currentValue()} 
          formatter={(v) => formatSpeed(v, 0)}
        />
      </div>
    </Show>
  );
}