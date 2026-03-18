export type RecoveryIssueTone = 'none' | 'amber' | 'rose' | 'blue';

export function getRecoveryIssueRailClass(tone: Exclude<RecoveryIssueTone, 'none'>): string {
  switch (tone) {
    case 'rose':
      return 'bg-rose-500';
    case 'blue':
      return 'bg-blue-500';
    default:
      return 'bg-amber-400';
  }
}
