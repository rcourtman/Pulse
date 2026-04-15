import { createEffect, createSignal, onCleanup } from 'solid-js';
import type { Accessor } from 'solid-js';
import {
  resolveTooltipPosition,
  sanitizeTooltipContent,
  type TooltipOptions,
  type TooltipPosition,
  type TooltipViewportRect,
} from './tooltipModel';

interface TooltipStateOptions extends TooltipOptions {
  content: string;
  visible: boolean;
  x: number;
  y: number;
}

interface TooltipPortalStateOptions extends TooltipOptions {
  maxWidth?: number;
  when: boolean;
  x: number;
  y: number;
}

interface TooltipLayoutStateOptions {
  align: Accessor<TooltipOptions['align'] | undefined>;
  direction: Accessor<TooltipOptions['direction'] | undefined>;
  maxWidth: Accessor<number | undefined>;
  visible: Accessor<boolean>;
  x: Accessor<number>;
  y: Accessor<number>;
}

interface TooltipInstance {
  hide: () => void;
  show: (content: string, x: number, y: number, options?: TooltipOptions) => void;
}

let tooltipInstance: TooltipInstance | null = null;

function useTooltipLayoutState(options: TooltipLayoutStateOptions): {
  maxWidth: Accessor<number>;
  position: Accessor<TooltipPosition>;
  setTooltipRef: (el: HTMLDivElement) => void;
  viewport: Accessor<TooltipViewportRect>;
} {
  let tooltipRef: HTMLDivElement | undefined;
  let positionRafId: number | null = null;
  const [position, setPosition] = createSignal<TooltipPosition>({ left: 0, top: 0 });
  const [viewport, setViewport] = createSignal<TooltipViewportRect>({
    height: typeof window === 'undefined' ? 0 : window.innerHeight,
    width: typeof window === 'undefined' ? 0 : window.innerWidth,
  });

  const updateViewport = () => {
    if (typeof window === 'undefined') return;
    setViewport({ height: window.innerHeight, width: window.innerWidth });
  };

  const updatePosition = () => {
    if (typeof window === 'undefined' || !tooltipRef || !options.visible()) return;
    updateViewport();
    const rect = tooltipRef.getBoundingClientRect();
    setPosition(
      resolveTooltipPosition({
        align: options.align(),
        direction: options.direction(),
        rect,
        viewportHeight: window.innerHeight,
        viewportWidth: window.innerWidth,
        x: options.x(),
        y: options.y(),
      }),
    );
  };

  const schedulePositionUpdate = () => {
    if (typeof window === 'undefined') return;
    if (positionRafId !== null) {
      window.cancelAnimationFrame(positionRafId);
    }
    positionRafId = window.requestAnimationFrame(() => {
      positionRafId = null;
      updatePosition();
    });
  };

  createEffect(() => {
    if (typeof window === 'undefined') return;
    updateViewport();
    const handleResize = () => {
      updateViewport();
      if (options.visible()) {
        schedulePositionUpdate();
      }
    };
    window.addEventListener('resize', handleResize);
    onCleanup(() => window.removeEventListener('resize', handleResize));
  });

  createEffect(() => {
    const visible = options.visible();
    // Read the live layout inputs here so pointer-driven coordinate changes
    // retrigger portal positioning while the tooltip remains visible.
    const layoutInputs = {
      align: options.align(),
      direction: options.direction(),
      maxWidth: options.maxWidth(),
      x: options.x(),
      y: options.y(),
    };

    if (!visible) {
      if (typeof window !== 'undefined' && positionRafId !== null) {
        window.cancelAnimationFrame(positionRafId);
        positionRafId = null;
      }
      setPosition({ left: layoutInputs.x, top: layoutInputs.y });
      return;
    }

    schedulePositionUpdate();
  });

  onCleanup(() => {
    if (typeof window !== 'undefined' && positionRafId !== null) {
      window.cancelAnimationFrame(positionRafId);
      positionRafId = null;
    }
  });

  return {
    maxWidth: () =>
      Math.max(48, Math.min(options.maxWidth() ?? 240, Math.max(48, viewport().width - 8))),
    position,
    setTooltipRef: (el) => {
      tooltipRef = el;
      if (options.visible()) {
        schedulePositionUpdate();
      }
    },
    viewport,
  };
}

export function useTooltipState(options: TooltipStateOptions): {
  maxWidth: Accessor<number>;
  position: Accessor<TooltipPosition>;
  sanitizedContent: Accessor<string>;
  setTooltipRef: (el: HTMLDivElement) => void;
  viewport: Accessor<TooltipViewportRect>;
} {
  const layout = useTooltipLayoutState({
    align: () => options.align,
    direction: () => options.direction,
    maxWidth: () => options.maxWidth,
    visible: () => options.visible,
    x: () => options.x,
    y: () => options.y,
  });

  return {
    maxWidth: layout.maxWidth,
    position: layout.position,
    sanitizedContent: () => sanitizeTooltipContent(options.content),
    setTooltipRef: layout.setTooltipRef,
    viewport: layout.viewport,
  };
}

export function useTooltipPortalState(
  options: TooltipPortalStateOptions,
): {
  maxWidth: Accessor<number>;
  position: Accessor<TooltipPosition>;
  setTooltipRef: (el: HTMLDivElement) => void;
  viewport: Accessor<TooltipViewportRect>;
} {
  return useTooltipLayoutState({
    align: () => options.align,
    direction: () => options.direction,
    maxWidth: () => options.maxWidth ?? 320,
    visible: () => options.when,
    x: () => options.x,
    y: () => options.y,
  });
}

export function createTooltipSystemState() {
  const [visible, setVisible] = createSignal(false);
  const [content, setContent] = createSignal('');
  const [position, setPosition] = createSignal({ x: 0, y: 0 });
  const [options, setOptions] = createSignal<TooltipOptions>({});

  tooltipInstance = {
    show: (contentValue, x, y, nextOptions) => {
      setContent(contentValue);
      setPosition({ x, y });
      setOptions(nextOptions ?? {});
      setVisible(true);
    },
    hide: () => {
      setVisible(false);
    },
  };

  return {
    content,
    options,
    position,
    visible,
  };
}

export function showTooltip(content: string, x: number, y: number, options?: TooltipOptions) {
  tooltipInstance?.show(content, x, y, options);
}

export function hideTooltip() {
  tooltipInstance?.hide();
}
