import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
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
    expect(screen.getByText(/nothing is created until/i)).toBeInTheDocument();

    await fireEvent.change(screen.getByLabelText('Observation location · you control'), {
      target: { value: 'edge-1' },
    });
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
    expect(screen.getByText(/created and attached to this resource/i)).toBeInTheDocument();
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
      expect(screen.getByText(/already covered by “Existing Grafana”/i)).toBeInTheDocument(),
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
});
