export type TooltipAlignment = 'left' | 'center';
export type TooltipDirection = 'up' | 'down';

export interface TooltipOptions {
  align?: TooltipAlignment;
  direction?: TooltipDirection;
  maxWidth?: number;
}

export interface TooltipViewportRect {
  height: number;
  width: number;
}

export interface TooltipPosition {
  left: number;
  top: number;
}

interface ResolveTooltipPositionOptions extends TooltipOptions {
  rect: TooltipViewportRect;
  viewportHeight: number;
  viewportWidth: number;
  x: number;
  y: number;
}

export function sanitizeTooltipContent(content: string): string {
  return content
    .replace(/<[^>]*>/g, '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#x27;');
}

export function resolveTooltipPosition(options: ResolveTooltipPositionOptions): TooltipPosition {
  const padding = 8;
  const viewportPadding = 4;
  const align = options.align ?? 'center';
  const direction = options.direction ?? 'up';

  let left = options.x;
  let top = options.y;

  if (align === 'center') {
    left = options.x - options.rect.width / 2;
  }

  if (direction === 'up') {
    top = options.y - options.rect.height - padding;
  } else {
    top = options.y + padding;
  }

  const maxLeft = options.viewportWidth - options.rect.width - viewportPadding;
  const maxTop = options.viewportHeight - options.rect.height - viewportPadding;

  left = Math.min(Math.max(left, viewportPadding), Math.max(maxLeft, viewportPadding));
  top = Math.min(Math.max(top, viewportPadding), Math.max(maxTop, viewportPadding));

  return { left, top };
}
