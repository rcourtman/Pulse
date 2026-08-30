import {
  For,
  Show,
  createEffect,
  createMemo,
  createResource,
  createSignal,
  type Component,
} from 'solid-js';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import MapPinIcon from 'lucide-solid/icons/map-pin';
import XIcon from 'lucide-solid/icons/x';
import {
  AvailabilityTargetsAPI,
  type AvailabilityTarget,
  type AvailabilityTestResponse,
} from '@/api/availabilityTargets';
import {
  listDiscoveriesByAgent,
  updateAvailabilityProposal,
  type ConnectedAgent,
} from '@/api/discovery';
import { ActionIconButton, Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import type { DiscoverySummary, ResourceDiscovery, ResourceType } from '@/types/discovery';
import {
  buildAvailabilityTargetFromProposal,
  findAvailabilityProposalDuplicate,
  isAvailabilityProposalDismissed,
  reviewableAvailabilitySummaries,
} from './availabilityProposalModel';

interface AvailabilityProposalCardProps {
  discovery: ResourceDiscovery;
  resourceType: ResourceType;
  targetId: string;
  resourceId: string;
  canonicalResourceId: string;
  connectedAgents?: readonly ConnectedAgent[];
  onDiscoveryUpdated: (discovery: ResourceDiscovery) => void;
}

const endpointLabel = (proposal: ResourceDiscovery['suggested_availability_probe']): string => {
  if (!proposal) return '';
  const protocol = proposal.protocol.toLowerCase();
  const port = proposal.port ? `:${proposal.port}` : '';
  const path = proposal.path || '';
  return `${protocol}://${proposal.address}${port}${path}`;
};

const expectedBehaviorLabel = (
  proposal: ResourceDiscovery['suggested_availability_probe'],
): string =>
  proposal?.protocol === 'http' || proposal?.protocol === 'https'
    ? 'GET returns HTTP 200–399'
    : 'TCP connection is accepted';

const testResultLabel = (result: AvailabilityTestResponse): string => {
  if (!result.success) return result.error || 'The endpoint did not satisfy this proposal.';
  if (result.application?.outcome === 'passed') {
    return result.application.statusCode
      ? `Application response passed with HTTP ${result.application.statusCode}.`
      : 'Application response passed.';
  }
  return `Connection passed in ${result.latencyMillis} ms.`;
};

const proposalTargetId = (summary: DiscoverySummary): string =>
  summary.agent_id || summary.target_id || summary.resource_id;

export const AvailabilityProposalCard: Component<AvailabilityProposalCardProps> = (props) => {
  const proposal = createMemo(() => props.discovery.suggested_availability_probe);
  const [name, setName] = createSignal('');
  const [intervalSeconds, setIntervalSeconds] = createSignal(60);
  const [probeAgentId, setProbeAgentId] = createSignal('');
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);
  const [updatingDisposition, setUpdatingDisposition] = createSignal(false);
  const [proposalError, setProposalError] = createSignal<string | null>(null);
  const [bulkReviewError, setBulkReviewError] = createSignal<string | null>(null);
  const [testResult, setTestResult] = createSignal<AvailabilityTestResponse | null>(null);
  const [createdTarget, setCreatedTarget] = createSignal<AvailabilityTarget | null>(null);
  const [showBulkReview, setShowBulkReview] = createSignal(false);

  const [targets, { refetch: refetchTargets }] = createResource(
    () => proposal()?.evidence_fingerprint || null,
    async () => AvailabilityTargetsAPI.list(),
  );
  const [machineDiscoveries, { refetch: refetchMachineDiscoveries }] = createResource(
    () => (showBulkReview() && props.targetId ? props.targetId : null),
    async (targetId) => listDiscoveriesByAgent(targetId),
  );

  createEffect(() => {
    const current = proposal();
    if (!current) return;
    setName(current.service_name || props.discovery.service_name || props.discovery.hostname);
    setTestResult(null);
    setCreatedTarget(null);
    setProposalError(null);
    setBulkReviewError(null);
  });

  const duplicate = createMemo(() => {
    const current = proposal();
    if (!current || targets.error) return null;
    return findAvailabilityProposalDuplicate(current, props.canonicalResourceId, targets() ?? []);
  });
  const dismissed = createMemo(() => isAvailabilityProposalDismissed(props.discovery));
  const bulkItems = createMemo(() => {
    if (machineDiscoveries.error) return [];
    return reviewableAvailabilitySummaries(machineDiscoveries()?.discoveries ?? []);
  });

  const buildTarget = (): AvailabilityTarget | null => {
    const current = proposal();
    if (!current) return null;
    return buildAvailabilityTargetFromProposal({
      proposal: current,
      canonicalResourceId: props.canonicalResourceId,
      name: name(),
      intervalSeconds: intervalSeconds(),
      probeAgentId: probeAgentId(),
    });
  };

  const handleTest = async (trigger?: HTMLButtonElement) => {
    const target = buildTarget();
    if (!target) return;
    setTesting(true);
    setProposalError(null);
    setTestResult(null);
    try {
      setTestResult(await AvailabilityTargetsAPI.test(target));
    } catch (cause) {
      setProposalError(cause instanceof Error ? cause.message : 'Failed to test this proposal.');
    } finally {
      setTesting(false);
      queueMicrotask(() => trigger?.focus());
    }
  };

  const handleCreate = async () => {
    const target = buildTarget();
    if (!target || targets.loading || targets.error || duplicate()?.kind === 'endpoint') return;
    setSaving(true);
    setProposalError(null);
    try {
      const created = await AvailabilityTargetsAPI.create(target);
      setCreatedTarget(created);
      await refetchTargets();
    } catch (cause) {
      setProposalError(cause instanceof Error ? cause.message : 'Failed to create this check.');
    } finally {
      setSaving(false);
    }
  };

  const updateCurrentDisposition = async (status: 'dismissed' | 'reviewable') => {
    const current = proposal();
    if (!current) return;
    const fingerprint = current.evidence_fingerprint?.trim();
    if (!fingerprint) {
      setProposalError('Run discovery again before changing this legacy suggestion.');
      return;
    }
    setUpdatingDisposition(true);
    setProposalError(null);
    try {
      const updated = await updateAvailabilityProposal(
        props.resourceType,
        props.targetId,
        props.resourceId,
        fingerprint,
        status,
      );
      props.onDiscoveryUpdated(updated);
      if (showBulkReview()) await refetchMachineDiscoveries();
    } catch (cause) {
      setProposalError(
        cause instanceof Error ? cause.message : 'Failed to update this proposal review state.',
      );
    } finally {
      setUpdatingDisposition(false);
    }
  };

  const updateBulkDisposition = async (
    summary: DiscoverySummary,
    status: 'dismissed' | 'reviewable',
  ) => {
    const current = summary.suggested_availability_probe;
    if (!current) return;
    const fingerprint = current.evidence_fingerprint?.trim();
    if (!fingerprint) {
      setBulkReviewError('Run discovery again before changing this legacy suggestion.');
      return;
    }
    setUpdatingDisposition(true);
    setBulkReviewError(null);
    try {
      const updated = await updateAvailabilityProposal(
        summary.resource_type,
        proposalTargetId(summary),
        summary.resource_id,
        fingerprint,
        status,
      );
      if (summary.id === props.discovery.id) props.onDiscoveryUpdated(updated);
      await refetchMachineDiscoveries();
    } catch (cause) {
      setBulkReviewError(
        cause instanceof Error ? cause.message : 'Failed to update the selected proposal.',
      );
    } finally {
      setUpdatingDisposition(false);
    }
  };

  return (
    <Show when={proposal()}>
      {(current) => (
        <section
          class="rounded-lg border border-blue-300 bg-blue-50/70 p-4 shadow-sm dark:border-blue-800 dark:bg-blue-950/30"
          aria-label="Suggested service verification"
        >
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-sm font-semibold text-blue-950 dark:text-blue-100">
                  Suggested service verification
                </h3>
                <span class="rounded-full border border-blue-300 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-blue-700 dark:border-blue-700 dark:text-blue-300">
                  Discovery evidence
                </span>
              </div>
              <p class="mt-1 max-w-2xl text-xs text-blue-800 dark:text-blue-200">
                Pulse inferred a useful check from this service. Review exactly what will run.
                Nothing is created until you choose the active-check action below.
              </p>
            </div>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setBulkReviewError(null);
                setShowBulkReview(true);
              }}
            >
              Review machine suggestions
            </Button>
          </div>

          <Show
            when={!dismissed()}
            fallback={
              <div class="mt-4 rounded-md border border-border bg-surface px-3 py-3">
                <p class="text-sm font-medium text-base-content">Suggestion dismissed</p>
                <p class="mt-1 text-xs text-muted">
                  It stays hidden for this evidence fingerprint. A materially changed endpoint or
                  service identity will make it reviewable again.
                </p>
                <Button
                  class="mt-3"
                  size="sm"
                  variant="secondary"
                  isLoading={updatingDisposition()}
                  onClick={() => void updateCurrentDisposition('reviewable')}
                >
                  Restore suggestion
                </Button>
              </div>
            }
          >
            <div class="mt-4 grid gap-2 sm:grid-cols-2">
              <div class="rounded-md border border-blue-200 bg-white/70 p-3 dark:border-blue-900 dark:bg-black/10">
                <p class="text-[10px] font-medium uppercase tracking-wide text-blue-600 dark:text-blue-400">
                  Endpoint · inferred
                </p>
                <code class="mt-1 block break-all text-xs text-base-content">
                  {endpointLabel(current())}
                </code>
              </div>
              <div class="rounded-md border border-blue-200 bg-white/70 p-3 dark:border-blue-900 dark:bg-black/10">
                <p class="text-[10px] font-medium uppercase tracking-wide text-blue-600 dark:text-blue-400">
                  Expected behavior · reviewed default
                </p>
                <p class="mt-1 text-xs font-medium text-base-content">
                  {expectedBehaviorLabel(current())}
                </p>
              </div>
            </div>

            <p class="mt-2 text-[11px] text-blue-700 dark:text-blue-300">
              Why Pulse proposed it: {current().reason}. The check will attach to this canonical
              resource, not a guessed hostname match.
            </p>

            <div class="mt-4 grid gap-3 sm:grid-cols-3">
              <label class="block sm:col-span-1">
                <span class="text-[11px] font-medium text-base-content">
                  Check name · you control
                </span>
                <input
                  class="mt-1 w-full rounded-md border border-border bg-surface px-2.5 py-2 text-sm text-base-content"
                  value={name()}
                  onInput={(event) => setName(event.currentTarget.value)}
                />
              </label>
              <label class="block">
                <span class="text-[11px] font-medium text-base-content">
                  Interval · you control
                </span>
                <select
                  class="mt-1 w-full rounded-md border border-border bg-surface px-2.5 py-2 text-sm text-base-content"
                  value={intervalSeconds()}
                  onChange={(event) => setIntervalSeconds(Number(event.currentTarget.value))}
                >
                  <option value={30}>Every 30 seconds</option>
                  <option value={60}>Every minute</option>
                  <option value={300}>Every 5 minutes</option>
                </select>
              </label>
              <label class="block">
                <span class="text-[11px] font-medium text-base-content">
                  Observation location · you control
                </span>
                <select
                  class="mt-1 w-full rounded-md border border-border bg-surface px-2.5 py-2 text-sm text-base-content"
                  value={probeAgentId()}
                  onChange={(event) => setProbeAgentId(event.currentTarget.value)}
                >
                  <option value="">This Pulse server</option>
                  <For each={props.connectedAgents ?? []}>
                    {(agent) => (
                      <option value={agent.agent_id}>{agent.hostname || agent.agent_id}</option>
                    )}
                  </For>
                </select>
              </label>
            </div>

            <Show when={duplicate()}>
              {(match) => (
                <div
                  class="mt-3 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-200"
                  role="status"
                >
                  {match().kind === 'endpoint'
                    ? `Already covered by “${match().target.name || match().target.address}”. Pulse will not create a duplicate endpoint check.`
                    : `This resource already has “${match().target.name || match().target.address}”, but it covers a different endpoint. Review both before adding another.`}
                </div>
              )}
            </Show>
            <Show when={targets.loading}>
              <div class="mt-3 flex items-center gap-2 text-xs text-muted" role="status">
                <LoadingSpinner size="sm" /> Checking existing active checks…
              </div>
            </Show>
            <Show when={targets.error}>
              <div class="mt-3 rounded-md border border-red-300 bg-red-50 px-3 py-2 text-xs text-red-900 dark:border-red-800 dark:bg-red-950/30 dark:text-red-200">
                <p role="alert">
                  Pulse could not check for existing active checks. Creating a check is unavailable
                  until that safety check succeeds.
                </p>
                <Button
                  class="mt-2"
                  size="sm"
                  variant="secondary"
                  onClick={() => void refetchTargets()}
                >
                  Retry existing-check scan
                </Button>
              </div>
            </Show>

            <Show when={testResult()}>
              {(result) => (
                <div
                  class={`mt-3 rounded-md border px-3 py-2 text-xs ${
                    result().success
                      ? 'border-emerald-300 bg-emerald-50 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-200'
                      : 'border-red-300 bg-red-50 text-red-900 dark:border-red-800 dark:bg-red-950/30 dark:text-red-200'
                  }`}
                  role={result().success ? 'status' : 'alert'}
                >
                  {testResultLabel(result())}
                </div>
              )}
            </Show>
            <Show when={createdTarget()}>
              {(created) => (
                <div
                  class="mt-3 flex items-center gap-2 rounded-md border border-emerald-300 bg-emerald-50 px-3 py-2 text-xs text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-200"
                  role="status"
                >
                  <CheckCircleIcon class="h-4 w-4" aria-hidden="true" />
                  Active check “{created().name}” created and attached to this resource.
                </div>
              )}
            </Show>
            <Show when={proposalError()}>
              {(message) => (
                <p class="mt-3 text-xs font-medium text-red-700 dark:text-red-300" role="alert">
                  {message()}
                </p>
              )}
            </Show>

            <div class="mt-4 flex flex-wrap items-center gap-2">
              <Button
                size="sm"
                variant="primary"
                isLoading={saving()}
                disabled={
                  targets.loading ||
                  Boolean(targets.error) ||
                  Boolean(createdTarget()) ||
                  duplicate()?.kind === 'endpoint'
                }
                onClick={() => void handleCreate()}
              >
                Create active check
              </Button>
              <Button
                size="sm"
                variant="secondary"
                isLoading={testing()}
                onClick={(event) => void handleTest(event.currentTarget)}
              >
                Test proposal
              </Button>
              <Button
                size="sm"
                variant="ghost"
                isLoading={updatingDisposition()}
                onClick={() => void updateCurrentDisposition('dismissed')}
              >
                Not now
              </Button>
            </div>
          </Show>

          <Dialog
            isOpen={showBulkReview()}
            onClose={() => setShowBulkReview(false)}
            panelClass="max-w-3xl"
            ariaLabel="Review machine assurance suggestions"
          >
            <div class="max-h-[90vh] overflow-y-auto">
              <div class="flex items-start justify-between gap-3 border-b border-border px-5 py-4">
                <div>
                  <h2 class="text-lg font-semibold text-base-content">
                    Machine assurance suggestions
                  </h2>
                  <p class="mt-1 text-sm text-muted">
                    Review the services discovered through{' '}
                    {props.discovery.hostname || props.targetId}. Dismissing is evidence-specific.
                    Creating a check still happens from its canonical resource so Pulse never
                    guesses the attachment.
                  </p>
                </div>
                <ActionIconButton
                  label="Close machine suggestions"
                  tone="muted"
                  size="md"
                  onClick={() => setShowBulkReview(false)}
                >
                  <XIcon class="h-5 w-5" aria-hidden="true" />
                </ActionIconButton>
              </div>
              <div class="space-y-3 px-5 py-4">
                <Show when={machineDiscoveries.loading}>
                  <div class="flex items-center gap-2 py-8 text-sm text-muted" role="status">
                    <LoadingSpinner size="md" /> Loading discovered services…
                  </div>
                </Show>
                <Show when={machineDiscoveries.error}>
                  <p
                    class="rounded-md border border-red-300 bg-red-50 p-4 text-sm text-red-900 dark:border-red-800 dark:bg-red-950/30 dark:text-red-200"
                    role="alert"
                  >
                    Pulse could not load this machine’s assurance suggestions. Close this review and
                    try again.
                  </p>
                </Show>
                <Show when={!machineDiscoveries.loading && !machineDiscoveries.error}>
                  <Show
                    when={bulkItems().length > 0}
                    fallback={
                      <p class="rounded-md border border-dashed border-border p-4 text-sm text-muted">
                        No availability suggestions are currently available for this machine.
                      </p>
                    }
                  >
                    <For each={bulkItems()}>
                      {(item) => {
                        const itemProposal = () => item.suggested_availability_probe!;
                        const itemDismissed = () => isAvailabilityProposalDismissed(item);
                        return (
                          <article class="rounded-md border border-border bg-surface p-3">
                            <div class="flex flex-wrap items-start justify-between gap-3">
                              <div class="min-w-0">
                                <div class="flex items-center gap-2">
                                  <MapPinIcon
                                    class="h-4 w-4 shrink-0 text-blue-500"
                                    aria-hidden="true"
                                  />
                                  <h3 class="truncate text-sm font-semibold text-base-content">
                                    {item.service_name || item.hostname || item.resource_id}
                                  </h3>
                                  <Show when={itemDismissed()}>
                                    <span class="rounded-full bg-surface-hover px-2 py-0.5 text-[10px] text-muted">
                                      Dismissed
                                    </span>
                                  </Show>
                                </div>
                                <code class="mt-1 block break-all text-xs text-muted">
                                  {endpointLabel(itemProposal())}
                                </code>
                                <p class="mt-1 text-[11px] text-muted">
                                  {expectedBehaviorLabel(itemProposal())} · {itemProposal().reason}
                                </p>
                              </div>
                              <Button
                                size="sm"
                                variant={itemDismissed() ? 'secondary' : 'ghost'}
                                disabled={updatingDisposition()}
                                onClick={() =>
                                  void updateBulkDisposition(
                                    item,
                                    itemDismissed() ? 'reviewable' : 'dismissed',
                                  )
                                }
                              >
                                {itemDismissed() ? 'Restore' : 'Dismiss'}
                              </Button>
                            </div>
                          </article>
                        );
                      }}
                    </For>
                  </Show>
                </Show>
                <Show when={bulkReviewError()}>
                  {(message) => (
                    <p class="text-sm font-medium text-red-700 dark:text-red-300" role="alert">
                      {message()}
                    </p>
                  )}
                </Show>
              </div>
            </div>
          </Dialog>
        </section>
      )}
    </Show>
  );
};
