// AI feature types

export type AIProvider =
  | 'anthropic'
  | 'openai'
  | 'openrouter'
  | 'ollama'
  | 'deepseek'
  | 'gemini'
  | 'zai'
  | 'groq'
  | 'mistral'
  | 'cerebras'
  | 'together'
  | 'fireworks';
export type AuthMethod = 'api_key' | 'oauth';
export type PatrolReadinessStatus = 'ready' | 'warning' | 'not_ready';

export interface AIProviderDefinition {
  id: AIProvider | string;
  display_name: string;
  description: string;
  protocol: 'anthropic' | 'openai_compatible' | 'gemini' | 'ollama' | 'retired' | string;
  default_model?: string;
  default_base_url?: string;
  api_key_field?: string;
  configured_field?: string;
  clear_key_field?: string;
  base_url_field?: string;
  requires_api_key: boolean;
  user_configurable: boolean;
  gateway: boolean;
  configured: boolean;
  models_dev_provider_id?: string;
  env_vars: string[];
  docs_url?: string;
  // Patrol-blessed quickstart model for providers where users must pick a
  // model themselves (Ollama). Absent for curated-catalog providers.
  suggested_model?: string;
  suggested_model_note?: string;
  suggested_model_equivalents?: string[];
}

export interface PatrolReadinessCheck {
  id: string;
  status: PatrolReadinessStatus;
  cause?: string;
  label: string;
  message: string;
  action?: string;
}

export interface PatrolReadiness {
  status: PatrolReadinessStatus;
  ready: boolean;
  cause?: string;
  summary: string;
  provider?: string;
  model?: string;
  checks: PatrolReadinessCheck[];
}

export interface ModelInfo {
  id: string;
  name: string;
  description?: string;
  is_default?: boolean;
  notable?: boolean;
  // Authoritative provider for this model, supplied by the server. Preferred
  // over deriving the provider from the (possibly opaque) model id (#1320).
  provider?: string;
}

export interface AISettings {
  enabled: boolean;
  model: string;
  chat_model?: string; // Model for interactive chat (empty = use default)
  patrol_model?: string; // Model for background patrol (empty = use default)
  discovery_model?: string; // Model for infrastructure discovery (empty = use default)
  auto_fix_model?: string; // Model for Patrol fix actions (empty = use patrol model)
  configured: boolean; // true if AI is ready to use
  custom_context: string; // user-provided infrastructure context
  // Legacy OAuth fields are retained for cleanup/migration only.
  auth_method: AuthMethod; // "api_key" or legacy "oauth"
  oauth_connected: boolean; // true if legacy OAuth tokens are stored
  // Patrol settings for token efficiency
  patrol_interval_minutes?: number; // Patrol interval in minutes (0 = disabled, minimum 10)
  patrol_enabled?: boolean; // Legacy/server-authored patrol runtime toggle still surfaced by the API
  alert_triggered_analysis?: boolean; // true if AI should analyze when alerts fire
  patrol_event_triggers_enabled?: boolean; // legacy aggregate toggle, true if any scoped Patrol trigger source is enabled
  patrol_alert_triggers_enabled?: boolean; // true if alert-driven scoped Patrol triggers are enabled
  patrol_anomaly_triggers_enabled?: boolean; // true if anomaly-driven scoped Patrol triggers are enabled
  patrol_alert_trigger_min_severity?: 'warning' | 'critical'; // minimum alert level that triggers a scoped investigation
  patrol_alert_trigger_types?: string[]; // optional allowlist of alert types (empty = all types)
  patrol_auto_fix?: boolean; // true if Patrol can remediate without approval
  // Multi-provider configuration
  anthropic_configured: boolean; // true if Anthropic API key is set
  openai_configured: boolean; // true if OpenAI API key is set
  openrouter_configured: boolean; // true if OpenRouter API key is set
  deepseek_configured: boolean; // true if DeepSeek API key is set
  gemini_configured: boolean; // true if Gemini API key is set
  zai_configured?: boolean; // true if Z.ai (Zhipu) API key is set
  groq_configured?: boolean; // true if Groq API key is set
  mistral_configured?: boolean; // true if Mistral API key is set
  cerebras_configured?: boolean; // true if Cerebras API key is set
  together_configured?: boolean; // true if Together AI API key is set
  fireworks_configured?: boolean; // true if Fireworks AI API key is set
  ollama_configured: boolean; // true (always available for attempt)
  ollama_base_url: string; // Ollama server URL
  ollama_keep_alive: string; // Ollama keep_alive value; empty uses the server default
  openai_base_url?: string; // Custom OpenAI base URL
  zai_base_url?: string; // Custom Z.ai base URL (e.g. coding endpoint)
  configured_providers: AIProvider[]; // List of providers with credentials
  providers?: AIProviderDefinition[]; // Server-authored provider registry metadata

  // Cost controls (30-day budget, pro-rated in UI)
  cost_budget_usd_30d?: number;

  // Request timeout (in seconds) - for slow Ollama hardware
  request_timeout_seconds?: number;

  // Infrastructure control settings
  control_level?: 'read_only' | 'controlled' | 'autonomous';
  protected_guests?: string[];

  // AI Discovery settings
  discovery_enabled?: boolean;
  discovery_interval_hours?: number;

  // Current Pulse Patrol runtime readiness for this settings snapshot
  patrol_readiness?: PatrolReadiness;
  // Most recent Patrol tool-call preflight result, recorded by Pulse so
  // the UI can render a "last verified" indicator without forcing
  // operators to re-run preflight on every page load. Absent when
  // preflight has never run on this Pulse instance.
  patrol_preflight?: PatrolPreflightSnapshot;
}

export interface PatrolPreflightSnapshot {
  success: boolean;
  provider?: string;
  model?: string;
  tool_call_observed: boolean;
  duration_ms: number;
  cause?: string;
  title?: string;
  summary?: string;
  recommendation?: string;
  recorded_at: string;
  recorded_at_unix: number;
}

export interface AISettingsUpdateRequest {
  enabled?: boolean;
  model?: string;
  custom_context?: string; // user-provided infrastructure context
  auth_method?: AuthMethod; // "api_key" or legacy "oauth"
  // Model overrides for different use cases
  chat_model?: string; // Model for interactive chat
  patrol_model?: string; // Model for background patrol
  discovery_model?: string; // Model for infrastructure discovery
  auto_fix_model?: string; // Model for Patrol fix actions
  // Patrol settings for token efficiency
  patrol_interval_minutes?: number; // Custom interval in minutes (0 = disabled, minimum 10)
  patrol_enabled?: boolean; // Legacy/server-authored patrol runtime toggle still accepted by the API
  alert_triggered_analysis?: boolean; // true if AI should analyze when alerts fire
  patrol_event_triggers_enabled?: boolean; // legacy aggregate toggle, applies to both scoped Patrol trigger sources
  patrol_alert_triggers_enabled?: boolean; // true if alert-driven scoped Patrol triggers are enabled
  patrol_anomaly_triggers_enabled?: boolean; // true if anomaly-driven scoped Patrol triggers are enabled
  patrol_alert_trigger_min_severity?: 'warning' | 'critical'; // minimum alert level that triggers a scoped investigation
  patrol_alert_trigger_types?: string[]; // optional allowlist of alert types (empty = all types)
  patrol_auto_fix?: boolean; // true if Patrol can remediate without approval
  // Multi-provider credentials
  anthropic_api_key?: string; // Set Anthropic API key
  openai_api_key?: string; // Set OpenAI API key
  openrouter_api_key?: string; // Set OpenRouter API key
  deepseek_api_key?: string; // Set DeepSeek API key
  gemini_api_key?: string; // Set Gemini API key
  zai_api_key?: string; // Set Z.ai (Zhipu) API key
  groq_api_key?: string; // Set Groq API key
  mistral_api_key?: string; // Set Mistral API key
  cerebras_api_key?: string; // Set Cerebras API key
  together_api_key?: string; // Set Together AI API key
  fireworks_api_key?: string; // Set Fireworks AI API key
  ollama_base_url?: string; // Set Ollama server URL
  ollama_keep_alive?: string; // Set Ollama keep_alive; empty uses the server default
  openai_base_url?: string; // Set custom OpenAI base URL
  zai_base_url?: string; // Set custom Z.ai base URL (e.g. coding endpoint)
  // Clear flags for removing credentials
  clear_anthropic_key?: boolean; // Clear Anthropic API key
  clear_openai_key?: boolean; // Clear OpenAI API key
  clear_openrouter_key?: boolean; // Clear OpenRouter API key
  clear_deepseek_key?: boolean; // Clear DeepSeek API key
  clear_gemini_key?: boolean; // Clear Gemini API key
  clear_zai_key?: boolean; // Clear Z.ai API key
  clear_groq_key?: boolean; // Clear Groq API key
  clear_mistral_key?: boolean; // Clear Mistral API key
  clear_cerebras_key?: boolean; // Clear Cerebras API key
  clear_together_key?: boolean; // Clear Together AI API key
  clear_fireworks_key?: boolean; // Clear Fireworks AI API key
  clear_ollama_url?: boolean; // Clear Ollama URL

  // Cost controls
  cost_budget_usd_30d?: number;

  // Request timeout (in seconds) - for slow Ollama hardware
  request_timeout_seconds?: number;

  // Infrastructure control settings
  control_level?: 'read_only' | 'controlled' | 'autonomous';
  protected_guests?: string[];

  // AI Discovery settings
  discovery_enabled?: boolean;
  discovery_interval_hours?: number;
}

export interface AITestResult {
  success: boolean;
  message: string;
  model?: string;
  cause?: string;
  summary?: string;
  recommendation?: string;
  action?: string;
}

export interface AIProviderTestResult {
  success: boolean;
  message: string;
  provider: AIProvider;
  model?: string;
  cause?: string;
  summary?: string;
  recommendation?: string;
  action?: string;
}

// Provider descriptions
export const PROVIDER_DESCRIPTIONS: Record<AIProvider, string> = {
  anthropic: 'Claude models from Anthropic',
  openai: 'GPT models from OpenAI',
  openrouter: 'Unified gateway for OpenAI-compatible models',
  ollama: 'Local models via Ollama',
  deepseek: 'DeepSeek V4 models',
  gemini: 'Gemini models from Google',
  zai: 'GLM models from Z.ai',
  groq: 'Hosted models from Groq',
  mistral: 'Mistral models',
  cerebras: 'Cerebras Inference models',
  together: 'Together AI hosted models',
  fireworks: 'Fireworks AI hosted models',
};

// Conversation history for multi-turn chats
export interface AIConversationMessage {
  role: 'user' | 'assistant';
  content: string;
}

// Tool execution info
export interface AIToolExecution {
  name: string; // "run_command", "read_file"
  input: string; // The command or file path
  output: string; // Result of execution
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
export type AIStreamEventType =
  | 'tool_start'
  | 'tool_progress'
  | 'tool_cancel'
  | 'tool_end'
  | 'content'
  | 'thinking'
  | 'done'
  | 'error'
  | 'complete'
  | 'approval_needed'
  | 'workflow_state'
  | 'processing';

export interface AIStreamToolStartData {
  name: string;
  input: string;
}

export interface AIStreamToolProgressData {
  name: string;
  input?: string;
  phase?: string;
  message?: string;
}

export interface AIStreamToolCancelData {
  id: string;
  name: string;
  reason?: string;
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
  target_type?: string;
  target_id?: string;
  risk?: string;
  description?: string;
  audit_id?: string;
  plan?: {
    action_id?: string;
    request_id?: string;
    summary?: string;
    requires_approval: boolean;
    approval_policy?: string;
    blast_radius?: string;
    rollback_available: boolean;
    plan_hash?: string;
    expires_at?: string;
  };
  context_confidence?: {
    level?: string;
    summary?: string;
    evidence?: string[];
  };
  preflight?: {
    target?: string;
    current_state?: string;
    intended_change?: string;
    dry_run_available: boolean;
    dry_run_summary?: string;
    safety_checks?: string[];
    verification_steps?: string[];
    generated_at?: string;
  };
}

export interface AIStreamWorkflowStateData {
  phase: string;
  message: string;
  state?: string;
  tool?: string;
}

export interface AIStreamEvent {
  type: AIStreamEventType;
  data?:
    | string
    | AIStreamToolStartData
    | AIStreamToolProgressData
    | AIStreamToolCancelData
    | AIStreamToolEndData
    | AIStreamCompleteData
    | AIStreamApprovalNeededData
    | AIStreamWorkflowStateData;
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

// Summary returned by list endpoint (no messages)
export interface AIChatSessionHandoffResource {
  id?: string;
  name?: string;
  type?: string;
  node?: string;
}

export interface AIChatSessionHandoffSummary {
  kind?: string;
  finding_id?: string;
  run_id?: string;
  run_type?: string;
  run_status?: string;
  runtime_failure?: boolean;
  has_model_context: boolean;
  resource_count?: number;
  primary_resource?: AIChatSessionHandoffResource;
  action_count?: number;
  requires_approval?: boolean;
  last_known_approval_status?: string;
  last_known_action_state?: string;
  last_known_action_risk?: string;
  updated_at?: string;
}

export interface AIChatSessionSummary {
  id: string;
  title: string;
  message_count: number;
  created_at: string;
  updated_at: string;
  handoff_summary?: AIChatSessionHandoffSummary;
}
