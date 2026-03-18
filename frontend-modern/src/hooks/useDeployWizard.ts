/**
 * Central state machine for the agent deployment wizard.
 *
 * Manages the full lifecycle: candidates -> preflight -> confirm -> deploying -> results.
 * Internally drives two useDeployStream instances (preflight + deploy SSE).
 */

import { createSignal, createMemo, onCleanup } from 'solid-js';
import { AgentDeployAPI } from '@/api/agentDeploy';
import { useDeployStream } from '@/hooks/useDeployStream';
import { getLimit } from '@/stores/license';
import type {
  CandidateNode,
  SourceAgentInfo,
  DeployTarget,
  DeployEvent,
  DeployJobStatus,
  DeployTargetStatus,
} from '@/types/agentDeploy';
import { logger } from '@/utils/logger';

export type WizardStep = 'candidates' | 'preflight' | 'confirm' | 'deploying' | 'results';

export interface UseDeployWizardOptions {
  clusterId: string;
  clusterName: string;
}

export function useDeployWizard(opts: UseDeployWizardOptions) {
  // -- Step state --
  const [step, setStep] = createSignal<WizardStep>('candidates');

  // -- Candidates --
  const [candidates, setCandidates] = createSignal<CandidateNode[]>([]);
  const [sourceAgents, setSourceAgents] = createSignal<SourceAgentInfo[]>([]);
  const [candidatesLoading, setCandidatesLoading] = createSignal(false);
  const [candidatesError, setCandidatesError] = createSignal('');
  const [selectedSourceAgent, setSelectedSourceAgent] = createSignal('');
  const [selectedNodeIds, setSelectedNodeIds] = createSignal<Set<string>>(new Set());

  // -- Preflight --
  const [preflightId, setPreflightId] = createSignal('');
  const [preflightTargets, setPreflightTargets] = createSignal<DeployTarget[]>([]);
  const [preflightStatus, setPreflightStatus] = createSignal<DeployJobStatus | ''>('');
  const [preflightEventsUrl, setPreflightEventsUrl] = createSignal<string | null>(null);
  const [preflightError, setPreflightError] = createSignal('');

  // -- Confirm step selection (nodes user wants to deploy) --
  const [confirmSelectedNodeIds, setConfirmSelectedNodeIds] = createSignal<Set<string>>(
    new Set<string>(),
  );

  // -- Deploy --
  const [jobId, setJobId] = createSignal('');
  const [jobTargets, setJobTargets] = createSignal<DeployTarget[]>([]);
  const [jobStatus, setJobStatus] = createSignal<DeployJobStatus | ''>('');
  const [deployEventsUrl, setDeployEventsUrl] = createSignal<string | null>(null);
  const [deployError, setDeployError] = createSignal('');

  // -- Operation loading states --
  const [startingPreflight, setStartingPreflight] = createSignal(false);
  const [startingDeploy, setStartingDeploy] = createSignal(false);
  const [retrying, setRetrying] = createSignal(false);
  const [canceling, setCanceling] = createSignal(false);

  // -- SSE streams --
  const preflightStream = useDeployStream({
    eventsUrl: preflightEventsUrl,
    onEvent: (event) => handlePreflightEvent(event),
    onComplete: (status) => handlePreflightComplete(status),
    onError: (msg) => {
      setPreflightError(msg);
      // SSE retries exhausted — fallback to polling final state.
      if (preflightId()) void pollPreflightFinalState();
    },
  });

  const deployStream = useDeployStream({
    eventsUrl: deployEventsUrl,
    onEvent: (event) => handleDeployEvent(event),
    onComplete: (status) => handleDeployComplete(status),
    onError: (msg) => {
      setDeployError(msg);
      // SSE retries exhausted — fallback to polling final state.
      if (jobId()) void pollDeployFinalState();
    },
  });

  // -- Derived state --
  const deployableNodes = createMemo(() => candidates().filter((n) => n.deployable && !n.hasAgent));

  const onlineSourceAgents = createMemo(() => sourceAgents().filter((a) => a.online));

  const readyNodes = createMemo(() => preflightTargets().filter((t) => t.status === 'ready'));

  const failedPreflightNodes = createMemo(() =>
    preflightTargets().filter(
      (t) => t.status === 'failed_retryable' || t.status === 'failed_permanent',
    ),
  );

  const succeededTargets = createMemo(() =>
    jobTargets().filter(
      (t) => t.status === 'succeeded' || t.status === 'enrolling' || t.status === 'verifying',
    ),
  );

  const failedTargets = createMemo(() =>
    jobTargets().filter((t) => t.status === 'failed_retryable' || t.status === 'failed_permanent'),
  );

  const retryableTargets = createMemo(() =>
    jobTargets().filter((t) => t.status === 'failed_retryable'),
  );

  const skippedTargets = createMemo(() =>
    jobTargets().filter(
      (t) => t.status === 'skipped_already_agent' || t.status === 'skipped_license',
    ),
  );

  const canceledTargets = createMemo(() => jobTargets().filter((t) => t.status === 'canceled'));

  const maxAgentSlots = createMemo(() => {
    const limit = getLimit('max_monitored_systems');
    return limit?.limit ?? 0;
  });

  const isOperationActive = createMemo(() => step() === 'preflight' || step() === 'deploying');

  // -- Event handlers --
  //
  // SSE events are deploy.Event structs with:
  //   id, jobId, targetId, type, message, data, createdAt
  //
  // The `data` field is a JSON string containing:
  //   - Preflight complete: {ssh_reachable, pulse_reachable, has_agent, arch, error_detail}
  //   - Install result: {exit_code, output}
  //   - Empty for intermediate progress steps
  //
  // `phase` and `status` from the agent's progress payload do NOT reach the browser —
  // they are consumed server-side for target status updates in the store.
  // The frontend infers per-target status from the data blob structure and message text,
  // with authoritative state coming from the final GET poll on job_complete.

  interface PreflightResultData {
    ssh_reachable?: boolean;
    pulse_reachable?: boolean;
    has_agent?: boolean;
    arch?: string;
    error_detail?: string;
  }

  interface InstallResultData {
    exit_code?: number;
    output?: string;
  }

  function updateTargetInList(
    setter: typeof setPreflightTargets | typeof setJobTargets,
    targetId: string,
    targetStatus: DeployTargetStatus,
    errorMsg?: string,
  ) {
    setter((prev) =>
      prev.map((t) => {
        if (t.id !== targetId) return t;
        return {
          ...t,
          status: targetStatus,
          errorMessage: errorMsg ?? t.errorMessage,
        };
      }),
    );
  }

  function handlePreflightEvent(event: DeployEvent) {
    const targetId = event.targetId;
    if (!targetId) return;

    if (event.type === 'preflight_result') {
      if (event.data) {
        // Preflight complete event with structured result data.
        try {
          const result = JSON.parse(event.data) as PreflightResultData;
          if (result.has_agent) {
            updateTargetInList(setPreflightTargets, targetId, 'skipped_already_agent');
          } else if (result.ssh_reachable && result.pulse_reachable) {
            updateTargetInList(setPreflightTargets, targetId, 'ready');
            // Also update arch if provided.
            if (result.arch) {
              setPreflightTargets((prev) =>
                prev.map((t) => (t.id === targetId ? { ...t, arch: result.arch } : t)),
              );
            }
          } else {
            const detail = result.error_detail || event.message || 'Preflight check failed';
            updateTargetInList(setPreflightTargets, targetId, 'failed_permanent', detail);
          }
        } catch {
          // Structured data parse failed — use message heuristics.
          if (
            event.message?.toLowerCase().includes('fail') ||
            event.message?.toLowerCase().includes('error')
          ) {
            updateTargetInList(setPreflightTargets, targetId, 'failed_permanent', event.message);
          }
        }
      } else {
        // Intermediate preflight event (no data) — mark as in-progress if still pending.
        const tgt = preflightTargets().find((t) => t.id === targetId);
        if (tgt && tgt.status === 'pending') {
          updateTargetInList(setPreflightTargets, targetId, 'preflighting');
        }
      }
    } else if (event.type === 'error') {
      updateTargetInList(setPreflightTargets, targetId, 'failed_permanent', event.message);
    }
  }

  function handleDeployEvent(event: DeployEvent) {
    const targetId = event.targetId;
    if (!targetId) return;

    if (event.type === 'install_output') {
      if (event.data) {
        // Install result with structured data.
        try {
          const result = JSON.parse(event.data) as InstallResultData;
          if (result.exit_code !== undefined && result.exit_code !== 0) {
            const detail = result.output
              ? `Exit ${result.exit_code}: ${result.output.slice(0, 200)}`
              : event.message || `Install failed (exit ${result.exit_code})`;
            updateTargetInList(setJobTargets, targetId, 'failed_retryable', detail);
          }
          // Don't map exit_code=0 to succeeded — backend waits for enrollment.
          // Authoritative succeeded status comes from the final GET poll.
        } catch {
          // Structured data parse failed — use message heuristics.
          if (
            event.message?.toLowerCase().includes('fail') ||
            event.message?.toLowerCase().includes('error')
          ) {
            updateTargetInList(setJobTargets, targetId, 'failed_retryable', event.message);
          }
        }
      } else {
        // Intermediate install event (no data) — mark as installing if still pending/ready.
        const tgt = jobTargets().find((t) => t.id === targetId);
        if (tgt && (tgt.status === 'pending' || tgt.status === 'ready')) {
          updateTargetInList(setJobTargets, targetId, 'installing');
        }
      }
    } else if (event.type === 'enroll_complete') {
      // Enrollment done — backend has set the final status, but we optimistically
      // show enrolling until the final poll. Don't set succeeded here since the
      // backend may still be verifying.
      const tgt = jobTargets().find((t) => t.id === targetId);
      if (tgt && tgt.status === 'installing') {
        updateTargetInList(setJobTargets, targetId, 'enrolling');
      }
    } else if (event.type === 'error') {
      updateTargetInList(setJobTargets, targetId, 'failed_retryable', event.message);
    }
  }

  async function handlePreflightComplete(status: string) {
    setPreflightEventsUrl(null);
    setPreflightStatus(status as DeployJobStatus);
    // Poll GET for authoritative final state (retry once).
    for (let attempt = 0; attempt < 2; attempt++) {
      try {
        const job = await AgentDeployAPI.getPreflight(preflightId());
        setPreflightTargets(job.targets || []);
        setPreflightStatus(job.status);
        break;
      } catch (err) {
        if (attempt === 1) {
          logger.warn('[DeployWizard] Failed to poll final preflight state after retry', err);
        }
      }
    }
    // Initialize confirm selection with all ready nodes.
    const ready = preflightTargets().filter((t) => t.status === 'ready');
    setConfirmSelectedNodeIds(new Set(ready.map((t) => t.nodeId)));
    setStep('confirm');
  }

  async function handleDeployComplete(status: string) {
    setDeployEventsUrl(null);
    setJobStatus(status as DeployJobStatus);
    // Poll GET for authoritative final state (retry once).
    for (let attempt = 0; attempt < 2; attempt++) {
      try {
        const job = await AgentDeployAPI.getJob(jobId());
        setJobTargets(job.targets || []);
        setJobStatus(job.status);
        break;
      } catch (err) {
        if (attempt === 1) {
          logger.warn('[DeployWizard] Failed to poll final deploy state after retry', err);
        }
      }
    }
    setStep('results');
  }

  // Fallback polling when SSE connection is lost after max retries.
  // Polls every 5s until the job reaches a terminal state.
  const TERMINAL_JOB_STATUSES = new Set(['succeeded', 'partial_success', 'failed', 'canceled']);
  let pollTimerPreflight: ReturnType<typeof setTimeout> | undefined;
  let pollTimerDeploy: ReturnType<typeof setTimeout> | undefined;
  let disposed = false;

  function clearPollTimers() {
    if (pollTimerPreflight !== undefined) {
      clearTimeout(pollTimerPreflight);
      pollTimerPreflight = undefined;
    }
    if (pollTimerDeploy !== undefined) {
      clearTimeout(pollTimerDeploy);
      pollTimerDeploy = undefined;
    }
  }

  function pollPreflightFinalState() {
    const id = preflightId();
    if (!id || disposed) return;
    const poll = async () => {
      if (disposed) return;
      try {
        const job = await AgentDeployAPI.getPreflight(id);
        if (disposed) return; // Re-check after await
        setPreflightTargets(job.targets || []);
        setPreflightStatus(job.status);
        if (TERMINAL_JOB_STATUSES.has(job.status)) {
          pollTimerPreflight = undefined;
          handlePreflightComplete(job.status);
          return;
        }
      } catch (err) {
        if (disposed) return;
        logger.warn('[DeployWizard] Fallback poll failed for preflight', err);
      }
      if (!disposed && step() === 'preflight') {
        pollTimerPreflight = setTimeout(poll, 5000);
      }
    };
    pollTimerPreflight = setTimeout(poll, 2000);
  }

  function pollDeployFinalState() {
    const id = jobId();
    if (!id || disposed) return;
    const poll = async () => {
      if (disposed) return;
      try {
        const job = await AgentDeployAPI.getJob(id);
        if (disposed) return; // Re-check after await
        setJobTargets(job.targets || []);
        setJobStatus(job.status);
        if (TERMINAL_JOB_STATUSES.has(job.status)) {
          pollTimerDeploy = undefined;
          handleDeployComplete(job.status);
          return;
        }
      } catch (err) {
        if (disposed) return;
        logger.warn('[DeployWizard] Fallback poll failed for deploy', err);
      }
      if (!disposed && step() === 'deploying') {
        pollTimerDeploy = setTimeout(poll, 5000);
      }
    };
    pollTimerDeploy = setTimeout(poll, 2000);
  }

  // -- Actions --

  async function loadCandidates() {
    setCandidatesLoading(true);
    setCandidatesError('');
    try {
      const resp = await AgentDeployAPI.getCandidates(opts.clusterId);
      setCandidates(resp.nodes || []);
      setSourceAgents(resp.sourceAgents || []);

      // Auto-select deployable nodes.
      const deployable = (resp.nodes || []).filter((n) => n.deployable && !n.hasAgent);
      setSelectedNodeIds(new Set(deployable.map((n) => n.nodeId)));

      // Auto-select source agent if only one online.
      const online = (resp.sourceAgents || []).filter((a) => a.online);
      if (online.length === 1) {
        setSelectedSourceAgent(online[0].agentId);
      }
    } catch (err) {
      setCandidatesError(err instanceof Error ? err.message : 'Failed to load candidates');
    } finally {
      setCandidatesLoading(false);
    }
  }

  async function startPreflight() {
    if (selectedNodeIds().size === 0 || !selectedSourceAgent()) return;
    setStartingPreflight(true);
    setPreflightError('');
    try {
      const resp = await AgentDeployAPI.createPreflight(opts.clusterId, {
        sourceAgentId: selectedSourceAgent(),
        targetNodeIds: Array.from(selectedNodeIds()),
      });
      setPreflightId(resp.preflightId);
      setPreflightStatus(resp.status as DeployJobStatus);

      // Start SSE stream and transition immediately — this is the critical path.
      setPreflightEventsUrl(resp.eventsUrl);
      setStep('preflight');

      // Fetch initial targets (retry once on failure — targets must be populated
      // before SSE events arrive so updates don't get dropped).
      for (let attempt = 0; attempt < 2; attempt++) {
        try {
          const job = await AgentDeployAPI.getPreflight(resp.preflightId);
          setPreflightTargets(job.targets || []);
          break;
        } catch (err) {
          if (attempt === 1) {
            logger.warn(
              '[DeployWizard] Failed to fetch initial preflight targets after retry',
              err,
            );
          }
        }
      }
    } catch (err) {
      setPreflightError(err instanceof Error ? err.message : 'Failed to start preflight');
    } finally {
      setStartingPreflight(false);
    }
  }

  function toggleConfirmNode(nodeId: string) {
    setConfirmSelectedNodeIds((prev) => {
      const next = new Set(prev);
      if (next.has(nodeId)) {
        next.delete(nodeId);
      } else {
        next.add(nodeId);
      }
      return next;
    });
  }

  async function startDeploy(nodeIds?: string[]) {
    const targetNodeIds = nodeIds || Array.from(confirmSelectedNodeIds());
    if (targetNodeIds.length === 0) return;
    setStartingDeploy(true);
    setDeployError('');
    try {
      const resp = await AgentDeployAPI.createJob(opts.clusterId, {
        sourceAgentId: selectedSourceAgent(),
        preflightId: preflightId(),
        targetNodeIds,
      });
      setJobId(resp.jobId);

      // Start SSE stream and transition immediately — this is the critical path.
      setDeployEventsUrl(resp.eventsUrl);
      setStep('deploying');

      // Fetch initial targets (retry once on failure — targets must be populated
      // before SSE events arrive so updates don't get dropped).
      for (let attempt = 0; attempt < 2; attempt++) {
        try {
          const job = await AgentDeployAPI.getJob(resp.jobId);
          setJobTargets(job.targets || []);
          setJobStatus(job.status);
          break;
        } catch (err) {
          if (attempt === 1) {
            logger.warn('[DeployWizard] Failed to fetch initial deploy targets after retry', err);
          }
        }
      }
    } catch (err) {
      setDeployError(err instanceof Error ? err.message : 'Failed to start deployment');
    } finally {
      setStartingDeploy(false);
    }
  }

  async function cancelDeploy() {
    if (!jobId()) return;
    setCanceling(true);
    try {
      await AgentDeployAPI.cancelJob(jobId());
    } catch (err) {
      setDeployError(err instanceof Error ? err.message : 'Failed to cancel');
    } finally {
      setCanceling(false);
    }
  }

  async function retryFailed() {
    if (!jobId()) return;
    const targetIds = retryableTargets().map((t) => t.id);
    if (targetIds.length === 0) return;
    setRetrying(true);
    setDeployError('');
    try {
      const resp = await AgentDeployAPI.retryJob(jobId(), targetIds);
      setJobStatus(resp.status as DeployJobStatus);

      // Restart SSE stream and transition immediately.
      setDeployEventsUrl(resp.eventsUrl);
      setStep('deploying');

      // Refresh targets (retry once on failure).
      for (let attempt = 0; attempt < 2; attempt++) {
        try {
          const job = await AgentDeployAPI.getJob(jobId());
          setJobTargets(job.targets || []);
          break;
        } catch (err) {
          if (attempt === 1) {
            logger.warn('[DeployWizard] Failed to refresh targets after retry', err);
          }
        }
      }
    } catch (err) {
      setDeployError(err instanceof Error ? err.message : 'Failed to retry');
    } finally {
      setRetrying(false);
    }
  }

  function toggleNodeSelection(nodeId: string) {
    setSelectedNodeIds((prev) => {
      const next = new Set(prev);
      if (next.has(nodeId)) {
        next.delete(nodeId);
      } else {
        next.add(nodeId);
      }
      return next;
    });
  }

  function selectAllNodes() {
    setSelectedNodeIds(new Set(deployableNodes().map((n) => n.nodeId)));
  }

  function deselectAllNodes() {
    setSelectedNodeIds(new Set<string>());
  }

  // Load candidates immediately.
  void loadCandidates();

  // Cleanup poll timers on disposal (component unmount).
  onCleanup(() => {
    disposed = true;
    clearPollTimers();
  });

  return {
    // Step
    step,
    setStep,

    // Candidates
    candidates,
    candidatesLoading,
    candidatesError,
    sourceAgents,
    onlineSourceAgents,
    selectedSourceAgent,
    setSelectedSourceAgent,
    selectedNodeIds,
    toggleNodeSelection,
    selectAllNodes,
    deselectAllNodes,
    deployableNodes,

    // Preflight
    preflightId,
    preflightTargets,
    preflightStatus,
    preflightError,
    preflightStream,
    readyNodes,
    failedPreflightNodes,

    // Confirm step
    confirmSelectedNodeIds,
    toggleConfirmNode,

    // Deploy
    jobId,
    jobTargets,
    jobStatus,
    deployError,
    deployStream,
    succeededTargets,
    failedTargets,
    retryableTargets,
    skippedTargets,
    canceledTargets,

    // License
    maxAgentSlots,

    // Operation states
    startingPreflight,
    startingDeploy,
    retrying,
    canceling,
    isOperationActive,

    // Actions
    loadCandidates,
    startPreflight,
    startDeploy,
    cancelDeploy,
    retryFailed,
  };
}

export type DeployWizardState = ReturnType<typeof useDeployWizard>;
