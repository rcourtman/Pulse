import { For, Show, splitProps } from 'solid-js';
import {
  getFilterButtonGroupButtonClass,
  getFilterButtonGroupClass,
  getFilterButtonGroupCompactLabel,
  getFilterButtonGroupLabelClass,
  type FilterButtonGroupProps,
} from './filterButtonGroupModel';
import { useFilterButtonGroupState } from './useFilterButtonGroupState';

export type {
  FilterButtonGroupProps,
  FilterButtonGroupOptionTone,
  FilterOption,
} from './filterButtonGroupModel';

export function FilterButtonGroup<T extends string | number>(props: FilterButtonGroupProps<T>) {
  const [local, divProps] = splitProps(props, [
    'options',
    'value',
    'onChange',
    'ariaLabel',
    'label',
    'class',
    'variant',
    'disabled',
  ]);
  const filterButtonGroup = useFilterButtonGroupState(props);
  const groupLabel = () => local.ariaLabel ?? divProps['aria-label'] ?? 'Filter Options';

  return (
    <div
      {...divProps}
      class={getFilterButtonGroupClass(filterButtonGroup.variant(), local.class)}
      role={divProps.role ?? 'group'}
      aria-label={groupLabel()}
    >
      <Show when={local.label}>
        <span class={getFilterButtonGroupLabelClass(filterButtonGroup.variant())}>
          {local.label}
        </span>
      </Show>
      <For each={local.options}>
        {(option) => {
          const Icon = option.icon;
          const renderedLabel = () => option.visualLabel ?? option.label;
          const iconClass = () =>
            filterButtonGroup.variant() === 'compact'
              ? 'h-3 w-3'
              : 'w-4 h-4 sm:w-[18px] sm:h-[18px]';

          return (
            <button
              type="button"
              aria-label={
                option.ariaLabel ??
                (option.visualLabel
                  ? undefined
                  : option.count === undefined
                    ? option.label
                    : `${option.label}, ${option.count.toLocaleString()}`)
              }
              title={option.title}
              onClick={() => filterButtonGroup.handleOptionClick(option)}
              class={getFilterButtonGroupButtonClass(
                filterButtonGroup.variant(),
                filterButtonGroup.isOptionActive(option),
                filterButtonGroup.isOptionDisabled(option),
                option.tone,
              )}
              aria-pressed={filterButtonGroup.isOptionActive(option)}
              disabled={filterButtonGroup.isOptionDisabled(option)}
            >
              {option.leading}
              {Icon && <Icon class={iconClass()} />}
              <Show
                when={option.visualLabel}
                fallback={
                  filterButtonGroup.variant() === 'prominent' ? (
                    <span>{option.label}</span>
                  ) : (
                    <>
                      <span class="hidden sm:inline">{option.label}</span>
                      <span class="sm:hidden">{getFilterButtonGroupCompactLabel(option)}</span>
                    </>
                  )
                }
              >
                <span class="inline-flex items-center gap-1.5 whitespace-nowrap">
                  {renderedLabel()}
                </span>
              </Show>
              <Show when={option.count !== undefined}>
                <span
                  class="inline-flex items-center self-center text-[11px] font-semibold leading-none tabular-nums text-base-content/70"
                  aria-hidden="true"
                >
                  {option.count!.toLocaleString()}
                </span>
              </Show>
            </button>
          );
        }}
      </For>
    </div>
  );
}

export default FilterButtonGroup;
