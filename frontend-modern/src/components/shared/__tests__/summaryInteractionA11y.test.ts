import { describe, expect, it, vi } from 'vitest';
import {
  buildSummaryDisclosureControlsId,
  createSummaryInteractiveActionKeydownHandler,
  createSummaryInteractiveRowPreviewHandlers,
} from '@/components/shared/summaryInteractionA11y';

const createPointerEvent = (pointerType: string) =>
  ({
    pointerType,
    currentTarget: document.body,
    target: document.body,
  }) as unknown as PointerEvent & { currentTarget: HTMLElement; target: Element };

const createFocusEvent = (
  currentTarget: HTMLElement,
  relatedTarget: EventTarget | null = null,
) =>
  ({
    currentTarget,
    target: currentTarget,
    relatedTarget,
  }) as unknown as FocusEvent & { currentTarget: HTMLElement; target: Element };

const createKeyboardEvent = (
  currentTarget: HTMLButtonElement,
  key: string,
  code: string,
  preventDefault: () => void,
) =>
  ({
    key,
    code,
    currentTarget,
    target: currentTarget,
    preventDefault,
  }) as unknown as KeyboardEvent & { currentTarget: HTMLButtonElement; target: Element };

describe('summaryInteractionA11y', () => {
  it('previews on fine pointers and ignores touch pointers', () => {
    const onPreview = vi.fn();
    const handlers = createSummaryInteractiveRowPreviewHandlers({ onPreview });

    handlers.onPointerEnter(createPointerEvent('mouse'));
    handlers.onPointerEnter(createPointerEvent('pen'));
    handlers.onPointerEnter(createPointerEvent('touch'));

    expect(onPreview).toHaveBeenCalledTimes(2);
  });

  it('previews on focus within the row and clears once focus leaves the subtree', () => {
    const onPreview = vi.fn();
    const onPreviewClear = vi.fn();
    const handlers = createSummaryInteractiveRowPreviewHandlers({ onPreview, onPreviewClear });
    const row = document.createElement('tr');
    const child = document.createElement('button');
    row.appendChild(child);

    handlers.onFocusIn(createFocusEvent(row));
    handlers.onFocusOut(createFocusEvent(row, child));
    handlers.onFocusOut(createFocusEvent(row, document.body));

    expect(onPreview).toHaveBeenCalledTimes(1);
    expect(onPreviewClear).toHaveBeenCalledTimes(1);
  });

  it('activates the shared action control on Enter and Space', () => {
    const onAction = vi.fn();
    const onKeyDown = createSummaryInteractiveActionKeydownHandler({ onAction });
    const button = document.createElement('button');

    const enterPreventDefault = vi.fn();
    onKeyDown(createKeyboardEvent(button, 'Enter', 'Enter', enterPreventDefault));

    const spacePreventDefault = vi.fn();
    onKeyDown(createKeyboardEvent(button, 'Space', 'Space', spacePreventDefault));

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
    onKeyDown(createKeyboardEvent(button, 'Escape', 'Escape', preventDefault));

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
