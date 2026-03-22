import { ThresholdsTableProxmoxBackupsSection } from './ThresholdsTableProxmoxBackupsSection';
import { ThresholdsTableProxmoxGuestFilteringSection } from './ThresholdsTableProxmoxGuestFilteringSection';
import { ThresholdsTableProxmoxGuestsSection } from './ThresholdsTableProxmoxGuestsSection';
import { ThresholdsTableProxmoxNodesSection } from './ThresholdsTableProxmoxNodesSection';
import { ThresholdsTableProxmoxPBSSection } from './ThresholdsTableProxmoxPBSSection';
import { ThresholdsTableProxmoxSnapshotsSection } from './ThresholdsTableProxmoxSnapshotsSection';
import { ThresholdsTableProxmoxStorageSection } from './ThresholdsTableProxmoxStorageSection';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableProxmoxTab(props: ThresholdsTableSectionProps) {
  return (
    <>
      <ThresholdsTableProxmoxNodesSection {...props} />
      <ThresholdsTableProxmoxPBSSection {...props} />
      <ThresholdsTableProxmoxGuestsSection {...props} />
      <ThresholdsTableProxmoxGuestFilteringSection {...props} />
      <ThresholdsTableProxmoxBackupsSection {...props} />
      <ThresholdsTableProxmoxSnapshotsSection {...props} />
      <ThresholdsTableProxmoxStorageSection {...props} />
    </>
  );
}
