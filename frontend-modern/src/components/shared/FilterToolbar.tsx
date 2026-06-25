import { Component, For, JSX, Show, splitProps } from 'solid-js';
import BarChartIcon from 'lucide-solid/icons/bar-chart';
import ListFilterIcon from 'lucide-solid/icons/list-filter';
import { FormSelect } from './FormSelect';
import { FilterButtonGroup, type FilterButtonGroupOptionTone, type FilterOption } from './FilterButtonGroup';

export const filterToolbarShellClass = '';
export const filterToolbarRowClass = 'flex flex-wrap items-center gap-2 text-xs text-muted';
export const filterToolbarContentClass = 'flex flex-col gap-2';
export const filterToolbarSearchRowClass = 'flex w-full items-center gap-2';
export const filterGroupClass =
  'inline-flex items-center gap-1 rounded-md bg-surface-hover p-0.5 ring-1 ring-border-subtle';
export const filterLabelClass =
  'px-1.5 text-[9px] font-semibold uppercase tracking-wide text-muted';
export const filterActionButtonClass =
  'inline-flex items-center gap-1.5 rounded-md bg-surface-hover px-2.5 py-1 text-xs font-medium text-muted ring-1 ring-border-subtle transition-colors hover:bg-surface hover:text-base-content';
export const filterActionButtonActiveClass =
  'bg-surface text-base-content shadow-sm';
export const filterSelectClass =
  'rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content outline-none focus:border-blue-500';
export const filterDividerClass = 'hidden h-5 w-px bg-surface-hover sm:block';
export const filterPanelClass =
  'absolute right-0 top-[calc(100%+0.5rem)] z-[80] rounded-md border border-border bg-surface p-3 shadow-lg';
export const filterPanelDefaultWidthClass = 'w-[min(40rem,calc(100vw-2rem))]';
export const filterPanelTitleClass = 'text-sm font-medium text-base-content';
export const filterPanelDescriptionClass = 'text-xs text-muted';
export const mobileFiltersButtonClass =
  'flex items-center gap-1.5 rounded-md bg-surface-hover px-2.5 py-1.5 text-xs font-medium text-muted';
export const filterCountBadgeClass =
  'ml-0.5 rounded-full bg-blue-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white';
export const filterUtilityBadgeClass =
  'ml-0.5 inline-flex items-center whitespace-nowrap rounded-md bg-surface-alt px-1.5 py-px text-[10px] font-medium text-base-content';

export const resolveFilterSelectDomValue = (
  value: unknown,
  optionValues: readonly string[],
): string | undefined => {
  if (typeof value !== 'string') return undefined;
  if (optionValues.includes(value)) return value;
  return '';
};

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
  searchLeading?: JSX.Element;
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
    'searchLeading',
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
            {local.searchLeading}
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

export interface FilterSegmentOption {
  value: string;
  label: JSX.Element;
  title?: string;
  ariaLabel?: string;
  leading?: JSX.Element;
  disabled?: boolean;
  tone?: FilterButtonGroupOptionTone;
}

export const COMPACT_FILTER_TOGGLE_MAX_OPTIONS = 5;

export const isCompactFilterToggleGroupEligible = (
  options: readonly FilterSegmentOption[],
  maxOptions = COMPACT_FILTER_TOGGLE_MAX_OPTIONS,
): boolean => options.length > 1 && options.length <= maxOptions;

interface FilterSegmentedControlProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  value: string;
  onChange: (value: string) => void;
  options: FilterSegmentOption[];
  label?: JSX.Element;
  disabled?: boolean;
}

export const FilterSegmentedControl: Component<FilterSegmentedControlProps> = (props) => {
  const [local, divProps] = splitProps(props, ['value', 'onChange', 'options', 'label', 'class', 'disabled']);
  const options = (): FilterOption<string>[] =>
    local.options.map((option) => ({
      value: option.value,
      label:
        option.ariaLabel ??
        (typeof option.label === 'string' ? option.label : option.title ?? option.value),
      visualLabel: option.label,
      leading: option.leading,
      title: option.title,
      ariaLabel: option.ariaLabel,
      disabled: option.disabled,
      tone: option.tone,
    }));

  return (
    <FilterButtonGroup
      {...divProps}
      class={local.class}
      label={local.label}
      value={local.value}
      onChange={local.onChange}
      ariaLabel={divProps['aria-label'] as string | undefined}
      variant="compact"
      disabled={local.disabled}
      options={options()}
    />
  );
};

interface LabeledFilterToggleGroupProps extends Omit<
  FilterSegmentedControlProps,
  'label' | 'options' | 'id' | 'class'
> {
  id?: string;
  label: string;
  options: FilterSegmentOption[];
  selectClass?: string;
  selectFallback?: boolean;
  selectGroupClass?: string;
  toggleClass?: string;
}

export const LabeledFilterToggleGroup: Component<LabeledFilterToggleGroupProps> = (props) => {
  const [local, divProps] = splitProps(props, [
    'id',
    'label',
    'value',
    'onChange',
    'options',
    'selectClass',
    'selectFallback',
    'selectGroupClass',
    'toggleClass',
  ]);
  const selectId = () => local.id;
  return (
    <>
      <FilterSegmentedControl
        {...divProps}
        id={selectId() ? `${selectId()}-toggle` : undefined}
        role="group"
        aria-label={local.label}
        class={local.toggleClass ?? 'hidden xl:inline-flex'}
        value={local.value}
        onChange={local.onChange}
        label={local.label}
        options={local.options}
      />

      <Show when={local.selectFallback !== false}>
        <LabeledFilterSelect
          id={selectId()}
          label={local.label}
          value={local.value}
          onChange={(event) => local.onChange(event.currentTarget.value)}
          groupClass={local.selectGroupClass ?? 'xl:hidden'}
          selectClass={local.selectClass}
        >
          <For each={local.options}>
            {(option) => <option value={option.value}>{option.label}</option>}
          </For>
        </LabeledFilterSelect>
      </Show>
    </>
  );
};

type FilterDividerProps = JSX.HTMLAttributes<HTMLDivElement>;

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
    'aria-label',
  ]);

  return (
    <FormSelect
      {...selectProps}
      label={local.label}
      aria-label={local['aria-label'] ?? local.label}
      fieldBaseClass={`${filterGroupClass} ${local.groupClass ?? ''}`.trim()}
      labelClass={filterLabelClass}
      selectBaseClass={filterSelectClass}
      selectClass={local.selectClass}
    >
      {local.children}
    </FormSelect>
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

interface ChartVisibilityToggleButtonProps extends Omit<
  JSX.ButtonHTMLAttributes<HTMLButtonElement>,
  'aria-label' | 'aria-pressed' | 'children' | 'onClick' | 'title'
> {
  collapsed: boolean;
  onToggle: () => void;
}

export const ChartVisibilityToggleButton: Component<ChartVisibilityToggleButtonProps> = (props) => {
  const [local, buttonProps] = splitProps(props, ['collapsed', 'onToggle', 'class']);
  const label = () => (local.collapsed ? 'Show charts' : 'Hide charts');
  return (
    <FilterActionButton
      {...buttonProps}
      class={`hidden lg:inline-flex ${local.class ?? ''}`.trim()}
      active={!local.collapsed}
      aria-label={label()}
      aria-pressed={!local.collapsed}
      title={label()}
      onClick={() => local.onToggle()}
    >
      <BarChartIcon class="h-3 w-3" />
      Charts
    </FilterActionButton>
  );
};

interface FilterToolbarPanelProps extends JSX.HTMLAttributes<HTMLDivElement> {
  children: JSX.Element;
  widthClass?: string;
}

export const FilterToolbarPanel: Component<FilterToolbarPanelProps> = (props) => {
  const [local, divProps] = splitProps(props, ['children', 'class', 'widthClass']);
  return (
    <div
      {...divProps}
      class={`${filterPanelClass} ${local.widthClass ?? filterPanelDefaultWidthClass} ${
        local.class ?? ''
      }`.trim()}
    >
      {local.children}
    </div>
  );
};
