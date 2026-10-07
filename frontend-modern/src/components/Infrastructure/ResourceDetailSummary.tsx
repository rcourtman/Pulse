import { For, Show, type Component, type JSX } from 'solid-js';
import type { Resource } from '@/types/resource';
import { isContainerUpdatePinned } from '@/components/shared/containerUpdateBadgeModel';
import { DetailSectionTable } from '@/components/shared/DetailSectionTable';
import {
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailRow,
  type DetailSection,
} from '@/components/shared/detailSectionModel';
import { TagBadges } from '@/components/shared/TagBadges';
import { formatBytes, formatRelativeTime, formatUptime } from '@/utils/format';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { getDockerImageRegistryLink } from '@/features/docker/dockerImageReference';
import {
  getDockerContainerHealthCheckPresentation,
  getDockerContainerRunStatePresentation,
  getDockerContainerUptimeSeconds,
  type DockerContainerStatePresentation,
} from '@/features/docker/dockerContainerStatePresentation';
import { formatInteger } from './resourceDetailMappers';
import { buildKubernetesControllerSection } from './resourceDetailDrawerKubernetesModel';
import type { UseResourceDetailDrawerStateResult } from './useResourceDetailDrawerState';

interface ResourceSummaryPresentationProps {
  resource: Resource;
  drawer: UseResourceDetailDrawerStateResult;
  showPlatformId: boolean;
  content?: 'all' | 'overview' | 'technical';
  dataTestId?: string;
}

const dockerContainerMeta = (resource: Resource): NonNullable<Resource['docker']> | null => {
  if (resource.type !== 'app-container') return null;
  const docker = resource.docker;
  if (!docker) return null;
  if (
    !docker.containerId &&
    !docker.containerState &&
    !docker.image &&
    !docker.updateStatus &&
    !docker.startedAt &&
    !docker.finishedAt &&
    !docker.blockIo &&
    !docker.podman
  ) {
    return null;
  }
  return docker;
};

const trimmedDockerValue = (value: string | undefined): string => (value || '').trim();

const composeLabelValue = (
  labels: Record<string, string> | undefined,
  suffix: 'project' | 'service',
): string =>
  (
    labels?.[`com.docker.compose.${suffix}`] ||
    labels?.[`io.podman.compose.${suffix}`] ||
    ''
  ).trim();

const dockerTimestampMillis = (value: string | undefined): number | null => {
  const raw = trimmedDockerValue(value);
  if (!raw) return null;
  const parsed = Date.parse(raw);
  return Number.isFinite(parsed) ? parsed : null;
};

const dockerByteTotal = (value: number | undefined): number | null =>
  typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : null;

const richRow = (
  label: string,
  value: string,
  valueContent: JSX.Element,
  options: Pick<DetailRow, 'title' | 'tone' | 'wrap' | 'valueClass'> = {},
): DetailRow => ({ label, value, valueContent, ...options });

// A Docker container's runtime rows match its table row: Docker's own state,
// the health check on a row of its own while it runs, and uptime only for the
// current run (Docker keeps the last value on a stopped container).
const dockerRuntimeRow = (
  label: string,
  presentation: DockerContainerStatePresentation | undefined,
) =>
  presentation
    ? makeDetailRow(label, presentation.label, {
        title: presentation.title,
        tone: presentation.tone ?? 'default',
      })
    : null;

const dockerSection = (docker: NonNullable<Resource['docker']>, now: number): DetailSection => {
  const labels = docker.labels ?? {};
  const labelEntries = Object.entries(labels);
  const createdAt = dockerTimestampMillis(docker.createdAt);
  const startedAt = dockerTimestampMillis(docker.startedAt);
  const finishedAt = dockerTimestampMillis(docker.finishedAt);
  const podmanPodName = trimmedDockerValue(docker.podman?.podName);
  const podmanPodId = trimmedDockerValue(docker.podman?.podId);
  const composeProject =
    trimmedDockerValue(docker.podman?.composeProject) || composeLabelValue(labels, 'project');
  const composeService =
    trimmedDockerValue(docker.podman?.composeService) || composeLabelValue(labels, 'service');
  const autoUpdatePolicy = trimmedDockerValue(docker.podman?.autoUpdatePolicy);
  const userNamespace = trimmedDockerValue(docker.podman?.userNamespace);
  const blockReadBytes = dockerByteTotal(docker.blockIo?.readBytes);
  const blockWriteBytes = dockerByteTotal(docker.blockIo?.writeBytes);
  const update = docker.updateStatus;
  const updateState = update
    ? isContainerUpdatePinned(update)
      ? 'Pinned digest'
      : trimmedDockerValue(update.error)
        ? 'Check failed'
        : update.updateAvailable
          ? 'Available'
          : 'Current'
    : '';
  const image = trimmedDockerValue(docker.image);
  const imageRegistryLink = update?.updateAvailable
    ? getDockerImageRegistryLink(docker.image)
    : null;

  const rows = compactDetailRows([
    makeDetailRow('Image', image, { title: image }),
    makeDetailRow('Image update', updateState, {
      title: update?.error,
      tone: update?.updateAvailable ? 'accent' : update?.error ? 'danger' : 'default',
    }),
    makeDetailRow('Current digest', trimmedDockerValue(update?.currentDigest), {
      valueClass: 'break-all font-mono text-[10px]',
      wrap: true,
    }),
    makeDetailRow('Target digest', trimmedDockerValue(update?.latestDigest), {
      valueClass: 'break-all font-mono text-[10px]',
      wrap: true,
    }),
    imageRegistryLink
      ? richRow(
          'Release information',
          imageRegistryLink.label,
          <a
            href={imageRegistryLink.href}
            target="_blank"
            rel="noopener noreferrer"
            class="text-sky-700 hover:underline dark:text-sky-300"
          >
            {imageRegistryLink.label}
          </a>,
        )
      : null,
    typeof docker.restartCount === 'number'
      ? makeDetailRow('Restarts', formatInteger(docker.restartCount), {
          tone: docker.restartCount > 5 ? 'danger' : 'default',
        })
      : null,
    createdAt
      ? makeDetailRow('Created', formatRelativeTime(createdAt, { now }), {
          title: new Date(createdAt).toLocaleString(),
        })
      : null,
    startedAt
      ? makeDetailRow('Started', formatRelativeTime(startedAt, { now }), {
          title: new Date(startedAt).toLocaleString(),
        })
      : null,
    finishedAt
      ? makeDetailRow('Finished', formatRelativeTime(finishedAt, { now }), {
          title: new Date(finishedAt).toLocaleString(),
        })
      : null,
    makeDetailRow('Podman pod', podmanPodName),
    makeDetailRow('Podman pod ID', podmanPodId),
    typeof docker.podman?.infra === 'boolean'
      ? makeDetailRow('Podman infra', docker.podman.infra ? 'Yes' : 'No')
      : null,
    makeDetailRow('Compose project', composeProject),
    makeDetailRow('Compose service', composeService),
    makeDetailRow('Auto-update', autoUpdatePolicy),
    makeDetailRow('User namespace', userNamespace),
    blockReadBytes !== null ? makeDetailRow('Block I/O read', formatBytes(blockReadBytes)) : null,
    blockWriteBytes !== null
      ? makeDetailRow('Block I/O write', formatBytes(blockWriteBytes))
      : null,
    labelEntries.length > 0
      ? richRow(
          'Labels',
          labelEntries.map(([key, value]) => (value ? `${key}: ${value}` : key)).join(', '),
          <div class="flex flex-wrap gap-1">
            <For each={labelEntries}>
              {([key, value]) => (
                <span
                  class="inline-flex max-w-full items-center truncate rounded-sm bg-surface-alt px-1.5 py-0.5 text-[10px]"
                  title={value ? `${key}: ${value}` : key}
                >
                  {key}
                  <Show when={value}>: {value}</Show>
                </span>
              )}
            </For>
          </div>,
          { wrap: true },
        )
      : null,
  ]);

  return { label: 'Container', rows, testId: 'resource-docker-container-section' };
};

export const InlineResourceSummaryTables: Component<ResourceSummaryPresentationProps> = (props) => {
  // An open drawer keeps its rows while the resource's timestamps stay the
  // same, so container and controller ages read the shared clock to keep
  // moving.
  const now = useRelativeTimeNow();
  const sections = (): DetailSection[] => {
    const docker = dockerContainerMeta(props.resource);
    // An offline resource has no current uptime: the retained value is from
    // its last report, and the Last seen row already says when that was.
    const uptimeSeconds = docker
      ? getDockerContainerUptimeSeconds(props.resource)
      : props.resource.status === 'offline'
        ? undefined
        : props.resource.uptime;
    const identityRows = compactDetailRows([
      ...props.drawer.primaryIdentityRows().map((row) => makeDetailRow(row.label, row.value)),
      props.showPlatformId ? makeDetailRow('Platform ID', props.resource.platformId) : null,
      props.drawer.identityIpValues().length > 0
        ? richRow(
            'IP Addresses',
            props.drawer.identityIpValues().join(', '),
            <div class="flex flex-wrap gap-1">
              <For each={props.drawer.identityIpValues()}>
                {(ip) => (
                  <span
                    class="inline-flex items-center rounded-sm bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900/25 dark:text-blue-200"
                    title={ip}
                  >
                    {ip}
                  </span>
                )}
              </For>
            </div>,
          )
        : null,
      props.resource.tags?.length
        ? richRow(
            'Tags',
            props.resource.tags.join(', '),
            <TagBadges tags={props.resource.tags} maxVisible={6} />,
          )
        : null,
      props.drawer.identityAliasValues().length > 0
        ? richRow(
            'Aliases',
            props.drawer.identityAliasValues().join(', '),
            <div class="flex flex-wrap gap-1">
              <For each={props.drawer.aliasPreviewValues()}>
                {(value) => (
                  <span
                    class="inline-flex items-center rounded-sm bg-surface-alt px-1.5 py-0.5 text-[10px]"
                    title={value}
                  >
                    {value}
                  </span>
                )}
              </For>
              <Show when={props.drawer.hasAliasOverflow()}>
                <span class="inline-flex items-center rounded-sm bg-surface-alt px-1.5 py-0.5 text-[10px] text-muted">
                  +
                  {props.drawer.identityAliasValues().length -
                    props.drawer.aliasPreviewValues().length}
                </span>
              </Show>
            </div>,
          )
        : null,
      !props.drawer.identityCardHasRichData()
        ? makeDetailRow('Metadata', 'No identity metadata yet.', { tone: 'muted' })
        : null,
    ]);

    return compactDetailSections([
      props.content !== 'technical'
        ? {
            label: 'Operator context',
            testId: 'resource-current-state-section',
            rows: compactDetailRows([
              props.drawer.sourceSummary()
                ? makeDetailRow('Pulse coverage', props.drawer.sourceSummary()?.label, {
                    title: props.drawer.sourceSummary()?.title,
                    valueClass: props.drawer.sourceSummary()?.className,
                  })
                : null,
              makeDetailRow('Primary IP', props.drawer.identityIpValues()[0]),
            ]),
          }
        : null,
      props.content !== 'overview'
        ? {
            label: 'Runtime context',
            testId: 'resource-runtime-context-section',
            rows: compactDetailRows([
              docker
                ? dockerRuntimeRow(
                    'Observed state',
                    getDockerContainerRunStatePresentation(props.resource),
                  )
                : makeDetailRow('Observed state', props.resource.status || 'unknown', {
                    valueClass: 'capitalize',
                  }),
              docker
                ? dockerRuntimeRow(
                    'Health check',
                    getDockerContainerHealthCheckPresentation(props.resource),
                  )
                : null,
              uptimeSeconds ? makeDetailRow('Uptime', formatUptime(uptimeSeconds)) : null,
              props.resource.lastSeen
                ? makeDetailRow('Last seen', props.drawer.lastSeen() || '—', {
                    title: props.drawer.lastSeenAbsolute(),
                  })
                : null,
            ]),
          }
        : null,
      props.content !== 'overview' && docker ? dockerSection(docker, now()) : null,
      props.content !== 'overview' ? buildKubernetesControllerSection(props.resource, now()) : null,
      props.content !== 'overview'
        ? {
            label: 'Identity',
            testId: 'resource-identity-section',
            rows: identityRows,
          }
        : null,
    ]);
  };

  return (
    <DetailSectionTable
      sections={sections()}
      dataTestId={props.dataTestId ?? 'resource-summary-section'}
    />
  );
};
