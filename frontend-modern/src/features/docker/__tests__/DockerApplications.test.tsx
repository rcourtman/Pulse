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

  it('classifies stale Compose applications under Attention, renders Stale data, and warns in detail view', () => {
    window.history.replaceState({}, '', '/docker/applications');
    const staleHost: Resource = {
      ...makeHost('host-stale', 'ops-stale-01'),
      status: 'warning',
      health: { verdict: 'stale', reasons: [{ code: 'telemetry_stale', detail: '18m' }] },
      sourceStatus: { docker: { status: 'stale', lastSeen: '2026-11-27T10:00:00Z' } },
    };
    const freshHost = makeHost('host-fresh', 'ops-fresh-01');
    const hosts = [staleHost, freshHost];
    const containers = [
      makeContainer({
        id: 'c-stale-ok',
        name: 'billing-web',
        status: 'online',
        docker: {
          hostSourceId: 'host-stale',
          hostname: 'ops-stale-01',
          containerId: 'c-stale-ok',
          containerState: 'running',
          labels: {
            'com.docker.compose.project': 'billing-stack',
            'com.docker.compose.service': 'web',
          },
        },
      }),
      makeContainer({
        id: 'c-stale-bad',
        name: 'jobs-worker',
        status: 'degraded',
        docker: {
          hostSourceId: 'host-stale',
          hostname: 'ops-stale-01',
          containerId: 'c-stale-bad',
          containerState: 'restarting',
          labels: {
            'com.docker.compose.project': 'jobs-stack',
            'com.docker.compose.service': 'worker',
          },
        },
      }),
      makeContainer({
        id: 'c-fresh-ok',
        name: 'proxy-web',
        status: 'online',
        docker: {
          hostSourceId: 'host-fresh',
          hostname: 'ops-fresh-01',
          containerId: 'c-fresh-ok',
          containerState: 'running',
          labels: {
            'com.docker.compose.project': 'edge-proxy',
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

    // Both the failing stale app and the otherwise-healthy stale app sit under Attention.
    expect(screen.getByRole('button', { name: /^Attention,\s*2$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Healthy,\s*1$/i })).toBeInTheDocument();

    const billingRow = document.querySelector('[data-docker-application-row="billing-stack"]')!;
    expect(billingRow.querySelector('[data-docker-application-stale-pill]')).toHaveTextContent(
      'Stale data',
    );
    expect(billingRow.textContent).not.toContain('Healthy');

    // A stale app that also has a failing container keeps its attention pill and shows Stale data.
    const jobsRow = document.querySelector('[data-docker-application-row="jobs-stack"]')!;
    expect(jobsRow.querySelector('[data-docker-application-attention-pill]')).toHaveTextContent(
      '1 needs attention',
    );
    expect(jobsRow.querySelector('[data-docker-stale="application"]')).toHaveTextContent(
      'Stale data',
    );

    // Filtering by Healthy shows only the fresh app.
    fireEvent.click(screen.getByRole('button', { name: /^Healthy,\s*1$/i }));
    expect(screen.getByText('edge-proxy')).toBeInTheDocument();
    expect(screen.queryByText('billing-stack')).toBeNull();
    expect(screen.queryByText('jobs-stack')).toBeNull();

    cleanup();
    window.history.replaceState(
      {},
      '',
      '/docker/applications?runtime=host-stale&project=billing-stack',
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

    const notice = screen.getByTestId('docker-application-stale-notice');
    expect(notice).toHaveTextContent('Stale data.');
    expect(notice).toHaveTextContent('No Docker report for 18m');
  });
});
