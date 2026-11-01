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
    expect(sources).toEqual([{ label: 'pbs-energy-offsite on delly', accessDenied: false }]);
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
    const sources = ['a', 'b', 'c', 'd', 'e'].map((label) => ({ label, accessDenied: false }));
    expect(describeUnreadBackupSources(sources)).toMatch(/^Pulse cannot read a, b, c and 2 more,/);
  });
});
