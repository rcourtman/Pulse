import { Show } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { JSX } from 'solid-js';

interface TooltipPortalProps {
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
  return (
    <Show when={props.when}>
      <Portal mount={document.body}>
        <div
          class="fixed z-[9999] pointer-events-none"
          style={{
            left: `${props.x}px`,
            top: `${props.y - 8}px`,
            transform: 'translate(-50%, -100%)',
          }}
        >
          <div
            class="text-[10px] rounded-md shadow-lg px-2 py-1.5 border border-slate-600"
            style={{ 'background-color': 'rgb(15, 23, 42)', color: 'rgb(248, 250, 252)' }}
          >
            {props.children}
          </div>
        </div>
      </Portal>
    </Show>
  );
}
