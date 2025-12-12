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

// API response types
export interface PatternsResponse {
    patterns: FailurePattern[];
    count: number;
}

export interface PredictionsResponse {
    predictions: FailurePrediction[];
    count: number;
}

export interface CorrelationsResponse {
    correlations: ResourceCorrelation[];
    count: number;
}

export interface ChangesResponse {
    changes: InfrastructureChange[];
    count: number;
    hours: number;
}

export interface BaselinesResponse {
    baselines: ResourceBaseline[];
    count: number;
}
