import { Component, For, Show, createMemo, createUniqueId } from 'solid-js';
import type { ClusterEndpoint } from '@/types/nodes';
import type { NodeModalProps } from '@/components/Settings/nodeModalModel';
import type { NodeModalState } from '@/components/Settings/useNodeModalState';
import { InlineNotice } from '@/components/shared/InlineNotice';
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

// Per-member presentation and connection settings for an existing PVE
// cluster. Display names are keyed by Pulse's immutable node identity while
// connection overrides remain keyed by the provider's native node name.
export const NodeModalClusterMembersSection: Component<NodeModalClusterMembersSectionProps> = (
  props,
) => {
  const { modalProps, state } = props;
  const trustDescriptionId = createUniqueId();

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
          Give each node an optional display name for Pulse. This never changes its Proxmox name,
          identity, credentials, or connection address.
        </p>
        <div class="rounded-md border border-border">
          <For each={endpoints()}>
            {(endpoint, index) => (
              <div class="grid gap-3 border-b border-border-subtle p-3 last:border-b-0 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
                <div class="min-w-0">
                  <div class="flex items-center gap-1.5 text-sm text-base-content">
                    <span class="truncate">{endpoint.displayName || endpoint.nodeName}</span>
                  </div>
                  <Show when={endpoint.displayName}>
                    <div class="truncate text-xs text-muted" title={endpoint.nodeName}>
                      Proxmox node: {endpoint.nodeName}
                    </div>
                  </Show>
                  <div class="truncate text-xs text-muted" title={discoveredAddress(endpoint)}>
                    Discovered: {discoveredAddress(endpoint)}
                  </div>
                  <Show when={endpoint.pulseReachable === false}>
                    <details class="mt-2 rounded-md bg-amber-100 text-xs text-amber-800 dark:bg-amber-950/25 dark:text-amber-300">
                      <summary class="min-h-11 cursor-pointer rounded-md px-3 py-3 font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-amber-600 dark:focus-visible:outline-amber-300">
                        Unreachable — details
                        <span class="sr-only"> for {endpoint.nodeName}</span>
                      </summary>
                      <p class="break-words whitespace-pre-wrap px-3 pb-3 pt-1">
                        {endpoint.pulseError || 'Pulse could not connect to this node'}
                      </p>
                    </details>
                  </Show>
                </div>
                <div class="grid gap-2">
                  <label class="grid gap-1 text-xs text-muted">
                    Display name
                    <input
                      type="text"
                      maxlength={128}
                      value={
                        endpoint.nodeIdentity
                          ? (state.formData().clusterNodeDisplayNames[endpoint.nodeIdentity] ?? '')
                          : ''
                      }
                      onInput={(event) => {
                        if (endpoint.nodeIdentity) {
                          state.updateClusterNodeDisplayName(
                            endpoint.nodeIdentity,
                            event.currentTarget.value,
                          );
                        }
                      }}
                      disabled={!endpoint.nodeIdentity}
                      placeholder={endpoint.nodeName}
                      aria-label={`Display name for ${endpoint.nodeName}`}
                      class={controlClass()}
                    />
                  </label>
                  <label class="grid gap-1 text-xs text-muted">
                    Connection address override
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
                  </label>
                  <Show when={endpoint.tlsEndpoint}>
                    <div class="grid min-w-0 gap-2">
                      <p class="break-all text-xs text-muted">
                        TLS endpoint: {endpoint.tlsEndpoint}
                      </p>
                      <Show
                        when={!endpoint.usesPrimaryTLS}
                        fallback={
                          <p class="text-xs text-muted">
                            Uses the primary SSL settings. Editing them does not change other
                            members.
                          </p>
                        }
                      >
                        <label class="grid gap-1 text-xs text-muted">
                          Certificate fingerprint (optional)
                          <input
                            type="text"
                            value={
                              state.formData().clusterEndpointFingerprints[endpoint.tlsEndpoint!] ??
                              endpoint.fingerprint ??
                              ''
                            }
                            onInput={(event) =>
                              state.updateClusterEndpointFingerprint(
                                endpoint.tlsEndpoint!,
                                event.currentTarget.value,
                              )
                            }
                            aria-label={`Certificate fingerprint for ${endpoint.nodeName}`}
                            aria-describedby={`${trustDescriptionId}-${index()}`}
                            disabled={!endpoint.tlsEndpoint?.startsWith('https://')}
                            class={controlClass('font-mono')}
                          />
                        </label>
                      </Show>
                      <p id={`${trustDescriptionId}-${index()}`} class="text-xs text-muted">
                        {endpoint.usesPrimaryTLS
                          ? 'Primary endpoint only.'
                          : endpoint.tlsFirstUse &&
                              !state.formData().clusterEndpointTrustEdited[endpoint.tlsEndpoint!]
                            ? 'No saved member pin. Existing first-use policy applies. Enter a verified pin or explicitly choose CA validation.'
                            : (state.formData().clusterEndpointFingerprints[
                                  endpoint.tlsEndpoint!
                                ] ??
                                  endpoint.fingerprint ??
                                  '') !== ''
                              ? 'This endpoint must present the matching certificate.'
                              : state.formData().verifySSL
                                ? 'No pin: a trusted, hostname-matching certificate is required.'
                                : 'No pin and verification unchecked: certificate identity is not verified.'}
                      </p>
                      <Show
                        when={
                          !endpoint.usesPrimaryTLS &&
                          state.formData().verifySSL &&
                          endpoint.tlsEndpoint?.startsWith('https://')
                        }
                      >
                        <button
                          type="button"
                          class="min-h-11 justify-self-start rounded-md border border-border px-3 py-2 text-xs text-base-content hover:bg-surface-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
                          onClick={() =>
                            state.updateClusterEndpointFingerprint(endpoint.tlsEndpoint!, '')
                          }
                        >
                          Use CA validation for {endpoint.nodeName}
                        </button>
                      </Show>
                      <button
                        type="button"
                        class="min-h-11 justify-self-start rounded-md border border-border px-3 py-2 text-xs text-base-content hover:bg-surface-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
                        disabled={state.isTesting()}
                        onClick={() => void state.handleTestClusterEndpoint(endpoint)}
                      >
                        Test TLS for {endpoint.nodeName}
                      </button>
                      <Show when={state.testResult()?.endpoint === endpoint.tlsEndpoint}>
                        <div
                          ref={(element) =>
                            queueMicrotask(() => {
                              if (element.isConnected)
                                element.scrollIntoView?.({ block: 'nearest' });
                            })
                          }
                        >
                          <InlineNotice
                            class="break-words"
                            tone={
                              state.testResult()?.status === 'error'
                                ? 'danger'
                                : state.testResult()?.status === 'warning'
                                  ? 'warning'
                                  : 'success'
                            }
                            role="status"
                            aria-label={`TLS test result for ${endpoint.nodeName}`}
                            aria-live="polite"
                            aria-atomic="true"
                          >
                            {state.testResult()?.message}
                          </InlineNotice>
                        </div>
                      </Show>
                    </div>
                  </Show>
                </div>
              </div>
            )}
          </For>
        </div>
        <p class={formHelpText}>
          Leave a display name blank to use the native Proxmox name. Connection addresses are also
          optional. Enter an IP or hostname, with a port if it isn't 8006, only when Pulse cannot
          reach the discovered address.
        </p>
        <p class={formHelpText}>
          Trust edits apply only to the displayed TLS endpoint. Independently verify a renewed
          certificate through an already-trusted administration channel before changing a pin. For
          CA-managed certificates, clear only that endpoint's pin and keep Verify SSL enabled. Tests
          use stored credentials without saving TLS edits or updating background/agent health. Save
          address or credential changes before testing them.
        </p>
      </div>
    </Show>
  );
};
