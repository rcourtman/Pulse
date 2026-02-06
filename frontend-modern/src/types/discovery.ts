// Discovery types for AI-powered infrastructure discovery

export type ResourceType = 'vm' | 'lxc' | 'docker' | 'k8s' | 'host' | 'docker_vm' | 'docker_lxc';

export type ServiceCategory =
    | 'database'
    | 'web_server'
    | 'cache'
    | 'message_queue'
    | 'monitoring'
    | 'backup'
    | 'nvr'
    | 'storage'
    | 'container'
    | 'virtualizer'
    | 'network'
    | 'security'
    | 'media'
    | 'home_automation'
    | 'unknown';

export type FactCategory =
    | 'version'
    | 'config'
    | 'service'
    | 'port'
    | 'hardware'
    | 'network'
    | 'storage'
    | 'dependency'
    | 'security';

export interface DiscoveryFact {
    category: FactCategory;
    key: string;
    value: string;
    source: string;
    confidence: number;
    discovered_at: string;
}

export interface PortInfo {
    port: number;
    protocol: string;
    process: string;
    address: string;
}

export interface ResourceDiscovery {
    id: string;
    resource_type: ResourceType;
    resource_id: string;
    host_id: string;
    hostname: string;
    service_type: string;
    service_name: string;
    service_version: string;
    category: ServiceCategory;
    cli_access: string;
    facts: DiscoveryFact[];
    config_paths: string[];
    data_paths: string[];
    log_paths: string[];
    ports: PortInfo[];
    user_notes: string;
    user_secrets: Record<string, string>;
    confidence: number;
    ai_reasoning: string;
    discovered_at: string;
    updated_at: string;
    scan_duration: number;
    raw_command_output?: Record<string, string>;
    // Fingerprint tracking for just-in-time discovery
    fingerprint?: string;                // Hash when discovery was done
    fingerprinted_at?: string;           // When fingerprint was captured
    fingerprint_schema_version?: number; // Schema version when fingerprint was captured
    // Auto-suggested web interface URL
    suggested_url?: string;
    suggested_url_source_code?: string;
    suggested_url_source_detail?: string;
    suggested_url_diagnostic?: string;
}

export interface DiscoverySummary {
    id: string;
    resource_type: ResourceType;
    resource_id: string;
    host_id: string;
    hostname: string;
    service_type: string;
    service_name: string;
    service_version: string;
    category: ServiceCategory;
    confidence: number;
    has_user_notes: boolean;
    updated_at: string;
    fingerprint?: string;     // Current fingerprint
    needs_discovery?: boolean; // True if fingerprint changed
}

export interface DiscoveryProgress {
    resource_id: string;
    status: 'pending' | 'running' | 'completed' | 'failed' | 'not_started';
    current_step?: string;         // Empty when idle
    current_command?: string;      // Current command being executed
    total_steps?: number;          // 0 when idle
    completed_steps?: number;      // 0 when idle
    elapsed_ms?: number;           // Milliseconds since scan started
    percent_complete?: number;     // 0-100 percentage
    started_at?: string;           // Empty when not_started
    error?: string;
    updated_at?: string;
}

export interface DiscoveryListResponse {
    discoveries: DiscoverySummary[];
    total: number;
}

export interface DiscoveryStatus {
    running: boolean;
    last_run: string;
    interval: string;
    cache_size: number;
    ai_analyzer_set: boolean;
    scanner_set: boolean;
    store_set: boolean;
    // Fingerprint-based discovery stats
    max_discovery_age?: string;
    fingerprint_count?: number;
    last_fingerprint_scan?: string;
    changed_count?: number;  // Containers with changed fingerprints
    stale_count?: number;    // Discoveries > 30 days old
}

export interface TriggerDiscoveryRequest {
    force?: boolean;
    hostname?: string;
}

export interface UpdateNotesRequest {
    user_notes: string;
    user_secrets?: Record<string, string>;
}

export interface UpdateSettingsRequest {
    max_discovery_age_days?: number;  // Days before rediscovery (default 30)
}

// AI provider information for discovery transparency
export interface AIProviderInfo {
    provider: string;    // e.g., "anthropic", "openai", "ollama"
    model: string;       // e.g., "claude-haiku-4-5", "gpt-4o"
    is_local: boolean;   // true for ollama (local models)
    label: string;       // Human-readable label, e.g., "Local (Ollama)" or "Cloud (Anthropic)"
}

// Discovery command information
export interface DiscoveryCommand {
    name: string;        // Human-readable name
    command: string;     // The actual command
    description: string; // What this command discovers
    categories: string[]; // Categories this provides info for
    timeout?: number;    // Timeout in seconds
    optional?: boolean;  // If true, failure won't stop discovery
}

// Discovery info metadata (AI provider, commands that will run)
export interface DiscoveryInfo {
    ai_provider?: AIProviderInfo;      // Current AI provider info
    commands?: DiscoveryCommand[];     // Commands that will be run
    command_categories?: string[];     // Unique categories of commands
}
