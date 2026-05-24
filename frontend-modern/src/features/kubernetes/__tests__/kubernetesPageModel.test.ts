import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  KUBERNETES_TAB_SPECS,
  buildKubernetesPageModel,
  compareKubernetesControllers,
  compareKubernetesDeployments,
  compareKubernetesEvents,
  compareKubernetesNodes,
  compareKubernetesPods,
  mapKubernetesControllerStatus,
  mapKubernetesCronJobStatus,
  mapKubernetesDaemonSetStatus,
  mapKubernetesDeploymentStatus,
  mapKubernetesEventSeverity,
  mapKubernetesJobStatus,
  mapKubernetesNodeStatus,
  mapKubernetesPodStatus,
  mapKubernetesReplicaSetStatus,
  mapKubernetesStatefulSetStatus,
  resolveKubernetesPageTabId,
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

describe('kubernetesPageModel', () => {
  it('declares operator workflow tabs for Kubernetes inventory', () => {
    expect(KUBERNETES_TAB_SPECS.map((tab) => tab.id)).toEqual([
      'overview',
      'nodes',
      'workloads',
      'services',
      'storage',
      'configuration',
      'events',
    ]);
  });

  it('keeps legacy Kubernetes object routes mapped to workflow tabs', () => {
    expect(resolveKubernetesPageTabId(undefined)).toBe('overview');
    expect(resolveKubernetesPageTabId('nodes')).toBe('nodes');
    expect(resolveKubernetesPageTabId('pods')).toBe('workloads');
    expect(resolveKubernetesPageTabId('deployments')).toBe('workloads');
    expect(resolveKubernetesPageTabId('controllers')).toBe('workloads');
    expect(resolveKubernetesPageTabId('autoscaling')).toBe('workloads');
    expect(resolveKubernetesPageTabId('networking')).toBe('services');
    expect(resolveKubernetesPageTabId('config')).toBe('configuration');
    expect(resolveKubernetesPageTabId('policy')).toBe('configuration');
    expect(resolveKubernetesPageTabId('unknown')).toBe('overview');
  });

  it('buckets clusters, nodes, workloads, services, storage, config, policy, autoscaling, and events', () => {
    const model = buildKubernetesPageModel([
      makeResource({ id: 'cluster-1', type: 'k8s-cluster' }),
      makeResource({ id: 'node-1', type: 'k8s-node' }),
      makeResource({ id: 'pod-1', type: 'pod' }),
      makeResource({ id: 'dep-1', type: 'k8s-deployment' }),
      makeResource({ id: 'rs-1', type: 'k8s-replicaset' }),
      makeResource({ id: 'svc-1', type: 'k8s-service' }),
      makeResource({ id: 'sts-1', type: 'k8s-statefulset' }),
      makeResource({ id: 'ds-1', type: 'k8s-daemonset' }),
      makeResource({ id: 'job-1', type: 'k8s-job' }),
      makeResource({ id: 'cron-1', type: 'k8s-cronjob' }),
      makeResource({ id: 'ing-1', type: 'k8s-ingress' }),
      makeResource({ id: 'eps-1', type: 'k8s-endpoint-slice' }),
      makeResource({ id: 'netpol-1', type: 'k8s-network-policy' }),
      makeResource({ id: 'pv-1', type: 'k8s-persistent-volume' }),
      makeResource({ id: 'pvc-1', type: 'k8s-persistent-volume-claim' }),
      makeResource({ id: 'sc-1', type: 'k8s-storage-class' }),
      makeResource({ id: 'ns-1', type: 'k8s-namespace' }),
      makeResource({ id: 'cm-1', type: 'k8s-configmap' }),
      makeResource({ id: 'secret-1', type: 'k8s-secret' }),
      makeResource({ id: 'sa-1', type: 'k8s-serviceaccount' }),
      makeResource({ id: 'quota-1', type: 'k8s-resource-quota' }),
      makeResource({ id: 'limits-1', type: 'k8s-limit-range' }),
      makeResource({ id: 'pdb-1', type: 'k8s-pod-disruption-budget' }),
      makeResource({ id: 'hpa-1', type: 'k8s-horizontal-pod-autoscaler' }),
      makeResource({ id: 'event-1', type: 'k8s-event' }),
      makeResource({ id: 'proxmox-vm', type: 'vm', platformType: 'proxmox-pve' }),
    ]);

    expect(model.clusters.map((r) => r.id)).toEqual(['cluster-1']);
    expect(model.nodes.map((r) => r.id)).toEqual(['node-1']);
    expect(model.pods.map((r) => r.id)).toEqual(['pod-1']);
    expect(model.deployments.map((r) => r.id)).toEqual(['dep-1']);
    expect(model.replicaSets.map((r) => r.id)).toEqual(['rs-1']);
    expect(model.services.map((r) => r.id)).toEqual(['svc-1']);
    expect(model.statefulSets.map((r) => r.id)).toEqual(['sts-1']);
    expect(model.daemonSets.map((r) => r.id)).toEqual(['ds-1']);
    expect(model.jobs.map((r) => r.id)).toEqual(['job-1']);
    expect(model.cronJobs.map((r) => r.id)).toEqual(['cron-1']);
    expect(model.ingresses.map((r) => r.id)).toEqual(['ing-1']);
    expect(model.endpointSlices.map((r) => r.id)).toEqual(['eps-1']);
    expect(model.networkPolicies.map((r) => r.id)).toEqual(['netpol-1']);
    expect(model.persistentVolumes.map((r) => r.id)).toEqual(['pv-1']);
    expect(model.persistentVolumeClaims.map((r) => r.id)).toEqual(['pvc-1']);
    expect(model.storageClasses.map((r) => r.id)).toEqual(['sc-1']);
    expect(model.namespaces.map((r) => r.id)).toEqual(['ns-1']);
    expect(model.configMaps.map((r) => r.id)).toEqual(['cm-1']);
    expect(model.secrets.map((r) => r.id)).toEqual(['secret-1']);
    expect(model.serviceAccounts.map((r) => r.id)).toEqual(['sa-1']);
    expect(model.resourceQuotas.map((r) => r.id)).toEqual(['quota-1']);
    expect(model.limitRanges.map((r) => r.id)).toEqual(['limits-1']);
    expect(model.podDisruptionBudgets.map((r) => r.id)).toEqual(['pdb-1']);
    expect(model.horizontalPodAutoscalers.map((r) => r.id)).toEqual(['hpa-1']);
    expect(model.events.map((r) => r.id)).toEqual(['event-1']);
    expect(model.workloads.map((r) => r.id).sort()).toEqual(
      ['cron-1', 'dep-1', 'ds-1', 'job-1', 'pod-1', 'rs-1', 'sts-1'].sort(),
    );
    expect(model.storage.map((r) => r.id).sort()).toEqual(['pv-1', 'pvc-1', 'sc-1']);
    expect(model.serviceNetworking.map((r) => r.id).sort()).toEqual(['eps-1', 'ing-1']);
    expect(model.config.map((r) => r.id).sort()).toEqual(
      ['cm-1', 'ns-1', 'sa-1', 'secret-1'].sort(),
    );
    expect(model.policy.map((r) => r.id).sort()).toEqual(
      ['limits-1', 'netpol-1', 'pdb-1', 'quota-1'].sort(),
    );
    expect(model.autoscaling.map((r) => r.id)).toEqual(['hpa-1']);
    expect(model.resources).toHaveLength(25);
  });

  it('treats agent rows that report a kubernetes source as Kubernetes nodes', () => {
    const model = buildKubernetesPageModel([
      makeResource({
        id: 'merged-node-1',
        type: 'agent',
        platformType: 'kubernetes',
        sources: ['agent', 'kubernetes'],
      }),
      makeResource({
        id: 'plain-host',
        type: 'agent',
        platformType: 'agent',
        sources: ['agent'],
      }),
    ]);

    expect(model.nodes.map((r) => r.id)).toEqual(['merged-node-1']);
  });

  it('maps Kubernetes Event severity from the event type instead of generic resource status', () => {
    expect(mapKubernetesEventSeverity('Warning')).toEqual({
      variant: 'warning',
      label: 'Warning',
    });
    expect(mapKubernetesEventSeverity('Normal')).toEqual({
      variant: 'muted',
      label: 'Normal',
    });
    expect(mapKubernetesEventSeverity(undefined)).toEqual({
      variant: 'muted',
      label: 'Unknown',
    });
  });

  it('orders Kubernetes Events by observed time from newest to oldest', () => {
    const older = makeResource({
      id: 'older',
      type: 'k8s-event',
      kubernetes: {
        eventTime: '2026-05-24T11:00:00Z',
      },
    });
    const newerFromFirstSeen = makeResource({
      id: 'newer-first-seen',
      type: 'k8s-event',
      kubernetes: {
        firstSeen: '2026-05-24T13:00:00Z',
      },
    });
    const newestFromCreatedAt = makeResource({
      id: 'newest-created',
      type: 'k8s-event',
      kubernetes: {
        createdAt: '2026-05-24T14:00:00Z',
      },
    });

    expect([older, newestFromCreatedAt, newerFromFirstSeen].sort(compareKubernetesEvents)).toEqual([
      newestFromCreatedAt,
      newerFromFirstSeen,
      older,
    ]);
    expect(
      buildKubernetesPageModel([older, newestFromCreatedAt, newerFromFirstSeen]).events.map(
        (event) => event.id,
      ),
    ).toEqual(['newest-created', 'newer-first-seen', 'older']);
  });

  describe('mapKubernetesPodStatus', () => {
    it('escalates CrashLoopBackOff to danger regardless of phase', () => {
      const indicator = mapKubernetesPodStatus(
        makeResource({
          id: 'crash',
          type: 'pod',
          kubernetes: {
            podPhase: 'Running',
            podContainers: [
              { name: 'app', ready: false, state: 'waiting', reason: 'CrashLoopBackOff' },
            ],
          },
        }),
      );
      expect(indicator.variant).toBe('danger');
      expect(indicator.label).toBe('CrashLoopBackOff');
    });

    it('flags ImagePullBackOff and OOMKilled as danger', () => {
      expect(
        mapKubernetesPodStatus(
          makeResource({
            id: 'pull',
            type: 'pod',
            kubernetes: {
              podPhase: 'Pending',
              podContainers: [{ ready: false, state: 'waiting', reason: 'ImagePullBackOff' }],
            },
          }),
        ).variant,
      ).toBe('danger');
      expect(
        mapKubernetesPodStatus(
          makeResource({
            id: 'oom',
            type: 'pod',
            kubernetes: {
              podPhase: 'Running',
              podContainers: [{ ready: false, state: 'terminated', reason: 'OOMKilled' }],
            },
          }),
        ).variant,
      ).toBe('danger');
    });

    it('warns on Running pods whose containers are not all ready', () => {
      expect(
        mapKubernetesPodStatus(
          makeResource({
            id: 'not-ready',
            type: 'pod',
            kubernetes: {
              podPhase: 'Running',
              podContainers: [
                { ready: true, state: 'running' },
                { ready: false, state: 'running' },
              ],
            },
          }),
        ),
      ).toEqual({ variant: 'warning', label: 'Not ready' });
    });

    it('returns success when phase is Running and all containers ready', () => {
      expect(
        mapKubernetesPodStatus(
          makeResource({
            id: 'happy',
            type: 'pod',
            kubernetes: {
              podPhase: 'Running',
              podContainers: [{ ready: true, state: 'running' }],
            },
          }),
        ),
      ).toEqual({ variant: 'success', label: 'Running' });
    });

    it('treats Pending and Failed phases distinctly', () => {
      expect(
        mapKubernetesPodStatus(
          makeResource({
            id: 'pending',
            type: 'pod',
            kubernetes: { podPhase: 'Pending', podContainers: [] },
          }),
        ),
      ).toEqual({ variant: 'warning', label: 'Pending' });
      expect(
        mapKubernetesPodStatus(
          makeResource({
            id: 'failed',
            type: 'pod',
            kubernetes: { podPhase: 'Failed', podContainers: [] },
          }),
        ),
      ).toEqual({ variant: 'danger', label: 'Failed' });
    });
  });

  describe('mapKubernetesNodeStatus', () => {
    it('returns danger when ready is false even if status reports online', () => {
      expect(
        mapKubernetesNodeStatus(
          makeResource({
            id: 'not-ready',
            type: 'k8s-node',
            status: 'online',
            kubernetes: { ready: false },
          }),
        ),
      ).toEqual({ variant: 'danger', label: 'NotReady' });
    });

    it('returns success when ready is true', () => {
      expect(
        mapKubernetesNodeStatus(
          makeResource({
            id: 'ready',
            type: 'k8s-node',
            status: 'online',
            kubernetes: { ready: true },
          }),
        ),
      ).toEqual({ variant: 'success', label: 'Ready' });
    });

    it('falls back to resource.status when ready is undefined', () => {
      expect(
        mapKubernetesNodeStatus(makeResource({ id: 'fallback-ok', type: 'k8s-node', status: 'online' })),
      ).toEqual({ variant: 'success', label: 'Ready' });
      expect(
        mapKubernetesNodeStatus(
          makeResource({ id: 'fallback-off', type: 'k8s-node', status: 'offline' }),
        ),
      ).toEqual({ variant: 'danger', label: 'NotReady' });
    });
  });

  describe('controller status mappers', () => {
    it('classifies Deployments by ready vs desired replicas', () => {
      const fullyReady = makeResource({
        id: 'dep-ok',
        type: 'k8s-deployment',
        kubernetes: { desiredReplicas: 3, readyReplicas: 3 },
      });
      const partiallyReady = makeResource({
        id: 'dep-partial',
        type: 'k8s-deployment',
        kubernetes: { desiredReplicas: 3, readyReplicas: 1 },
      });
      const zeroReady = makeResource({
        id: 'dep-zero',
        type: 'k8s-deployment',
        kubernetes: { desiredReplicas: 3, readyReplicas: 0 },
      });
      const scaledToZero = makeResource({
        id: 'dep-scaled-zero',
        type: 'k8s-deployment',
        kubernetes: { desiredReplicas: 0, readyReplicas: 0 },
      });

      expect(mapKubernetesDeploymentStatus(fullyReady).variant).toBe('success');
      expect(mapKubernetesDeploymentStatus(partiallyReady).variant).toBe('warning');
      expect(mapKubernetesDeploymentStatus(zeroReady).variant).toBe('danger');
      expect(mapKubernetesDeploymentStatus(scaledToZero).variant).toBe('muted');
    });

    it('shares the replica indicator across ReplicaSet and StatefulSet', () => {
      const partial = {
        kubernetes: { desiredReplicas: 5, readyReplicas: 2 },
      } as const;
      expect(
        mapKubernetesReplicaSetStatus(makeResource({ id: 'rs', type: 'k8s-replicaset', ...partial }))
          .variant,
      ).toBe('warning');
      expect(
        mapKubernetesStatefulSetStatus(
          makeResource({ id: 'sts', type: 'k8s-statefulset', ...partial }),
        ).variant,
      ).toBe('warning');
    });

    it('flags misscheduled DaemonSet pods even when scheduling is otherwise complete', () => {
      const indicator = mapKubernetesDaemonSetStatus(
        makeResource({
          id: 'ds',
          type: 'k8s-daemonset',
          kubernetes: {
            desiredNumberScheduled: 3,
            numberReady: 3,
            numberMisscheduled: 1,
          },
        }),
      );
      expect(indicator).toEqual({ variant: 'warning', label: '1 misscheduled' });
    });

    it('classifies Jobs by failed/active/succeeded counts', () => {
      expect(
        mapKubernetesJobStatus(
          makeResource({ id: 'j-fail', type: 'k8s-job', kubernetes: { failed: 2 } }),
        ).variant,
      ).toBe('danger');
      expect(
        mapKubernetesJobStatus(
          makeResource({ id: 'j-active', type: 'k8s-job', kubernetes: { active: 1 } }),
        ).variant,
      ).toBe('warning');
      expect(
        mapKubernetesJobStatus(
          makeResource({ id: 'j-done', type: 'k8s-job', kubernetes: { succeeded: 1 } }),
        ).variant,
      ).toBe('success');
    });

    it('treats suspended CronJobs as muted', () => {
      expect(
        mapKubernetesCronJobStatus(
          makeResource({ id: 'cj', type: 'k8s-cronjob', kubernetes: { suspend: true } }),
        ),
      ).toEqual({ variant: 'muted', label: 'Suspended' });
      expect(
        mapKubernetesCronJobStatus(
          makeResource({ id: 'cj-on', type: 'k8s-cronjob', kubernetes: { suspend: false } }),
        ),
      ).toEqual({ variant: 'success', label: 'Scheduled' });
    });

    it('routes controllers to the correct mapper by resource type', () => {
      expect(
        mapKubernetesControllerStatus(
          makeResource({
            id: 'rs-routed',
            type: 'k8s-replicaset',
            kubernetes: { desiredReplicas: 2, readyReplicas: 0 },
          }),
        ).variant,
      ).toBe('danger');
      expect(
        mapKubernetesControllerStatus(
          makeResource({ id: 'cj-routed', type: 'k8s-cronjob', kubernetes: { suspend: true } }),
        ).variant,
      ).toBe('muted');
    });
  });

  describe('rank comparators float attention rows above healthy rows', () => {
    it('orders pods danger → warning → success', () => {
      const happy = makeResource({
        id: 'happy',
        type: 'pod',
        kubernetes: {
          podPhase: 'Running',
          podContainers: [{ ready: true, state: 'running' }],
        },
      });
      const pending = makeResource({
        id: 'pending',
        type: 'pod',
        kubernetes: { podPhase: 'Pending', podContainers: [] },
      });
      const crashing = makeResource({
        id: 'crashing',
        type: 'pod',
        kubernetes: {
          podPhase: 'Running',
          podContainers: [{ ready: false, state: 'waiting', reason: 'CrashLoopBackOff' }],
        },
      });

      expect([happy, pending, crashing].sort(compareKubernetesPods).map((r) => r.id)).toEqual([
        'crashing',
        'pending',
        'happy',
      ]);
    });

    it('floats NotReady nodes above Ready nodes', () => {
      const ready = makeResource({
        id: 'node-ready',
        type: 'k8s-node',
        kubernetes: { ready: true },
      });
      const notReady = makeResource({
        id: 'node-not-ready',
        type: 'k8s-node',
        kubernetes: { ready: false },
      });
      expect([ready, notReady].sort(compareKubernetesNodes).map((r) => r.id)).toEqual([
        'node-not-ready',
        'node-ready',
      ]);
    });

    it('orders Deployments by replica health', () => {
      const happy = makeResource({
        id: 'dep-happy',
        type: 'k8s-deployment',
        kubernetes: { desiredReplicas: 2, readyReplicas: 2 },
      });
      const partial = makeResource({
        id: 'dep-partial',
        type: 'k8s-deployment',
        kubernetes: { desiredReplicas: 2, readyReplicas: 1 },
      });
      const broken = makeResource({
        id: 'dep-broken',
        type: 'k8s-deployment',
        kubernetes: { desiredReplicas: 2, readyReplicas: 0 },
      });
      expect(
        [happy, partial, broken].sort(compareKubernetesDeployments).map((r) => r.id),
      ).toEqual(['dep-broken', 'dep-partial', 'dep-happy']);
    });

    it('mixes controller kinds in a single attention-first order', () => {
      const cronOk = makeResource({
        id: 'cron-ok',
        type: 'k8s-cronjob',
        kubernetes: { suspend: false },
      });
      const jobFailed = makeResource({
        id: 'job-fail',
        type: 'k8s-job',
        kubernetes: { failed: 1 },
      });
      const dsMisscheduled = makeResource({
        id: 'ds-mis',
        type: 'k8s-daemonset',
        kubernetes: { desiredNumberScheduled: 2, numberReady: 2, numberMisscheduled: 1 },
      });
      const rsHappy = makeResource({
        id: 'rs-ok',
        type: 'k8s-replicaset',
        kubernetes: { desiredReplicas: 1, readyReplicas: 1 },
      });
      // `muted` (suspended CronJob, scaled-to-zero Deployment, etc.) sits between
      // `warning` and `success` so deliberate-non-running rows float above
      // fully-healthy rows — matches the rank ordering vSphere already uses for
      // its VM status table.
      expect(
        [cronOk, jobFailed, dsMisscheduled, rsHappy].sort(compareKubernetesControllers).map(
          (r) => r.id,
        ),
      ).toEqual(['job-fail', 'ds-mis', 'cron-ok', 'rs-ok']);
    });

    it('emits pre-sorted buckets from buildKubernetesPageModel', () => {
      const crashing = makeResource({
        id: 'pod-crash',
        type: 'pod',
        kubernetes: {
          podPhase: 'Running',
          podContainers: [{ ready: false, state: 'waiting', reason: 'CrashLoopBackOff' }],
        },
      });
      const happy = makeResource({
        id: 'pod-happy',
        type: 'pod',
        kubernetes: {
          podPhase: 'Running',
          podContainers: [{ ready: true, state: 'running' }],
        },
      });
      const partial = makeResource({
        id: 'dep-partial',
        type: 'k8s-deployment',
        kubernetes: { desiredReplicas: 3, readyReplicas: 1 },
      });
      const broken = makeResource({
        id: 'dep-broken',
        type: 'k8s-deployment',
        kubernetes: { desiredReplicas: 3, readyReplicas: 0 },
      });

      const model = buildKubernetesPageModel([happy, crashing, partial, broken]);
      expect(model.pods.map((r) => r.id)).toEqual(['pod-crash', 'pod-happy']);
      expect(model.deployments.map((r) => r.id)).toEqual(['dep-broken', 'dep-partial']);
      expect(model.workloads.slice(0, 2).map((r) => r.id)).toEqual(['dep-broken', 'dep-partial']);
    });
  });
});
