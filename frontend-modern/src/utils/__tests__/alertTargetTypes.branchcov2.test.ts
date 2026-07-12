import { describe, expect, it } from 'vitest';
import { inferAlertTargetTypeFromResourceId } from '@/utils/alertTargetTypes';

describe('alertTargetTypes · inferAlertTargetTypeFromResourceId (branch coverage)', () => {
  describe('empty / falsy input → undefined (guard branch)', () => {
    it('returns undefined for undefined input', () => {
      expect(inferAlertTargetTypeFromResourceId(undefined)).toBeUndefined();
    });

    it('returns undefined for empty string', () => {
      expect(inferAlertTargetTypeFromResourceId('')).toBeUndefined();
    });

    it('returns undefined for whitespace-only string (normalized is empty)', () => {
      expect(inferAlertTargetTypeFromResourceId('   ')).toBeUndefined();
    });

    it('returns undefined for null cast to string | undefined', () => {
      const maybeNull = null as unknown as string | undefined;
      expect(inferAlertTargetTypeFromResourceId(maybeNull)).toBeUndefined();
    });
  });

  describe('VM inference (startsWith vm- / qemu- / includes /qemu/)', () => {
    it('detects "vm-" prefix', () => {
      expect(inferAlertTargetTypeFromResourceId('vm-101')).toBe('vm');
    });

    it('detects "qemu-" prefix', () => {
      expect(inferAlertTargetTypeFromResourceId('qemu-202')).toBe('vm');
    });

    it('detects "/qemu/" segment in a path-style id', () => {
      expect(inferAlertTargetTypeFromResourceId('nodes/prox/qemu/303')).toBe('vm');
    });

    it('is case-insensitive via normalization', () => {
      expect(inferAlertTargetTypeFromResourceId('VM-404')).toBe('vm');
      expect(inferAlertTargetTypeFromResourceId('  QEMU-505  ')).toBe('vm');
    });
  });

  describe('system-container inference (ct- / lxc- / includes /lxc/)', () => {
    it('detects "ct-" prefix', () => {
      expect(inferAlertTargetTypeFromResourceId('ct-100')).toBe('system-container');
    });

    it('detects "lxc-" prefix', () => {
      expect(inferAlertTargetTypeFromResourceId('lxc-100')).toBe('system-container');
    });

    it('detects "/lxc/" segment in a path-style id', () => {
      expect(inferAlertTargetTypeFromResourceId('nodes/prox/lxc/101')).toBe('system-container');
    });
  });

  describe('app-container inference (docker: / app-container: / includes /container:)', () => {
    it('detects "docker:" prefix', () => {
      expect(inferAlertTargetTypeFromResourceId('docker:abc')).toBe('app-container');
    });

    it('detects "app-container:" prefix', () => {
      expect(inferAlertTargetTypeFromResourceId('app-container:my-svc')).toBe('app-container');
    });

    it('detects "/container:" segment in a path-style id', () => {
      expect(inferAlertTargetTypeFromResourceId('host/stack/container:worker')).toBe(
        'app-container',
      );
    });
  });

  describe('singleton-prefix targets', () => {
    it('detects "node:" → agent', () => {
      expect(inferAlertTargetTypeFromResourceId('node:host1')).toBe('agent');
    });

    it('detects "storage:" → storage', () => {
      expect(inferAlertTargetTypeFromResourceId('storage:zfs-pool')).toBe('storage');
    });

    it('detects "disk:" → disk', () => {
      expect(inferAlertTargetTypeFromResourceId('disk:sdb')).toBe('disk');
    });

    it('detects "pbs:" → pbs', () => {
      expect(inferAlertTargetTypeFromResourceId('pbs:main')).toBe('pbs');
    });

    it('detects "pmg:" → pmg', () => {
      expect(inferAlertTargetTypeFromResourceId('pmg:mail')).toBe('pmg');
    });
  });

  describe('pod inference (k8s-pod: / k8s-pod- / pod: / includes :pod:)', () => {
    it('detects "k8s-pod:" prefix', () => {
      expect(inferAlertTargetTypeFromResourceId('k8s-pod:ns/web')).toBe('pod');
    });

    it('detects "k8s-pod-" prefix (hyphen form, takes priority over k8s: cluster fallback)', () => {
      expect(inferAlertTargetTypeFromResourceId('k8s-pod-abc')).toBe('pod');
    });

    it('detects "pod:" prefix', () => {
      expect(inferAlertTargetTypeFromResourceId('pod:ns/name')).toBe('pod');
    });

    it('detects ":pod:" embedded segment', () => {
      expect(inferAlertTargetTypeFromResourceId('cluster:pod:web')).toBe('pod');
    });
  });

  describe('k8s: hierarchy (nested conditionals)', () => {
    it('detects ":deployment:" inside k8s: → k8s-deployment', () => {
      expect(inferAlertTargetTypeFromResourceId('k8s:cluster-1:deployment:api')).toBe(
        'k8s-deployment',
      );
    });

    it('detects ":namespace:" inside k8s: → k8s-namespace', () => {
      expect(inferAlertTargetTypeFromResourceId('k8s:cluster-1:namespace:default')).toBe(
        'k8s-namespace',
      );
    });

    it('detects ":node:" inside k8s: → k8s-node', () => {
      expect(inferAlertTargetTypeFromResourceId('k8s:cluster-1:node:worker-1')).toBe('k8s-node');
    });

    it('falls back to k8s-cluster when no nested marker is present', () => {
      expect(inferAlertTargetTypeFromResourceId('k8s:cluster-1')).toBe('k8s-cluster');
    });
  });

  describe('truenas / disk targets', () => {
    it('detects "truenas-system:" prefix → truenas', () => {
      expect(inferAlertTargetTypeFromResourceId('truenas-system:box-1')).toBe('truenas');
    });

    it('detects "system:truenas" prefix → truenas (alternate spelling)', () => {
      expect(inferAlertTargetTypeFromResourceId('system:truenas')).toBe('truenas');
    });

    it('detects "truenas-pool:" prefix → pool', () => {
      expect(inferAlertTargetTypeFromResourceId('truenas-pool:tank')).toBe('pool');
    });

    it('detects "truenas-dataset:" prefix → truenas-dataset', () => {
      expect(inferAlertTargetTypeFromResourceId('truenas-dataset:tank/media')).toBe(
        'truenas-dataset',
      );
    });

    it('detects "truenas-disk:" prefix → physical_disk', () => {
      expect(inferAlertTargetTypeFromResourceId('truenas-disk:da0')).toBe('physical_disk');
    });

    it('detects "physical-disk:" prefix → physical_disk (alternate spelling)', () => {
      expect(inferAlertTargetTypeFromResourceId('physical-disk:sda')).toBe('physical_disk');
    });
  });

  describe('vmware / vsphere prefix forms (startsWith branches)', () => {
    it('detects "vmware-host:" prefix → vmware-host', () => {
      expect(inferAlertTargetTypeFromResourceId('vmware-host:host-1')).toBe('vmware-host');
    });

    it('detects "vsphere-host:" prefix → vmware-host', () => {
      expect(inferAlertTargetTypeFromResourceId('vsphere-host:host-2')).toBe('vmware-host');
    });

    it('detects "vmware-vm:" prefix → vmware-vm', () => {
      expect(inferAlertTargetTypeFromResourceId('vmware-vm:vm-1')).toBe('vmware-vm');
    });

    it('detects "vsphere-vm:" prefix → vmware-vm', () => {
      expect(inferAlertTargetTypeFromResourceId('vsphere-vm:vm-2')).toBe('vmware-vm');
    });

    it('detects "vmware-datastore:" prefix → vmware-datastore', () => {
      expect(inferAlertTargetTypeFromResourceId('vmware-datastore:ds-1')).toBe('vmware-datastore');
    });

    it('detects "vsphere-datastore:" prefix → vmware-datastore', () => {
      expect(inferAlertTargetTypeFromResourceId('vsphere-datastore:ds-2')).toBe('vmware-datastore');
    });

    it('detects "vmware-network:" prefix → vmware-network', () => {
      expect(inferAlertTargetTypeFromResourceId('vmware-network:net-1')).toBe('vmware-network');
    });

    it('detects "vsphere-network:" prefix → vmware-network', () => {
      expect(inferAlertTargetTypeFromResourceId('vsphere-network:net-2')).toBe('vmware-network');
    });
  });

  describe('vmware: / vsphere: generic block (nested conditionals)', () => {
    it('detects ":host:" inside "vmware:" → vmware-host', () => {
      expect(inferAlertTargetTypeFromResourceId('vmware:vc-1:host:host-101')).toBe('vmware-host');
    });

    it('detects ":vm:" inside "vmware:" → vmware-vm', () => {
      expect(inferAlertTargetTypeFromResourceId('vmware:vc-1:vm:vm-201')).toBe('vmware-vm');
    });

    it('detects ":datastore:" inside "vmware:" → vmware-datastore', () => {
      expect(inferAlertTargetTypeFromResourceId('vmware:vc-1:datastore:ds-301')).toBe(
        'vmware-datastore',
      );
    });

    it('detects ":network:" inside "vmware:" → vmware-network', () => {
      expect(inferAlertTargetTypeFromResourceId('vmware:vc-1:network:net-401')).toBe(
        'vmware-network',
      );
    });

    it('falls back to vmware-vsphere when "vmware:" has no nested marker', () => {
      expect(inferAlertTargetTypeFromResourceId('vmware:vc-1')).toBe('vmware-vsphere');
    });

    it('handles "vsphere:" generic block fallback → vmware-vsphere', () => {
      expect(inferAlertTargetTypeFromResourceId('vsphere:vc-2')).toBe('vmware-vsphere');
    });

    it('detects nested marker inside "vsphere:" → vmware-vm', () => {
      expect(inferAlertTargetTypeFromResourceId('vsphere:vc-2:vm:vm-9')).toBe('vmware-vm');
    });
  });

  describe('terminating fallthrough → undefined', () => {
    it('returns undefined for a completely unknown id shape', () => {
      expect(inferAlertTargetTypeFromResourceId('random-resource-xyz')).toBeUndefined();
    });

    it('returns undefined when a partial substring would match but no prefix does', () => {
      // "vm" appears but not at the start; nothing matches → undefined
      expect(inferAlertTargetTypeFromResourceId('service-vm-runner')).toBeUndefined();
    });

    it('returns undefined for a bare colon-only string', () => {
      // normalize yields ":" which has no matching prefix
      expect(inferAlertTargetTypeFromResourceId(':')).toBeUndefined();
    });
  });
});
