import { For, Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import { SearchInput } from '@/components/shared/SearchInput';
import { StatusDot } from '@/components/shared/StatusDot';
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
  filterPlatformResources,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';

// TrueNAS physical disks are SMART-instrumented storage devices, not
// compute hosts. The canonical infrastructure table's CPU / Memory /
// Disk I/O / Uptime columns are conceptually N/A; the operator columns
// are model, serial, type, size, health, temperature, and wearout.
// This bespoke table surfaces those from the canonical `physicalDisk`
// payload already attached to each row.

const STATUS_FILTER_OPTIONS: FilterOption<PlatformResourceStatusFilter>[] = [
  { value: 'all', label: 'All' },
  { value: 'online', label: 'Healthy' },
  { value: 'degraded', label: 'Degraded' },
  { value: 'offline', label: 'Offline' },
];

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

const healthVariant = (
  health: string | undefined,
): 'success' | 'warning' | 'danger' | 'muted' => {
  const normalized = (health || '').trim().toLowerCase();
  if (normalized === 'healthy' || normalized === 'passed' || normalized === 'pass') return 'success';
  if (normalized === 'warning' || normalized === 'degraded') return 'warning';
  if (normalized === 'failed' || normalized === 'fail' || normalized === 'critical') return 'danger';
  return 'muted';
};

export const TrueNASDisksTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
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
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title={props.emptyTitle}
            description={props.emptyDescription}
          />
        </Card>
      }
    >
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
          <div class="min-w-[200px] flex-1 sm:max-w-xs">
            <SearchInput
              value={search}
              onChange={setSearch}
              placeholder="Search disks"
            />
          </div>
          <FilterButtonGroup
            options={STATUS_FILTER_OPTIONS}
            value={status()}
            onChange={setStatus}
          />
          <span class="ml-auto whitespace-nowrap text-xs font-medium text-muted">
            <Show when={visible() !== total()} fallback={<>{total()} disks</>}>
              {visible()} of {total()} disks
            </Show>
          </span>
        </div>

        <Show
          when={filtered().length > 0}
          fallback={
            <Card padding="lg">
              <EmptyState
                icon={props.emptyIcon}
                title="No disks match current filters"
                description="Adjust the search or status filter to see more disks."
              />
            </Card>
          }
        >
          <Card padding="none" tone="card" class="overflow-hidden">
            <Table class="w-full min-w-[880px] border-collapse text-xs">
              <TableHeader class="bg-surface-alt text-muted border-b border-border">
                <TableRow class="text-left text-[10px] uppercase tracking-wide">
                  <TableHead class="px-3 py-2 font-medium">Disk</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Model</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Type</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Size</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Health</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Temp</TableHead>
                  <TableHead class="px-3 py-2 font-medium text-right">Wearout</TableHead>
                  <TableHead class="px-3 py-2 font-medium">Serial</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border-subtle">
                <For each={filtered()}>
                  {(disk) => {
                    const meta = () => disk.physicalDisk;
                    const name = () => asTrimmedString(disk.name) || disk.id;
                    const model = () => asTrimmedString(meta()?.model) || '—';
                    const type = () => asTrimmedString(meta()?.diskType) || '—';
                    const health = () => asTrimmedString(meta()?.health) || '—';
                    const serial = () => asTrimmedString(meta()?.serial) || '—';
                    return (
                      <TableRow class="hover:bg-surface-hover">
                        <TableCell class="px-3 py-2">
                          <span class="font-semibold text-base-content truncate" title={name()}>
                            {name()}
                          </span>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">
                          <span class="truncate inline-block max-w-[14rem]" title={model()}>
                            {model()}
                          </span>
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content">{type()}</TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatBytes(meta()?.sizeBytes)}
                        </TableCell>
                        <TableCell class="px-3 py-2">
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
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatTemperature(meta()?.temperature)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-right text-base-content">
                          {formatWearout(meta()?.wearout)}
                        </TableCell>
                        <TableCell class="px-3 py-2 text-base-content font-mono text-[11px]">
                          <span class="truncate inline-block max-w-[10rem]" title={serial()}>
                            {serial()}
                          </span>
                        </TableCell>
                      </TableRow>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </Card>
        </Show>
      </div>
    </Show>
  );
};

export default TrueNASDisksTable;
