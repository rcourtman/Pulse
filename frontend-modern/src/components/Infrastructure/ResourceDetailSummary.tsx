import { For, Show } from 'solid-js';
import type { Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import { isContainerUpdatePinned } from '@/components/shared/containerUpdateBadgeModel';
import { TagBadges } from '@/components/shared/TagBadges';
import { formatBytes, formatRelativeTime, formatUptime } from '@/utils/format';
import { formatInteger } from './resourceDetailMappers';
import type { UseResourceDetailDrawerStateResult } from './useResourceDetailDrawerState';
import { getDockerImageRegistryLink } from '@/features/docker/dockerImageReference';

interface ResourceSummaryPresentationProps {
  resource: Resource;
  drawer: UseResourceDetailDrawerStateResult;
  showPlatformId: boolean;
  content?: 'all' | 'overview' | 'technical';
  dataTestId?: string;
}

// Docker / Podman containers carry runtime facts (image, restart count,
// created-at, compose membership, labels) that no generic summary row
// surfaces. v5 rendered all of these in the container drawer; the unified
// payload still ships them on resource.docker.
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

const DockerContainerSummarySection: Component<{ docker: NonNullable<Resource['docker']> }> = (
  props,
) => {
  const labels = () => props.docker.labels ?? {};
  const labelEntries = () => Object.entries(labels());
  const createdAt = () => dockerTimestampMillis(props.docker.createdAt);
  const startedAt = () => dockerTimestampMillis(props.docker.startedAt);
  const finishedAt = () => dockerTimestampMillis(props.docker.finishedAt);
  const podman = () => props.docker.podman;
  const podmanPodName = () => trimmedDockerValue(podman()?.podName);
  const podmanPodId = () => trimmedDockerValue(podman()?.podId);
  const composeProject = () =>
    trimmedDockerValue(podman()?.composeProject) || composeLabelValue(labels(), 'project');
  const composeService = () =>
    trimmedDockerValue(podman()?.composeService) || composeLabelValue(labels(), 'service');
  const autoUpdatePolicy = () => trimmedDockerValue(podman()?.autoUpdatePolicy);
  const userNamespace = () => trimmedDockerValue(podman()?.userNamespace);
  const blockReadBytes = () => dockerByteTotal(props.docker.blockIo?.readBytes);
  const blockWriteBytes = () => dockerByteTotal(props.docker.blockIo?.writeBytes);
  const restartCount = () => props.docker.restartCount;
  const updateStatus = () => props.docker.updateStatus;
  const imageRegistryLink = () =>
    updateStatus()?.updateAvailable ? getDockerImageRegistryLink(props.docker.image) : null;
  const updateState = () => {
    const update = updateStatus();
    if (!update) return '';
    if (isContainerUpdatePinned(update)) return 'Pinned digest';
    if (trimmedDockerValue(update.error)) return 'Check failed';
    return update.updateAvailable ? 'Available' : 'Current';
  };

  return (
    <tbody
      data-testid="resource-docker-container-section"
      class="divide-y divide-border border-t border-border"
    >
      <tr class="bg-surface-alt">
        <th
          colspan="2"
          class="px-2 py-1 text-left text-[10px] font-semibold uppercase tracking-wide text-muted"
        >
          Container
        </th>
      </tr>
      <Show when={(props.docker.image || '').trim()}>
        <tr>
          <td class="w-[38%] px-2 py-1 text-muted">Image</td>
          <td class="px-2 py-1 text-right font-medium text-base-content" title={props.docker.image}>
            <span class="block truncate">{props.docker.image}</span>
          </td>
        </tr>
      </Show>
      <Show when={updateState()}>
        <tr>
          <td class="px-2 py-1 text-muted">Image update</td>
          <td
            class={`px-2 py-1 text-right font-medium ${
              updateStatus()?.updateAvailable
                ? 'text-sky-700 dark:text-sky-300'
                : updateStatus()?.error
                  ? 'text-red-600 dark:text-red-400'
                  : 'text-base-content'
            }`}
            title={updateStatus()?.error}
          >
            {updateState()}
          </td>
        </tr>
      </Show>
      <Show when={trimmedDockerValue(updateStatus()?.currentDigest)}>
        {(digest) => (
          <tr>
            <td class="px-2 py-1 text-muted">Current digest</td>
            <td
              class="break-all px-2 py-1 text-right font-mono text-[10px] font-medium text-base-content"
              title={digest()}
            >
              {digest()}
            </td>
          </tr>
        )}
      </Show>
      <Show when={trimmedDockerValue(updateStatus()?.latestDigest)}>
        {(digest) => (
          <tr>
            <td class="px-2 py-1 text-muted">Target digest</td>
            <td
              class="break-all px-2 py-1 text-right font-mono text-[10px] font-medium text-base-content"
              title={digest()}
            >
              {digest()}
            </td>
          </tr>
        )}
      </Show>
      <Show when={imageRegistryLink()}>
        {(link) => (
          <tr>
            <td class="px-2 py-1 text-muted">Release information</td>
            <td class="px-2 py-1 text-right font-medium">
              <a
                href={link().href}
                target="_blank"
                rel="noopener noreferrer"
                class="text-sky-700 hover:underline dark:text-sky-300"
              >
                {link().label}
              </a>
            </td>
          </tr>
        )}
      </Show>
      <Show when={typeof restartCount() === 'number'}>
        <tr>
          <td class="px-2 py-1 text-muted">Restarts</td>
          <td
            class={`px-2 py-1 text-right font-medium ${
              (restartCount() ?? 0) > 5 ? 'text-red-600 dark:text-red-400' : 'text-base-content'
            }`}
          >
            {formatInteger(restartCount())}
          </td>
        </tr>
      </Show>
      <Show when={createdAt()}>
        {(created) => (
          <tr>
            <td class="px-2 py-1 text-muted">Created</td>
            <td
              class="px-2 py-1 text-right font-medium text-base-content"
              title={new Date(created()).toLocaleString()}
            >
              {formatRelativeTime(created())}
            </td>
          </tr>
        )}
      </Show>
      <Show when={startedAt()}>
        {(started) => (
          <tr>
            <td class="px-2 py-1 text-muted">Started</td>
            <td
              class="px-2 py-1 text-right font-medium text-base-content"
              title={new Date(started()).toLocaleString()}
            >
              {formatRelativeTime(started())}
            </td>
          </tr>
        )}
      </Show>
      <Show when={finishedAt()}>
        {(finished) => (
          <tr>
            <td class="px-2 py-1 text-muted">Finished</td>
            <td
              class="px-2 py-1 text-right font-medium text-base-content"
              title={new Date(finished()).toLocaleString()}
            >
              {formatRelativeTime(finished())}
            </td>
          </tr>
        )}
      </Show>
      <Show when={podmanPodName()}>
        <tr>
          <td class="px-2 py-1 text-muted">Podman pod</td>
          <td class="px-2 py-1 text-right font-medium text-base-content" title={podmanPodName()}>
            <span class="block truncate">{podmanPodName()}</span>
          </td>
        </tr>
      </Show>
      <Show when={podmanPodId()}>
        <tr>
          <td class="px-2 py-1 text-muted">Podman pod ID</td>
          <td class="px-2 py-1 text-right font-medium text-base-content" title={podmanPodId()}>
            <span class="block truncate">{podmanPodId()}</span>
          </td>
        </tr>
      </Show>
      <Show when={typeof podman()?.infra === 'boolean'}>
        <tr>
          <td class="px-2 py-1 text-muted">Podman infra</td>
          <td class="px-2 py-1 text-right font-medium text-base-content">
            {podman()?.infra ? 'Yes' : 'No'}
          </td>
        </tr>
      </Show>
      <Show when={composeProject()}>
        <tr>
          <td class="px-2 py-1 text-muted">Compose project</td>
          <td class="px-2 py-1 text-right font-medium text-base-content">{composeProject()}</td>
        </tr>
      </Show>
      <Show when={composeService()}>
        <tr>
          <td class="px-2 py-1 text-muted">Compose service</td>
          <td class="px-2 py-1 text-right font-medium text-base-content">{composeService()}</td>
        </tr>
      </Show>
      <Show when={autoUpdatePolicy()}>
        <tr>
          <td class="px-2 py-1 text-muted">Auto-update</td>
          <td class="px-2 py-1 text-right font-medium text-base-content">{autoUpdatePolicy()}</td>
        </tr>
      </Show>
      <Show when={userNamespace()}>
        <tr>
          <td class="px-2 py-1 text-muted">User namespace</td>
          <td class="px-2 py-1 text-right font-medium text-base-content" title={userNamespace()}>
            <span class="block truncate">{userNamespace()}</span>
          </td>
        </tr>
      </Show>
      <Show when={blockReadBytes()}>
        {(readBytes) => (
          <tr>
            <td class="px-2 py-1 text-muted">Block I/O read</td>
            <td class="px-2 py-1 text-right font-medium text-base-content">
              {formatBytes(readBytes())}
            </td>
          </tr>
        )}
      </Show>
      <Show when={blockWriteBytes()}>
        {(writeBytes) => (
          <tr>
            <td class="px-2 py-1 text-muted">Block I/O write</td>
            <td class="px-2 py-1 text-right font-medium text-base-content">
              {formatBytes(writeBytes())}
            </td>
          </tr>
        )}
      </Show>
      <Show when={labelEntries().length > 0}>
        <tr>
          <td class="px-2 py-1 align-top text-muted">Labels</td>
          <td class="px-2 py-1">
            <div class="flex flex-wrap justify-end gap-1">
              <For each={labelEntries()}>
                {([key, value]) => (
                  <span
                    class="inline-flex max-w-full items-center truncate rounded bg-surface-alt px-1.5 py-0.5 text-[10px]"
                    title={value ? `${key}: ${value}` : key}
                  >
                    {key}
                    <Show when={value}>: {value}</Show>
                  </span>
                )}
              </For>
            </div>
          </td>
        </tr>
      </Show>
    </tbody>
  );
};

export const InlineResourceSummaryTables: Component<ResourceSummaryPresentationProps> = (props) => (
  <div
    data-testid={props.dataTestId ?? 'resource-summary-section'}
    class="overflow-hidden rounded border border-border bg-surface"
  >
    <table class="w-full table-fixed text-[11px]">
      <Show
        when={
          props.content !== 'technical' &&
          Boolean(props.drawer.sourceSummary() || props.drawer.identityIpValues()[0])
        }
      >
        <tbody data-testid="resource-current-state-section" class="divide-y divide-border">
          <tr class="bg-surface-alt">
            <th
              colspan="2"
              class="px-2 py-1 text-left text-[10px] font-semibold uppercase tracking-wide text-muted"
            >
              Operator context
            </th>
          </tr>
          <Show when={props.drawer.sourceSummary()}>
            {(source) => (
              <tr>
                <td class="w-[38%] px-2 py-1 text-muted">Pulse coverage</td>
                <td
                  class={`px-2 py-1 text-right font-medium ${source().className}`}
                  title={source().title}
                >
                  {source().label}
                </td>
              </tr>
            )}
          </Show>
          <Show when={props.drawer.identityIpValues()[0]}>
            {(ip) => (
              <tr>
                <td class="w-[38%] px-2 py-1 text-muted">Primary IP</td>
                <td class="px-2 py-1 text-right font-medium text-base-content" title={ip()}>
                  <span class="block truncate">{ip()}</span>
                </td>
              </tr>
            )}
          </Show>
        </tbody>
      </Show>
      <Show when={props.content !== 'overview'}>
        <tbody
          data-testid="resource-runtime-context-section"
          class="divide-y divide-border border-t border-border"
        >
          <tr class="bg-surface-alt">
            <th
              colspan="2"
              class="px-2 py-1 text-left text-[10px] font-semibold uppercase tracking-wide text-muted"
            >
              Runtime context
            </th>
          </tr>
          <tr>
            <td class="w-[38%] px-2 py-1 text-muted">Observed state</td>
            <td class="px-2 py-1 text-right font-medium capitalize text-base-content">
              {props.resource.status || 'unknown'}
            </td>
          </tr>
          <Show when={props.resource.uptime}>
            <tr>
              <td class="px-2 py-1 text-muted">Uptime</td>
              <td class="px-2 py-1 text-right font-medium text-base-content">
                {formatUptime(props.resource.uptime ?? 0)}
              </td>
            </tr>
          </Show>
          <Show when={props.resource.lastSeen}>
            <tr>
              <td class="px-2 py-1 text-muted">Last seen</td>
              <td
                class="px-2 py-1 text-right font-medium text-base-content"
                title={props.drawer.lastSeenAbsolute()}
              >
                {props.drawer.lastSeen() || '—'}
              </td>
            </tr>
          </Show>
        </tbody>
      </Show>
      <Show when={props.content !== 'overview' && dockerContainerMeta(props.resource)}>
        {(docker) => <DockerContainerSummarySection docker={docker()} />}
      </Show>
      <Show when={props.content !== 'overview'}>
        <tbody
          data-testid="resource-identity-section"
          class="divide-y divide-border border-t border-border"
        >
          <tr class="bg-surface-alt">
            <th
              colspan="2"
              class="px-2 py-1 text-left text-[10px] font-semibold uppercase tracking-wide text-muted"
            >
              Identity
            </th>
          </tr>
          <For each={props.drawer.primaryIdentityRows()}>
            {(row) => (
              <tr>
                <td class="w-[38%] px-2 py-1 text-muted">{row.label}</td>
                <td class="px-2 py-1 text-right font-medium text-base-content" title={row.value}>
                  <span class="block truncate">{row.value}</span>
                </td>
              </tr>
            )}
          </For>
          <Show when={props.showPlatformId}>
            <tr>
              <td class="w-[38%] px-2 py-1 text-muted">Platform ID</td>
              <td
                class="px-2 py-1 text-right font-medium text-base-content"
                title={props.resource.platformId}
              >
                <span class="block truncate">{props.resource.platformId}</span>
              </td>
            </tr>
          </Show>
          <Show when={props.drawer.identityIpValues().length > 0}>
            <tr>
              <td class="px-2 py-1 align-top text-muted">IP Addresses</td>
              <td class="px-2 py-1">
                <div class="flex flex-wrap justify-end gap-1">
                  <For each={props.drawer.identityIpValues()}>
                    {(ip) => (
                      <span
                        class="inline-flex items-center rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900 dark:text-blue-200"
                        title={ip}
                      >
                        {ip}
                      </span>
                    )}
                  </For>
                </div>
              </td>
            </tr>
          </Show>
          <Show when={props.resource.tags && props.resource.tags.length > 0}>
            <tr>
              <td class="px-2 py-1 align-top text-muted">Tags</td>
              <td class="px-2 py-1">
                <div class="flex justify-end">
                  <TagBadges tags={props.resource.tags} maxVisible={6} />
                </div>
              </td>
            </tr>
          </Show>
          <Show when={props.drawer.identityAliasValues().length > 0}>
            <tr>
              <td class="px-2 py-1 align-top text-muted">Aliases</td>
              <td class="px-2 py-1">
                <div class="flex flex-wrap justify-end gap-1">
                  <For each={props.drawer.aliasPreviewValues()}>
                    {(value) => (
                      <span
                        class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]"
                        title={value}
                      >
                        {value}
                      </span>
                    )}
                  </For>
                  <Show when={props.drawer.hasAliasOverflow()}>
                    <span class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px] text-muted">
                      +
                      {props.drawer.identityAliasValues().length -
                        props.drawer.aliasPreviewValues().length}
                    </span>
                  </Show>
                </div>
              </td>
            </tr>
          </Show>
          <Show when={!props.drawer.identityCardHasRichData()}>
            <tr>
              <td colspan="2" class="px-2 py-1 text-muted">
                No identity metadata yet.
              </td>
            </tr>
          </Show>
        </tbody>
      </Show>
    </table>
  </div>
);
