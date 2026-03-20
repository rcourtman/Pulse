export interface DiagnosticsNode {
  id: string;
  name: string;
  host: string;
  type: string;
  authMethod: string;
  connected: boolean;
  error?: string;
  details?: Record<string, unknown>;
  lastPoll?: string;
  clusterInfo?: Record<string, unknown>;
}

export interface DiagnosticsPBS {
  id: string;
  name: string;
  host: string;
  connected: boolean;
  error?: string;
  details?: Record<string, unknown>;
}

export interface SystemDiagnostic {
  os: string;
  arch: string;
  goVersion: string;
  numCPU: number;
  numGoroutine: number;
  memoryMB: number;
}

export interface DiscoveryDiagnostic {
  enabled: boolean;
  configuredSubnet?: string;
  activeSubnet?: string;
  environmentOverride?: string;
  subnetAllowlist?: string[];
  subnetBlocklist?: string[];
  scanning?: boolean;
  scanInterval?: string;
  lastScanStartedAt?: string;
  lastResultTimestamp?: string;
  lastResultServers?: number;
  lastResultErrors?: number;
}

export interface APITokenDiagnostic {
  enabled: boolean;
  tokenCount: number;
  recommendTokenSetup: boolean;
  unusedTokenCount?: number;
  notes?: string[];
}

export interface DockerAgentDiagnostic {
  agentsTotal: number;
  agentsOnline: number;
  agentsReportingVersion: number;
  agentsWithTokenBinding: number;
  agentsWithoutTokenBinding: number;
  agentsWithoutVersion?: number;
  agentsOutdatedVersion?: number;
  agentsWithStaleCommand?: number;
  agentsPendingUninstall?: number;
  agentsNeedingAttention: number;
  recommendedAgentVersion?: string;
  notes?: string[];
}

export interface AlertsDiagnostic {
  missingCooldown: boolean;
  missingGroupingWindow: boolean;
  notes?: string[];
}

export interface MetricsStoreDiagnostic {
  enabled: boolean;
  status: 'healthy' | 'buffering' | 'empty' | 'unavailable';
  dbSize?: number;
  rawCount?: number;
  minuteCount?: number;
  hourlyCount?: number;
  dailyCount?: number;
  totalPoints?: number;
  bufferSize?: number;
  notes?: string[];
  error?: string;
}

export interface AIChatDiagnostic {
  enabled: boolean;
  running: boolean;
  healthy: boolean;
  port?: number;
  url?: string;
  model?: string;
  mcpConnected: boolean;
  mcpToolCount?: number;
  notes?: string[];
}

export interface DiagnosticsData {
  version: string;
  runtime: string;
  uptime: number;
  nodes: DiagnosticsNode[];
  pbs: DiagnosticsPBS[];
  system: SystemDiagnostic;
  metricsStore?: MetricsStoreDiagnostic | null;
  apiTokens?: APITokenDiagnostic | null;
  dockerAgents?: DockerAgentDiagnostic | null;
  alerts?: AlertsDiagnostic | null;
  aiChat?: AIChatDiagnostic | null;
  discovery?: DiscoveryDiagnostic | null;
  errors: string[];
}

export function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours < 24) return `${hours}h ${minutes}m`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

export function sanitizeDiagnosticsData(raw: DiagnosticsData): DiagnosticsData {
  const data: DiagnosticsData = JSON.parse(JSON.stringify(raw));
  const ipv4Re = /\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(\/\d{1,2})?\b/g;
  const redactString = (value: string): string => value.replace(ipv4Re, '[REDACTED_IP]');

  if (Array.isArray(data.nodes)) {
    data.nodes = data.nodes.map((node, index) => ({
      ...node,
      host: `node-${index + 1}`,
      name: `node-${index + 1}`,
      id: `node-${index + 1}`,
      error: node.error ? redactString(node.error) : undefined,
    }));
  }

  if (Array.isArray(data.pbs)) {
    data.pbs = data.pbs.map((pbs, index) => ({
      ...pbs,
      host: `pbs-${index + 1}`,
      name: `pbs-${index + 1}`,
      id: `pbs-${index + 1}`,
      error: pbs.error ? redactString(pbs.error) : undefined,
    }));
  }

  if (data.discovery) {
    data.discovery = {
      ...data.discovery,
      configuredSubnet: data.discovery.configuredSubnet ? '[REDACTED_SUBNET]' : undefined,
      activeSubnet: data.discovery.activeSubnet ? '[REDACTED_SUBNET]' : undefined,
      environmentOverride: data.discovery.environmentOverride ? '[REDACTED]' : undefined,
      subnetAllowlist: data.discovery.subnetAllowlist?.map(() => '[REDACTED_SUBNET]'),
      subnetBlocklist: data.discovery.subnetBlocklist?.map(() => '[REDACTED_SUBNET]'),
    };

    const discovery = data.discovery as DiscoveryDiagnostic & {
      history?: Array<Record<string, unknown>>;
    };
    if (Array.isArray(discovery.history)) {
      discovery.history = discovery.history.map((historyEntry) => ({
        ...historyEntry,
        subnet: '[REDACTED_SUBNET]',
      }));
    }
  }

  if (data.apiTokens) {
    const apiTokens = data.apiTokens as APITokenDiagnostic & {
      tokens?: Array<Record<string, unknown>>;
      usage?: Array<Record<string, unknown>>;
    };

    if (Array.isArray(apiTokens.tokens)) {
      apiTokens.tokens = apiTokens.tokens.map((token, index) => ({
        ...token,
        hint: '[REDACTED]',
        id: `token-${index + 1}`,
        name: `token-${index + 1}`,
      }));
    }

    if (Array.isArray(apiTokens.usage)) {
      apiTokens.usage = apiTokens.usage.map((usage) => ({
        ...usage,
        hosts: undefined,
      }));
    }
  }

  if (data.dockerAgents) {
    const dockerAgents = data.dockerAgents as DockerAgentDiagnostic & {
      attention?: Array<Record<string, unknown>>;
    };

    if (Array.isArray(dockerAgents.attention)) {
      dockerAgents.attention = dockerAgents.attention.map((attention, index) => ({
        ...attention,
        agentId: `docker-host-${index + 1}`,
        name: `docker-host-${index + 1}`,
        tokenHint: attention.tokenHint ? '[REDACTED]' : undefined,
      }));
    }
  }

  if (data.aiChat?.url) {
    data.aiChat.url = '[REDACTED]';
  }

  if (Array.isArray(data.errors)) {
    data.errors = data.errors.map(redactString);
  }

  const rawSnapshotData = data as DiagnosticsData & {
    nodeSnapshots?: Array<Record<string, unknown>>;
    guestSnapshots?: Array<Record<string, unknown>>;
    memorySources?: Array<Record<string, unknown>>;
  };

  if (Array.isArray(rawSnapshotData.nodeSnapshots)) {
    rawSnapshotData.nodeSnapshots = rawSnapshotData.nodeSnapshots.map((snapshot, index) => ({
      ...snapshot,
      instance: `node-${index + 1}`,
    }));
  }

  if (Array.isArray(rawSnapshotData.guestSnapshots)) {
    rawSnapshotData.guestSnapshots = rawSnapshotData.guestSnapshots.map((snapshot, index) => ({
      ...snapshot,
      instance: `node-${index + 1}`,
    }));
  }

  if (Array.isArray(rawSnapshotData.memorySources)) {
    rawSnapshotData.memorySources = rawSnapshotData.memorySources.map((snapshot, index) => ({
      ...snapshot,
      instance: `node-${index + 1}`,
    }));
  }

  return data;
}

export function buildDiagnosticsExportFilename(sanitize: boolean, now = new Date()): string {
  const exportType = sanitize ? 'sanitized' : 'full';
  const date = now.toISOString().split('T')[0];
  return `pulse-diagnostics-${exportType}-${date}.json`;
}
