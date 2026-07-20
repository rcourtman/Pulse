import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import containerUpdateBadgeSource from '@/components/shared/ContainerUpdateBadge.tsx?raw';
import containerUpdateBadgeModelSource from '@/components/shared/containerUpdateBadgeModel.ts?raw';
import containerUpdateButtonStateSource from '@/components/shared/useContainerUpdateButtonState.ts?raw';
import { ContainerUpdateBadge, UpdateButton } from '@/components/shared/ContainerUpdateBadge';
import { getUpdatePlanErrorMessage } from '@/components/shared/containerUpdateBadgeModel';

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {},
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

    expect(containerUpdateButtonStateSource).toContain('ResourceActionsAPI.planAction');
    expect(containerUpdateButtonStateSource).not.toContain('MonitoringAPI.updateDockerContainer');
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

  it('disables the button with the refusal reason when the server refuses the update capability', () => {
    render(() => (
      <UpdateButton
        agentId="agent-1"
        containerId="container-1"
        containerName="web"
        resourceId="resource-1"
        actionReadiness={[
          {
            name: 'update',
            available: false,
            reasonCode: 'command_agent_disconnected',
            reason: 'The Pulse agent on this host is still on an older version.',
          },
        ]}
        updateStatus={{
          updateAvailable: true,
          currentDigest: 'sha256:current',
          latestDigest: 'sha256:latest',
          lastChecked: 0,
        }}
      />
    ));

    const button = screen.getByRole('button', { name: /update unavailable/i });
    expect(button).toBeDisabled();
    expect(button.getAttribute('aria-label')).toContain(
      'The Pulse agent on this host is still on an older version.',
    );
  });

  it('keeps the button enabled when the update readiness entry is available', () => {
    render(() => (
      <UpdateButton
        agentId="agent-1"
        containerId="container-1"
        containerName="web"
        resourceId="resource-1"
        actionReadiness={[{ name: 'restart', available: false, reason: 'restart refused' }]}
        updateStatus={{
          updateAvailable: true,
          currentDigest: 'sha256:current',
          latestDigest: 'sha256:latest',
          lastChecked: 0,
        }}
      />
    ));

    expect(screen.getByRole('button', { name: /update/i })).not.toBeDisabled();
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

    expect(screen.getByText('Check failed')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});

describe('getUpdatePlanErrorMessage', () => {
  it('prefers the availability refusal reason over the generic message', () => {
    const error = Object.assign(new Error('Action execution is unavailable'), {
      details: {
        reasonCode: 'operation_receipt_unsupported',
        reason: 'The Pulse agent on this host is still on an older version.',
      },
    });

    expect(getUpdatePlanErrorMessage(error)).toBe(
      'The Pulse agent on this host is still on an older version.',
    );
  });

  it('falls back to the error message when no reason detail exists', () => {
    expect(getUpdatePlanErrorMessage(new Error('Pulse refused the update plan.'))).toBe(
      'Pulse refused the update plan.',
    );
  });

  it('falls back to a default when the error is empty', () => {
    expect(getUpdatePlanErrorMessage(new Error(''))).toBe('Failed to plan the update');
    expect(getUpdatePlanErrorMessage(null)).toBe('Failed to plan the update');
  });
});
