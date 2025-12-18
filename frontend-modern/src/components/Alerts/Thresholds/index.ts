/**
 * Alert Thresholds Components
 *
 * Public exports for the redesigned thresholds components.
 */

// Types
export * from './types';

// Hooks
export { useCollapsedSections, type UseCollapsedSectionsResult } from './hooks/useCollapsedSections';

// Components
export { ThresholdBadge, ThresholdBadgeGroup, type ThresholdBadgeProps, type ThresholdBadgeGroupProps } from './components/ThresholdBadge';
export { ResourceCard, type ResourceCardProps } from './components/ResourceCard';
export { GlobalDefaultsRow, type GlobalDefaultsRowProps } from './components/GlobalDefaultsRow';

// Sections
export { CollapsibleSection, SectionActionButton, NestedGroupHeader, type CollapsibleSectionProps, type SectionActionButtonProps, type NestedGroupHeaderProps } from './sections/CollapsibleSection';
export { ProxmoxNodesSection, type ProxmoxNodesSectionProps } from './sections/ProxmoxNodesSection';

// Re-export existing components for compatibility during migration
export { ResourceTable } from '../ResourceTable';
