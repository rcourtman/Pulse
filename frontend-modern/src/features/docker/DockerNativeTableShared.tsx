import { type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell } from '@/components/shared/Table';
import { getSimpleStatusIndicator, type StatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  formatPlatformTableBytesValue,
  getPlatformTableCellClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import type { DockerResourceStatusFilter } from './dockerPageModel';

export type DockerNativeTableProps = {
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
  externalSearch?: () => string;
  externalStatus?: () => DockerResourceStatusFilter;
};

export const dockerTextValue = (value: string | undefined): string => asTrimmedString(value) || '—';

export const dockerNumberValue = (value: number | undefined): JSX.Element =>
  typeof value === 'number' ? <span class="tabular-nums">{value}</span> : <span>—</span>;

export const dockerByteValue = (value: number | undefined): string =>
  formatPlatformTableBytesValue(value);

export const dockerCpuValue = (nanoCpus: number | undefined): string => {
  if (typeof nanoCpus !== 'number' || nanoCpus <= 0) return '—';
  const cpus = nanoCpus / 1_000_000_000;
  return cpus >= 10 ? `${Math.round(cpus)}` : cpus.toFixed(cpus % 1 === 0 ? 0 : 1);
};

export const dockerJoinValues = (
  values: readonly (string | undefined)[] | undefined,
  empty = '—',
): string => {
  const joined = (values ?? [])
    .map((value) => asTrimmedString(value))
    .filter((value): value is string => typeof value === 'string' && value.length > 0)
    .join(', ');
  return joined || empty;
};

export const dockerLabelsSummary = (labels: Record<string, string> | undefined): string => {
  if (!labels || Object.keys(labels).length === 0) return '—';
  return Object.entries(labels)
    .slice(0, 3)
    .map(([key, value]) => `${key}=${value}`)
    .join(', ');
};

export const dockerHostName = (resource: Resource): string =>
  dockerTextValue(resource.docker?.hostname);

export const dockerResourceName = (resource: Resource): string =>
  asTrimmedString(resource.name) || asTrimmedString(resource.displayName) || resource.id;

export const DockerResourceNameCell: Component<{
  resource: Resource;
  // When set, the row pulls its StatusDot variant and tooltip text from a
  // domain-specific mapper (mapDockerContainerStatus, mapDockerTaskStatus,
  // etc.) rather than from the generic resource.status triad.
  indicator?: StatusIndicator;
  detailToggle?: JSX.Element;
}> = (props) => {
  const resolvedIndicator = (): StatusIndicator =>
    props.indicator ?? getSimpleStatusIndicator(props.resource.status);
  const name = () => dockerResourceName(props.resource);

  return (
    <TableCell class={getPlatformTableCellClassForKind('name')}>
      <div class="flex min-w-0 items-center gap-2">
        {props.detailToggle}
        <StatusDot
          size="sm"
          variant={resolvedIndicator().variant}
          title={resolvedIndicator().label}
          ariaHidden
        />
        <span class="truncate font-semibold text-base-content" title={name()}>
          {name()}
        </span>
      </div>
    </TableCell>
  );
};
