import { describe, expect, it } from 'vitest';
import {
  AGENT_PROFILE_SUGGESTION_EXAMPLE_PROMPTS,
  formatAgentProfileSuggestionValue,
  getAgentProfileSuggestionLoadingState,
  getAgentProfileSuggestionKeyLabel,
  getAgentProfileSuggestionRiskHints,
  getAgentProfileSuggestionValueBadgeClass,
  hasAgentProfileSuggestionValue,
} from '@/utils/agentProfileSuggestionPresentation';

describe('agentProfileSuggestionPresentation', () => {
  it('formats canonical labels and values for profile suggestions', () => {
    expect(getAgentProfileSuggestionKeyLabel('enable_docker')).toBe('Enable Docker');
    expect(formatAgentProfileSuggestionValue(true)).toBe('Enabled');
    expect(formatAgentProfileSuggestionValue(false)).toBe('Disabled');
    expect(formatAgentProfileSuggestionValue('')).toBe('(empty)');
    expect(formatAgentProfileSuggestionValue(undefined)).toBe('unset');
    expect(hasAgentProfileSuggestionValue(undefined)).toBe(false);
    expect(hasAgentProfileSuggestionValue(0)).toBe(true);
  });

  it('returns canonical suggestion example prompts and value badge classes', () => {
    expect(AGENT_PROFILE_SUGGESTION_EXAMPLE_PROMPTS).toContain(
      'Kubernetes monitoring profile with all pods visible',
    );
    expect(getAgentProfileSuggestionValueBadgeClass(true)).toContain('bg-green-100');
    expect(getAgentProfileSuggestionValueBadgeClass('debug')).toContain('bg-blue-100');
  });

  it('derives canonical risk hints from suggested profile config', () => {
    expect(
      getAgentProfileSuggestionRiskHints({
        enable_docker: false,
        disable_auto_update: true,
        disable_ceph: true,
      }),
    ).toEqual([
      'Auto updates are disabled. Plan manual patching for agents.',
      'Docker monitoring is disabled. Container metrics and update tracking will stop.',
      'Ceph monitoring is disabled. Cluster health checks will be skipped.',
    ]);
  });

  it('returns canonical loading copy', () => {
    expect(getAgentProfileSuggestionLoadingState()).toEqual({
      text: 'Generating suggestion...',
    });
  });
});
