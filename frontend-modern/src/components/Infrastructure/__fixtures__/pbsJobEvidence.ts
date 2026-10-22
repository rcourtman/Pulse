// Read-only synthetic values shared by DOM and real-browser regression checks.
import type { PBSPlatformData } from '../resourceDetailMappers';

export const PBS_JOB_EVIDENCE_FIXTURE = {
  taskId: `nightly-backup-${'x'.repeat(72)}`,
  evidenceId: `backup-history-${'y'.repeat(72)}`,
  store: `datastore-${'d'.repeat(56)}`,
  remote: `offsite-${'r'.repeat(56)}`,
  namespace: `department/${'n'.repeat(56)}`,
  workerId: `worker-${'w'.repeat(56)}`,
  state: 'Unknown: the partial read did not establish the last job result',
  reason:
    'The observed task history is incomplete. No current protection assessment can be made from this read.',
  error: `Synthetic read error for scope-${'s'.repeat(72)}.\nThe final line must remain readable: <em>literal text, not markup</em>.`,
  confidence: `unclassified-${'c'.repeat(72)}`,
  observedAt: '2026-10-22T06:17:07Z',
} as const;

export function pbsJobEvidenceFixture(): PBSPlatformData {
  const f = PBS_JOB_EVIDENCE_FIXTURE;
  return {
    datastoreCount: 1,
    backupJobCount: 1,
    backupJobs: [
      {
        id: f.taskId,
        store: f.store,
        type: 'vm',
        vmid: '100',
        lastBackup: '',
        nextRun: '',
        status: 'running',
        error: f.error,
      },
    ],
    jobHealthEvidence: [
      {
        id: f.evidenceId,
        family: 'backup',
        store: f.store,
        remote: f.remote,
        namespace: f.namespace,
        'worker-id': f.workerId,
        'last-run-state': f.state,
        evidenceSource: 'partial-read',
        evidenceScope: 'partial-read',
        confidence: 'permission gap',
        posture: 'unknown',
        postureReason: f.reason,
        freshness: { observedAt: f.observedAt },
        error: f.error,
      },
      {
        id: 'unknown-confidence',
        family: 'verify',
        confidence: f.confidence,
        posture: 'unknown',
      },
    ],
  };
}
