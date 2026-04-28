import { Show } from 'solid-js';
import type { Component } from 'solid-js';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { TableCard } from '@/components/shared/TableCard';
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
      <TableCard class="mb-0">
        <TableCardHeader
          title="Service Infrastructure"
          showClearAction={table.showServiceClearAction()}
          onClear={tableProps.clearPinnedSummaryScope}
        />
        <UnifiedResourcePBSTableSection tableProps={tableProps} table={table} />
        <UnifiedResourcePMGTableSection tableProps={tableProps} table={table} />
      </TableCard>
    </Show>
  );
};
