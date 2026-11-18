import { For, Show, createEffect, createMemo, createSignal, on, onCleanup } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { FormSelect } from '@/components/shared/FormSelect';
import { eventBus } from '@/stores/events';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import type { Resource } from '@/types/resource';
import { apiFetchJSON, getOrgID } from '@/utils/apiClient';

type LogTail = { lines: string[] | null; truncated: boolean };

export function DockerLiveLogs(props: {
  hostId: string;
  enabled: boolean;
  containers: Resource[];
}) {
  const [selected, setSelected] = createSignal('');
  const [following, setFollowing] = createSignal(false);
  const [tail, setTail] = createSignal<LogTail>();
  const [error, setError] = createSignal('');
  let active: AbortController | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const available = createMemo(() => props.containers.filter((c) => c.docker?.containerId));
  const container = createMemo(
    () => available().find((c) => c.docker?.containerId === selected()) ?? available()[0],
  );
  const stop = () => {
    active?.abort();
    active = undefined;
    clearTimeout(timer);
    setFollowing(false);
    setTail(undefined);
    setError('');
  };
  const scope = createMemo(() =>
    JSON.stringify([
      props.hostId,
      props.enabled,
      container()?.docker?.containerId,
      presentationPolicyIsReadOnly(),
    ]),
  );
  createEffect(on(scope, stop, { defer: true }));
  const offOrg = eventBus.on('org_switched', stop);
  const offMembership = eventBus.on('organizations_changed', stop);
  onCleanup(() => {
    stop();
    offOrg();
    offMembership();
  });

  const follow = () => {
    const id = container()?.docker?.containerId;
    if (!props.enabled || !id || following() || presentationPolicyIsReadOnly()) return;
    const controller = new AbortController();
    active = controller;
    const org = getOrgID() || 'default';
    const hostId = props.hostId;
    setError('');
    setFollowing(true);
    const read = async () => {
      try {
        const latest = await apiFetchJSON<LogTail>('/api/agents/docker/log-tail', {
          method: 'POST',
          body: JSON.stringify({ hostId, containerId: id }),
          signal: controller.signal,
          retry: false,
          cache: 'no-store',
          expectedOrgID: org,
        });
        if (active !== controller) return;
        setTail(latest);
        timer = setTimeout(() => void read(), 1000);
      } catch (err) {
        if (active !== controller) return;
        stop();
        setError(err instanceof Error ? err.message : 'Could not read live logs.');
      }
    };
    void read();
  };
  return (
    <details
      class="rounded-lg border border-border bg-surface p-4 text-sm"
      onToggle={(e) => {
        if (!e.currentTarget.open) stop();
      }}
    >
      <summary class="cursor-pointer font-semibold">Live container logs</summary>
      <Show
        when={props.enabled && !presentationPolicyIsReadOnly()}
        fallback={
          <p class="mt-2 text-muted">
            {presentationPolicyIsReadOnly()
              ? 'Live reads require an administrator session on your Pulse instance.'
              : 'To allow live reads, set PULSE_ENABLE_LIVE_LOGS=true on the Docker agent and restart it.'}
          </p>
        }
      >
        <p class="my-2 text-muted">
          Follow the latest 30 masked lines from one container. The view refreshes when the agent
          reports.
        </p>
        <div class="flex flex-wrap items-end gap-2">
          <FormSelect
            label="Container"
            value={container()?.docker?.containerId ?? ''}
            onChange={(e) => {
              stop();
              setSelected(e.currentTarget.value);
            }}
          >
            <For each={available()}>
              {(c) => <option value={c.docker!.containerId}>{c.displayName || c.name}</option>}
            </For>
          </FormSelect>
          <Button
            size="mdCompact"
            disabled={!container()}
            onClick={() => (following() ? stop() : follow())}
          >
            {following() ? 'Stop following' : 'Follow logs'}
          </Button>
        </div>
        <Show when={following() && !tail()}>
          <p role="status" class="mt-2">
            Waiting for the agent…
          </p>
        </Show>
        <Show when={error()}>
          {(message) => (
            <p role="alert" class="mt-2">
              {message()}
            </p>
          )}
        </Show>
        <Show when={tail()}>
          {(latest) => (
            <>
              <Show when={latest().truncated}>
                <p class="mt-2 text-muted">Showing the end of the log.</p>
              </Show>
              <pre class="mt-2 max-h-64 overflow-auto whitespace-pre-wrap wrap-anywhere rounded bg-surface-alt p-2 text-xs">
                {latest().lines?.join('\n') || 'The container returned no log lines.'}
              </pre>
            </>
          )}
        </Show>
      </Show>
    </details>
  );
}
