import { describe, expect, it } from 'vitest';

import {
  formatAgentProfileSuggestionValue,
  getAgentProfileSuggestionRiskHints,
} from '@/utils/agentProfileSuggestionPresentation';

describe('formatAgentProfileSuggestionValue (branch coverage)', () => {
  it('returns "unset" for the null arm of the nullish guard', () => {
    expect(formatAgentProfileSuggestionValue(null)).toBe('unset');
  });

  it('returns the string verbatim for a non-empty string arm', () => {
    expect(formatAgentProfileSuggestionValue('debug')).toBe('debug');
    expect(formatAgentProfileSuggestionValue('k3s-agent')).toBe('k3s-agent');
  });

  it('serializes numbers through the typeof number arm', () => {
    expect(formatAgentProfileSuggestionValue(0)).toBe('0');
    expect(formatAgentProfileSuggestionValue(42)).toBe('42');
    expect(formatAgentProfileSuggestionValue(-7)).toBe('-7');
    expect(formatAgentProfileSuggestionValue(3.5)).toBe('3.5');
  });

  it('JSON-stringifies objects through the default arm', () => {
    expect(formatAgentProfileSuggestionValue({ tag: 'prod', region: 'us-east' })).toBe(
      '{"tag":"prod","region":"us-east"}',
    );
  });

  it('JSON-stringifies arrays through the default arm', () => {
    expect(formatAgentProfileSuggestionValue(['a', 'b'])).toBe('["a","b"]');
    expect(formatAgentProfileSuggestionValue([])).toBe('[]');
  });
});

describe('getAgentProfileSuggestionRiskHints (branch coverage)', () => {
  it('pushes the docker-update-checks hint for the disable_docker_update_checks===true arm', () => {
    expect(
      getAgentProfileSuggestionRiskHints({ disable_docker_update_checks: true }),
    ).toStrictEqual([
      'Docker update checks are disabled. Update visibility will be limited.',
    ]);
  });

  it('pushes the host-monitoring hint for the enable_host===false arm', () => {
    expect(getAgentProfileSuggestionRiskHints({ enable_host: false })).toStrictEqual([
      'Agent monitoring is disabled. Agent metrics and command execution will stop.',
    ]);
  });

  it('pushes all five hints in source order when every risk flag is set', () => {
    expect(
      getAgentProfileSuggestionRiskHints({
        disable_auto_update: true,
        disable_docker_update_checks: true,
        enable_host: false,
        enable_docker: false,
        disable_ceph: true,
      }),
    ).toStrictEqual([
      'Auto updates are disabled. Plan manual patching for agents.',
      'Docker update checks are disabled. Update visibility will be limited.',
      'Agent monitoring is disabled. Agent metrics and command execution will stop.',
      'Docker monitoring is disabled. Container metrics and update tracking will stop.',
      'Ceph monitoring is disabled. Cluster health checks will be skipped.',
    ]);
  });

  it('returns an empty array for an empty config (all skip arms)', () => {
    expect(getAgentProfileSuggestionRiskHints({})).toStrictEqual([]);
  });

  it('skips every flag when the config is empty of any matching keys', () => {
    expect(
      getAgentProfileSuggestionRiskHints({ unrelated_key: true, enable_docker: true }),
    ).toStrictEqual([]);
  });

  it('does not treat a truthy-but-non-boolean disable_auto_update as === true', () => {
    expect(
      getAgentProfileSuggestionRiskHints({
        disable_auto_update: 1,
        disable_docker_update_checks: 'yes',
        disable_ceph: 'true',
      }),
    ).toStrictEqual([]);
  });

  it('does not treat a falsy-but-non-boolean enable_host/enable_docker as === false', () => {
    expect(
      getAgentProfileSuggestionRiskHints({
        enable_host: 0,
        enable_docker: '',
      }),
    ).toStrictEqual([]);
  });

  it('only pushes hints for the flags that strictly match while skipping the rest', () => {
    expect(
      getAgentProfileSuggestionRiskHints({
        disable_auto_update: 'true',
        disable_ceph: true,
        enable_docker: false,
        enable_host: 'false',
      }),
    ).toStrictEqual([
      'Docker monitoring is disabled. Container metrics and update tracking will stop.',
      'Ceph monitoring is disabled. Cluster health checks will be skipped.',
    ]);
  });
});
