import { Component, createSignal, createEffect, Show } from 'solid-js';
import { Portal } from 'solid-js/web';

type TooltipAlignment = 'left' | 'center';
type TooltipDirection = 'up' | 'down';

export interface TooltipOptions {
  align?: TooltipAlignment;
  direction?: TooltipDirection;
  maxWidth?: number;
}

interface TooltipProps extends TooltipOptions {
  content: string;
  x: number;
  y: number;
  visible: boolean;
}

// Sanitize tooltip content to prevent XSS
function sanitizeContent(content: string): string {
  // Remove any HTML tags and encode special characters
  return content
    .replace(/<[^>]*>/g, '') // Remove HTML tags
    .replace(/&/g, '&amp;') // Encode ampersands
    .replace(/</g, '&lt;') // Encode less than
    .replace(/>/g, '&gt;') // Encode greater than
    .replace(/"/g, '&quot;') // Encode quotes
    .replace(/'/g, '&#x27;'); // Encode apostrophes
}

const Tooltip: Component<TooltipProps> = (props) => {
  let tooltipRef: HTMLDivElement | undefined;
  const [position, setPosition] = createSignal({ left: 0, top: 0 });

  createEffect(() => {
    if (!props.visible) {
      setPosition({ left: props.x, top: props.y });
      return;
    }

    // Use requestAnimationFrame to ensure DOM is updated
    requestAnimationFrame(() => {
      if (!tooltipRef) return;

      const rect = tooltipRef.getBoundingClientRect();
      const padding = 8;
      let left = props.x;
      let top = props.y;

      const align = props.align ?? 'center';
      const direction = props.direction ?? 'up';

      if (align === 'center') {
        left = props.x - rect.width / 2;
      }

      if (direction === 'up') {
        top = props.y - rect.height - padding;
      } else {
        top = props.y + padding;
      }

      // Clamp to viewport bounds with small offset to avoid touching edges
      const viewportPadding = 4;
      const maxLeft = window.innerWidth - rect.width - viewportPadding;
      const maxTop = window.innerHeight - rect.height - viewportPadding;

      left = Math.min(Math.max(left, viewportPadding), Math.max(maxLeft, viewportPadding));
      top = Math.min(Math.max(top, viewportPadding), Math.max(maxTop, viewportPadding));

      setPosition({ left, top });
    });
  });

  return (
    <Show when={props.visible}>
      <Portal mount={document.body}>
        <div
          ref={tooltipRef}
          class="fixed z-[9999] px-3 py-2 text-xs whitespace-pre-line rounded-md border shadow-sm pointer-events-none bg-white text-slate-900 border-slate-200 leading-tight dark:bg-slate-800 dark:text-slate-100 dark:border-slate-700"
          style={{
            left: `${position().left}px`,
            top: `${position().top}px`,
            'max-width': `${props.maxWidth ?? 240}px`,
            opacity: props.visible ? '1' : '0',
            transition: 'opacity 120ms ease-out',
          }}
          textContent={sanitizeContent(props.content)}
        />
      </Portal>
    </Show>
  );
};

// Global tooltip singleton
let tooltipInstance: {
  show: (content: string, x: number, y: number, options?: TooltipOptions) => void;
  hide: () => void;
} | null = null;

export function createTooltipSystem() {
  const [visible, setVisible] = createSignal(false);
  const [content, setContent] = createSignal('');
  const [position, setPosition] = createSignal({ x: 0, y: 0 });
  const [options, setOptions] = createSignal<TooltipOptions>({});

  tooltipInstance = {
    show: (content: string, x: number, y: number, opts?: TooltipOptions) => {
      setContent(content);
      setPosition({ x, y });
      setOptions(opts || {});
      setVisible(true);
    },
    hide: () => {
      setVisible(false);
    },
  };

  return () => (
    <Tooltip
      content={content()}
      x={position().x}
      y={position().y}
      visible={visible()}
      align={options().align}
      direction={options().direction}
      maxWidth={options().maxWidth}
    />
  );
}

export function showTooltip(content: string, x: number, y: number, options?: TooltipOptions) {
  tooltipInstance?.show(content, x, y, options);
}

export function hideTooltip() {
  tooltipInstance?.hide();
}

export default Tooltip;
