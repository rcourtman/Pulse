import { describe, expect, it } from 'vitest';
import type { ProtectionPosture, ProtectionProviderState } from '@/types/recovery';
import { protectionPosture } from '@/utils/__fixtures__/protectionPosture';
import {
  collectUnreadBackupSources,
  describeUnreadBackupSources,
} from '../proxmoxBackupUnreadSources';

const withProvider = (
  id: string,
  provider: Partial<ProtectionProviderState>,
): ProtectionPosture => {
  const posture = protectionPosture(id);
  posture.providerStates = [{ ...posture.providerStates[0], ...provider }];
  return posture;
};

describe('unread backup sources', () => {
  it('collapses the same unread storage across guests into one actionable line', () => {
    const partial: Partial<ProtectionProviderState> = {
      provider: 'proxmox-pve',
      source: 'pve-backup-enumeration',
      scope: 'homelab',
      historyCompleteness: 'partial',
      unreadScopes: [{ kind: 'storage', node: 'delly', storage: 'pbs-energy-offsite' }],
    };
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', partial),
      withProvider('vm:2', partial),
    ]);
    expect(sources).toEqual([
      { label: 'pbs-energy-offsite on delly', provider: 'proxmox-pve', accessDenied: false },
    ]);
    expect(describeUnreadBackupSources(sources)).toBe(
      'Pulse cannot read pbs-energy-offsite on delly, so it cannot confirm the backups stored there. Check that the storage is reachable from Proxmox.',
    );
  });

  it('distinguishes denied reads even when PVE and PBS use the same connection name', () => {
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', {
        provider: 'proxmox-pve',
        source: 'pve-backup-enumeration',
        scope: 'site',
        historyCompleteness: 'partial',
        unreadScopes: [{ kind: 'node', node: 'pve2', accessDenied: true }],
      }),
      withProvider('vm:2', {
        provider: 'proxmox-pbs',
        scope: 'site',
        historyCompleteness: 'partial',
        unreadScopes: [
          { kind: 'namespace', storage: 'archive', namespace: '', accessDenied: true },
        ],
      }),
    ]);
    expect(sources.map((source) => source.label)).toEqual([
      'the root namespace of datastore archive (PBS connection: site)',
      'the storage list on node pve2 (PVE connection: site)',
    ]);
    const message = describeUnreadBackupSources(sources);
    expect(message).toMatch(/^Pulse does not have permission to read .* and .*, so/);
    expect(message).toContain(
      "Check the Pulse API token's read permissions where access was denied.",
    );
    expect(message).not.toContain('reachable');
  });

  it('names the denied reader even if it is the only known connection', () => {
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', {
        scope: ' offsite ',
        historyCompleteness: 'partial',
        unreadScopes: [{ kind: 'namespace', storage: 'vault', accessDenied: true }],
      }),
    ]);
    expect(sources[0]?.label).toBe(
      'the root namespace of datastore vault (PBS connection: offsite)',
    );
  });

  it('does not interpret a legacy retained-point scope as a second configured reader', () => {
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', {
        scope: 'offsite',
        historyCompleteness: 'partial',
        unreadScopes: [{ kind: 'datastore', storage: 'vault' }],
      }),
      withProvider('vm:2', {
        scope: 'another-scope',
        source: 'legacy-recovery-point',
        historyCompleteness: 'unknown',
      }),
    ]);
    expect(sources[0]?.label).toBe('the namespace list of datastore vault');
  });

  it('points a direct PBS namespace read to the PBS connection, not the VE host', () => {
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', {
        provider: 'proxmox-pbs',
        scope: 'offsite',
        historyCompleteness: 'partial',
        unreadScopes: [{ kind: 'namespace', storage: 'vault', namespace: 'team' }],
      }),
    ]);
    expect(describeUnreadBackupSources(sources)).toBe(
      'Pulse cannot read namespace team on datastore vault, so it cannot confirm the backups stored there. Check that Pulse can reach the storage through its Proxmox Backup Server connection.',
    );
  });

  it('keeps a refusal visible and gives reachability advice for the other unread source', () => {
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', {
        provider: 'proxmox-pve',
        source: 'pve-backup-enumeration',
        scope: 'east',
        historyCompleteness: 'partial',
        unreadScopes: [{ kind: 'storage', node: 'pve1', storage: 'archive', accessDenied: true }],
      }),
      withProvider('vm:1', {
        provider: 'proxmox-pbs',
        scope: 'offsite',
        historyCompleteness: 'partial',
        unreadScopes: [{ kind: 'namespace', storage: 'vault', namespace: 'team' }],
      }),
    ]);
    expect(describeUnreadBackupSources(sources)).toBe(
      "Pulse cannot read archive on pve1 (PVE connection: east) (access denied) and namespace team on datastore vault (PBS connection: offsite), so it cannot confirm the backups stored there. Check the Pulse API token's read permissions where access was denied. For the other unread sources, check that Pulse can reach the storage through its Proxmox Backup Server connection.",
    );
  });

  it('checks the configured connections instead of picking one when both routes are unread', () => {
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', {
        provider: 'proxmox-pve',
        source: 'pve-backup-enumeration',
        scope: 'east',
        historyCompleteness: 'partial',
        unreadScopes: [{ kind: 'storage', node: 'pve1', storage: 'archive' }],
      }),
      withProvider('vm:1', {
        provider: 'proxmox-pbs',
        scope: 'offsite',
        historyCompleteness: 'partial',
        unreadScopes: [{ kind: 'datastore', storage: 'vault' }],
      }),
    ]);
    expect(describeUnreadBackupSources(sources)).toBe(
      'Pulse cannot read archive on pve1 (PVE connection: east) and the namespace list of datastore vault (PBS connection: offsite), so it cannot confirm the backups stored there. Check that the backup sources are reachable through their configured connections.',
    );
  });

  it('keeps a denial visible when that source is past the first three names', () => {
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', {
        provider: 'proxmox-pve',
        source: 'pve-backup-enumeration',
        scope: 'east',
        historyCompleteness: 'partial',
        unreadScopes: [
          { kind: 'storage', storage: 'a' },
          { kind: 'storage', storage: 'b' },
          { kind: 'storage', storage: 'c' },
          { kind: 'storage', storage: 'd', accessDenied: true },
          { kind: 'storage', storage: 'e' },
        ],
      }),
    ]);
    expect(describeUnreadBackupSources(sources)).toMatch(
      /^Pulse cannot read a, b, c and 2 more \(1 access denied\),/,
    );
    expect(describeUnreadBackupSources(sources)).toContain(
      'read permissions where access was denied',
    );
  });

  it('shows nothing for complete history or when no scope is named', () => {
    expect(
      collectUnreadBackupSources([
        protectionPosture('vm:1'),
        withProvider('vm:2', {
          historyCompleteness: 'complete',
          unreadScopes: [{ kind: 'storage', storage: 'stale' }],
        }),
        withProvider('vm:3', { historyCompleteness: 'partial' }),
      ]),
    ).toEqual([]);
    expect(describeUnreadBackupSources([])).toBeNull();
  });

  it('counts sources past the first three', () => {
    const sources = ['a', 'b', 'c', 'd', 'e'].map((label) => ({
      label,
      provider: 'proxmox-pve',
      accessDenied: false,
    }));
    expect(describeUnreadBackupSources(sources)).toMatch(/^Pulse cannot read a, b, c and 2 more,/);
  });
});
