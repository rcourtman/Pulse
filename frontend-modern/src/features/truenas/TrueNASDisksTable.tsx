import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  filterPlatformResources,
  getPlatformTableCellClass,
  getPlatformTableHeadClass,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';

// TrueNAS physical disks are SMART-instrumented storage devices, not
// compute hosts. The canonical infrastructure table's CPU / Memory /
// Disk I/O / Uptime columns are conceptually N/A; the operator columns
// are model, serial, type, size, health, temperature, and wearout.
// This bespoke table surfaces those from the canonical `physicalDisk`
// payload already attached to each row.

const formatBytes = (bytes: number | undefined): string => {
  if (!bytes || bytes <= 0) return '—';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let value = bytes;
  let unitIdx = 0;
  while (value >= 1024 && unitIdx < units.length - 1) {
    value /= 1024;
    unitIdx += 1;
  }
  return `${value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)} ${units[unitIdx]}`;
};

const formatTemperature = (celsius: number | undefined): JSX.Element => {
  if (typeof celsius !== 'number' || celsius <= 0) return <span class="text-muted">—</span>;
  return <span class="tabular-nums">{celsius.toFixed(1)}°C</span>;
};

const formatWearout = (wearout: number | undefined): JSX.Element => {
  if (typeof wearout !== 'number' || wearout < 0) return <span class="text-muted">—</span>;
  return <span class="tabular-nums">{wearout.toFixed(0)}%</span>;
};

const healthVariant = (health: string | undefined): 'success' | 'warning' | 'danger' | 'muted' => {
  const normalized = (health || '').trim().toLowerCase();
  if (normalized === 'healthy' || normalized === 'passed' || normalized === 'pass')
    return 'success';
  if (normalized === 'warning' || normalized === 'degraded') return 'warning';
  if (normalized === 'failed' || normalized === 'fail' || normalized === 'critical')
    return 'danger';
  return 'muted';
};

export const TrueNASDisksTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const [search, setSearch] = createSignal('');
  const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');

  const filtered = createMemo(() => filterPlatformResources(props.resources, search(), status()));
  const visible = createMemo(() => filtered().length);
  const total = createMemo(() => props.resources.length);

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={search}
            onSearchChange={setSearch}
            searchPlaceholder="Search disks"
            status={status()}
            onStatusChange={setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={visible()}
            total={total()}
            rowNoun="disks"
          />
        </Show>

        <Show
          when={filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No disks match current filters"
              description="Adjust the search or status filter to see more disks."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Disks'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[880px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={getPlatformTableHeadClass()}>Disk</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Model
                  </TableHead>
                  <TableHead class={getPlatformTableHeadClass()}>Type</TableHead>
                  <TableHead class={getPlatformTableHeadClass('right')}>Size</TableHead>
                  <TableHead class={getPlatformTableHeadClass()}>Health</TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Temp
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass('right')} hidden md:table-cell`}>
                    Wearout
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClass()} hidden md:table-cell`}>
                    Serial
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={filtered()}>
                  {(disk) => {
                    const meta = () => disk.physicalDisk;
                    const name = () => asTrimmedString(disk.name) || disk.id;
                    const model = () => asTrimmedString(meta()?.model) || '—';
                    const type = () => asTrimmedString(meta()?.diskType) || '—';
                    const health = () => asTrimmedString(meta()?.health) || '—';
                    const serial = () => asTrimmedString(meta()?.serial) || '—';
                    return (
                      <TableRow class="text-[11px] sm:text-xs">
                        <TableCell class={getPlatformTableCellClass()}>
                          <span class="truncate font-semibold text-base-content" title={name()}>
                            {name()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden text-base-content md:table-cell`}
                        >
                          <span class="inline-block max-w-[14rem] truncate" title={model()}>
                            {model()}
                          </span>
                        </TableCell>
                        <TableCell class={`${getPlatformTableCellClass()} text-base-content`}>
                          {type()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} text-base-content`}
                        >
                          {formatBytes(meta()?.sizeBytes)}
                        </TableCell>
                        <TableCell class={getPlatformTableCellClass()}>
                          <div class="flex items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={healthVariant(meta()?.health)}
                              title={meta()?.health || 'unknown'}
                              ariaHidden
                            />
                            <span class="text-base-content">{health()}</span>
                          </div>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                        >
                          {formatTemperature(meta()?.temperature)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass('right')} hidden text-base-content md:table-cell`}
                        >
                          {formatWearout(meta()?.wearout)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClass()} hidden font-mono text-[11px] text-base-content md:table-cell`}
                        >
                          <span class="inline-block max-w-[10rem] truncate" title={serial()}>
                            {serial()}
                          </span>
                        </TableCell>
                      </TableRow>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default TrueNASDisksTable;
