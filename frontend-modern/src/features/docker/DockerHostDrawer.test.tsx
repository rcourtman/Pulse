import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetAIRuntimeState } from '@/stores/aiRuntimeState';
import type { Resource } from '@/types/resource';
import { DockerHostDrawer } from './DockerHostDrawer';

const wsActiveAlerts = vi.hoisted(() => ({}) as Record<string, unknown>);
const listActionAudits = vi.hoisted(() =>
  vi.fn(async (_options?: { resourceId?: string; limit?: number }) => ({
    audits: [] as unknown[],
    count: 0,
    available: false,
  })),
);

vi.mock('@/api/actionAudit', () => ({ ActionAuditAPI: { listActionAudits } }));
vi.mock('@/api/resourceOperatorState', () => ({
  getResourceOperatorState: vi.fn().mockResolvedValue(null),
  setResourceOperatorState: vi.fn(),
  clearResourceOperatorState: vi.fn(),
}));

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
    currentMetrics?: Record<string, number | undefined>;
  }) => (
    <div
      data-testid="docker-host-history"
      data-alert-metrics={Object.keys(props.alerts ?? {}).join(',')}
      data-live-metrics={props.currentMetrics ? 'shown' : 'withheld'}
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
  it("lists a container's memory alert but draws no trigger for it on the host's charts", async () => {
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
    expect(await screen.findByTestId('docker-host-history')).toHaveAttribute(
      'data-alert-metrics',
      '',
    );
  });

  it('charts history on Overview with its range picker, and pauses it behind Manage', async () => {
    render(() => <DockerHostDrawer host={host()} />);
    expect(screen.queryByRole('tab', { name: 'History' })).toBeNull();
    const history = await screen.findByTestId('docker-host-history');
    expect(history).toHaveAttribute('data-range', '24h');
    expect(history).toHaveAttribute('data-range-picker', 'true');
    expect(history).toHaveAttribute('data-enabled', 'true');
    fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
    expect(screen.getByTestId('docker-host-history')).toHaveAttribute('data-enabled', 'false');
  });

  it('shows live readings for a reporting host', async () => {
    render(() => <DockerHostDrawer host={{ ...host(), temperature: 48 } as Resource} />);
    expect(await screen.findByTestId('docker-host-history')).toHaveAttribute(
      'data-live-metrics',
      'shown',
    );
    expect(screen.queryByTestId('docker-host-drawer-readings-withheld')).toBeNull();
  });

  it("does not present an offline host's last readings as live", async () => {
    render(() => <DockerHostDrawer host={{ ...host(), status: 'offline' } as Resource} />);
    expect(await screen.findByTestId('docker-host-history')).toHaveAttribute(
      'data-live-metrics',
      'withheld',
    );
    expect(screen.getByTestId('docker-host-drawer-readings-withheld')).toHaveTextContent(
      'Docker host is offline. Showing its history only.',
    );
  });

  it("does not present a stale host's last readings as live", async () => {
    render(() => (
      <DockerHostDrawer
        host={
          {
            ...host(),
            health: {
              verdict: 'stale',
              reasons: [{ code: 'telemetry_stale', detail: '12m' }],
            },
          } as Resource
        }
      />
    ));
    expect(await screen.findByTestId('docker-host-history')).toHaveAttribute(
      'data-live-metrics',
      'withheld',
    );
    expect(screen.getByTestId('docker-host-drawer-readings-withheld')).toHaveTextContent(
      'No report for 12m. Showing its history only.',
    );
  });
});

describe('DockerHostDrawer header', () => {
  it('opens the saved web interface from the header without collapsing the drawer', () => {
    const onClose = vi.fn();
    render(() => (
      <DockerHostDrawer
        host={host()}
        onClose={onClose}
        customUrl="https://portainer.example:9443"
      />
    ));
    const open = screen.getByRole('link', { name: 'Open web interface for docker-1' });
    expect(open).toHaveAttribute('href', 'https://portainer.example:9443');
    expect(open).toHaveTextContent('Open');
    fireEvent.click(open);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('shows no Open action without a saved web interface', () => {
    render(() => <DockerHostDrawer host={host()} onClose={vi.fn()} />);
    expect(screen.queryByRole('link', { name: /Open web interface/ })).toBeNull();
  });
});

describe('DockerHostDrawer Manage', () => {
  it('orders operator state, container updates, web interface URL, then action history', async () => {
    listActionAudits.mockResolvedValueOnce({
      audits: [
        {
          id: 'audit-1',
          createdAt: '2026-10-20T10:00:00Z',
          updatedAt: '2026-10-20T10:01:00Z',
          state: 'completed',
          request: { capabilityName: 'docker.container.update', requestedBy: 'admin' },
          plan: { approvalPolicy: 'admin' },
        },
      ],
      count: 1,
      available: true,
    });
    render(() => <DockerHostDrawer host={host()} />);
    expect(listActionAudits).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));

    const manage = screen.getByTestId('docker-host-manage-tab');
    const operator = within(manage).getByRole('region', { name: 'Operator overrides' });
    const updates = within(manage).getByTestId('docker-host-management-actions');
    const url = within(manage).getByText('Web Interface URL');
    const history = await within(manage).findByTestId('resource-action-history-section');
    expect(listActionAudits).toHaveBeenCalledWith({ resourceId: 'agent:docker-1', limit: 5 });
    const follows = (first: Element, second: Element) =>
      Boolean(first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING);
    expect(follows(operator, updates)).toBe(true);
    expect(follows(updates, url)).toBe(true);
    expect(follows(url, history)).toBe(true);
  });

  it('leaves action history out where the server does not offer it', async () => {
    render(() => <DockerHostDrawer host={host()} />);
    fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
    await vi.waitFor(() => expect(listActionAudits).toHaveBeenCalled());
    expect(screen.queryByTestId('resource-action-history-section')).toBeNull();
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

describe('DockerHostDrawer inventory', () => {
  const inventoryHost = (overrides: Partial<Resource> = {}): Resource =>
    ({
      ...host(),
      memory: { current: 50, total: 8 * 1024 ** 3, used: 4 * 1024 ** 3 },
      disk: { current: 25, total: 100 * 1024 ** 3, used: 25 * 1024 ** 3 },
      docker: {
        hostSourceId: 'docker-source-1',
        runtime: 'docker',
        dockerVersion: '27.3.1',
        os: 'Debian GNU/Linux 12',
        kernelVersion: '6.1.0',
        updatesAvailableCount: 2,
      },
      ...overrides,
    }) as Resource;

  it('lifts what needs checking, states one facts line and keeps the rest under Details', () => {
    render(() => <DockerHostDrawer host={inventoryHost()} />);
    expect(screen.getByTestId('drawer-no-attention')).toHaveTextContent('No active alerts');
    const lifted = screen.getByTestId('docker-host-drawer-exceptions');
    expect(within(lifted).getByText('Updates')).toBeInTheDocument();
    expect(within(lifted).getByText('2 available')).toBeInTheDocument();
    const facts = screen.getByTestId('docker-host-drawer-facts');
    // The row already states the engine version.
    expect(facts).not.toHaveTextContent('Docker 27.3.1');
    expect(facts).toHaveTextContent('Debian GNU/Linux 12');
    expect(facts).toHaveTextContent('4.00 GB of 8.00 GB RAM');
    expect(facts).toHaveTextContent('25.0 GB of 100 GB disk');
    const details = screen.getByTestId('docker-host-technical-details') as HTMLDetailsElement;
    expect(details.tagName).toBe('DETAILS');
    expect(details.open).toBe(false);
    expect(within(details).getByText('Kernel')).toBeInTheDocument();
    expect(within(details).queryByText('Updates')).toBeNull();
    // Values a reader copies or compares wrap instead of hiding behind an ellipsis.
    for (const value of ['6.1.0', 'Docker 27.3.1', 'Debian GNU/Linux 12', 'Direct API']) {
      const cell = within(details).getByText(value);
      expect(cell).toHaveClass('whitespace-normal');
      expect(cell).not.toHaveClass('truncate');
    }
  });

  it('lifts an offline host and states sizes, not used amounts that would read as live', () => {
    render(() => (
      <DockerHostDrawer
        host={inventoryHost({ status: 'offline', lastSeen: Date.now() - 600_000 })}
      />
    ));
    const lifted = screen.getByTestId('docker-host-drawer-exceptions');
    expect(within(lifted).getByText(/^Offline · last seen/)).toBeInTheDocument();
    const facts = screen.getByTestId('docker-host-drawer-facts');
    expect(facts).toHaveTextContent('8.00 GB RAM');
    expect(facts).not.toHaveTextContent('of 8.00 GB');
  });
});
