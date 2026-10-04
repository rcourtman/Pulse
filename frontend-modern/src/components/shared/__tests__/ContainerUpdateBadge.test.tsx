import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import containerUpdateBadgeSource from '@/components/shared/ContainerUpdateBadge.tsx?raw';
import containerUpdateBadgeModelSource from '@/components/shared/containerUpdateBadgeModel.ts?raw';
import containerUpdateButtonStateSource from '@/components/shared/useContainerUpdateButtonState.ts?raw';
import { ContainerUpdateBadge, UpdateButton } from '@/components/shared/ContainerUpdateBadge';
import { getUpdatePlanErrorMessage } from '@/components/shared/containerUpdateBadgeModel';
import { ResourceActionsAPI } from '@/api/resourceActions';
import {
  getContainerUpdateState,
  markContainerUpdateError,
  markContainerUpdateInconclusive,
  markContainerUpdateSuccess,
} from '@/stores/containerUpdates';

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {},
}));

vi.mock('@/stores/containerUpdates', () => ({
  clearContainerUpdateState: vi.fn(),
  getContainerUpdateState: vi.fn(() => undefined),
  markContainerQueued: vi.fn(),
  markContainerUpdateError: vi.fn(),
  markContainerUpdateInconclusive: vi.fn(),
  markContainerUpdateSuccess: vi.fn(),
  updateStates: vi.fn(() => ({})),
}));

vi.mock('@/api/resourceActions', () => ({
  ResourceActionsAPI: { getAction: vi.fn(), planAction: vi.fn() },
}));

vi.mock('@/features/actions/ActionReviewDialog', () => ({
  ActionReviewDialog: (props: { detail: { audit: { id: string } } | null }) => (
    <div data-testid="action-review-fixture">{props.detail?.audit.id}</div>
  ),
}));

vi.mock('@/stores/systemSettings', () => ({
  areSystemSettingsLoaded: () => true,
  shouldHideDockerUpdateActions: () => false,
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.mocked(getContainerUpdateState).mockReturnValue(undefined);
});

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

  it('renders the pinned badge instead of a failure for digest-pinned images', () => {
    render(() => (
      <ContainerUpdateBadge
        updateStatus={{
          updateAvailable: false,
          lastChecked: 0,
          error: 'digest-pinned image',
        }}
      />
    ));

    expect(screen.getByText('Pinned')).toBeInTheDocument();
    expect(screen.queryByText('Check failed')).not.toBeInTheDocument();
  });

  it('renders the pinned badge through the update button for digest-pinned images', () => {
    render(() => (
      <UpdateButton
        agentId="agent-1"
        containerId="container-1"
        containerName="backup"
        updateStatus={{
          updateAvailable: false,
          currentDigest: 'sha256:current',
          lastChecked: 0,
          error: 'digest-pinned image',
        }}
      />
    ));

    expect(screen.getByText('Pinned')).toBeInTheDocument();
    expect(screen.queryByText('Check failed')).not.toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
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

  it('keeps an audited update pending when the registry check no longer shows an update', async () => {
    vi.mocked(getContainerUpdateState).mockReturnValue({
      state: 'queued',
      startedAt: Date.now() - 60_000,
      actionId: 'action-pending-1',
    });
    vi.mocked(ResourceActionsAPI.getAction).mockResolvedValue({
      audit: { id: 'action-pending-1', state: 'executing' },
    } as never);
    render(() => (
      <UpdateButton
        agentId="agent-1"
        containerId="container-1"
        containerName="web"
        resourceId="resource-1"
        updateStatus={{ updateAvailable: false, currentDigest: 'sha256:current', lastChecked: 1 }}
      />
    ));

    const review = screen.getByRole('button', { name: /outcome not yet known/i });
    expect(review).toBeEnabled();
    expect(screen.getByText('Review action')).toBeVisible();
    expect(screen.queryByText('Current')).not.toBeInTheDocument();
    expect(markContainerUpdateSuccess).not.toHaveBeenCalled();

    fireEvent.click(review);
    await waitFor(() =>
      expect(ResourceActionsAPI.getAction).toHaveBeenCalledWith('action-pending-1'),
    );
    expect(await screen.findByTestId('action-review-fixture')).toHaveTextContent(
      'action-pending-1',
    );
    expect(ResourceActionsAPI.planAction).not.toHaveBeenCalled();
    expect(markContainerUpdateSuccess).not.toHaveBeenCalled();
  });

  it('keeps an inconclusive audit reviewable instead of offering another update', async () => {
    vi.mocked(getContainerUpdateState).mockReturnValue({
      state: 'inconclusive',
      startedAt: Date.now() - 3600_000,
      actionId: 'action-unknown-1',
    });
    vi.mocked(ResourceActionsAPI.getAction).mockRejectedValueOnce(new Error('offline'));
    render(() => (
      <UpdateButton
        agentId="agent-1"
        containerId="container-1"
        containerName="web"
        resourceId="resource-1"
        updateStatus={{ updateAvailable: true, lastChecked: 1 }}
      />
    ));

    const review = screen.getByRole('button', { name: /outcome unknown/i });
    fireEvent.click(review);
    await waitFor(() =>
      expect(ResourceActionsAPI.getAction).toHaveBeenCalledWith('action-unknown-1'),
    );
    expect(ResourceActionsAPI.planAction).not.toHaveBeenCalled();
    expect(screen.getByText('Review action')).toBeVisible();
    expect(screen.getByRole('button', { name: /Could not re-read the action/i })).toBeVisible();
  });

  it('keeps an operator-closed audit outcome unknown rather than showing an update failure', async () => {
    vi.mocked(getContainerUpdateState).mockReturnValue({
      state: 'queued',
      startedAt: Date.now() - 60_000,
      actionId: 'action-pending-1',
    });
    vi.mocked(ResourceActionsAPI.getAction).mockResolvedValueOnce({
      audit: {
        id: 'action-pending-1',
        state: 'failed',
        result: { actionResultV2: { execution: { reasonCode: 'operator_force_failed' } } },
      },
    } as never);
    render(() => (
      <UpdateButton
        agentId="agent-1"
        containerId="container-1"
        containerName="web"
        resourceId="resource-1"
        updateStatus={{ updateAvailable: true, lastChecked: 1 }}
      />
    ));

    fireEvent.click(screen.getByRole('button', { name: /outcome not yet known/i }));
    await waitFor(() =>
      expect(markContainerUpdateInconclusive).toHaveBeenCalledWith(
        'agent-1',
        'container-1',
        'action-pending-1',
      ),
    );
    expect(markContainerUpdateError).not.toHaveBeenCalled();
    expect(ResourceActionsAPI.planAction).not.toHaveBeenCalled();
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
