import { A, useSearchParams } from '@solidjs/router';
import { For, Show, createEffect, createMemo } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import { buildDockerPath } from '@/routing/resourceLinks';
import { formatAbsoluteTime } from '@/utils/format';
import {
  getAvailabilityProbeEndpointLabel,
  getAvailabilityProbePresentation,
} from '@/utils/availabilityProbePresentation';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import {
  buildDockerComposeApplications,
  composeActivity,
  composeAttentionCount,
  type DockerComposeApplication,
} from './dockerComposeApplication';
import { useDockerHostIdentities } from './dockerHostIdentity';
import { dockerComposeMembership } from './dockerPageModel';
import { DockerPortChecksNotice } from './DockerPortChecksNotice';
import { DockerContainersTable } from './DockerContainersTable';
import { DockerHostsTable } from './DockerHostsTable';

const applicationsPath = buildDockerPath('applications');

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
  return (
    <li class="flex flex-wrap items-baseline justify-between gap-x-4 border-b border-border py-2 last:border-0">
      <div class="min-w-0">
        <span class="font-medium">{props.check.name || props.container.name}</span>
        <span class="ml-2 break-all text-muted">
          {getAvailabilityProbeEndpointLabel(props.check)}
        </span>
      </div>
      <span>{props.check.enabled === false ? 'Disabled' : presentation()?.rowLabel}</span>
      <Show when={props.check.enabled !== false && presentation()?.freshnessLabel === 'stale'}>
        <span class="text-muted">Stale reading</span>
      </Show>
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
  return (
    <div class="space-y-4">
      <div>
        <A class="text-sm text-primary hover:underline" href={applicationsPath}>
          All applications
        </A>
        <h1 class="break-words text-xl font-semibold">{props.app.project}</h1>
      </div>
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

export function DockerApplications(props: {
  hosts: Resource[];
  containers: Resource[];
  refresh: () => Promise<void>;
}) {
  const [params] = useSearchParams();
  const identities = useDockerHostIdentities();
  const applications = createMemo(
    () =>
      new Map(
        buildDockerComposeApplications(props.hosts, props.containers).map((app) => [
          applicationPath(app),
          app,
        ]),
      ),
  );
  const selection = createMemo(() => {
    if (typeof params.runtime !== 'string' || typeof params.project !== 'string') return undefined;
    return applications().get(applicationPath({ hostId: params.runtime, project: params.project }));
  });
  return (
    <Show
      when={params.runtime || params.project}
      fallback={
        <section class="rounded-lg border border-border bg-surface p-4">
          <h1 class="text-lg font-semibold">Compose applications</h1>
          <ul class="mt-2 divide-y divide-border">
            <For
              each={[...applications().keys()]}
              fallback={
                <li class="py-3 text-sm text-muted">
                  No Compose projects reported. Standalone containers appear in Overview.
                </li>
              }
            >
              {(path) => {
                const app = () => applications().get(path)!;
                const serviceCount = () =>
                  new Set(
                    app()
                      .containers.map(
                        (container) => dockerComposeMembership(container.docker).service,
                      )
                      .filter(Boolean),
                  ).size;
                return (
                  <li>
                    <A
                      href={path}
                      class="flex flex-wrap items-baseline justify-between gap-x-4 rounded py-3 text-sm hover:bg-surface-alt"
                    >
                      <div class="min-w-0">
                        <span class="break-all font-semibold text-primary">{app().project}</span>
                        <span class="ml-2 break-words text-muted">
                          {identities().labelFor(app().containers[0])}
                        </span>
                      </div>
                      <div class="flex flex-wrap gap-x-3">
                        <Show when={serviceCount()}>
                          {(count) => (
                            <span>
                              {count()} {count() === 1 ? 'service' : 'services'}
                            </span>
                          )}
                        </Show>
                        <span>
                          {app().containers.length}{' '}
                          {app().containers.length === 1 ? 'container' : 'containers'}
                        </span>
                        <Show when={composeAttentionCount(app())}>
                          {(count) => (
                            <span class="font-medium">
                              {count()} {count() === 1 ? 'needs' : 'need'} attention
                            </span>
                          )}
                        </Show>
                      </div>
                    </A>
                  </li>
                );
              }}
            </For>
          </ul>
        </section>
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
