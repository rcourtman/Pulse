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
  state?: 'active' | 'paused' | 'unauthorized' | 'unreachable' | 'stale' | 'pending';
  stateReason?: string;
  lastSeen?: string;
  error?: string;
  details?: Record<string, unknown>;
  probe?: {
    connected: boolean;
    error?: string;
    errorKind?: string;
    troubleshooting?: string;
    details?: Record<string, unknown>;
  };
}

export interface SystemDiagnostic {
  os: string;
  arch: string;
  goVersion: string;
  numCPU: number;
  numGoroutine: number;
  memoryMB: number;
  processRssMB?: number;
  heapAllocMB?: number;
  heapInUseMB?: number;
  heapIdleMB?: number;
  heapReleasedMB?: number;
  runtimeRetainedMB?: number;
  gcMemoryLimitMB?: number;
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
  history?: DiscoveryHistoryDiagnostic[];
}

export interface DiscoveryHistoryDiagnostic {
  startedAt?: string;
  completedAt?: string;
  duration?: string;
  durationMs?: number;
  subnet?: string;
  serverCount?: number;
  errorCount?: number;
  blocklistLength?: number;
  status?: string;
  [key: string]: unknown;
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

export interface AlertsOverrideDiagnostic {
  key: string;
  disabled?: boolean;
  disableConnectivity?: boolean;
  thresholds?: Record<string, number>;
}

export interface AlertsDiagnostic {
  missingCooldown: boolean;
  missingGroupingWindow: boolean;
  notes?: string[];
  overrides?: AlertsOverrideDiagnostic[];
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
  assistantRuntimeConnected: boolean;
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

const INTERNAL_ANALYTICS_DIAGNOSTICS_FIELDS = [
  'commercialFunnel',
  'infrastructureOnboarding',
] as const;

export function stripInternalAnalyticsDiagnosticsFields(raw: DiagnosticsData): DiagnosticsData {
  const data = JSON.parse(JSON.stringify(raw)) as DiagnosticsData & Record<string, unknown>;
  for (const field of INTERNAL_ANALYTICS_DIAGNOSTICS_FIELDS) {
    delete data[field];
  }
  return data;
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
  const data = stripInternalAnalyticsDiagnosticsFields(raw);
  const ipv4Re = /\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(\/\d{1,2})?\b/g;
  const redactString = (value: string): string => value.replace(ipv4Re, '[REDACTED_IP]');
  // Aliases belong to this download, not to array positions. In particular,
  // token usage and memory breakdowns must join the same anonymised identity
  // as the corresponding summary even when their arrays have different orders.
  const aliases = (prefix: string) => {
    const values = new Map<string, string>();
    return (value: unknown): string => {
      if (typeof value !== 'string' || !value) return '[REDACTED]';
      if (!values.has(value)) values.set(value, `${prefix}-${values.size + 1}`);
      return values.get(value)!;
    };
  };
  const tokenAlias = aliases('token');
  const agentAlias = aliases('docker-host');
  const instanceAlias = aliases('instance');
  const nodeAlias = aliases('host');
  const guestAlias = aliases('guest');

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
      // Probe failures and state reasons carry raw transport text such as
      // "dial tcp 192.168.1.20:8007: connect: connection refused". They were
      // added after this sanitizer was written and are exported in support
      // bundles, so they need the same redaction the top-level error gets.
      // Spread conditionally so an entry without them keeps its exact shape.
      ...(pbs.stateReason ? { stateReason: redactString(pbs.stateReason) } : {}),
      ...(pbs.probe
        ? {
            probe: {
              ...pbs.probe,
              ...(pbs.probe.error ? { error: redactString(pbs.probe.error) } : {}),
              ...(pbs.probe.troubleshooting
                ? { troubleshooting: redactString(pbs.probe.troubleshooting) }
                : {}),
            },
          }
        : {}),
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

    if (Array.isArray(data.discovery.history)) {
      data.discovery.history = data.discovery.history.map((historyEntry) => ({
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
        id: tokenAlias(token.id ?? `missing-token-${index}`),
        name: `token-${index + 1}`,
      }));
    }

    if (Array.isArray(apiTokens.usage)) {
      apiTokens.usage = apiTokens.usage.map((usage) => ({
        ...usage,
        tokenId: tokenAlias(usage.tokenId),
        // The server emits agents, not hosts. Retain the count and joins,
        // never the configured agent names or the original token ID.
        ...(Array.isArray(usage.agents) ? { agents: usage.agents.map(agentAlias) } : {}),
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
        name: agentAlias(attention.name ?? `missing-agent-${index}`),
        tokenHint: attention.tokenHint ? '[REDACTED]' : undefined,
      }));
    }
  }

  if (data.aiChat?.url) {
    data.aiChat.url = '[REDACTED]';
  }

  if (data.alerts && Array.isArray(data.alerts.overrides)) {
    data.alerts = {
      ...data.alerts,
      overrides: data.alerts.overrides.map((override, index) => ({
        ...override,
        key: `override-${index + 1}`,
      })),
    };
  }

  if (Array.isArray(data.errors)) {
    data.errors = data.errors.map(redactString);
  }

  const rawSnapshotData = data as DiagnosticsData & {
    nodeSnapshots?: Array<Record<string, unknown>>;
    guestSnapshots?: Array<Record<string, unknown>>;
    memorySources?: Array<Record<string, unknown>>;
    memorySourceBreakdown?: Array<Record<string, unknown>>;
  };

  if (Array.isArray(rawSnapshotData.nodeSnapshots)) {
    rawSnapshotData.nodeSnapshots = rawSnapshotData.nodeSnapshots.map((snapshot) => ({
      ...snapshot,
      instance: instanceAlias(snapshot.instance),
      ...(snapshot.node !== undefined ? { node: nodeAlias(snapshot.node) } : {}),
    }));
  }

  if (Array.isArray(rawSnapshotData.guestSnapshots)) {
    rawSnapshotData.guestSnapshots = rawSnapshotData.guestSnapshots.map((snapshot) => {
      const { vmid, ...rest } = snapshot;
      return {
        ...rest,
        instance: instanceAlias(snapshot.instance),
        ...(snapshot.node !== undefined ? { node: nodeAlias(snapshot.node) } : {}),
        ...(snapshot.name !== undefined
          ? {
              name: guestAlias(
                JSON.stringify([snapshot.instance, snapshot.node, vmid, snapshot.name]),
              ),
            }
          : {}),
      };
    });
  }

  if (Array.isArray(rawSnapshotData.memorySources)) {
    rawSnapshotData.memorySources = rawSnapshotData.memorySources.map((snapshot) => ({
      ...snapshot,
      instance: instanceAlias(snapshot.instance),
    }));
  }

  if (Array.isArray(rawSnapshotData.memorySourceBreakdown)) {
    rawSnapshotData.memorySourceBreakdown = rawSnapshotData.memorySourceBreakdown.map(
      (snapshot) => ({
        ...snapshot,
        instance: instanceAlias(snapshot.instance),
      }),
    );
  }

  // These nested PVE results were added after the original export redactor.
  // Keep status, permissions and measurements, but not guest names/IDs,
  // private mount paths, disk paths or raw upstream response bodies.
  for (const node of Array.isArray(data.nodes) ? data.nodes : []) {
    const nested = node as DiagnosticsNode & {
      vmDiskCheck?: Record<string, unknown>;
      physicalDisks?: Record<string, unknown>;
    };
    if (nested.vmDiskCheck) {
      const check = nested.vmDiskCheck;
      delete check.testVMID;
      if (check.testVMName !== undefined) check.testVMName = '[REDACTED]';
      if (Array.isArray(check.problematicVMs)) {
        check.problematicVMs = check.problematicVMs.map(
          ({ vmid: _vmid, name: _name, ...issue }) => ({
            ...issue,
            name: '[REDACTED]',
          }),
        );
      }
      if (Array.isArray(check.filesystemsFound)) {
        check.filesystemsFound = check.filesystemsFound.map((filesystem) => ({
          ...filesystem,
          mountpoint: '[REDACTED_PATH]',
        }));
      }
    }
    if (nested.physicalDisks && Array.isArray(nested.physicalDisks.nodeResults)) {
      nested.physicalDisks.nodeResults = nested.physicalDisks.nodeResults.map((result) => ({
        ...result,
        nodeName: nodeAlias(result.nodeName),
        ...(Array.isArray(result.diskDevices)
          ? { diskDevices: result.diskDevices.map(() => '[REDACTED_PATH]') }
          : {}),
        ...(result.apiResponse
          ? {
              apiResponse: [
                'Permission denied',
                'Timeout',
                'Endpoint not available',
                'API error',
                'Empty response (no traditional disks found)',
              ].includes(result.apiResponse)
                ? result.apiResponse
                : '[REDACTED]',
            }
          : {}),
      }));
    }
  }

  // Apply the existing IP redaction to every nested prose field, including
  // lastError, notes and recommendations. This is deliberately not a promise
  // that arbitrary error text or future fields are safe to publish: the UI
  // and help require a manual privacy review before sharing any download.
  const redactNestedText = (value: unknown): unknown => {
    if (typeof value === 'string') return redactString(value);
    if (Array.isArray(value)) return value.map(redactNestedText);
    if (value && typeof value === 'object') {
      return Object.fromEntries(
        Object.entries(value).map(([key, field]) => [key, redactNestedText(field)]),
      );
    }
    return value;
  };

  return redactNestedText(data) as DiagnosticsData;
}

export function buildDiagnosticsExportFilename(sanitize: boolean, now = new Date()): string {
  const exportType = sanitize ? 'sanitized' : 'full';
  const date = now.toISOString().split('T')[0];
  return `pulse-diagnostics-${exportType}-${date}.json`;
}
