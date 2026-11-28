import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
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

  it('shows upgrade warnings and still permits confirming after acknowledgement', async () => {
    const onConfirm = vi.fn();

    render(() => (
      <UpdateConfirmationModal
        isOpen={true}
        onClose={() => {}}
        onConfirm={onConfirm}
        currentVersion="6.9.0"
        latestVersion="6.10.0"
        plan={{
          canAutoUpdate: true,
          requiresRoot: false,
          rollbackSupport: true,
          readiness: {
            status: 'attention',
            summary: 'Update can proceed, but review 1 upgrade warning before installing.',
            checks: [
              {
                id: 'agent-continuity',
                status: 'warning',
                title: 'Agent continuity',
                summary: '1 agent is not recently connected.',
                details: ['3 agents currently registered.'],
              },
            ],
          },
        }}
        isApplying={false}
      />
    ));

    const panel = await screen.findByTestId('update-readiness-panel');
    expect(within(panel).getByText('Agent continuity')).toBeInTheDocument();
    expect(within(panel).getByText('Warning')).toBeInTheDocument();
    expect(within(panel).getByText('3 agents currently registered.')).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('checkbox'));
    await fireEvent.click(screen.getByRole('button', { name: 'Start Update' }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('disables confirmation when upgrade readiness checks are blocked', async () => {
    const onConfirm = vi.fn();

    render(() => (
      <UpdateConfirmationModal
        isOpen={true}
        onClose={() => {}}
        onConfirm={onConfirm}
        currentVersion="6.9.0"
        latestVersion="6.10.0"
        plan={{
          canAutoUpdate: true,
          requiresRoot: false,
          rollbackSupport: true,
          readiness: {
            status: 'blocked',
            summary: 'Resolve 1 blocked upgrade check before installing this update.',
            checks: [
              {
                id: 'server-update-path',
                status: 'pass',
                title: 'Server update path',
                summary: 'Automatic install and rollback support are available.',
              },
              {
                id: 'agent-token-scopes',
                status: 'blocked',
                title: 'Agent token scopes',
                summary: 'All loaded agent reporting tokens are expired.',
                details: ['Create or refresh an agent install token before updating.'],
              },
            ],
          },
        }}
        isApplying={false}
      />
    ));

    const panel = await screen.findByTestId('update-readiness-panel');
    expect(panel).toHaveAttribute('role', 'alert');
    expect(within(panel).getByText('Agent token scopes')).toBeInTheDocument();
    expect(
      screen.getByText('Resolve blocked upgrade checks before starting this update.'),
    ).toBeInTheDocument();

    const checkbox = screen.getByRole('checkbox');
    expect(checkbox).toBeDisabled();

    const blockedButton = screen.getByRole('button', { name: 'Install blocked' });
    expect(blockedButton).toBeDisabled();
    await fireEvent.click(blockedButton);
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
