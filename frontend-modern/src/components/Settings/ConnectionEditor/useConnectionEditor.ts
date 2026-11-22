import { batch, createSignal } from 'solid-js';
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

export type CompletedProbePhase = 'detected' | 'no-match' | 'error';
type ProbeState =
  | { phase: 'idle' | 'probing' | 'address-changed' }
  | { phase: 'detected' | 'no-match'; response: ProbeResponse }
  | { phase: 'error'; message: string };

export interface ConnectionEditorState {
  address: () => string;
  setAddress: (value: string) => void;
  phase: () => ProbeState['phase'];
  candidates: () => ProbeCandidate[];
  probedMs: () => number;
  errorMessage: () => string;
  reset: () => void;
  runProbe: () => Promise<CompletedProbePhase | undefined>;
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
  const [probe, setProbe] = createSignal<ProbeState>({ phase: 'idle' });
  const probeResponse = () => {
    const result = probe();
    return 'response' in result ? result.response : undefined;
  };

  const setAddress = (value: string) => {
    const phase = probe().phase;
    batch(() => {
      if (phase !== 'idle' && phase !== 'address-changed' && address().trim() !== value.trim()) {
        setProbe({ phase: 'address-changed' });
      }
      setAddressValue(value);
    });
  };

  const reset = () =>
    batch(() => {
      setAddressValue('');
      setProbe({ phase: 'idle' });
    });

  const runProbe = async (): Promise<CompletedProbePhase | undefined> => {
    if (probe().phase === 'probing') return;
    const value = address().trim();
    if (!isSubmittableAddress(value)) {
      setProbe({ phase: 'error', message: 'Enter an API endpoint to probe.' });
      return 'error';
    }

    // A new attempt owns its response even when the address returns to the same string.
    const pending = { phase: 'probing' } as const;
    setProbe(pending);

    let response: ProbeResponse;
    try {
      response = await ConnectionsAPI.probe(value);
    } catch (error: unknown) {
      if (probe() !== pending) return;
      setProbe({ phase: 'error', message: describeProbeError(error) });
      return 'error';
    }

    if (probe() !== pending) return;
    const nextPhase = response.candidates.length > 0 ? 'detected' : 'no-match';
    setProbe({ phase: nextPhase, response });
    return nextPhase;
  };

  return {
    address,
    setAddress,
    phase: () => probe().phase,
    candidates: () => probeResponse()?.candidates ?? [],
    probedMs: () => probeResponse()?.probedMs ?? 0,
    errorMessage: () => {
      const result = probe();
      return result.phase === 'error' ? result.message : '';
    },
    reset,
    runProbe,
  };
}
