import { createSignal } from 'solid-js';

export interface TooltipPos {
  x: number;
  y: number;
}

/**
 * Shared tooltip positioning hook.
 *
 * Returns reactive signals for visibility and position, plus mouse handlers
 * that can be spread onto any element:
 *
 *   const tip = useTooltip();
 *   <div onMouseEnter={tip.onMouseEnter} onMouseLeave={tip.onMouseLeave}>
 *     ...
 *     <TooltipPortal when={tip.show()} x={tip.pos().x} y={tip.pos().y}>
 *       <span>Tooltip content</span>
 *     </TooltipPortal>
 *   </div>
 */
export function useTooltip() {
  const [show, setShow] = createSignal(false);
  const [pos, setPos] = createSignal<TooltipPos>({ x: 0, y: 0 });

  const onMouseEnter = (e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const hasPointerPosition = Number.isFinite(e.clientX) && Number.isFinite(e.clientY);
    setPos({
      x: hasPointerPosition ? e.clientX : rect.left + rect.width / 2,
      y: hasPointerPosition ? e.clientY : rect.top,
    });
    setShow(true);
  };

  const onMouseLeave = () => setShow(false);

  return { show, setShow, pos, onMouseEnter, onMouseLeave } as const;
}
