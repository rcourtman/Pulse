import { For, Show } from 'solid-js';
import type { Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import { Card } from '@/components/shared/Card';
import { TagBadges } from '@/components/shared/TagBadges';
import { formatRelativeTime, formatUptime } from '@/utils/format';
import { formatInteger } from './resourceDetailMappers';
import type { UseResourceDetailDrawerStateResult } from './useResourceDetailDrawerState';

interface ResourceSummaryPresentationProps {
  resource: Resource;
  drawer: UseResourceDetailDrawerStateResult;
  showPlatformId: boolean;
}

// Docker / Podman containers carry runtime facts (image, restart count,
// created-at, compose membership, labels) that no generic summary row
// surfaces. v5 rendered all of these in the container drawer; the unified
// payload still ships them on resource.docker.
const dockerContainerMeta = (resource: Resource): NonNullable<Resource['docker']> | null => {
  if (resource.type !== 'app-container') return null;
  const docker = resource.docker;
  if (!docker) return null;
  if (!docker.containerId && !docker.containerState && !docker.image) return null;
  return docker;
};

const composeLabelValue = (
  labels: Record<string, string> | undefined,
  suffix: 'project' | 'service',
): string =>
  (
    labels?.[`com.docker.compose.${suffix}`] ||
    labels?.[`io.podman.compose.${suffix}`] ||
    ''
  ).trim();

const dockerCreatedAtMillis = (docker: NonNullable<Resource['docker']>): number | null => {
  const raw = (docker.createdAt || '').trim();
  if (!raw) return null;
  const parsed = Date.parse(raw);
  return Number.isFinite(parsed) ? parsed : null;
};

const DockerContainerSummarySection: Component<{ docker: NonNullable<Resource['docker']> }> = (
  props,
) => {
  const labels = () => props.docker.labels ?? {};
  const labelEntries = () => Object.entries(labels());
  const createdAt = () => dockerCreatedAtMillis(props.docker);
  const restartCount = () => props.docker.restartCount;

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
      <Show when={typeof restartCount() === 'number'}>
        <tr>
          <td class="px-2 py-1 text-muted">Restarts</td>
          <td
            class={`px-2 py-1 text-right font-medium ${
              (restartCount() ?? 0) > 5
                ? 'text-red-600 dark:text-red-400'
                : 'text-base-content'
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
      <Show when={composeLabelValue(labels(), 'project')}>
        <tr>
          <td class="px-2 py-1 text-muted">Compose project</td>
          <td class="px-2 py-1 text-right font-medium text-base-content">
            {composeLabelValue(labels(), 'project')}
          </td>
        </tr>
      </Show>
      <Show when={composeLabelValue(labels(), 'service')}>
        <tr>
          <td class="px-2 py-1 text-muted">Compose service</td>
          <td class="px-2 py-1 text-right font-medium text-base-content">
            {composeLabelValue(labels(), 'service')}
          </td>
        </tr>
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
    data-testid="resource-summary-section"
    class="overflow-hidden rounded border border-border bg-surface"
  >
    <table class="w-full table-fixed text-[11px]">
      <tbody data-testid="resource-current-state-section" class="divide-y divide-border">
        <tr class="bg-surface-alt">
          <th
            colspan="2"
            class="px-2 py-1 text-left text-[10px] font-semibold uppercase tracking-wide text-muted"
          >
            Current state
          </th>
        </tr>
        <tr>
          <td class="w-[38%] px-2 py-1 text-muted">State</td>
          <td class="px-2 py-1 text-right font-medium capitalize text-base-content">
            {props.resource.status || 'unknown'}
          </td>
        </tr>
        <Show when={props.drawer.healthIssue()}>
          {(issue) => (
            <>
              <tr>
                <td class="px-2 py-1 align-top text-muted">Reason</td>
                <td
                  class="px-2 py-1 text-right align-top font-medium text-amber-700 dark:text-amber-300"
                  title={issue().title}
                >
                  <span class="block truncate">{issue().primary}</span>
                </td>
              </tr>
              <Show when={issue().details.length > 0}>
                <tr>
                  <td class="px-2 py-1 align-top text-muted">Also</td>
                  <td
                    class="px-2 py-1 text-right align-top font-medium text-base-content"
                    title={issue().details.join(' · ')}
                  >
                    <span class="block truncate">{issue().details.slice(0, 2).join(' · ')}</span>
                  </td>
                </tr>
              </Show>
            </>
          )}
        </Show>
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
            <td class="px-2 py-1 text-muted">Last Seen</td>
            <td
              class="px-2 py-1 text-right font-medium text-base-content"
              title={props.drawer.lastSeenAbsolute()}
            >
              {props.drawer.lastSeen() || '—'}
            </td>
          </tr>
        </Show>
        <Show when={props.drawer.sourceSummary()}>
          {(source) => (
            <tr>
              <td class="px-2 py-1 text-muted">Sources</td>
              <td
                class={`px-2 py-1 text-right font-medium ${source().className}`}
                title={source().title}
              >
                {source().label}
              </td>
            </tr>
          )}
        </Show>
        <Show when={(props.resource.alerts?.length || 0) > 0}>
          <tr>
            <td class="px-2 py-1 text-muted">Alerts</td>
            <td class="px-2 py-1 text-right font-medium text-amber-600 dark:text-amber-400">
              {formatInteger(props.resource.alerts?.length)}
            </td>
          </tr>
        </Show>
        <Show when={props.showPlatformId && !props.drawer.hasRuntimeOperationalContext()}>
          <tr>
            <td class="px-2 py-1 text-muted">Platform ID</td>
            <td
              class="px-2 py-1 text-right font-medium text-base-content"
              title={props.resource.platformId}
            >
              <span class="block truncate">{props.resource.platformId}</span>
            </td>
          </tr>
        </Show>
      </tbody>
      <Show when={dockerContainerMeta(props.resource)}>
        {(docker) => <DockerContainerSummarySection docker={docker()} />}
      </Show>
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
    </table>
  </div>
);

export const ResourceSummaryCards: Component<ResourceSummaryPresentationProps> = (props) => {
  const { resource, drawer, showPlatformId } = props;

  return (
    <div data-testid="resource-summary-section" class="grid gap-3 sm:grid-cols-2">
      <Card data-testid="resource-current-state-section" padding="sm" class="h-full shadow-sm">
        <div class="mb-2 text-[10px] font-medium uppercase tracking-wide text-base-content">
          Current state
        </div>
        <div class="space-y-1.5 text-[11px]">
          <div class="flex items-center justify-between gap-2">
            <span class="text-muted">State</span>
            <span class="font-medium text-base-content capitalize">
              {resource.status || 'unknown'}
            </span>
          </div>
          <Show when={drawer.healthIssue()}>
            {(issue) => (
              <>
                <div class="flex items-start justify-between gap-2">
                  <span class="shrink-0 text-muted">Reason</span>
                  <span
                    class="max-w-[68%] text-right font-medium text-amber-700 dark:text-amber-300"
                    title={issue().title}
                  >
                    {issue().primary}
                  </span>
                </div>
                <Show when={issue().details.length > 0}>
                  <div class="flex items-start justify-between gap-2">
                    <span class="shrink-0 text-muted">Also</span>
                    <span
                      class="max-w-[68%] text-right font-medium text-base-content"
                      title={issue().details.join(' · ')}
                    >
                      {issue().details.slice(0, 2).join(' · ')}
                    </span>
                  </div>
                </Show>
              </>
            )}
          </Show>
          <Show when={resource.uptime}>
            <div class="flex items-center justify-between gap-2">
              <span class="text-muted">Uptime</span>
              <span class="font-medium text-base-content">
                {formatUptime(resource.uptime ?? 0)}
              </span>
            </div>
          </Show>
          <Show when={resource.lastSeen}>
            <div class="flex items-center justify-between gap-2">
              <span class="text-muted">Last Seen</span>
              <span class="font-medium text-base-content" title={drawer.lastSeenAbsolute()}>
                {drawer.lastSeen() || '—'}
              </span>
            </div>
          </Show>
          <Show when={drawer.sourceSummary()}>
            <div class="flex items-center justify-between gap-2">
              <span class="text-muted">Sources</span>
              <span
                class={`font-medium ${drawer.sourceSummary()!.className}`}
                title={drawer.sourceSummary()!.title}
              >
                {drawer.sourceSummary()!.label}
              </span>
            </div>
          </Show>
          <Show when={(resource.alerts?.length || 0) > 0}>
            <div class="flex items-center justify-between gap-2">
              <span class="text-muted">Alerts</span>
              <span class="font-medium text-amber-600 dark:text-amber-400">
                {formatInteger(resource.alerts?.length)}
              </span>
            </div>
          </Show>
          <Show when={showPlatformId && !drawer.hasRuntimeOperationalContext()}>
            <div class="flex items-center justify-between gap-2">
              <span class="text-muted">Platform ID</span>
              <span class="font-medium text-base-content truncate" title={resource.platformId}>
                {resource.platformId}
              </span>
            </div>
          </Show>
          <Show when={drawer.hasRuntimeOperationalContext()}>
            <div class="mt-2 space-y-1.5">
              <Show when={showPlatformId}>
                <div class="flex items-center justify-between gap-2">
                  <span class="text-muted">Platform ID</span>
                  <span class="font-medium text-base-content truncate" title={resource.platformId}>
                    {resource.platformId}
                  </span>
                </div>
              </Show>
              <Show when={drawer.kubernetesCapabilityBadges().length > 0}>
                <div class="flex flex-col gap-1">
                  <span class="text-muted">Platform signals</span>
                  <div class="flex flex-wrap gap-1">
                    <For each={drawer.kubernetesCapabilityBadges()}>
                      {(badge) => (
                        <span class={badge.classes} title={badge.title}>
                          {badge.label}
                        </span>
                      )}
                    </For>
                  </div>
                </div>
              </Show>
            </div>
          </Show>
        </div>
      </Card>

      <Card data-testid="resource-identity-section" padding="sm" class="h-full shadow-sm">
        <div class="mb-2 text-[10px] font-medium uppercase tracking-wide text-base-content">
          Identity
        </div>
        <div class="space-y-1.5 text-[11px]">
          <For each={drawer.primaryIdentityRows()}>
            {(row) => (
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted">{row.label}</span>
                <span class="font-medium text-base-content truncate" title={row.value}>
                  {row.value}
                </span>
              </div>
            )}
          </For>
          <Show when={drawer.identityIpValues().length > 0}>
            <div class="flex flex-col gap-1">
              <span class="text-muted">IP Addresses</span>
              <div class="flex flex-wrap gap-1">
                <For each={drawer.identityIpValues()}>
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
            </div>
          </Show>
          <Show when={resource.tags && resource.tags.length > 0}>
            <div class="flex items-center justify-between gap-2">
              <span class="text-muted">Tags</span>
              <TagBadges tags={resource.tags} maxVisible={6} />
            </div>
          </Show>
          <Show when={drawer.identityAliasValues().length > 0}>
            <Show
              when={drawer.hasAliasOverflow()}
              fallback={
                <div class="flex flex-col gap-1">
                  <span class="text-muted">Aliases</span>
                  <div class="flex flex-wrap gap-1">
                    <For each={drawer.aliasPreviewValues()}>
                      {(value) => (
                        <span
                          class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]"
                          title={value}
                        >
                          {value}
                        </span>
                      )}
                    </For>
                  </div>
                </div>
              }
            >
              <details class="rounded border border-border bg-surface px-2 py-1.5">
                <summary class="flex cursor-pointer list-none items-center justify-between text-[10px] font-medium text-muted">
                  <span>Aliases</span>
                  <span class="text-muted">{drawer.identityAliasValues().length}</span>
                </summary>
                <div class="mt-2 flex flex-wrap gap-1 border-t border-border pt-2">
                  <For each={drawer.identityAliasValues()}>
                    {(value) => (
                      <span
                        class="inline-flex items-center rounded bg-surface-alt px-1.5 py-0.5 text-[10px]"
                        title={value}
                      >
                        {value}
                      </span>
                    )}
                  </For>
                </div>
              </details>
            </Show>
          </Show>
          <Show when={!drawer.identityCardHasRichData()}>
            <div class="rounded border border-dashed bg-surface-hover px-2 py-1.5 text-[10px] ">
              No identity metadata yet.
            </div>
          </Show>
        </div>
      </Card>
    </div>
  );
};
