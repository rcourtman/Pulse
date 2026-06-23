// This file is generated from scripts/generate-types.go; DO NOT EDIT.
// Source: internal/agentcapabilities manifest structs.

/* eslint-disable */

export type AgentCapabilityActionMode = 'read' | 'mixed' | 'write';
export type AgentCapabilityApprovalPolicy = 'scope_only' | 'action_plan';

export interface Capability {
  name: string;
  title?: string;
  description: string;
  category: string;
  method: string;
  path: string;
  scope: string;
  actionMode: AgentCapabilityActionMode;
  approvalPolicy: AgentCapabilityApprovalPolicy;
  responseShape?: string;
  outputSchema?: Record<string, unknown>;
  errorCodes?: string[];
  requestBodyShape?: string;
  inputSchema?: Record<string, unknown>;
}

export interface CapabilityCategory {
  id: string;
  label: string;
  description?: string;
}

export interface MCPAdapterConfigFamily {
  id: string;
  label: string;
  shape: string;
  description?: string;
  fileHints?: string[];
  clientLabels?: string[];
}

export interface MCPAdapterContract {
  serverName: string;
  command: string;
  baseUrlFlag: string;
  defaultBaseUrl: string;
  tokenEnv: string;
  configFamilies: MCPAdapterConfigFamily[];
}

export interface Manifest {
  version: string;
  surfaceContract: SurfaceContract;
  surfaceToolContracts: SurfaceToolContract[];
  mcpAdapter: MCPAdapterContract;
  requiredScopes: string[];
  categories: CapabilityCategory[];
  workflowPrompts: PulseWorkflowPrompt[];
  capabilities: Capability[];
}

export interface OperatorSurfaceContract {
  id: string;
  label: string;
  description: string;
  native: boolean;
  externalAdapter: boolean;
  affordances?: SurfaceAffordanceContract;
}

export interface PulseWorkflowPrompt {
  name: string;
  label?: string;
  presentationKind?: string;
  description?: string;
  arguments?: PulseWorkflowPromptArgument[];
}

export interface PulseWorkflowPromptArgument {
  name: string;
  description?: string;
  required?: boolean;
}

export interface SurfaceAffordanceContract {
  tools?: boolean;
  resources?: boolean;
  prompts?: boolean;
  capabilityMetadata?: boolean;
  interactiveQuestions?: boolean;
}

export interface SurfaceContract {
  core: SurfaceContractComponent;
  proactiveEngine: SurfaceContractComponent;
  operatorSurfaces: OperatorSurfaceContract[];
}

export interface SurfaceContractComponent {
  id: string;
  label: string;
  description: string;
}

export interface SurfaceToolContract {
  surfaceId: string;
  surfaceLabel?: string;
  toolSource: string;
  toolNames: string[];
  registryToolNames?: string[];
  capabilityNames?: string[];
  nativeToolNames?: string[];
  affordances?: SurfaceAffordanceContract;
}
