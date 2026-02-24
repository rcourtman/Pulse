import { createSignal, For } from 'solid-js';
import X from 'lucide-solid/icons/x';

export interface TagInputProps {
  tags: string[];
  onChange: (tags: string[]) => void;
  placeholder?: string;
  class?: string;
}

export function TagInput(props: TagInputProps) {
  const [inputValue, setInputValue] = createSignal('');

  const handleKeyDown = (e: KeyboardEvent) => {
    // Prevent default on enter/comma to use them as delimiters
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      addTag();
    } else if (e.key === 'Backspace' && !inputValue() && props.tags.length > 0) {
      // Remove last tag if backspace is pressed while input is empty
      props.onChange(props.tags.slice(0, -1));
    }
  };

  const handleBlur = () => {
    // Also try adding the tag on blur
    addTag();
  };

  const addTag = () => {
    const newTag = inputValue().trim();
    if (newTag && !props.tags.includes(newTag)) {
      props.onChange([...props.tags, newTag]);
    }
    setInputValue('');
  };

  const removeTag = (indexToRemove: number) => {
    props.onChange(props.tags.filter((_, i) => i !== indexToRemove));
  };

  return (
    <div
      class={`min-h-[42px] flex flex-wrap items-center gap-2 rounded-md border border-border bg-surface p-2 text-sm text-base-content focus-within:border-sky-500 focus-within:ring-1 focus-within:ring-sky-500 ${props.class || ''}`}
      onClick={(e) => {
        // Focus the input when clicking anywhere in the container
        const input = e.currentTarget.querySelector('input');
        if (input) input.focus();
      }}
    >
      <For each={props.tags}>
        {(tag, index) => (
          <span class="inline-flex items-center gap-1 rounded bg-surface-alt px-2 py-1 text-xs font-medium text-base-content">
            {tag}
            <button
              type="button"
              class="rounded-full p-0.5 text-slate-400 hover:bg-slate-300 hover:bg-surface-hover cursor-pointer focus:outline-none focus:ring-2 focus:ring-sky-500"
              onClick={(e) => {
                e.stopPropagation();
                removeTag(index());
              }}
              title={`Remove ${tag}`}
            >
              <X class="w-3 h-3" />
            </button>
          </span>
        )}
      </For>
      <input
        type="text"
        value={inputValue()}
        onInput={(e) => setInputValue(e.currentTarget.value)}
        onKeyDown={handleKeyDown}
        onBlur={handleBlur}
        placeholder={props.tags.length === 0 ? props.placeholder : ''}
        class="flex-1 bg-transparent min-w-[120px] focus:outline-none"
      />
    </div>
  );
}
