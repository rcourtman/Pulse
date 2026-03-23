export const TAG_INPUT_CONTAINER_CLASS =
  'min-h-[42px] flex flex-wrap items-center gap-2 rounded-md border border-border bg-surface p-2 text-sm text-base-content focus-within:border-sky-500 focus-within:ring-1 focus-within:ring-sky-500';
export const TAG_INPUT_TAG_CLASS =
  'inline-flex items-center gap-1 rounded bg-surface-alt px-2 py-1 text-xs font-medium text-base-content';
export const TAG_INPUT_REMOVE_BUTTON_CLASS =
  'rounded-full p-0.5 text-slate-400 hover:bg-slate-300 hover:bg-surface-hover cursor-pointer focus:outline-none focus:ring-2 focus:ring-sky-500';
export const TAG_INPUT_FIELD_CLASS = 'flex-1 bg-transparent min-w-[120px] focus:outline-none';
export const TAG_INPUT_DELIMITER_KEYS = ['Enter', ','] as const;

export function isTagInputCommitKey(key: string): boolean {
  return TAG_INPUT_DELIMITER_KEYS.includes(key as (typeof TAG_INPUT_DELIMITER_KEYS)[number]);
}

export function getTagInputPlaceholder(tagCount: number, placeholder?: string): string {
  return tagCount === 0 ? (placeholder ?? '') : '';
}

export function normalizeTagInputValue(value: string): string {
  return value.trim();
}

export function canAddTag(tags: string[], value: string): boolean {
  return Boolean(value) && !tags.includes(value);
}

export function getNextTagsAfterAdd(tags: string[], value: string): string[] {
  return [...tags, value];
}

export function getNextTagsAfterBackspace(tags: string[]): string[] {
  return tags.slice(0, -1);
}

export function getNextTagsAfterRemove(tags: string[], indexToRemove: number): string[] {
  return tags.filter((_, index) => index !== indexToRemove);
}

export function getTagInputRemoveTitle(tag: string): string {
  return `Remove ${tag}`;
}
