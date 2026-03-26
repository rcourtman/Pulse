export type RecoveryArtifactMode = 'snapshot' | 'local' | 'remote';

export interface RecoveryArtifactModePresentation {
  label: string;
  aggregateLabel: string;
  badgeClassName: string;
  segmentClassName: string;
}

export function getRecoveryArtifactModePresentation(
  mode: RecoveryArtifactMode,
): RecoveryArtifactModePresentation {
  switch (mode) {
    case 'snapshot':
      return {
        label: 'Snapshot',
        aggregateLabel: 'Snapshots',
        badgeClassName: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300',
        segmentClassName: 'bg-yellow-500',
      };
    case 'remote':
      return {
        label: 'Remote Copy',
        aggregateLabel: 'Remote Copies',
        badgeClassName: 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300',
        segmentClassName: 'bg-violet-500',
      };
    default:
      return {
        label: 'Local Copy',
        aggregateLabel: 'Local Copies',
        badgeClassName: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
        segmentClassName: 'bg-orange-500',
      };
  }
}
