import { createSignal, onCleanup } from 'solid-js';

import { supportsHoverTooltips } from '@/components/shared/hoverCapability';

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
  let pointerDismissDocument: Document | undefined;

  const disarmPointerDismissal = () => {
    pointerDismissDocument?.removeEventListener('pointerdown', dismissTooltip, true);
    pointerDismissDocument = undefined;
  };

  const dismissTooltip = () => {
    setShow(false);
    disarmPointerDismissal();
  };

  const armPointerDismissal = (ownerDocument?: Document) => {
    disarmPointerDismissal();
    pointerDismissDocument =
      ownerDocument ?? (typeof document === 'undefined' ? undefined : document);
    pointerDismissDocument?.addEventListener('pointerdown', dismissTooltip, {
      capture: true,
      once: true,
    });
  };

  const onMouseEnter = (e: MouseEvent) => {
    if (!supportsHoverTooltips()) {
      dismissTooltip();
      return;
    }
    const currentTarget = e.currentTarget as HTMLElement;
    const rect = currentTarget.getBoundingClientRect();
    const hasPointerPosition = Number.isFinite(e.clientX) && Number.isFinite(e.clientY);
    setPos({
      x: hasPointerPosition ? e.clientX : rect.left + rect.width / 2,
      y: hasPointerPosition ? e.clientY : rect.top,
    });
    setShow(true);
    armPointerDismissal(currentTarget.ownerDocument);
  };

  const onMouseLeave = dismissTooltip;

  onCleanup(disarmPointerDismissal);

  return { show, setShow, pos, onMouseEnter, onMouseLeave } as const;
}
