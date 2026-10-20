import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';

const mockCheck = vi.fn();
const mockApply = vi.fn();
const mockNotifyError = vi.fn();

vi.mock('@/api/updates', () => ({
  UpdatesAPI: {
    checkProRuntimeSwitch: (...args: unknown[]) => mockCheck(...args),
    applyProRuntimeSwitch: (...args: unknown[]) => mockApply(...args),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: { error: (...args: unknown[]) => mockNotifyError(...args) },
}));

import { ProRuntimeSwitchCard } from '../ProRuntimeSwitchCard';

describe('ProRuntimeSwitchCard', () => {
  beforeEach(() => {
    mockCheck.mockReset();
    mockApply.mockReset();
    mockNotifyError.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('does not ask the server when the install is not a community build', () => {
    render(() => <ProRuntimeSwitchCard shouldCheck={false} />);
    expect(mockCheck).not.toHaveBeenCalled();
    expect(screen.queryByTestId('pro-runtime-switch')).not.toBeInTheDocument();
  });

  it('renders nothing when the licence is not eligible', async () => {
    mockCheck.mockResolvedValue({
      available: false,
      reason: 'not_eligible',
      currentVersion: '6.5.0',
      installInPlace: false,
    });
    render(() => <ProRuntimeSwitchCard shouldCheck />);
    await waitFor(() => expect(mockCheck).toHaveBeenCalledTimes(1));
    expect(screen.queryByTestId('pro-runtime-switch')).not.toBeInTheDocument();
  });

  it('explains the wait, with no switch button, when only a preview Pro build exists', async () => {
    mockCheck.mockResolvedValue({
      available: false,
      reason: 'prerelease_only',
      currentVersion: '6.5.0',
      version: '6.6.0-rc.1',
      installInPlace: false,
    });
    render(() => <ProRuntimeSwitchCard shouldCheck />);
    const card = await screen.findByTestId('pro-runtime-switch');
    expect(card).toHaveTextContent('The stable Pulse Pro build is not out yet');
    expect(card).toHaveTextContent('v6.6.0-rc.1');
    expect(screen.queryByRole('button', { name: /Switch to Pulse Pro/ })).not.toBeInTheDocument();
  });

  it('confirms before starting an in-place switch', async () => {
    mockCheck.mockResolvedValue({
      available: true,
      currentVersion: '6.5.0',
      version: '6.5.0',
      installInPlace: true,
    });
    mockApply.mockResolvedValue({ status: 'started', message: 'ok' });
    render(() => <ProRuntimeSwitchCard shouldCheck />);

    fireEvent.click(await screen.findByRole('button', { name: 'Switch to Pulse Pro v6.5.0' }));
    expect(mockApply).not.toHaveBeenCalled();
    expect(screen.getByText(/Pulse restarts during the switch/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Switch now' }));
    await waitFor(() => expect(mockApply).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Switch now' })).not.toBeInTheDocument(),
    );
    expect(screen.getByRole('status')).toHaveTextContent('Switch requested.');
    expect(screen.queryByRole('button', { name: /Switch to Pulse Pro/ })).not.toBeInTheDocument();
  });

  it('reports a refused switch and stays on the confirm step', async () => {
    mockCheck.mockResolvedValue({
      available: true,
      currentVersion: '6.5.0',
      version: '6.5.0',
      installInPlace: true,
    });
    mockApply.mockRejectedValue(
      Object.assign(new Error('Update already in progress'), { status: 409 }),
    );
    render(() => <ProRuntimeSwitchCard shouldCheck />);

    fireEvent.click(await screen.findByRole('button', { name: 'Switch to Pulse Pro v6.5.0' }));
    fireEvent.click(screen.getByRole('button', { name: 'Switch now' }));
    await waitFor(() => expect(mockNotifyError).toHaveBeenCalledWith('Update already in progress'));
    expect(screen.getByRole('button', { name: 'Switch now' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Switch now' })).toHaveFocus();
  });

  it('keeps keyboard focus in the confirmation and restores it on cancellation', async () => {
    mockCheck.mockResolvedValue({
      available: true,
      currentVersion: '6.5.0',
      version: '6.5.0',
      installInPlace: true,
    });
    render(() => <ProRuntimeSwitchCard shouldCheck />);
    fireEvent.click(await screen.findByRole('button', { name: /Switch to Pulse Pro/ }));
    expect(screen.getByRole('button', { name: 'Switch now' })).toHaveFocus();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.getByRole('button', { name: /Switch to Pulse Pro/ })).toHaveFocus();
    expect(mockApply).not.toHaveBeenCalled();
  });

  it('does not invite another switch after a lost response', async () => {
    mockCheck.mockResolvedValue({
      available: true,
      currentVersion: '6.5.0',
      version: '6.5.0',
      installInPlace: true,
    });
    mockApply.mockRejectedValue(new TypeError('Failed to fetch'));
    render(() => <ProRuntimeSwitchCard shouldCheck />);
    fireEvent.click(await screen.findByRole('button', { name: /Switch to Pulse Pro/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Switch now' }));
    const notice = await screen.findByRole('status');
    expect(notice).toHaveTextContent('Could not confirm whether the switch started.');
    expect(notice).toHaveFocus();
    expect(screen.queryByRole('button', { name: /Switch/ })).not.toBeInTheDocument();
    expect(mockApply).toHaveBeenCalledTimes(1);
    expect(mockNotifyError).not.toHaveBeenCalled();
  });

  it('owns a pending submission through its acknowledgement', async () => {
    mockCheck.mockResolvedValue({
      available: true,
      currentVersion: '6.5.0',
      version: '6.5.0',
      installInPlace: true,
    });
    let acknowledge!: (result: { status: string; message: string }) => void;
    mockApply.mockImplementation(
      () =>
        new Promise((resolve) => {
          acknowledge = resolve;
        }),
    );
    render(() => <ProRuntimeSwitchCard shouldCheck />);
    fireEvent.click(await screen.findByRole('button', { name: /Switch to Pulse Pro/ }));
    const submit = screen.getByRole('button', { name: 'Switch now' });
    fireEvent.click(submit);
    fireEvent.click(submit);
    expect(mockApply).toHaveBeenCalledTimes(1);
    expect(submit).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled();
    acknowledge({ status: 'started', message: 'ok' });
    await screen.findByText(/Switch requested/);
    expect(screen.queryByRole('button', { name: /Switch/ })).not.toBeInTheDocument();
  });

  it('shows a quiet notice when the check fails, without offering a switch', async () => {
    mockCheck.mockRejectedValue(new Error('unreachable'));
    render(() => <ProRuntimeSwitchCard shouldCheck />);
    expect(await screen.findByTestId('pro-runtime-switch')).toHaveTextContent('Could not check');
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('withdraws a retained offer when the build becomes ineligible', async () => {
    mockCheck.mockResolvedValue({
      available: true,
      currentVersion: '6.5.0',
      version: '6.5.0',
      installInPlace: true,
    });
    const [shouldCheck, setShouldCheck] = createSignal(true);
    render(() => <ProRuntimeSwitchCard shouldCheck={shouldCheck()} />);
    await screen.findByRole('button', { name: /Switch to Pulse Pro/ });
    setShouldCheck(false);
    expect(screen.queryByTestId('pro-runtime-switch')).not.toBeInTheDocument();
  });

  it('lists the Docker commands with copy buttons', async () => {
    mockCheck.mockResolvedValue({
      available: true,
      currentVersion: '6.5.0',
      version: '6.5.0',
      installInPlace: false,
      docker: {
        version: 'v6.5.0',
        image: 'registry.pulserelay.pro/pulse/pulse-pro',
        imageDigest: 'sha256:' + 'ab'.repeat(32),
        composePullCommand: "PULSE_IMAGE='x@sha256:ab' docker compose pull",
        composeUpCommand: "PULSE_IMAGE='x@sha256:ab' docker compose up -d",
      },
    });
    render(() => <ProRuntimeSwitchCard shouldCheck />);

    expect(
      await screen.findByText("PULSE_IMAGE='x@sha256:ab' docker compose pull"),
    ).toBeInTheDocument();
    expect(screen.getByText("PULSE_IMAGE='x@sha256:ab' docker compose up -d")).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Copy to clipboard' })).toHaveLength(2);
    expect(screen.getByText(/The commands set PULSE_IMAGE/)).toHaveClass(
      '[overflow-wrap:anywhere]',
    );
    expect(screen.queryByRole('button', { name: /Switch to Pulse Pro/ })).not.toBeInTheDocument();
  });
});
