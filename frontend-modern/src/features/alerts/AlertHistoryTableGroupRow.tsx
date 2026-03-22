import { TableCell, TableRow } from '@/components/shared/Table';

import type { AlertHistoryState } from './useAlertHistoryState';

type AlertHistoryGroup = ReturnType<AlertHistoryState['groupedAlerts']>[number];

interface AlertHistoryTableGroupRowProps {
  group: AlertHistoryGroup;
}

function getGroupSummaryLabel(group: AlertHistoryGroup) {
  const alertCount = group.alerts.filter((alert) => alert.source === 'alert').length;
  const aiCount = group.alerts.filter((alert) => alert.source === 'ai').length;
  const parts = [];

  if (alertCount > 0) {
    parts.push(`${alertCount} alert${alertCount === 1 ? '' : 's'}`);
  }
  if (aiCount > 0) {
    parts.push(`${aiCount} patrol insight${aiCount === 1 ? '' : 's'}`);
  }

  return (
    parts.join(', ') || `${group.alerts.length} item${group.alerts.length === 1 ? '' : 's'}`
  );
}

export function AlertHistoryTableGroupRow(props: AlertHistoryTableGroupRowProps) {
  return (
    <TableRow class="bg-surface-alt">
      <TableCell colspan={10} class="py-1.5 pr-3 pl-4 text-[12px] font-semibold sm:text-sm">
        <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
          <span class="truncate" title={props.group.fullLabel}>
            {props.group.label}
          </span>
          <span class="text-[10px] font-medium text-muted">
            {getGroupSummaryLabel(props.group)}
          </span>
        </div>
      </TableCell>
    </TableRow>
  );
}
