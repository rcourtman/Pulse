import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  EXTERNAL_PROBE_FEATURE,
  LOCAL_PROBE_AGENT_LABEL,
  PROBE_AGENT_STALE_ERROR,
  buildProbeAgentOptions,
  getExternalProbeGateBody,
  getExternalProbeGateTitle,
  getExternalProbeLockedHelpText,
  getProbeAgentLabel,
  getProbeSourceChipLabel,
  isExternalProbeLicenseError,
  isProbeAgentMissing,
  isProbeAgentStaleStatus,
} from '@/utils/availabilityProbeAgents';

const agentResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'agent:edge-01',
    name: 'edge-01',
    displayName: 'Edge 01',
    type: 'agent',
    platformId: 'edge-01',
    platformType: 'agent',
    sourceType: 'agent',
    sources: ['agent'],
    status: 'online',
    lastSeen: 1_700_000_000_000,
    agent: { agentId: 'host-edge-01' },
    ...overrides,
  }) as Resource;

describe('buildProbeAgentOptions', () => {
  it('lists connected Pulse Agent hosts by stable agent id and display name', () => {
    const options = buildProbeAgentOptions([
      agentResource(),
      agentResource({
        id: 'agent:branch-02',
        name: 'branch-02',
        displayName: 'Branch 02',
        platformId: 'branch-02',
        agent: { agentId: 'host-branch-02' },
      } as Partial<Resource>),
    ]);

    expect(options).toEqual([
      { id: 'host-branch-02', label: 'Branch 02' },
      { id: 'host-edge-01', label: 'Edge 01' },
    ]);
  });

  it('excludes offline hosts, provider-owned rows, and non-agent resources', () => {
    const options = buildProbeAgentOptions([
      agentResource({ status: 'offline' }),
      agentResource({
        id: 'agent:truenas',
        platformType: 'truenas',
        sources: ['truenas'],
        agent: { agentId: 'host-truenas' },
      } as Partial<Resource>),
      {
        id: 'availability:mqtt',
        type: 'network-endpoint',
        name: 'mqtt',
        displayName: 'mqtt',
        platformType: 'availability',
        status: 'online',
      } as Resource,
    ]);

    expect(options).toEqual([]);
  });

  it('deduplicates resource rows that resolve to the same agent id', () => {
    const options = buildProbeAgentOptions([
      agentResource(),
      agentResource({ id: 'agent:edge-01-duplicate' } as Partial<Resource>),
    ]);

    expect(options).toHaveLength(1);
    expect(options[0].id).toBe('host-edge-01');
  });
});

describe('probe agent labelling', () => {
  const options = [{ id: 'host-edge-01', label: 'Edge 01' }];

  it('names the local Pulse server for an empty assignment', () => {
    expect(getProbeAgentLabel(options, '')).toBe(LOCAL_PROBE_AGENT_LABEL);
    expect(getProbeSourceChipLabel(options, '')).toBeNull();
    expect(getProbeSourceChipLabel(options, undefined)).toBeNull();
  });

  it('attributes probe-reported results to the host display name', () => {
    expect(getProbeSourceChipLabel(options, 'host-edge-01')).toBe('via Edge 01');
  });

  it('falls back to the raw id when the host list does not contain it', () => {
    expect(getProbeAgentLabel(options, 'host-unknown')).toBe('host-unknown');
    expect(getProbeSourceChipLabel(options, 'host-unknown')).toBe('via host-unknown');
    expect(isProbeAgentMissing(options, 'host-unknown')).toBe(true);
    expect(isProbeAgentMissing(options, 'host-edge-01')).toBe(false);
    expect(isProbeAgentMissing(options, '')).toBe(false);
  });
});

describe('isProbeAgentStaleStatus', () => {
  it('detects a probe assignment that stopped reporting', () => {
    expect(
      isProbeAgentStaleStatus({ outcome: 'indeterminate', lastError: PROBE_AGENT_STALE_ERROR }),
    ).toBe(true);
  });

  it('leaves other indeterminate states alone', () => {
    expect(isProbeAgentStaleStatus({ outcome: 'indeterminate', lastError: '' })).toBe(false);
    expect(
      isProbeAgentStaleStatus({ outcome: 'unreachable', lastError: PROBE_AGENT_STALE_ERROR }),
    ).toBe(false);
    expect(isProbeAgentStaleStatus(null)).toBe(false);
    expect(isProbeAgentStaleStatus(undefined)).toBe(false);
  });
});

describe('isExternalProbeLicenseError', () => {
  it('recognizes the canonical 402 license_required body', () => {
    expect(
      isExternalProbeLicenseError(
        Object.assign(new Error('external probe requires a paid feature'), {
          status: 402,
          feature: EXTERNAL_PROBE_FEATURE,
        }),
      ),
    ).toBe(true);
  });

  it('recognizes a bare license_required message and the feature key alone', () => {
    expect(isExternalProbeLicenseError(new Error('license_required'))).toBe(true);
    expect(
      isExternalProbeLicenseError(Object.assign(new Error('nope'), { feature: 'external_probe' })),
    ).toBe(true);
  });

  it('does not swallow unrelated failures', () => {
    expect(isExternalProbeLicenseError(new Error('unknown_probe_agent'))).toBe(false);
    expect(isExternalProbeLicenseError(undefined)).toBe(false);
    expect(isExternalProbeLicenseError('license_required')).toBe(false);
  });
});

describe('external probe gate copy', () => {
  it('uses the generated catalog name and the canonical minimum tier label', () => {
    expect(getExternalProbeGateTitle()).toBe('External Probes');
    expect(getExternalProbeGateBody()).toContain('Pro');
    expect(getExternalProbeGateBody()).toContain('this Pulse server');
    // Same shape as getTabLockReason: name the minimum tier, not the feature.
    expect(getExternalProbeLockedHelpText()).toBe(
      'Remote probe hosts require Pro. This check runs from the Pulse server.',
    );
  });
});
