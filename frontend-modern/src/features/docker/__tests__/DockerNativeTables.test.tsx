import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { DockerContainersTable } from '../DockerContainersTable';
import { DockerConfigsTable } from '../DockerConfigsTable';
import { DockerImagesTable } from '../DockerImagesTable';
import { DockerNetworksTable } from '../DockerNetworksTable';
import { DockerSecretsTable } from '../DockerSecretsTable';
import { DockerSwarmNodesTable } from '../DockerSwarmNodesTable';
import { DockerTasksTable } from '../DockerTasksTable';
import { DockerVolumesTable } from '../DockerVolumesTable';

const makeResource = ({
  id,
  type,
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'docker-1',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  status: 'online',
  type,
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

afterEach(() => {
  cleanup();
});

describe('Docker native tables', () => {
  it('renders Docker container API fields', () => {
    render(() => (
      <DockerContainersTable
        resources={[
          makeResource({
            id: 'container-1',
            type: 'app-container',
            name: 'edge-web',
            status: 'running',
            docker: {
              hostname: 'edge-01',
              runtime: 'docker',
              runtimeVersion: '27.5.1',
              image: 'nginx:latest',
              containerState: 'running',
              health: 'healthy',
              restartCount: 2,
              ports: [{ ip: '0.0.0.0', publicPort: 8080, privatePort: 80, protocol: 'tcp' }],
              networks: [{ name: 'frontend', ipv4: '172.18.0.2' }],
              mounts: [
                {
                  type: 'volume',
                  source: 'nginx-html',
                  destination: '/usr/share/nginx/html',
                  mode: 'rw',
                  rw: true,
                },
              ],
              updateStatus: {
                updateAvailable: true,
                currentDigest: 'sha256:current',
                latestDigest: 'sha256:latest',
                lastChecked: '2026-05-24T13:00:00Z',
              },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No containers"
        emptyDescription="No containers"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Container')).toBeInTheDocument();
    expect(screen.getByText('Runtime')).toBeInTheDocument();
    expect(screen.getByText('Restarts')).toBeInTheDocument();
    expect(screen.getByText('Updates')).toBeInTheDocument();
    expect(screen.getByText('edge-web')).toBeInTheDocument();
    expect(screen.getByText('edge-01')).toBeInTheDocument();
    expect(screen.getByText('docker 27.5.1')).toBeInTheDocument();
    expect(screen.getByText('nginx:latest')).toBeInTheDocument();
    expect(screen.getByText('healthy')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('0.0.0.0:8080->80/tcp')).toBeInTheDocument();
    expect(screen.getByText('frontend 172.18.0.2')).toBeInTheDocument();
    expect(screen.getByText('volume:/usr/share/nginx/html (rw)')).toBeInTheDocument();
    expect(screen.getByText('Available')).toBeInTheDocument();
  });

  it('renders Docker image API fields', () => {
    render(() => (
      <DockerImagesTable
        resources={[
          makeResource({
            id: 'image-1',
            type: 'docker-image',
            name: 'nginx:latest',
            docker: {
              hostname: 'edge-01',
              repoTags: ['nginx:latest', 'nginx:stable'],
              repoDigests: ['nginx@sha256:manifest'],
              sizeBytes: 805306368,
              imageContainers: 2,
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No images"
        emptyDescription="No images"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Tags')).toBeInTheDocument();
    expect(screen.getByText('Digests')).toBeInTheDocument();
    expect(screen.getByText('nginx:latest')).toBeInTheDocument();
    expect(screen.getByText('nginx:latest, nginx:stable')).toBeInTheDocument();
    expect(screen.getByText('nginx@sha256:manifest')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('edge-01')).toBeInTheDocument();
  });

  it('renders Docker volume API fields', () => {
    render(() => (
      <DockerVolumesTable
        resources={[
          makeResource({
            id: 'volume-1',
            type: 'docker-volume',
            name: 'app-data',
            docker: {
              driver: 'local',
              scope: 'global',
              sizeBytes: 2048,
              refCount: 3,
              createdAt: '2026-05-24T13:00:00Z',
              mountpoint: '/var/lib/docker/volumes/app-data/_data',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No volumes"
        emptyDescription="No volumes"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Created')).toBeInTheDocument();
    expect(screen.getByText('Refs')).toBeInTheDocument();
    expect(screen.getByText('app-data')).toBeInTheDocument();
    expect(screen.getByText('local')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('2026-05-24T13:00:00Z')).toBeInTheDocument();
    expect(screen.getByText('/var/lib/docker/volumes/app-data/_data')).toBeInTheDocument();
  });

  it('renders Docker network API fields', () => {
    render(() => (
      <DockerNetworksTable
        resources={[
          makeResource({
            id: 'network-1',
            type: 'docker-network',
            name: 'frontend',
            docker: {
              hostname: 'edge-01',
              driver: 'overlay',
              scope: 'swarm',
              enableIpv4: true,
              attachable: true,
              ingress: true,
              subnets: [{ subnet: '10.88.0.0/24', gateway: '10.88.0.1' }],
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No networks"
        emptyDescription="No networks"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Addressing')).toBeInTheDocument();
    expect(screen.getByText('Flags')).toBeInTheDocument();
    expect(screen.getByText('frontend')).toBeInTheDocument();
    expect(screen.getByText('overlay')).toBeInTheDocument();
    expect(screen.getByText('IPv4')).toBeInTheDocument();
    expect(screen.getByText('attachable, ingress')).toBeInTheDocument();
    expect(screen.getByText('10.88.0.0/24 via 10.88.0.1')).toBeInTheDocument();
  });

  it('renders Docker Swarm node API fields', () => {
    render(() => (
      <DockerSwarmNodesTable
        resources={[
          makeResource({
            id: 'node-1',
            type: 'docker-swarm-node',
            name: 'worker-1',
            docker: {
              nodeRole: 'manager',
              availability: 'active',
              managerReachability: 'reachable',
              leader: true,
              engineVersion: '26.1.4',
              nanoCpus: 4_000_000_000,
              memoryBytes: 17179869184,
              address: '10.0.0.11',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No nodes"
        emptyDescription="No nodes"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Reachability')).toBeInTheDocument();
    expect(screen.getByText('worker-1')).toBeInTheDocument();
    expect(screen.getByText('manager')).toBeInTheDocument();
    expect(screen.getByText('leader')).toBeInTheDocument();
    expect(screen.getByText('26.1.4')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('10.0.0.11')).toBeInTheDocument();
  });

  it('renders Docker Swarm task API fields', () => {
    render(() => (
      <DockerTasksTable
        resources={[
          makeResource({
            id: 'task-1',
            type: 'docker-task',
            name: 'web.2',
            docker: {
              serviceName: 'web',
              slot: 2,
              desiredState: 'running',
              currentState: 'running 2 minutes',
              nodeName: 'worker-1',
              startedAt: '2026-05-24T13:05:00Z',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No tasks"
        emptyDescription="No tasks"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Slot')).toBeInTheDocument();
    expect(screen.getByText('Desired')).toBeInTheDocument();
    expect(screen.getByText('Current')).toBeInTheDocument();
    expect(screen.getByText('web.2')).toBeInTheDocument();
    expect(screen.getByText('web')).toBeInTheDocument();
    expect(screen.getByText('running 2 minutes')).toBeInTheDocument();
    expect(screen.getByText('2026-05-24T13:05:00Z')).toBeInTheDocument();
  });

  it('renders Docker Swarm secret API metadata without secret data', () => {
    render(() => (
      <DockerSecretsTable
        resources={[
          makeResource({
            id: 'secret-1',
            type: 'docker-secret',
            name: 'api-token',
            docker: {
              hostname: 'manager-1',
              driver: 'vault',
              templatingDriver: 'golang',
              objectCreatedAt: '2026-05-24T13:10:00Z',
              labels: { stack: 'ops' },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No secrets"
        emptyDescription="No secrets"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Secret')).toBeInTheDocument();
    expect(screen.getByText('Template')).toBeInTheDocument();
    expect(screen.getByText('api-token')).toBeInTheDocument();
    expect(screen.getByText('vault')).toBeInTheDocument();
    expect(screen.getByText('golang')).toBeInTheDocument();
    expect(screen.getByText('stack=ops')).toBeInTheDocument();
    expect(screen.getByText('manager-1')).toBeInTheDocument();
  });

  it('renders Docker Swarm config API metadata', () => {
    render(() => (
      <DockerConfigsTable
        resources={[
          makeResource({
            id: 'config-1',
            type: 'docker-config',
            name: 'nginx-conf',
            docker: {
              hostname: 'manager-1',
              templatingDriver: 'golang',
              objectCreatedAt: '2026-05-24T13:15:00Z',
              labels: { stack: 'edge' },
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No configs"
        emptyDescription="No configs"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Config')).toBeInTheDocument();
    expect(screen.getByText('Template')).toBeInTheDocument();
    expect(screen.getByText('nginx-conf')).toBeInTheDocument();
    expect(screen.getByText('golang')).toBeInTheDocument();
    expect(screen.getByText('stack=edge')).toBeInTheDocument();
    expect(screen.getByText('manager-1')).toBeInTheDocument();
  });
});
