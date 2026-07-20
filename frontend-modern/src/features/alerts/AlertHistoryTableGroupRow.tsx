import { TableCell, TableRow } from '@/components/shared/Table';
import {
  GROUPED_TABLE_ROW_META_CLASS,
  getGroupedTableRowCellClass,
  getGroupedTableRowClass,
} from '@/components/shared/groupedTableRowPresentation';

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

  return parts.join(', ') || `${group.alerts.length} item${group.alerts.length === 1 ? '' : 's'}`;
}

export function AlertHistoryTableGroupRow(props: AlertHistoryTableGroupRowProps) {
  return (
    <TableRow class={getGroupedTableRowClass()}>
      <TableCell colspan={9} class={getGroupedTableRowCellClass()}>
        <div class="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
          <span class="truncate" title={props.group.fullLabel}>
            {props.group.label}
          </span>
          <span class={GROUPED_TABLE_ROW_META_CLASS}>{getGroupSummaryLabel(props.group)}</span>
        </div>
      </TableCell>
    </TableRow>
  );
}
