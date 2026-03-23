import { For, Show } from 'solid-js';
import type { Component } from 'solid-js';
import { Card } from '@/components/shared/Card';
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { InfrastructureSummaryTableRow } from './InfrastructureSummaryTableRow';
import type { InfrastructureSummaryTableProps } from './infrastructureSummaryTableModel';
import { useInfrastructureSummaryTableState } from './useInfrastructureSummaryTableState';

export const InfrastructureSummaryTable: Component<InfrastructureSummaryTableProps> = (props) => {
  const table = useInfrastructureSummaryTableState(props);

  const thClassBase =
    'px-1.5 sm:px-2 py-0.5 text-[11px] sm:text-xs font-medium uppercase tracking-wider cursor-pointer hover:bg-surface-hover whitespace-nowrap';
  const thClass = `${thClassBase} text-center`;

  return (
    <Card padding="none" tone="card" class="mb-4 overflow-hidden">
      <Table
        class="w-full border-collapse whitespace-nowrap"
        style={{ 'table-layout': 'fixed', 'min-width': '800px' }}
      >
        <TableHeader>
          <TableRow class="bg-surface-alt text-muted border-b border-border">
            <TableHead class={`${thClassBase} text-left pl-3`} onClick={() => table.handleSort('name')}>
              {props.currentTab === 'recovery' ? 'Node / PBS' : 'Node'}{' '}
              {table.renderSortIndicator('name')}
            </TableHead>

            <TableHead
              class={thClass}
              style={{ width: '80px', 'min-width': '80px', 'max-width': '80px' }}
              onClick={() => table.handleSort('uptime')}
            >
              Uptime {table.renderSortIndicator('uptime')}
            </TableHead>
            <TableHead
              class={thClass}
              style={
                table.isMobile()
                  ? { 'min-width': '80px' }
                  : { 'min-width': '140px', 'max-width': '180px' }
              }
              onClick={() => table.handleSort('cpu')}
            >
              CPU {table.renderSortIndicator('cpu')}
            </TableHead>
            <TableHead
              class={thClass}
              style={
                table.isMobile()
                  ? { 'min-width': '80px' }
                  : { 'min-width': '140px', 'max-width': '180px' }
              }
              onClick={() => table.handleSort('memory')}
            >
              Memory {table.renderSortIndicator('memory')}
            </TableHead>
            <TableHead
              class={thClass}
              style={
                table.isMobile()
                  ? { 'min-width': '80px' }
                  : { 'min-width': '140px', 'max-width': '180px' }
              }
              onClick={() => table.handleSort('disk')}
            >
              Disk {table.renderSortIndicator('disk')}
            </TableHead>
            <Show when={table.hasAnyTemperatureData()}>
              <TableHead
                class={thClass}
                style={{ width: '60px', 'min-width': '60px', 'max-width': '60px' }}
                onClick={() => table.handleSort('temperature')}
              >
                Temp {table.renderSortIndicator('temperature')}
              </TableHead>
            </Show>
            <Show when={props.currentTab === 'dashboard'}>
              <TableHead
                class={thClass}
                style={{ width: '50px', 'min-width': '50px', 'max-width': '50px' }}
                onClick={() => table.handleSort('vmCount')}
              >
                VMs {table.renderSortIndicator('vmCount')}
              </TableHead>
              <TableHead
                class={thClass}
                style={{ width: '50px', 'min-width': '50px', 'max-width': '50px' }}
                onClick={() => table.handleSort('containerCount')}
              >
                CTs {table.renderSortIndicator('containerCount')}
              </TableHead>
            </Show>
            <Show when={props.currentTab === 'storage'}>
              <TableHead
                class={thClass}
                style={{ width: '70px', 'min-width': '70px', 'max-width': '70px' }}
                onClick={() => table.handleSort('storageCount')}
              >
                Storage {table.renderSortIndicator('storageCount')}
              </TableHead>
              <TableHead
                class={thClass}
                style={{ width: '60px', 'min-width': '60px', 'max-width': '60px' }}
                onClick={() => table.handleSort('diskCount')}
              >
                Disks {table.renderSortIndicator('diskCount')}
              </TableHead>
            </Show>
            <Show when={props.currentTab === 'recovery'}>
              <TableHead
                class={thClass}
                style={{ width: '70px', 'min-width': '70px', 'max-width': '70px' }}
                onClick={() => table.handleSort('backupCount')}
              >
                Recovery {table.renderSortIndicator('backupCount')}
              </TableHead>
            </Show>
            <TableHead
              class={thClass}
              style={{ width: '28px', 'min-width': '28px', 'max-width': '28px' }}
            />
          </TableRow>
        </TableHeader>

        <TableBody class="divide-y divide-border">
          <For each={table.sortedItems()}>
            {(item) => <InfrastructureSummaryTableRow item={item} table={table} tableProps={props} />}
          </For>
        </TableBody>
      </Table>
    </Card>
  );
};
