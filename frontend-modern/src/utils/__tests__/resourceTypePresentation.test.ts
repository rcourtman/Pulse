import { describe, expect, it } from 'vitest';
import {
  getResourceTypeLabel,
  getResourceTypePresentation,
} from '@/utils/resourceTypePresentation';

describe('resourceTypePresentation', () => {
  it('returns canonical labels for unified resource types', () => {
    expect(getResourceTypeLabel('docker-host')).toBe('Container Runtime');
    expect(getResourceTypeLabel('k8s-cluster')).toBe('K8s Cluster');
    expect(getResourceTypeLabel('truenas')).toBe('TrueNAS');
  });

  it('returns shared presentations for external recovery subject aliases', () => {
    expect(getResourceTypePresentation('proxmox-vm')).toMatchObject({ label: 'VM' });
    expect(getResourceTypePresentation('proxmox-lxc')).toMatchObject({ label: 'LXC' });
    expect(getResourceTypePresentation('truenas-dataset')).toMatchObject({ label: 'Dataset' });
    expect(getResourceTypePresentation('docker-container')).toMatchObject({
      label: 'Container',
    });
  });
});
