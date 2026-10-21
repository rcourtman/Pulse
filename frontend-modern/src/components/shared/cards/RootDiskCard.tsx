import { Component, Show } from 'solid-js';
import { Node } from '@/types/api';
import { InfoCardFrame, InfoCardKeyValueRow } from '@/components/shared/InfoCardFrame';
import { formatBytes } from '@/utils/format';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';

interface RootDiskCardProps {
  node: Node;
}

export const RootDiskCard: Component<RootDiskCardProps> = (props) => {
  const diskStats = () => {
    const { total, used, usage } = props.node.disk ?? {};
    // This is an aggregate, not an omitempty filesystem entry: absent used
    // bytes are not a measured zero, even when capacity or percent exists.
    if (
      typeof total !== 'number' ||
      !Number.isFinite(total) ||
      total <= 0 ||
      typeof used !== 'number' ||
      !Number.isFinite(used) ||
      used < 0 ||
      used > total ||
      (usage !== undefined && (!Number.isFinite(usage) || usage < 0))
    )
      return null;
    return {
      total,
      used,
      free: total - used,
      usage: used / total,
    };
  };
  const capacity = () => {
    const total = props.node.disk?.total;
    return typeof total === 'number' && Number.isFinite(total) && total > 0
      ? ` · ${formatBytes(total)} total`
      : '';
  };

  return (
    <InfoCardFrame data-testid="root-disk-card">
      <div class="text-[11px] font-medium uppercase tracking-wide text-base-content mb-2">
        Root Disk
      </div>
      <Show
        when={diskStats()}
        fallback={
          <p class="text-xs text-muted whitespace-normal wrap-break-word" role="status">
            Usage unavailable{capacity()}
          </p>
        }
      >
        {(disk) => (
          <div class="mb-3">
            <InfoCardKeyValueRow
              class="mb-1 text-[10px]"
              label="Usage"
              value={`${formatBytes(disk().used)} / ${formatBytes(disk().total)}`}
              valueClass="font-normal"
            />
            <StackedDiskBar aggregateDisk={disk()} />
          </div>
        )}
      </Show>
    </InfoCardFrame>
  );
};
