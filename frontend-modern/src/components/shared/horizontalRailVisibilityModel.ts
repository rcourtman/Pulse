const DEFAULT_HORIZONTAL_RAIL_EDGE_PADDING = 8;

export interface HorizontalRailVisibilityMetrics {
  scrollLeft: number;
  scrollWidth: number;
  clientWidth: number;
  itemOffsetLeft: number;
  itemOffsetWidth: number;
  edgePadding?: number;
}

export function getHorizontalRailScrollLeft(options: HorizontalRailVisibilityMetrics): number {
  const maxScrollLeft = Math.max(0, options.scrollWidth - options.clientWidth);
  const edgePadding = Number.isFinite(options.edgePadding)
    ? Math.max(0, options.edgePadding ?? DEFAULT_HORIZONTAL_RAIL_EDGE_PADDING)
    : DEFAULT_HORIZONTAL_RAIL_EDGE_PADDING;
  const visibleStart = options.scrollLeft;
  const visibleEnd = visibleStart + options.clientWidth;
  const itemStart = options.itemOffsetLeft;
  const itemEnd = itemStart + options.itemOffsetWidth;

  if (itemStart < visibleStart + edgePadding) {
    return Math.max(0, itemStart - edgePadding);
  }
  if (itemEnd > visibleEnd - edgePadding) {
    return Math.min(maxScrollLeft, itemEnd + edgePadding - options.clientWidth);
  }
  return Math.min(maxScrollLeft, Math.max(0, options.scrollLeft));
}
