import { describe, expect, it, vi } from 'vitest';
import { render } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab" />,
}));

const buildKubernetesResource = (): Resource => ({
  id: 'k8s-cluster-1',
  type: 'k8s-cluster',
  name: 'cluster-a',
  displayName: 'cluster-a',
  platformId: 'cluster-a',
  platformType: 'kubernetes',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  platformData: {
    sources: ['kubernetes'],
    kubernetes: {
      metricCapabilities: {
        nodeCpuMemory: true,
        nodeTelemetry: true,
        podCpuMemory: true,
        podNetwork: true,
        podEphemeralDisk: true,
        podDiskIo: false,
      },
    },
  },
});

describe('ResourceDetailDrawer kubernetes capabilities', () => {
  it('renders Kubernetes capability badges when metric capabilities are present', () => {
    const { getByText } = render(() => <ResourceDetailDrawer resource={buildKubernetesResource()} />);

    expect(getByText('K8s Node CPU/Memory')).toBeInTheDocument();
    expect(getByText('Node Telemetry (Agent)')).toBeInTheDocument();
    expect(getByText('Pod CPU/Memory')).toBeInTheDocument();
    expect(getByText('Pod Network')).toBeInTheDocument();
    expect(getByText('Pod Ephemeral Disk')).toBeInTheDocument();
    expect(getByText('Pod Disk I/O Unsupported')).toBeInTheDocument();
  });
});
