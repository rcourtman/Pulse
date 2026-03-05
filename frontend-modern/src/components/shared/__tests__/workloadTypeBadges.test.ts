import { describe, expect, it } from 'vitest';
import { getWorkloadTypeBadge } from '@/components/shared/workloadTypeBadges';

describe('workloadTypeBadges', () => {
  describe('getWorkloadTypeBadge', () => {
    describe('VM types', () => {
      it('returns VM badge for vm', () => {
        const result = getWorkloadTypeBadge('vm');
        expect(result.label).toBe('VM');
        expect(result.title).toBe('Virtual Machine');
      });

      it('normalizes legacy qemu alias to VM', () => {
        const result = getWorkloadTypeBadge('qemu');
        expect(result.label).toBe('VM');
        expect(result.title).toBe('Virtual Machine');
      });

      it('returns VM badge for VM (case insensitive)', () => {
        const result = getWorkloadTypeBadge('VM');
        expect(result.label).toBe('VM');
      });
    });

    describe('System container types', () => {
      it('returns Container badge for system-container', () => {
        const result = getWorkloadTypeBadge('system-container');
        expect(result.label).toBe('Container');
        expect(result.title).toBe('System Container');
      });

      it('normalizes legacy ct alias to system-container', () => {
        const result = getWorkloadTypeBadge('ct');
        expect(result.label).toBe('Container');
        expect(result.title).toBe('System Container');
      });

      it('normalizes legacy container alias to system-container', () => {
        const result = getWorkloadTypeBadge('container');
        expect(result.label).toBe('Container');
        expect(result.title).toBe('System Container');
      });
    });

    describe('Docker types', () => {
      it('returns Containers badge for docker', () => {
        const result = getWorkloadTypeBadge('docker');
        expect(result.label).toBe('Containers');
      });

      it('does not normalize removed docker-container alias', () => {
        const result = getWorkloadTypeBadge('docker-container');
        expect(result.label).toBe('Docker Container');
        expect(result.title).toBe('Docker Container');
      });

      it('does not normalize removed docker_container alias', () => {
        const result = getWorkloadTypeBadge('docker_container');
        expect(result.label).toBe('Docker Container');
        expect(result.title).toBe('Docker Container');
      });
    });

    describe('Kubernetes types', () => {
      it('returns K8s badge for k8s', () => {
        const result = getWorkloadTypeBadge('k8s');
        expect(result.label).toBe('K8s');
      });

      it('returns K8s badge for kubernetes', () => {
        const result = getWorkloadTypeBadge('kubernetes');
        expect(result.label).toBe('K8s');
      });

      it('returns Pod badge for pod', () => {
        const result = getWorkloadTypeBadge('pod');
        expect(result.label).toBe('Pod');
      });
    });

    describe('Agent types', () => {
      it('does not normalize removed host alias to Agent', () => {
        const result = getWorkloadTypeBadge('host');
        expect(result.label).toBe('Host');
        expect(result.title).toBe('Host');
      });

      it('returns Agent badge for agent', () => {
        const result = getWorkloadTypeBadge('agent');
        expect(result.label).toBe('Agent');
        expect(result.title).toBe('Agent');
      });
    });

    describe('OCI types', () => {
      it('returns OCI badge for oci', () => {
        const result = getWorkloadTypeBadge('oci');
        expect(result.label).toBe('OCI');
        expect(result.title).toBe('OCI Container');
      });
    });

    describe('unknown types', () => {
      it('returns Unknown for empty string', () => {
        const result = getWorkloadTypeBadge('');
        expect(result.label).toBe('Unknown');
      });

      it('returns Unknown for undefined', () => {
        const result = getWorkloadTypeBadge(undefined);
        expect(result.label).toBe('Unknown');
      });

      it('returns Unknown for null', () => {
        const result = getWorkloadTypeBadge(null);
        expect(result.label).toBe('Unknown');
      });

      it('returns titleized unknown type', () => {
        const result = getWorkloadTypeBadge('custom-type');
        expect(result.label).toBe('Custom Type');
      });
    });

    describe('overrides', () => {
      it('allows label override', () => {
        const result = getWorkloadTypeBadge('vm', { label: 'Virtual Machine' });
        expect(result.label).toBe('Virtual Machine');
      });

      it('allows title override', () => {
        const result = getWorkloadTypeBadge('vm', { title: 'My VM' });
        expect(result.title).toBe('My VM');
      });

      it('allows both label and title override', () => {
        const result = getWorkloadTypeBadge('vm', { label: 'VM Label', title: 'VM Title' });
        expect(result.label).toBe('VM Label');
        expect(result.title).toBe('VM Title');
      });
    });
  });
});
