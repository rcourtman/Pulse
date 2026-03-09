import { Component, JSX, Show } from 'solid-js';
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
  const mobileSearchAccessory = () => (
    <Show when={props.mobileFilters?.enabled}>
      <FilterMobileToggleButton
        onClick={() => props.mobileFilters?.onToggle()}
        count={props.mobileFilters?.count ?? 0}
      />
    </Show>
  );

  return (
    <FilterHeader
      search={props.search}
      searchAccessory={mobileSearchAccessory()}
      showFilters={props.showFilters}
      contentClass={props.contentClass}
      searchRowClass={props.searchRowClass}
      toolbarClass={props.toolbarClass}
      mobileRowClass={props.mobileRowClass}
      mobileLeading={props.mobileLeading}
      mobileTrailing={props.mobileTrailing}
      class={props.class}
    >
      {props.children}

      <Show when={props.utilityActions}>
        <FilterDivider />
        {props.utilityActions}
      </Show>

      <Show
        when={props.columnVisibility && props.columnVisibility.availableToggles().length > 0}
      >
        <FilterDivider />
        <ColumnPicker
          columns={props.columnVisibility!.availableToggles()}
          isHidden={props.columnVisibility!.isHiddenByUser}
          onToggle={props.columnVisibility!.toggle}
          onReset={props.columnVisibility!.resetToDefaults}
        />
      </Show>

      <Show when={props.resetAction?.show}>
        <FilterDivider />
        <FilterActionButton
          onClick={() => props.resetAction?.onClick()}
          title={props.resetAction?.title}
          class={props.resetAction?.class}
        >
          {props.resetAction?.icon}
          {props.resetAction?.label ?? 'Reset'}
        </FilterActionButton>
      </Show>
    </FilterHeader>
  );
};
