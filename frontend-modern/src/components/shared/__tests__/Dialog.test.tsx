import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Dialog } from '@/components/shared/Dialog';
import dialogSource from '@/components/shared/Dialog.tsx?raw';
import dialogModelSource from '@/components/shared/dialogModel.ts?raw';
import dialogStateSource from '@/components/shared/useDialogState.ts?raw';

describe('Dialog', () => {
  afterEach(() => {
    cleanup();
    document.body.style.overflow = '';
  });

  it('keeps dialog on shell, runtime, and model owners', () => {
    expect(dialogSource).toContain('useDialogState');
    expect(dialogSource).toContain('getDialogViewportClass');
    expect(dialogSource).toContain('getDialogAlignmentClass');
    expect(dialogSource).toContain('getDialogPanelClass');
    expect(dialogSource).not.toContain('createEffect');
    expect(dialogSource).not.toContain('onCleanup');
    expect(dialogSource).not.toContain('FOCUSABLE_SELECTOR');
    expect(dialogSource).not.toContain('document.body.style.overflow');
    expect(dialogSource).not.toContain('querySelectorAll<HTMLElement>');

    expect(dialogStateSource).toContain('export function useDialogState');
    expect(dialogStateSource).toContain('createEffect');
    expect(dialogStateSource).toContain('onCleanup');
    expect(dialogStateSource).toContain('document.body.style.overflow');
    expect(dialogStateSource).toContain('openDialogCount');
    expect(dialogStateSource).toContain('getDialogFocusableElements');

    expect(dialogModelSource).toContain('export function getDialogLayout');
    expect(dialogModelSource).toContain('export function getDialogFocusableElements');
    expect(dialogModelSource).toContain('export function getDialogViewportClass');
    expect(dialogModelSource).toContain('export function getDialogAlignmentClass');
    expect(dialogModelSource).toContain('export function getDialogPanelClass');
    expect(dialogModelSource).toContain('FOCUSABLE_SELECTOR');
  });

  it('renders as a modal dialog and closes on backdrop click', () => {
    const onClose = vi.fn();
    render(() => (
      <Dialog isOpen={true} onClose={onClose}>
        <div class="p-4">
          <button type="button">Action</button>
        </div>
      </Dialog>
    ));

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    const backdrop = document.querySelector('[data-dialog-backdrop]') as HTMLElement | null;
    expect(backdrop).not.toBeNull();
    if (!backdrop) return;

    fireEvent.click(backdrop);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('closes on Escape and locks body scroll while open', () => {
    const onClose = vi.fn();
    const { unmount } = render(() => (
      <Dialog isOpen={true} onClose={onClose}>
        <div class="p-4">Body</div>
      </Dialog>
    ));

    expect(document.body.style.overflow).toBe('hidden');
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
    unmount();
    expect(document.body.style.overflow).toBe('');
  });

  it('keeps keyboard focus trapped in the dialog', async () => {
    const onClose = vi.fn();
    render(() => (
      <Dialog isOpen={true} onClose={onClose}>
        <div class="p-4">
          <button type="button">First</button>
          <button type="button">Last</button>
        </div>
      </Dialog>
    ));

    const first = screen.getByRole('button', { name: 'First' });
    const last = screen.getByRole('button', { name: 'Last' });

    last.focus();
    fireEvent.keyDown(document, { key: 'Tab' });
    expect(first).toHaveFocus();

    first.focus();
    fireEvent.keyDown(document, { key: 'Tab', shiftKey: true });
    expect(last).toHaveFocus();
  });
});
