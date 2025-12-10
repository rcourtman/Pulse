// AI feature types

export type AIProvider = 'anthropic' | 'openai' | 'ollama' | 'deepseek';
export type AuthMethod = 'api_key' | 'oauth';

export interface AISettings {
  enabled: boolean;
  provider: AIProvider;
  api_key_set: boolean; // API key is never exposed, just whether it's set
  model: string;
  base_url?: string;
  configured: boolean; // true if AI is ready to use
  autonomous_mode: boolean; // true if AI can execute commands without approval
  custom_context: string; // user-provided infrastructure context
  // OAuth fields for Claude Pro/Max subscription authentication
  auth_method: AuthMethod; // "api_key" or "oauth"
  oauth_connected: boolean; // true if OAuth tokens are configured
  // Patrol settings for token efficiency
  patrol_schedule_preset?: string; // "15min" | "1hr" | "6hr" | "12hr" | "daily" | "disabled"
  alert_triggered_analysis?: boolean; // true if AI should analyze when alerts fire
  patrol_auto_fix?: boolean; // true if patrol can attempt automatic remediation
}

export interface AISettingsUpdateRequest {
  enabled?: boolean;
  provider?: AIProvider;
  api_key?: string; // empty string clears, undefined preserves
  model?: string;
  base_url?: string;
  autonomous_mode?: boolean;
  custom_context?: string; // user-provided infrastructure context
  auth_method?: AuthMethod; // "api_key" or "oauth"
  // Patrol settings for token efficiency
  patrol_schedule_preset?: string; // "15min" | "1hr" | "6hr" | "12hr" | "daily" | "disabled"
  alert_triggered_analysis?: boolean; // true if AI should analyze when alerts fire
  patrol_auto_fix?: boolean; // true if patrol can attempt automatic remediation
}


export interface AITestResult {
  success: boolean;
  message: string;
  model?: string;
}

// Default models for each provider
export const DEFAULT_MODELS: Record<AIProvider, string> = {
  anthropic: 'claude-opus-4-5-20251101',
  openai: 'gpt-4o',
  ollama: 'llama3',
  deepseek: 'deepseek-reasoner',
};

// Provider display names
export const PROVIDER_NAMES: Record<AIProvider, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  ollama: 'Ollama',
  deepseek: 'DeepSeek',
};

// Provider descriptions
export const PROVIDER_DESCRIPTIONS: Record<AIProvider, string> = {
  anthropic: 'Claude models from Anthropic',
  openai: 'GPT models from OpenAI',
  ollama: 'Local models via Ollama',
  deepseek: 'DeepSeek reasoning models',
};

// Conversation history for multi-turn chats
export interface AIConversationMessage {
  role: 'user' | 'assistant';
  content: string;
}

// AI Execute request/response types
export interface AIExecuteRequest {
  prompt: string;
  target_type?: string; // "host", "container", "vm", "node"
  target_id?: string;
  context?: Record<string, unknown>;
  history?: AIConversationMessage[]; // Previous conversation messages
}

// Tool execution info
export interface AIToolExecution {
  name: string;      // "run_command", "read_file"
  input: string;     // The command or file path
  output: string;    // Result of execution
  success: boolean;
}

export interface AIExecuteResponse {
  content: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  tool_calls?: AIToolExecution[]; // Commands that were executed
}

// Streaming event types
export type AIStreamEventType = 'tool_start' | 'tool_end' | 'content' | 'thinking' | 'done' | 'error' | 'complete' | 'approval_needed' | 'processing';

export interface AIStreamToolStartData {
  name: string;
  input: string;
}

export interface AIStreamToolEndData {
  name: string;
  input: string;
  output: string;
  success: boolean;
}

export interface AIStreamApprovalNeededData {
  command: string;
  tool_id: string;
  tool_name: string;
  run_on_host: boolean;
  target_host?: string; // Explicit host to route the command to
}


export interface AIStreamEvent {
  type: AIStreamEventType;
  data?: string | AIStreamToolStartData | AIStreamToolEndData | AIStreamCompleteData | AIStreamApprovalNeededData;
}

export interface AIStreamCompleteData {
  model: string;
  input_tokens: number;
  output_tokens: number;
  tool_calls?: AIToolExecution[];
}
