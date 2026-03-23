import { For } from 'solid-js';
import X from 'lucide-solid/icons/x';
import {
  getTagInputPlaceholder,
  getTagInputRemoveTitle,
  TAG_INPUT_FIELD_CLASS,
  TAG_INPUT_REMOVE_BUTTON_CLASS,
  TAG_INPUT_TAG_CLASS,
} from '@/components/shared/tagInputModel';
import { type TagInputProps, useTagInputState } from '@/components/shared/useTagInputState';

export function TagInput(props: TagInputProps) {
  const state = useTagInputState(props);

  return (
    <div class={state.containerClass()} onClick={state.handleContainerClick}>
      <For each={props.tags}>
        {(tag, index) => (
          <span class={TAG_INPUT_TAG_CLASS}>
            {tag}
            <button
              type="button"
              class={TAG_INPUT_REMOVE_BUTTON_CLASS}
              onClick={(event) => state.handleRemoveTag(event, index())}
              title={getTagInputRemoveTitle(tag)}
            >
              <X class="w-3 h-3" />
            </button>
          </span>
        )}
      </For>
      <input
        ref={state.setInputRef}
        type="text"
        value={state.inputValue()}
        onInput={state.handleInput}
        onKeyDown={state.handleKeyDown}
        onBlur={state.handleBlur}
        placeholder={getTagInputPlaceholder(props.tags.length, props.placeholder)}
        class={TAG_INPUT_FIELD_CLASS}
      />
    </div>
  );
}
