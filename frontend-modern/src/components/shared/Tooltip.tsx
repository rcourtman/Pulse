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
        <div
          ref={state.setTooltipRef}
          class="fixed z-[9999] px-3 py-2 text-xs whitespace-pre-line rounded-md border shadow-sm pointer-events-none bg-surface text-base-content border-border leading-tight"
          style={{
            left: `${state.position().left}px`,
            top: `${state.position().top}px`,
            'max-width': `${props.maxWidth ?? 240}px`,
            opacity: props.visible ? '1' : '0',
            transition: 'opacity 120ms ease-out',
          }}
          textContent={state.sanitizedContent()}
        />
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
