import { describe, expect, it } from 'vitest';

import type { DockerStorageUsageMeta, Resource } from '@/types/resource';
import {
  buildDockerIncidentRows,
  buildDockerNetworkAttachmentRows,
  dockerResourceSearchHaystack,
  filterDockerIncidents,
  filterDockerResources,
  hasDockerStorageUsageBucket,
  hasDockerSwarmEvidence,
  mapDockerServiceStatus,
  mapDockerSwarmNodeStatus,
  mapDockerTaskStatus,
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

// ---------------------------------------------------------------------------
// dockerResourceSearchHaystack — uncovered field branches
// ---------------------------------------------------------------------------

describe('dockerResourceSearchHaystack — docker labels, podman, and swarm fields', () => {
  it('indexes docker label keys and values', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'c-labels',
        type: 'app-container',
        docker: {
          labels: { 'com.docker.compose.project': 'orion', 'traefik.enable': 'true' },
        },
      }),
    );
    expect(haystack).toContain('com.docker.compose.project');
    expect(haystack).toContain('orion');
    expect(haystack).toContain('traefik.enable');
    expect(haystack).toContain('true');
  });

  it('indexes podman fields and prefixed tokens', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'c-podman',
        type: 'app-container',
        docker: {
          podman: {
            podName: 'edge-pod',
            podId: 'pod-abc',
            composeProject: 'orion',
            composeService: 'web',
            autoUpdatePolicy: 'registry',
            userNamespace: 'keep-id',
          },
        },
      }),
    );
    expect(haystack).toContain('edge-pod');
    expect(haystack).toContain('pod-abc');
    expect(haystack).toContain('registry');
    expect(haystack).toContain('keep-id');
    expect(haystack).toContain('pod:edge-pod');
    expect(haystack).toContain('pod:pod-abc');
    expect(haystack).toContain('compose:orion');
    expect(haystack).toContain('compose:web');
  });

  it('indexes swarm metadata fields', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'host-swarm',
        type: 'agent',
        docker: {
          swarm: {
            clusterId: 'cluster-123',
            clusterName: 'prod-swarm',
            nodeRole: 'manager',
            localState: 'active',
          },
        },
      }),
    );
    expect(haystack).toContain('cluster-123');
    expect(haystack).toContain('prod-swarm');
    expect(haystack).toContain('manager');
    expect(haystack).toContain('active');
  });

  it('indexes updateStatus error and digests', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'c-update',
        type: 'app-container',
        docker: {
          updateStatus: {
            error: 'pull rate limit exceeded',
            currentDigest: 'sha256:aaa',
            latestDigest: 'sha256:bbb',
          },
        },
      }),
    );
    expect(haystack).toContain('pull rate limit exceeded');
    expect(haystack).toContain('sha256:aaa');
    expect(haystack).toContain('sha256:bbb');
  });

  it('indexes numeric tokens restartCount and exitCode', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'c-nums',
        type: 'app-container',
        docker: { restartCount: 7, exitCode: 137 },
      }),
    );
    expect(haystack).toContain('7');
    expect(haystack).toContain('137');
  });

  it('indexes canonical identity fields and aliases', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'c-canonical',
        type: 'app-container',
        canonicalIdentity: {
          displayName: 'Canonical Name',
          hostname: 'canonical-host',
          primaryId: 'primary-123',
          aliases: ['alias-1', 'alias-2'],
        },
      }),
    );
    expect(haystack).toContain('canonical name');
    expect(haystack).toContain('canonical-host');
    expect(haystack).toContain('primary-123');
    expect(haystack).toContain('alias-1');
    expect(haystack).toContain('alias-2');
  });

  it('indexes agent and identity hostname', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'c-host',
        type: 'app-container',
        agent: { hostname: 'agent-host' },
        identity: { hostname: 'identity-host' },
      }),
    );
    expect(haystack).toContain('agent-host');
    expect(haystack).toContain('identity-host');
  });

  it('indexes volume, network, service, task, node, secret, and config fields', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'r-fields',
        type: 'docker-volume',
        docker: {
          volumeName: 'data-vol',
          networkId: 'net-abc',
          driver: 'local',
          mountpoint: '/mnt/data',
          serviceName: 'api-svc',
          taskId: 'task-1',
          nodeId: 'node-1',
          nodeName: 'node-edge',
          nodeRole: 'worker',
          availability: 'drain',
          address: '10.0.0.5',
          managerReachability: 'reachable',
          managerAddress: '10.0.0.1:2377',
          engineVersion: '27.5.1',
          secretName: 'db-password',
          configName: 'nginx-conf',
          mode: 'replicated',
          currentState: 'running',
          desiredState: 'running',
          message: 'task running',
          error: 'connection refused',
        },
      }),
    );
    expect(haystack).toContain('data-vol');
    expect(haystack).toContain('net-abc');
    expect(haystack).toContain('local');
    expect(haystack).toContain('/mnt/data');
    expect(haystack).toContain('api-svc');
    expect(haystack).toContain('task-1');
    expect(haystack).toContain('node-1');
    expect(haystack).toContain('node-edge');
    expect(haystack).toContain('worker');
    expect(haystack).toContain('drain');
    expect(haystack).toContain('10.0.0.5');
    expect(haystack).toContain('reachable');
    expect(haystack).toContain('10.0.0.1:2377');
    expect(haystack).toContain('27.5.1');
    expect(haystack).toContain('db-password');
    expect(haystack).toContain('nginx-conf');
    expect(haystack).toContain('replicated');
    expect(haystack).toContain('connection refused');
  });

  it('indexes runtime, versions, and container identity fields', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'c-runtime',
        type: 'app-container',
        docker: {
          runtime: 'docker',
          runtimeVersion: '27.5.1',
          dockerVersion: '27.5.1',
          containerId: 'abc123',
          image: 'nginx:latest',
          imageId: 'sha256:def',
          containerState: 'running',
          health: 'healthy',
          hostname: 'docker-host',
          displayName: 'Docker Display',
        },
      }),
    );
    expect(haystack).toContain('docker');
    expect(haystack).toContain('27.5.1');
    expect(haystack).toContain('abc123');
    expect(haystack).toContain('nginx:latest');
    expect(haystack).toContain('sha256:def');
    expect(haystack).toContain('docker display');
  });

  it('indexes resource tags', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'c-tags',
        type: 'app-container',
        tags: ['env:prod', 'team:platform'],
      }),
    );
    expect(haystack).toContain('env:prod');
    expect(haystack).toContain('team:platform');
  });

  it('indexes parentName and platformType', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({
        id: 'c-parent',
        type: 'app-container',
        parentName: 'parent-host',
        platformType: 'docker',
      }),
    );
    expect(haystack).toContain('parent-host');
    expect(haystack).toContain('docker');
  });
});

describe('dockerResourceSearchHaystack — empty and minimal inputs', () => {
  it('returns a non-empty haystack from resource identity even when docker is absent', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({ id: 'c-minimal', type: 'app-container' }),
    );
    expect(haystack).toContain('c-minimal');
    expect(haystack.length).toBeGreaterThan(0);
  });

  it('handles an empty docker object', () => {
    const haystack = dockerResourceSearchHaystack(
      makeResource({ id: 'c-empty-docker', type: 'app-container', docker: {} }),
    );
    expect(haystack).toContain('c-empty-docker');
  });
});

// ---------------------------------------------------------------------------
// hasDockerStorageUsageBucket
// ---------------------------------------------------------------------------

describe('hasDockerStorageUsageBucket', () => {
  it('returns false for undefined', () => {
    expect(hasDockerStorageUsageBucket(undefined)).toBe(false);
  });

  it('returns false for null', () => {
    expect(hasDockerStorageUsageBucket(null as unknown as DockerStorageUsageMeta)).toBe(false);
  });

  it('returns false when all fields are zero or absent', () => {
    expect(hasDockerStorageUsageBucket({ totalCount: 0, activeCount: 0 })).toBe(false);
    expect(hasDockerStorageUsageBucket({})).toBe(false);
  });

  it('returns true when totalCount is positive', () => {
    expect(hasDockerStorageUsageBucket({ totalCount: 5 })).toBe(true);
  });

  it('returns true when activeCount is positive', () => {
    expect(hasDockerStorageUsageBucket({ activeCount: 3 })).toBe(true);
  });

  it('returns true when totalSizeBytes is positive', () => {
    expect(hasDockerStorageUsageBucket({ totalSizeBytes: 1024 })).toBe(true);
  });

  it('returns true when reclaimableBytes is positive', () => {
    expect(hasDockerStorageUsageBucket({ reclaimableBytes: 512 })).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// mapDockerSwarmNodeStatus — uncovered branches
// ---------------------------------------------------------------------------

describe('mapDockerSwarmNodeStatus — down, degraded, ready, and unknown branches', () => {
  it.each(['offline', 'stopped', 'failed'])(
    'returns danger Down for status %s',
    (status) => {
      expect(
        mapDockerSwarmNodeStatus(
          makeResource({
            id: `n-${status}`,
            type: 'docker-swarm-node',
            status: status as Resource['status'],
          }),
        ),
      ).toEqual({ variant: 'danger', label: 'Down' });
    },
  );

  it.each(['degraded', 'warning'])(
    'returns warning Degraded for status %s',
    (status) => {
      expect(
        mapDockerSwarmNodeStatus(
          makeResource({
            id: `n-${status}`,
            type: 'docker-swarm-node',
            status: status as Resource['status'],
          }),
        ),
      ).toEqual({ variant: 'warning', label: 'Degraded' });
    },
  );

  it('returns success Ready for an active non-leader running node', () => {
    expect(
      mapDockerSwarmNodeStatus(
        makeResource({
          id: 'n-ready',
          type: 'docker-swarm-node',
          status: 'running',
          docker: { availability: 'active' },
        }),
      ),
    ).toEqual({ variant: 'success', label: 'Ready' });
  });

  it('returns muted Unknown when no condition matches', () => {
    expect(
      mapDockerSwarmNodeStatus(
        makeResource({
          id: 'n-unknown',
          type: 'docker-swarm-node',
          status: 'unknown',
          docker: {},
        }),
      ),
    ).toEqual({ variant: 'muted', label: 'Unknown' });
  });
});

// ---------------------------------------------------------------------------
// mapDockerServiceStatus — rollbackstarted branch
// ---------------------------------------------------------------------------

describe('mapDockerServiceStatus — rollback started branch', () => {
  it('flags rollbackstarted as warning Rollback paused', () => {
    expect(
      mapDockerServiceStatus(
        makeResource({
          id: 'svc-rb',
          type: 'docker-service',
          docker: {
            desiredTasks: 3,
            runningTasks: 3,
            serviceUpdate: { state: 'rollbackstarted' },
          },
        }),
      ),
    ).toEqual({ variant: 'warning', label: 'Rollback paused' });
  });
});

// ---------------------------------------------------------------------------
// mapDockerTaskStatus — uncovered state branches
// ---------------------------------------------------------------------------

describe('mapDockerTaskStatus — uncovered state branches', () => {
  it.each([
    ['rejected', 'Rejected'],
    ['orphaned', 'Orphaned'],
  ])('returns danger for %s current state', (state, label) => {
    expect(
      mapDockerTaskStatus(
        makeResource({ id: `t-${state}`, type: 'docker-task', docker: { currentState: state } }),
      ),
    ).toEqual({ variant: 'danger', label });
  });

  it('returns success for complete', () => {
    expect(
      mapDockerTaskStatus(
        makeResource({ id: 't-complete', type: 'docker-task', docker: { currentState: 'complete' } }),
      ),
    ).toEqual({ variant: 'success', label: 'Complete' });
  });

  it.each([
    ['starting', 'Starting'],
    ['pending', 'Pending'],
    ['assigned', 'Assigned'],
    ['accepted', 'Accepted'],
    ['ready', 'Ready'],
  ])('returns warning for %s current state', (state, label) => {
    expect(
      mapDockerTaskStatus(
        makeResource({ id: `t-${state}`, type: 'docker-task', docker: { currentState: state } }),
      ),
    ).toEqual({ variant: 'warning', label });
  });

  it('returns muted Unknown when no current state is set', () => {
    expect(
      mapDockerTaskStatus(makeResource({ id: 't-empty', type: 'docker-task', docker: {} })),
    ).toEqual({ variant: 'muted', label: 'Unknown' });
  });

  it('returns muted title-cased label for an unrecognized state', () => {
    expect(
      mapDockerTaskStatus(
        makeResource({ id: 't-weird', type: 'docker-task', docker: { currentState: 'weird' } }),
      ),
    ).toEqual({ variant: 'muted', label: 'Weird' });
  });

  it('title-cases shutdown when the desired state does not match', () => {
    expect(
      mapDockerTaskStatus(
        makeResource({
          id: 't-shutdown-mismatch',
          type: 'docker-task',
          docker: { currentState: 'shutdown', desiredState: 'running' },
        }),
      ),
    ).toEqual({ variant: 'muted', label: 'Shutdown' });
  });
});

// ---------------------------------------------------------------------------
// hasDockerSwarmEvidence — uncovered branches
// ---------------------------------------------------------------------------

describe('hasDockerSwarmEvidence — inactive and active evidence branches', () => {
  it('returns false when docker.swarm is absent', () => {
    expect(
      hasDockerSwarmEvidence(makeResource({ id: 'host-1', type: 'agent', docker: {} })),
    ).toBe(false);
  });

  it('returns false when docker is absent entirely', () => {
    expect(
      hasDockerSwarmEvidence(makeResource({ id: 'host-1', type: 'agent', docker: undefined })),
    ).toBe(false);
  });

  it('returns true for inactive swarm with controlAvailable', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { localState: 'inactive', controlAvailable: true } },
        }),
      ),
    ).toBe(true);
  });

  it('returns true for inactive swarm with clusterId', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { localState: 'inactive', clusterId: 'cluster-1' } },
        }),
      ),
    ).toBe(true);
  });

  it('returns true for inactive swarm with clusterName', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { localState: 'inactive', clusterName: 'prod' } },
        }),
      ),
    ).toBe(true);
  });

  it('returns true for inactive swarm with error', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { localState: 'inactive', error: 'node left swarm' } },
        }),
      ),
    ).toBe(true);
  });

  it('returns false for inactive swarm with only empty fields', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { localState: 'inactive' } },
        }),
      ),
    ).toBe(false);
  });

  it('returns true for active swarm with nodeId', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { localState: 'active', nodeId: 'node-1' } },
        }),
      ),
    ).toBe(true);
  });

  it('returns true for swarm with localState and no other evidence', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { localState: 'active' } },
        }),
      ),
    ).toBe(true);
  });

  it('returns true for swarm with controlAvailable and no localState', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { controlAvailable: true } },
        }),
      ),
    ).toBe(true);
  });

  it('returns true for swarm with clusterId and no localState', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { clusterId: 'c-1' } },
        }),
      ),
    ).toBe(true);
  });

  it('returns true for swarm with clusterName and no localState', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { clusterName: 'prod' } },
        }),
      ),
    ).toBe(true);
  });

  it('returns true for swarm with error and no localState', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { error: 'something went wrong' } },
        }),
      ),
    ).toBe(true);
  });

  it('returns false for swarm with only whitespace fields', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'host-1',
          type: 'agent',
          docker: { swarm: { nodeId: '  ', localState: '  ', clusterId: '' } },
        }),
      ),
    ).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// dockerDisplayName (internal — tested via buildDockerNetworkAttachmentRows)
// ---------------------------------------------------------------------------

describe('dockerDisplayName fallback chain (via buildDockerNetworkAttachmentRows)', () => {
  const net = () =>
    makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'h1' },
    });

  it('uses docker.serviceName when displayName and name are empty', () => {
    const container = makeResource({
      id: 'c-svc',
      type: 'app-container',
      name: '',
      displayName: '',
      status: 'running',
      docker: {
        hostSourceId: 'h1',
        containerState: 'running',
        serviceName: 'api-svc',
        networks: [{ name: 'frontend' }],
      },
    });
    const rows = buildDockerNetworkAttachmentRows(net(), [net(), container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].name).toBe('api-svc');
  });

  it('uses docker.nodeName when displayName, name, and serviceName are empty', () => {
    const container = makeResource({
      id: 'c-node',
      type: 'app-container',
      name: '',
      displayName: '',
      status: 'running',
      docker: {
        hostSourceId: 'h1',
        containerState: 'running',
        nodeName: 'swarm-node-1',
        networks: [{ name: 'frontend' }],
      },
    });
    const rows = buildDockerNetworkAttachmentRows(net(), [net(), container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].name).toBe('swarm-node-1');
  });

  it('falls back to resource.id when all display fields are empty', () => {
    const container = makeResource({
      id: 'c-id-only',
      type: 'app-container',
      name: '',
      displayName: '',
      status: 'running',
      docker: {
        hostSourceId: 'h1',
        containerState: 'running',
        networks: [{ name: 'frontend' }],
      },
    });
    const rows = buildDockerNetworkAttachmentRows(net(), [net(), container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].name).toBe('c-id-only');
  });
});

// ---------------------------------------------------------------------------
// dockerIncidentResourceDisplayName (internal — via buildDockerIncidentRows)
// ---------------------------------------------------------------------------

describe('dockerIncidentResourceDisplayName fallback chain (via buildDockerIncidentRows)', () => {
  it('uses docker.serviceName when displayName and name are empty', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        name: '',
        displayName: '',
        docker: { serviceName: 'api-svc' },
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].resourceName).toBe('api-svc');
  });

  it('uses docker.hostname when displayName, name, and serviceName are empty', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        name: '',
        displayName: '',
        docker: { hostname: 'docker-host-1' },
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].resourceName).toBe('docker-host-1');
  });

  it('falls back to resource.id when all display fields are empty', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        name: '',
        displayName: '',
        docker: {},
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].resourceName).toBe('r1');
  });
});

// ---------------------------------------------------------------------------
// dockerIncidentLabel (internal — tested via buildDockerIncidentRows)
// ---------------------------------------------------------------------------

describe('dockerIncidentLabel (via buildDockerIncidentRows)', () => {
  it('uses resource.incidentLabel when present', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentLabel: 'Custom Alert Label',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows[0].label).toBe('Custom Alert Label');
  });

  it('title-cases the docker_ code prefix when no incidentLabel', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: 'docker_host_down', severity: 'warning', summary: '' }],
      }),
    ]);
    expect(rows[0].label).toBe('Host Down');
  });

  it('title-cases a non-docker code without stripping', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: 'container_crash_loop', severity: 'warning', summary: '' }],
      }),
    ]);
    expect(rows[0].label).toBe('Container Crash Loop');
  });

  it('falls back to Docker Alert when no code or label is set', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: '', severity: 'warning', summary: 'something happened' }],
      }),
    ]);
    expect(rows[0].label).toBe('Docker Alert');
  });
});

// ---------------------------------------------------------------------------
// hasIncidentSignal (internal — tested via buildDockerIncidentRows)
// ---------------------------------------------------------------------------

describe('hasIncidentSignal (via buildDockerIncidentRows)', () => {
  it('includes incidents that have only a code', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: '' }],
      }),
    ]);
    expect(rows).toHaveLength(1);
  });

  it('includes incidents that have only a summary', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: '', severity: 'warning', summary: 'something happened' }],
      }),
    ]);
    expect(rows).toHaveLength(1);
  });

  it('filters out incidents with neither code nor summary', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [
          { code: '', severity: 'warning', summary: '' },
          { code: 'real_alert', severity: 'critical', summary: 'real' },
        ],
      }),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].code).toBe('real_alert');
  });
});

// ---------------------------------------------------------------------------
// buildDockerIncidentRow fallback chains (internal — via buildDockerIncidentRows)
// ---------------------------------------------------------------------------

describe('buildDockerIncidentRow fallback chains (via buildDockerIncidentRows)', () => {
  it('falls back to resource.incidentSeverity when incident.severity is empty', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentSeverity: 'critical',
        incidents: [{ code: 'docker_alert', severity: '', summary: 'x' }],
      }),
    ]);
    expect(rows[0].severity).toBe('critical');
    expect(rows[0].severityBucket).toBe('critical');
  });

  it('defaults severity to info when neither incident nor resource severity is set', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: 'docker_alert', severity: '', summary: 'x' }],
      }),
    ]);
    expect(rows[0].severity).toBe('info');
    expect(rows[0].severityBucket).toBe('info');
  });

  it('falls back to resource.incidentCode when incident.code is empty', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentCode: 'resource_code',
        incidents: [{ code: '', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows[0].code).toBe('resource_code');
  });

  it('defaults code to docker_alert when neither incident nor resource code is set', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: '', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows[0].code).toBe('docker_alert');
  });

  it('falls back to resource.incidentSummary when incident.summary is empty', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentSummary: 'Resource summary',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: '' }],
      }),
    ]);
    expect(rows[0].summary).toBe('Resource summary');
  });

  it('falls back to dockerIncidentLabel when no summary is available', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: 'docker_host_down', severity: 'warning', summary: '' }],
      }),
    ]);
    expect(rows[0].summary).toBe('Host Down');
  });

  it('uses nativeId for the row key when present', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [
          { code: 'docker_alert', severity: 'warning', summary: 'x', nativeId: 'native-1' },
        ],
      }),
    ]);
    expect(rows[0].id).toBe('r1:incident:native-1:0');
  });

  it('uses code for the row key when nativeId is absent', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: 'my_code', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows[0].id).toBe('r1:incident:my_code:0');
  });

  it('uses the default code for the row key when nativeId is absent and code falls through', () => {
    // When incident.code and resource.incidentCode are both empty, code defaults
    // to 'docker_alert', so the rowKey is always at least that — String(index)
    // fallback is unreachable.
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: '', severity: 'warning', summary: 'something' }],
      }),
    ]);
    expect(rows[0].id).toBe('r1:incident:docker_alert:0');
  });

  it('falls back to incident.provider when source is absent', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [
          { code: 'docker_alert', severity: 'warning', summary: 'x', provider: 'swarm' },
        ],
      }),
    ]);
    expect(rows[0].source).toBe('swarm');
  });

  it('defaults source to docker when neither source nor provider is set', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows[0].source).toBe('docker');
  });

  it('uses explicit source when set on the incident', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [
          { code: 'docker_alert', severity: 'warning', summary: 'x', source: 'monitoring' },
        ],
      }),
    ]);
    expect(rows[0].source).toBe('monitoring');
  });

  it('falls back to resource.incidentCategory when set', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentCategory: 'custom-category',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows[0].category).toBe('custom-category');
  });

  it('defaults category to docker-health', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows[0].category).toBe('docker-health');
  });

  it('falls back to resource.incidentAction when set', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentAction: 'Check the logs',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows[0].action).toBe('Check the logs');
  });

  it('defaults action to the standard Pulse alerts prompt', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows[0].action).toBe('Investigate in Pulse alerts');
  });

  it('uses resource.incidentPriority when set', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentPriority: 42,
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(rows[0].priority).toBe(42);
  });

  it('derives priority from severity rank when incidentPriority is absent', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [{ code: 'docker_alert', severity: 'critical', summary: 'x' }],
      }),
    ]);
    expect(rows[0].priority).toBe(3000);
  });

  it('passes through incident.startedAt', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidents: [
          {
            code: 'docker_alert',
            severity: 'warning',
            summary: 'x',
            startedAt: '2026-01-01T00:00:00Z',
          },
        ],
      }),
    ]);
    expect(rows[0].startedAt).toBe('2026-01-01T00:00:00Z');
  });
});

// ---------------------------------------------------------------------------
// buildDockerRollupIncidentRow (internal — via buildDockerIncidentRows)
// ---------------------------------------------------------------------------

describe('buildDockerRollupIncidentRow (via buildDockerIncidentRows)', () => {
  it('uses resource.incidentSummary for the rollup summary', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentCount: 3,
        incidentSeverity: 'warning',
        incidentSummary: 'Three alerts firing',
      }),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].id).toBe('r1:incident:rollup');
    expect(rows[0].summary).toBe('Three alerts firing');
  });

  it('falls back to incidentLabel when incidentSummary is absent', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentCount: 2,
        incidentLabel: 'Docker Alerts',
      }),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].summary).toBe('Docker Alerts');
  });

  it('generates a singular alert summary when count is 1', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentCount: 1,
      }),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].summary).toBe('1 active Docker alert');
  });

  it('generates a plural alert summary when count is not 1', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentCount: 5,
      }),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].summary).toBe('5 active Docker alerts');
  });

  it('produces a default singular count when incidentCount is 0 but incidentCode is set', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentCount: 0,
        incidentCode: 'docker_alert',
      }),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].summary).toBe('1 active Docker alerts');
  });

  it('defaults severity to info when incidentSeverity is absent', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentCount: 1,
      }),
    ]);
    expect(rows[0].severity).toBe('info');
  });

  it('defaults code to docker_alert when incidentCode is absent', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        incidentCount: 1,
      }),
    ]);
    expect(rows[0].code).toBe('docker_alert');
  });
});

// ---------------------------------------------------------------------------
// buildDockerIncidentRows — sorting tiebreakers
// ---------------------------------------------------------------------------

describe('buildDockerIncidentRows — sorting tiebreakers', () => {
  it('sorts by priority descending when severity is equal', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r-low-prio',
        type: 'app-container',
        name: 'zzz',
        incidents: [{ code: 'a1', severity: 'info', summary: 'x' }],
      }),
      makeResource({
        id: 'r-high-prio',
        type: 'app-container',
        name: 'aaa',
        incidentPriority: 5000,
        incidents: [{ code: 'a2', severity: 'info', summary: 'x' }],
      }),
    ]);
    expect(rows.map((r) => r.resourceId)).toEqual(['r-high-prio', 'r-low-prio']);
  });

  it('breaks ties by resourceName localeCompare when severity and priority match', () => {
    const rows = buildDockerIncidentRows([
      makeResource({
        id: 'r-z',
        type: 'app-container',
        name: 'zebra',
        displayName: 'zebra',
        incidents: [{ code: 'a1', severity: 'info', summary: 'x' }],
      }),
      makeResource({
        id: 'r-a',
        type: 'app-container',
        name: 'alpha',
        displayName: 'alpha',
        incidents: [{ code: 'a2', severity: 'info', summary: 'x' }],
      }),
    ]);
    expect(rows.map((r) => r.resourceName)).toEqual(['alpha', 'zebra']);
  });
});

// ---------------------------------------------------------------------------
// dockerIncidentSearchHaystack (internal — via filterDockerIncidents)
// ---------------------------------------------------------------------------

describe('dockerIncidentSearchHaystack (via filterDockerIncidents)', () => {
  it('matches resource.docker.swarm.clusterName', () => {
    const incidents = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'agent',
        name: 'host-1',
        docker: { swarm: { clusterName: 'prod-swarm' } },
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(
      filterDockerIncidents(incidents, 'prod-swarm', 'all').map((r) => r.resourceId),
    ).toEqual(['r1']);
  });

  it('matches resource.docker.serviceName', () => {
    const incidents = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'docker-service',
        name: 'svc-1',
        docker: { serviceName: 'payments-worker' },
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(
      filterDockerIncidents(incidents, 'payments-worker', 'all').map((r) => r.resourceId),
    ).toEqual(['r1']);
  });

  it('matches resource tags', () => {
    const incidents = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        name: 'c1',
        tags: ['env:prod'],
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(
      filterDockerIncidents(incidents, 'env:prod', 'all').map((r) => r.resourceId),
    ).toEqual(['r1']);
  });

  it('matches resource.parentName', () => {
    const incidents = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        name: 'c1',
        parentName: 'parent-host',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(
      filterDockerIncidents(incidents, 'parent-host', 'all').map((r) => r.resourceId),
    ).toEqual(['r1']);
  });

  it('matches resource.platformId', () => {
    const incidents = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        name: 'c1',
        platformId: 'lab-cluster-1',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(
      filterDockerIncidents(incidents, 'lab-cluster-1', 'all').map((r) => r.resourceId),
    ).toEqual(['r1']);
  });

  it('matches row action and category fields', () => {
    const incidents = buildDockerIncidentRows([
      makeResource({
        id: 'r1',
        type: 'app-container',
        name: 'c1',
        incidentAction: 'Run playbook',
        incidentCategory: 'disk-pressure',
        incidents: [{ code: 'docker_alert', severity: 'warning', summary: 'x' }],
      }),
    ]);
    expect(
      filterDockerIncidents(incidents, 'playbook', 'all').map((r) => r.resourceId),
    ).toEqual(['r1']);
    expect(
      filterDockerIncidents(incidents, 'disk-pressure', 'all').map((r) => r.resourceId),
    ).toEqual(['r1']);
  });
});

// ---------------------------------------------------------------------------
// mapResourceStatusToTriad (internal — via filterDockerResources)
// ---------------------------------------------------------------------------

describe('mapResourceStatusToTriad (via filterDockerResources)', () => {
  it('maps offline status to the offline triad', () => {
    const resource = makeResource({
      id: 'r-off',
      type: 'app-container',
      status: 'offline',
    });
    expect(filterDockerResources([resource], '', 'offline').map((r) => r.id)).toEqual(['r-off']);
    expect(filterDockerResources([resource], '', 'online')).toEqual([]);
  });

  it('maps stopped status to the offline triad', () => {
    const resource = makeResource({
      id: 'r-stopped',
      type: 'app-container',
      status: 'stopped',
    });
    expect(
      filterDockerResources([resource], '', 'offline').map((r) => r.id),
    ).toEqual(['r-stopped']);
  });

  it('maps unknown status to unknown (excluded from all specific filters)', () => {
    const resource = makeResource({
      id: 'r-unknown',
      type: 'app-container',
      status: 'unknown',
    });
    expect(filterDockerResources([resource], '', 'online')).toEqual([]);
    expect(filterDockerResources([resource], '', 'offline')).toEqual([]);
    expect(filterDockerResources([resource], '', 'degraded')).toEqual([]);
    expect(filterDockerResources([resource], '', 'all').map((r) => r.id)).toEqual(['r-unknown']);
  });
});

// ---------------------------------------------------------------------------
// buildDockerNetworkAttachmentRows — searchText and sorting branches
// ---------------------------------------------------------------------------

describe('buildDockerNetworkAttachmentRows — searchText and sorting', () => {
  it('includes tags in the searchText', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'h1' },
    });
    const container = makeResource({
      id: 'c-tags',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      tags: ['env:prod', 'team:platform'],
      docker: {
        hostSourceId: 'h1',
        containerState: 'running',
        networks: [{ name: 'frontend', ipv4: '10.0.0.1' }],
      },
    });
    const rows = buildDockerNetworkAttachmentRows(network, [network, container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].searchText).toContain('env:prod');
    expect(rows[0].searchText).toContain('team:platform');
  });

  it('includes container.id in the searchText', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'h1' },
    });
    const container = makeResource({
      id: 'c-unique-id',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      docker: {
        hostSourceId: 'h1',
        containerState: 'running',
        networks: [{ name: 'frontend', ipv4: '10.0.0.1' }],
      },
    });
    const rows = buildDockerNetworkAttachmentRows(network, [network, container]);
    expect(rows).toHaveLength(1);
    expect(rows[0].searchText).toContain('c-unique-id');
  });

  it('sorts rows by container status rank (danger before success)', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'h1' },
    });
    const healthy = makeResource({
      id: 'c-healthy',
      type: 'app-container',
      name: 'healthy',
      displayName: 'healthy',
      status: 'running',
      docker: {
        hostSourceId: 'h1',
        containerState: 'running',
        health: 'healthy',
        networks: [{ name: 'frontend', ipv4: '10.0.0.1' }],
      },
    });
    const dead = makeResource({
      id: 'c-dead',
      type: 'app-container',
      name: 'dead',
      displayName: 'dead',
      status: 'running',
      docker: {
        hostSourceId: 'h1',
        containerState: 'dead',
        networks: [{ name: 'frontend', ipv4: '10.0.0.2' }],
      },
    });
    const rows = buildDockerNetworkAttachmentRows(network, [network, healthy, dead]);
    expect(rows.map((r) => r.id)).toEqual(['net-1:c-dead', 'net-1:c-healthy']);
  });

  it('excludes non-container resources from the attachment rows', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'h1' },
    });
    const volume = makeResource({
      id: 'vol-1',
      type: 'docker-volume',
      name: 'data',
      docker: { hostSourceId: 'h1' },
    });
    expect(buildDockerNetworkAttachmentRows(network, [network, volume])).toEqual([]);
  });

  it('returns an empty array when no resources match the container type', () => {
    const network = makeResource({
      id: 'net-1',
      type: 'docker-network',
      name: 'frontend',
      docker: { hostSourceId: 'h1' },
    });
    expect(buildDockerNetworkAttachmentRows(network, [])).toEqual([]);
  });
});
