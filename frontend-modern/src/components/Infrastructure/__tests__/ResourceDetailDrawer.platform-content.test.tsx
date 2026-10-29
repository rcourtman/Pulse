import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, within } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

// The estate the drawer reads an image's consumers from.
const estate: Resource[] = [];

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    activeAlerts: {},
    state: { pmg: [] as unknown[], resources: estate },
    connected: () => true,
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: vi.fn(),
  }),
  useDarkMode: () => () => false,
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getFacetBundle: vi.fn().mockResolvedValue({
      capabilities: [],
      relationships: [],
      recentChanges: [],
    }),
  },
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getResourceIntelligence: vi.fn().mockResolvedValue({
      resource_id: 'resource-1',
      health: {
        score: 100,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Stable',
      },
      recent_changes: [],
      dependencies: [],
      dependents: [],
      correlations: [],
      note_count: 0,
    }),
  },
}));

const baseResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: 'resource-1',
    type: 'vm',
    name: 'resource-1',
    displayName: 'resource-1',
    platformId: 'platform-1',
    platformType: 'docker',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    ...overrides,
  }) as Resource;

const facts = (getByTestId: (id: string) => HTMLElement) =>
  getByTestId('resource-drawer-facts').textContent ?? '';

// Problems in platform sections reach "Worth checking", an object's own content
// stays in view, and the facts line carries what a phone row hides.
describe('ResourceDetailDrawer platform content', () => {
  afterEach(() => {
    estate.length = 0;
  });

  it('describes a Docker image and lifts an update its consumers report', () => {
    estate.push(
      baseResource({
        id: 'container-web',
        type: 'app-container',
        name: 'web',
        docker: {
          image: 'nginx:1.27',
          updateStatus: { updateAvailable: true, currentDigest: 'sha256:aaa' },
        },
      }),
    );
    const image = baseResource({
      id: 'image-nginx',
      type: 'docker-image',
      name: 'nginx:1.27',
      docker: {
        hostname: 'docker-1',
        image: 'nginx:1.27',
        imageId: 'sha256:0123456789abcdef',
        repoTags: ['nginx:1.27', 'nginx:stable'],
        repoDigests: ['nginx@sha256:fedcba9876543210'],
        sizeBytes: 190 * 1024 ** 2,
        imageContainers: 1,
        labels: { maintainer: 'NGINX Docker Maintainers' },
      },
    });

    const { getByTestId } = render(() => <ResourceDetailDrawer resource={image} />);

    const lifted = within(getByTestId('resource-drawer-exceptions'));
    expect(lifted.getByText('Update check')).toBeInTheDocument();
    expect(lifted.getByText('Update available')).toBeInTheDocument();
    expect(facts(getByTestId)).toContain('Used by web');
    expect(facts(getByTestId)).toContain('Host docker-1');

    const section = within(getByTestId('resource-docker-image-section'));
    expect(section.getByText('1 container: web')).toBeInTheDocument();
    expect(section.getByText('nginx:stable')).toBeInTheDocument();
    expect(section.getByText('sha256:0123456789abcdef')).toBeInTheDocument();
    expect(section.getByText('nginx@sha256:fedcba9876543210')).toBeInTheDocument();
    expect(section.getByText('190 MB')).toBeInTheDocument();
  });

  it('flags an image in use that has never been checked, but not an unused one', () => {
    const image = (imageContainers: number) =>
      baseResource({
        id: `image-${imageContainers}`,
        type: 'docker-image',
        name: 'local/app:dev',
        docker: { image: 'local/app:dev', imageContainers },
      });

    const used = render(() => <ResourceDetailDrawer resource={image(2)} />);
    expect(
      within(used.getByTestId('resource-drawer-exceptions')).getByText('Not checked'),
    ).toBeInTheDocument();
    used.unmount();

    const unused = render(() => <ResourceDetailDrawer resource={image(0)} />);
    expect(unused.queryByTestId('resource-drawer-exceptions')).toBeNull();
    expect(facts(unused.getByTestId)).toContain('Unused');
  });

  it.each(['image', 'consumer'])(
    'shows the %s check failure as ordinary wrapping text',
    (source) => {
      const error = 'authentication required\n<em>registry.example.invalid</em>';
      const updateStatus = { updateAvailable: false, error };
      const image = baseResource({
        id: 'image-failed',
        type: 'docker-image',
        name: 'registry.example.invalid/team/web:latest',
        docker: {
          hostname: 'edge-01',
          image: 'registry.example.invalid/team/web:latest',
          ...(source === 'image' ? { updateStatus } : {}),
        },
      });
      if (source === 'consumer') {
        estate.push(
          baseResource({
            id: 'consumer-failed',
            type: 'app-container',
            name: 'web',
            docker: { image: image.docker!.image, updateStatus },
          }),
        );
      }
      const { getByTestId } = render(() => <ResourceDetailDrawer resource={image} />);
      const lifted = getByTestId('resource-drawer-exceptions');
      const reason = within(lifted).getByText(error, { exact: true, normalizer: (value) => value });
      expect(within(lifted).getByText('Check failed')).toBeInTheDocument();
      expect(reason).toHaveClass('select-text', 'whitespace-pre-wrap');
      expect(reason.parentElement).toHaveClass('wrap-anywhere');
      expect(reason.closest('details')).toBeNull();
      expect(reason.querySelector('em')).toBeNull();
      expect(reason.textContent).toBe(error);
    },
  );

  it.each([
    [
      'Current',
      { updateAvailable: false },
      'No newer digest was reported by the last image check.',
    ],
    ['Not checked', undefined, 'No update comparison has been reported for this image.'],
    [
      'Pinned',
      { updateAvailable: false, error: 'digest-pinned image' },
      'The image is pinned to a specific digest, so there is no tag to check against.',
    ],
    [
      'Local',
      { updateAvailable: false, error: 'local image' },
      'The image was built or loaded on this host, so no registry has a newer version to check.',
    ],
  ] as const)(
    'keeps %s distinct and gives its explanation in Details',
    (label, updateStatus, reason) => {
      const image = baseResource({
        id: 'image-state',
        type: 'docker-image',
        name: 'service:latest',
        docker: { image: 'service:latest', updateStatus },
      });
      const { getByTestId, queryByTestId } = render(() => (
        <ResourceDetailDrawer resource={image} />
      ));
      const section = within(getByTestId('resource-docker-image-section'));
      expect(section.getByText(label)).toBeInTheDocument();
      expect(section.getByText(reason)).toBeInTheDocument();
      expect(queryByTestId('resource-drawer-exceptions')).toBeNull();
    },
  );

  it('describes a Docker volume with its mountpoint in full', () => {
    const volume = baseResource({
      id: 'volume-data',
      type: 'docker-volume',
      name: 'app_data',
      docker: {
        hostname: 'docker-1',
        volumeName: 'app_data',
        driver: 'local',
        scope: 'local',
        mountpoint: '/var/lib/docker/volumes/app_data/_data',
        sizeBytes: 2 * 1024 ** 3,
        refCount: 2,
        labels: { 'com.docker.compose.project': 'app' },
      },
    });

    const { getByTestId } = render(() => <ResourceDetailDrawer resource={volume} />);

    expect(facts(getByTestId)).toContain('local driver, local scope');
    expect(facts(getByTestId)).toContain('Used by 2 containers');
    const section = within(getByTestId('resource-docker-volume-section'));
    const mountpoint = section.getByText('/var/lib/docker/volumes/app_data/_data');
    expect(mountpoint.className).toContain('whitespace-normal');
    expect(section.getByText('2 containers')).toBeInTheDocument();
    expect(section.getByText(/com\.docker\.compose\.project/)).toBeInTheDocument();
  });

  it("shows a pod's containers in front and lifts the one that is not ready", () => {
    const pod = baseResource({
      id: 'pod-web',
      type: 'pod',
      name: 'web-7d9-abc',
      platformType: 'kubernetes',
      kubernetes: {
        namespace: 'apps',
        ownerKind: 'ReplicaSet',
        ownerName: 'web-7d9',
        nodeName: 'worker-2',
        podContainers: [
          { name: 'api', ready: true, state: 'running', image: 'ghcr.io/acme/api:1.2.3' },
          {
            name: 'sidecar',
            ready: false,
            state: 'waiting',
            reason: 'CrashLoopBackOff',
            restartCount: 7,
          },
        ],
      },
    });

    const { getByTestId } = render(() => <ResourceDetailDrawer resource={pod} />);

    const cards = getByTestId('resource-drawer-cards');
    expect(cards.closest('[data-testid="resource-technical-details"]')).toBeNull();
    expect(within(cards).getByText('Containers (2)')).toBeInTheDocument();
    expect(within(cards).getByText(/ghcr\.io\/acme\/api:1\.2\.3/)).toBeInTheDocument();
    expect(
      within(getByTestId('resource-drawer-exceptions')).getByText('sidecar'),
    ).toBeInTheDocument();
    expect(facts(getByTestId)).toContain('Namespace apps');
    expect(facts(getByTestId)).toContain('ReplicaSet web-7d9');
    expect(facts(getByTestId)).toContain('Node worker-2');
  });

  it('lifts a cordoned node and a Deployment short of its target', () => {
    const node = render(() => (
      <ResourceDetailDrawer
        resource={baseResource({
          id: 'node-1',
          type: 'k8s-node',
          platformType: 'kubernetes',
          kubernetes: { osImage: 'Ubuntu 24.04 LTS', unschedulable: true },
        })}
      />
    ));
    expect(
      within(node.getByTestId('resource-drawer-exceptions')).getByText('Cordoned (unschedulable)'),
    ).toBeInTheDocument();
    node.unmount();

    const deployment = render(() => (
      <ResourceDetailDrawer
        resource={baseResource({
          id: 'deploy-web',
          type: 'k8s-deployment',
          platformType: 'kubernetes',
          kubernetes: { desiredReplicas: 3, readyReplicas: 1, availableReplicas: 1 },
        })}
      />
    ));
    const lifted = within(deployment.getByTestId('resource-drawer-exceptions'));
    expect(lifted.getAllByText('1 of 3')).toHaveLength(2);
    expect(lifted.getByText('0 of 3')).toBeInTheDocument();
  });

  it('lifts an inaccessible vSphere datastore out of Details', () => {
    const { getByTestId } = render(() => (
      <ResourceDetailDrawer
        resource={baseResource({
          id: 'datastore-1',
          type: 'storage',
          platformType: 'vmware-vsphere',
          vmware: {
            entityType: 'datastore',
            datastoreType: 'VMFS',
            datastoreAccessible: false,
            maintenanceMode: 'normal',
          },
        })}
      />
    ));
    const lifted = within(getByTestId('resource-drawer-exceptions'));
    expect(lifted.getByText('Accessible')).toBeInTheDocument();
    expect(lifted.queryByText('Maintenance')).toBeNull();
    expect(within(getByTestId('resource-technical-details')).getByText('VMFS')).toBeInTheDocument();
  });
});
