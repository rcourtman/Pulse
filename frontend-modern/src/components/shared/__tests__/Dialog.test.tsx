import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import { dialogStackHasBlockingDialog } from '@/components/shared/useDialogState';
import dialogSource from '@/components/shared/Dialog.tsx?raw';
import dialogModelSource from '@/components/shared/dialogModel.ts?raw';
import dialogStateSource from '@/components/shared/useDialogState.ts?raw';

function getBodyChild(element: HTMLElement): HTMLElement {
  let bodyChild = element;
  while (bodyChild.parentElement && bodyChild.parentElement !== document.body) {
    bodyChild = bodyChild.parentElement;
  }
  return bodyChild;
}

describe('Dialog', () => {
  afterEach(() => {
    cleanup();
    document
      .querySelectorAll('[data-dialog-test-background]')
      .forEach((element) => element.remove());
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
    expect(dialogStateSource).toContain('dialogStackHasBlockingDialog');
    expect(dialogStateSource).toContain('getDialogFocusableElements');
    expect(dialogStateSource).toContain('syncBackgroundInertness');
    expect(dialogStateSource).toContain('MutationObserver');

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
    expect(screen.getByRole('dialog')).toHaveClass('min-h-0');
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
    const laterDocumentHandler = vi.fn();
    document.addEventListener('keydown', laterDocumentHandler);

    expect(document.body.style.overflow).toBe('hidden');
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(laterDocumentHandler).not.toHaveBeenCalled();
    document.removeEventListener('keydown', laterDocumentHandler);
    unmount();
    expect(document.body.style.overflow).toBe('');
  });

  it('publishes blocking dialog state while a modal is open', () => {
    expect(dialogStackHasBlockingDialog()).toBe(false);

    const { unmount } = render(() => (
      <Dialog isOpen={true} onClose={() => undefined}>
        <div class="p-4">Body</div>
      </Dialog>
    ));

    expect(dialogStackHasBlockingDialog()).toBe(true);
    unmount();
    expect(dialogStackHasBlockingDialog()).toBe(false);
  });

  it('makes every body-level background surface inert while open and restores it on close', () => {
    const background = document.createElement('main');
    background.dataset.dialogTestBackground = 'true';
    document.body.appendChild(background);

    const { unmount } = render(() => (
      <Dialog isOpen={true} onClose={() => undefined}>
        <button type="button">Dialog action</button>
      </Dialog>
    ));

    const dialogBodyChild = getBodyChild(screen.getByRole('dialog'));

    expect(background).toHaveAttribute('inert');
    expect(dialogBodyChild).not.toHaveAttribute('inert');

    unmount();
    expect(background).not.toHaveAttribute('inert');
  });

  it('preserves a pre-existing inert state after the final dialog closes', () => {
    const background = document.createElement('main');
    background.dataset.dialogTestBackground = 'true';
    background.setAttribute('inert', '');
    document.body.appendChild(background);

    const { unmount } = render(() => (
      <Dialog isOpen={true} onClose={() => undefined}>
        <button type="button">Dialog action</button>
      </Dialog>
    ));

    expect(background).toHaveAttribute('inert');
    unmount();
    expect(background).toHaveAttribute('inert');
  });

  it('keeps only the topmost dialog layer interactive', async () => {
    const [innerOpen, setInnerOpen] = createSignal(false);
    render(() => (
      <>
        <Dialog isOpen={true} onClose={() => undefined} ariaLabel="Outer dialog">
          <button type="button" onClick={() => setInnerOpen(true)}>
            Open inner dialog
          </button>
        </Dialog>
        <Dialog isOpen={innerOpen()} onClose={() => setInnerOpen(false)} ariaLabel="Inner dialog">
          <button type="button">Inner action</button>
        </Dialog>
      </>
    ));

    const outerDialog = screen.getByRole('dialog', { name: 'Outer dialog' });
    const outerLayer = outerDialog.closest<HTMLElement>('[data-dialog-layer]');
    expect(outerLayer).not.toBeNull();
    expect(outerLayer).not.toHaveAttribute('inert');

    fireEvent.click(screen.getByRole('button', { name: 'Open inner dialog' }));
    await Promise.resolve();

    const innerDialog = screen.getByRole('dialog', { name: 'Inner dialog' });
    const innerLayer = innerDialog.closest<HTMLElement>('[data-dialog-layer]');
    expect(innerLayer).not.toBeNull();
    expect(getBodyChild(outerDialog)).toHaveAttribute('inert');
    expect(getBodyChild(innerDialog)).not.toHaveAttribute('inert');

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(getBodyChild(outerDialog)).not.toHaveAttribute('inert');
  });

  it('makes body-level surfaces added while a dialog is open inert', async () => {
    render(() => (
      <Dialog isOpen={true} onClose={() => undefined}>
        <button type="button">Dialog action</button>
      </Dialog>
    ));

    const lateSurface = document.createElement('aside');
    lateSurface.dataset.dialogTestBackground = 'true';
    document.body.appendChild(lateSurface);
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(lateSurface).toHaveAttribute('inert');
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

  it('gives Escape, focus trapping, and backdrop dismissal to only the topmost dialog', async () => {
    const [innerOpen, setInnerOpen] = createSignal(false);
    const onOuterClose = vi.fn();
    const onInnerClose = vi.fn(() => setInnerOpen(false));
    render(() => (
      <>
        <Dialog isOpen={true} onClose={onOuterClose} ariaLabel="Outer dialog">
          <button type="button" onClick={() => setInnerOpen(true)}>
            Open inner dialog
          </button>
        </Dialog>
        <Dialog isOpen={innerOpen()} onClose={onInnerClose} ariaLabel="Inner dialog">
          <button type="button">Inner first</button>
          <button type="button">Inner last</button>
        </Dialog>
      </>
    ));

    await Promise.resolve();
    const outerAction = screen.getByRole('button', { name: 'Open inner dialog' });
    outerAction.focus();
    fireEvent.click(outerAction);
    await Promise.resolve();
    expect(screen.getByRole('button', { name: 'Inner first' })).toHaveFocus();

    const innerLast = screen.getByRole('button', { name: 'Inner last' });
    innerLast.focus();
    fireEvent.keyDown(document, { key: 'Tab' });
    expect(screen.getByRole('button', { name: 'Inner first' })).toHaveFocus();
    expect(outerAction).not.toHaveFocus();

    const [outerBackdrop, innerBackdrop] = Array.from(
      document.querySelectorAll<HTMLElement>('[data-dialog-backdrop]'),
    );
    fireEvent.click(outerBackdrop);
    expect(onOuterClose).not.toHaveBeenCalled();
    expect(onInnerClose).not.toHaveBeenCalled();

    fireEvent.click(innerBackdrop);
    expect(onInnerClose).toHaveBeenCalledTimes(1);
    expect(onOuterClose).not.toHaveBeenCalled();
    expect(outerAction).toHaveFocus();

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onInnerClose).toHaveBeenCalledTimes(1);
    expect(onOuterClose).toHaveBeenCalledTimes(1);
  });

  it('does not let an underlying dialog cleanup steal focus from the topmost dialog', async () => {
    const [outerOpen, setOuterOpen] = createSignal(true);
    const [innerOpen, setInnerOpen] = createSignal(false);
    render(() => (
      <>
        <Dialog isOpen={outerOpen()} onClose={() => setOuterOpen(false)} ariaLabel="Outer dialog">
          <button type="button" onClick={() => setInnerOpen(true)}>
            Open inner dialog
          </button>
        </Dialog>
        <Dialog isOpen={innerOpen()} onClose={() => setInnerOpen(false)} ariaLabel="Inner dialog">
          <button type="button">Inner action</button>
        </Dialog>
      </>
    ));

    await Promise.resolve();
    const outerAction = screen.getByRole('button', { name: 'Open inner dialog' });
    outerAction.focus();
    fireEvent.click(outerAction);
    await Promise.resolve();

    const innerAction = screen.getByRole('button', { name: 'Inner action' });
    expect(innerAction).toHaveFocus();
    setOuterOpen(false);

    expect(innerAction).toHaveFocus();
    expect(document.body.style.overflow).toBe('hidden');
  });

  it('honors an explicitly requested initial focus target', async () => {
    render(() => (
      <Dialog isOpen={true} onClose={() => undefined}>
        <div class="p-4">
          <button type="button">Close</button>
          <textarea aria-label="Outcome" autofocus />
        </div>
      </Dialog>
    ));

    await Promise.resolve();
    expect(screen.getByRole('textbox', { name: 'Outcome' })).toHaveFocus();
  });

  it('restores trigger focus without moving the background scroll position', async () => {
    const [isOpen, setIsOpen] = createSignal(false);
    render(() => (
      <>
        <button type="button" onClick={() => setIsOpen(true)}>
          Open investigation
        </button>
        <Dialog isOpen={isOpen()} onClose={() => setIsOpen(false)}>
          <button type="button" onClick={() => setIsOpen(false)}>
            Close investigation
          </button>
        </Dialog>
      </>
    ));

    const trigger = screen.getByRole('button', { name: 'Open investigation' });
    trigger.focus();
    const focusSpy = vi.spyOn(trigger, 'focus');
    await fireEvent.click(trigger);
    await fireEvent.click(screen.getByRole('button', { name: 'Close investigation' }));

    expect(focusSpy).toHaveBeenCalledWith({ preventScroll: true });
    expect(trigger).toHaveFocus();
  });
});
