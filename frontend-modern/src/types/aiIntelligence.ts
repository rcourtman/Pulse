/**
 * AI Intelligence Types
 *
 * Shared type definitions for AI intelligence features:
 * - Anomaly detection (baseline deviation)
 * - Learning status (baseline progress)
 * - Unified resource intelligence summaries
 *
 * Store logic lives in @/stores/aiIntelligence.ts
 */

import type {
  ResourceChange,
  ResourceRedactionHint,
  ResourceRoutingScope,
  ResourceSensitivity,
} from '@/types/resource';

// ============================================
// Anomaly Detection Types
// ============================================

export interface AnomalyReport {
  resource_id: string;
  resource_name: string;
  resource_type: string;
  metric: string;
  current_value: number;
  baseline_mean: number;
  baseline_std_dev: number;
  z_score: number;
  severity: string;
  description: string;
}

export interface AnomaliesResponse {
  anomalies: AnomalyReport[];
  message?: string;
}

// ============================================
// Learning Status Types
// ============================================

export interface LearningStatusResponse {
  resources_baselined: number;
  total_metrics: number;
  metric_breakdown: Record<string, number>;
  status: 'waiting' | 'learning' | 'active';
  message: string;
  license_required: boolean;
}

// ============================================
// Unified Intelligence Summary Types
// ============================================

export interface IntelligenceHealthFactor {
  name: string;
  impact: number;
  description: string;
  category: string;
}

export interface IntelligenceHealthScore {
  score: number;
  grade: 'A' | 'B' | 'C' | 'D' | 'F';
  trend: 'improving' | 'stable' | 'declining';
  factors: IntelligenceHealthFactor[];
  prediction: string;
}

export interface IntelligenceFindingsCounts {
  critical: number;
  warning: number;
  watch: number;
  info: number;
  total: number;
}

export interface IntelligenceLearningStats {
  resources_with_knowledge: number;
  total_notes: number;
  resources_with_baselines: number;
  patterns_detected: number;
  correlations_learned: number;
  incidents_tracked: number;
}

export interface IntelligencePolicyPostureSummary {
  total_resources: number;
  sensitivity_counts: Partial<Record<ResourceSensitivity, number>>;
  routing_counts: Partial<Record<ResourceRoutingScope, number>>;
  redaction_counts?: Partial<Record<ResourceRedactionHint, number>>;
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
  avg_delay: number | string;
  confidence: number;
  last_seen: string;
  description: string;
}

export interface CorrelationsResponse {
  correlations: ResourceCorrelation[];
  count: number;
  message?: string;
  license_required?: boolean;
  upgrade_url?: string;
}

export interface IntelligenceSummary {
  timestamp: string;
  overall_health: IntelligenceHealthScore;
  findings_count: IntelligenceFindingsCounts;
  predictions_count: number;
  recent_changes_count: number;
  recent_changes?: ResourceChange[];
  recent_remediations?: Array<Record<string, unknown>>;
  policy_posture?: IntelligencePolicyPostureSummary;
  learning: IntelligenceLearningStats;
  resources_at_risk?: Array<Record<string, unknown>>;
}

export interface ResourceIntelligence {
  resource_id: string;
  resource_name?: string;
  resource_type?: string;
  health: IntelligenceHealthScore;
  dependencies?: string[];
  dependents?: string[];
  correlations?: ResourceCorrelation[];
  recent_changes?: ResourceChange[];
  note_count: number;
}
