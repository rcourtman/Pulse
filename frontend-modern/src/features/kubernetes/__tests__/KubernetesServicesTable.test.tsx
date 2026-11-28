import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesServicesTable } from '../KubernetesServicesTable';

const makeResource = ({
  id,
  type,
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'cluster-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'online',
  type,
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

afterEach(() => {
  cleanup();
});

describe('KubernetesServicesTable', () => {
  it('renders Service fields from the Kubernetes Service API contract', () => {
    render(() => (
      <KubernetesServicesTable
        resources={[
          makeResource({
            id: 'checkout-public',
            type: 'k8s-service',
            customUrl: 'https://checkout-service.internal',
            kubernetes: {
              clusterId: 'cluster-1',
              namespace: 'apps',
              resourceKind: 'Service',
              serviceType: 'NodePort',
              clusterIp: '10.96.12.20',
              externalIps: ['203.0.113.24'],
              servicePorts: [
                {
                  name: 'https',
                  protocol: 'TCP',
                  port: 443,
                  targetPort: '8443',
                  nodePort: 30443,
                },
              ],
              selector: {
                app: 'checkout-web',
                tier: 'frontend',
              },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No services"
        emptyDescription="No services"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Service')).toBeInTheDocument();
    expect(screen.getByText('Scope')).toBeInTheDocument();
    expect(screen.getByText('Cluster IP')).toBeInTheDocument();
    expect(screen.getByText('External IPs')).toBeInTheDocument();
    expect(screen.getByText('Selector')).toBeInTheDocument();
    expect(screen.getByText('NodePort')).toBeInTheDocument();
    expect(screen.getByText('10.96.12.20')).toBeInTheDocument();
    expect(screen.getByText('203.0.113.24')).toBeInTheDocument();
    expect(screen.getByText('443:8443/tcp node:30443')).toBeInTheDocument();
    expect(screen.getByText('app=checkout-web, tier=frontend')).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Open web interface for checkout-public' }),
    ).toHaveAttribute('href', 'https://checkout-service.internal');
    expect(screen.getByText('checkout-public').closest('a')).toBeNull();
  });

  it('surfaces stale readings on services while keeping offline severity and routing fields', () => {
    const staleSource = {
      kubernetes: { status: 'stale', lastSeen: '2026-11-26T10:00:00Z' },
    };
    const { container } = render(() => (
      <KubernetesServicesTable
        resources={[
          makeResource({
            id: 'fresh-svc',
            type: 'k8s-service',
            status: 'online',
            kubernetes: {
              clusterId: 'prod',
              namespace: 'apps',
              serviceType: 'ClusterIP',
              clusterIp: '10.96.0.10',
            },
          }),
          makeResource({
            id: 'stale-svc',
            type: 'k8s-service',
            status: 'online',
            sourceStatus: staleSource,
            kubernetes: {
              clusterId: 'edge',
              namespace: 'apps',
              serviceType: 'LoadBalancer',
              clusterIp: '10.96.12.20',
            },
          }),
          makeResource({
            id: 'offline-stale-svc',
            type: 'k8s-service',
            status: 'offline',
            sourceStatus: staleSource,
            kubernetes: {
              clusterId: 'edge',
              namespace: 'apps',
              serviceType: 'ClusterIP',
              clusterIp: '10.96.12.21',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No services"
        emptyDescription="No services"
        showToolbar={false}
      />
    ));

    const freshRow = container.querySelector('[data-kubernetes-service-row="fresh-svc"]');
    expect(freshRow?.querySelector('[title="Online"]')).toHaveClass('bg-emerald-500');
    expect(freshRow?.querySelector('[data-kubernetes-stale="service"]')).toBeNull();

    const staleRow = container.querySelector('[data-kubernetes-service-row="stale-svc"]');
    expect(staleRow?.querySelector('[title="Stale data"]')).toHaveClass('bg-amber-500');
    expect(staleRow?.querySelector('[data-kubernetes-stale="service"]')).toHaveTextContent(
      'Stale data',
    );
    expect(staleRow).toHaveTextContent('10.96.12.20');

    const offlineRow = container.querySelector('[data-kubernetes-service-row="offline-stale-svc"]');
    expect(offlineRow?.querySelector('[title="Offline"]')).toHaveClass('bg-red-500');
    expect(offlineRow?.querySelector('[data-kubernetes-stale="service"]')).toHaveTextContent(
      'Stale data',
    );
  });
});
