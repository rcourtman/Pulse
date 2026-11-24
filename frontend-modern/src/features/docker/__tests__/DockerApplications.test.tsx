import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { DockerApplications } from '../DockerApplications';
import { DockerHostIdentitiesProvider, buildDockerHostIdentities } from '../dockerHostIdentity';

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {} }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    detectionEnabled: () => true,
    getMetricThresholds: () => ({ warning: 80, critical: 85 }),
  }),
}));

const makeHost = (id: string, name: string): Resource => ({
  id,
  name,
  displayName: name,
  platformId: id,
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  type: 'agent',
  lastSeen: 1_700_000_000_000,
  docker: { hostname: name, hostSourceId: id },
});

const makeContainer = (overrides: Partial<Resource> & Pick<Resource, 'id' | 'name'>): Resource => ({
  displayName: overrides.name,
  platformId: 'host-1',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  type: 'oci-container',
  lastSeen: 1_700_000_000_000,
  docker: {
    hostSourceId: 'host-1',
    hostname: 'ops-services-01',
    containerId: overrides.id,
    labels: {
      'com.docker.compose.project': 'document-ingest',
      'com.docker.compose.service': 'worker',
    },
  },
  ...overrides,
});

afterEach(() => {
  cleanup();
});

describe('DockerApplications', () => {
  it('renders a searchable, filterable Compose applications table with status dots and attention pills', () => {
    const hosts = [makeHost('host-1', 'ops-services-01')];
    const containers = [
      makeContainer({
        id: 'c-1',
        name: 'ingest-worker',
        status: 'degraded',
        docker: {
          hostSourceId: 'host-1',
          hostname: 'ops-services-01',
          containerId: 'c-1',
          labels: {
            'com.docker.compose.project': 'document-ingest',
            'com.docker.compose.service': 'worker',
          },
        },
      }),
      makeContainer({
        id: 'c-2',
        name: 'proxy-web',
        status: 'online',
        docker: {
          hostSourceId: 'host-1',
          hostname: 'ops-services-01',
          containerId: 'c-2',
          labels: {
            'com.docker.compose.project': 'edge-proxy-01',
            'com.docker.compose.service': 'proxy',
          },
        },
      }),
    ];

    render(() => (
      <DockerHostIdentitiesProvider value={() => buildDockerHostIdentities(hosts)}>
        <Router>
          <Route
            path="*"
            component={() => (
              <DockerApplications hosts={hosts} containers={containers} refresh={vi.fn()} />
            )}
          />
        </Router>
      </DockerHostIdentitiesProvider>
    ));

    expect(screen.getByText('Compose applications')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Search Compose applications or hosts')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Attention,\s*1$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Healthy,\s*1$/i })).toBeInTheDocument();
    expect(screen.getByText('1 needs attention')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /^Attention,\s*1$/i }));
    expect(screen.getByText('document-ingest')).toBeInTheDocument();
    expect(screen.queryByText('edge-proxy-01')).toBeNull();
  });

  it('highlights failing HTTP service checks in the application detail view', () => {
    const hosts = [makeHost('host-1', 'ops-services-01')];
    const containers = [
      makeContainer({
        id: 'c-1',
        name: 'ingest-api',
        status: 'degraded',
        availabilityChecks: [
          {
            targetId: 'chk-1',
            name: 'Ingest health',
            protocol: 'http',
            address: 'http://ingest.local/health',
            enabled: true,
            available: false,
            applicationOutcome: 'failed',
            applicationStatusCode: 503,
            correlationState: 'attached',
            lastChecked: new Date(Date.now() - 14_000).toISOString(),
          },
        ],
      }),
    ];

    window.history.replaceState(
      {},
      '',
      '/docker/applications?runtime=host-1&project=document-ingest',
    );
    render(() => (
      <DockerHostIdentitiesProvider value={() => buildDockerHostIdentities(hosts)}>
        <Router>
          <Route
            path="*"
            component={() => (
              <DockerApplications hosts={hosts} containers={containers} refresh={vi.fn()} />
            )}
          />
        </Router>
      </DockerHostIdentitiesProvider>
    ));

    const result = document.querySelector('[data-endpoint-check-result]');
    expect(result).not.toBeNull();
    expect(result?.className).toContain('bg-red-100');
  });
});
