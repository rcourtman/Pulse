/**
 * AI Intelligence Types
 *
 * Shared type definitions for AI intelligence features:
 * - Anomaly detection (baseline deviation)
 * - Learning status (baseline progress)
 *
 * Store logic lives in @/stores/aiIntelligence.ts
 */

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
