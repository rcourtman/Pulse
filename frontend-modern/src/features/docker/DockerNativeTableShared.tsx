import { Show, type Component, type JSX } from 'solid-js';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell } from '@/components/shared/Table';
import { ResourceNameWithWebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import {
  getSimpleStatusIndicator,
  type StatusIndicator,
  type StatusIndicatorVariant,
} from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformTableNumberValue,
  formatPlatformTableBytesValue,
  getPlatformTableCellClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import { RecordOwnerLink } from '@/features/platformPage/RecordOwnerLink';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import type { Resource } from '@/types/resource';
import { getPlatformStatusTextToneClass } from '@/features/platformPage/platformStatusTone';
import type { DockerResourceStatusFilter } from './dockerPageModel';
import { DockerImageReferenceText } from './DockerImageReferenceText';
import { makeDetailTimestampRow } from '@/features/platformPage/platformDetailTimestampRow';

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
  PlatformTableNumberValue({ value });

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

// State words read in their row dot's colour, without a second dot; the
// platform helper owns the tones every platform table shares.
export const dockerStateTextToneClass = (variant: StatusIndicatorVariant | null): string =>
  getPlatformStatusTextToneClass(variant);

export const DockerResourceNameCell: Component<{
  resource: Resource;
  // When set, the row pulls its StatusDot variant and tooltip text from a
  // domain-specific mapper (mapDockerContainerStatus, mapDockerTaskStatus,
  // etc.) rather than from the generic resource.status triad.
  indicator?: StatusIndicator;
  detailToggle?: JSX.Element;
  // Image rows name the resource by its reference, so a narrow cell keeps the
  // repository and tag visible instead of the shared registry prefix.
  imageReference?: boolean;
  // A second line under the name, for a field the layout has no column for.
  detail?: JSX.Element;
}> = (props) => {
  const resolvedIndicator = (): StatusIndicator =>
    props.indicator ?? getSimpleStatusIndicator(props.resource.status);
  const name = () => dockerResourceName(props.resource);
  const nameLink = () => (
    <ResourceNameWithWebInterfaceLink
      name={name()}
      url={props.resource.customUrl}
      class="min-w-0 max-[359px]:[&>a]:hidden"
      nameClass={`${props.imageReference ? 'flex min-w-0' : 'truncate'} font-semibold text-base-content`}
    >
      {props.imageReference ? <DockerImageReferenceText reference={name()} /> : undefined}
    </ResourceNameWithWebInterfaceLink>
  );

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
        <Show when={props.detail} fallback={nameLink()}>
          {(detail) => (
            <div class="flex min-w-0 flex-col">
              {nameLink()}
              {detail()}
            </div>
          )}
        </Show>
      </div>
    </TableCell>
  );
};

// Swarm objects (services, tasks, nodes, secrets, configs) and networks are
// records a Docker host reports, not machines Pulse monitors: their drawer is
// a record panel headed by the object's name and its row's own status dot,
// with a link to the Docker host whose agent reported it.
export const DockerNativeDetailPanel: Component<{
  testId: string;
  resource: Resource;
  status: StatusIndicator;
  summary?: JSX.Element;
  /** Built against the shared relative-time clock so ages keep moving while open. */
  sections: (now: number) => Array<DetailSection | null>;
  onClose: () => void;
  /** Content that matters more than the sections, such as a network's containers. */
  children?: JSX.Element;
}> = (props) => {
  const now = useRelativeTimeNow();
  const hostname = () => asTrimmedString(props.resource.docker?.hostname);
  return (
    <InlineDetailPanel
      testId={props.testId}
      detailFor={props.resource.id}
      title={dockerResourceName(props.resource)}
      status={props.status}
      summary={props.summary}
      actions={
        <Show when={hostname()}>
          {(name) => (
            <RecordOwnerLink
              subject={{
                id: asTrimmedString(props.resource.docker?.hostSourceId) || name(),
                name: name(),
                type: 'docker-host',
              }}
            />
          )}
        </Show>
      }
      sections={compactDetailSections(props.sections(now()))}
      onClose={props.onClose}
    >
      {props.children}
    </InlineDetailPanel>
  );
};

// Docker reports an unset time as the zero time (0001-01-01), so stamps
// before 2000 are omitted rather than shown as a date.
export const dockerDetailTimestampRow = (
  label: string,
  timestamp: string | undefined,
  now: number,
): DetailRow | null => makeDetailTimestampRow(label, timestamp, now, { minYear: 2000 });

// Every label, one row each with its key above its value: keys such as
// com.docker.stack.namespace are too long for the label column, and the
// table's summary keeps only the first few.
export const dockerLabelsSection = (
  label: string,
  labels: Record<string, string> | undefined,
): DetailSection | null => {
  const entries = Object.entries(labels ?? {}).sort(([a], [b]) => a.localeCompare(b));
  if (entries.length === 0) return null;
  return {
    label: `${label} (${entries.length})`,
    rows: compactDetailRows(
      entries.map(([key, value]) =>
        makeDetailRow(key, asTrimmedString(value) || '(empty)', { layout: 'stacked', wrap: true }),
      ),
    ),
  };
};

// A detail value behind the row dot's verdict reads in the dot's colour when
// the verdict needs a look; healthy and idle values stay plain.
export const dockerDetailToneForVariant = (
  variant: StatusIndicatorVariant | null | undefined,
): DetailValueTone | undefined =>
  variant === 'danger' ? 'danger' : variant === 'warning' ? 'warning' : undefined;

export const dockerReportedBySection = (resource: Resource): DetailSection | null => {
  const rows = compactDetailRows([
    makeDetailRow('Reported by', asTrimmedString(resource.docker?.hostname), { wrap: true }),
    makeDetailRow('Swarm', asTrimmedString(resource.docker?.swarm?.clusterName), { wrap: true }),
  ]);
  return rows.length > 0 ? { label: 'Source', rows } : null;
};
