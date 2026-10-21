import { For, Show, getOwner, runWithOwner, type Component, type JSX } from 'solid-js';
import type { Resource } from '@/types/resource';
import {
  isContainerUpdateLocalImage,
  isContainerUpdatePinned,
  isContainerUpdateUncheckable,
} from '@/components/shared/containerUpdateBadgeModel';
import { DetailSectionTable } from '@/components/shared/DetailSectionTable';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
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
import { getDockerContainerUpdateAction } from '@/features/docker/dockerContainerUpdateAction';
import {
  dockerContainerRestartAttentionLabel,
  dockerContainerRestartCountTitle,
} from '@/features/docker/dockerPageModel';
import { UpdateButton } from '@/components/shared/ContainerUpdateBadge';
import {
  getDockerContainerHealthCheckPresentation,
  getDockerContainerRunStatePresentation,
  getDockerContainerUptimeSeconds,
  type DockerContainerStatePresentation,
} from '@/features/docker/dockerContainerStatePresentation';
import { formatInteger } from './resourceDetailMappers';
import { buildKubernetesControllerSection } from './resourceDetailDrawerKubernetesModel';
import type { UseResourceDetailDrawerStateResult } from './useResourceDetailDrawerState';
import { getResourceMemoryObservationPresentation } from './resourceDetailDrawerMetricsHistoryModel';

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

// The drawer is where a truncated table cell sends the operator for the full
// value, so identifying values (IDs, names, image references, addresses) wrap
// here instead of ending in an ellipsis. Their distinguishing part is often at
// the end (a pod suffix, a hash, an image tag), and a phone cannot hover the
// title to read it.
const identityRow = (label: string, value?: string | null) =>
  makeDetailRow(label, value, { wrap: true });

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

// An available update's next steps: the containers table's own update control
// and where to see the new image. The drawer rebuilds its inventory rows on
// every refresh and clock tick, so this is created once and reused; a
// recreated button would drop its state, including an open review dialog.
const ContainerUpdateNextSteps: Component<{ resource: Resource }> = (props) => {
  const action = () => getDockerContainerUpdateAction(props.resource);
  const registryLink = () => getDockerImageRegistryLink(props.resource.docker?.image);
  // One wrapping flex box (overriding the drawer row's inline children), so on
  // a phone the controls wrap inside the card and keep their own tap sizes.
  return (
    <span class="!inline-flex max-w-full flex-wrap items-center gap-x-2 gap-y-1">
      <span>Available</span>
      <Show when={action()}>
        {(update) => (
          <UpdateButton
            agentId={update().agentId}
            containerId={update().containerId}
            containerName={update().containerName}
            updateStatus={update().updateStatus}
            resourceId={update().resourceId}
            actionReadiness={update().actionReadiness}
            updateNotOfferedReason={update().updateNotOfferedReason}
            touchTarget
          />
        )}
      </Show>
      <Show when={registryLink()}>
        {(link) => (
          <a
            href={link().href}
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center font-normal text-sky-700 hover:underline max-sm:min-h-11 dark:text-sky-300"
          >
            {link().label}
          </a>
        )}
      </Show>
    </span>
  );
};

const dockerSection = (
  resource: Resource,
  docker: NonNullable<Resource['docker']>,
  now: number,
  updateNextSteps: () => JSX.Element,
): DetailSection => {
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
      : isContainerUpdateLocalImage(update)
        ? 'Local image'
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
    identityRow('Image', image),
    makeDetailRow('Image update', updateState, {
      title: update?.error,
      tone: update?.updateAvailable
        ? 'accent'
        : update?.error && !isContainerUpdateUncheckable(update)
          ? 'danger'
          : 'default',
      // An available update is something to act on: drawers lift it in front.
      lift: update?.updateAvailable === true,
      // An available update offers its next steps where it is stated.
      valueContent: update?.updateAvailable ? updateNextSteps() : undefined,
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
          // The row dot's restart verdict: running, but worth a look.
          tone: dockerContainerRestartAttentionLabel(resource) ? 'warning' : 'default',
          title: dockerContainerRestartCountTitle(docker.restartCount),
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
    identityRow('Podman pod', podmanPodName),
    identityRow('Podman pod ID', podmanPodId),
    typeof docker.podman?.infra === 'boolean'
      ? makeDetailRow('Podman infra', docker.podman.infra ? 'Yes' : 'No')
      : null,
    identityRow('Compose project', composeProject),
    identityRow('Compose service', composeService),
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
                <MetadataBadge
                  size="xs"
                  shape="rounded"
                  wrap
                  title={value ? `${key}: ${value}` : key}
                >
                  {key}
                  <Show when={value}>: {value}</Show>
                </MetadataBadge>
              )}
            </For>
          </div>,
          { wrap: true },
        )
      : null,
  ]);

  return { label: 'Container', rows, testId: 'resource-docker-container-section' };
};

// A resource that is not reporting must say so in front, not only in Details:
// offline is danger-toned (with when it was last seen), and a state Pulse
// cannot vouch for is warning-toned, so the drawer lifts either.
const observedStateRow = (status: string | undefined, lastSeen: string | undefined) => {
  const state = (status || 'unknown').trim().toLowerCase();
  const label = state.charAt(0).toUpperCase() + state.slice(1);
  const offline = state === 'offline';
  return makeDetailRow(
    'Observed state',
    offline && lastSeen ? `${label} · last seen ${lastSeen}` : label,
    {
      tone: offline
        ? 'danger'
        : ['unknown', 'degraded', 'stale', 'warning'].includes(state)
          ? 'warning'
          : undefined,
    },
  );
};

/**
 * The resource drawer's inventory as detail sections: operator context,
 * runtime, container, controller and identity. The drawer Overview lifts any
 * warning row in front and keeps the rest under Details; the table form below
 * renders them all.
 */
export const useResourceSummarySections = (
  props: Omit<ResourceSummaryPresentationProps, 'dataTestId'>,
): (() => DetailSection[]) => {
  // An open drawer keeps its rows while the resource's timestamps stay the
  // same, so container and controller ages read the shared clock to keep
  // moving.
  const now = useRelativeTimeNow();
  // Created on first need under the drawer's owner, then reused by every
  // rebuild of the rows below.
  const owner = getOwner();
  let nextSteps: JSX.Element | undefined;
  const updateNextSteps = () =>
    (nextSteps ??= runWithOwner(owner, () => (
      <ContainerUpdateNextSteps resource={props.resource} />
    )));
  return (): DetailSection[] => {
    const docker = dockerContainerMeta(props.resource);
    // An offline resource has no current uptime: the retained value is from
    // its last report, and the Last seen row already says when that was.
    const uptimeSeconds = docker
      ? getDockerContainerUptimeSeconds(props.resource)
      : props.resource.status === 'offline'
        ? undefined
        : props.resource.uptime;
    const identityRows = compactDetailRows([
      ...props.drawer.primaryIdentityRows().map((row) => identityRow(row.label, row.value)),
      props.showPlatformId ? identityRow('Platform ID', props.resource.platformId) : null,
      props.drawer.identityIpValues().length > 0
        ? richRow(
            'IP Addresses',
            props.drawer.identityIpValues().join(', '),
            <div class="flex flex-wrap gap-1">
              <For each={props.drawer.identityIpValues()}>
                {(ip) => (
                  <MetadataBadge tone="info" size="xs" shape="rounded" wrap title={ip}>
                    {ip}
                  </MetadataBadge>
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
                  <MetadataBadge size="xs" shape="rounded" wrap title={value}>
                    {value}
                  </MetadataBadge>
                )}
              </For>
              <Show when={props.drawer.hasAliasOverflow()}>
                <MetadataBadge tone="muted" size="xs" shape="rounded">
                  +
                  {props.drawer.identityAliasValues().length -
                    props.drawer.aliasPreviewValues().length}
                </MetadataBadge>
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
              identityRow('Primary IP', props.drawer.identityIpValues()[0]),
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
                : observedStateRow(props.resource.status, props.drawer.lastSeen()),
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
      props.content !== 'overview' && docker
        ? dockerSection(props.resource, docker, now(), updateNextSteps)
        : null,
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
};

/**
 * The facts line under a resource drawer's charts: what the row does not
 * already say. A container states its image and Compose project and service
 * (a phone row hides the image); a host its
 * CPU count, the RAM and disk the row's percentages are of, and its primary
 * address. An offline resource states sizes only, not used amounts that would
 * read as live.
 */
export const getResourceDrawerFacts = (resource: Resource, primaryIp?: string): string[] => {
  const docker = dockerContainerMeta(resource);
  if (docker) {
    const labels = docker.labels;
    const project =
      trimmedDockerValue(docker.podman?.composeProject) || composeLabelValue(labels, 'project');
    const service =
      trimmedDockerValue(docker.podman?.composeService) || composeLabelValue(labels, 'service');
    return [
      trimmedDockerValue(docker.image),
      project && service ? `Compose ${project} / ${service}` : project ? `Compose ${project}` : '',
      primaryIp ?? '',
    ].filter(Boolean);
  }
  // Used amounts only from a current reading: an offline or unvouched resource,
  // or a retained memory observation, states its size alone.
  const status = (resource.status || '').trim().toLowerCase();
  const reporting = status === 'online' || status === 'running';
  const size = (
    metric: Resource['memory'],
    label: string,
    current = !metric?.observation || metric.observation.state === 'current',
  ) => {
    const total = metric?.total ?? 0;
    if (!(total > 0)) return '';
    const used = metric?.used;
    return reporting && current && typeof used === 'number' && used >= 0
      ? `${formatBytes(used)} of ${formatBytes(total)} ${label}`
      : `${formatBytes(total)} ${label}`;
  };
  const cpus = resource.agent?.cpuCount ?? 0;
  const memoryReading = getResourceMemoryObservationPresentation(resource);
  return [
    cpus > 0 ? `${cpus} logical CPUs` : '',
    size(resource.memory, 'RAM', !memoryReading || memoryReading.state === 'current'),
    size(resource.disk, 'disk'),
    primaryIp ?? '',
  ].filter(Boolean);
};

export const InlineResourceSummaryTables: Component<ResourceSummaryPresentationProps> = (props) => {
  const sections = useResourceSummarySections(props);

  return (
    <DetailSectionTable
      sections={sections()}
      dataTestId={props.dataTestId ?? 'resource-summary-section'}
    />
  );
};
