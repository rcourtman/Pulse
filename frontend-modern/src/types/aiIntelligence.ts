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

// Real-time anomaly detection types
export type AnomalySeverity = 'low' | 'medium' | 'high' | 'critical';

export interface AnomalyReport {
    resource_id: string;
    resource_name: string;
    resource_type: string;
    metric: string;
    current_value: number;
    baseline_mean: number;
    baseline_std_dev: number;
    z_score: number;
    severity: AnomalySeverity;
    description: string;
}

export interface AnomaliesResponse extends LicenseGatedResponse {
    anomalies: AnomalyReport[];
    count: number;
    severity_counts: {
        critical: number;
        high: number;
        medium: number;
        low: number;
    };
}

// Learning/baseline status for showing system intelligence state
export type LearningStatus = 'waiting' | 'learning' | 'active';

export interface LearningStatusResponse {
    resources_baselined: number;
    total_metrics: number;
    metric_breakdown: {
        cpu?: number;
        memory?: number;
        disk?: number;
    };
    status: LearningStatus;
    message: string;
    license_required: boolean;
}


// ============================================================================
// Unified Intelligence Types (for /api/ai/intelligence endpoint)
// ============================================================================

export type HealthGrade = 'A' | 'B' | 'C' | 'D' | 'F';
export type HealthTrend = 'improving' | 'stable' | 'declining';

export interface HealthFactor {
    name: string;
    impact: number;  // -1 to 1, negative is bad, positive is good
    description: string;
    category: string;  // "finding", "prediction", "baseline", "learning"
}

export interface HealthScore {
    score: number;  // 0-100
    grade: HealthGrade;
    trend: HealthTrend;
    factors: HealthFactor[];
    prediction?: string;  // AI-predicted future state
}

export interface FindingsCounts {
    total: number;
    critical: number;
    warning: number;
    watch: number;
    info: number;
}

export interface LearningStats {
    resources_with_knowledge: number;
    total_notes: number;
    resources_with_baselines: number;
    patterns_detected: number;
    correlations_learned: number;
    incidents_tracked: number;
}

export interface ResourceRiskSummary {
    resource_id: string;
    resource_name: string;
    resource_type: string;
    health: HealthScore;
    top_issue: string;
}

// System-wide intelligence summary
export interface IntelligenceSummary {
    timestamp: string;
    overall_health: HealthScore;

    // Findings overview
    findings_count: FindingsCounts;
    top_findings?: unknown[];  // Top N findings by severity

    // Predictions overview
    predictions_count: number;
    upcoming_risks?: FailurePrediction[];

    // Recent activity
    recent_changes_count: number;
    recent_remediations?: RemediationRecord[];

    // Learning progress
    learning: LearningStats;

    // Resources needing attention
    resources_at_risk?: ResourceRiskSummary[];
}

// Per-resource intelligence
export interface ResourceIntelligence {
    resource_id: string;
    resource_name: string;
    resource_type: string;

    // Health score for this resource
    health: HealthScore;

    // Active findings for this resource
    active_findings?: unknown[];

    // Predictions for this resource
    predictions?: FailurePrediction[];

    // Correlations involving this resource
    correlations?: ResourceCorrelation[];
    dependencies?: string[];  // Resources this depends on
    dependents?: string[];    // Resources that depend on this

    // Baselines for this resource
    baselines?: Record<string, ResourceBaseline>;

    // Current anomalies
    anomalies?: AnomalyReport[];

    // Recent incidents
    recent_incidents?: unknown[];

    // Knowledge/notes
    knowledge?: unknown;
    note_count: number;
}

