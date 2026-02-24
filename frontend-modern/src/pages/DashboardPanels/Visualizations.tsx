import { Component, createMemo, For, Show } from 'solid-js';

interface MiniDonutProps {
  data: Array<{ value: number; color: string; tooltip?: string }>;
  size?: number;
  strokeWidth?: number;
  centerText?: string;
  className?: string; // Allow custom classes
}

export const MiniDonut: Component<MiniDonutProps> = (props) => {
  const size = props.size ?? 32;
  const strokeWidth = props.strokeWidth ?? 4;
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;

  const total = createMemo(() => props.data.reduce((acc, item) => acc + item.value, 0));

  return (
    <div
      class={`relative inline-flex items-center justify-center ${props.className || ''}`}
      style={{ width: `${size}px`, height: `${size}px` }}
    >
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        class={`absolute inset-0 rotate-[-90deg]`}
      >
        <For
          each={(() => {
            let accumulated = 0;
            const t = total();
            if (t === 0) return [];

            return props.data.map((item) => {
              const val = (item.value / t) * circumference;
              const offset = -accumulated;
              accumulated += val;
              return { color: item.color, value: val, offset };
            });
          })()}
        >
          {(segment) => (
            <circle
              cx={size / 2}
              cy={size / 2}
              r={radius}
              fill="none"
              stroke="currentColor"
              stroke-width={strokeWidth}
              stroke-dasharray={`${segment.value} ${circumference}`}
              stroke-dashoffset={segment.offset}
              class={segment.color}
            />
          )}
        </For>
      </svg>

      {props.centerText && (
        <span class="absolute text-[10px] font-bold text-base-content">{props.centerText}</span>
      )}
    </div>
  );
};

interface MiniGaugeProps {
  percent: number; // 0-100
  size?: number;
  strokeWidth?: number;
  color?: string;
}

export const MiniGauge: Component<MiniGaugeProps> = (props) => {
  const size = props.size ?? 32;
  const strokeWidth = props.strokeWidth ?? 4;
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  // Semi-circle gauge (180deg) or 3/4 gauge (270deg)? Let's do 270deg (open at bottom)
  const arcLength = circumference * 0.75;

  return (
    <div
      class="relative inline-flex items-center justify-center"
      style={{ width: `${size}px`, height: `${size}px` }}
    >
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} class="rotate-[135deg]">
        {/* Track */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="currentColor"
          stroke-width={strokeWidth}
          stroke-dasharray={`${arcLength} ${circumference}`}
          stroke-linecap="round"
          class="text-base-content"
        />
        {/* Value */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="currentColor"
          stroke-width={strokeWidth}
          stroke-dasharray={`${(props.percent / 100) * arcLength} ${circumference}`}
          stroke-linecap="round"
          class={props.color || (props.percent > 90 ? 'text-red-500' : 'text-cyan-500')}
        />
      </svg>
    </div>
  );
};

interface StackedBarProps {
  data: Array<{ label: string; value: number; color: string }>;
  height?: number;
  className?: string; // Allow custom classes
}

export const StackedBar: Component<StackedBarProps> = (props) => {
  const total = createMemo(() => props.data.reduce((sum, item) => sum + item.value, 0));

  return (
    <div
      class={`flex w-full overflow-hidden rounded-full bg-surface-alt ${props.className || ''}`}
      style={{ height: `${props.height || 8}px` }}
    >
      <For each={props.data}>
        {(item) => {
          const percent = createMemo(() => (total() > 0 ? (item.value / total()) * 100 : 0));
          return (
            <Show when={percent() > 0}>
              <div
                class={`h-full ${item.color} transition-all duration-500`}
                style={{ width: `${percent()}%` }}
                title={`${item.label}: ${item.value} (${Math.round(percent())}%)`}
              />
            </Show>
          );
        }}
      </For>
    </div>
  );
};
