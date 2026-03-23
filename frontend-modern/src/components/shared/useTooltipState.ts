import { createEffect, createSignal } from 'solid-js';
import type { Accessor } from 'solid-js';
import {
  resolveTooltipPosition,
  sanitizeTooltipContent,
  type TooltipOptions,
  type TooltipPosition,
} from './tooltipModel';

interface TooltipStateOptions extends TooltipOptions {
  content: string;
  visible: boolean;
  x: number;
  y: number;
}

interface TooltipInstance {
  hide: () => void;
  show: (content: string, x: number, y: number, options?: TooltipOptions) => void;
}

let tooltipInstance: TooltipInstance | null = null;

export function useTooltipState(options: TooltipStateOptions): {
  position: Accessor<TooltipPosition>;
  sanitizedContent: Accessor<string>;
  setTooltipRef: (el: HTMLDivElement) => void;
} {
  let tooltipRef: HTMLDivElement | undefined;
  const [position, setPosition] = createSignal<TooltipPosition>({ left: 0, top: 0 });

  createEffect(() => {
    if (!options.visible) {
      setPosition({ left: options.x, top: options.y });
      return;
    }

    requestAnimationFrame(() => {
      if (!tooltipRef) return;

      const rect = tooltipRef.getBoundingClientRect();
      setPosition(
        resolveTooltipPosition({
          align: options.align,
          direction: options.direction,
          rect,
          viewportHeight: window.innerHeight,
          viewportWidth: window.innerWidth,
          x: options.x,
          y: options.y,
        }),
      );
    });
  });

  return {
    position,
    sanitizedContent: () => sanitizeTooltipContent(options.content),
    setTooltipRef: (el) => {
      tooltipRef = el;
    },
  };
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
