import type { BackupTask, GuestSnapshot, PBSBackup, StorageBackup } from '@/types/api';
import type { Resource } from '@/types/resource';

export const BACKUP_IDENTITY_NOW = Date.parse('2026-10-04T12:00:00Z');

export const identityGuest = (
  id: string,
  instance: string,
  node: string,
  extra: Partial<NonNullable<Resource['proxmox']>> = {},
): Resource =>
  ({
    id,
    name: id,
    type: 'vm',
    platformId: instance,
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'running',
    lastSeen: BACKUP_IDENTITY_NOW,
    proxmox: { vmid: 100, instance, nodeName: node, ...extra },
  }) as Resource;

export const identityArchive = (extra: Partial<StorageBackup> = {}): StorageBackup => ({
  id: 'file-100',
  instance: 'east',
  node: 'pve-1',
  type: 'vm',
  vmid: 100,
  storage: 'local',
  time: '2026-10-04T11:00:00Z',
  ctime: BACKUP_IDENTITY_NOW / 1000 - 3600,
  size: 1024,
  format: 'zst',
  volid: 'local:backup/vzdump-qemu-100.vma.zst',
  protected: false,
  isPBS: false,
  verified: false,
  ...extra,
});

export const identitySnapshot = (extra: Partial<GuestSnapshot> = {}): GuestSnapshot => ({
  id: 'snapshot-100',
  instance: 'east',
  node: 'pve-1',
  type: 'vm',
  vmid: 100,
  name: 'before-update',
  time: '2026-10-04T11:30:00Z',
  vmstate: false,
  ...extra,
});

export const identityPBS = (extra: Partial<PBSBackup> = {}): PBSBackup => ({
  id: 'pbs-100',
  instance: 'pbs-server',
  namespace: 'pve-1',
  datastore: 'main',
  backupType: 'vm',
  vmid: '100',
  backupTime: '2026-10-04T10:00:00Z',
  size: 2048,
  files: ['index.json.blob'],
  protected: false,
  verified: true,
  ...extra,
});

export const identityTask = (extra: Partial<BackupTask> = {}): BackupTask => ({
  id: 'task-100',
  instance: 'east',
  node: 'pve-1',
  type: 'vzdump',
  vmid: 100,
  status: 'OK',
  startTime: '2026-10-04T11:00:00Z',
  endTime: '2026-10-04T11:05:00Z',
  ...extra,
});
