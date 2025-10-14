import { createMemo } from 'solid-js';

interface MetricBarProps {
  value: number;
  label: string;
  sublabel?: string;
  type?: 'cpu' | 'memory' | 'disk' | 'generic';
}

export function MetricBar(props: MetricBarProps) {
  const width = createMemo(() => Math.min(props.value, 100));

  // Get color based on percentage and metric type (matching original)
  const getColor = createMemo(() => {
    const percentage = props.value;
    const metric = props.type || 'generic';

    if (metric === 'cpu') {
      if (percentage >= 90) return 'red';
      if (percentage >= 80) return 'yellow';
      return 'green';
    } else if (metric === 'memory') {
      if (percentage >= 85) return 'red';
      if (percentage >= 75) return 'yellow';
      return 'green';
    } else if (metric === 'disk') {
      if (percentage >= 90) return 'red';
      if (percentage >= 80) return 'yellow';
      return 'green';
    } else {
      if (percentage >= 90) return 'red';
      if (percentage >= 75) return 'yellow';
      return 'green';
    }
  });

  // Map color to CSS classes
  const progressColorClass = createMemo(() => {
    const colorMap = {
      red: 'bg-red-500/60 dark:bg-red-500/50',
      yellow: 'bg-yellow-500/60 dark:bg-yellow-500/50',
      green: 'bg-green-500/60 dark:bg-green-500/50',
    };
    return colorMap[getColor()] || 'bg-gray-500/60 dark:bg-gray-500/50';
  });

  // Combine label and sublabel for display text
  const displayText = createMemo(() => {
    if (props.sublabel) {
      return `${props.label} (${props.sublabel})`;
    }
    return props.label;
  });

  return (
    <div class="metric-text">
      <div class="relative min-w-[96px] w-full h-3.5 rounded overflow-hidden bg-gray-200 dark:bg-gray-600">
        <div
          class={`absolute top-0 left-0 h-full ${progressColorClass()}`}
          style={{ width: `${width()}%` }}
        />
        <span class="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-gray-800 dark:text-gray-100 leading-none">
          <span class="whitespace-nowrap px-0.5">{displayText()}</span>
        </span>
      </div>
    </div>
  );
}
