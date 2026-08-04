import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';

const normalizeElementWidth = (width: number): number | null =>
  Number.isFinite(width) && width > 0 ? Math.round(width) : null;

export interface ObservedElementWidth {
  setElement: (element: HTMLElement) => void;
  width: Accessor<number | null>;
}

/**
 * Tracks the usable inline width of a rendered surface. Responsive tables use
 * this instead of the viewport so side panels and constrained layouts select
 * columns that actually fit their container.
 */
export const useObservedElementWidth = (): ObservedElementWidth => {
  const [element, setElement] = createSignal<HTMLElement>();
  const [width, setWidth] = createSignal<number | null>(null);

  createEffect(() => {
    const target = element();
    if (!target) return;

    const update = () => setWidth(normalizeElementWidth(target.clientWidth));
    update();

    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(update);
    observer.observe(target);
    onCleanup(() => observer.disconnect());
  });

  return { setElement, width };
};
