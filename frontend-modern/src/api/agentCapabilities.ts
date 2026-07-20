import { apiFetchJSON } from '@/utils/apiClient';
import type {
  AgentCapabilityActionMode,
  AgentCapabilityApprovalPolicy,
  Capability,
  CapabilityCategory,
  Manifest,
  MCPAdapterConfigFamily,
  MCPAdapterContract,
  OperatorSurfaceContract,
  PulseWorkflowPrompt,
  PulseWorkflowPromptArgument,
  SurfaceAffordanceContract,
  SurfaceContractComponent,
  SurfaceToolContract,
} from './generated/agentCapabilities';

export const AGENT_CAPABILITIES_PATH = '/api/agent/capabilities';
export const AGENT_PATROL_CONTROL_STATUS_PATH = '/api/agent/patrol-control/status';
export const AGENT_OPERATIONS_LOOP_STATUS_PATH = AGENT_PATROL_CONTROL_STATUS_PATH;

export type { AgentCapabilityActionMode, AgentCapabilityApprovalPolicy };
export type AgentCapability = Capability;
export type AgentCapabilityCategory = CapabilityCategory;
export type AgentCapabilitiesManifest = Manifest;
export type AgentMCPAdapterConfigFamily = MCPAdapterConfigFamily;
export type AgentMCPAdapterContract = MCPAdapterContract;
export type AgentOperatorSurfaceContract = OperatorSurfaceContract;
export type AgentWorkflowPrompt = PulseWorkflowPrompt;
export type AgentWorkflowPromptArgument = PulseWorkflowPromptArgument;
export type AgentSurfaceAffordanceContract = SurfaceAffordanceContract;
export type AgentSurfaceContractComponent = SurfaceContractComponent;
export type AgentSurfaceToolContract = SurfaceToolContract;

export interface AgentCapabilitySection {
  id: string;
  label: string;
  description?: string;
  entries: AgentCapability[];
}

export interface AgentSurfaceContractEntry {
  id: string;
  label: string;
  description?: string;
  badges: string[];
}

export interface AgentCapabilityErrorCodeSummary {
  code: string;
  capabilityNames: string[];
}

export interface AgentSurfaceToolPosturePresentation {
  surfaceLabel: string;
  label: string;
  title: string;
  detail?: string;
  tone: 'ready' | 'empty';
  toolCount: number;
}

export type AgentOperationsLoopNextAction =
  | 'run_patrol'
  | 'review_findings'
  | 'open_assistant'
  | 'review_approvals'
  | 'open_mcp'
  | 'complete';

export type AgentOperationsLoopStepStatus = 'complete' | 'current' | 'pending';

export type AgentOperationsLoopProActivationValueProofState =
  'not_started' | 'in_progress' | 'governed_decision_recorded' | 'verified_needs_mcp' | 'verified';

export type AgentOperationsLoopPatrolControlValueState =
  AgentOperationsLoopProActivationValueProofState;

export type AgentOperationsLoopPatrolAutonomyValueState =
  AgentOperationsLoopPatrolControlValueState;

export interface AgentOperationsLoopStep {
  id: 'patrol' | 'assistant' | 'governance' | 'verification';
  label: string;
  status: AgentOperationsLoopStepStatus;
  count?: number;
}

export interface AgentOperationsLoopStatus {
  nextAction: AgentOperationsLoopNextAction;
  progressLabel: string;
  steps: AgentOperationsLoopStep[];
  patrolEvidenceCount: number;
  patrolIssueEvidenceCount: number;
  activeFindingCount: number;
  pendingApprovalCount: number;
  governedActionCount: number;
  approvedDecisionCount: number;
  rejectedDecisionCount: number;
  verifiedOutcomeCount: number;
  operationsLoopStarterCount: number;
  assistantOperationsLoopStarterCount: number;
  patrolOperationsLoopStarterCount: number;
  patrolControlOperationsLoopStarterCount?: number;
  patrolControlCompletedOperationsLoopCount?: number;
  patrolControlResolvedOperationsLoopCount?: number;
  patrolControlValueState?: AgentOperationsLoopPatrolControlValueState;
  patrolAutonomyOperationsLoopStarterCount?: number;
  patrolAutonomyCompletedOperationsLoopCount?: number;
  patrolAutonomyResolvedOperationsLoopCount?: number;
  patrolAutonomyValueState?: AgentOperationsLoopPatrolAutonomyValueState;
  proActivationOperationsLoopStarterCount?: number;
  proActivationCompletedOperationsLoopCount?: number;
  proActivationResolvedOperationsLoopCount?: number;
  proActivationValueProofState?: AgentOperationsLoopProActivationValueProofState;
  mcpOperationsLoopStarterCount: number;
  externalAgentReady: boolean;
  windowStart: string;
  generatedAt: string;
}

export const AGENT_MCP_TOKEN_PLACEHOLDER = '<your-api-token>';
export const AGENT_SURFACE_ID_PULSE_MCP = 'pulse_mcp';
export const AGENT_WORKFLOW_PROMPT_OPERATIONS_LOOP = 'pulse_operations_loop';

const AGENT_OPERATIONS_LOOP_CAPABILITY_NAMES = [
  'get_patrol_control_status',
  'get_fleet_context',
  'get_resource_context',
  'list_findings',
  'plan_action',
  'decide_action',
  'execute_action',
  'resolve_finding',
] as const;

export interface AgentMCPOperationsLoopReadiness {
  available: boolean;
  hasAdapter: boolean;
  hasPrompt: boolean;
  hasSurfaceTools: boolean;
  missingCapabilities: string[];
}

const DEFAULT_AGENT_MCP_CONFIG_FAMILIES: AgentMCPAdapterConfigFamily[] = [
  {
    id: 'opencode',
    label: 'OpenCode',
    shape: 'opencode_mcp',
    description: "Uses OpenCode's top-level mcp object.",
    fileHints: ['opencode.json', 'opencode.jsonc', '~/.config/opencode/opencode.json'],
    clientLabels: ['OpenCode'],
  },
  {
    id: 'claude-style',
    label: 'Claude-style clients',
    shape: 'mcp_servers',
    description: 'Uses the common mcpServers object supported by Claude Desktop and Claude Code.',
    fileHints: ['~/Library/Application Support/Claude/claude_desktop_config.json', '.mcp.json'],
    clientLabels: ['Claude Desktop', 'Claude Code'],
  },
  {
    id: 'custom-mcp',
    label: 'custom clients',
    shape: 'custom',
    description:
      'Keeps the connector command, base URL flag, and token environment variable while adapting the outer client config shape.',
    clientLabels: ['custom clients'],
  },
];

const DEFAULT_AGENT_MCP_ADAPTER: AgentMCPAdapterContract = {
  serverName: 'pulse',
  command: 'pulse-mcp',
  baseUrlFlag: '--base-url',
  defaultBaseUrl: 'http://localhost:7655',
  tokenEnv: 'PULSE_API_TOKEN',
  configFamilies: DEFAULT_AGENT_MCP_CONFIG_FAMILIES,
};

const DEFAULT_AGENT_SURFACE_AFFORDANCES_BY_ID: Record<string, AgentSurfaceAffordanceContract> = {
  pulse_assistant: {
    tools: true,
    interactiveQuestions: true,
  },
  pulse_mcp: {
    tools: true,
    resources: true,
    prompts: true,
    capabilityMetadata: true,
  },
};

export async function fetchAgentCapabilitiesManifest(): Promise<AgentCapabilitiesManifest> {
  return apiFetchJSON<AgentCapabilitiesManifest>(AGENT_CAPABILITIES_PATH, {
    skipAuth: true,
    headers: { Accept: 'application/json' },
  });
}

export async function fetchAgentOperationsLoopStatus(): Promise<AgentOperationsLoopStatus> {
  return apiFetchJSON<AgentOperationsLoopStatus>(AGENT_OPERATIONS_LOOP_STATUS_PATH, {
    headers: { Accept: 'application/json' },
  });
}

function trimOrDefault(value: string | undefined, fallback: string): string {
  return value?.trim() || fallback;
}

function uniqueTrimmed(values: string[] | undefined): string[] {
  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const raw of values ?? []) {
    const value = raw.trim();
    if (!value || seen.has(value)) continue;
    seen.add(value);
    normalized.push(value);
  }
  return normalized;
}

function optionalNonEmptyList(values: string[]): string[] | undefined {
  return values.length > 0 ? values : undefined;
}

function formatSurfaceToolSource(source: string): string {
  switch (source) {
    case 'assistant_registry':
      return 'Assistant registry';
    case 'capability_manifest':
      return 'Capability manifest';
    default:
      return source
        .split(/[_\s-]+/)
        .filter(Boolean)
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(' ');
  }
}

function findAgentOperatorSurface(
  manifest: AgentCapabilitiesManifest | undefined,
  surfaceId: string,
): AgentOperatorSurfaceContract | undefined {
  const normalizedSurfaceId = surfaceId.trim();
  if (!normalizedSurfaceId) return undefined;
  return manifest?.surfaceContract?.operatorSurfaces?.find(
    (surface) => surface.id?.trim() === normalizedSurfaceId,
  );
}

function findAgentSurfaceToolContract(
  manifest: AgentCapabilitiesManifest | undefined,
  surfaceId: string,
): AgentSurfaceToolContract | undefined {
  const normalizedSurfaceId = surfaceId.trim();
  if (!normalizedSurfaceId) return undefined;
  return manifest?.surfaceToolContracts?.find(
    (contract) => contract.surfaceId?.trim() === normalizedSurfaceId,
  );
}

function cloneConfigFamilies(
  families: AgentMCPAdapterConfigFamily[],
): AgentMCPAdapterConfigFamily[] {
  return families.map((family) => ({
    ...family,
    fileHints: family.fileHints ? [...family.fileHints] : undefined,
    clientLabels: family.clientLabels ? [...family.clientLabels] : undefined,
  }));
}

function normalizedConfigFamilies(
  families: AgentMCPAdapterConfigFamily[] | undefined,
): AgentMCPAdapterConfigFamily[] {
  if (!families || families.length === 0) {
    return cloneConfigFamilies(DEFAULT_AGENT_MCP_CONFIG_FAMILIES);
  }

  const normalized: AgentMCPAdapterConfigFamily[] = [];
  for (const family of families) {
    const id = family.id?.trim() || family.label?.trim() || family.shape?.trim();
    const label = family.label?.trim() || id;
    const shape = family.shape?.trim() || id;
    if (!id || !label || !shape) continue;
    normalized.push({
      ...family,
      id,
      label,
      shape,
      description: family.description?.trim() || undefined,
      fileHints: family.fileHints?.map((hint) => hint.trim()).filter(Boolean),
      clientLabels: family.clientLabels?.map((label) => label.trim()).filter(Boolean),
    });
  }

  return normalized.length > 0
    ? normalized
    : cloneConfigFamilies(DEFAULT_AGENT_MCP_CONFIG_FAMILIES);
}

export function normalizeAgentMCPAdapter(
  adapter: AgentMCPAdapterContract | undefined,
): AgentMCPAdapterContract {
  return {
    serverName: trimOrDefault(adapter?.serverName, DEFAULT_AGENT_MCP_ADAPTER.serverName),
    command: trimOrDefault(adapter?.command, DEFAULT_AGENT_MCP_ADAPTER.command),
    baseUrlFlag: trimOrDefault(adapter?.baseUrlFlag, DEFAULT_AGENT_MCP_ADAPTER.baseUrlFlag),
    defaultBaseUrl: trimOrDefault(
      adapter?.defaultBaseUrl,
      DEFAULT_AGENT_MCP_ADAPTER.defaultBaseUrl,
    ),
    tokenEnv: trimOrDefault(adapter?.tokenEnv, DEFAULT_AGENT_MCP_ADAPTER.tokenEnv),
    configFamilies: normalizedConfigFamilies(adapter?.configFamilies),
  };
}

export function getAgentMCPAdapterConfigFamilies(
  adapter: AgentMCPAdapterContract | undefined,
): AgentMCPAdapterConfigFamily[] {
  return normalizeAgentMCPAdapter(adapter).configFamilies;
}

export function getAgentMCPConfigFamilyByShape(
  adapter: AgentMCPAdapterContract | undefined,
  shape: string,
): AgentMCPAdapterConfigFamily | undefined {
  const normalizedShape = shape.trim();
  if (!normalizedShape) return undefined;
  return getAgentMCPAdapterConfigFamilies(adapter).find(
    (family) => family.shape === normalizedShape,
  );
}

export function getAgentMCPClientExamples(
  manifest: AgentCapabilitiesManifest | undefined,
): string[] {
  const labels: string[] = [];
  for (const family of getAgentMCPAdapterConfigFamilies(manifest?.mcpAdapter)) {
    const clientLabels = family.clientLabels?.filter(Boolean);
    if (clientLabels && clientLabels.length > 0) {
      labels.push(...clientLabels);
    } else {
      labels.push(family.label);
    }
  }
  return labels;
}

export function formatAgentOpenCodeMCPConfig(
  adapter: AgentMCPAdapterContract | undefined,
  baseUrl: string,
  tokenPlaceholder = AGENT_MCP_TOKEN_PLACEHOLDER,
): string {
  const normalized = normalizeAgentMCPAdapter(adapter);
  const config = {
    $schema: 'https://opencode.ai/config.json',
    mcp: {
      [normalized.serverName]: {
        type: 'local',
        command: [normalized.command, normalized.baseUrlFlag, baseUrl],
        enabled: true,
        environment: {
          [normalized.tokenEnv]: tokenPlaceholder,
        },
      },
    },
  };
  return JSON.stringify(config, null, 2);
}

export function formatAgentMCPServersConfig(
  adapter: AgentMCPAdapterContract | undefined,
  baseUrl: string,
  tokenPlaceholder = AGENT_MCP_TOKEN_PLACEHOLDER,
): string {
  const normalized = normalizeAgentMCPAdapter(adapter);
  const config = {
    mcpServers: {
      [normalized.serverName]: {
        command: normalized.command,
        args: [normalized.baseUrlFlag, baseUrl],
        env: {
          [normalized.tokenEnv]: tokenPlaceholder,
        },
      },
    },
  };
  return JSON.stringify(config, null, 2);
}

function normalizedCapabilityCategoryID(category: string | undefined): string {
  return category?.trim() || 'uncategorized';
}

function categoryFallbackLabel(category: string): string {
  if (category === 'uncategorized') return 'Uncategorized';
  return category || 'Uncategorized';
}

function sanitizeAgentCapabilityDescription(description: string | undefined): string {
  const trimmed = description?.trim();
  if (!trimmed) return '';
  return trimmed
    .replace(/legacy Pro activation aliases/g, 'compatibility aliases')
    .replace(
      /Pro activation completed\/resolved proof/g,
      'Patrol mode completed/resolved outcome status',
    )
    .replace(
      /Patrol autonomy completed\/resolved outcome status/g,
      'Patrol mode completed/resolved outcome status',
    )
    .replace(
      /Patrol autonomy completed\/resolved outcome evidence/g,
      'Patrol mode outcome evidence',
    );
}

function normalizeAgentCapabilityPresentation(capability: AgentCapability): AgentCapability {
  return {
    ...capability,
    description: sanitizeAgentCapabilityDescription(capability.description),
  };
}

export function groupAgentCapabilitiesByManifestCategories(
  manifest: AgentCapabilitiesManifest | undefined,
): AgentCapabilitySection[] {
  if (!manifest) return [];

  const byCategory = new Map<string, AgentCapability[]>();
  for (const cap of manifest.capabilities) {
    const category = normalizedCapabilityCategoryID(cap.category);
    const list = byCategory.get(category) ?? [];
    list.push(normalizeAgentCapabilityPresentation(cap));
    byCategory.set(category, list);
  }

  const sections: AgentCapabilitySection[] = [];
  for (const category of manifest.categories ?? []) {
    const id = normalizedCapabilityCategoryID(category.id);
    const entries = byCategory.get(id);
    if (!entries || entries.length === 0) continue;
    const label = category.label?.trim() || categoryFallbackLabel(id);
    const description = category.description?.trim();
    sections.push({
      id,
      label,
      description: description || undefined,
      entries,
    });
    byCategory.delete(id);
  }

  for (const [unknownCategory, entries] of byCategory) {
    sections.push({
      id: unknownCategory,
      label: categoryFallbackLabel(unknownCategory),
      entries,
    });
  }

  return sections;
}

function trimSurfaceContractComponent(
  component: AgentSurfaceContractComponent | undefined,
): AgentSurfaceContractEntry | undefined {
  const label = component?.label?.trim();
  if (!label) return undefined;
  return {
    id: component?.id?.trim() || label,
    label,
    description: component?.description?.trim() || undefined,
    badges: [],
  };
}

function surfaceBadges(surface: AgentOperatorSurfaceContract): string[] {
  const badges: string[] = [];
  if (surface.native) badges.push('Native surface');
  if (surface.externalAdapter) badges.push('External adapter');
  badges.push(...surfaceAffordanceLabels(normalizeSurfaceAffordances(surface)));
  return badges;
}

function surfaceAffordancesDeclared(
  affordances: AgentSurfaceAffordanceContract | undefined,
): boolean {
  return Boolean(
    affordances?.tools ||
    affordances?.resources ||
    affordances?.prompts ||
    affordances?.capabilityMetadata ||
    affordances?.interactiveQuestions,
  );
}

function normalizeSurfaceAffordances(
  surface: AgentOperatorSurfaceContract,
): AgentSurfaceAffordanceContract {
  if (surfaceAffordancesDeclared(surface.affordances)) {
    return surface.affordances!;
  }
  const id = surface.id?.trim();
  return id ? (DEFAULT_AGENT_SURFACE_AFFORDANCES_BY_ID[id] ?? {}) : {};
}

function surfaceAffordanceLabels(affordances: AgentSurfaceAffordanceContract): string[] {
  const labels: string[] = [];
  if (affordances.tools) labels.push('Actions');
  if (affordances.resources) labels.push('Resources');
  if (affordances.prompts) labels.push('Prompts');
  if (affordances.capabilityMetadata) labels.push('Capability metadata');
  if (affordances.interactiveQuestions) labels.push('Interactive questions');
  return labels;
}

export function getAgentSurfaceContractEntries(
  manifest: AgentCapabilitiesManifest | undefined,
): AgentSurfaceContractEntry[] {
  if (!manifest) return [];

  const entries: AgentSurfaceContractEntry[] = [];
  const core = trimSurfaceContractComponent(manifest.surfaceContract?.core);
  if (core) entries.push(core);

  const proactiveEngine = trimSurfaceContractComponent(manifest.surfaceContract?.proactiveEngine);
  if (proactiveEngine) entries.push(proactiveEngine);

  for (const surface of manifest.surfaceContract?.operatorSurfaces ?? []) {
    const label = surface.label?.trim();
    if (!label) continue;
    entries.push({
      id: surface.id?.trim() || label,
      label,
      description: surface.description?.trim() || undefined,
      badges: surfaceBadges(surface),
    });
  }

  return entries;
}

export function getAgentWorkflowPrompts(
  manifest: AgentCapabilitiesManifest | undefined,
): AgentWorkflowPrompt[] {
  const prompts: AgentWorkflowPrompt[] = [];
  for (const prompt of manifest?.workflowPrompts ?? []) {
    const name = prompt.name?.trim();
    if (!name) continue;

    const args: AgentWorkflowPromptArgument[] = [];
    for (const arg of prompt.arguments ?? []) {
      const argName = arg.name?.trim();
      if (!argName) continue;
      const normalizedArg: AgentWorkflowPromptArgument = {
        name: argName,
        required: Boolean(arg.required),
      };
      const argDescription = arg.description?.trim();
      if (argDescription) normalizedArg.description = argDescription;
      args.push(normalizedArg);
    }

    const normalizedPrompt: AgentWorkflowPrompt = {
      name,
      arguments: args,
    };
    const label = prompt.label?.trim();
    if (label) normalizedPrompt.label = label;
    const presentationKind = prompt.presentationKind?.trim();
    if (presentationKind) normalizedPrompt.presentationKind = presentationKind;
    const description = prompt.description?.trim();
    if (description) normalizedPrompt.description = description;
    prompts.push(normalizedPrompt);
  }
  return prompts;
}

export function getAgentMCPOperationsLoopReadiness(
  manifest: AgentCapabilitiesManifest | undefined,
): AgentMCPOperationsLoopReadiness {
  const adapter = normalizeAgentMCPAdapter(manifest?.mcpAdapter);
  const hasAdapter =
    Boolean(adapter.command) &&
    Boolean(adapter.serverName) &&
    Boolean(adapter.tokenEnv) &&
    adapter.configFamilies.length > 0;
  const hasPrompt = getAgentWorkflowPrompts(manifest).some(
    (prompt) => prompt.name === AGENT_WORKFLOW_PROMPT_OPERATIONS_LOOP,
  );
  const contract = getAgentManifestSurfaceToolContract(manifest, AGENT_SURFACE_ID_PULSE_MCP);
  const toolNames = new Set(contract?.toolNames ?? []);
  const missingCapabilities = AGENT_OPERATIONS_LOOP_CAPABILITY_NAMES.filter(
    (name) => !toolNames.has(name),
  );
  const hasSurfaceTools = Boolean(contract) && missingCapabilities.length === 0;

  return {
    available: hasAdapter && hasPrompt && hasSurfaceTools,
    hasAdapter,
    hasPrompt,
    hasSurfaceTools,
    missingCapabilities,
  };
}

export function normalizeAgentSurfaceToolContract(
  contract: AgentSurfaceToolContract | undefined,
): AgentSurfaceToolContract | undefined {
  const surfaceId = contract?.surfaceId?.trim();
  if (!contract || !surfaceId) return undefined;

  const toolNames = uniqueTrimmed(contract.toolNames);
  const registryToolNames = uniqueTrimmed(contract.registryToolNames);
  const capabilityNames = uniqueTrimmed(contract.capabilityNames);
  const nativeToolNames = uniqueTrimmed(contract.nativeToolNames);
  const affordances = surfaceAffordancesDeclared(contract.affordances)
    ? contract.affordances
    : (DEFAULT_AGENT_SURFACE_AFFORDANCES_BY_ID[surfaceId] ?? undefined);
  const toolsEnabled = Boolean(affordances?.tools);

  return {
    surfaceId,
    surfaceLabel: contract.surfaceLabel?.trim() || undefined,
    toolSource: contract.toolSource?.trim() || 'unknown',
    toolNames: toolsEnabled ? toolNames : [],
    registryToolNames: optionalNonEmptyList(toolsEnabled ? registryToolNames : []),
    capabilityNames: optionalNonEmptyList(toolsEnabled ? capabilityNames : []),
    nativeToolNames: optionalNonEmptyList(toolsEnabled ? nativeToolNames : []),
    affordances,
  };
}

export function getAgentManifestSurfaceToolContract(
  manifest: AgentCapabilitiesManifest | undefined,
  surfaceId: string,
): AgentSurfaceToolContract | undefined {
  if (!manifest) return undefined;

  const normalizedSurfaceId = surfaceId.trim();
  if (!normalizedSurfaceId) return undefined;

  const surface = findAgentOperatorSurface(manifest, normalizedSurfaceId);
  if (!surface?.externalAdapter) return undefined;

  const manifestOwnedContract = findAgentSurfaceToolContract(manifest, normalizedSurfaceId);
  if (!manifestOwnedContract) return undefined;

  return normalizeAgentSurfaceToolContract({
    ...manifestOwnedContract,
    surfaceLabel: surface.label?.trim() || normalizedSurfaceId,
    affordances: normalizeSurfaceAffordances(surface),
  });
}

export function getAgentManifestSurfaceToolContracts(
  manifest: AgentCapabilitiesManifest | undefined,
): AgentSurfaceToolContract[] {
  const contracts: AgentSurfaceToolContract[] = [];
  for (const surface of manifest?.surfaceContract?.operatorSurfaces ?? []) {
    if (!surface.externalAdapter) continue;
    const contract = getAgentManifestSurfaceToolContract(manifest, surface.id);
    if (contract) contracts.push(contract);
  }
  return contracts;
}

export function getAgentSurfaceToolPosturePresentation(
  contract: AgentSurfaceToolContract | undefined,
): AgentSurfaceToolPosturePresentation | null {
  const normalized = normalizeAgentSurfaceToolContract(contract);
  if (!normalized) return null;

  const toolCount = normalized.toolNames.length;
  const registryCount = normalized.registryToolNames?.length ?? 0;
  const capabilityCount = normalized.capabilityNames?.length ?? 0;
  const nativeCount = normalized.nativeToolNames?.length ?? 0;
  const source = formatSurfaceToolSource(normalized.toolSource);
  const surfaceLabel = normalized.surfaceLabel || normalized.surfaceId;
  const label = toolCount === 1 ? '1 capability' : `${toolCount} capabilities`;
  const breakdown = [
    registryCount > 0
      ? `${registryCount} ${registryCount === 1 ? 'registry capability' : 'registry capabilities'}`
      : '',
    capabilityCount > 0
      ? `${capabilityCount} published ${capabilityCount === 1 ? 'capability' : 'capabilities'}`
      : '',
    nativeCount > 0
      ? `${nativeCount} ${nativeCount === 1 ? 'native capability' : 'native capabilities'}`
      : '',
  ].filter(Boolean);
  const affordanceLabels = surfaceAffordanceLabels(normalized.affordances ?? {});
  const detail = [...breakdown, ...affordanceLabels].join(', ');

  return {
    surfaceLabel,
    label,
    title: [`${surfaceLabel} capability availability`, `Source: ${source}`, detail || undefined]
      .filter(Boolean)
      .join('. '),
    detail: detail || undefined,
    tone: toolCount > 0 ? 'ready' : 'empty',
    toolCount,
  };
}

export function getAgentCapabilityErrorCodeSummaries(
  manifest: AgentCapabilitiesManifest | undefined,
): AgentCapabilityErrorCodeSummary[] {
  if (!manifest) return [];

  const summaries = new Map<string, Set<string>>();
  for (const capability of manifest.capabilities) {
    const capabilityName = capability.name.trim();
    if (!capabilityName) continue;

    for (const rawCode of capability.errorCodes ?? []) {
      const code = rawCode.trim();
      if (!code) continue;
      const capabilityNames = summaries.get(code) ?? new Set<string>();
      capabilityNames.add(capabilityName);
      summaries.set(code, capabilityNames);
    }
  }

  return Array.from(summaries, ([code, capabilityNames]) => ({
    code,
    capabilityNames: Array.from(capabilityNames),
  }));
}
