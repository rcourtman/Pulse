import type { ProtectionPosture, ProtectionPosturePolicy } from '@/types/recovery';

export const protectionPolicy: ProtectionPosturePolicy = {
  freshnessWindowSeconds: 604800,
  verificationWindowSeconds: 604800,
  requireVerification: true,
};

export function protectionPosture(subjectResourceId = 'vm:1'): ProtectionPosture {
  return {
    subjectResourceId,
    state: 'protected',
    freshness: 'current',
    verification: 'verified',
    coverage: 'complete',
    providerStates: [
      {
        provider: 'proxmox-pbs',
        source: 'pbs-backup-enumeration',
        scope: 'pbs-fixture/main',
        jobState: 'success',
        historyCompleteness: 'complete',
        permissions: 'sufficient',
        evidenceIds: [],
      },
    ],
    repositoryResourceIds: [],
    evidenceIds: [],
    explanation: 'A current verified backup is available.',
    evaluatedAt: '2026-10-26T12:00:00Z',
  };
}
