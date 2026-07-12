import { describe, expect, it } from 'vitest';

import type { WorkloadGuest } from '@/types/workloads';
import {
  buildWorkloadNodeOptions,
  buildWorkloadsKubernetesNamespaceOptions,
  buildWorkloadsVmwareClusterOptions,
} from '../workloadRouteModel';

const makeGuest = (overrides?: Partial<WorkloadGuest>): WorkloadGuest =>
  ({
    id: 'guest-1',
    vmid: 101,
    name: 'guest-1',
    node: 'node-a',
    instance: 'cluster-a',
    status: 'running',
    type: 'vm',
    cpu: 0,
    cpus: 2,
    memory: { total: 1024, used: 256, free: 768, usage: 0.25 },
    disk: { total: 1024, used: 256, free: 768, usage: 0.25 },
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 0,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: new Date().toISOString(),
    workloadType: 'vm',
    ...overrides,
  }) as WorkloadGuest;

describe('workloadRouteModel (branch coverage 2)', () => {
  describe('buildWorkloadNodeOptions', () => {
    it('disambiguates app-container hosts sharing a host label by appending scope when both scopes differ from the label', () => {
      // Two Docker app-containers share contextLabel "tower.local" (the host
      // label) but have distinct dockerHostId scopes. In loop 1 both register
      // under disambiguationLabel "tower.local" -> set size 2. In loop 2 the
      // app-container arm hits hasDuplicateHostLabel===true and, because each
      // scope !== hostLabel, formats "${hostLabel} (${scope})".
      const options = buildWorkloadNodeOptions([
        makeGuest({
          id: 'docker-a',
          type: 'app-container',
          workloadType: 'app-container',
          node: '',
          instance: '',
          contextLabel: 'tower.local',
          dockerHostId: 'docker-host-1',
        }),
        makeGuest({
          id: 'docker-b',
          type: 'app-container',
          workloadType: 'app-container',
          node: '',
          instance: '',
          contextLabel: 'tower.local',
          dockerHostId: 'docker-host-2',
        }),
      ]);

      expect(options).toEqual([
        { value: 'docker-host-1', label: 'tower.local (docker-host-1)' },
        { value: 'docker-host-2', label: 'tower.local (docker-host-2)' },
      ]);
    });

    it('keeps the bare host label for the app-container whose scope equals the host label even when duplicates exist', () => {
      // guestA has dockerHostId="" so its scope falls through to contextLabel
      // "tower.local" == hostLabel; guestB has dockerHostId="dh1" so scope="dh1"
      // != hostLabel. Both share hostLabel "tower.local" -> duplicate set size 2.
      // For guestA: hasDuplicateHostLabel && scope !== hostLabel is true && false
      // -> false arm -> bare hostLabel. For guestB: true arm -> disambiguated.
      const options = buildWorkloadNodeOptions([
        makeGuest({
          id: 'docker-canonical',
          type: 'app-container',
          workloadType: 'app-container',
          node: '',
          instance: '',
          contextLabel: 'tower.local',
          dockerHostId: '',
        }),
        makeGuest({
          id: 'docker-named',
          type: 'app-container',
          workloadType: 'app-container',
          node: '',
          instance: '',
          contextLabel: 'tower.local',
          dockerHostId: 'dh1',
        }),
      ]);

      expect(options).toEqual([
        { value: 'tower.local', label: 'tower.local' },
        { value: 'dh1', label: 'tower.local (dh1)' },
      ]);
    });

    it('skips vm guests whose node-scope id is the placeholder "-" (empty instance and node)', () => {
      // workloadNodeScopeId = `${instance}-${node}` -> "-" when both empty.
      // Loop 1: scope === '-' -> B2 dash arm skip. Loop 2: same -> B8 dash arm.
      const options = buildWorkloadNodeOptions([
        makeGuest({ id: 'empty-vm', node: '', instance: '' }),
        makeGuest({ id: 'vm-real', node: 'node-a', instance: 'cluster-a' }),
      ]);

      expect(options).toEqual([{ value: 'cluster-a-node-a', label: 'node-a' }]);
    });

    it('skips app-container guests whose host scope is empty string (all host-id candidates blank)', () => {
      // getWorkloadContainerHostId returns "" when dockerHostId, contextLabel,
      // node and instance are all empty -> !scope true arm (loop 1 B2 and loop 2 B8).
      const options = buildWorkloadNodeOptions([
        makeGuest({
          id: 'docker-blank',
          type: 'app-container',
          workloadType: 'app-container',
          node: '',
          instance: '',
          contextLabel: '',
          dockerHostId: '',
        }),
        makeGuest({
          id: 'docker-real',
          type: 'app-container',
          workloadType: 'app-container',
          node: '',
          instance: '',
          contextLabel: 'host-1',
          dockerHostId: 'dh1',
        }),
      ]);

      expect(options).toEqual([{ value: 'dh1', label: 'host-1' }]);
    });

    it('skips vm guests with a valid scope but empty node (empty host label) in both loops', () => {
      // instance set, node empty -> scope "c1-" (non-empty, not "-") passes B2/B8.
      // label = (node||'').trim() = "" -> loop 1 B3 (!label) skip. Loop 2 vm arm:
      // nodeName = "" -> B13 (!nodeName) skip. The guest produces no option.
      const options = buildWorkloadNodeOptions([
        makeGuest({ id: 'no-node', node: '', instance: 'c1' }),
        makeGuest({ id: 'with-node', node: 'node-a', instance: 'c2' }),
      ]);

      expect(options).toEqual([{ value: 'c2-node-a', label: 'node-a' }]);
    });

    it('falls back to the bare node name when a duplicate node has no instance to disambiguate with', () => {
      // vm1: node 'shared', instance 'c1' -> scope 'c1-shared'.
      // vm2: node 'shared', instance ''   -> scope '-shared'.
      // Loop 1: scopesByLabel['shared'] = {'c1-shared', '-shared'} size 2.
      // Loop 2 vm2: hasDuplicateNodeName===true but instance is '' (falsy) ->
      // `hasDuplicateNodeName && instance` short-circuits to false -> label = nodeName.
      // vm1: true && 'c1' -> true arm -> 'shared (c1)'.
      const options = buildWorkloadNodeOptions([
        makeGuest({ id: 'vm-with-inst', node: 'shared', instance: 'c1' }),
        makeGuest({ id: 'vm-no-inst', node: 'shared', instance: '' }),
      ]);

      expect(options).toEqual([
        { value: '-shared', label: 'shared' },
        { value: 'c1-shared', label: 'shared (c1)' },
      ]);
    });

    it('skips pod guests in both passes so they never produce node options', () => {
      const options = buildWorkloadNodeOptions([
        makeGuest({
          id: 'pod-a',
          type: 'pod',
          workloadType: 'pod',
          node: 'worker-a',
          instance: 'ctx',
        }),
        makeGuest({ id: 'vm-a', node: 'node-a', instance: 'cluster-a' }),
      ]);

      expect(options).toEqual([{ value: 'cluster-a-node-a', label: 'node-a' }]);
    });

    it('returns an empty array for an empty guest list', () => {
      expect(buildWorkloadNodeOptions([])).toEqual([]);
    });
  });

  describe('buildWorkloadsKubernetesNamespaceOptions', () => {
    it('ignores the context filter when selectedContext is null and collects namespaces from pods in every context', () => {
      // (selectedContext || '').trim() -> '' -> contextFilter falsy ->
      // `contextFilter && ...` short-circuits to false (no skip) for every pod.
      const guests = [
        makeGuest({
          id: 'pod-a',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'prod',
          namespace: 'default',
        }),
        makeGuest({
          id: 'pod-b',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'stage',
          namespace: 'kube-system',
        }),
        makeGuest({
          id: 'pod-c',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'prod',
          namespace: 'default',
        }),
      ];

      expect(buildWorkloadsKubernetesNamespaceOptions(guests, null)).toEqual([
        'default',
        'kube-system',
      ]);
      // Contrast: with an explicit context, only that context's namespaces surface.
      expect(buildWorkloadsKubernetesNamespaceOptions(guests, 'prod')).toEqual(['default']);
      expect(buildWorkloadsKubernetesNamespaceOptions(guests, 'stage')).toEqual(['kube-system']);
    });

    it('treats a whitespace-only selectedContext as no filter after trimming', () => {
      const guests = [
        makeGuest({
          id: 'pod-a',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'prod',
          namespace: 'default',
        }),
        makeGuest({
          id: 'pod-b',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'stage',
          namespace: 'observability',
        }),
      ];

      // '  ' -> trim -> '' -> falsy -> both contexts' namespaces collected.
      expect(buildWorkloadsKubernetesNamespaceOptions(guests, '  ')).toEqual([
        'default',
        'observability',
      ]);
    });

    it('skips non-pod guests even when they carry a namespace', () => {
      // resolveWorkloadType(guest) !== 'pod' -> continue (C2 true arm).
      const guests = [
        makeGuest({
          id: 'vm-a',
          type: 'vm',
          workloadType: 'vm',
          contextLabel: 'prod',
          namespace: 'should-be-ignored',
        }),
        makeGuest({
          id: 'app-a',
          type: 'app-container',
          workloadType: 'app-container',
          contextLabel: 'prod',
          namespace: 'also-ignored',
        }),
        makeGuest({
          id: 'pod-a',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'prod',
          namespace: 'default',
        }),
      ];

      expect(buildWorkloadsKubernetesNamespaceOptions(guests, 'prod')).toEqual(['default']);
    });

    it('skips a pod whose namespace is missing or blank even when the context matches', () => {
      // namespace undefined -> (guest.namespace || '').trim() -> '' (C4 ?? arm)
      // -> if (namespace) false -> not added (C5 empty skip).
      const guests = [
        makeGuest({
          id: 'pod-undef',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'prod',
        }),
        makeGuest({
          id: 'pod-empty',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'prod',
          namespace: '',
        }),
        makeGuest({
          id: 'pod-ws',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'prod',
          namespace: '   ',
        }),
        makeGuest({
          id: 'pod-real',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'prod',
          namespace: 'default',
        }),
      ];

      expect(buildWorkloadsKubernetesNamespaceOptions(guests, 'prod')).toEqual(['default']);
    });

    it('drops pods whose context key does not match the selected context', () => {
      // contextFilter truthy && getKubernetesContextKey(guest) !== contextFilter
      // -> C3 true arm (mismatch skip). pod-b context 'stage' filtered out.
      const guests = [
        makeGuest({
          id: 'pod-a',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'prod',
          namespace: 'default',
        }),
        makeGuest({
          id: 'pod-b',
          type: 'pod',
          workloadType: 'pod',
          contextLabel: 'stage',
          namespace: 'kube-system',
        }),
      ];

      expect(buildWorkloadsKubernetesNamespaceOptions(guests, 'prod')).toEqual(['default']);
    });

    it('returns an empty array for an empty guest list regardless of context', () => {
      expect(buildWorkloadsKubernetesNamespaceOptions([], 'prod')).toEqual([]);
      expect(buildWorkloadsKubernetesNamespaceOptions([], null)).toEqual([]);
    });
  });

  describe('buildWorkloadsVmwareClusterOptions', () => {
    it('collects, deduplicates and sorts cluster names from vm guests', () => {
      // resolveWorkloadType === 'vm' -> proceed (D1 false arm);
      // clusterName non-empty -> add (D3 true arm).
      const guests = [
        makeGuest({ id: 'vm-a', clusterName: 'cluster-b' }),
        makeGuest({ id: 'vm-b', clusterName: 'cluster-a' }),
        makeGuest({ id: 'vm-c', clusterName: 'cluster-a' }),
        makeGuest({ id: 'vm-d', clusterName: 'cluster-c' }),
      ];

      expect(buildWorkloadsVmwareClusterOptions(guests)).toEqual([
        'cluster-a',
        'cluster-b',
        'cluster-c',
      ]);
    });

    it('skips vm guests whose clusterName is undefined', () => {
      // (guest.clusterName || '').trim() -> '' (D2 ?? arm) -> if (cluster) false (D3 empty skip).
      const guests = [
        makeGuest({ id: 'vm-undef' }),
        makeGuest({ id: 'vm-real', clusterName: 'cluster-a' }),
      ];

      expect(buildWorkloadsVmwareClusterOptions(guests)).toEqual(['cluster-a']);
    });

    it('skips vm guests whose clusterName is empty or whitespace-only', () => {
      const guests = [
        makeGuest({ id: 'vm-empty', clusterName: '' }),
        makeGuest({ id: 'vm-ws', clusterName: '   ' }),
        makeGuest({ id: 'vm-real', clusterName: 'cluster-a' }),
      ];

      expect(buildWorkloadsVmwareClusterOptions(guests)).toEqual(['cluster-a']);
    });

    it('skips non-vm guests even when they carry a clusterName', () => {
      // resolveWorkloadType !== 'vm' -> continue (D1 true arm) for each.
      const guests = [
        makeGuest({
          id: 'pod-a',
          type: 'pod',
          workloadType: 'pod',
          clusterName: 'cluster-a',
        }),
        makeGuest({
          id: 'app-a',
          type: 'app-container',
          workloadType: 'app-container',
          clusterName: 'cluster-b',
        }),
        makeGuest({
          id: 'lxc-a',
          type: 'lxc',
          workloadType: 'system-container',
          clusterName: 'cluster-c',
        }),
        makeGuest({ id: 'vm-a', clusterName: 'cluster-real' }),
      ];

      expect(buildWorkloadsVmwareClusterOptions(guests)).toEqual(['cluster-real']);
    });

    it('returns an empty array for an empty guest list', () => {
      expect(buildWorkloadsVmwareClusterOptions([])).toEqual([]);
    });
  });
});
