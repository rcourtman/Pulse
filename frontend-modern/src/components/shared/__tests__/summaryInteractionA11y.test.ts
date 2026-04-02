import { describe, expect, it, vi } from 'vitest';
import { createSummaryInteractiveRowHandlers } from '@/components/shared/summaryInteractionA11y';

describe('summaryInteractionA11y', () => {
  it('previews on fine pointers and ignores touch pointers', () => {
    const onPreview = vi.fn();
    const handlers = createSummaryInteractiveRowHandlers({ onPreview });

    handlers.onPointerEnter?.({ pointerType: 'mouse' } as PointerEvent);
    handlers.onPointerEnter?.({ pointerType: 'pen' } as PointerEvent);
    handlers.onPointerEnter?.({ pointerType: 'touch' } as PointerEvent);

    expect(onPreview).toHaveBeenCalledTimes(2);
  });

  it('previews on focus and only clears once focus leaves the row subtree', () => {
    const onPreview = vi.fn();
    const onPreviewClear = vi.fn();
    const handlers = createSummaryInteractiveRowHandlers({ onPreview, onPreviewClear });
    const row = document.createElement('tr');
    const child = document.createElement('button');
    row.appendChild(child);

    handlers.onFocusIn?.({ currentTarget: row } as FocusEvent & { currentTarget: HTMLElement });
    handlers.onFocusOut?.({
      currentTarget: row,
      relatedTarget: child,
    } as FocusEvent & { currentTarget: HTMLElement; relatedTarget: EventTarget | null });
    handlers.onFocusOut?.({
      currentTarget: row,
      relatedTarget: document.body,
    } as FocusEvent & { currentTarget: HTMLElement; relatedTarget: EventTarget | null });

    expect(onPreview).toHaveBeenCalledTimes(1);
    expect(onPreviewClear).toHaveBeenCalledTimes(1);
  });

  it('toggles only when the row itself handles Enter or Space, and Escape clears preview', () => {
    const onPreviewClear = vi.fn();
    const onToggle = vi.fn();
    const handlers = createSummaryInteractiveRowHandlers({ onPreviewClear, onToggle });
    const row = document.createElement('tr');
    const child = document.createElement('button');
    row.appendChild(child);
    row.blur = vi.fn();

    const enterPreventDefault = vi.fn();
    handlers.onKeyDown?.({
      key: 'Enter',
      currentTarget: row,
      target: row,
      preventDefault: enterPreventDefault,
    } as KeyboardEvent & {
      currentTarget: HTMLElement;
      target: EventTarget | null;
      preventDefault: () => void;
    });

    const childSpacePreventDefault = vi.fn();
    handlers.onKeyDown?.({
      key: ' ',
      currentTarget: row,
      target: child,
      preventDefault: childSpacePreventDefault,
    } as KeyboardEvent & {
      currentTarget: HTMLElement;
      target: EventTarget | null;
      preventDefault: () => void;
    });

    const escapePreventDefault = vi.fn();
    handlers.onKeyDown?.({
      key: 'Escape',
      currentTarget: row,
      target: row,
      preventDefault: escapePreventDefault,
    } as KeyboardEvent & {
      currentTarget: HTMLElement;
      target: EventTarget | null;
      preventDefault: () => void;
    });

    expect(onToggle).toHaveBeenCalledTimes(1);
    expect(enterPreventDefault).toHaveBeenCalledTimes(1);
    expect(childSpacePreventDefault).not.toHaveBeenCalled();
    expect(onPreviewClear).toHaveBeenCalledTimes(1);
    expect(escapePreventDefault).toHaveBeenCalledTimes(1);
    expect(row.blur).toHaveBeenCalledTimes(1);
  });
});
