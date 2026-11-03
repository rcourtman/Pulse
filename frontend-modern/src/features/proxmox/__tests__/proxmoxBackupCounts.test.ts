import { describe, expect, it } from 'vitest';
import {
  describeIncompleteArtifacts,
  restorePointCountNoun,
  type RecoverableArtifact,
} from '../proxmoxBackupRecoveryModel';

const artifact = (flags: { running?: boolean; failed?: boolean } = {}) =>
  flags as RecoverableArtifact;

describe('describeIncompleteArtifacts', () => {
  it.each([
    ['nothing incomplete', [artifact()], ''],
    ['an empty list', [], ''],
    ['running only', [artifact({ running: true }), artifact()], '1 running'],
    ['failed only', [artifact({ failed: true }), artifact({ failed: true })], '2 failed'],
    ['both', [artifact({ running: true }), artifact({ failed: true })], '1 running · 1 failed'],
    [
      'an entry flagged both, once as running',
      [artifact({ running: true, failed: true })],
      '1 running',
    ],
  ])('describes %s', (_label, artifacts, expected) => {
    expect(describeIncompleteArtifacts(artifacts)).toBe(expected);
  });
});

describe('restorePointCountNoun', () => {
  it.each([
    [1, false, 'restore point'],
    [2, false, 'restore points'],
    [0, false, 'restore points'],
    [1, true, 'restore point read'],
    [2, true, 'restore points read'],
  ])('reads %i with incomplete=%s as %s', (count, incomplete, expected) => {
    expect(restorePointCountNoun(count, incomplete)).toBe(expected);
  });
});
