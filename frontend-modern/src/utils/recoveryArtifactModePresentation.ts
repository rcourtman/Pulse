export type RecoveryArtifactMode = 'snapshot' | 'local' | 'remote';

export interface RecoveryArtifactModePresentation {
  label: string;
  badgeClassName: string;
  segmentClassName: string;
}

export function getRecoveryArtifactModePresentation(
  mode: RecoveryArtifactMode,
): RecoveryArtifactModePresentation {
  switch (mode) {
    case 'snapshot':
      return {
        label: 'Snapshots',
        badgeClassName: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300',
        segmentClassName: 'bg-yellow-500',
      };
    case 'remote':
      return {
        label: 'Remote',
        badgeClassName: 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300',
        segmentClassName: 'bg-violet-500',
      };
    default:
      return {
        label: 'Local',
        badgeClassName: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
        segmentClassName: 'bg-orange-500',
      };
  }
}
