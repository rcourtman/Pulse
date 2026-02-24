/**
 * CollapsibleSection Component
 *
 * A reusable accordion-style section with animated expand/collapse.
 * Used to organize resource groups in the thresholds page.
 */

import { Component, Show, createSignal, createEffect, JSX } from 'solid-js';
import ChevronRight from 'lucide-solid/icons/chevron-right';
import ChevronDown from 'lucide-solid/icons/chevron-down';

export interface CollapsibleSectionProps {
  /** Unique identifier for the section */
  id: string;
  /** Section title */
  title: string;
  /** Number of resources in this section */
  resourceCount?: number;
  /** Whether the section is currently collapsed */
  collapsed?: boolean;
  /** Callback when collapse state changes */
  onToggle?: (collapsed: boolean) => void;
  /** Section content (children) */
  children: JSX.Element;
  /** Action buttons to show in the header (e.g., "Edit Defaults") */
  headerActions?: JSX.Element;
  /** Icon to show before the title */
  icon?: JSX.Element;
  /** Subtitle or description */
  subtitle?: string;
  /** Message to show when section is empty */
  emptyMessage?: string;
  /** Whether to show a visual indicator for global disable state */
  isGloballyDisabled?: boolean;
  /** Whether the section has unsaved changes */
  hasChanges?: boolean;
  /** Test ID for e2e testing */
  testId?: string;
}

export const CollapsibleSection: Component<CollapsibleSectionProps> = (props) => {
  // Local collapsed state if not controlled externally
  const [localCollapsed, setLocalCollapsed] = createSignal(props.collapsed ?? false);

  // Sync with external collapsed state
  createEffect(() => {
    if (props.collapsed !== undefined) {
      setLocalCollapsed(props.collapsed);
    }
  });

  const isCollapsed = () => {
    return props.collapsed !== undefined ? props.collapsed : localCollapsed();
  };

  const handleToggle = () => {
    const newState = !isCollapsed();
    setLocalCollapsed(newState);
    props.onToggle?.(newState);
  };

  const isEmpty = () => props.resourceCount === 0;
  const showEmpty = () => isCollapsed() === false && isEmpty() && props.emptyMessage;

  return (
    <div
      class={`rounded-md border transition-all duration-200 ${props.isGloballyDisabled ? ' bg-surface-alt opacity-60' : 'border-border bg-surface '} ${props.hasChanges ? 'ring-2 ring-blue-400 ring-opacity-50' : ''}`}
      data-testid={props.testId || `section-${props.id}`}
    >
      {/* Section Header */}
      <button
        type="button"
        onClick={handleToggle}
        class={`w-full flex items-center justify-between gap-3 px-4 py-3
 text-left cursor-pointer select-none
 hover:bg-surface-hover
 transition-colors duration-150
 ${isCollapsed() ? 'rounded-md' : 'rounded-t-lg border-b border-border'}`}
        aria-expanded={!isCollapsed()}
        aria-controls={`section-content-${props.id}`}
      >
        {/* Left side: Chevron + Icon + Title + Count */}
        <div class="flex items-center gap-3 min-w-0">
          {/* Expand/Collapse chevron */}
          <div class="flex-shrink-0 text-muted transition-transform duration-200">
            <Show when={isCollapsed()} fallback={<ChevronDown class="w-5 h-5" />}>
              <ChevronRight class="w-5 h-5" />
            </Show>
          </div>

          {/* Optional icon */}
          <Show when={props.icon}>
            <div class="flex-shrink-0 text-muted">{props.icon}</div>
          </Show>

          {/* Title and count */}
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h3 class="font-semibold text-base-content truncate">{props.title}</h3>
              <Show when={props.resourceCount !== undefined}>
                <span class="flex-shrink-0 inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-surface-alt text-muted">
                  {props.resourceCount}
                </span>
              </Show>
              <Show when={props.isGloballyDisabled}>
                <span class="flex-shrink-0 inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-400">
                  Disabled
                </span>
              </Show>
              <Show when={props.hasChanges}>
                <span
                  class="flex-shrink-0 w-2 h-2 rounded-full bg-blue-500"
                  title="Unsaved changes"
                />
              </Show>
            </div>
            <Show when={props.subtitle}>
              <p class="text-sm text-muted truncate">{props.subtitle}</p>
            </Show>
          </div>
        </div>

        {/* Right side: Header actions */}
        <div class="flex items-center gap-2 flex-shrink-0" onClick={(e) => e.stopPropagation()}>
          {props.headerActions}
        </div>
      </button>

      {/* Section Content */}
      <div
        id={`section-content-${props.id}`}
        class={`overflow-hidden transition-all duration-200 ease-in-out
 ${isCollapsed() ? 'max-h-0 opacity-0' : 'max-h-[5000px] opacity-100'}`}
      >
        <div class="p-4">
          <Show when={showEmpty()}>
            <div class="text-center py-8 text-muted">
              <p>{props.emptyMessage}</p>
            </div>
          </Show>
          <Show when={!isEmpty() || !props.emptyMessage}>{props.children}</Show>
        </div>
      </div>
    </div>
  );
};

/**
 * A button for section header actions (e.g., "Edit Defaults") */
export interface SectionActionButtonProps {
  label: string;
  onClick: () => void;
  icon?: JSX.Element;
  variant?: 'default' | 'primary' | 'danger';
  disabled?: boolean;
  title?: string;
}
export const SectionActionButton: Component<SectionActionButtonProps> = (props) => {
  const variantClasses = {
    default: 'text-slate-600 hover:text-base-content hover:bg-slate-100',
    primary:
      'text-blue-600 hover:text-blue-700 hover:bg-blue-50 dark:text-blue-400 dark:hover:text-blue-300 dark:hover:bg-blue-900',
    danger:
      'text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:text-red-300 dark:hover:bg-red-900',
  };
  return (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        props.onClick();
      }}
      disabled={props.disabled}
      title={props.title}
      class={`inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-sm font-medium
 transition-colors duration-150
 ${variantClasses[props.variant || 'default']}
 ${props.disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}
    >
      <Show when={props.icon}>{props.icon}</Show>
      {props.label}
    </button>
  );
};

/**
 * Nested group header within a section (e.g., node name grouping VMs)
 */
export interface NestedGroupHeaderProps {
  title: string;
  subtitle?: string;
  count?: number;
  collapsed?: boolean;
  onToggle?: () => void;
  actions?: JSX.Element;
  status?: 'online' | 'offline' | 'unknown';
  href?: string;
}

export const NestedGroupHeader: Component<NestedGroupHeaderProps> = (props) => {
  const statusColors = {
    online: 'bg-green-500',
    offline: 'bg-red-500',
    unknown: 'bg-slate-400',
  };

  return (
    <div
      class={`flex items-center justify-between gap-3 px-3 py-2 -mx-3 rounded-md
 ${props.onToggle ? 'cursor-pointer hover:bg-surface-hover' : ''}
 transition-colors duration-150`}
      onClick={props.onToggle}
    >
      <div class="flex items-center gap-2 min-w-0">
        <Show when={props.onToggle}>
          <div class="flex-shrink-0 text-slate-400">
            <Show when={props.collapsed} fallback={<ChevronDown class="w-4 h-4" />}>
              <ChevronRight class="w-4 h-4" />
            </Show>
          </div>
        </Show>

        <Show when={props.status}>
          <span
            class={`flex-shrink-0 w-2 h-2 rounded-full ${statusColors[props.status!]}`}
            title={props.status}
          />
        </Show>

        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <Show
              when={props.href}
              fallback={<span class="font-medium text-base-content truncate">{props.title}</span>}
            >
              <a
                href={props.href}
                target="_blank"
                rel="noopener noreferrer"
                class="font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 truncate"
                onClick={(e) => e.stopPropagation()}
              >
                {props.title}
              </a>
            </Show>
            <Show when={props.count !== undefined}>
              <span class="text-xs text-muted">({props.count})</span>
            </Show>
          </div>
          <Show when={props.subtitle}>
            <p class="text-xs text-muted truncate">{props.subtitle}</p>
          </Show>
        </div>
      </div>

      <Show when={props.actions}>
        <div class="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
          {props.actions}
        </div>
      </Show>
    </div>
  );
};

export default CollapsibleSection;
