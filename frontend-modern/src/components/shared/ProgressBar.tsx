import type { Component, JSX } from 'solid-js';

interface ProgressBarProps {
  value: number;
  class?: string;
  fillClass?: string;
  label?: JSX.Element;
  overlays?: JSX.Element;
  onMouseEnter?: JSX.EventHandlerUnion<HTMLDivElement, MouseEvent>;
  onMouseLeave?: JSX.EventHandlerUnion<HTMLDivElement, MouseEvent>;
}

const clampPercent = (value: number): number => {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(value, 100));
};

export const ProgressBar: Component<ProgressBarProps> = (props) => {
  const width = () => clampPercent(props.value);

  return (
    <div
      class={`relative w-full overflow-hidden rounded bg-surface-hover ${props.class ?? ''}`}
      onMouseEnter={props.onMouseEnter}
      onMouseLeave={props.onMouseLeave}
    >
      <svg
        class="absolute inset-0 h-full w-full overflow-visible pointer-events-none"
        viewBox="0 0 100 100"
        preserveAspectRatio="none"
        aria-hidden="true"
      >
        <foreignObject data-progress-fill="true" x="0" y="0" width={width()} height="100">
          <div class={`progress-fill h-full w-full ${props.fillClass ?? ''}`} />
        </foreignObject>
      </svg>
      {props.overlays}
      {props.label}
    </div>
  );
};
