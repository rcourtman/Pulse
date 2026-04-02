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
  mobileRowClass?: string;
  mobileLeading?: JSX.Element;
  mobileTrailing?: JSX.Element;
  utilityActions?: JSX.Element;
  columnVisibility?: PageControlsColumnVisibility;
  resetAction?: PageControlsResetAction;
  mobileFilters?: PageControlsMobileFilters;
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
    'mobileRowClass',
    'mobileLeading',
    'mobileTrailing',
    'utilityActions',
    'columnVisibility',
    'resetAction',
    'mobileFilters',
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
      {local.children}

      <Show when={activeUtilityActions()}>
        <FilterDivider />
        {activeUtilityActions()}
      </Show>

      <Show
        when={local.columnVisibility && local.columnVisibility.availableToggles().length > 0}
      >
        <FilterDivider />
        <ColumnPicker
          columns={local.columnVisibility!.availableToggles()}
          isHidden={local.columnVisibility!.isHiddenByUser}
          onToggle={local.columnVisibility!.toggle}
          onReset={local.columnVisibility!.resetToDefaults}
        />
      </Show>

      <Show when={local.resetAction?.show}>
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
    </FilterHeader>
  );
};
