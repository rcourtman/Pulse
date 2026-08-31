import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TagInput } from './TagInput';
import tagInputSource from './TagInput.tsx?raw';
import tagInputModelSource from './tagInputModel.ts?raw';
import tagInputStateSource from './useTagInputState.ts?raw';

describe('TagInput', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps tag input on shell, runtime, and model owners', () => {
    expect(tagInputSource).toContain('useTagInputState');
    expect(tagInputSource).toContain('getTagInputPlaceholder');
    expect(tagInputSource).not.toContain('createSignal');
    expect(tagInputSource).not.toContain('querySelector');
    expect(tagInputSource).not.toContain('Backspace');
    expect(tagInputSource).not.toContain('addTag');

    expect(tagInputStateSource).toContain('export function useTagInputState');
    expect(tagInputStateSource).toContain('createSignal');
    expect(tagInputStateSource).toContain('createMemo');
    expect(tagInputStateSource).toContain('inputRef?.focus');
    expect(tagInputStateSource).toContain("event.key === 'Backspace'");
    expect(tagInputStateSource).toContain('commitTag');

    expect(tagInputModelSource).toContain('TAG_INPUT_DELIMITER_KEYS');
    expect(tagInputModelSource).toContain('isTagInputCommitKey');
    expect(tagInputModelSource).toContain('getTagInputPlaceholder');
    expect(tagInputModelSource).toContain('getNextTagsAfterRemove');
    expect(tagInputModelSource).toContain('getTagInputRemoveTitle');
  });

  it('adds tags on enter and removes the last one on backspace when empty', () => {
    const onChange = vi.fn();

    render(() => (
      <TagInput tags={['alpha']} label="Resource tags" onChange={onChange} placeholder="Add tag" />
    ));

    const input = screen.getByRole('textbox', { name: 'Resource tags' });

    fireEvent.input(input, { target: { value: 'beta' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onChange).toHaveBeenCalledWith(['alpha', 'beta']);

    fireEvent.input(input, { target: { value: '' } });
    fireEvent.keyDown(input, { key: 'Backspace' });
    expect(onChange).toHaveBeenCalledWith([]);
  });

  it('keeps its accessible name when tags hide the placeholder and focuses on container click', () => {
    const [tags, setTags] = createSignal<string[]>([]);
    render(() => (
      <TagInput tags={tags()} label="Resource tags" onChange={vi.fn()} placeholder="Add tag" />
    ));

    const input = screen.getByRole('textbox', { name: 'Resource tags' }) as HTMLInputElement;
    expect(input).toHaveAttribute('placeholder', 'Add tag');
    setTags(['alpha']);

    expect(screen.getByRole('textbox', { name: 'Resource tags' })).toBe(input);
    expect(input.placeholder).toBe('');

    fireEvent.click(input.parentElement as HTMLElement);
    expect(document.activeElement).toBe(input);
  });
});
