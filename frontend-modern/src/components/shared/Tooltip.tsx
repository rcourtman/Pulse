import { Component, createSignal, createEffect, Show } from 'solid-js';
import { Portal } from 'solid-js/web';

interface TooltipProps {
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
  const [position, setPosition] = createSignal({ x: 0, y: 0 });

  createEffect(() => {
    if (!props.visible) {
      setPosition({ x: props.x, y: props.y });
      return;
    }

    // Use requestAnimationFrame to ensure DOM is updated
    requestAnimationFrame(() => {
      if (!tooltipRef) return;

      // Calculate position to keep tooltip on screen
      const rect = tooltipRef.getBoundingClientRect();
      const padding = 20; // Increased padding for better separation

      let x = props.x + padding;
      let y = props.y - rect.height - padding - 10; // Extra 10px vertical separation

      // Keep within viewport
      if (x + rect.width > window.innerWidth) {
        x = props.x - rect.width - padding;
      }

      if (y < 0) {
        y = props.y + padding;
      }

      // Ensure x and y are not negative
      x = Math.max(0, x);
      y = Math.max(0, y);

      setPosition({ x, y });
    });
  });

  return (
    <Show when={props.visible}>
      <Portal mount={document.body}>
        <div
          ref={tooltipRef}
          class="fixed z-50 px-2 py-1 text-xs text-white bg-gray-900 dark:bg-gray-800 rounded shadow-lg pointer-events-none whitespace-nowrap"
          style={{
            left: '0',
            top: '0',
            transform: `translate(${position().x}px, ${position().y}px)`,
            opacity: props.visible ? '1' : '0',
            transition: 'opacity 200ms ease-out',
          }}
          textContent={sanitizeContent(props.content)}
        />
      </Portal>
    </Show>
  );
};

// Global tooltip singleton
let tooltipInstance: {
  show: (content: string, x: number, y: number) => void;
  hide: () => void;
} | null = null;

export function createTooltipSystem() {
  const [visible, setVisible] = createSignal(false);
  const [content, setContent] = createSignal('');
  const [position, setPosition] = createSignal({ x: 0, y: 0 });

  tooltipInstance = {
    show: (content: string, x: number, y: number) => {
      setContent(content);
      setPosition({ x, y });
      setVisible(true);
    },
    hide: () => {
      setVisible(false);
    },
  };

  return () => (
    <Tooltip content={content()} x={position().x} y={position().y} visible={visible()} />
  );
}

export function showTooltip(content: string, x: number, y: number) {
  tooltipInstance?.show(content, x, y);
}

export function hideTooltip() {
  tooltipInstance?.hide();
}

export default Tooltip;
