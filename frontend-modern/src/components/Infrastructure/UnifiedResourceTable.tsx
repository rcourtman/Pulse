import type { Component } from 'solid-js';
import {
  useUnifiedResourceTableState,
  type UnifiedResourceTableProps,
} from './useUnifiedResourceTableState';
import { UnifiedResourceHostTableCard } from './UnifiedResourceHostTableCard';
import { UnifiedResourceServiceInfrastructureCard } from './UnifiedResourceServiceInfrastructureCard';

export const UnifiedResourceTable: Component<UnifiedResourceTableProps> = (props) => {
  const table = useUnifiedResourceTableState(props);

  return (
    <div class="space-y-4">
      <UnifiedResourceHostTableCard table={table} tableProps={props} />
      <UnifiedResourceServiceInfrastructureCard table={table} tableProps={props} />
    </div>
  );
};

export default UnifiedResourceTable;
