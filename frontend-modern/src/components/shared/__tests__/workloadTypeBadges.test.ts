import { describe, expect, it } from 'vitest';
import * as workloadTypeBadges from '@/components/shared/workloadTypeBadges';
import { getWorkloadTypeBadge } from '@/components/shared/workloadTypeBadges';

describe('workloadTypeBadges', () => {
  it('keeps the badge module rendering-only', () => {
    expect(workloadTypeBadges).toHaveProperty('getWorkloadTypeBadge');
    expect(workloadTypeBadges).not.toHaveProperty('getWorkloadTypePresentation');
    expect(workloadTypeBadges).not.toHaveProperty('normalizeWorkloadTypePresentationKey');
  });

  describe('getWorkloadTypeBadge', () => {
    describe('VM types', () => {
      it('returns VM badge for vm', () => {
        const result = getWorkloadTypeBadge('vm');
        expect(result.label).toBe('VM');
        expect(result.title).toBe('Virtual Machine');
      });

      it('does not normalize removed qemu alias', () => {
        const result = getWorkloadTypeBadge('qemu');
        expect(result.label).toBe('Qemu');
        expect(result.title).toBe('Qemu');
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

      it('does not normalize removed ct alias', () => {
        const result = getWorkloadTypeBadge('ct');
        expect(result.label).toBe('Ct');
        expect(result.title).toBe('Ct');
      });

      it('does not normalize removed container alias', () => {
        const result = getWorkloadTypeBadge('container');
        expect(result.label).toBe('Container');
        expect(result.title).toBe('Container');
      });
    });

    describe('App container types', () => {
      it('returns Containers badge for canonical app-container', () => {
        const result = getWorkloadTypeBadge('app-container');
        expect(result.label).toBe('Containers');
      });

      it('retains docker alias compatibility', () => {
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

    describe('Pod types', () => {
      it('returns Pod badge for pod', () => {
        const result = getWorkloadTypeBadge('pod');
        expect(result.label).toBe('Pod');
      });

      it('retains k8s alias compatibility', () => {
        const result = getWorkloadTypeBadge('k8s');
        expect(result.label).toBe('Pod');
      });

      it('retains kubernetes alias compatibility', () => {
        const result = getWorkloadTypeBadge('kubernetes');
        expect(result.label).toBe('Pod');
      });
    });

    describe('Agent types', () => {
      it('normalizes legacy host alias to Agent badge label', () => {
        const result = getWorkloadTypeBadge('host');
        expect(result.label).toBe('Agent');
        expect(result.title).toBe('Agent');
      });

      it('returns Agent badge for agent', () => {
        const result = getWorkloadTypeBadge('agent');
        expect(result.label).toBe('Agent');
        expect(result.title).toBe('Agent');
      });
    });

    describe('OCI types', () => {
      it('returns OCI badge for oci-container', () => {
        const result = getWorkloadTypeBadge('oci-container');
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
