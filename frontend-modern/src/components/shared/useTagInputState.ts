import { createMemo, createSignal } from 'solid-js';
import {
  canAddTag,
  getNextTagsAfterAdd,
  getNextTagsAfterBackspace,
  getNextTagsAfterRemove,
  isTagInputCommitKey,
  normalizeTagInputValue,
  TAG_INPUT_CONTAINER_CLASS,
} from '@/components/shared/tagInputModel';

export interface TagInputProps {
  tags: string[];
  onChange: (tags: string[]) => void;
  placeholder?: string;
  class?: string;
}

export function useTagInputState(props: TagInputProps) {
  const [inputValue, setInputValue] = createSignal('');
  let inputRef: HTMLInputElement | undefined;

  const commitTag = () => {
    const normalizedValue = normalizeTagInputValue(inputValue());
    if (canAddTag(props.tags, normalizedValue)) {
      props.onChange(getNextTagsAfterAdd(props.tags, normalizedValue));
    }
    setInputValue('');
  };

  const containerClass = createMemo(() =>
    `${TAG_INPUT_CONTAINER_CLASS} ${props.class ?? ''}`.trim(),
  );

  return {
    containerClass,
    handleBlur: () => {
      commitTag();
    },
    handleContainerClick: () => {
      inputRef?.focus();
    },
    handleInput: (event: InputEvent & { currentTarget: HTMLInputElement; target: Element }) => {
      setInputValue(event.currentTarget.value);
    },
    handleKeyDown: (event: KeyboardEvent) => {
      if (isTagInputCommitKey(event.key)) {
        event.preventDefault();
        commitTag();
        return;
      }

      if (event.key === 'Backspace' && !inputValue() && props.tags.length > 0) {
        props.onChange(getNextTagsAfterBackspace(props.tags));
      }
    },
    handleRemoveTag: (event: MouseEvent, indexToRemove: number) => {
      event.stopPropagation();
      props.onChange(getNextTagsAfterRemove(props.tags, indexToRemove));
    },
    inputValue,
    setInputRef: (element: HTMLInputElement) => {
      inputRef = element;
    },
  };
}
