import { describe, expect, it } from 'vitest';
import type { UnifiedBackup } from '@/types/backups';
import {
  buildBackupSourceOptions,
  normalizeBackupSourceKey,
  resolveLegacyBackupTypeForSource,
  resolveSourceFromLegacyBackupType,
} from '@/components/Backups/backupSourceOptions';

const makeBackup = (partial: Partial<UnifiedBackup>): UnifiedBackup => ({
  source: partial.source || 'pve',
  backupType: partial.backupType || 'local',
  vmid: partial.vmid || 101,
  name: partial.name || 'guest-101',
  type: partial.type || 'VM',
  node: partial.node || 'pve1',
  instance: partial.instance || 'cluster-a',
  backupTime: partial.backupTime || 1,
  backupName: partial.backupName || 'vzdump-qemu-101',
  description: partial.description || '',
  status: partial.status || 'ok',
  size: partial.size ?? 1,
  storage: partial.storage ?? 'local',
  datastore: partial.datastore ?? null,
  namespace: partial.namespace ?? null,
  verified: partial.verified ?? null,
  protected: partial.protected ?? false,
  encrypted: partial.encrypted,
  owner: partial.owner,
  comment: partial.comment,
});

describe('backupSourceOptions', () => {
  it('normalizes source aliases and maps legacy backupType to source', () => {
    expect(normalizeBackupSourceKey('proxmox')).toBe('pve');
    expect(normalizeBackupSourceKey('remote')).toBe('pbs');
    expect(resolveSourceFromLegacyBackupType('snapshot')).toBe('snapshot');
    expect(resolveSourceFromLegacyBackupType('local')).toBe('pve');
    expect(resolveSourceFromLegacyBackupType('remote')).toBe('pbs');
  });

  it('maps source to legacy backupType for URL compatibility', () => {
    expect(resolveLegacyBackupTypeForSource('snapshot')).toBe('snapshot');
    expect(resolveLegacyBackupTypeForSource('pve')).toBe('local');
    expect(resolveLegacyBackupTypeForSource('pbs')).toBe('remote');
    expect(resolveLegacyBackupTypeForSource('pmg')).toBeNull();
  });

  it('builds source options from discovered backup sources', () => {
    const options = buildBackupSourceOptions([
      makeBackup({ source: 'snapshot', backupType: 'snapshot' }),
      makeBackup({ source: 'pve', backupType: 'local', vmid: 102 }),
      makeBackup({ source: 'pbs', backupType: 'remote', vmid: 103 }),
      makeBackup({ source: 'pmg', backupType: 'local', vmid: 'pmg-host' }),
    ]);

    expect(options[0]).toMatchObject({ key: 'all', label: 'All Sources' });
    expect(options.map((option) => option.key)).toEqual(['all', 'snapshot', 'pve', 'pbs', 'pmg']);
  });
});
