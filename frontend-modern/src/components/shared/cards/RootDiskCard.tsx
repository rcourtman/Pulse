import { Component } from 'solid-js';
import { Node } from '@/types/api';
import { InfoCardFrame, InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';
import { formatBytes } from '@/utils/format';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';

interface RootDiskCardProps {
  node: Node;
}

export const RootDiskCard: Component<RootDiskCardProps> = (props) => {
  const diskStats = () => {
    if (!props.node.disk) return { percent: 0, used: 0, total: 0 };
    const total = props.node.disk.total || 0;
    const used = props.node.disk.used || 0;
    return {
      percent: total > 0 ? (used / total) * 100 : 0,
      used: used,
      total: total,
    };
  };

  return (
    <InfoCardFrame>
      <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
        Root Disk
      </div>
      <div class="mb-3">
        <InfoCardKeyValueRow
          class="mb-1 text-[10px]"
          label="Usage"
          value={`${formatBytes(diskStats().used)} / ${formatBytes(diskStats().total)}`}
          valueClass="font-normal"
        />
        <StackedDiskBar
          aggregateDisk={{
            total: diskStats().total,
            used: diskStats().used,
            free: diskStats().total - diskStats().used,
            usage: diskStats().percent / 100,
          }}
        />
      </div>
    </InfoCardFrame>
  );
};
