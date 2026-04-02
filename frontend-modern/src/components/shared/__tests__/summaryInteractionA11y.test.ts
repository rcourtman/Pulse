import { describe, expect, it, vi } from 'vitest';
import {
  buildSummaryDisclosureControlsId,
  createSummaryInteractiveActionKeydownHandler,
  createSummaryInteractiveRowPreviewHandlers,
} from '@/components/shared/summaryInteractionA11y';

describe('summaryInteractionA11y', () => {
  it('previews on fine pointers and ignores touch pointers', () => {
    const onPreview = vi.fn();
    const handlers = createSummaryInteractiveRowPreviewHandlers({ onPreview });

    handlers.onPointerEnter?.({ pointerType: 'mouse' } as PointerEvent);
    handlers.onPointerEnter?.({ pointerType: 'pen' } as PointerEvent);
    handlers.onPointerEnter?.({ pointerType: 'touch' } as PointerEvent);

    expect(onPreview).toHaveBeenCalledTimes(2);
  });

  it('previews on focus within the row and clears once focus leaves the subtree', () => {
    const onPreview = vi.fn();
    const onPreviewClear = vi.fn();
    const handlers = createSummaryInteractiveRowPreviewHandlers({ onPreview, onPreviewClear });
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

  it('activates the shared action control on Enter and Space', () => {
    const onAction = vi.fn();
    const onKeyDown = createSummaryInteractiveActionKeydownHandler({ onAction });
    const button = document.createElement('button');

    const enterPreventDefault = vi.fn();
    onKeyDown({
      key: 'Enter',
      code: 'Enter',
      currentTarget: button,
      preventDefault: enterPreventDefault,
    } as KeyboardEvent & {
      currentTarget: HTMLButtonElement;
      preventDefault: () => void;
    });

    const spacePreventDefault = vi.fn();
    onKeyDown({
      key: 'Space',
      code: 'Space',
      currentTarget: button,
      preventDefault: spacePreventDefault,
    } as KeyboardEvent & {
      currentTarget: HTMLButtonElement;
      preventDefault: () => void;
    });

    expect(onAction).toHaveBeenCalledTimes(2);
    expect(enterPreventDefault).toHaveBeenCalledTimes(1);
    expect(spacePreventDefault).toHaveBeenCalledTimes(1);
  });

  it('clears preview and blurs on Escape from the shared action control', () => {
    const onPreviewClear = vi.fn();
    const onKeyDown = createSummaryInteractiveActionKeydownHandler({ onPreviewClear });
    const button = document.createElement('button');
    button.blur = vi.fn();

    const preventDefault = vi.fn();
    onKeyDown({
      key: 'Escape',
      currentTarget: button,
      preventDefault,
    } as KeyboardEvent & {
      currentTarget: HTMLButtonElement;
      preventDefault: () => void;
    });

    expect(onPreviewClear).toHaveBeenCalledTimes(1);
    expect(preventDefault).toHaveBeenCalledTimes(1);
    expect(button.blur).toHaveBeenCalledTimes(1);
  });

  it('builds stable disclosure control ids from summary series ids', () => {
    expect(buildSummaryDisclosureControlsId('resource:alpha/01')).toBe(
      'summary-row-detail-resource-alpha-01',
    );
  });
});
