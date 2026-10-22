import type { ResourcePageSubject } from '@/features/platformNavigation/resourcePagePath';
import type { WorkloadReference } from './proxmoxBackupRecoveryModel';

// The guest a backup row is about, when Pulse matched it to a monitored one.
// Host backups have no workload drawer to open.
export const proxmoxBackupWorkloadSubject = (
  workload: WorkloadReference,
): ResourcePageSubject | null => {
  const id = workload.resourceId?.trim();
  if (!id || (workload.type !== 'vm' && workload.type !== 'ct')) return null;
  return {
    id,
    name: workload.name || workload.label,
    type: workload.type === 'ct' ? 'lxc' : 'vm',
  };
};
