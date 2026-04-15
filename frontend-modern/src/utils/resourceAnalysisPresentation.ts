export const RESOURCE_ANALYSIS_LABEL = 'Analysis';
export const RESOURCE_SAFE_SUMMARY_LABEL = 'Safe Summary';
export const DISCOVERY_ANALYSIS_REASONING_LABEL = 'Analysis Reasoning';
export const DISCOVERY_ANALYSIS_EXPLANATION =
  'Discovery runs read-only commands to gather system information (processes, ports, services), then uses the configured analysis provider to identify what is running. No data is stored externally; only the analysis result is saved locally.';

export const formatResourceAnalysisSummary = (grade: string, score: number): string =>
  `${RESOURCE_ANALYSIS_LABEL} ${grade} · ${Math.round(score)}/100`;
