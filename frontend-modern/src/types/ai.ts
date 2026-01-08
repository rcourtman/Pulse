// AI feature types

export type AIProvider = 'anthropic' | 'openai' | 'ollama' | 'deepseek' | 'gemini';
export type AuthMethod = 'api_key' | 'oauth';

export interface ModelInfo {
  id: string;
  name: string;
  description?: string;
  is_default?: boolean;
}

export interface AISettings {
  enabled: boolean;
  provider: AIProvider; // DEPRECATED: legacy single provider
  api_key_set: boolean; // DEPRECATED: whether legacy API key is set
  model: string;
  chat_model?: string; // Model for interactive chat (empty = use default)
  patrol_model?: string; // Model for background patrol (empty = use default)
  auto_fix_model?: string; // Model for auto-fix remediation (empty = use patrol model)
  base_url?: string; // DEPRECATED: legacy base URL
  configured: boolean; // true if AI is ready to use
  autonomous_mode: boolean; // true if AI can execute commands without approval
  custom_context: string; // user-provided infrastructure context
  // OAuth fields for Claude Pro/Max subscription authentication
  auth_method: AuthMethod; // "api_key" or "oauth"
  oauth_connected: boolean; // true if OAuth tokens are configured
  // Patrol settings for token efficiency
  patrol_schedule_preset?: string; // DEPRECATED: use patrol_interval_minutes
  patrol_interval_minutes?: number; // Patrol interval in minutes (0 = disabled, minimum 10)
  alert_triggered_analysis?: boolean; // true if AI should analyze when alerts fire
  patrol_auto_fix?: boolean; // true if patrol can attempt automatic remediation
  available_models?: ModelInfo[]; // DEPRECATED: use /api/ai/models endpoint
  // Multi-provider configuration
  anthropic_configured: boolean; // true if Anthropic API key or OAuth is set
  openai_configured: boolean; // true if OpenAI API key is set
  deepseek_configured: boolean; // true if DeepSeek API key is set
  gemini_configured: boolean; // true if Gemini API key is set
  ollama_configured: boolean; // true (always available for attempt)
  ollama_base_url: string; // Ollama server URL
  openai_base_url?: string; // Custom OpenAI base URL
  configured_providers: AIProvider[]; // List of providers with credentials

  // Cost controls (30-day budget, pro-rated in UI)
  cost_budget_usd_30d?: number;

  // Request timeout (in seconds) - for slow Ollama hardware
  request_timeout_seconds?: number;
}

export interface AISettingsUpdateRequest {
  enabled?: boolean;
  provider?: AIProvider; // DEPRECATED: use model selection instead
  api_key?: string; // DEPRECATED: use per-provider keys
  model?: string;
  base_url?: string; // DEPRECATED: use per-provider URLs
  autonomous_mode?: boolean;
  custom_context?: string; // user-provided infrastructure context
  auth_method?: AuthMethod; // "api_key" or "oauth"
  // Model overrides for different use cases
  chat_model?: string; // Model for interactive chat
  patrol_model?: string; // Model for background patrol
  auto_fix_model?: string; // Model for auto-fix remediation
  // Patrol settings for token efficiency
  patrol_schedule_preset?: string; // DEPRECATED: use patrol_interval_minutes
  patrol_interval_minutes?: number; // Custom interval in minutes (0 = disabled, minimum 10)
  alert_triggered_analysis?: boolean; // true if AI should analyze when alerts fire
  patrol_auto_fix?: boolean; // true if patrol can attempt automatic remediation
  // Multi-provider credentials
  anthropic_api_key?: string; // Set Anthropic API key
  openai_api_key?: string; // Set OpenAI API key
  deepseek_api_key?: string; // Set DeepSeek API key
  gemini_api_key?: string; // Set Gemini API key
  ollama_base_url?: string; // Set Ollama server URL
  openai_base_url?: string; // Set custom OpenAI base URL
  // Clear flags for removing credentials
  clear_anthropic_key?: boolean; // Clear Anthropic API key
  clear_openai_key?: boolean; // Clear OpenAI API key
  clear_deepseek_key?: boolean; // Clear DeepSeek API key
  clear_gemini_key?: boolean; // Clear Gemini API key
  clear_ollama_url?: boolean; // Clear Ollama URL

  // Cost controls
  cost_budget_usd_30d?: number;

  // Request timeout (in seconds) - for slow Ollama hardware
  request_timeout_seconds?: number;
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
  gemini: 'gemini-2.5-flash',
};

// Provider display names
export const PROVIDER_NAMES: Record<AIProvider, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  ollama: 'Ollama',
  deepseek: 'DeepSeek',
  gemini: 'Google Gemini',
};

// Provider descriptions
export const PROVIDER_DESCRIPTIONS: Record<AIProvider, string> = {
  anthropic: 'Claude models from Anthropic',
  openai: 'GPT models from OpenAI',
  ollama: 'Local models via Ollama',
  deepseek: 'DeepSeek reasoning models',
  gemini: 'Gemini models from Google',
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
  finding_id?: string; // If fixing a patrol finding, the ID to resolve on success
  model?: string; // Override model for this request (user selection in chat)
  use_case?: 'chat' | 'patrol'; // Optional server-side routing/model selection
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
  pending_approvals?: AIStreamApprovalNeededData[]; // Non-streaming approvals
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

// AI cost/usage summary types
export interface AICostProviderModelSummary {
  provider: string;
  model: string;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  estimated_usd?: number;
  pricing_known: boolean;
}

export interface AICostDailySummary {
  date: string;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  estimated_usd?: number;
}

export interface AICostUseCaseSummary {
  use_case: string;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  estimated_usd?: number;
  pricing_known: boolean;
}

export interface AICostTargetSummary {
  target_type: string;
  target_id: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  estimated_usd?: number;
  pricing_known: boolean;
}

export interface AICostSummary {
  days: number;
  retention_days: number;
  effective_days: number;
  truncated: boolean;
  pricing_as_of?: string;
  provider_models: AICostProviderModelSummary[];
  use_cases: AICostUseCaseSummary[];
  targets: AICostTargetSummary[];
  daily_totals: AICostDailySummary[];
  totals: AICostProviderModelSummary;
}

// ============================================
// AI Chat Session Types (server-synced)
// ============================================

export interface AIChatMessageTokens {
  input: number;
  output: number;
}

export interface AIChatToolCall {
  name: string;
  input: string;
  output: string;
  success: boolean;
}

export interface AIChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
  model?: string;
  tokens?: AIChatMessageTokens;
  toolCalls?: AIChatToolCall[];
}

export interface AIChatSession {
  id: string;
  username: string;
  title: string;
  createdAt: Date;
  updatedAt: Date;
  messages: AIChatMessage[];
}

// Summary returned by list endpoint (no messages)
export interface AIChatSessionSummary {
  id: string;
  title: string;
  message_count: number;
  created_at: string;
  updated_at: string;
}
