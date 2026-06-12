import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ResourceActionsAPI } from '@/api/resourceActions';
import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '../ResourceDetailDrawer';

vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: (props: {
    target: { resourceType: string; resourceId: string } | null;
    range: string;
    fallbackMetrics?: Record<string, number | undefined>;
  }) => (
    <div
      data-testid="container-history"
      data-resource-type={props.target?.resourceType}
      data-resource-id={props.target?.resourceId}
      data-range={props.range}
      data-cpu={props.fallbackMetrics?.cpu}
    />
  ),
  GuestDrawerHistoryRangeSelect: (props: {
    range: string;
    onRangeChange: (range: string) => void;
  }) => (
    <select
      aria-label="History range"
      value={props.range}
      onChange={(event) => props.onRangeChange(event.currentTarget.value)}
    >
      <option value="24h">24 hours</option>
      <option value="7d">7 days</option>
    </select>
  ),
}));

vi.mock('@/api/resourceActions', () => ({
  ResourceActionsAPI: {
    planAction: vi.fn().mockResolvedValue({
      actionId: 'detail-action-1',
      requestId: 'detail-request-1',
      allowed: true,
      requiresApproval: true,
      approvalPolicy: 'admin',
      rollbackAvailable: false,
    }),
    decideAction: vi.fn().mockResolvedValue({
      actionId: 'detail-action-1',
      outcome: 'approved',
      decidedAt: '2026-06-12T20:00:01Z',
    }),
    executeAction: vi.fn().mockResolvedValue({
      actionId: 'detail-action-1',
      state: 'completed',
      result: { success: true },
    }),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
  },
}));

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'app-container-1',
    name: overrides.name ?? overrides.id ?? 'app-container-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'app-container-1',
    type: overrides.type ?? 'app-container',
    platformId: 'docker',
    platformType: 'docker',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('ResourceDetailDrawer for Docker containers', () => {
  it('renders governed lifecycle controls in Docker container details', async () => {
    const onResourceActionSettled = vi.fn();

    render(() => (
      <ResourceDetailDrawer
        presentation="table-row"
        onResourceActionSettled={onResourceActionSettled}
        resource={resource({
          id: 'app-container-web',
          name: 'edge-web',
          docker: {
            agentId: 'agent-edge',
            containerId: 'abc123def456',
            containerState: 'running',
            runtime: 'docker',
            image: 'ghcr.io/example/edge-web:2026.05',
          },
          capabilities: [
            {
              name: 'restart',
              type: 'common',
              platform: 'docker',
              minimumApprovalLevel: 'admin',
            },
          ],
          sourceStatus: { docker: { status: 'online' } },
        })}
      />
    ));

    const restartButton = screen.getByRole('button', {
      name: 'Restart edge-web through governed action',
    });
    expect(restartButton.closest('[data-docker-container-actions-surface]')).toHaveAttribute(
      'data-docker-container-actions-surface',
      'resource-detail',
    );

    fireEvent.click(restartButton);
    fireEvent.click(screen.getByRole('button', { name: 'Click again to restart edge-web' }));

    await waitFor(() =>
      expect(ResourceActionsAPI.planAction).toHaveBeenCalledWith(
        expect.objectContaining({
          resourceId: 'app-container-web',
          capabilityName: 'restart',
          requestedBy: 'ui:resource-detail',
        }),
      ),
    );
    await waitFor(() =>
      expect(ResourceActionsAPI.executeAction).toHaveBeenCalledWith(
        'detail-action-1',
        expect.stringContaining('from the resource details'),
      ),
    );
    expect(ResourceActionsAPI.decideAction).toHaveBeenCalledWith(
      'detail-action-1',
      'approved',
      expect.stringContaining('restart Docker container edge-web'),
    );
    await waitFor(() => expect(onResourceActionSettled).toHaveBeenCalledTimes(1));
  });

  it('adds a metrics history tab for app-containers with a metrics target', async () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'app-container-web',
          name: 'edge-web',
          metricsTarget: { resourceType: 'app-container', resourceId: 'abc123def456' },
          cpu: { current: 75 },
        })}
      />
    ));

    await fireEvent.click(screen.getByRole('tab', { name: 'History' }));

    const history = screen.getByTestId('container-history');
    expect(history).toHaveAttribute('data-resource-type', 'app-container');
    expect(history).toHaveAttribute('data-resource-id', 'abc123def456');
    expect(history).toHaveAttribute('data-cpu', '75');
  });

  it('omits the history tab when no metrics target resolves', () => {
    render(() => (
      <ResourceDetailDrawer
        resource={resource({
          id: 'docker-image-1',
          type: 'docker-image',
          name: 'nginx:latest',
        })}
      />
    ));

    expect(screen.queryByRole('tab', { name: 'History' })).not.toBeInTheDocument();
  });

  it('renders container runtime facts in the table-row summary', () => {
    render(() => (
      <ResourceDetailDrawer
        presentation="table-row"
        resource={resource({
          id: 'app-container-web',
          name: 'edge-web',
          docker: {
            containerId: 'abc123def456',
            containerState: 'running',
            image: 'ghcr.io/example/edge-web:2026.05',
            restartCount: 7,
            createdAt: '2026-04-13T01:14:01Z',
            startedAt: '2026-04-13T01:15:01Z',
            finishedAt: '2026-04-13T02:15:01Z',
            blockIo: { readBytes: 1_048_576, writeBytes: 2_097_152 },
            podman: {
              podName: 'edge-pod',
              podId: 'pod-123',
              infra: true,
              composeProject: 'orion',
              composeService: 'web',
              autoUpdatePolicy: 'registry',
              userNamespace: 'keep-id',
            },
            labels: {
              'com.docker.compose.project': 'legacy-project',
              'com.docker.compose.service': 'legacy-service',
              'traefik.enable': 'true',
            },
          },
        })}
      />
    ));

    const section = screen.getByTestId('resource-docker-container-section');
    expect(within(section).getByText('Image')).toBeInTheDocument();
    expect(within(section).getByText('ghcr.io/example/edge-web:2026.05')).toBeInTheDocument();
    expect(within(section).getByText('Restarts')).toBeInTheDocument();
    expect(within(section).getByText('7')).toHaveClass('text-red-600');
    expect(within(section).getByText('Created')).toBeInTheDocument();
    expect(within(section).getByText('Started')).toBeInTheDocument();
    expect(within(section).getByText('Finished')).toBeInTheDocument();
    expect(within(section).getAllByText(/ago$/).length).toBeGreaterThanOrEqual(3);
    expect(within(section).getByText('Podman pod')).toBeInTheDocument();
    expect(within(section).getByText('edge-pod')).toBeInTheDocument();
    expect(within(section).getByText('Podman pod ID')).toBeInTheDocument();
    expect(within(section).getByText('pod-123')).toBeInTheDocument();
    expect(within(section).getByText('Podman infra')).toBeInTheDocument();
    expect(within(section).getByText('Yes')).toBeInTheDocument();
    expect(within(section).getByText('Compose project')).toBeInTheDocument();
    expect(within(section).getByText('orion')).toBeInTheDocument();
    expect(within(section).queryByText('legacy-project')).not.toBeInTheDocument();
    expect(within(section).getByText('Compose service')).toBeInTheDocument();
    expect(within(section).getByText('Auto-update')).toBeInTheDocument();
    expect(within(section).getByText('registry')).toBeInTheDocument();
    expect(within(section).getByText('User namespace')).toBeInTheDocument();
    expect(within(section).getByText('keep-id')).toBeInTheDocument();
    expect(within(section).getByText('Block I/O read')).toBeInTheDocument();
    expect(within(section).getByText('1.00 MB')).toBeInTheDocument();
    expect(within(section).getByText('Block I/O write')).toBeInTheDocument();
    expect(within(section).getByText('2.00 MB')).toBeInTheDocument();
    expect(within(section).getByText('Labels')).toBeInTheDocument();
    expect(within(section).getByText(/traefik\.enable/)).toBeInTheDocument();
  });

  it('does not render the container section for non-container resources', () => {
    render(() => (
      <ResourceDetailDrawer
        presentation="table-row"
        resource={resource({
          id: 'docker-volume-1',
          type: 'docker-volume',
          name: 'app-data',
          docker: { volumeName: 'app-data', driver: 'local', createdAt: '2026-04-13T01:14:01Z' },
        })}
      />
    ));

    expect(screen.queryByTestId('resource-docker-container-section')).not.toBeInTheDocument();
  });
});
