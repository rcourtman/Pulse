// AI Intelligence types for patterns, predictions, and correlations

export interface FailurePattern {
    key: string;
    resource_id: string;
    event_type: string;
    occurrences: number;
    average_interval: string;
    average_duration: string;
    last_occurrence: string;
    confidence: number;
}

export interface FailurePrediction {
    resource_id: string;
    event_type: string;
    predicted_at: string;
    days_until: number;
    confidence: number;
    basis: string;
    is_overdue: boolean;
}

export interface ResourceCorrelation {
    source_id: string;
    source_name: string;
    source_type: string;
    target_id: string;
    target_name: string;
    target_type: string;
    event_pattern: string;
    occurrences: number;
    avg_delay: string;
    confidence: number;
    last_seen: string;
    description: string;
}

export interface InfrastructureChange {
    id: string;
    resource_id: string;
    resource_name: string;
    resource_type: string;
    change_type: string;
    before: unknown;
    after: unknown;
    detected_at: string;
    description: string;
}

export interface ResourceBaseline {
    key: string;
    resource_id: string;
    metric: string;
    mean: number;
    std_dev: number;
    min: number;
    max: number;
    samples: number;
    last_update: string;
}

export interface RemediationRecord {
    id: string;
    timestamp: string;
    resource_id: string;
    resource_type: string;
    resource_name: string;
    finding_id?: string;
    problem: string;
    summary?: string;  // AI-generated summary of what was achieved
    action: string;
    output?: string;
    outcome: string;
    duration_ms?: number;
    note?: string;
    automatic: boolean;
}

export interface RemediationStats {
    total: number;
    resolved: number;
    partial: number;
    failed: number;
    unknown: number;
    automatic: number;
    manual: number;
}

// API response types
interface LicenseGatedResponse {
    license_required?: boolean;
    upgrade_url?: string;
    message?: string;
}

export interface PatternsResponse extends LicenseGatedResponse {
    patterns: FailurePattern[];
    count: number;
}

export interface PredictionsResponse extends LicenseGatedResponse {
    predictions: FailurePrediction[];
    count: number;
}

export interface CorrelationsResponse extends LicenseGatedResponse {
    correlations: ResourceCorrelation[];
    count: number;
}

export interface ChangesResponse extends LicenseGatedResponse {
    changes: InfrastructureChange[];
    count: number;
    hours: number;
}

export interface BaselinesResponse extends LicenseGatedResponse {
    baselines: ResourceBaseline[];
    count: number;
}

export interface RemediationsResponse extends LicenseGatedResponse {
    remediations: RemediationRecord[];
    count: number;
    stats?: RemediationStats;
}
