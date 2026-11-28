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

  it('surfaces stale readings on storage classes, volumes, and claims while keeping offline severity', () => {
    const staleSource = {
      kubernetes: { status: 'stale', lastSeen: '2026-11-26T10:00:00Z' },
    };
    const { container } = render(() => (
      <KubernetesStorageTable
        resources={[
          makeResource({
            id: 'fresh-pvc',
            type: 'k8s-persistent-volume-claim',
            status: 'online',
            kubernetes: {
              namespace: 'apps',
              phase: 'Bound',
              storageClass: 'fast-csi',
              capacityBytes: gib(50),
            },
          }),
          makeResource({
            id: 'stale-pvc',
            type: 'k8s-persistent-volume-claim',
            status: 'online',
            sourceStatus: staleSource,
            kubernetes: {
              namespace: 'billing',
              phase: 'Bound',
              storageClass: 'fast-csi',
              capacityBytes: gib(100),
            },
          }),
          makeResource({
            id: 'offline-stale-pv',
            type: 'k8s-persistent-volume',
            status: 'offline',
            sourceStatus: staleSource,
            kubernetes: {
              phase: 'Failed',
              storageClass: 'fast-csi',
              capacityBytes: gib(200),
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
        showToolbar={false}
      />
    ));

    const freshRow = container.querySelector('[data-kubernetes-storage-row="fresh-pvc"]');
    expect(freshRow?.querySelector('[title="Online"]')).toHaveClass('bg-emerald-500');
    expect(freshRow?.querySelector('[data-kubernetes-stale="storage"]')).toBeNull();

    const staleRow = container.querySelector('[data-kubernetes-storage-row="stale-pvc"]');
    expect(staleRow?.querySelector('[title="Stale data"]')).toHaveClass('bg-amber-500');
    expect(staleRow?.querySelector('[data-kubernetes-stale="storage"]')).toHaveTextContent(
      'Stale data',
    );
    expect(staleRow).toHaveTextContent('100 GB');

    const offlineRow = container.querySelector('[data-kubernetes-storage-row="offline-stale-pv"]');
    expect(offlineRow?.querySelector('[title="Offline"]')).toHaveClass('bg-red-500');
    expect(offlineRow?.querySelector('[data-kubernetes-stale="storage"]')).toHaveTextContent(
      'Stale data',
    );
  });
});
