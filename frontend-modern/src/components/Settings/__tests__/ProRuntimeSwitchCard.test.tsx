import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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
  });

  it('reports a refused switch and stays on the confirm step', async () => {
    mockCheck.mockResolvedValue({
      available: true,
      currentVersion: '6.5.0',
      version: '6.5.0',
      installInPlace: true,
    });
    mockApply.mockRejectedValue(new Error('Update already in progress'));
    render(() => <ProRuntimeSwitchCard shouldCheck />);

    fireEvent.click(await screen.findByRole('button', { name: 'Switch to Pulse Pro v6.5.0' }));
    fireEvent.click(screen.getByRole('button', { name: 'Switch now' }));
    await waitFor(() => expect(mockNotifyError).toHaveBeenCalledWith('Update already in progress'));
    expect(screen.getByRole('button', { name: 'Switch now' })).toBeInTheDocument();
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
    expect(screen.queryByRole('button', { name: /Switch to Pulse Pro/ })).not.toBeInTheDocument();
  });
});
