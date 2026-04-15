import { describe, expect, it } from 'vitest';

import {
  DISCOVERY_ANALYSIS_EXPLANATION,
  DISCOVERY_ANALYSIS_REASONING_LABEL,
  RESOURCE_ANALYSIS_LABEL,
  RESOURCE_SAFE_SUMMARY_LABEL,
  formatResourceAnalysisSummary,
} from '@/utils/resourceAnalysisPresentation';

describe('resourceAnalysisPresentation', () => {
  it('keeps resource investigation labels product-neutral', () => {
    expect(RESOURCE_ANALYSIS_LABEL).toBe('Analysis');
    expect(RESOURCE_SAFE_SUMMARY_LABEL).toBe('Safe Summary');
    expect(DISCOVERY_ANALYSIS_REASONING_LABEL).toBe('Analysis Reasoning');
  });

  it('formats resource analysis summaries with rounded health scores', () => {
    expect(formatResourceAnalysisSummary('A', 91.6)).toBe('Analysis A · 92/100');
  });

  it('keeps discovery explanation focused on the configured analysis provider', () => {
    expect(DISCOVERY_ANALYSIS_EXPLANATION).toContain('configured analysis provider');
    expect(DISCOVERY_ANALYSIS_EXPLANATION).not.toContain('uses AI');
  });
});
