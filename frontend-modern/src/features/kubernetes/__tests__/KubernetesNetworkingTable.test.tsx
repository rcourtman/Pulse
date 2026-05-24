import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesNetworkingTable } from '../KubernetesNetworkingTable';

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

describe('KubernetesNetworkingTable', () => {
  it('renders Service, Ingress, and EndpointSlice fields from the Kubernetes networking APIs', () => {
    render(() => (
      <KubernetesNetworkingTable
        resources={[
          makeResource({
            id: 'checkout-api',
            type: 'k8s-service',
            kubernetes: {
              clusterId: 'cluster-1',
              namespace: 'services',
              resourceKind: 'Service',
              serviceType: 'ClusterIP',
              clusterIp: '10.96.18.24',
              servicePorts: [{ name: 'http', protocol: 'TCP', port: 8080, targetPort: '8080' }],
              selector: { app: 'checkout-api' },
            },
          }),
          makeResource({
            id: 'checkout-web',
            type: 'k8s-ingress',
            kubernetes: {
              clusterId: 'cluster-1',
              namespace: 'apps',
              resourceKind: 'Ingress',
              className: 'nginx',
              hosts: ['shop.example.com'],
              ingressRuleCount: 2,
            },
          }),
          makeResource({
            id: 'checkout-api-abc12',
            type: 'k8s-endpoint-slice',
            kubernetes: {
              clusterId: 'cluster-1',
              namespace: 'services',
              resourceKind: 'EndpointSlice',
              addressType: 'IPv4',
              serviceName: 'checkout-api',
              endpointCount: 3,
              readyEndpointCount: 3,
              endpointPorts: [{ name: 'http', protocol: 'TCP', port: 8080 }],
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No networking"
        emptyDescription="No networking"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Type / class')).toBeInTheDocument();
    expect(screen.getByText('Address / hosts')).toBeInTheDocument();
    expect(screen.getByText('Targets')).toBeInTheDocument();
    expect(screen.getByText('ClusterIP')).toBeInTheDocument();
    expect(screen.getByText('10.96.18.24')).toBeInTheDocument();
    expect(screen.getAllByText('8080:8080/tcp')).toHaveLength(1);
    expect(screen.getByText('app=checkout-api')).toBeInTheDocument();
    expect(screen.getByText('nginx')).toBeInTheDocument();
    expect(screen.getByText('shop.example.com')).toBeInTheDocument();
    expect(screen.getByText('2 rules')).toBeInTheDocument();
    expect(screen.getByText('IPv4')).toBeInTheDocument();
    expect(screen.getAllByText('3/3 ready')).toHaveLength(1);
    expect(screen.getByText('checkout-api · 3/3 ready')).toBeInTheDocument();
  });
});
