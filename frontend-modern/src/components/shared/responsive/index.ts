// Responsive table components and utilities
// These provide a unified system for responsive column visibility across all tables

export { ResponsiveHeader, StickyHeader } from './ResponsiveHeader';
export { ResponsiveMetricCell, MetricText, DualMetricCell } from './ResponsiveMetricCell';
export { useGridTemplate, generateStaticGridTemplate, generateResponsiveTemplates, getColumnVisibilityClass } from './useGridTemplate';

// Re-export types for convenience
export type { ResponsiveHeaderProps } from './ResponsiveHeader';
export type { ResponsiveMetricCellProps } from './ResponsiveMetricCell';
export type { GridTemplateOptions, GridTemplateResult } from './useGridTemplate';
