import { createMemo } from 'solid-js';
import {
  resolveSelectionCardGroupVariant,
  resolveSelectionCardTone,
  type SelectionCardGroupProps,
  type SelectionCardOption,
} from './selectionCardGroupModel';

export function useSelectionCardGroupState<T extends string | number>(
  props: SelectionCardGroupProps<T>,
) {
  const variant = createMemo(() => resolveSelectionCardGroupVariant(props.variant));

  const isOptionActive = (option: SelectionCardOption<T>) => option.value === props.value;
  const isOptionDisabled = (option: SelectionCardOption<T>) => Boolean(props.disabled || option.disabled);
  const getOptionTone = (option: SelectionCardOption<T>) => resolveSelectionCardTone(option.tone);

  const handleOptionClick = (option: SelectionCardOption<T>) => {
    if (isOptionDisabled(option)) {
      return;
    }

    props.onChange(option.value);
  };

  return {
    getOptionTone,
    handleOptionClick,
    isOptionActive,
    isOptionDisabled,
    variant,
  };
}
