import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesStorageTable } from '../KubernetesStorageTable';

const gib = (value: number): number => value * 1024 * 1024 * 1024;

const makeResource = ({
  id,
  type,
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'cluster-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'online',
  type,
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

afterEach(() => {
  cleanup();
});

describe('KubernetesStorageTable', () => {
  it('renders StorageClass, PersistentVolume, and PersistentVolumeClaim fields from the Kubernetes storage APIs', () => {
    render(() => (
      <KubernetesStorageTable
        resources={[
          makeResource({
            id: 'fast-csi',
            type: 'k8s-storage-class',
            kubernetes: {
              resourceKind: 'StorageClass',
              provisioner: 'csi.truenas.example',
              reclaimPolicy: 'Delete',
              volumeBindingMode: 'WaitForFirstConsumer',
              allowVolumeExpansion: true,
              parameterKeys: ['fstype'],
            },
          }),
          makeResource({
            id: 'pv-postgres',
            type: 'k8s-persistent-volume',
            kubernetes: {
              resourceKind: 'PersistentVolume',
              phase: 'Bound',
              storageClass: 'fast-csi',
              capacityBytes: gib(100),
              accessModes: ['ReadWriteOnce'],
              reclaimPolicy: 'Retain',
              claimNamespace: 'services',
              claimName: 'postgres-data',
            },
          }),
          makeResource({
            id: 'postgres-data',
            type: 'k8s-persistent-volume-claim',
            kubernetes: {
              namespace: 'services',
              resourceKind: 'PersistentVolumeClaim',
              phase: 'Bound',
              storageClass: 'fast-csi',
              requestedBytes: gib(100),
              capacityBytes: gib(100),
              accessModes: ['ReadWriteOnce'],
              volumeName: 'pv-postgres',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Binding / phase')).toBeInTheDocument();
    expect(screen.getByText('Access / policy')).toBeInTheDocument();
    expect(screen.getByText('Provider / binding')).toBeInTheDocument();
    expect(screen.getByText('WaitForFirstConsumer')).toBeInTheDocument();
    expect(screen.getByText('csi.truenas.example')).toBeInTheDocument();
    expect(screen.getAllByText('Bound')).toHaveLength(2);
    expect(screen.getAllByText('100 GB')).toHaveLength(2);
    expect(screen.getByText('RWO · Retain')).toBeInTheDocument();
    expect(screen.getByText('services/postgres-data')).toBeInTheDocument();
    expect(screen.getAllByText('pv-postgres').length).toBeGreaterThanOrEqual(1);
  });
});
