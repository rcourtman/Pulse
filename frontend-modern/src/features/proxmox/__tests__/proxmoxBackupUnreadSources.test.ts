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

  it('names connections only when the gap spans several, and reports denied access', () => {
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', {
        provider: 'proxmox-pve',
        scope: 'east',
        historyCompleteness: 'partial',
        unreadScopes: [{ kind: 'node', node: 'pve2', accessDenied: true }],
      }),
      withProvider('vm:2', {
        provider: 'proxmox-pbs',
        scope: 'west',
        historyCompleteness: 'partial',
        unreadScopes: [
          { kind: 'namespace', storage: 'archive', namespace: '', accessDenied: true },
        ],
      }),
    ]);
    expect(sources.map((source) => source.label)).toEqual([
      'the root namespace of datastore archive (west)',
      'the storage list on node pve2 (east)',
    ]);
    expect(describeUnreadBackupSources(sources)).toMatch(
      /^Pulse does not have permission to read .* and .*\. Check the permissions of the Pulse API token\.$/,
    );
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
      'Pulse cannot read archive on pve1 (east) (access denied) and namespace team on datastore vault (offsite), so it cannot confirm the backups stored there. For the other unread sources, check that Pulse can reach the storage through its Proxmox Backup Server connection.',
    );
  });

  it('checks the configured connections instead of picking one when both routes are unread', () => {
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', {
        provider: 'proxmox-pve',
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
      'Pulse cannot read archive on pve1 (east) and the namespace list of datastore vault (offsite), so it cannot confirm the backups stored there. Check that the backup sources are reachable through their configured connections.',
    );
  });

  it('keeps a denial visible when that source is past the first three names', () => {
    const sources = collectUnreadBackupSources([
      withProvider('vm:1', {
        provider: 'proxmox-pve',
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
    expect(describeUnreadBackupSources(sources)).not.toMatch(/API token/);
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
