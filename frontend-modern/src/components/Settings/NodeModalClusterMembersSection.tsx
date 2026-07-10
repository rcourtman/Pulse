import { Component, For, Show, createMemo } from 'solid-js';
import type { ClusterEndpoint } from '@/types/nodes';
import type { NodeModalProps } from '@/components/Settings/nodeModalModel';
import type { NodeModalState } from '@/components/Settings/useNodeModalState';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { controlClass, formHelpText } from '@/components/shared/Form';

interface NodeModalClusterMembersSectionProps {
  modalProps: NodeModalProps;
  state: NodeModalState;
}

const discoveredAddress = (endpoint: ClusterEndpoint): string => {
  const host = endpoint.host?.replace(/^https?:\/\//, '') ?? '';
  if (endpoint.ip && host && !host.startsWith(endpoint.ip)) {
    return `${host} (${endpoint.ip})`;
  }
  return host || endpoint.ip || '';
};

// Per-member connection addresses for an existing PVE cluster. The discovered
// host and IP are rebuilt from cluster status on every re-discovery, so the
// only durable user edit is the override (ClusterEndpoint.ipOverride), which
// wins at poll time via EffectiveIP.
export const NodeModalClusterMembersSection: Component<NodeModalClusterMembersSectionProps> = (
  props,
) => {
  const { modalProps, state } = props;

  const endpoints = createMemo<ClusterEndpoint[]>(() => {
    const node = modalProps.editingNode;
    if (!node || node.type !== 'pve' || !('clusterEndpoints' in node)) return [];
    return node.clusterEndpoints ?? [];
  });

  return (
    <Show when={endpoints().length > 0}>
      <div>
        <SectionHeader
          title="Cluster members"
          size="sm"
          class="mb-1"
          titleClass="text-base-content"
        />
        <p class="mb-3 text-xs text-muted">
          Pulse found these nodes in the cluster and connects to each one directly. If a node's
          discovered address isn't reachable from Pulse (for example when the cluster reports an
          internal network IP), enter the address Pulse should use instead.
        </p>
        <div class="rounded-md border border-border">
          <For each={endpoints()}>
            {(endpoint) => (
              <div class="flex flex-col gap-2 border-b border-border-subtle p-3 last:border-b-0 sm:flex-row sm:items-center">
                <div class="min-w-0 sm:w-1/2">
                  <div class="flex items-center gap-1.5 text-sm text-base-content">
                    <span class="truncate">{endpoint.nodeName}</span>
                    <Show when={endpoint.pulseReachable === false}>
                      <span
                        class="inline-flex flex-shrink-0 items-center rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-950 dark:text-amber-300"
                        title={endpoint.pulseError || 'Pulse could not connect to this node'}
                      >
                        Unreachable
                      </span>
                    </Show>
                  </div>
                  <div class="truncate text-xs text-muted" title={discoveredAddress(endpoint)}>
                    Discovered: {discoveredAddress(endpoint)}
                  </div>
                </div>
                <div class="sm:w-1/2">
                  <input
                    type="text"
                    value={state.formData().clusterEndpointOverrides[endpoint.nodeName] ?? ''}
                    onInput={(event) =>
                      state.updateClusterEndpointOverride(
                        endpoint.nodeName,
                        event.currentTarget.value,
                      )
                    }
                    placeholder={endpoint.ip || 'IP or hostname'}
                    aria-label={`Connection address for ${endpoint.nodeName}`}
                    class={controlClass()}
                  />
                </div>
              </div>
            )}
          </For>
        </div>
        <p class={formHelpText}>
          Optional. IP or hostname, with a port if it isn't 8006. Leave blank to use the discovered
          address.
        </p>
      </div>
    </Show>
  );
};
