import { createSignal, type Accessor } from 'solid-js';

export interface TooltipPosition {
  x: number;
  y: number;
}

export interface UseTooltipResult {
  /** Whether the tooltip is currently visible */
  showTooltip: Accessor<boolean>;
  /** Current tooltip position */
  tooltipPos: Accessor<TooltipPosition>;
  /** Handler for mouse enter - calculates position and shows tooltip */
  handleMouseEnter: (e: MouseEvent) => void;
  /** Handler for mouse leave - hides tooltip */
  handleMouseLeave: () => void;
}

/**
 * Hook to manage tooltip visibility and positioning.
 * Positions tooltip centered above the target element.
 *
 * @returns Object with tooltip state and event handlers
 *
 * @example
 * ```tsx
 * function MyComponent() {
 *   const { showTooltip, tooltipPos, handleMouseEnter, handleMouseLeave } = useTooltip();
 *
 *   return (
 *     <>
 *       <div onMouseEnter={handleMouseEnter} onMouseLeave={handleMouseLeave}>
 *         Hover me
 *       </div>
 *       <Show when={showTooltip()}>
 *         <Portal mount={document.body}>
 *           <div style={{ left: `${tooltipPos().x}px`, top: `${tooltipPos().y - 8}px` }}>
 *             Tooltip content
 *           </div>
 *         </Portal>
 *       </Show>
 *     </>
 *   );
 * }
 * ```
 */
export function useTooltip(): UseTooltipResult {
  const [showTooltip, setShowTooltip] = createSignal(false);
  const [tooltipPos, setTooltipPos] = createSignal<TooltipPosition>({ x: 0, y: 0 });

  const handleMouseEnter = (e: MouseEvent) => {
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    setTooltipPos({ x: rect.left + rect.width / 2, y: rect.top });
    setShowTooltip(true);
  };

  const handleMouseLeave = () => {
    setShowTooltip(false);
  };

  return {
    showTooltip,
    tooltipPos,
    handleMouseEnter,
    handleMouseLeave,
  };
}
