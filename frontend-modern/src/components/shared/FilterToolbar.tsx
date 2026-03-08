import { Component, For, JSX, Show, splitProps } from 'solid-js';
import ListFilterIcon from 'lucide-solid/icons/list-filter';
import { segmentedButtonClass } from '@/utils/segmentedButton';

export const filterToolbarShellClass = '';
export const filterToolbarRowClass = 'flex flex-wrap items-center gap-2 text-xs text-muted';
export const filterToolbarContentClass = 'flex flex-col gap-2';
export const filterToolbarSearchRowClass = 'flex items-center gap-2';
export const filterGroupClass = 'inline-flex items-center gap-1 rounded-md bg-surface-hover p-0.5';
export const filterLabelClass =
  'px-1.5 text-[9px] font-semibold uppercase tracking-wide text-muted';
export const filterActionButtonClass =
  'inline-flex items-center gap-2 rounded-md border border-border bg-surface px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover';
export const filterActionButtonActiveClass =
  'border-blue-200 bg-surface text-base-content ring-1 ring-blue-200 dark:border-blue-900 dark:ring-blue-900';
export const filterSelectClass =
  'rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content outline-none focus:border-blue-500';
export const filterDividerClass = 'hidden h-5 w-px bg-surface-hover sm:block';
export const filterPanelClass =
  'absolute right-0 top-[calc(100%+0.5rem)] z-20 w-[min(40rem,calc(100vw-2rem))] rounded-md border border-border bg-surface p-3 shadow-lg';
export const filterPanelTitleClass = 'text-sm font-medium text-base-content';
export const filterPanelDescriptionClass = 'text-xs text-muted';
export const mobileFiltersButtonClass =
  'flex items-center gap-1.5 rounded-md bg-surface-hover px-2.5 py-1.5 text-xs font-medium text-muted';
export const filterCountBadgeClass =
  'ml-0.5 rounded-full bg-blue-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white';
export const filterUtilityBadgeClass =
  'ml-0.5 inline-flex items-center whitespace-nowrap rounded-md bg-surface-alt px-1.5 py-px text-[10px] font-medium text-base-content';

interface FilterToolbarShellProps extends JSX.HTMLAttributes<HTMLDivElement> {
  children: JSX.Element;
}

export const FilterToolbarShell: Component<FilterToolbarShellProps> = (props) => {
  const [local, divProps] = splitProps(props, ['children', 'class']);
  return (
    <div {...divProps} class={`${filterToolbarShellClass} ${local.class ?? ''}`.trim()}>
      {local.children}
    </div>
  );
};

interface FilterHeaderProps extends JSX.HTMLAttributes<HTMLDivElement> {
  search: JSX.Element;
  showFilters: boolean;
  children: JSX.Element;
  layout?: 'stacked' | 'inline';
  contentClass?: string;
  searchRowClass?: string;
  toolbarClass?: string;
  mobileRowClass?: string;
  searchAccessory?: JSX.Element;
  mobileLeading?: JSX.Element;
  mobileTrailing?: JSX.Element;
}

export const FilterHeader: Component<FilterHeaderProps> = (props) => {
  const [local, divProps] = splitProps(props, [
    'search',
    'showFilters',
    'children',
    'layout',
    'contentClass',
    'searchRowClass',
    'toolbarClass',
    'mobileRowClass',
    'searchAccessory',
    'mobileLeading',
    'mobileTrailing',
    'class',
  ]);
  const hasMobileRow = () => Boolean(local.mobileLeading || local.mobileTrailing);

  return (
    <FilterToolbarShell {...divProps} class={local.class}>
      {local.layout === 'inline' ? (
        <FilterToolbarRow class={local.contentClass}>
          {local.search}
          <Show when={hasMobileRow()}>
            <FilterToolbarSearchRow
              class={`w-full justify-between sm:hidden ${local.mobileRowClass ?? ''}`.trim()}
            >
              {local.mobileLeading ?? <span />}
              {local.mobileTrailing ?? <span />}
            </FilterToolbarSearchRow>
          </Show>
          <Show when={local.showFilters}>
            <FilterToolbarRow class={local.toolbarClass}>{local.children}</FilterToolbarRow>
          </Show>
        </FilterToolbarRow>
      ) : (
        <FilterToolbarContent class={local.contentClass}>
          <FilterToolbarSearchRow class={local.searchRowClass}>
            {local.search}
            {local.searchAccessory}
          </FilterToolbarSearchRow>
          <Show when={hasMobileRow()}>
            <FilterToolbarSearchRow
              class={`justify-between sm:hidden ${local.mobileRowClass ?? ''}`.trim()}
            >
              {local.mobileLeading ?? <span />}
              {local.mobileTrailing ?? <span />}
            </FilterToolbarSearchRow>
          </Show>
          <Show when={local.showFilters}>
            <FilterToolbarRow class={local.toolbarClass}>{local.children}</FilterToolbarRow>
          </Show>
        </FilterToolbarContent>
      )}
    </FilterToolbarShell>
  );
};

interface FilterToolbarContentProps extends JSX.HTMLAttributes<HTMLDivElement> {
  children: JSX.Element;
}

export const FilterToolbarContent: Component<FilterToolbarContentProps> = (props) => {
  const [local, divProps] = splitProps(props, ['children', 'class']);
  return (
    <div {...divProps} class={`${filterToolbarContentClass} ${local.class ?? ''}`.trim()}>
      {local.children}
    </div>
  );
};

interface FilterToolbarSearchRowProps extends JSX.HTMLAttributes<HTMLDivElement> {
  children: JSX.Element;
}

export const FilterToolbarSearchRow: Component<FilterToolbarSearchRowProps> = (props) => {
  const [local, divProps] = splitProps(props, ['children', 'class']);
  return (
    <div {...divProps} class={`${filterToolbarSearchRowClass} ${local.class ?? ''}`.trim()}>
      {local.children}
    </div>
  );
};

interface FilterToolbarRowProps extends JSX.HTMLAttributes<HTMLDivElement> {
  children: JSX.Element;
}

export const FilterToolbarRow: Component<FilterToolbarRowProps> = (props) => {
  const [local, divProps] = splitProps(props, ['children', 'class']);
  return (
    <div {...divProps} class={`${filterToolbarRowClass} ${local.class ?? ''}`.trim()}>
      {local.children}
    </div>
  );
};

interface FilterGroupProps extends JSX.HTMLAttributes<HTMLDivElement> {
  children: JSX.Element;
  label?: JSX.Element;
}

export const FilterGroup: Component<FilterGroupProps> = (props) => {
  const [local, divProps] = splitProps(props, ['children', 'label', 'class']);
  return (
    <div {...divProps} class={`${filterGroupClass} ${local.class ?? ''}`.trim()}>
      <Show when={local.label}>
        <span class={filterLabelClass}>{local.label}</span>
      </Show>
      {local.children}
    </div>
  );
};

interface FilterSegmentOption {
  value: string;
  label: JSX.Element;
  title?: string;
  ariaLabel?: string;
  disabled?: boolean;
}

interface FilterSegmentedControlProps extends JSX.HTMLAttributes<HTMLDivElement> {
  value: string;
  onChange: (value: string) => void;
  options: FilterSegmentOption[];
  label?: JSX.Element;
}

export const FilterSegmentedControl: Component<FilterSegmentedControlProps> = (props) => {
  const [local, divProps] = splitProps(props, ['value', 'onChange', 'options', 'label', 'class']);
  return (
    <FilterGroup {...divProps} class={local.class} label={local.label}>
      <For each={local.options}>
        {(option) => (
          <button
            type="button"
            onClick={() => local.onChange(option.value)}
            aria-pressed={local.value === option.value}
            aria-label={option.ariaLabel}
            title={option.title}
            disabled={option.disabled}
            class={segmentedButtonClass(local.value === option.value)}
          >
            {option.label}
          </button>
        )}
      </For>
    </FilterGroup>
  );
};

interface FilterDividerProps extends JSX.HTMLAttributes<HTMLDivElement> {}

export const FilterDivider: Component<FilterDividerProps> = (props) => {
  const [local, divProps] = splitProps(props, ['class']);
  return <div {...divProps} class={`${filterDividerClass} ${local.class ?? ''}`.trim()} />;
};

interface LabeledFilterSelectProps extends JSX.SelectHTMLAttributes<HTMLSelectElement> {
  label: string;
  children: JSX.Element;
  groupClass?: string;
  selectClass?: string;
}

export const LabeledFilterSelect: Component<LabeledFilterSelectProps> = (props) => {
  const [local, selectProps] = splitProps(props, [
    'label',
    'children',
    'groupClass',
    'selectClass',
  ]);
  const selectId = typeof selectProps.id === 'string' ? selectProps.id : undefined;
  return (
    <div class={`${filterGroupClass} ${local.groupClass ?? ''}`.trim()}>
      <label for={selectId} class={filterLabelClass}>
        {local.label}
      </label>
      <select {...selectProps} class={`${filterSelectClass} ${local.selectClass ?? ''}`.trim()}>
        {local.children}
      </select>
    </div>
  );
};

interface FilterMobileToggleButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> {
  count?: number;
  label?: string;
}

export const FilterMobileToggleButton: Component<FilterMobileToggleButtonProps> = (props) => {
  const [local, buttonProps] = splitProps(props, ['count', 'label', 'class']);
  return (
    <button
      type="button"
      {...buttonProps}
      class={`${mobileFiltersButtonClass} ${local.class ?? ''}`.trim()}
    >
      <ListFilterIcon class="h-3.5 w-3.5" />
      {local.label ?? 'Filters'}
      <Show when={(local.count ?? 0) > 0}>
        <span class={filterCountBadgeClass}>{local.count}</span>
      </Show>
    </button>
  );
};

interface FilterActionButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> {
  active?: boolean;
  children: JSX.Element;
}

export const FilterActionButton: Component<FilterActionButtonProps> = (props) => {
  const [local, buttonProps] = splitProps(props, ['active', 'children', 'class']);
  return (
    <button
      type="button"
      {...buttonProps}
      class={`${filterActionButtonClass} ${local.active ? filterActionButtonActiveClass : ''} ${
        local.class ?? ''
      }`.trim()}
    >
      {local.children}
    </button>
  );
};

interface FilterToolbarPanelProps extends JSX.HTMLAttributes<HTMLDivElement> {
  children: JSX.Element;
}

export const FilterToolbarPanel: Component<FilterToolbarPanelProps> = (props) => {
  const [local, divProps] = splitProps(props, ['children', 'class']);
  return (
    <div {...divProps} class={`${filterPanelClass} ${local.class ?? ''}`.trim()}>
      {local.children}
    </div>
  );
};
