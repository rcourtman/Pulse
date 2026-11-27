import { A, useNavigate, useSearchParams } from '@solidjs/router';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import LayersIcon from 'lucide-solid/icons/layers';
import { For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import {
  PlatformTableEmptyState,
  PlatformTableShell,
  PlatformTableToolbar,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformTableFilterOption,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import { buildDockerPath } from '@/routing/resourceLinks';
import { formatAbsoluteTime } from '@/utils/format';
import {
  getAvailabilityProbeEndpointLabel,
  getAvailabilityProbePresentation,
} from '@/utils/availabilityProbePresentation';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { getSimpleStatusIndicator } from '@/utils/status';
import {
  buildDockerComposeApplications,
  composeActivity,
  composeAttentionCount,
  composeGuestLocation,
  getDockerComposeApplicationStaleness,
  type DockerComposeApplication,
} from './dockerComposeApplication';
import { useDockerHostIdentities } from './dockerHostIdentity';
import { dockerComposeMembership, type DockerStaleNotice } from './dockerPageModel';
import { DockerComposeDeployments } from './DockerComposeDeployments';
import { DockerLiveLogs } from './DockerLiveLogs';
import { DockerStaleDataCue } from './DockerNativeTableShared';
import { DockerPortChecksNotice } from './DockerPortChecksNotice';
import { DockerContainersTable } from './DockerContainersTable';
import { DockerHostsTable } from './DockerHostsTable';

const applicationsPath = buildDockerPath('applications');

type DockerApplicationStatusFilter = 'all' | 'attention' | 'healthy';

const DOCKER_APPLICATION_FILTER_OPTIONS: PlatformTableFilterOption<DockerApplicationStatusFilter>[] =
  [
    { value: 'all', label: 'All' },
    {
      value: 'attention',
      label: 'Attention',
      tone: 'warning',
      leading: filterChipStatusDot('bg-amber-500'),
    },
    {
      value: 'healthy',
      label: 'Healthy',
      tone: 'success',
      leading: filterChipStatusDot('bg-emerald-500'),
    },
  ];

function applicationPath(app: Pick<DockerComposeApplication, 'hostId' | 'project'>) {
  return (
    applicationsPath + '?' + new URLSearchParams({ runtime: app.hostId, project: app.project })
  );
}

function attachedChecks(container: Resource) {
  const checks = container.availabilityChecks?.length
    ? container.availabilityChecks
    : container.availability
      ? [container.availability]
      : [];
  return checks.filter((check) => check.correlationState === 'attached');
}

function endpointVariant(
  check: ResourceAvailabilityMeta,
): 'success' | 'warning' | 'danger' | 'muted' {
  if (check.enabled === false) return 'muted';
  if (check.available === false || check.applicationOutcome === 'failed') return 'danger';
  const code = check.applicationStatusCode;
  if (typeof code === 'number') {
    if (code >= 500) return 'danger';
    if (code >= 400) return 'warning';
  }
  if (check.available === true) return 'success';
  return 'muted';
}

function EndpointResult(props: { check: ResourceAvailabilityMeta; container: Resource }) {
  const now = useRelativeTimeNow();
  const presentation = createMemo(() =>
    getAvailabilityProbePresentation(
      {
        type: 'network-endpoint',
        platformType: 'availability',
        status: 'unknown',
        availability: props.check,
      },
      new Date(now()),
    ),
  );
  const variant = () => endpointVariant(props.check);
  const isFailing = () => variant() === 'danger' || variant() === 'warning';
  return (
    <li class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-b border-border py-2 last:border-0">
      <div class="flex min-w-0 items-center gap-2">
        <StatusDot size="sm" variant={variant()} ariaHidden />
        <span class="font-medium text-base-content">
          {props.check.name || props.container.name}
        </span>
        <span class="break-all text-muted">{getAvailabilityProbeEndpointLabel(props.check)}</span>
      </div>
      <div class="flex items-center gap-2">
        <span
          class={
            props.check.enabled === false
              ? 'text-xs text-muted'
              : isFailing()
                ? `inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                    variant() === 'danger'
                      ? 'bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-300'
                      : 'bg-amber-100 text-amber-800 dark:bg-amber-950/50 dark:text-amber-300'
                  }`
                : `text-xs font-medium ${presentation()?.toneClassName ?? 'text-base-content'}`
          }
          data-endpoint-check-result
        >
          {props.check.enabled === false ? 'Disabled' : presentation()?.rowLabel}
        </span>
        <Show when={props.check.enabled !== false && presentation()?.freshnessLabel === 'stale'}>
          <span class="text-xs text-muted">Stale reading</span>
        </Show>
      </div>
    </li>
  );
}

function ApplicationDetail(props: { app: DockerComposeApplication; refresh: () => Promise<void> }) {
  const containerIds = createMemo(
    () => new Set(props.app.containers.map((c) => c.docker?.containerId).filter(Boolean)),
  );
  const [events, setEvents] = createStore<ReturnType<typeof composeActivity>>([]);
  createEffect(() => setEvents(reconcile(composeActivity(props.app.containers))));
  const checks = createMemo(() =>
    props.app.containers.flatMap((container) =>
      attachedChecks(container).map((check) => ({ container, check })),
    ),
  );
  const hosts = createMemo(() => (props.app.host ? [props.app.host] : []));
  const staleness = createMemo(() => getDockerComposeApplicationStaleness(props.app));
  return (
    <div class="space-y-4">
      <div>
        <A class="text-sm text-primary hover:underline" href={applicationsPath}>
          All applications
        </A>
        <h1 class="break-words text-xl font-semibold">{props.app.project}</h1>
        <Show when={composeGuestLocation(props.app.host)}>
          {(location) => <p class="mt-1 break-words text-sm text-muted">{location()}</p>}
        </Show>
      </div>
      <Show when={staleness()}>
        {(stale) => (
          <div data-testid="docker-application-stale-notice">
            <InlineNotice tone="warning" role="status">
              <span class="font-medium">{stale().label}.</span> {stale().detail}
            </InlineNotice>
          </div>
        )}
      </Show>
      <Show when={hosts().length}>
        <DockerHostsTable
          title="Runtime host"
          resources={hosts()}
          sourceCount={hosts().length}
          emptyIcon={null}
          emptyTitle="Runtime host unavailable"
          emptyDescription=""
          showToolbar={false}
        />
      </Show>
      <section class="rounded-lg border border-border bg-surface p-4 text-sm">
        <h2 class="font-semibold">Service checks</h2>
        <DockerPortChecksNotice
          containerSignature={JSON.stringify([...containerIds()].sort())}
          includesPort={(port) =>
            port.hostId === props.app.hostId && containerIds().has(port.containerId)
          }
          onChecksAdded={() => void props.refresh()}
        />

        <ul>
          <For
            each={checks()}
            fallback={<li class="mt-2 text-muted">No linked endpoint checks.</li>}
          >
            {(entry) => <EndpointResult check={entry.check} container={entry.container} />}
          </For>
        </ul>
      </section>
      <DockerComposeDeployments runtime={props.app.hostId} project={props.app.project} />
      <DockerLiveLogs
        hostId={props.app.hostId}
        enabled={props.app.host?.docker?.liveLogsEnabled === true}
        containers={props.app.containers}
      />
      <DockerContainersTable
        resources={props.app.containers}
        hosts={hosts()}
        title="Service containers"
        emptyIcon={null}
        emptyTitle="No service containers"
        emptyDescription=""
        onLifecycleActionSettled={props.refresh}
      />
      <section class="rounded-lg border border-border bg-surface p-4 text-sm">
        <h2 class="font-semibold">Recent activity</h2>
        <ol class="divide-y divide-border">
          <For
            each={events}
            fallback={<li class="mt-2 text-muted">No timestamped activity reported.</li>}
          >
            {(event) => (
              <li class="py-2">
                <time class="mr-3 text-xs text-muted" dateTime={new Date(event.time).toISOString()}>
                  {formatAbsoluteTime(event.time)}
                </time>
                <span class="mr-2 break-all font-medium">{event.container}</span>
                {event.message}
                <Show when={event.lines}>
                  {(lines) => (
                    <details class="mt-1">
                      <summary class="cursor-pointer">Last log lines</summary>
                      <pre class="mt-2 max-h-64 overflow-auto rounded bg-surface-alt p-2 text-xs whitespace-pre-wrap wrap-anywhere">
                        {lines().join('\n')}
                      </pre>
                    </details>
                  )}
                </Show>
              </li>
            )}
          </For>
        </ol>
      </section>
    </div>
  );
}

function applicationStatusVariant(
  app: DockerComposeApplication,
  staleness: DockerStaleNotice | null,
): 'success' | 'warning' | 'danger' {
  const attention = composeAttentionCount(app);
  if (attention === 0) return staleness ? 'warning' : 'success';
  const hasCritical = app.containers.some(
    (container) => getSimpleStatusIndicator(container.status).variant === 'danger',
  );
  return hasCritical ? 'danger' : 'warning';
}

export function DockerApplications(props: {
  hosts: Resource[];
  containers: Resource[];
  refresh: () => Promise<void>;
}) {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const identities = useDockerHostIdentities();
  const [search, setSearch] = createSignal('');
  const [statusFilter, setStatusFilter] = createSignal<DockerApplicationStatusFilter>('all');

  const appEntries = createMemo(() =>
    buildDockerComposeApplications(props.hosts, props.containers)
      .map((app) => {
        const path = applicationPath(app);
        const hostLabel = identities().labelFor(app.containers[0]);
        const serviceCount = new Set(
          app.containers
            .map((container) => dockerComposeMembership(container.docker).service)
            .filter(Boolean),
        ).size;
        const attentionCount = composeAttentionCount(app);
        const staleness = getDockerComposeApplicationStaleness(app);
        const needsAttention = attentionCount > 0 || Boolean(staleness);
        const variant = applicationStatusVariant(app, staleness);
        const searchHaystack = [
          app.project,
          hostLabel,
          staleness?.label ?? '',
          staleness?.detail ?? '',
          ...app.containers.map((container) => container.name),
        ]
          .join(' ')
          .toLowerCase();
        return {
          path,
          app,
          hostLabel,
          serviceCount,
          attentionCount,
          staleness,
          needsAttention,
          variant,
          searchHaystack,
        };
      })
      .sort(
        (left, right) =>
          right.attentionCount - left.attentionCount ||
          Number(Boolean(right.staleness)) - Number(Boolean(left.staleness)) ||
          left.app.project.localeCompare(right.app.project),
      ),
  );

  const applicationsByPath = createMemo(
    () => new Map(appEntries().map((entry) => [entry.path, entry.app])),
  );

  const selection = createMemo(() => {
    if (typeof params.runtime !== 'string' || typeof params.project !== 'string') return undefined;
    return applicationsByPath().get(
      applicationPath({ hostId: params.runtime, project: params.project }),
    );
  });

  const filterEntries = (filter: DockerApplicationStatusFilter) => {
    const needle = search().trim().toLowerCase();
    return appEntries().filter((entry) => {
      if (filter === 'attention' && !entry.needsAttention) return false;
      if (filter === 'healthy' && entry.needsAttention) return false;
      if (!needle) return true;
      return entry.searchHaystack.includes(needle);
    });
  };

  const filteredEntries = createMemo(() => filterEntries(statusFilter()));
  const countForStatus = (value: DockerApplicationStatusFilter) => filterEntries(value).length;
  const hasActiveFilters = () => search().trim().length > 0 || statusFilter() !== 'all';
  const resetFilters = () => {
    setSearch('');
    setStatusFilter('all');
  };

  return (
    <Show
      when={params.runtime || params.project}
      fallback={
        <Show
          when={appEntries().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={<LayersIcon class="h-6 w-6 text-slate-400" />}
              title="No Compose applications reported"
              description="No Compose projects reported. Standalone containers appear in Overview."
            />
          }
        >
          <div class="space-y-3">
            <PlatformTableToolbar
              search={search}
              onSearchChange={setSearch}
              searchPlaceholder="Search Compose applications or hosts"
              status={statusFilter()}
              onStatusChange={setStatusFilter}
              statusOptions={withPlatformStatusCounts(
                DOCKER_APPLICATION_FILTER_OPTIONS,
                countForStatus,
              )}
              visible={filteredEntries().length}
              total={appEntries().length}
              rowNoun="applications"
              hasActiveFilters={hasActiveFilters()}
              onResetFilters={resetFilters}
            />

            <Show
              when={filteredEntries().length > 0}
              fallback={
                <PlatformTableEmptyState
                  icon={<LayersIcon class="h-6 w-6 text-slate-400" />}
                  title="No applications match current filters"
                  description="Adjust the search or status filter to see more Compose applications."
                />
              }
            >
              <PlatformTableShell
                title="Compose applications"
                tableClass="min-w-full table-fixed text-xs md:min-w-[780px]"
                header={
                  <>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('name')} platform-table-mobile-w-38 md:w-[30%]`}
                    >
                      Application
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-25 md:w-[22%]`}
                    >
                      Host
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden sm:table-cell md:w-[11%]`}
                    >
                      Services
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden sm:table-cell md:w-[12%]`}
                    >
                      Containers
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-30 md:w-[21%]`}
                    >
                      Status
                    </TableHead>
                    <TableHead class="platform-table-phone-hidden w-8" aria-label="Open" />
                  </>
                }
                body={
                  <For each={filteredEntries()}>
                    {(entry) => (
                      <TableRow
                        class="cursor-pointer text-[11px] sm:text-xs"
                        data-docker-application-row={entry.app.project}
                        onClick={(event) => {
                          if ((event.target as HTMLElement | null)?.closest('a,button')) return;
                          navigate(entry.path);
                        }}
                      >
                        <TableCell class={getPlatformTableCellClassForKind('name')}>
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={entry.variant}
                              title={
                                entry.attentionCount > 0
                                  ? `${entry.attentionCount} ${entry.attentionCount === 1 ? 'needs' : 'need'} attention${entry.staleness ? ` · ${entry.staleness.detail}` : ''}`
                                  : (entry.staleness?.detail ?? 'Healthy')
                              }
                              ariaHidden
                            />
                            <A
                              href={entry.path}
                              class="truncate font-semibold text-base-content hover:text-primary hover:underline"
                              title={entry.app.project}
                            >
                              {entry.app.project}
                            </A>
                            <Show when={entry.attentionCount > 0 && entry.staleness}>
                              <DockerStaleDataCue notice={entry.staleness} scope="application" />
                            </Show>
                          </div>
                        </TableCell>
                        <TableCell class={getPlatformTableCellClassForKind('text')}>
                          <span class="block truncate text-muted" title={entry.hostLabel}>
                            {entry.hostLabel}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums sm:table-cell`}
                        >
                          {entry.serviceCount > 0 ? entry.serviceCount : '—'}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums sm:table-cell`}
                        >
                          {entry.app.containers.length}
                        </TableCell>
                        <TableCell class={getPlatformTableCellClassForKind('text')}>
                          <Show
                            when={entry.attentionCount > 0}
                            fallback={
                              <Show
                                when={entry.staleness}
                                fallback={
                                  <span class="inline-flex items-center rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300">
                                    Healthy
                                  </span>
                                }
                              >
                                {(stale) => (
                                  <span
                                    class="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950/50 dark:text-amber-300"
                                    title={stale().detail}
                                    data-docker-application-stale-pill
                                  >
                                    {stale().label}
                                  </span>
                                )}
                              </Show>
                            }
                          >
                            <span
                              class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${
                                entry.variant === 'danger'
                                  ? 'bg-red-100 text-red-700 dark:bg-red-950/50 dark:text-red-300'
                                  : 'bg-amber-100 text-amber-800 dark:bg-amber-950/50 dark:text-amber-300'
                              }`}
                              data-docker-application-attention-pill
                            >
                              {entry.attentionCount} {entry.attentionCount === 1 ? 'needs' : 'need'}{' '}
                              attention
                            </span>
                          </Show>
                        </TableCell>
                        <TableCell class="platform-table-phone-hidden text-right text-muted">
                          <ChevronRightIcon
                            class="ml-auto h-4 w-4 transition-transform group-hover:translate-x-0.5 group-hover:text-base-content"
                            aria-hidden="true"
                          />
                        </TableCell>
                      </TableRow>
                    )}
                  </For>
                }
              />
            </Show>
          </div>
        </Show>
      }
    >
      <Show
        when={selection()}
        fallback={
          <div class="space-y-2">
            <p>This application is missing from the current Docker inventory.</p>
            <A href={applicationsPath} class="text-primary hover:underline">
              All applications
            </A>
          </div>
        }
      >
        {(app) => <ApplicationDetail app={app()} refresh={props.refresh} />}
      </Show>
    </Show>
  );
}
