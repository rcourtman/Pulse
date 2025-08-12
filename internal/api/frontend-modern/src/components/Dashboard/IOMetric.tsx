import { createMemo, Show } from 'solid-js';
import { formatSpeed } from '@/utils/format';

interface IOMetricProps {
  value: number;
  disabled?: boolean;
}

export function IOMetric(props: IOMetricProps) {
  const formatted = createMemo(() => formatSpeed(props.value, 0));
  
  // Color based on speed (MB/s) - matching current dashboard
  const colorClass = createMemo(() => {
    if (props.disabled) return 'text-gray-400 dark:text-gray-500';
    
    const mbps = props.value / (1024 * 1024);
    if (mbps < 1) return 'text-gray-300 dark:text-gray-400';
    if (mbps < 10) return 'text-green-600 dark:text-green-400';
    if (mbps < 50) return 'text-yellow-600 dark:text-yellow-400';
    return 'text-red-600 dark:text-red-400';
  });

  return (
    <Show when={!props.disabled} fallback={<span class="text-sm text-gray-400">-</span>}>
      <span class={`text-sm font-mono ${colorClass()}`}>
        {formatted()}
      </span>
    </Show>
  );
}