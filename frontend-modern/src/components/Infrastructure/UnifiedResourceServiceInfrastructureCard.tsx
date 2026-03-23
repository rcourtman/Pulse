import { Show } from 'solid-js';
import type { Component } from 'solid-js';
import { Card } from '@/components/shared/Card';
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
        <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
          Service Infrastructure
        </div>
        <UnifiedResourcePBSTableSection tableProps={tableProps} table={table} />
        <UnifiedResourcePMGTableSection tableProps={tableProps} table={table} />
      </Card>
    </Show>
  );
};
