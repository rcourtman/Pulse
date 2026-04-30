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
  const hasTrailingActions = () =>
    Boolean(
      local.toolbarTrailing ||
      activeUtilityActions() ||
      showColumnVisibility() ||
      showResetAction(),
    );
  const actionsLayout = () => local.actionsLayout ?? 'inline';
  const filterControlsClass = () =>
    local.filterControlsClass ??
    (actionsLayout() === 'stacked'
      ? 'page-controls-filter-controls flex w-full min-w-0 flex-none basis-full flex-wrap items-center gap-2'
      : 'page-controls-filter-controls flex min-w-0 flex-1 basis-0 flex-wrap items-center gap-2');
  const toolbarActionsClass = () =>
    local.toolbarActionsClass ??
    (actionsLayout() === 'stacked'
      ? 'page-controls-toolbar-actions inline-flex w-full flex-wrap items-center gap-2 border-t border-border-subtle pt-2'
      : 'page-controls-toolbar-actions ml-auto inline-flex shrink-0 flex-wrap items-center justify-end gap-2 self-start');
  const toolbarControls = () => (
    <>
      <div class={filterControlsClass()}>{local.children}</div>

      <div class={toolbarActionsClass()}>
        <Show when={local.toolbarTrailing}>{local.toolbarTrailing}</Show>

        <Show when={activeUtilityActions()}>
          <FilterDivider />
          {activeUtilityActions()}
        </Show>

        <Show when={showColumnVisibility()}>
          <FilterDivider />
          <ColumnPicker
            columns={local.columnVisibility!.availableToggles()}
            isHidden={local.columnVisibility!.isHiddenByUser}
            onToggle={local.columnVisibility!.toggle}
            onReset={local.columnVisibility!.resetToDefaults}
          />
        </Show>

        <Show when={showResetAction()}>
          <FilterDivider />
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
      <Show when={hasTrailingActions()}>
        <Show when={local.controlDeckClass} fallback={toolbarControls()}>
          {(controlDeckClass) => <div class={controlDeckClass()}>{toolbarControls()}</div>}
        </Show>
      </Show>

      <Show when={!hasTrailingActions()}>{local.children}</Show>
    </FilterHeader>
  );
};
