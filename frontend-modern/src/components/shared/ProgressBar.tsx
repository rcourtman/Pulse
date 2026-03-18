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
  const width = () => `${clampPercent(props.value)}%`;

  return (
    <div
      class={`relative w-full overflow-hidden rounded bg-surface-hover ${props.class ?? ''}`}
      onMouseEnter={props.onMouseEnter}
      onMouseLeave={props.onMouseLeave}
    >
      <div
        class={`progress-fill absolute inset-y-0 left-0 ${props.fillClass ?? ''}`}
        style={{ width: width() }}
      />
      {props.overlays}
      {props.label}
    </div>
  );
};
