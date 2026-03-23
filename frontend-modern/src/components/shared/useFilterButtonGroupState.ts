import { createMemo } from 'solid-js';
import {
  resolveFilterButtonGroupVariant,
  type FilterButtonGroupProps,
  type FilterOption,
} from './filterButtonGroupModel';

export function useFilterButtonGroupState<T extends string | number>(
  props: FilterButtonGroupProps<T>,
) {
  const variant = createMemo(() => resolveFilterButtonGroupVariant(props.variant));

  const isOptionActive = (option: FilterOption<T>) => option.value === props.value;
  const isOptionDisabled = (option: FilterOption<T>) => Boolean(props.disabled || option.disabled);

  const handleOptionClick = (option: FilterOption<T>) => {
    if (isOptionDisabled(option)) {
      return;
    }

    props.onChange(option.value);
  };

  return {
    handleOptionClick,
    isOptionActive,
    isOptionDisabled,
    variant,
  };
}
