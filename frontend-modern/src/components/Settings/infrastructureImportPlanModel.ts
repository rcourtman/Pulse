import type { ProbeCandidate } from '@/api/connections';
import type { MonitoredSystemLedgerPreviewRequest } from '@/api/monitoredSystemLedger';
import type { NodeModalFormData, NodeModalNodeType } from '@/utils/nodeModalPresentation';
import { getNodeProductName } from '@/utils/nodeModalPresentation';
import type { DiscoveredServer } from './infrastructureSettingsModel';
import { deriveNameFromHost } from './nodeModalModel';

export type NodeImportPlanCandidate =
  | {
      kind: 'discovery';
      server: DiscoveredServer;
    }
  | {
      kind: 'probe';
      candidate: ProbeCandidate;
    };

export interface NodeImportPlanStep {
  id: 'candidate' | 'credentials' | 'dry-run' | 'approval' | 'verification';
  title: string;
  detail: string;
}

export interface NodeImportPlan {
  signature: string;
  nodeType: NodeModalNodeType;
  source: 'proxmox' | 'pbs' | 'pmg';
  sourceLabel: string;
  endpoint: string;
  hostname: string;
  name: string;
  detectedVersion: string;
  candidateLabel: string;
  credentialLabel: string;
  coverageLabel: string;
  steps: NodeImportPlanStep[];
  previewRequest: MonitoredSystemLedgerPreviewRequest | null;
}

const nodeTypeToMonitoredSource = (nodeType: NodeModalNodeType): NodeImportPlan['source'] => {
  switch (nodeType) {
    case 'pve':
      return 'proxmox';
    case 'pbs':
      return 'pbs';
    case 'pmg':
      return 'pmg';
  }
};

const nodeTypeToResourceType = (nodeType: NodeModalNodeType): string => {
  switch (nodeType) {
    case 'pve':
      return 'agent';
    case 'pbs':
      return 'pbs';
    case 'pmg':
      return 'pmg';
  }
};

const normalizeEndpoint = (endpoint: string): string => endpoint.trim();

const hostnameFromEndpoint = (endpoint: string): string => {
  const trimmed = endpoint.trim();
  if (!trimmed) return '';

  try {
    const url = trimmed.includes('://') ? new URL(trimmed) : new URL(`https://${trimmed}`);
    return url.hostname || trimmed;
  } catch {
    return (
      trimmed
        .replace(/^https?:\/\//i, '')
        .split('/')[0]
        ?.split(':')[0] ?? trimmed
    );
  }
};

const endpointFromCandidate = (candidate: NodeImportPlanCandidate): string => {
  if (candidate.kind === 'probe') {
    return normalizeEndpoint(candidate.candidate.host);
  }
  const host = candidate.server.hostname?.trim() || candidate.server.ip.trim();
  return `https://${host}:${candidate.server.port}`;
};

const versionFromCandidate = (candidate: NodeImportPlanCandidate): string => {
  if (candidate.kind === 'probe') {
    const product = candidate.candidate.hints?.product?.trim();
    const version = candidate.candidate.hints?.version?.trim();
    return [product, version].filter(Boolean).join(' ') || 'Detected API endpoint';
  }
  return [candidate.server.version, candidate.server.release].filter(Boolean).join(' ');
};

const candidateLabel = (candidate: NodeImportPlanCandidate): string => {
  if (candidate.kind === 'probe') {
    return `Detected API probe at ${candidate.candidate.host}`;
  }
  const host = candidate.server.hostname?.trim() || candidate.server.ip;
  return `Discovery candidate at ${host}:${candidate.server.port}`;
};

const credentialLabel = (formData: NodeModalFormData): string => {
  if (formData.authType === 'password') {
    return 'Validate username and password credentials before saving.';
  }
  if (formData.setupMode === 'manual') {
    return 'Validate the supplied API token before saving.';
  }
  if (formData.setupMode === 'agent') {
    return 'Run the Host Telemetry Agent handoff; Pulse will add the source after the agent reports.';
  }
  return 'Run the API Inventory setup handoff; Pulse will add the source after setup completes.';
};

const coverageLabel = (nodeType: NodeModalNodeType, formData: NodeModalFormData): string => {
  if (nodeType === 'pve') {
    const enabled = [
      formData.monitorVMs ? 'VMs' : '',
      formData.monitorContainers ? 'containers' : '',
      formData.monitorStorage ? 'storage' : '',
      formData.monitorBackups ? 'backups' : '',
      formData.monitorPhysicalDisks ? 'SMART disks' : '',
    ].filter(Boolean);
    return enabled.length > 0 ? enabled.join(', ') : 'No Proxmox collectors selected';
  }
  if (nodeType === 'pbs') {
    const enabled = [
      formData.monitorDatastores ? 'datastores' : '',
      formData.monitorSyncJobs ? 'sync jobs' : '',
      formData.monitorVerifyJobs ? 'verify jobs' : '',
      formData.monitorPruneJobs ? 'prune jobs' : '',
      formData.monitorGarbageJobs ? 'garbage collection' : '',
    ].filter(Boolean);
    return enabled.length > 0 ? enabled.join(', ') : 'No PBS collectors selected';
  }
  const enabled = [
    formData.monitorMailStats ? 'mail stats' : '',
    formData.monitorQueues ? 'queues' : '',
    formData.monitorQuarantine ? 'quarantine' : '',
    formData.monitorDomainStats ? 'domain stats' : '',
  ].filter(Boolean);
  return enabled.length > 0 ? enabled.join(', ') : 'No PMG collectors selected';
};

const buildSteps = (plan: Omit<NodeImportPlan, 'steps'>): NodeImportPlanStep[] => [
  {
    id: 'candidate',
    title: 'Candidate',
    detail: plan.candidateLabel,
  },
  {
    id: 'credentials',
    title: 'Credentials',
    detail: plan.credentialLabel,
  },
  {
    id: 'dry-run',
    title: 'Dry-run impact',
    detail:
      'Preview whether this source creates a new monitored system or attaches to an existing one.',
  },
  {
    id: 'approval',
    title: 'Approval',
    detail:
      'Approve the current endpoint, credential path, and collection scope before any setup handoff or manual save.',
  },
  {
    id: 'verification',
    title: 'Verification',
    detail:
      'After the handoff or save, Pulse refreshes connected systems and discovery candidates.',
  },
];

export const buildNodeImportPlan = (
  nodeType: NodeModalNodeType,
  formData: NodeModalFormData,
  candidate: NodeImportPlanCandidate | null | undefined,
): NodeImportPlan | null => {
  if (!candidate) return null;

  const endpoint = normalizeEndpoint(formData.host) || endpointFromCandidate(candidate);
  const hostname = hostnameFromEndpoint(endpoint);
  const name = formData.name.trim() || deriveNameFromHost(endpoint) || hostname;
  const source = nodeTypeToMonitoredSource(nodeType);
  const sourceLabel = getNodeProductName(nodeType);
  const detectedVersion = versionFromCandidate(candidate);
  const candidateSummary = candidateLabel(candidate);
  const coverageSummary = coverageLabel(nodeType, formData);
  const basePlan = {
    signature: [
      source,
      candidateSummary,
      detectedVersion,
      name,
      endpoint,
      formData.authType,
      formData.setupMode,
      coverageSummary,
    ].join('|'),
    nodeType,
    source,
    sourceLabel,
    endpoint,
    hostname,
    name,
    detectedVersion,
    candidateLabel: candidateSummary,
    credentialLabel: credentialLabel(formData),
    coverageLabel: coverageSummary,
    previewRequest:
      endpoint && name
        ? {
            candidate: {
              source,
              type: nodeTypeToResourceType(nodeType),
              name,
              hostname,
              host_url: endpoint,
              resource_id: name,
            },
          }
        : null,
  };

  return {
    ...basePlan,
    steps: buildSteps(basePlan),
  };
};
