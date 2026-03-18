import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { UpdateConfirmationModal } from '@/components/UpdateConfirmationModal';

describe('UpdateConfirmationModal', () => {
  afterEach(() => {
    cleanup();
  });

  it('requires acknowledgement again when the modal is reopened', async () => {
    const onConfirm = vi.fn();

    render(() => {
      const [isOpen, setIsOpen] = createSignal(true);

      return (
        <>
          <button onClick={() => setIsOpen((current) => !current)}>Toggle Modal</button>
          <UpdateConfirmationModal
            isOpen={isOpen()}
            onClose={() => setIsOpen(false)}
            onConfirm={onConfirm}
            currentVersion="5.0.0"
            latestVersion="5.1.0"
            plan={{ canAutoUpdate: true, requiresRoot: false, rollbackSupport: true }}
            isApplying={false}
          />
        </>
      );
    });

    const startButton = await screen.findByRole('button', { name: 'Start Update' });
    expect(startButton).toBeDisabled();

    const checkbox = await screen.findByRole('checkbox');
    await fireEvent.click(checkbox);
    expect(startButton).not.toBeDisabled();

    await fireEvent.click(screen.getByRole('button', { name: 'Toggle Modal' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Toggle Modal' }));

    const reopenedCheckbox = await screen.findByRole('checkbox');
    expect((reopenedCheckbox as HTMLInputElement).checked).toBe(false);
    expect(await screen.findByRole('button', { name: 'Start Update' })).toBeDisabled();
  });
});
