import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildKubernetesIncidentRows,
  compareKubernetesEvents,
  compareKubernetesPods,
  filterKubernetesResources,
  kubernetesResourceSearchHaystack,
  mapKubernetesNodeStatus,
  mapKubernetesPodStatus,
} from '../kubernetesPageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('mapKubernetesPodStatus — uncovered branches', () => {
  it('returns success for a Running pod with no containers', () => {
    expect(
      mapKubernetesPodStatus(
        makeResource({
          id: 'running-bare',
          type: 'pod',
          kubernetes: { podPhase: 'Running' },
        }),
      ),
    ).toEqual({ variant: 'success', label: 'Running' });
  });

  it('returns success for Succeeded phase', () => {
    expect(
      mapKubernetesPodStatus(
        makeResource({
          id: 'succeeded',
          type: 'pod',
          kubernetes: { podPhase: 'Succeeded' },
        }),
      ),
    ).toEqual({ variant: 'success', label: 'Succeeded' });
  });

  it('returns muted for Unknown phase', () => {
    expect(
      mapKubernetesPodStatus(
        makeResource({
          id: 'unknown-phase',
          type: 'pod',
          kubernetes: { podPhase: 'Unknown' },
        }),
      ),
    ).toEqual({ variant: 'muted', label: 'Unknown' });
  });

  it('returns muted Unknown when no phase fields are set at all', () => {
    expect(mapKubernetesPodStatus(makeResource({ id: 'bare-pod', type: 'pod' }))).toEqual({
      variant: 'muted',
      label: 'Unknown',
    });
  });

  it('capitalises an unrecognised non-empty phase via eventTypeLabel fallback', () => {
    expect(
      mapKubernetesPodStatus(
        makeResource({
          id: 'terminating',
          type: 'pod',
          kubernetes: { podPhase: 'Terminating' },
        }),
      ),
    ).toEqual({ variant: 'muted', label: 'Terminating' });
  });

  it('falls back to kubernetes.phase when podPhase is absent', () => {
    expect(
      mapKubernetesPodStatus(
        makeResource({
          id: 'phase-fallback',
          type: 'pod',
          kubernetes: { phase: 'Running' },
        }),
      ),
    ).toEqual({ variant: 'success', label: 'Running' });
  });

  it('escalates terminated non-ready containers to danger with "Container error" label when reason is absent', () => {
    expect(
      mapKubernetesPodStatus(
        makeResource({
          id: 'term-noreason',
          type: 'pod',
          kubernetes: {
            podPhase: 'Running',
            podContainers: [{ ready: false, state: 'terminated' }],
          },
        }),
      ),
    ).toEqual({ variant: 'danger', label: 'Container error' });
  });

  it('does not flag a terminated but ready container as fatal', () => {
    expect(
      mapKubernetesPodStatus(
        makeResource({
          id: 'term-ready',
          type: 'pod',
          kubernetes: {
            podPhase: 'Running',
            podContainers: [{ ready: true, state: 'terminated' }],
          },
        }),
      ),
    ).toEqual({ variant: 'success', label: 'Running' });
  });
});

describe('mapKubernetesNodeStatus — uncovered branches', () => {
  it('maps running and healthy statuses to success when ready is undefined', () => {
    expect(
      mapKubernetesNodeStatus(
        makeResource({ id: 'n-running', type: 'k8s-node', status: 'running' }),
      ),
    ).toEqual({ variant: 'success', label: 'Ready' });
    expect(
      mapKubernetesNodeStatus(
        makeResource({
          id: 'n-healthy',
          type: 'k8s-node',
          status: 'healthy' as unknown as Resource['status'],
        }),
      ),
    ).toEqual({ variant: 'success', label: 'Ready' });
  });

  it('maps stopped and failed statuses to danger when ready is undefined', () => {
    expect(
      mapKubernetesNodeStatus(
        makeResource({ id: 'n-stopped', type: 'k8s-node', status: 'stopped' }),
      ),
    ).toEqual({ variant: 'danger', label: 'NotReady' });
    expect(
      mapKubernetesNodeStatus(
        makeResource({
          id: 'n-failed',
          type: 'k8s-node',
          status: 'failed' as unknown as Resource['status'],
        }),
      ),
    ).toEqual({ variant: 'danger', label: 'NotReady' });
  });

  it('maps degraded, warning, and pending statuses to warning when ready is undefined', () => {
    expect(
      mapKubernetesNodeStatus(
        makeResource({ id: 'n-degraded', type: 'k8s-node', status: 'degraded' }),
      ),
    ).toEqual({ variant: 'warning', label: 'Degraded' });
    expect(
      mapKubernetesNodeStatus(
        makeResource({ id: 'n-warning', type: 'k8s-node', status: 'warning' }),
      ),
    ).toEqual({ variant: 'warning', label: 'Degraded' });
    expect(
      mapKubernetesNodeStatus(
        makeResource({
          id: 'n-pending',
          type: 'k8s-node',
          status: 'pending' as unknown as Resource['status'],
        }),
      ),
    ).toEqual({ variant: 'warning', label: 'Degraded' });
  });

  it('returns muted Unknown for an unrecognised status with no ready flag', () => {
    expect(
      mapKubernetesNodeStatus(
        makeResource({ id: 'n-unknown', type: 'k8s-node', status: 'unknown' }),
      ),
    ).toEqual({ variant: 'muted', label: 'Unknown' });
  });

  it('returns muted Unknown for paused status (not handled by the node mapper)', () => {
    expect(
      mapKubernetesNodeStatus(
        makeResource({ id: 'n-paused', type: 'k8s-node', status: 'paused' }),
      ),
    ).toEqual({ variant: 'muted', label: 'Unknown' });
  });
});

describe('kubernetesResourceSearchHaystack', () => {
  it('collects kubernetes-specific fields into a lowercase haystack', () => {
    const resource = makeResource({
      id: 'pod-full',
      type: 'pod',
      name: 'my-pod',
      displayName: 'My Pod',
      parentName: 'parent-dep',
      tags: ['team-payments', 'env-prod'],
      agent: { hostname: 'agent-host' },
      canonicalIdentity: {
        displayName: 'checkout-api',
        hostname: 'ci-host',
        primaryId: 'pri-1',
        aliases: ['alias-1'],
      },
      kubernetes: {
        clusterId: 'prod-cluster-id',
        clusterName: 'Production',
        namespace: 'payments',
        podName: 'checkout-abc',
        podPhase: 'Running',
        nodeName: 'worker-1',
        ownerKind: 'Deployment',
        ownerName: 'checkout',
        image: 'ghcr.io/app:v1',
        resourceKind: 'Pod',
        externalIps: ['1.2.3.4'],
        addresses: ['10.0.0.1'],
        podContainers: [
          { name: 'main', image: 'ghcr.io/app:v1', state: 'running', reason: '' },
        ],
      },
    });

    const haystack = kubernetesResourceSearchHaystack(resource);
    expect(haystack).toBe(haystack.toLowerCase());
    expect(haystack).toContain('my-pod');
    expect(haystack).toContain('production');
    expect(haystack).toContain('payments');
    expect(haystack).toContain('checkout-abc');
    expect(haystack).toContain('worker-1');
    expect(haystack).toContain('team-payments');
    expect(haystack).toContain('env-prod');
    expect(haystack).toContain('main');
    expect(haystack).toContain('1.2.3.4');
    expect(haystack).toContain('10.0.0.1');
    expect(haystack).toContain('checkout-api');
    expect(haystack).toContain('pri-1');
    expect(haystack).toContain('alias-1');
    expect(haystack).toContain('agent-host');
  });

  it('returns a minimal haystack for a bare resource', () => {
    const haystack = kubernetesResourceSearchHaystack(makeResource({ id: 'bare', type: 'pod' }));
    expect(haystack).toBe('bare bare bare lab kubernetes');
  });

  it('filters out whitespace-only and undefined values', () => {
    const haystack = kubernetesResourceSearchHaystack(
      makeResource({
        id: 'pod-ws',
        type: 'pod',
        name: '   ',
        displayName: '   ',
      }),
    );
    expect(haystack).toBe('pod-ws lab kubernetes');
  });

  it('includes podContainer name, image, state, and reason when defined', () => {
    const haystack = kubernetesResourceSearchHaystack(
      makeResource({
        id: 'pod-containers',
        type: 'pod',
        kubernetes: {
          podPhase: 'Running',
          podContainers: [
            { name: 'sidecar', image: 'busybox:latest', state: 'running', reason: '' },
            { name: 'main', image: 'app:v2', state: 'waiting', reason: 'ImagePullBackOff' },
          ],
        },
      }),
    );
    expect(haystack).toContain('sidecar');
    expect(haystack).toContain('busybox:latest');
    expect(haystack).toContain('main');
    expect(haystack).toContain('imagepullbackoff');
  });
});

describe('displayName fallback chain (via compareKubernetesPods)', () => {
  const runningReady = {
    podPhase: 'Running',
    podContainers: [{ ready: true, state: 'running' }],
  };

  it('uses resource.displayName for tie-breaking when present', () => {
    const zzz = makeResource({
      id: 'id-zzz',
      type: 'pod',
      displayName: 'zzz-name',
      kubernetes: runningReady,
    });
    const aaa = makeResource({
      id: 'id-aaa',
      type: 'pod',
      displayName: 'aaa-name',
      kubernetes: runningReady,
    });
    expect([zzz, aaa].sort(compareKubernetesPods).map((r) => r.id)).toEqual(['id-aaa', 'id-zzz']);
  });

  it('falls back to resource.name when displayName is empty', () => {
    const zzz = makeResource({
      id: 'id-zzz',
      type: 'pod',
      displayName: '',
      name: 'zzz-fallback',
      kubernetes: runningReady,
    });
    const aaa = makeResource({
      id: 'id-aaa',
      type: 'pod',
      displayName: '',
      name: 'aaa-fallback',
      kubernetes: runningReady,
    });
    expect([zzz, aaa].sort(compareKubernetesPods).map((r) => r.id)).toEqual(['id-aaa', 'id-zzz']);
  });

  it('falls back to kubernetes.podName when displayName and name are empty', () => {
    const zzz = makeResource({
      id: 'id-zzz',
      type: 'pod',
      displayName: '',
      name: '',
      kubernetes: { ...runningReady, podName: 'zzz-pod' },
    });
    const aaa = makeResource({
      id: 'id-aaa',
      type: 'pod',
      displayName: '',
      name: '',
      kubernetes: { ...runningReady, podName: 'aaa-pod' },
    });
    expect([zzz, aaa].sort(compareKubernetesPods).map((r) => r.id)).toEqual(['id-aaa', 'id-zzz']);
  });

  it('falls back to resource.id when all display fields are empty', () => {
    const zzz = makeResource({
      id: 'id-zzz',
      type: 'pod',
      displayName: '',
      name: '',
      kubernetes: runningReady,
    });
    const aaa = makeResource({
      id: 'id-aaa',
      type: 'pod',
      displayName: '',
      name: '',
      kubernetes: runningReady,
    });
    expect([zzz, aaa].sort(compareKubernetesPods).map((r) => r.id)).toEqual(['id-aaa', 'id-zzz']);
  });
});

describe('kubernetesIncidentResourceDisplayName (via buildKubernetesIncidentRows)', () => {
  const incident = { code: 'k8s_x', severity: 'warning', summary: 'sig' };

  it('falls through displayName then name then podName then id', () => {
    const fromDisplay = buildKubernetesIncidentRows([
      makeResource({ id: 'r1', type: 'pod', displayName: 'Display', incidents: [incident] }),
    ]);
    expect(fromDisplay[0].resourceName).toBe('Display');

    const fromName = buildKubernetesIncidentRows([
      makeResource({
        id: 'r2',
        type: 'pod',
        displayName: '',
        name: 'Namey',
        incidents: [incident],
      }),
    ]);
    expect(fromName[0].resourceName).toBe('Namey');

    const fromPodName = buildKubernetesIncidentRows([
      makeResource({
        id: 'r3',
        type: 'pod',
        displayName: '',
        name: '',
        kubernetes: { podName: 'pod-xyz' },
        incidents: [incident],
      }),
    ]);
    expect(fromPodName[0].resourceName).toBe('pod-xyz');

    const fromId = buildKubernetesIncidentRows([
      makeResource({ id: 'r4', type: 'pod', displayName: '', name: '', incidents: [incident] }),
    ]);
    expect(fromId[0].resourceName).toBe('r4');
  });
});

describe('kubernetesIncidentLabel (via buildKubernetesIncidentRows)', () => {
  it('uses resource.incidentLabel when present', () => {
    const rows = buildKubernetesIncidentRows([
      makeResource({
        id: 'r1',
        type: 'pod',
        incidentLabel: 'Disk Pressure Alert',
        incidents: [{ code: 'k8s_x', severity: 'warning', summary: 'sig' }],
      }),
    ]);
    expect(rows[0].label).toBe('Disk Pressure Alert');
  });

  it('title-cases incident.code (stripping k8s_ prefix) when no incidentLabel', () => {
    const withPrefix = buildKubernetesIncidentRows([
      makeResource({
        id: 'r1',
        type: 'pod',
        incidents: [{ code: 'k8s_pod_crashloop', severity: 'warning', summary: 'sig' }],
      }),
    ]);
    expect(withPrefix[0].label).toBe('Pod Crashloop');

    const withoutPrefix = buildKubernetesIncidentRows([
      makeResource({
        id: 'r2',
        type: 'pod',
        incidents: [{ code: 'custom_event', severity: 'warning', summary: 'sig' }],
      }),
    ]);
    expect(withoutPrefix[0].label).toBe('Custom Event');
  });

  it('returns "Kubernetes Alert" when neither incidentLabel nor incident.code is set', () => {
    const rows = buildKubernetesIncidentRows([
      makeResource({
        id: 'r1',
        type: 'pod',
        incidents: [{ code: '', severity: 'warning', summary: 'sig' }],
      }),
    ]);
    expect(rows[0].label).toBe('Kubernetes Alert');
  });
});

describe('buildKubernetesIncidentRow field fallbacks (via buildKubernetesIncidentRows)', () => {
  it('derives severity from incident, then resource, then defaults to info', () => {
    const fromIncident = buildKubernetesIncidentRows([
      makeResource({
        id: 'a',
        type: 'pod',
        incidents: [{ code: 'k8s_x', severity: 'critical', summary: 's' }],
      }),
    ]);
    expect(fromIncident[0].severity).toBe('critical');
    expect(fromIncident[0].severityBucket).toBe('critical');

    const fromResource = buildKubernetesIncidentRows([
      makeResource({
        id: 'b',
        type: 'pod',
        incidentSeverity: 'warning',
        incidents: [{ code: 'k8s_x', severity: '', summary: 's' }],
      }),
    ]);
    expect(fromResource[0].severity).toBe('warning');
    expect(fromResource[0].severityBucket).toBe('warning');

    const defaulted = buildKubernetesIncidentRows([
      makeResource({
        id: 'c',
        type: 'pod',
        incidents: [{ code: 'k8s_x', severity: '', summary: 's' }],
      }),
    ]);
    expect(defaulted[0].severity).toBe('info');
    expect(defaulted[0].severityBucket).toBe('info');
  });

  it('derives code from incident, then resource, then defaults to k8s_alert', () => {
    const fromResource = buildKubernetesIncidentRows([
      makeResource({
        id: 'a',
        type: 'pod',
        incidentCode: 'custom_code',
        incidents: [{ code: '', severity: 'warning', summary: 's' }],
      }),
    ]);
    expect(fromResource[0].code).toBe('custom_code');

    const defaulted = buildKubernetesIncidentRows([
      makeResource({
        id: 'b',
        type: 'pod',
        incidents: [{ code: '', severity: 'warning', summary: 's' }],
      }),
    ]);
    expect(defaulted[0].code).toBe('k8s_alert');
  });

  it('derives summary from incident, then resource.incidentSummary, then kubernetesIncidentLabel', () => {
    const fromIncident = buildKubernetesIncidentRows([
      makeResource({
        id: 'a',
        type: 'pod',
        incidents: [{ code: 'k8s_x', severity: 'warning', summary: 'direct summary' }],
      }),
    ]);
    expect(fromIncident[0].summary).toBe('direct summary');

    const fromResource = buildKubernetesIncidentRows([
      makeResource({
        id: 'b',
        type: 'pod',
        incidentSummary: 'resource summary',
        incidents: [{ code: 'k8s_x', severity: 'warning', summary: '' }],
      }),
    ]);
    expect(fromResource[0].summary).toBe('resource summary');

    const fromLabel = buildKubernetesIncidentRows([
      makeResource({
        id: 'c',
        type: 'pod',
        incidentLabel: 'Fallback Label',
        incidents: [{ code: 'k8s_x', severity: 'warning', summary: '' }],
      }),
    ]);
    expect(fromLabel[0].summary).toBe('Fallback Label');
  });

  it('uses nativeId for the row key when present, else falls to code', () => {
    const withNativeId = buildKubernetesIncidentRows([
      makeResource({
        id: 'a',
        type: 'pod',
        incidents: [
          { code: 'k8s_x', severity: 'warning', summary: 's', nativeId: 'evt-42' },
        ],
      }),
    ]);
    expect(withNativeId[0].id).toBe('a:incident:evt-42:0');

    const withoutNativeId = buildKubernetesIncidentRows([
      makeResource({
        id: 'b',
        type: 'pod',
        incidents: [{ code: 'k8s_y', severity: 'warning', summary: 's' }],
      }),
    ]);
    expect(withoutNativeId[0].id).toBe('b:incident:k8s_y:0');
  });

  it('derives source from incident.source, then incident.provider, then defaults to kubernetes', () => {
    const fromSource = buildKubernetesIncidentRows([
      makeResource({
        id: 'a',
        type: 'pod',
        incidents: [
          { code: 'k8s_x', severity: 'warning', summary: 's', source: 'prometheus' },
        ],
      }),
    ]);
    expect(fromSource[0].source).toBe('prometheus');

    const fromProvider = buildKubernetesIncidentRows([
      makeResource({
        id: 'b',
        type: 'pod',
        incidents: [
          { code: 'k8s_x', severity: 'warning', summary: 's', provider: 'datadog' },
        ],
      }),
    ]);
    expect(fromProvider[0].source).toBe('datadog');

    const defaulted = buildKubernetesIncidentRows([
      makeResource({
        id: 'c',
        type: 'pod',
        incidents: [{ code: 'k8s_x', severity: 'warning', summary: 's' }],
      }),
    ]);
    expect(defaulted[0].source).toBe('kubernetes');
  });

  it('derives category from resource.incidentCategory, defaulting to kubernetes-health', () => {
    const custom = buildKubernetesIncidentRows([
      makeResource({
        id: 'a',
        type: 'pod',
        incidentCategory: 'custom-cat',
        incidents: [{ code: 'k8s_x', severity: 'warning', summary: 's' }],
      }),
    ]);
    expect(custom[0].category).toBe('custom-cat');

    const defaulted = buildKubernetesIncidentRows([
      makeResource({
        id: 'b',
        type: 'pod',
        incidents: [{ code: 'k8s_x', severity: 'warning', summary: 's' }],
      }),
    ]);
    expect(defaulted[0].category).toBe('kubernetes-health');
  });

  it('derives action from resource.incidentAction, defaulting to the investigate prompt', () => {
    const custom = buildKubernetesIncidentRows([
      makeResource({
        id: 'a',
        type: 'pod',
        incidentAction: 'Run kubectl describe',
        incidents: [{ code: 'k8s_x', severity: 'warning', summary: 's' }],
      }),
    ]);
    expect(custom[0].action).toBe('Run kubectl describe');

    const defaulted = buildKubernetesIncidentRows([
      makeResource({
        id: 'b',
        type: 'pod',
        incidents: [{ code: 'k8s_x', severity: 'warning', summary: 's' }],
      }),
    ]);
    expect(defaulted[0].action).toBe('Investigate in Pulse alerts');
  });

  it('derives priority from resource.incidentPriority, falling back to severity rank times 1000', () => {
    const explicit = buildKubernetesIncidentRows([
      makeResource({
        id: 'a',
        type: 'pod',
        incidentPriority: 42,
        incidents: [{ code: 'k8s_x', severity: 'critical', summary: 's' }],
      }),
    ]);
    expect(explicit[0].priority).toBe(42);

    const critical = buildKubernetesIncidentRows([
      makeResource({
        id: 'b',
        type: 'pod',
        incidents: [{ code: 'k8s_x', severity: 'critical', summary: 's' }],
      }),
    ]);
    expect(critical[0].priority).toBe(3000);

    const info = buildKubernetesIncidentRows([
      makeResource({
        id: 'c',
        type: 'pod',
        incidents: [{ code: 'k8s_x', severity: 'info', summary: 's' }],
      }),
    ]);
    expect(info[0].priority).toBe(1000);
  });

  it('passes through incident.startedAt to the row', () => {
    const withStarted = buildKubernetesIncidentRows([
      makeResource({
        id: 'a',
        type: 'pod',
        incidents: [
          { code: 'k8s_x', severity: 'warning', summary: 's', startedAt: '2026-01-01T00:00:00Z' },
        ],
      }),
    ]);
    expect(withStarted[0].startedAt).toBe('2026-01-01T00:00:00Z');

    const withoutStarted = buildKubernetesIncidentRows([
      makeResource({
        id: 'b',
        type: 'pod',
        incidents: [{ code: 'k8s_x', severity: 'warning', summary: 's' }],
      }),
    ]);
    expect(withoutStarted[0].startedAt).toBeUndefined();
  });
});

describe('buildKubernetesRollupIncidentRow (via buildKubernetesIncidentRows)', () => {
  it('synthesises a rollup row with id suffix :incident:rollup', () => {
    const rows = buildKubernetesIncidentRows([
      makeResource({
        id: 'node-x',
        type: 'k8s-node',
        incidentCount: 2,
        incidentSeverity: 'critical',
      }),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].id).toBe('node-x:incident:rollup');
  });

  it('uses incidentSummary, then incidentLabel, then auto-generates a count-based summary', () => {
    const fromSummary = buildKubernetesIncidentRows([
      makeResource({
        id: 'a',
        type: 'k8s-node',
        incidentCount: 1,
        incidentSummary: 'Disk pressure',
      }),
    ]);
    expect(fromSummary[0].summary).toBe('Disk pressure');

    const fromLabel = buildKubernetesIncidentRows([
      makeResource({
        id: 'b',
        type: 'k8s-node',
        incidentCount: 1,
        incidentLabel: 'NodeNotReady',
      }),
    ]);
    expect(fromLabel[0].summary).toBe('NodeNotReady');

    const auto3 = buildKubernetesIncidentRows([
      makeResource({ id: 'c', type: 'k8s-node', incidentCount: 3 }),
    ]);
    expect(auto3[0].summary).toBe('3 active Kubernetes alerts');

    const auto1 = buildKubernetesIncidentRows([
      makeResource({ id: 'd', type: 'k8s-node', incidentCount: 1 }),
    ]);
    expect(auto1[0].summary).toBe('1 active Kubernetes alert');
  });

  it('auto-generates a pluralised summary when count is 0 but rollup signal exists', () => {
    const rows = buildKubernetesIncidentRows([
      makeResource({ id: 'e', type: 'k8s-node', incidentCode: 'k8s_thing' }),
    ]);
    expect(rows[0].summary).toBe('1 active Kubernetes alerts');
  });

  it('defaults severity to info and code to k8s_alert when resource fields are absent', () => {
    const rows = buildKubernetesIncidentRows([
      makeResource({ id: 'f', type: 'k8s-node', incidentCount: 1 }),
    ]);
    expect(rows[0].severity).toBe('info');
    expect(rows[0].severityBucket).toBe('info');
    expect(rows[0].code).toBe('k8s_alert');
  });
});

describe('parseKubernetesEventObservedTime edge cases (via compareKubernetesEvents)', () => {
  it('returns 0 for events with no timestamp, breaking ties alphabetically by id', () => {
    const a = makeResource({ id: 'event-a', type: 'k8s-event' });
    const b = makeResource({ id: 'event-b', type: 'k8s-event' });
    expect([b, a].sort(compareKubernetesEvents).map((r) => r.id)).toEqual(['event-a', 'event-b']);
  });

  it('returns 0 for malformed timestamp strings so newer valid events sort first', () => {
    const bad = makeResource({
      id: 'event-bad',
      type: 'k8s-event',
      kubernetes: { eventTime: 'not-a-date' },
    });
    const good = makeResource({
      id: 'event-good',
      type: 'k8s-event',
      kubernetes: { eventTime: '2026-01-01T00:00:00Z' },
    });
    expect([bad, good].sort(compareKubernetesEvents).map((r) => r.id)).toEqual([
      'event-good',
      'event-bad',
    ]);
  });

  it('treats whitespace-only eventTime as empty, falling through to firstSeen', () => {
    const ws = makeResource({
      id: 'ws',
      type: 'k8s-event',
      kubernetes: { eventTime: '   ', firstSeen: '2026-06-01T00:00:00Z' },
    });
    const older = makeResource({
      id: 'older',
      type: 'k8s-event',
      kubernetes: { eventTime: '2026-01-01T00:00:00Z' },
    });
    expect([older, ws].sort(compareKubernetesEvents).map((r) => r.id)).toEqual(['ws', 'older']);
  });
});

describe('mapResourceStatusToTriad (via filterKubernetesResources)', () => {
  it('maps running to online and stopped to offline', () => {
    const resources = [
      makeResource({ id: 'r-running', type: 'pod', status: 'running' }),
      makeResource({ id: 'r-stopped', type: 'pod', status: 'stopped' }),
    ];
    expect(filterKubernetesResources(resources, '', 'online').map((r) => r.id)).toEqual([
      'r-running',
    ]);
    expect(filterKubernetesResources(resources, '', 'offline').map((r) => r.id)).toEqual([
      'r-stopped',
    ]);
  });

  it('maps paused to degraded', () => {
    const resources = [makeResource({ id: 'r-paused', type: 'pod', status: 'paused' })];
    expect(filterKubernetesResources(resources, '', 'degraded').map((r) => r.id)).toEqual([
      'r-paused',
    ]);
  });

  it('maps unknown and empty statuses to the unknown triad, excluded from all specific filters', () => {
    const resources = [
      makeResource({ id: 'r-unknown', type: 'pod', status: 'unknown' }),
      makeResource({
        id: 'r-empty',
        type: 'pod',
        status: '' as unknown as Resource['status'],
      }),
    ];
    expect(filterKubernetesResources(resources, '', 'online')).toEqual([]);
    expect(filterKubernetesResources(resources, '', 'degraded')).toEqual([]);
    expect(filterKubernetesResources(resources, '', 'offline')).toEqual([]);
    expect(filterKubernetesResources(resources, '', 'all').map((r) => r.id)).toEqual([
      'r-unknown',
      'r-empty',
    ]);
  });
});
