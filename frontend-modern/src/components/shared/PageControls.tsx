import { Component, JSX, Show, splitProps } from 'solid-js';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import {
  FilterActionButton,
  FilterDivider,
  FilterHeader,
  FilterMobileToggleButton,
} from '@/components/shared/FilterToolbar';
import type { ColumnDef } from '@/hooks/useColumnVisibility';

interface PageControlsColumnVisibility {
  availableToggles: () => ColumnDef[];
  isHiddenByUser: (id: string) => boolean;
  toggle: (id: string) => void;
  resetToDefaults: () => void;
}

interface PageControlsResetAction {
  show: boolean;
  onClick: () => void;
  title?: string;
  label?: string;
  class?: string;
  icon?: JSX.Element;
}

interface PageControlsMobileFilters {
  enabled: boolean;
  count?: number;
  onToggle: () => void;
}

export const pageControlsControlDeckClass =
  'page-controls-control-deck grid w-full min-w-0 items-start gap-2 rounded-md border border-border bg-surface-alt p-1.5 shadow-sm xl:grid-cols-[minmax(0,1fr)_auto]';

export const pageControlsFilterSectionClass =
  'page-controls-filter-section rounded-md border border-border-subtle bg-surface px-1.5 py-1 shadow-sm';

export const pageControlsSectionedFilterControlsClass =
  'page-controls-filter-controls flex w-full min-w-0 flex-wrap items-start gap-2';

interface PageControlsProps extends JSX.HTMLAttributes<HTMLDivElement> {
  searchLeading?: JSX.Element;
  searchTrailing?: JSX.Element;
  search: JSX.Element;
  showFilters: boolean;
  children: JSX.Element;
  contentClass?: string;
  searchRowClass?: string;
  toolbarClass?: string;
  toolbarTrailing?: JSX.Element;
  mobileRowClass?: string;
  mobileLeading?: JSX.Element;
  mobileTrailing?: JSX.Element;
  utilityActions?: JSX.Element;
  columnVisibility?: PageControlsColumnVisibility;
  resetAction?: PageControlsResetAction;
  mobileFilters?: PageControlsMobileFilters;
  actionsLayout?: 'inline' | 'stacked';
  controlDeckClass?: string;
  filterControlsClass?: string;
  filterControlsVariant?: 'single-section' | 'sectioned-children';
  toolbarActionsClass?: string;
}

export const PageControls: Component<PageControlsProps> = (props) => {
  const [local, divProps] = splitProps(props, [
    'searchLeading',
    'searchTrailing',
    'search',
    'showFilters',
    'children',
    'contentClass',
    'searchRowClass',
    'toolbarClass',
    'toolbarTrailing',
    'mobileRowClass',
    'mobileLeading',
    'mobileTrailing',
    'utilityActions',
    'columnVisibility',
    'resetAction',
    'mobileFilters',
    'actionsLayout',
    'controlDeckClass',
    'filterControlsClass',
    'filterControlsVariant',
    'toolbarActionsClass',
    'class',
  ]);
  const mobileSearchAccessory = () => (
    <Show when={local.mobileFilters?.enabled}>
      <FilterMobileToggleButton
        onClick={() => local.mobileFilters?.onToggle()}
        count={local.mobileFilters?.count ?? 0}
      />
    </Show>
  );
  const mobileControlsEnabled = () => local.mobileFilters?.enabled === true;
  const activeMobileTrailing = () => (mobileControlsEnabled() ? local.mobileTrailing : undefined);
  const activeUtilityActions = () => (mobileControlsEnabled() ? undefined : local.utilityActions);
  const activeSearchTrailing = () => (mobileControlsEnabled() ? undefined : local.searchTrailing);
  const showColumnVisibility = () =>
    Boolean(local.columnVisibility && local.columnVisibility.availableToggles().length > 0);
  const showResetAction = () => local.resetAction?.show === true;
  const showToolbarTrailing = () => Boolean(local.toolbarTrailing);
  const showUtilityActions = () => Boolean(activeUtilityActions());
  const hasTrailingActions = () =>
    Boolean(
      showToolbarTrailing() || showUtilityActions() || showColumnVisibility() || showResetAction(),
    );
  const showDividerBeforeUtilityActions = () => showToolbarTrailing() && showUtilityActions();
  const showDividerBeforeColumnVisibility = () =>
    (showToolbarTrailing() || showUtilityActions()) && showColumnVisibility();
  const showDividerBeforeResetAction = () =>
    (showToolbarTrailing() || showUtilityActions() || showColumnVisibility()) && showResetAction();
  const actionsLayout = () => local.actionsLayout ?? 'stacked';
  const filtersUseSectionedChildren = () => local.filterControlsVariant === 'sectioned-children';
  const resolvedControlDeckClass = () =>
    local.controlDeckClass ??
    (actionsLayout() === 'stacked' ? pageControlsControlDeckClass : undefined);
  const filterControlsClass = () =>
    local.filterControlsClass ??
    (actionsLayout() === 'stacked'
      ? filtersUseSectionedChildren()
        ? pageControlsSectionedFilterControlsClass
        : `page-controls-filter-controls ${pageControlsFilterSectionClass} inline-flex w-fit max-w-full min-w-0 flex-wrap items-center gap-2 justify-self-start`
      : 'page-controls-filter-controls flex min-w-0 flex-1 basis-0 flex-wrap items-center gap-2');
  const toolbarActionsClass = () =>
    local.toolbarActionsClass ??
    (actionsLayout() === 'stacked'
      ? `page-controls-toolbar-actions ${pageControlsFilterSectionClass} inline-flex max-w-full flex-wrap items-center gap-2 xl:justify-self-end`
      : 'page-controls-toolbar-actions ml-auto inline-flex shrink-0 flex-wrap items-center justify-end gap-2 self-start');
  const toolbarControls = () => (
    <>
      <div class={filterControlsClass()}>{local.children}</div>

      <Show when={hasTrailingActions()}>
        <div class={toolbarActionsClass()}>
          <Show when={local.toolbarTrailing}>{local.toolbarTrailing}</Show>

          <Show when={activeUtilityActions()}>
            <Show when={showDividerBeforeUtilityActions()}>
              <FilterDivider />
            </Show>
            {activeUtilityActions()}
          </Show>

          <Show when={showColumnVisibility()}>
            <Show when={showDividerBeforeColumnVisibility()}>
              <FilterDivider />
            </Show>
            <ColumnPicker
              columns={local.columnVisibility!.availableToggles()}
              isHidden={local.columnVisibility!.isHiddenByUser}
              onToggle={local.columnVisibility!.toggle}
              onReset={local.columnVisibility!.resetToDefaults}
            />
          </Show>

          <Show when={showResetAction()}>
            <Show when={showDividerBeforeResetAction()}>
              <FilterDivider />
            </Show>
            <FilterActionButton
              onClick={() => local.resetAction?.onClick()}
              title={local.resetAction?.title}
              class={local.resetAction?.class}
            >
              {local.resetAction?.icon}
              {local.resetAction?.label ?? 'Reset'}
            </FilterActionButton>
          </Show>
        </div>
      </Show>
    </>
  );

  return (
    <FilterHeader
      {...divProps}
      searchLeading={local.searchLeading}
      search={local.search}
      searchAccessory={activeSearchTrailing() ?? mobileSearchAccessory()}
      showFilters={local.showFilters}
      contentClass={local.contentClass}
      searchRowClass={local.searchRowClass}
      toolbarClass={local.toolbarClass}
      mobileRowClass={local.mobileRowClass}
      mobileLeading={local.mobileLeading}
      mobileTrailing={activeMobileTrailing()}
      class={local.class}
    >
      <Show when={resolvedControlDeckClass()} fallback={toolbarControls()}>
        {(controlDeckClass) => <div class={controlDeckClass()}>{toolbarControls()}</div>}
      </Show>
    </FilterHeader>
  );
};
