import { Show } from 'solid-js';
import type { Component } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SummaryTableCardHeader } from '@/components/shared/SummaryTableCardHeader';
import {
  type UnifiedResourceTableProps,
  type UnifiedResourceTableState,
} from './useUnifiedResourceTableState';
import { UnifiedResourcePBSTableSection } from './UnifiedResourcePBSTableSection';
import { UnifiedResourcePMGTableSection } from './UnifiedResourcePMGTableSection';

interface UnifiedResourceServiceInfrastructureCardProps {
  tableProps: UnifiedResourceTableProps;
  table: UnifiedResourceTableState;
}

export const UnifiedResourceServiceInfrastructureCard: Component<
  UnifiedResourceServiceInfrastructureCardProps
> = (props) => {
  const { table, tableProps } = props;

  return (
    <Show when={table.sortedPBSResources().length > 0 || table.sortedPMGResources().length > 0}>
      <Card padding="none" tone="card" class="mb-0 overflow-hidden">
        <SummaryTableCardHeader
          title="Service Infrastructure"
          showClearAction={table.showServiceClearAction()}
          onClear={tableProps.clearPinnedSummaryScope}
        />
        <UnifiedResourcePBSTableSection tableProps={tableProps} table={table} />
        <UnifiedResourcePMGTableSection tableProps={tableProps} table={table} />
      </Card>
    </Show>
  );
};
