import { Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { JSX } from 'solid-js';
import { useTooltipPortalState } from './useTooltipState';
import type { TooltipOptions } from './tooltipModel';

interface TooltipPortalProps extends TooltipOptions {
  maxWidth?: number;
  when: boolean;
  x: number;
  y: number;
  children: JSX.Element;
}

/**
 * Portal-mounted tooltip positioned above the trigger element.
 * Replaces 10+ copy-pasted Portal + fixed positioning blocks.
 */
export function TooltipPortal(props: TooltipPortalProps) {
  const state = useTooltipPortalState(props);

  return (
    <Show when={props.when}>
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
              data-tooltip-portal="true"
              class="inline-block max-w-full rounded-md border border-border bg-surface px-2 py-1.5 text-[10px] text-base-content shadow-lg"
            >
              {props.children}
            </div>
          </foreignObject>
        </svg>
      </Portal>
    </Show>
  );
}
