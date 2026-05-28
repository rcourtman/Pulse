import { describe, expect, it } from 'vitest';
import {
  canonicalizeAlertTargetType,
  inferAlertTargetTypeFromResourceId,
  resolveAlertTargetType,
} from '@/utils/alertTargetTypes';

describe('alertTargetTypes', () => {
  describe('canonicalizeAlertTargetType', () => {
    it('canonicalizes supported aliases used for alert investigation targets', () => {
      expect(canonicalizeAlertTargetType('host')).toBe('agent');
      expect(canonicalizeAlertTargetType('docker-service')).toBe('app-container');
      expect(canonicalizeAlertTargetType('kubernetes')).toBe('k8s-cluster');
      expect(canonicalizeAlertTargetType('Kubernetes Namespace')).toBe('k8s-namespace');
      expect(canonicalizeAlertTargetType('TrueNAS Dataset')).toBe('truenas-dataset');
      expect(canonicalizeAlertTargetType('TrueNAS Disk')).toBe('physical_disk');
      expect(canonicalizeAlertTargetType('vSphere Host')).toBe('vmware-host');
      expect(canonicalizeAlertTargetType('VMware Virtual Machine')).toBe('vmware-vm');
      expect(canonicalizeAlertTargetType('vSphere Datastore')).toBe('vmware-datastore');
      expect(canonicalizeAlertTargetType('VMware Network')).toBe('vmware-network');
      expect(canonicalizeAlertTargetType('lxc')).toBeUndefined();
    });
  });

  describe('inferAlertTargetTypeFromResourceId', () => {
    it('infers canonical target types from resource id patterns', () => {
      expect(inferAlertTargetTypeFromResourceId('vm-101')).toBe('vm');
      expect(inferAlertTargetTypeFromResourceId('lxc-100')).toBe('system-container');
      expect(inferAlertTargetTypeFromResourceId('docker:abc')).toBe('app-container');
      expect(inferAlertTargetTypeFromResourceId('node:host1')).toBe('agent');
      expect(inferAlertTargetTypeFromResourceId('pod:ns/name')).toBe('pod');
      expect(inferAlertTargetTypeFromResourceId('k8s:cluster-1:deployment:api')).toBe(
        'k8s-deployment',
      );
      expect(inferAlertTargetTypeFromResourceId('truenas-dataset:tank/media')).toBe(
        'truenas-dataset',
      );
      expect(inferAlertTargetTypeFromResourceId('vmware:vc-1:host:host-101')).toBe('vmware-host');
      expect(inferAlertTargetTypeFromResourceId('vmware:vc-1:datastore:datastore-301')).toBe(
        'vmware-datastore',
      );
    });
  });

  describe('resolveAlertTargetType', () => {
    it('prioritizes alert-type prefixes over other hints', () => {
      expect(
        resolveAlertTargetType({
          alertType: 'docker_cpu_high',
          resourceType: 'vm',
          resourceId: 'vm-101',
        }),
      ).toBe('app-container');
    });

    it('falls back through explicit type, metadata type, resource id, then agent', () => {
      expect(
        resolveAlertTargetType({
          alertType: 'custom_alert',
          resourceType: 'host',
        }),
      ).toBe('agent');

      expect(
        resolveAlertTargetType({
          alertType: 'custom_alert',
          metadataResourceType: 'docker-service',
        }),
      ).toBe('app-container');

      expect(
        resolveAlertTargetType({
          alertType: 'custom_alert',
          resourceId: 'pbs:main',
        }),
      ).toBe('pbs');

      expect(
        resolveAlertTargetType({
          alertType: 'custom_alert',
          metadataResourceType: 'TrueNAS Disk',
          resourceId: 'disk:sdb',
        }),
      ).toBe('physical_disk');

      expect(
        resolveAlertTargetType({
          alertType: 'custom_alert',
          metadataResourceType: 'vSphere VM',
          resourceId: 'vmware:vc-1:vm:vm-201',
        }),
      ).toBe('vmware-vm');

      expect(resolveAlertTargetType({ alertType: 'custom_alert' })).toBe('agent');
    });
  });
});
