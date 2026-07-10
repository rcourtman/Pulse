import { describe, expect, it } from 'vitest';

import type { Resource, ResourceRelationship } from '@/types/resource';
import {
  buildDockerNetworkAttachmentRows,
  dockerContainerPortLabel,
  dockerContainerPortsSummary,
  dockerResourceSearchHaystack,
  filterDockerResources,
  mapDockerContainerStatus,
} from '../dockerPageModel';

const makeResource = (
  resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>,
): Resource => ({
  ...resource,
  name: resource.name ?? resource.id,
  displayName: resource.displayName ?? resource.id,
  platformId: resource.platformId ?? 'lab',
  platformType: resource.platformType ?? 'docker',
  sourceType: resource.sourceType ?? 'agent',
  status: resource.status ?? 'online',
  lastSeen: resource.lastSeen ?? 1_700_000_000_000,
});

const rel = (
  sourceId: string,
  targetId: string,
  metadata?: Record<string, unknown>,
): ResourceRelationship => ({
  sourceId,
  targetId,
  type: 'attached_to',
  confidence: 1,
  active: true,
  discoverer: 'docker_adapter',
  observedAt: '2026-01-01T00:00:00Z',
  lastSeenAt: '2026-01-01T00:00:00Z',
  ...(metadata ? { metadata } : {}),
});

// ---------------------------------------------------------------------------
// TARGET 1: container-state ranking fallbacks (unknown / empty state)
// ---------------------------------------------------------------------------

describe('mapDockerContainerStatus — state fallbacks', () => {
  it('returns Unknown when no container state or status is present', () => {
    expect(
      mapDockerContainerStatus(
        makeResource({ id: 'c-empty', type: 'app-container', status: '' as unknown as Resource['status'], docker: {} }),
      ),
    ).toEqual({ variant: 'muted', label: 'Unknown' });
  });

  it('returns Unknown when docker is entirely absent and status is empty', () => {
    expect(
      mapDockerContainerStatus(
        makeResource({ id: 'c-no-docker', type: 'app-container', status: '' as unknown as Resource['status'] }),
      ),
    ).toEqual({ variant: 'muted', label: 'Unknown' });
  });

  it('title-cases unrecognized container states as muted', () => {
    expect(
      mapDockerContainerStatus(
        makeResource({ id: 'c-unknown', type: 'app-container', docker: { containerState: 'quarantined' } }),
      ),
    ).toEqual({ variant: 'muted', label: 'Quarantined' });
  });

  it('falls back to resource.status when containerState is absent', () => {
    expect(
      mapDockerContainerStatus(
        makeResource({ id: 'c-status', type: 'app-container', status: 'restarting' as unknown as Resource['status'], docker: {} }),
      ),
    ).toEqual({ variant: 'warning', label: 'Restarting' });
  });

  it.each([
    ['created', 'Created'],
    ['paused', 'Paused'],
    ['stopped', 'Stopped'],
  ])('treats %s as a deliberately stopped state', (state, label) => {
    expect(
      mapDockerContainerStatus(
        makeResource({ id: `c-${state}`, type: 'app-container', docker: { containerState: state } }),
      ),
    ).toEqual({ variant: 'muted', label });
  });

  it('treats removing as a fatal state with a title-cased label', () => {
    expect(
      mapDockerContainerStatus(
        makeResource({ id: 'c-removing', type: 'app-container', docker: { containerState: 'removing' } }),
      ),
    ).toEqual({ variant: 'danger', label: 'Removing' });
  });
});

// ---------------------------------------------------------------------------
// TARGET 2: image/tag normalization (no-tag, registry-prefixed, digest)
// ---------------------------------------------------------------------------

describe('dockerResourceSearchHaystack — image tags and digests', () => {
  it('indexes registry-prefixed repo tags', () => {
    const image = makeResource({
      id: 'img-registry',
      type: 'docker-image',
      docker: {
        image: 'ghcr.io/acme/api:latest',
        repoTags: ['ghcr.io/acme/api:v2'],
      },
    });
    expect(dockerResourceSearchHaystack(image)).toContain('ghcr.io/acme/api:v2');
    expect(filterDockerResources([image], 'ghcr.io/acme/api:v2', 'all')).toHaveLength(1);
  });

  it('indexes repo digests with sha256 references', () => {
    const image = makeResource({
      id: 'img-digest',
      type: 'docker-image',
      docker: {
        repoDigests: ['acme/api@sha256:deadbeefcafe'],
      },
    });
    expect(dockerResourceSearchHaystack(image)).toContain('acme/api@sha256:deadbeefcafe');
    expect(filterDockerResources([image], 'sha256:deadbeefcafe', 'all')).toHaveLength(1);
  });

  it('handles images with no repoTags or repoDigests without crashing', () => {
    const image = makeResource({
      id: 'img-bare',
      type: 'docker-image',
      docker: { image: 'nginx' },
    });
    expect(filterDockerResources([image], 'nginx', 'all')).toHaveLength(1);
    // repoTags/repoDigests absent — they simply contribute nothing
    expect(dockerResourceSearchHaystack(image)).not.toContain('@sha256');
  });

  it('matches the bare image name without a tag', () => {
    const container = makeResource({
      id: 'c-bare-img',
      type: 'app-container',
      docker: { image: 'redis' },
    });
    expect(filterDockerResources([container], 'redis', 'all')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// TARGET 3: port-map dedupe (port token shapes and summary fallback)
// ---------------------------------------------------------------------------

describe('dockerContainerPortLabel — port token shapes', () => {
  it('formats a fully mapped port with host IP', () => {
    expect(
      dockerContainerPortLabel({ ip: '0.0.0.0', publicPort: 8080, privatePort: 80, protocol: 'tcp' }),
    ).toBe('0.0.0.0:8080->80/tcp');
  });

  it('formats a fully mapped port without host IP', () => {
    expect(dockerContainerPortLabel({ publicPort: 8080, privatePort: 80 })).toBe('8080->80/tcp');
  });

  it('exposes only the private port when no host binding exists', () => {
    expect(dockerContainerPortLabel({ privatePort: 443, protocol: 'tcp' })).toBe('443/tcp');
  });

  it('formats a public port with IP when no private port exists', () => {
    expect(dockerContainerPortLabel({ ip: '10.0.0.1', publicPort: 53, protocol: 'udp' })).toBe(
      '10.0.0.1:53/udp',
    );
  });

  it('formats a public port without IP when no private port exists', () => {
    expect(dockerContainerPortLabel({ publicPort: 53 })).toBe('53/tcp');
  });

  it('falls back to just the protocol when neither port is present', () => {
    expect(dockerContainerPortLabel({ protocol: 'udp' })).toBe('udp');
  });

  it('defaults to tcp when protocol is missing and no ports are set', () => {
    expect(dockerContainerPortLabel({})).toBe('tcp');
  });
});

describe('dockerContainerPortsSummary — summary and empty fallback', () => {
  it('joins multiple port labels with commas', () => {
    const resource = makeResource({
      id: 'c-multi',
      type: 'app-container',
      docker: {
        ports: [
          { ip: '0.0.0.0', publicPort: 80, privatePort: 8080, protocol: 'tcp' },
          { privatePort: 443, protocol: 'tcp' },
        ],
      },
    });
    expect(dockerContainerPortsSummary(resource)).toBe('0.0.0.0:80->8080/tcp, 443/tcp');
  });

  it('returns an em dash when the ports array is empty', () => {
    expect(
      dockerContainerPortsSummary(
        makeResource({ id: 'c-empty-ports', type: 'app-container', docker: { ports: [] } }),
      ),
    ).toBe('—');
  });

  it('returns an em dash when no ports field is present', () => {
    expect(
      dockerContainerPortsSummary(makeResource({ id: 'c-no-ports', type: 'app-container' })),
    ).toBe('—');
  });
});

// ---------------------------------------------------------------------------
// TARGET 4: volume mount source-candidate guards (missing source/name)
// ---------------------------------------------------------------------------

describe('dockerResourceSearchHaystack — volume mount guards', () => {
  it('indexes read-only mounts via rw=false', () => {
    const container = makeResource({
      id: 'c-ro',
      type: 'app-container',
      docker: {
        mounts: [{ type: 'bind', source: '/host/config', destination: '/config', mode: 'Z', rw: false }],
      },
    });
    expect(dockerResourceSearchHaystack(container)).toContain('read-only');
    expect(filterDockerResources([container], 'read-only', 'all')).toHaveLength(1);
  });

  it('indexes read-write mounts via rw=true', () => {
    const container = makeResource({
      id: 'c-rw',
      type: 'app-container',
      docker: {
        mounts: [{ type: 'bind', source: '/host/data', destination: '/data', rw: true }],
      },
    });
    expect(dockerResourceSearchHaystack(container)).toContain('read-write');
    expect(filterDockerResources([container], 'read-write', 'all')).toHaveLength(1);
  });

  it('omits read-only and read-write when rw is undefined', () => {
    const container = makeResource({
      id: 'c-rw-undef',
      type: 'app-container',
      docker: {
        mounts: [{ type: 'volume', source: '/host/data', destination: '/data' }],
      },
    });
    const haystack = dockerResourceSearchHaystack(container);
    expect(haystack).not.toContain('read-only');
    expect(haystack).not.toContain('read-write');
  });

  it('still indexes destination when mount source is missing', () => {
    const container = makeResource({
      id: 'c-no-src',
      type: 'app-container',
      docker: {
        mounts: [{ type: 'volume', destination: '/var/lib/data' }],
      },
    });
    expect(filterDockerResources([container], '/var/lib/data', 'all')).toHaveLength(1);
  });

  it('handles an anonymous volume mount with no source, name, or mode', () => {
    const container = makeResource({
      id: 'c-anon',
      type: 'app-container',
      docker: {
        mounts: [{ type: 'volume', destination: '/data' }],
      },
    });
    // Does not crash; resource id still indexed
    expect(filterDockerResources([container], 'c-anon', 'all')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// TARGET 5: network alias empty/missing
// ---------------------------------------------------------------------------

describe('buildDockerNetworkAttachmentRows — network alias and address fallbacks', () => {
  it('uses ipv6 address when ipv4 is absent', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'host-1' },
    });
    const container = makeResource({
      id: 'c-v6',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      docker: {
        hostSourceId: 'host-1',
        containerState: 'running',
        networks: [{ name: 'frontend', ipv6: 'fd00::42' }],
      },
    });
    const rows = buildDockerNetworkAttachmentRows(network, [network, container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].address).toBe('fd00::42');
  });

  it('falls back to relationship metadata ipv4 when no attachment address exists', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'host-1' },
    });
    const container = makeResource({
      id: 'c-rel-addr',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      relationships: [rel('c-rel-addr', 'net-1', { ipv4: '172.20.0.5' })],
      docker: { hostSourceId: 'host-1', containerState: 'running' },
    });
    const rows = buildDockerNetworkAttachmentRows(network, [network, container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].address).toBe('172.20.0.5');
  });

  it('returns an em dash when no address source is available', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'host-1' },
    });
    const container = makeResource({
      id: 'c-no-addr',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      relationships: [rel('c-no-addr', 'net-1')],
      docker: { hostSourceId: 'host-1', containerState: 'running' },
    });
    const rows = buildDockerNetworkAttachmentRows(network, [network, container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].address).toBe('—');
  });

  it('uses relationship metadata networkName when attachment is absent', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'host-1' },
    });
    const container = makeResource({
      id: 'c-rel-name',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      relationships: [rel('c-rel-name', 'net-1', { networkName: 'legacy-frontend' })],
      docker: { hostSourceId: 'host-1', containerState: 'running' },
    });
    const rows = buildDockerNetworkAttachmentRows(network, [network, container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].networkName).toBe('legacy-frontend');
  });

  it('falls back to the network display name when neither attachment nor relationship names exist', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'host-1' },
    });
    const container = makeResource({
      id: 'c-fb-name',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      relationships: [rel('c-fb-name', 'net-1')],
      docker: { hostSourceId: 'host-1', containerState: 'running' },
    });
    const rows = buildDockerNetworkAttachmentRows(network, [network, container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].networkName).toBe('frontend');
  });

  it('excludes containers when the network has no name and only a host-scope attachment exists', () => {
    const network = makeResource({
      id: 'net-2',
      type: 'docker-network',
      name: '',
      displayName: '',
      docker: { hostSourceId: 'host-1' },
    });
    const container = makeResource({
      id: 'c-excluded',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      docker: {
        hostSourceId: 'host-1',
        containerState: 'running',
        networks: [{ name: '', ipv4: '10.0.0.1' }],
      },
    });
    // findContainerNetworkAttachment returns undefined when networkName is empty,
    // and no relationship exists, so the container is excluded.
    expect(buildDockerNetworkAttachmentRows(network, [network, container])).toEqual([]);
  });

  it('matches by hostname when hostSourceId is present on only one side', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostname: 'edge-01' },
    });
    const container = makeResource({
      id: 'c-hostname',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      docker: {
        hostSourceId: 'host-1',
        hostname: 'edge-01',
        containerState: 'running',
        networks: [{ name: 'frontend', ipv4: '10.0.0.1' }],
      },
    });
    const rows = buildDockerNetworkAttachmentRows(network, [network, container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].name).toBe('api');
  });

  it('excludes containers when hostnames differ and hostSourceId is only on one side', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostname: 'edge-01' },
    });
    const container = makeResource({
      id: 'c-hostname-mismatch',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      docker: {
        hostSourceId: 'host-2',
        hostname: 'edge-02',
        containerState: 'running',
        networks: [{ name: 'frontend', ipv4: '10.0.0.1' }],
      },
    });
    expect(buildDockerNetworkAttachmentRows(network, [network, container])).toEqual([]);
  });

  it('uses em dash for image when container has no image set', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'host-1' },
    });
    const container = makeResource({
      id: 'c-no-img',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      docker: {
        hostSourceId: 'host-1',
        containerState: 'running',
        networks: [{ name: 'frontend', ipv4: '10.0.0.1' }],
      },
    });
    const rows = buildDockerNetworkAttachmentRows(network, [network, container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].image).toBe('—');
  });
});

describe('dockerResourceSearchHaystack — network aliases', () => {
  it('indexes network attachment names and addresses', () => {
    const container = makeResource({
      id: 'c-net',
      type: 'app-container',
      docker: {
        networks: [{ name: 'bridge', ipv4: '172.17.0.2', ipv6: 'fd00::3' }],
      },
    });
    const haystack = dockerResourceSearchHaystack(container);
    expect(haystack).toContain('bridge');
    expect(haystack).toContain('172.17.0.2');
    expect(haystack).toContain('fd00::3');
    expect(filterDockerResources([container], 'bridge', 'all')).toHaveLength(1);
  });

  it('handles an undefined networks array without crashing', () => {
    const container = makeResource({
      id: 'c-no-nets',
      type: 'app-container',
      docker: { containerState: 'running' },
    });
    expect(filterDockerResources([container], 'c-no-nets', 'all')).toHaveLength(1);
  });
});
