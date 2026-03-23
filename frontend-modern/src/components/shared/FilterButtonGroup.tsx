import { For } from 'solid-js';
import {
  getFilterButtonGroupButtonClass,
  getFilterButtonGroupClass,
  getFilterButtonGroupCompactLabel,
  type FilterButtonGroupProps,
} from './filterButtonGroupModel';
import { useFilterButtonGroupState } from './useFilterButtonGroupState';

export type {
  FilterButtonGroupProps,
  FilterButtonGroupVariant,
  FilterOption,
} from './filterButtonGroupModel';

export function FilterButtonGroup<T extends string | number>(props: FilterButtonGroupProps<T>) {
  const filterButtonGroup = useFilterButtonGroupState(props);

  return (
    <div
      class={getFilterButtonGroupClass(filterButtonGroup.variant(), props.class)}
      role="group"
      aria-label="Filter Options"
    >
      <For each={props.options}>
        {(option) => {
          const Icon = option.icon;

          return (
            <button
              type="button"
              onClick={() => filterButtonGroup.handleOptionClick(option)}
              class={getFilterButtonGroupButtonClass(
                filterButtonGroup.variant(),
                filterButtonGroup.isOptionActive(option),
                filterButtonGroup.isOptionDisabled(option),
              )}
              aria-pressed={filterButtonGroup.isOptionActive(option)}
              disabled={filterButtonGroup.isOptionDisabled(option)}
            >
              {Icon && <Icon class="w-4 h-4 sm:w-[18px] sm:h-[18px]" />}
              {filterButtonGroup.variant() === 'prominent' ? (
                <span>{option.label}</span>
              ) : (
                <>
                  <span class="hidden sm:inline">{option.label}</span>
                  <span class="sm:hidden">{getFilterButtonGroupCompactLabel(option.label)}</span>
                </>
              )}
            </button>
          );
        }}
      </For>
    </div>
  );
}

export default FilterButtonGroup;
