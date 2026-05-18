import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import containerUpdateBadgeSource from '@/components/shared/ContainerUpdateBadge.tsx?raw';
import containerUpdateBadgeModelSource from '@/components/shared/containerUpdateBadgeModel.ts?raw';
import containerUpdateButtonStateSource from '@/components/shared/useContainerUpdateButtonState.ts?raw';
import {
  ContainerUpdateBadge,
  UpdateButton,
} from '@/components/shared/ContainerUpdateBadge';

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    updateDockerContainer: vi.fn().mockResolvedValue(undefined),
  },
}));

vi.mock('@/stores/containerUpdates', () => ({
  clearContainerUpdateState: vi.fn(),
  getContainerUpdateState: vi.fn(() => undefined),
  markContainerQueued: vi.fn(),
  markContainerUpdateError: vi.fn(),
  markContainerUpdateSuccess: vi.fn(),
  updateStates: vi.fn(() => ({})),
}));

vi.mock('@/stores/systemSettings', () => ({
  areSystemSettingsLoaded: () => true,
  shouldHideDockerUpdateActions: () => false,
}));

afterEach(cleanup);

describe('ContainerUpdateBadge', () => {
  it('keeps the badge on shell, runtime, and model owners', () => {
    expect(containerUpdateBadgeSource).toContain('useContainerUpdateButtonState');
    expect(containerUpdateBadgeSource).toContain('getUpdateButtonClass');
    expect(containerUpdateBadgeSource).not.toContain('MonitoringAPI.updateDockerContainer');
    expect(containerUpdateBadgeSource).not.toContain('markContainerQueued');
    expect(containerUpdateBadgeSource).not.toContain('createSignal');

    expect(containerUpdateButtonStateSource).toContain(
      'MonitoringAPI.updateDockerContainer',
    );
    expect(containerUpdateButtonStateSource).toContain('markContainerQueued');
    expect(containerUpdateButtonStateSource).toContain('createSignal');
    expect(containerUpdateButtonStateSource).toContain(
      'export function useContainerUpdateButtonState',
    );

    expect(containerUpdateBadgeModelSource).toContain('getUpdateButtonClass');
    expect(containerUpdateBadgeModelSource).toContain('getUpdateButtonTooltip');
    expect(containerUpdateBadgeModelSource).toContain('hasContainerUpdate');
    expect(containerUpdateBadgeModelSource).toContain('hasContainerUpdateCurrent');
  });

  it('renders the error badge fallback when update detection fails', () => {
    render(() => (
      <ContainerUpdateBadge
        updateStatus={{
          updateAvailable: false,
          lastChecked: 0,
          error: 'request timed out',
        }}
      />
    ));

    expect(screen.getByText('Check failed')).toBeInTheDocument();
  });

  it('renders the update action button when an update is available', () => {
    render(() => (
      <UpdateButton
        agentId="agent-1"
        containerId="container-1"
        containerName="web"
        updateStatus={{
          updateAvailable: true,
          currentDigest: 'sha256:current',
          latestDigest: 'sha256:latest',
          lastChecked: 0,
        }}
      />
    ));

    expect(screen.getByRole('button', { name: /update/i })).toBeInTheDocument();
  });

  it('renders a visible current state when the checked image is up to date', () => {
    render(() => (
      <UpdateButton
        agentId="agent-1"
        containerId="container-1"
        containerName="web"
        compact={true}
        updateStatus={{
          updateAvailable: false,
          currentDigest: 'sha256:current',
          lastChecked: 0,
        }}
      />
    ));

    expect(screen.getByText('Current')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('renders a visible failed check state without exposing an update button', () => {
    render(() => (
      <UpdateButton
        agentId="agent-1"
        containerId="container-1"
        containerName="web"
        compact={true}
        updateStatus={{
          updateAvailable: false,
          lastChecked: 0,
          error: 'request timed out',
        }}
      />
    ));

    expect(screen.getByText('Failed')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
