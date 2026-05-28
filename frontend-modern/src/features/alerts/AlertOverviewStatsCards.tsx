import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from '@/components/shared/Table';
import {
  ALERT_OVERVIEW_ACKNOWLEDGED_LABEL,
  ALERT_OVERVIEW_LAST_24_HOURS_LABEL,
  ALERT_OVERVIEW_WORKLOAD_OVERRIDES_LABEL,
} from '@/utils/alertOverviewPresentation';
import type { StatusIndicatorVariant } from '@/utils/status';

import type { AlertOverviewState } from './useAlertOverviewState';

interface AlertOverviewStatsCardsProps {
  state: AlertOverviewState;
}

const dotCellClass = 'w-6 pl-3 pr-0';
const labelCellClass = 'text-base-content';
const valueCellClass = 'pr-3 text-right font-semibold tabular-nums text-base-content';

const VARIANT_ACTIVE: Record<'triggered' | 'acknowledged', StatusIndicatorVariant> = {
  triggered: 'warning',
  acknowledged: 'success',
};

const variantForCount = (
  count: number,
  active: StatusIndicatorVariant,
): StatusIndicatorVariant => (count > 0 ? active : 'muted');

export function AlertOverviewStatsCards(props: AlertOverviewStatsCardsProps) {
  return (
    <Card padding="none" tone="card" class="overflow-hidden">
      <Table class="min-w-full text-xs">
        <TableBody>
          <TableRow>
            <TableCell class={dotCellClass}>
              <StatusDot
                variant={variantForCount(
                  props.state.alertStats().total24h,
                  VARIANT_ACTIVE.triggered,
                )}
                size="sm"
                ariaHidden
              />
            </TableCell>
            <TableCell class={labelCellClass}>{ALERT_OVERVIEW_LAST_24_HOURS_LABEL}</TableCell>
            <TableCell class={valueCellClass} data-testid="alert-overview-stat-value">
              {props.state.alertStats().total24h}
            </TableCell>
          </TableRow>
          <TableRow>
            <TableCell class={dotCellClass}>
              <StatusDot
                variant={variantForCount(
                  props.state.alertStats().acknowledged,
                  VARIANT_ACTIVE.acknowledged,
                )}
                size="sm"
                ariaHidden
              />
            </TableCell>
            <TableCell class={labelCellClass}>{ALERT_OVERVIEW_ACKNOWLEDGED_LABEL}</TableCell>
            <TableCell class={valueCellClass} data-testid="alert-overview-stat-value">
              {props.state.alertStats().acknowledged}
            </TableCell>
          </TableRow>
          <TableRow>
            <TableCell class={dotCellClass}>
              <StatusDot variant="muted" size="sm" ariaHidden />
            </TableCell>
            <TableCell class={labelCellClass}>{ALERT_OVERVIEW_WORKLOAD_OVERRIDES_LABEL}</TableCell>
            <TableCell class={valueCellClass} data-testid="alert-overview-stat-value">
              {props.state.alertStats().overrides}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </Card>
  );
}
