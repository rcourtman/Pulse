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
  });
});
