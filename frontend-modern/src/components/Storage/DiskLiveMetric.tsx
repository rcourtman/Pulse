import { useDiskLiveMetricModel } from './useDiskLiveMetricModel';

interface DiskLiveMetricProps {
  resourceId: string;
  type: 'read' | 'write' | 'ioTime';
}

export function DiskLiveMetric(props: DiskLiveMetricProps) {
  const { formatted, colorClass } = useDiskLiveMetricModel({
    resourceId: () => props.resourceId,
    type: () => props.type,
  });

  return <span class={`font-mono text-[11px] sm:text-xs ${colorClass()}`}>{formatted()}</span>;
}
