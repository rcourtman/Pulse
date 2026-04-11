import { Component, Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import { createTooltipSystemState, useTooltipState } from './useTooltipState';
import type { TooltipOptions } from './tooltipModel';

export { hideTooltip, showTooltip } from './useTooltipState';
export type { TooltipOptions } from './tooltipModel';

interface TooltipProps extends TooltipOptions {
  content: string;
  x: number;
  y: number;
  visible: boolean;
}

export const Tooltip: Component<TooltipProps> = (props) => {
  const state = useTooltipState(props);

  return (
    <Show when={props.visible}>
      <Portal mount={document.body}>
        <svg
          class="fixed inset-0 z-[9999] h-screen w-screen overflow-visible pointer-events-none"
          viewBox={`0 0 ${state.viewport().width} ${state.viewport().height}`}
          preserveAspectRatio="none"
          aria-hidden="true"
        >
          <foreignObject
            x={state.position().left}
            y={state.position().top}
            width={state.maxWidth()}
            height={state.viewport().height}
            overflow="visible"
          >
            <div
              ref={state.setTooltipRef}
              data-tooltip="true"
              class="inline-block max-w-full whitespace-pre-line rounded-md border border-border bg-surface px-3 py-2 text-xs leading-tight text-base-content shadow-sm"
            >
              {state.sanitizedContent()}
            </div>
          </foreignObject>
        </svg>
      </Portal>
    </Show>
  );
};

export function createTooltipSystem() {
  const state = createTooltipSystemState();

  return () => (
    <Tooltip
      content={state.content()}
      x={state.position().x}
      y={state.position().y}
      visible={state.visible()}
      align={state.options().align}
      direction={state.options().direction}
      maxWidth={state.options().maxWidth}
    />
  );
}

export default Tooltip;
