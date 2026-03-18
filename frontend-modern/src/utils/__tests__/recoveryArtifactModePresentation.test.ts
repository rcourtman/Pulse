import { describe, expect, it } from 'vitest';
import { getRecoveryArtifactModePresentation } from '@/utils/recoveryArtifactModePresentation';

describe('recoveryArtifactModePresentation', () => {
  it('returns canonical labels, badge tones, and chart segment classes', () => {
    expect(getRecoveryArtifactModePresentation('snapshot')).toEqual({
      label: 'Snapshots',
      badgeClassName: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300',
      segmentClassName: 'bg-yellow-500',
    });
    expect(getRecoveryArtifactModePresentation('local')).toEqual({
      label: 'Local',
      badgeClassName: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
      segmentClassName: 'bg-orange-500',
    });
    expect(getRecoveryArtifactModePresentation('remote')).toEqual({
      label: 'Remote',
      badgeClassName: 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300',
      segmentClassName: 'bg-violet-500',
    });
  });
});
