import { For } from 'solid-js';
import {
  getSelectionCardButtonClass,
  getSelectionCardDescriptionClass,
  getSelectionCardGroupClass,
  getSelectionCardIconContainerClass,
  getSelectionCardTitleClass,
  type SelectionCardGroupProps,
} from './selectionCardGroupModel';
import { useSelectionCardGroupState } from './useSelectionCardGroupState';

export type {
  SelectionCardGroupProps,
  SelectionCardGroupVariant,
  SelectionCardOption,
  SelectionCardTone,
} from './selectionCardGroupModel';

export function SelectionCardGroup<T extends string | number>(props: SelectionCardGroupProps<T>) {
  const selectionCardGroup = useSelectionCardGroupState(props);

  return (
    <div
      class={getSelectionCardGroupClass(selectionCardGroup.variant(), props.class)}
      role="group"
      aria-label="Selection Cards"
    >
      <For each={props.options}>
        {(option) => {
          return (
            <button
              type="button"
              onClick={() => selectionCardGroup.handleOptionClick(option)}
              class={getSelectionCardButtonClass(
                selectionCardGroup.variant(),
                selectionCardGroup.getOptionTone(option),
                selectionCardGroup.isOptionActive(option),
                selectionCardGroup.isOptionDisabled(option),
              )}
              aria-pressed={selectionCardGroup.isOptionActive(option)}
              disabled={selectionCardGroup.isOptionDisabled(option)}
            >
              {selectionCardGroup.variant() === 'detail' ? (
                <div class="flex items-center gap-3">
                  {option.icon && (
                    <div
                      class={getSelectionCardIconContainerClass(
                        selectionCardGroup.getOptionTone(option),
                        selectionCardGroup.isOptionActive(option),
                      )}
                    >
                      {option.icon({ active: selectionCardGroup.isOptionActive(option) })}
                    </div>
                  )}
                  <div>
                    <p
                      class={getSelectionCardTitleClass(
                        selectionCardGroup.variant(),
                        selectionCardGroup.getOptionTone(option),
                        selectionCardGroup.isOptionActive(option),
                      )}
                    >
                      {option.title}
                    </p>
                    {option.description && (
                      <p class={getSelectionCardDescriptionClass(selectionCardGroup.variant())}>
                        {option.description}
                      </p>
                    )}
                  </div>
                </div>
              ) : (
                <div>
                  <div
                    class={getSelectionCardTitleClass(
                      selectionCardGroup.variant(),
                      selectionCardGroup.getOptionTone(option),
                      selectionCardGroup.isOptionActive(option),
                    )}
                  >
                    {option.title}
                  </div>
                  {option.description && (
                    <div class={getSelectionCardDescriptionClass(selectionCardGroup.variant())}>
                      {option.description}
                    </div>
                  )}
                </div>
              )}
            </button>
          );
        }}
      </For>
    </div>
  );
}

export default SelectionCardGroup;
