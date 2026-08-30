import { fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AvailabilityTargetsAPI } from '@/api/availabilityTargets';
import { listDiscoveriesByAgent, updateAvailabilityProposal } from '@/api/discovery';
import type { ResourceDiscovery } from '@/types/discovery';
import { AvailabilityProposalCard } from '../AvailabilityProposalCard';

vi.mock('@/api/availabilityTargets', () => ({
  AvailabilityTargetsAPI: {
    list: vi.fn(),
    create: vi.fn(),
    test: vi.fn(),
  },
}));

vi.mock('@/api/discovery', () => ({
  listDiscoveriesByAgent: vi.fn(),
  updateAvailabilityProposal: vi.fn(),
}));

const discovery = (overrides: Partial<ResourceDiscovery> = {}): ResourceDiscovery => ({
  id: 'docker:agent-1:grafana',
  resource_type: 'app-container',
  resource_id: 'grafana',
  target_id: 'agent-1',
  hostname: 'docker-1',
  service_type: 'grafana',
  service_name: 'Grafana',
  service_version: '12',
  category: 'monitoring',
  cli_access: '',
  facts: [],
  config_paths: [],
  data_paths: [],
  log_paths: [],
  ports: [],
  user_notes: '',
  user_secrets: {},
  confidence: 0.98,
  ai_reasoning: '',
  discovered_at: '2026-08-30T12:00:00Z',
  updated_at: '2026-08-30T12:00:00Z',
  scan_duration: 10,
  suggested_availability_probe: {
    protocol: 'http',
    address: '10.0.0.8',
    port: 3000,
    path: '/',
    service_name: 'Grafana',
    reason: 'service default: grafana',
    evidence_fingerprint: 'sha256:grafana',
  },
  ...overrides,
});

describe('AvailabilityProposalCard', () => {
  beforeEach(() => {
    vi.mocked(AvailabilityTargetsAPI.list).mockReset().mockResolvedValue([]);
    vi.mocked(AvailabilityTargetsAPI.create)
      .mockReset()
      .mockImplementation(async (target) => ({
        ...target,
        id: 'created-check',
      }));
    vi.mocked(AvailabilityTargetsAPI.test)
      .mockReset()
      .mockResolvedValue({
        success: true,
        latencyMillis: 18,
        application: { outcome: 'passed', statusCode: 200 },
      });
    vi.mocked(listDiscoveriesByAgent).mockReset().mockResolvedValue({
      discoveries: [],
      total: 0,
    });
    vi.mocked(updateAvailabilityProposal)
      .mockReset()
      .mockResolvedValue(discovery({ dismissed_availability_probe_fingerprint: 'sha256:grafana' }));
  });

  it('previews provenance and creates one explicit canonical active check', async () => {
    const onDiscoveryUpdated = vi.fn();
    render(() => (
      <AvailabilityProposalCard
        discovery={discovery()}
        resourceType="app-container"
        targetId="agent-1"
        resourceId="grafana"
        canonicalResourceId="resource:grafana"
        connectedAgents={[
          {
            agent_id: 'edge-1',
            hostname: 'Edge office',
            version: '6.4.0',
            platform: 'linux',
            connected_at: '2026-08-30T12:00:00Z',
          },
        ]}
        onDiscoveryUpdated={onDiscoveryUpdated}
      />
    ));

    expect(screen.getByText('Endpoint · inferred')).toBeInTheDocument();
    expect(screen.getByText('Expected behavior · reviewed default')).toBeInTheDocument();
    expect(screen.getByText('GET returns HTTP 200–399')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Pulse inferred a useful check from this service. Review exactly what will run. Nothing is created until you choose the active-check action below.',
      ),
    ).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: 'Review machine suggestions' }));
    expect(
      screen.getByText(
        'Review the services discovered through docker-1. Dismissing is evidence-specific. Creating a check still happens from its canonical resource so Pulse never guesses the attachment.',
      ),
    ).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Close machine suggestions' }));

    await fireEvent.change(screen.getByLabelText('Observation location · you control'), {
      target: { value: 'edge-1' },
    });
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Create active check' })).toBeEnabled(),
    );
    await fireEvent.click(screen.getByRole('button', { name: 'Create active check' }));

    await waitFor(() => expect(AvailabilityTargetsAPI.create).toHaveBeenCalledTimes(1));
    expect(AvailabilityTargetsAPI.create).toHaveBeenCalledWith(
      expect.objectContaining({
        enabled: true,
        linkedResourceId: 'resource:grafana',
        probeAgentId: 'edge-1',
        http: expect.objectContaining({
          method: 'GET',
          expectedStatusMin: 200,
          expectedStatusMax: 399,
        }),
      }),
    );
    expect(screen.getByText(/created and attached to this resource/i)).toHaveAttribute(
      'role',
      'status',
    );
  });

  it('blocks an equivalent standalone endpoint and records evidence-bound dismissal', async () => {
    vi.mocked(AvailabilityTargetsAPI.list).mockResolvedValue([
      {
        id: 'standalone-grafana',
        name: 'Existing Grafana',
        targetKind: 'service',
        address: '10.0.0.8',
        protocol: 'http',
        port: 3000,
        path: '/',
        enabled: true,
      },
    ]);
    const onDiscoveryUpdated = vi.fn();
    render(() => (
      <AvailabilityProposalCard
        discovery={discovery()}
        resourceType="app-container"
        targetId="agent-1"
        resourceId="grafana"
        canonicalResourceId="resource:grafana"
        onDiscoveryUpdated={onDiscoveryUpdated}
      />
    ));

    await waitFor(() =>
      expect(screen.getByText(/already covered by “Existing Grafana”/i)).toHaveAttribute(
        'role',
        'status',
      ),
    );
    expect(screen.getByRole('button', { name: 'Create active check' })).toBeDisabled();

    await fireEvent.click(screen.getByRole('button', { name: 'Not now' }));
    await waitFor(() =>
      expect(updateAvailabilityProposal).toHaveBeenCalledWith(
        'app-container',
        'agent-1',
        'grafana',
        'sha256:grafana',
        'dismissed',
      ),
    );
    expect(onDiscoveryUpdated).toHaveBeenCalled();
  });

  it('keeps creation unavailable until duplicate checking succeeds', async () => {
    let resolveTargets!: (targets: []) => void;
    vi.mocked(AvailabilityTargetsAPI.list).mockImplementation(
      () => new Promise((resolve) => (resolveTargets = resolve)),
    );

    render(() => (
      <AvailabilityProposalCard
        discovery={discovery()}
        resourceType="app-container"
        targetId="agent-1"
        resourceId="grafana"
        canonicalResourceId="resource:grafana"
        onDiscoveryUpdated={vi.fn()}
      />
    ));

    expect(screen.getByText('Checking existing active checks…')).toHaveAttribute('role', 'status');
    const createButton = screen.getByRole('button', { name: 'Create active check' });
    expect(createButton).toBeDisabled();
    await fireEvent.click(createButton);
    expect(AvailabilityTargetsAPI.create).not.toHaveBeenCalled();

    resolveTargets([]);
    await waitFor(() => expect(createButton).toBeEnabled());
  });

  it('fails duplicate checking closed and explains why creation is unavailable', async () => {
    vi.mocked(AvailabilityTargetsAPI.list)
      .mockRejectedValueOnce(new Error('inventory unavailable'))
      .mockResolvedValueOnce([]);

    render(() => (
      <AvailabilityProposalCard
        discovery={discovery()}
        resourceType="app-container"
        targetId="agent-1"
        resourceId="grafana"
        canonicalResourceId="resource:grafana"
        onDiscoveryUpdated={vi.fn()}
      />
    ));

    const failure = await screen.findByText(/could not check for existing active checks/i);
    expect(failure).toHaveAttribute('role', 'alert');
    const createButton = screen.getByRole('button', { name: 'Create active check' });
    expect(createButton).toBeDisabled();

    await fireEvent.click(screen.getByRole('button', { name: 'Retry existing-check scan' }));
    await waitFor(() => expect(createButton).toBeEnabled());
  });

  it('announces proposal test success and failure without moving focus', async () => {
    let resolveTest!: (result: {
      success: true;
      latencyMillis: number;
      application: { outcome: 'passed'; statusCode: number };
    }) => void;
    vi.mocked(AvailabilityTargetsAPI.test).mockImplementationOnce(
      () => new Promise((resolve) => (resolveTest = resolve)),
    );

    render(() => (
      <AvailabilityProposalCard
        discovery={discovery()}
        resourceType="app-container"
        targetId="agent-1"
        resourceId="grafana"
        canonicalResourceId="resource:grafana"
        onDiscoveryUpdated={vi.fn()}
      />
    ));

    const testButton = screen.getByRole('button', { name: 'Test proposal' });
    testButton.focus();
    await fireEvent.click(testButton);
    testButton.blur();
    resolveTest({
      success: true,
      latencyMillis: 18,
      application: { outcome: 'passed', statusCode: 200 },
    });
    const success = await screen.findByText(/application response passed with HTTP 200/i);
    expect(success).toHaveAttribute('role', 'status');
    expect(document.activeElement).toBe(testButton);

    vi.mocked(AvailabilityTargetsAPI.test).mockResolvedValueOnce({
      success: false,
      latencyMillis: 0,
      error: 'Connection refused',
    });
    await fireEvent.click(testButton);
    const failure = await screen.findByText('Connection refused');
    expect(failure).toHaveAttribute('role', 'alert');
    expect(document.activeElement).toBe(testButton);
  });

  it('does not claim machine suggestions are empty while they are still loading', async () => {
    let resolveDiscoveries!: (result: { discoveries: []; total: 0 }) => void;
    vi.mocked(listDiscoveriesByAgent).mockImplementation(
      () => new Promise((resolve) => (resolveDiscoveries = resolve)),
    );

    render(() => (
      <AvailabilityProposalCard
        discovery={discovery()}
        resourceType="app-container"
        targetId="agent-1"
        resourceId="grafana"
        canonicalResourceId="resource:grafana"
        onDiscoveryUpdated={vi.fn()}
      />
    ));

    await fireEvent.click(screen.getByRole('button', { name: 'Review machine suggestions' }));
    expect(screen.getByText('Loading discovered services…')).toHaveAttribute('role', 'status');
    expect(screen.queryByText(/no availability suggestions are currently available/i)).toBeNull();

    resolveDiscoveries({ discoveries: [], total: 0 });
    expect(
      await screen.findByText(/no availability suggestions are currently available/i),
    ).toBeInTheDocument();
  });

  it('does not leak a proposal action failure into machine review', async () => {
    vi.mocked(AvailabilityTargetsAPI.test).mockRejectedValueOnce(new Error('Probe test failed'));

    render(() => (
      <AvailabilityProposalCard
        discovery={discovery()}
        resourceType="app-container"
        targetId="agent-1"
        resourceId="grafana"
        canonicalResourceId="resource:grafana"
        onDiscoveryUpdated={vi.fn()}
      />
    ));

    await fireEvent.click(screen.getByRole('button', { name: 'Test proposal' }));
    expect(await screen.findByText('Probe test failed')).toHaveAttribute('role', 'alert');

    await fireEvent.click(screen.getByRole('button', { name: 'Review machine suggestions' }));
    expect(within(screen.getByRole('dialog')).queryByText('Probe test failed')).toBeNull();
  });

  it('reports machine suggestion load failures instead of showing an empty state', async () => {
    vi.mocked(listDiscoveriesByAgent).mockRejectedValue(new Error('discovery unavailable'));

    render(() => (
      <AvailabilityProposalCard
        discovery={discovery()}
        resourceType="app-container"
        targetId="agent-1"
        resourceId="grafana"
        canonicalResourceId="resource:grafana"
        onDiscoveryUpdated={vi.fn()}
      />
    ));

    await fireEvent.click(screen.getByRole('button', { name: 'Review machine suggestions' }));
    const failure = await screen.findByText(/could not load this machine’s assurance suggestions/i);
    expect(failure).toHaveAttribute('role', 'alert');
    expect(screen.queryByText(/no availability suggestions are currently available/i)).toBeNull();
  });
});
