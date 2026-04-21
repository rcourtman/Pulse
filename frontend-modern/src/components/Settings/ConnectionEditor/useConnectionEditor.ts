import { createSignal } from 'solid-js';
import {
  ConnectionsAPI,
  type ConnectionType,
  type ProbeCandidate,
  type ProbeResponse,
} from '@/api/connections';
import { DEFAULT_INFRASTRUCTURE_SOURCE_ORDER } from '@/utils/platformSupportManifest';

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

export interface ConnectionEditorState {
  address: () => string;
  setAddress: (value: string) => void;
  phase: () => ProbePhase;
  candidates: () => ProbeCandidate[];
  probedMs: () => number;
  errorMessage: () => string;
  reset: () => void;
  runProbe: () => Promise<void>;
}

type PlatformConnectionType = Extract<ConnectionType, 'pve' | 'pbs' | 'pmg' | 'truenas' | 'vmware'>;

const SOURCE_PLATFORM_TO_CONNECTION_TYPE: Partial<Record<string, PlatformConnectionType>> = {
  'vmware-vsphere': 'vmware',
  truenas: 'truenas',
  'proxmox-pve': 'pve',
  'proxmox-pbs': 'pbs',
  'proxmox-pmg': 'pmg',
};

// The supported-source manifest order is reused where it applies, but the
// add-infrastructure catalog still needs to surface the admitted vSphere path.
const CONNECTION_EDITOR_PRIORITY_TYPES: PlatformConnectionType[] = ['vmware'];

const PLATFORM_CONNECTION_TYPE_FALLBACK_ORDER: PlatformConnectionType[] = [
  'truenas',
  'pve',
  'pbs',
  'pmg',
];

export const DEFAULT_CONNECTION_EDITOR_PLATFORM_TYPES: PlatformConnectionType[] = Array.from(
  new Set([
    ...CONNECTION_EDITOR_PRIORITY_TYPES,
    ...DEFAULT_INFRASTRUCTURE_SOURCE_ORDER.flatMap((platformKey) => {
      const type = SOURCE_PLATFORM_TO_CONNECTION_TYPE[platformKey];
      return type ? [type] : [];
    }),
    ...PLATFORM_CONNECTION_TYPE_FALLBACK_ORDER,
  ]),
);

// Validation on the client side is intentionally lenient: the backend is the
// real authority on what constitutes a probeable address. We only reject the
// obviously empty case so the API does not see a payload it will always
// refuse.
function isSubmittableAddress(address: string): boolean {
  return address.trim().length > 0;
}

export function createConnectionEditorState(): ConnectionEditorState {
  const [address, setAddress] = createSignal('');
  const [phase, setPhase] = createSignal<ProbePhase>('idle');
  const [candidates, setCandidates] = createSignal<ProbeCandidate[]>([]);
  const [probedMs, setProbedMs] = createSignal(0);
  const [errorMessage, setErrorMessage] = createSignal('');

  const reset = () => {
    setAddress('');
    setPhase('idle');
    setCandidates([]);
    setProbedMs(0);
    setErrorMessage('');
  };

  const runProbe = async () => {
    const value = address().trim();
    if (!isSubmittableAddress(value)) {
      setErrorMessage('Enter an address to probe.');
      setPhase('error');
      return;
    }

    setPhase('probing');
    setErrorMessage('');
    setCandidates([]);
    setProbedMs(0);

    let response: ProbeResponse;
    try {
      response = await ConnectionsAPI.probe(value);
    } catch (error: unknown) {
      setErrorMessage(describeProbeError(error));
      setPhase('error');
      return;
    }

    setProbedMs(response.probedMs);
    setCandidates(response.candidates);
    setPhase(response.candidates.length > 0 ? 'detected' : 'no-match');
  };

  return {
    address,
    setAddress,
    phase,
    candidates,
    probedMs,
    errorMessage,
    reset,
    runProbe,
  };
}

// CONNECTION_TYPE_LABELS drives both the detected-candidate header copy and
// the manual fallback menu. Keeping one table avoids drift between probe
// results and the manual route labels.
export const CONNECTION_TYPE_LABELS: Record<ConnectionType, string> = {
  pve: 'Proxmox VE',
  pbs: 'Proxmox Backup Server',
  pmg: 'Proxmox Mail Gateway',
  vmware: 'VMware vCenter / ESXi',
  truenas: 'TrueNAS SCALE',
  agent: 'Pulse Unified Agent',
  docker: 'Docker',
  kubernetes: 'Kubernetes',
};
