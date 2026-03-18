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
      expect(canonicalizeAlertTargetType('lxc')).toBeUndefined();
    });

    it('intentionally leaves k8s generic aliases unresolved at this layer', () => {
      expect(canonicalizeAlertTargetType('k8s')).toBeUndefined();
      expect(canonicalizeAlertTargetType('kubernetes')).toBeUndefined();
    });
  });

  describe('inferAlertTargetTypeFromResourceId', () => {
    it('infers canonical target types from resource id patterns', () => {
      expect(inferAlertTargetTypeFromResourceId('vm-101')).toBe('vm');
      expect(inferAlertTargetTypeFromResourceId('lxc-100')).toBe('system-container');
      expect(inferAlertTargetTypeFromResourceId('docker:abc')).toBe('app-container');
      expect(inferAlertTargetTypeFromResourceId('node:host1')).toBe('agent');
      expect(inferAlertTargetTypeFromResourceId('pod:ns/name')).toBe('pod');
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

      expect(resolveAlertTargetType({ alertType: 'custom_alert' })).toBe('agent');
    });
  });
});
