import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal, Show } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SettingsAccessRecovery } from '../SettingsAccessRecovery';

function mount() {
  const [failed, setFailed] = createSignal(true);
  const [checking, setChecking] = createSignal(false);
  let finish!: (failed: boolean) => void;
  const retry = vi.fn(() => {
    setChecking(true);
    return new Promise<void>((resolve) => {
      finish = (readFailed) => {
        setFailed(readFailed);
        setChecking(false);
        resolve();
      };
    });
  });

  render(() => (
    <>
      <button>Settings navigation</button>
      <SettingsAccessRecovery failed={failed()} checking={checking()} retry={retry}>
        <Show when={!failed()}>
          <button>Review settings</button>
        </Show>
      </SettingsAccessRecovery>
    </>
  ));

  return { retry, finish: (readFailed: boolean) => finish(readFailed), setChecking };
}

describe('Settings access recovery', () => {
  afterEach(cleanup);

  it('sets a stable focus position before disabling Retry and keeps it after a repeated failure', async () => {
    const { retry, finish } = mount();
    const region = screen.getByRole('region', { name: 'Settings content' });
    const button = screen.getByRole('button', { name: 'Retry access check' });
    const status = screen.getByRole('status');
    expect(region).not.toHaveFocus();
    button.focus();
    fireEvent.click(button);
    expect(region).toHaveFocus();
    expect(button).toBeDisabled();
    expect(status).toHaveTextContent('Checking settings access.');
    fireEvent.click(button);
    expect(retry).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('button', { name: 'Review settings' })).not.toBeInTheDocument();

    finish(true);
    await waitFor(() =>
      expect(status).toHaveTextContent('Settings access still could not be verified.'),
    );
    expect(button).toBeEnabled();
    expect(region).toHaveFocus();
  });

  it('announces a successful check without reclaiming deliberate outside focus', async () => {
    const { retry, finish, setChecking } = mount();
    const button = screen.getByRole('button', { name: 'Retry access check' });
    const navigation = screen.getByRole('button', { name: 'Settings navigation' });
    // Click need not focus the button first (for example, with touch or Safari).
    fireEvent.click(button);
    expect(screen.getByRole('region', { name: 'Settings content' })).toHaveFocus();
    navigation.focus();
    finish(false);

    await waitFor(() =>
      expect(screen.getByRole('status')).toHaveTextContent('Settings access checked.'),
    );
    expect(retry).toHaveBeenCalledTimes(1);
    expect(button).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Review settings' })).toBeEnabled();
    expect(navigation).toHaveFocus();

    setChecking(true);
    expect(screen.getByRole('status').textContent).toBe('');
    expect(navigation).toHaveFocus();
  });
});
