import { batch, createSignal, onCleanup } from 'solid-js';
import {
  ConnectionsAPI,
  type ConnectionType,
  type ProbeCandidate,
  type ProbeResponse,
} from '@/api/connections';
import { getInfrastructureOnboardingProductPresentation } from '@/utils/infrastructureOnboardingPresentation';

const PROBE_ERROR_FALLBACK = 'Probe failed. Try again or enter credentials manually.';

function describeProbeError(error: unknown): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  if (typeof error === 'string' && error.trim().length > 0) {
    return error.trim();
  }
  return PROBE_ERROR_FALLBACK;
}

export type ProbePhase = 'idle' | 'probing' | 'detected' | 'no-match' | 'error';
export type CompletedProbePhase = Extract<ProbePhase, 'detected' | 'no-match' | 'error'>;

export interface ConnectionEditorState {
  address: () => string;
  setAddress: (value: string) => void;
  phase: () => ProbePhase;
  probing: () => boolean;
  candidates: () => ProbeCandidate[];
  probedMs: () => number;
  errorMessage: () => string;
  reset: () => void;
  runProbe: () => Promise<CompletedProbePhase | null>;
}

// CONNECTION_TYPE_LABELS drives both the detected-candidate header copy and
// the manual fallback menu. Keeping one table avoids drift between probe
// results and the manual route labels.
export const CONNECTION_TYPE_LABELS: Record<ConnectionType, string> = {
  pve: 'Proxmox VE',
  pbs: 'Proxmox Backup Server',
  pmg: 'Proxmox Mail Gateway',
  vmware: getInfrastructureOnboardingProductPresentation('vmware').label,
  truenas: getInfrastructureOnboardingProductPresentation('truenas').label,
  availability: getInfrastructureOnboardingProductPresentation('availability').label,
  agent: 'Pulse Agent',
  docker: 'Docker',
  kubernetes: 'Kubernetes',
};

// Validation on the client side is intentionally lenient: the backend is the
// real authority on what constitutes a probeable address. We only reject the
// obviously empty case so the API does not see a payload it will always
// refuse.
function isSubmittableAddress(address: string): boolean {
  return address.trim().length > 0;
}

export function createConnectionEditorState(): ConnectionEditorState {
  const [address, setAddressValue] = createSignal('');
  const [phase, setPhase] = createSignal<ProbePhase>('idle');
  const [probing, setProbing] = createSignal(false);
  const [candidates, setCandidates] = createSignal<ProbeCandidate[]>([]);
  const [probedMs, setProbedMs] = createSignal(0);
  const [errorMessage, setErrorMessage] = createSignal('');

  let generation = 0;
  let disposed = false;
  onCleanup(() => {
    disposed = true;
    generation += 1;
  });

  const clearResult = () => {
    setPhase('idle');
    setCandidates([]);
    setProbedMs(0);
    setErrorMessage('');
  };

  const setAddress = (value: string) => {
    if (disposed || value === address()) return;
    generation += 1;
    batch(() => {
      setAddressValue(value);
      clearResult();
    });
  };

  const reset = () => {
    if (disposed) return;
    // Reset is also a lifecycle boundary, even when the address is already empty.
    generation += 1;
    batch(() => {
      setAddressValue('');
      clearResult();
    });
  };

  const runProbe = async () => {
    // Invalidating evidence does not cancel the server's request. Do not admit
    // an overlapping probe in this mounted editor while it is still settling.
    if (disposed || probing()) return null;
    const value = address().trim();
    if (!isSubmittableAddress(value)) {
      setErrorMessage('Enter an API endpoint to probe.');
      setPhase('error');
      return 'error';
    }

    const currentGeneration = ++generation;
    batch(() => {
      clearResult();
      setPhase('probing');
      setProbing(true);
    });

    try {
      const response: ProbeResponse = await ConnectionsAPI.probe(value);
      if (disposed || currentGeneration !== generation) return null;
      const nextPhase: CompletedProbePhase =
        response.candidates.length > 0 ? 'detected' : 'no-match';
      batch(() => {
        setProbedMs(response.probedMs);
        setCandidates(response.candidates);
        setPhase(nextPhase);
      });
      return nextPhase;
    } catch (error: unknown) {
      if (disposed || currentGeneration !== generation) return null;
      setErrorMessage(describeProbeError(error));
      setPhase('error');
      return 'error';
    } finally {
      if (!disposed) setProbing(false);
    }
  };

  return {
    address,
    setAddress,
    phase,
    probing,
    candidates,
    probedMs,
    errorMessage,
    reset,
    runProbe,
  };
}
