import { Component } from 'solid-js';
import { Node } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { StackedDiskBar } from '@/components/Dashboard/StackedDiskBar';

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
      total: total
    };
  };

  return (
    <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600/70 dark:bg-slate-800">
      <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Root Disk</div>
      <div class="mb-3">
        <div class="flex justify-between text-[10px] mb-1">
          <span class="text-slate-500 dark:text-slate-400">Usage</span>
          <span class="text-slate-700 dark:text-slate-200">
            {formatBytes(diskStats().used)} / {formatBytes(diskStats().total)}
          </span>
        </div>
        <StackedDiskBar
          aggregateDisk={{
            total: diskStats().total,
            used: diskStats().used,
            free: diskStats().total - diskStats().used,
            usage: diskStats().percent / 100
          }}
        />
      </div>
    </div>
  );
};
