const HORIZONTAL_RAIL_EDGE_PADDING = 8;

export interface HorizontalRailVisibilityMetrics {
  scrollLeft: number;
  scrollWidth: number;
  clientWidth: number;
  itemOffsetLeft: number;
  itemOffsetWidth: number;
}

export function getHorizontalRailScrollLeft(options: HorizontalRailVisibilityMetrics): number {
  const maxScrollLeft = Math.max(0, options.scrollWidth - options.clientWidth);
  const visibleStart = options.scrollLeft;
  const visibleEnd = visibleStart + options.clientWidth;
  const itemStart = options.itemOffsetLeft;
  const itemEnd = itemStart + options.itemOffsetWidth;

  if (itemStart < visibleStart + HORIZONTAL_RAIL_EDGE_PADDING) {
    return Math.max(0, itemStart - HORIZONTAL_RAIL_EDGE_PADDING);
  }
  if (itemEnd > visibleEnd - HORIZONTAL_RAIL_EDGE_PADDING) {
    return Math.min(maxScrollLeft, itemEnd + HORIZONTAL_RAIL_EDGE_PADDING - options.clientWidth);
  }
  return Math.min(maxScrollLeft, Math.max(0, options.scrollLeft));
}
