import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Dialog } from '@/components/shared/Dialog';

describe('Dialog', () => {
  afterEach(() => {
    cleanup();
    document.body.style.overflow = '';
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
