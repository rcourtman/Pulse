import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetAIRuntimeState } from '@/stores/aiRuntimeState';
import type { Resource } from '@/types/resource';
import { DockerHostDrawer } from './DockerHostDrawer';

const wsActiveAlerts = vi.hoisted(() => ({}) as Record<string, unknown>);

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: wsActiveAlerts }),
}));

vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: (props: {
    target: { resourceType: string; resourceId: string };
    range: string;
    enabled?: boolean;
    onRangeChange?: (range: string) => void;
    alerts?: Record<string, unknown>;
  }) => (
    <div
      data-testid="docker-host-history"
      data-alert-metrics={Object.keys(props.alerts ?? {}).join(',')}
      data-target={props.target.resourceId}
      data-range={props.range}
      data-enabled={String(props.enabled)}
      data-range-picker={String(Boolean(props.onRangeChange))}
    />
  ),
  GuestDrawerHistoryRangeSelect: () => <select aria-label="History range" />,
}));

const host = (): Resource =>
  ({
    id: 'agent:docker-1',
    name: 'docker-1',
    displayName: 'Docker 1',
    type: 'agent',
    platformId: 'docker-1',
    platformType: 'docker',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    discoveryTarget: {
      resourceType: 'agent',
      agentId: 'agent:docker-1',
      resourceId: 'agent:docker-1',
      hostname: 'docker-1',
    },
    docker: {
      hostSourceId: 'docker-source-1',
    },
  }) as Resource;

beforeEach(() => {
  resetAIRuntimeState();
});

afterEach(() => {
  cleanup();
  resetAIRuntimeState();
  vi.clearAllMocks();
  for (const id of Object.keys(wsActiveAlerts)) delete wsActiveAlerts[id];
});

const openAlert = (id: string, resourceId: string, message: string, level = 'warning') => ({
  id,
  type: 'docker-service-health',
  level,
  resourceId,
  resourceName: resourceId,
  node: 'docker-1',
  instance: 'Docker',
  message,
  value: 0,
  threshold: 0,
  startTime: '2026-10-06T10:00:00Z',
  acknowledged: false,
});

describe('DockerHostDrawer open alerts', () => {
  it('lists the open alerts of the host and its containers under Needs attention', () => {
    // Docker alerts key on "docker:<host source id>" and nest containers and
    // services under it. Resources carry no embedded alert list.
    wsActiveAlerts.offline = openAlert(
      'offline',
      'docker:docker-source-1',
      "Docker host 'Docker 1' is offline",
      'critical',
    );
    wsActiveAlerts.service = openAlert(
      'service',
      'docker:docker-source-1/service/svc-1',
      'Service backend-sftp has 0/2 running tasks',
    );
    wsActiveAlerts.elsewhere = openAlert(
      'elsewhere',
      'docker:docker-source-10/service/svc-1',
      'Another host service is down',
    );

    render(() => <DockerHostDrawer host={host()} />);

    const section = screen.getByTestId('drawer-attention-section');
    expect(section).toHaveTextContent('2 active');
    expect(section).toHaveTextContent("Docker host 'Docker 1' is offline");
    expect(section).toHaveTextContent('Service backend-sftp has 0/2 running tasks');
    expect(section).not.toHaveTextContent('Another host service is down');
  });
});

describe('DockerHostDrawer history', () => {
  it("lists a container's memory alert but draws no trigger for it on the host's charts", () => {
    wsActiveAlerts.container = {
      ...openAlert(
        'container',
        'docker:docker-source-1/abc123',
        'Container memory at 97%',
        'critical',
      ),
      type: 'memory',
      threshold: 85,
    };
    render(() => <DockerHostDrawer host={host()} />);
    expect(screen.getByTestId('drawer-attention-section')).toHaveTextContent(
      'Container memory at 97%',
    );
    expect(screen.getByTestId('docker-host-history')).toHaveAttribute('data-alert-metrics', '');
  });

  it('charts history on Overview with its range picker, and pauses it behind Manage', () => {
    render(() => <DockerHostDrawer host={host()} />);
    expect(screen.queryByRole('tab', { name: 'History' })).toBeNull();
    const history = screen.getByTestId('docker-host-history');
    expect(history).toHaveAttribute('data-range', '24h');
    expect(history).toHaveAttribute('data-range-picker', 'true');
    expect(history).toHaveAttribute('data-enabled', 'true');
    fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
    expect(screen.getByTestId('docker-host-history')).toHaveAttribute('data-enabled', 'false');
  });
});

describe('DockerHostDrawer typed-helper summary mode', () => {
  it('warns about reduced coverage and hides container update controls', async () => {
    const summaryHost = host();
    if (summaryHost.docker) {
      summaryHost.docker.collectionMode = 'typed-helper-summary';
    }

    render(() => <DockerHostDrawer host={summaryHost} />);

    expect(screen.getByText('Reduced container coverage')).toBeInTheDocument();
    expect(screen.getByText(/typed helper reports container summaries only/i)).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
    expect(screen.queryByTestId('docker-host-management-actions')).toBeNull();
  });
});
