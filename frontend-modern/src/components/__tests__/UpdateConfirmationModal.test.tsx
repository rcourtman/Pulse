import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { UpdateConfirmationModal } from '@/components/UpdateConfirmationModal';
import updateConfirmationModalSource from '@/components/UpdateConfirmationModal.tsx?raw';

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

  it('keeps update confirmation chrome on shared primitives', () => {
    expect(updateConfirmationModalSource).toContain('CalloutCard');
    expect(updateConfirmationModalSource).toContain('ActionIconButton');
    expect(updateConfirmationModalSource).toContain('lucide-solid/icons/arrow-right');
    expect(updateConfirmationModalSource).not.toContain('<svg');
    expect(updateConfirmationModalSource).not.toContain(
      'bg-blue-50 dark:bg-blue-900 border border-blue-200',
    );
    expect(updateConfirmationModalSource).not.toContain(
      'bg-yellow-50 dark:bg-yellow-900 border border-yellow-200',
    );
    expect(updateConfirmationModalSource).not.toContain(
      'px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors',
    );
  });
});
