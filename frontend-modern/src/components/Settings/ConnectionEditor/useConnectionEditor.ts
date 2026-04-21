import { createSignal } from 'solid-js';
import {
  ConnectionsAPI,
  type ConnectionType,
  type ProbeCandidate,
  type ProbeResponse,
} from '@/api/connections';
import {
  DEFAULT_INFRASTRUCTURE_SOURCE_ORDER,
  getSourcePlatformFamily,
} from '@/utils/platformSupportManifest';

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

export type PlatformConnectionType = Extract<
  ConnectionType,
  'pve' | 'pbs' | 'pmg' | 'truenas' | 'vmware'
>;
export type ConnectionEditorCatalogFamilyId = string;

export interface ConnectionEditorCatalogTypeEntry {
  kind: 'type';
  type: PlatformConnectionType;
}

export interface ConnectionEditorCatalogFamilyEntry {
  kind: 'family';
  id: ConnectionEditorCatalogFamilyId;
  label: string;
  description: string;
  childTypes: PlatformConnectionType[];
}

export type ConnectionEditorCatalogEntry =
  | ConnectionEditorCatalogTypeEntry
  | ConnectionEditorCatalogFamilyEntry;

const CONNECTION_TYPE_TO_SOURCE_PLATFORM: Record<PlatformConnectionType, string> = {
  vmware: 'vmware-vsphere',
  truenas: 'truenas',
  pve: 'proxmox-pve',
  pbs: 'proxmox-pbs',
  pmg: 'proxmox-pmg',
};

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

const DEFAULT_CONNECTION_EDITOR_AVAILABLE_TYPES: PlatformConnectionType[] = Array.from(
  new Set([
    ...CONNECTION_EDITOR_PRIORITY_TYPES,
    ...DEFAULT_INFRASTRUCTURE_SOURCE_ORDER.flatMap((platformKey) => {
      const type = SOURCE_PLATFORM_TO_CONNECTION_TYPE[platformKey];
      return type ? [type] : [];
    }),
    ...PLATFORM_CONNECTION_TYPE_FALLBACK_ORDER,
  ]),
);

function isPlatformConnectionType(value: ConnectionType): value is PlatformConnectionType {
  return (
    value === 'pve' ||
    value === 'pbs' ||
    value === 'pmg' ||
    value === 'truenas' ||
    value === 'vmware'
  );
}

const platformTypeOrder = (type: PlatformConnectionType): number => {
  const index = DEFAULT_CONNECTION_EDITOR_AVAILABLE_TYPES.indexOf(type);
  return index === -1 ? DEFAULT_CONNECTION_EDITOR_AVAILABLE_TYPES.length : index;
};

const orderPlatformTypes = (types: readonly PlatformConnectionType[]): PlatformConnectionType[] =>
  Array.from(new Set(types)).sort(
    (left, right) => platformTypeOrder(left) - platformTypeOrder(right),
  );

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

const buildCatalogFamilyId = (familyLabel: string): ConnectionEditorCatalogFamilyId =>
  familyLabel
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-');

const getCatalogFamilyLabel = (type: PlatformConnectionType): string | null =>
  getSourcePlatformFamily(CONNECTION_TYPE_TO_SOURCE_PLATFORM[type]);

const describeCatalogFamily = (
  familyLabel: string,
  childTypes: readonly PlatformConnectionType[],
): string =>
  childTypes
    .map((type) => {
      const label = CONNECTION_TYPE_LABELS[type];
      const prefix = `${familyLabel} `;
      return label.startsWith(prefix) ? label.slice(prefix.length) : label;
    })
    .join(', ');

export const DEFAULT_CONNECTION_EDITOR_CATALOG_ENTRIES: ConnectionEditorCatalogEntry[] = (() => {
  return buildConnectionEditorCatalogEntries(DEFAULT_CONNECTION_EDITOR_AVAILABLE_TYPES);
})();

export function buildConnectionEditorCatalogEntries(
  options?: readonly ConnectionType[],
): ConnectionEditorCatalogEntry[] {
  if (!options || options.length === 0) {
    return DEFAULT_CONNECTION_EDITOR_CATALOG_ENTRIES;
  }

  const availableTypes = orderPlatformTypes(options.filter(isPlatformConnectionType));
  const entries: ConnectionEditorCatalogEntry[] = [];
  const consumedTypes = new Set<PlatformConnectionType>();

  for (const type of availableTypes) {
    if (consumedTypes.has(type)) continue;

    const familyLabel = getCatalogFamilyLabel(type);
    if (!familyLabel) {
      entries.push({ kind: 'type', type });
      consumedTypes.add(type);
      continue;
    }

    const familyChildTypes = availableTypes.filter(
      (candidateType) => getCatalogFamilyLabel(candidateType) === familyLabel,
    );
    if (familyChildTypes.length < 2) {
      entries.push({ kind: 'type', type });
      consumedTypes.add(type);
      continue;
    }

    familyChildTypes.forEach((childType) => consumedTypes.add(childType));
    entries.push({
      kind: 'family',
      id: buildCatalogFamilyId(familyLabel),
      label: familyLabel,
      description: describeCatalogFamily(familyLabel, familyChildTypes),
      childTypes: familyChildTypes,
    });
  }

  return entries;
}

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
