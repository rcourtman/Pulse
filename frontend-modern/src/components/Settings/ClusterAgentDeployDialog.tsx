import { For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import type { Component } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { Card } from '@/components/shared/Card';
import { Dialog } from '@/components/shared/Dialog';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { SectionHeader } from '@/components/shared/SectionHeader';
import {
  ClusterAgentDeployAPI,
  isClusterDeployJobActive,
  type ClusterDeployCandidateNode,
  type ClusterDeployJob,
  type ClusterDeployTargetStatus,
} from '@/api/agentDeploy';

/**
 * One-step install of the Pulse Agent on uncovered Proxmox cluster members.
 *
 * Pulse already knows the cluster's members and has an agent on at least one
 * of them, so it asks that agent to install on its peers over the SSH trust
 * the cluster nodes already share. The server mints a per-node bootstrap
 * credential, so the operator never copies, pastes, or reuses a token. When
 * the cluster has no reporting agent to install from, or a member cannot be
 * reached, the dialog hands off to the scoped manual installer.
 */

export type ClusterAgentDeployNodeState =
  'idle' | 'checking' | 'installing' | 'connecting' | 'reporting' | 'failed';

export interface ClusterAgentDeployNodeProgress {
  state: ClusterAgentDeployNodeState;
  detail?: string;
}

/** Maps a server target status onto the four steps the operator cares about. */
export const clusterDeployNodeStateFromTarget = (
  status: ClusterDeployTargetStatus,
): ClusterAgentDeployNodeState => {
  switch (status) {
    case 'pending':
    case 'preflighting':
    case 'ready':
      return 'checking';
    case 'installing':
      return 'installing';
    case 'enrolling':
    case 'verifying':
    case 'succeeded':
    case 'skipped_already_agent':
      return 'connecting';
    case 'failed_retryable':
    case 'failed_permanent':
    case 'canceled':
      return 'failed';
    default:
      return 'checking';
  }
};

const NODE_STATE_PRESENTATION: Record<
  ClusterAgentDeployNodeState,
  { label: string; badgeClass: string }
> = {
  idle: { label: 'Not installed', badgeClass: 'bg-surface-alt text-muted' },
  checking: {
    label: 'Checking',
    badgeClass: 'bg-blue-100 text-blue-800 dark:bg-blue-900/25 dark:text-blue-200',
  },
  installing: {
    label: 'Installing',
    badgeClass: 'bg-blue-100 text-blue-800 dark:bg-blue-900/25 dark:text-blue-200',
  },
  connecting: {
    label: 'Connecting to Pulse',
    badgeClass: 'bg-blue-100 text-blue-800 dark:bg-blue-900/25 dark:text-blue-200',
  },
  reporting: {
    label: 'Reporting',
    badgeClass: 'bg-green-100 text-green-800 dark:bg-green-900/25 dark:text-green-300',
  },
  failed: {
    label: 'Failed',
    badgeClass: 'bg-red-100 text-red-800 dark:bg-red-900/25 dark:text-red-300',
  },
};

export const clusterDeployUnavailableReason = (node: ClusterDeployCandidateNode): string => {
  if (node.reason === 'no_address') {
    return 'Pulse has no IP address for this node. Set its connection address under Manage.';
  }
  return 'This node cannot be installed from here.';
};

type Phase = 'loading' | 'select' | 'running' | 'done' | 'unavailable' | 'error';

const POLL_INTERVAL_MS = 2000;
const REPORTING_TIMEOUT_MS = 90_000;
const JOB_TIMEOUT_MS = 20 * 60_000;
const REQUEST_TIMEOUT_MS = 30_000;

interface ClusterAgentDeployDialogProps {
  isOpen: boolean;
  /** The Proxmox cluster name, which the deploy API is keyed on. */
  clusterName: string;
  /** Member to preselect when opened from that member's row. */
  preselectNodeName?: string;
  onClose: () => void;
  /** Hands off to the scoped manual installer. */
  onUseInstaller: () => void;
  /** Called once at least one node is reporting, so the caller can refresh. */
  onInstalled?: () => void;
  /** Overridable for tests. */
  pollIntervalMs?: number;
  /** Overridable for tests. */
  jobTimeoutMs?: number;
  /** Overridable for tests. */
  requestTimeoutMs?: number;
}

const errorMessage = (error: unknown, fallback: string): string =>
  error instanceof Error && error.message ? error.message : fallback;

export const ClusterAgentDeployDialog: Component<ClusterAgentDeployDialogProps> = (props) => {
  const [phase, setPhase] = createSignal<Phase>('loading');
  const [nodes, setNodes] = createSignal<ClusterDeployCandidateNode[]>([]);
  const [sourceAgentId, setSourceAgentId] = createSignal('');
  const [sourceNodeName, setSourceNodeName] = createSignal('');
  const [selected, setSelected] = createSignal<ReadonlySet<string>>(new Set());
  const [progress, setProgress] = createSignal<Record<string, ClusterAgentDeployNodeProgress>>({});
  const [failure, setFailure] = createSignal('');
  let dismissButton: HTMLButtonElement | undefined;

  // A run keeps going after the dialog closes. The install continues on the
  // server, and abandoning the browser side mid-run would skip starting the
  // install after a passed check. Reopening starts a fresh view of the
  // cluster, and the server refuses an overlapping install on it.
  let runId = 0;
  const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

  const uncoveredNodes = createMemo(() => nodes().filter((node) => !node.hasAgent));
  const deployableNodes = createMemo(() => uncoveredNodes().filter((node) => node.deployable));
  const selectedCount = createMemo(
    () => deployableNodes().filter((node) => selected().has(node.nodeId)).length,
  );

  const setNodeProgress = (nodeId: string, next: ClusterAgentDeployNodeProgress) =>
    setProgress((current) => ({ ...current, [nodeId]: next }));

  const loadCandidates = async (run: number) => {
    setPhase('loading');
    setFailure('');
    setProgress({});
    try {
      const candidates = await ClusterAgentDeployAPI.getCandidates(props.clusterName);
      if (run !== runId) return;
      const candidateNodes = candidates.nodes ?? [];
      setNodes(candidateNodes);
      const source = (candidates.sourceAgents ?? []).find((agent) => agent.online);
      if (!source) {
        setPhase('unavailable');
        return;
      }
      setSourceAgentId(source.agentId);
      setSourceNodeName(
        candidateNodes.find((node) => node.nodeId === source.nodeId)?.name ?? 'a cluster node',
      );
      const deployable = candidateNodes.filter((node) => !node.hasAgent && node.deployable);
      // Opened from one member's row means that member only. If it cannot be
      // installed from here, nothing is preselected rather than its siblings.
      const preselect = props.preselectNodeName?.trim().toLowerCase();
      const chosen = preselect
        ? deployable.filter((node) => node.name.toLowerCase() === preselect)
        : deployable;
      setSelected(new Set(chosen.map((node) => node.nodeId)));
      setPhase('select');
    } catch (error) {
      if (run !== runId) return;
      setFailure(errorMessage(error, 'Pulse could not load this cluster’s nodes.'));
      setPhase('error');
    }
  };

  createEffect(() => {
    if (props.isOpen) {
      runId += 1;
      void loadCandidates(runId);
    }
  });

  // Everything a run needs is captured when it starts. The run outlives the
  // dialog when it is closed mid-install, and props can no longer be read once
  // the parent has unmounted it.
  interface RunConfig {
    run: number;
    clusterName: string;
    sourceAgentId: string;
    pollMs: number;
    jobTimeoutMs: number;
    requestTimeoutMs: number;
    onInstalled?: () => void;
  }

  // Bounds each request so a stalled one cannot hold the run past its
  // deadline or skip the final sweep.
  const withRequestDeadline = <T,>(cfg: RunConfig, request: Promise<T>): Promise<T> =>
    new Promise<T>((resolve, reject) => {
      const timer = setTimeout(
        () => reject(new Error('Pulse did not answer in time. Check this page again shortly.')),
        cfg.requestTimeoutMs,
      );
      request.then(
        (value) => {
          clearTimeout(timer);
          resolve(value);
        },
        (error: unknown) => {
          clearTimeout(timer);
          reject(error);
        },
      );
    });

  const pollUntilSettled = async (
    cfg: RunConfig,
    fetchJob: () => Promise<ClusterDeployJob>,
  ): Promise<ClusterDeployJob | null> => {
    const deadline = Date.now() + cfg.jobTimeoutMs;
    for (;;) {
      await wait(cfg.pollMs);
      if (cfg.run !== runId) return null;
      if (Date.now() > deadline) {
        throw new Error(
          'Pulse stopped waiting after 20 minutes. The install may still finish, so check this page again shortly.',
        );
      }
      const job = await withRequestDeadline(cfg, fetchJob());
      if (cfg.run !== runId) return null;
      for (const target of job.targets ?? []) {
        setNodeProgress(target.nodeId, {
          state: clusterDeployNodeStateFromTarget(target.status),
          detail: target.errorMessage,
        });
      }
      if (!isClusterDeployJobActive(job.status)) return job;
    }
  };

  // A finished job only means the agent was installed and started enrolling.
  // The outcome the operator wants is Pulse counting the node, so confirm it.
  const confirmReporting = async (cfg: RunConfig, nodeIds: string[]) => {
    const pending = new Set(nodeIds);
    const deadline = Date.now() + REPORTING_TIMEOUT_MS;
    while (pending.size > 0 && Date.now() < deadline) {
      await wait(cfg.pollMs);
      if (cfg.run !== runId) return;
      const candidates = await withRequestDeadline(
        cfg,
        ClusterAgentDeployAPI.getCandidates(cfg.clusterName),
      );
      if (cfg.run !== runId) return;
      for (const node of candidates.nodes ?? []) {
        if (pending.has(node.nodeId) && node.hasAgent) {
          pending.delete(node.nodeId);
          setNodeProgress(node.nodeId, { state: 'reporting' });
        }
      }
    }
    for (const nodeId of pending) {
      setNodeProgress(nodeId, {
        state: 'failed',
        detail: 'The agent was installed but has not reported to Pulse yet.',
      });
    }
  };

  const install = async () => {
    const cfg: RunConfig = {
      run: runId,
      clusterName: props.clusterName,
      sourceAgentId: sourceAgentId(),
      pollMs: props.pollIntervalMs ?? POLL_INTERVAL_MS,
      jobTimeoutMs: props.jobTimeoutMs ?? JOB_TIMEOUT_MS,
      requestTimeoutMs: props.requestTimeoutMs ?? REQUEST_TIMEOUT_MS,
      onInstalled: props.onInstalled,
    };
    const targetIds = deployableNodes()
      .filter((node) => selected().has(node.nodeId))
      .map((node) => node.nodeId);
    if (targetIds.length === 0) return;
    setPhase('running');
    setFailure('');
    for (const nodeId of targetIds) setNodeProgress(nodeId, { state: 'checking' });

    try {
      const preflight = await withRequestDeadline(
        cfg,
        ClusterAgentDeployAPI.createPreflight(cfg.clusterName, cfg.sourceAgentId, targetIds),
      );
      const checked = await pollUntilSettled(cfg, () =>
        ClusterAgentDeployAPI.getPreflight(preflight.preflightId),
      );
      if (!checked) return;
      const checkedTargets = checked.targets ?? [];
      const readyIds = checkedTargets
        .filter((target) => target.status === 'ready')
        .map((target) => target.nodeId);
      // A node that gained an agent since the dialog opened is skipped by the
      // check. It still needs confirming, not installing.
      const toConfirm = checkedTargets
        .filter((target) => target.status === 'skipped_already_agent')
        .map((target) => target.nodeId);
      if (readyIds.length > 0) {
        for (const nodeId of readyIds) setNodeProgress(nodeId, { state: 'installing' });
        const job = await withRequestDeadline(
          cfg,
          ClusterAgentDeployAPI.createJob(
            cfg.clusterName,
            cfg.sourceAgentId,
            preflight.preflightId,
            readyIds,
          ),
        );
        const finished = await pollUntilSettled(cfg, () => ClusterAgentDeployAPI.getJob(job.jobId));
        if (!finished) return;
        toConfirm.push(
          ...(finished.targets ?? [])
            .filter((target) => clusterDeployNodeStateFromTarget(target.status) === 'connecting')
            .map((target) => target.nodeId),
        );
      }
      if (toConfirm.length > 0) await confirmReporting(cfg, toConfirm);
    } catch (error) {
      if (cfg.run !== runId) return;
      setFailure(errorMessage(error, 'The install could not be started.'));
      for (const nodeId of targetIds) {
        if (progress()[nodeId]?.state !== 'reporting') {
          setNodeProgress(nodeId, { state: 'failed' });
        }
      }
    }
    if (cfg.run !== runId) return;
    // A node the preflight dropped or the job skipped never gets a terminal
    // status from the server, so close it out here rather than leaving it
    // looking busy forever.
    for (const nodeId of targetIds) {
      const state = progress()[nodeId]?.state;
      if (state === 'checking' || state === 'installing' || state === 'connecting') {
        setNodeProgress(nodeId, {
          state: 'failed',
          detail: 'Pulse could not finish installing on this node.',
        });
      }
    }
    setPhase('done');
    if (Object.values(progress()).some((entry) => entry.state === 'reporting')) {
      cfg.onInstalled?.();
    }
  };

  const toggle = (nodeId: string, checked: boolean) =>
    setSelected((current) => {
      const next = new Set(current);
      if (checked) next.add(nodeId);
      else next.delete(nodeId);
      return next;
    });

  const nodeState = (node: ClusterDeployCandidateNode): ClusterAgentDeployNodeState =>
    progress()[node.nodeId]?.state ?? 'idle';

  const anyFailed = createMemo(() =>
    Object.values(progress()).some((entry) => entry.state === 'failed'),
  );

  const useInstaller = () => {
    props.onClose();
    props.onUseInstaller();
  };

  return (
    <Dialog
      isOpen={props.isOpen}
      onClose={props.onClose}
      panelClass="max-w-lg"
      ariaLabel={`Install Pulse Agent on ${props.clusterName} nodes`}
    >
      <Card padding="lg" class="max-w-lg w-full">
        <SectionHeader
          title={`Install Pulse Agent on ${props.clusterName}`}
          size="md"
          class="mb-2"
        />

        <Show when={phase() === 'loading'}>
          <div class="flex items-center gap-2 py-6 text-sm text-muted">
            <LoadingSpinner size="sm" tone="muted" />
            Looking up the cluster’s nodes…
          </div>
        </Show>

        <Show when={phase() === 'unavailable'}>
          <p class="text-sm text-base-content">
            Pulse installs from a node that already runs the Pulse Agent, and none of this cluster’s
            nodes has one connected right now. Use the installer to add the first one. After that,
            the rest can be installed from here.
          </p>
        </Show>

        <Show when={phase() === 'error'}>
          <p class="text-sm text-red-700 dark:text-red-300" role="alert">
            {failure()}
          </p>
        </Show>

        <Show when={['select', 'running', 'done'].includes(phase())}>
          <p class="mb-4 text-sm text-muted">
            Pulse will install the agent from {sourceNodeName()} over the SSH access your cluster
            nodes already share. There is no token to copy.
          </p>

          <Show
            when={uncoveredNodes().length > 0}
            fallback={
              <p class="text-sm text-base-content">Every node in this cluster has the agent.</p>
            }
          >
            <ul class="space-y-2" aria-label="Cluster nodes without the Pulse Agent">
              <For each={uncoveredNodes()}>
                {(node) => (
                  <li class="rounded-md border border-border px-3 py-2">
                    <div class="flex items-center justify-between gap-3">
                      <label class="flex min-w-0 items-center gap-2 text-sm text-base-content">
                        <input
                          type="checkbox"
                          class="rounded"
                          checked={node.deployable && selected().has(node.nodeId)}
                          disabled={!node.deployable || phase() !== 'select'}
                          onChange={(event) => toggle(node.nodeId, event.currentTarget.checked)}
                        />
                        <span class="truncate font-medium">{node.name}</span>
                        <Show when={node.ip}>
                          <span class="text-xs text-muted">{node.ip}</span>
                        </Show>
                      </label>
                      <span
                        class={`shrink-0 rounded px-2 py-0.5 text-xs ${NODE_STATE_PRESENTATION[nodeState(node)].badgeClass}`}
                      >
                        <Show
                          when={['checking', 'installing', 'connecting'].includes(nodeState(node))}
                        >
                          <LoadingSpinner size="xs" tone="current" class="mr-1 inline-block" />
                        </Show>
                        {NODE_STATE_PRESENTATION[nodeState(node)].label}
                      </span>
                    </div>
                    <Show when={!node.deployable}>
                      <p class="mt-1 text-xs text-muted">{clusterDeployUnavailableReason(node)}</p>
                    </Show>
                    <Show when={progress()[node.nodeId]?.detail}>
                      <p class="mt-1 text-xs text-red-700 dark:text-red-300">
                        {progress()[node.nodeId]?.detail}
                      </p>
                    </Show>
                  </li>
                )}
              </For>
            </ul>
          </Show>

          <Show when={phase() === 'running'}>
            <p class="mt-3 text-xs text-muted">
              You can close this. The install keeps going, and each node shows up as covered once it
              reports to Pulse.
            </p>
          </Show>

          <Show when={failure()}>
            <p class="mt-3 text-sm text-red-700 dark:text-red-300" role="alert">
              {failure()}
            </p>
          </Show>
        </Show>

        <div class="mt-5 flex flex-wrap items-center justify-end gap-2">
          <Show
            when={
              phase() === 'unavailable' ||
              phase() === 'error' ||
              anyFailed() ||
              (phase() === 'select' && deployableNodes().length < uncoveredNodes().length)
            }
          >
            <Button type="button" variant="ghost" onClick={useInstaller}>
              Use the installer instead
            </Button>
          </Show>
          {/* Keep the focused dismissal control mounted through async phase changes. */}
          <Button
            ref={(element) => {
              dismissButton = element;
            }}
            type="button"
            variant="outline"
            onClick={props.onClose}
          >
            {phase() === 'select' ? 'Cancel' : 'Close'}
          </Button>
          <Show when={phase() === 'select'}>
            <Button
              type="button"
              variant="primary"
              disabled={selectedCount() === 0}
              onClick={(event) => {
                // The install action disappears while its run continues. Hand
                // keyboard focus to the stable control, without moving scroll
                // or stealing focus when this action was not focused.
                if (document.activeElement === event.currentTarget) {
                  dismissButton?.focus({ preventScroll: true });
                }
                void install();
              }}
            >
              {selectedCount() === 1 ? 'Install on 1 node' : `Install on ${selectedCount()} nodes`}
            </Button>
          </Show>
        </div>
      </Card>
    </Dialog>
  );
};
