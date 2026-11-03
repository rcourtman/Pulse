import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TagInput } from './TagInput';

describe('TagInput', () => {
  afterEach(() => {
    cleanup();
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
