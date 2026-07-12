/**
 * Branch-coverage tests for the still-uncovered named helpers in
 * diagnosticsModel:
 *   sanitizeDiagnosticsData, formatUptime.
 *
 * Every if/else, ternary, optional-chain (`?.`), `Array.isArray` guard and
 * regex alternation arm in each function is driven with a concrete input and
 * asserted against the exact emitted shape (no truthiness-only checks).
 *
 * Import path mirrors the sibling `diagnosticsModel.test.ts` (alias `@/...`).
 */
import { describe, expect, it } from 'vitest';
import {
  formatUptime,
  sanitizeDiagnosticsData,
  type DiagnosticsData,
} from '@/components/Settings/diagnosticsModel';

// The helper also sanitizes a few arrays that are intentionally NOT part of the
// typed DiagnosticsData surface (see diagnosticsModel.ts:261-286) and a couple
// of untyped sub-fields (apiTokens.tokens/usage, dockerAgents.attention).
// These aliases surface them so the test can build inputs and assert outputs.
type ApiTokensDiagnosticWithExtras = NonNullable<DiagnosticsData['apiTokens']> & {
  tokens?: Array<Record<string, unknown>>;
  usage?: Array<Record<string, unknown>>;
};

type DockerAgentsDiagnosticWithExtras = NonNullable<DiagnosticsData['dockerAgents']> & {
  attention?: Array<Record<string, unknown>>;
};

type DiagnosticsDataWithSnapshots = DiagnosticsData & {
  nodeSnapshots?: Array<Record<string, unknown>>;
  guestSnapshots?: Array<Record<string, unknown>>;
  memorySources?: Array<Record<string, unknown>>;
};

// Minimal valid base — every test spreads this then overrides only the field(s)
// needed to drive the branch under test.
const baseData = (): DiagnosticsDataWithSnapshots => ({
  version: '6.0.0',
  runtime: 'go',
  uptime: 0,
  nodes: [],
  pbs: [],
  system: {
    os: 'linux',
    arch: 'amd64',
    goVersion: 'go1.25',
    numCPU: 8,
    numGoroutine: 32,
    memoryMB: 128,
  },
  errors: [],
});

// ---- formatUptime -----------------------------------------------------------

describe('formatUptime', () => {
  it('formats the sub-minute arm as "<n>s" (seconds < 60)', () => {
    expect(formatUptime(0)).toBe('0s');
    expect(formatUptime(45)).toBe('45s');
    expect(formatUptime(59)).toBe('59s');
  });

  it('formats the minute+second arm for 60 <= seconds < 3600', () => {
    expect(formatUptime(60)).toBe('1m 0s');
    expect(formatUptime(125)).toBe('2m 5s');
    expect(formatUptime(3599)).toBe('59m 59s');
  });

  it('formats the hours+minutes arm for 3600 <= seconds < 86400 (hours < 24)', () => {
    expect(formatUptime(3600)).toBe('1h 0m');
    expect(formatUptime(3660)).toBe('1h 1m');
    expect(formatUptime(86399)).toBe('23h 59m');
  });

  it('formats the days+hours arm for seconds >= 86400 (hours >= 24)', () => {
    expect(formatUptime(86400)).toBe('1d 0h');
    expect(formatUptime(90000)).toBe('1d 1h');
    expect(formatUptime(172800)).toBe('2d 0h');
  });
});

// ---- sanitizeDiagnosticsData / nodes ---------------------------------------

describe('sanitizeDiagnosticsData / nodes', () => {
  it('overwrites host/name/id with positional placeholders and redacts IPs in node.error', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      nodes: [
        {
          id: 'raw-id',
          name: 'pve-01',
          host: '10.0.0.5',
          type: 'pve',
          authMethod: 'token',
          connected: true,
          error: 'dial tcp 10.0.0.5:8006 failed',
        },
      ],
    });

    expect(sanitized.nodes).toStrictEqual([
      {
        id: 'node-1',
        name: 'node-1',
        host: 'node-1',
        type: 'pve',
        authMethod: 'token',
        connected: true,
        error: 'dial tcp [REDACTED_IP]:8006 failed',
      },
    ]);
  });

  it('sets error to undefined when a node has no error (else arm of the error ternary)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      nodes: [
        {
          id: 'raw',
          name: 'pve-02',
          host: '10.0.0.6',
          type: 'pve',
          authMethod: 'token',
          connected: true,
        },
      ],
    });

    expect(sanitized.nodes[0]).toStrictEqual({
      id: 'node-1',
      name: 'node-1',
      host: 'node-1',
      type: 'pve',
      authMethod: 'token',
      connected: true,
      error: undefined,
    });
  });

  it('handles an empty nodes array (Array.isArray true, map yields [])', () => {
    const sanitized = sanitizeDiagnosticsData({ ...baseData(), nodes: [] });
    expect(sanitized.nodes).toStrictEqual([]);
  });

  it('redacts multiple IPs and the CIDR-suffix arm of the regex in a single error', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      nodes: [
        {
          id: 'x',
          name: 'x',
          host: '1.2.3.4',
          type: 'pve',
          authMethod: 'token',
          connected: false,
          error: 'scan 10.0.0.0/24 then 10.0.0.5 and 192.168.1.1/32',
        },
      ],
    });

    expect(sanitized.nodes[0].error).toBe(
      'scan [REDACTED_IP] then [REDACTED_IP] and [REDACTED_IP]',
    );
  });
});

// ---- sanitizeDiagnosticsData / pbs -----------------------------------------

describe('sanitizeDiagnosticsData / pbs', () => {
  it('overwrites host/name/id with pbs-N placeholders and redacts IPs in pbs.error', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      pbs: [
        {
          id: 'pbs-raw',
          name: 'pbs-01',
          host: '10.0.0.15',
          connected: false,
          error: 'Get https://10.0.0.15:8007: EOF',
        },
      ],
    });

    expect(sanitized.pbs).toStrictEqual([
      {
        id: 'pbs-1',
        name: 'pbs-1',
        host: 'pbs-1',
        connected: false,
        error: 'Get https://[REDACTED_IP]:8007: EOF',
      },
    ]);
  });

  it('sets error to undefined when a pbs has no error (else arm of the error ternary)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      pbs: [{ id: 'r', name: 'p', host: '10.0.0.16', connected: true }],
    });

    expect(sanitized.pbs[0]).toStrictEqual({
      id: 'pbs-1',
      name: 'pbs-1',
      host: 'pbs-1',
      connected: true,
      error: undefined,
    });
  });
});

// ---- sanitizeDiagnosticsData / discovery -----------------------------------

describe('sanitizeDiagnosticsData / discovery', () => {
  it('leaves discovery undefined when absent (top-level guard false)', () => {
    const sanitized = sanitizeDiagnosticsData({ ...baseData() });
    expect(sanitized.discovery).toBeUndefined();
  });

  it('collapses every falsy subnet field to undefined and skips the optional-chain maps when the arrays are absent', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      discovery: { enabled: true },
    });

    expect(sanitized.discovery).toStrictEqual({
      enabled: true,
      configuredSubnet: undefined,
      activeSubnet: undefined,
      environmentOverride: undefined,
      subnetAllowlist: undefined,
      subnetBlocklist: undefined,
    });
  });

  it('preserves empty arrays through the optional-chain map (?. on [] returns [])', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      discovery: { enabled: true, subnetAllowlist: [], subnetBlocklist: [] },
    });

    expect(sanitized.discovery?.subnetAllowlist).toStrictEqual([]);
    expect(sanitized.discovery?.subnetBlocklist).toStrictEqual([]);
  });

  it('redacts every populated subnet/environment field (truthy ternary arms)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      discovery: {
        enabled: true,
        configuredSubnet: '10.0.0.0/24',
        activeSubnet: '10.0.1.0/24',
        environmentOverride: 'PULSE_DISCOVERY_SUBNET=10.0.2.0/24',
        subnetAllowlist: ['10.0.0.0/24', '10.0.4.0/24'],
        subnetBlocklist: ['10.0.3.0/24'],
      },
    });

    expect(sanitized.discovery).toStrictEqual({
      enabled: true,
      configuredSubnet: '[REDACTED_SUBNET]',
      activeSubnet: '[REDACTED_SUBNET]',
      environmentOverride: '[REDACTED]',
      subnetAllowlist: ['[REDACTED_SUBNET]', '[REDACTED_SUBNET]'],
      subnetBlocklist: ['[REDACTED_SUBNET]'],
    });
  });

  it('leaves history untouched when it is not an array (history guard false)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      discovery: {
        enabled: true,
        history: 'not-an-array',
      } as unknown as DiagnosticsData['discovery'],
    });

    expect((sanitized.discovery as { history?: unknown }).history).toBe('not-an-array');
  });

  it('overwrites the subnet of every history entry (history guard true)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      discovery: {
        enabled: true,
        history: [
          { subnet: '10.0.0.0/24', status: 'completed', serverCount: 3 },
          { subnet: '10.0.1.0/24', status: 'failed', errorCount: 2 },
        ],
      },
    });

    expect(sanitized.discovery?.history).toStrictEqual([
      { subnet: '[REDACTED_SUBNET]', status: 'completed', serverCount: 3 },
      { subnet: '[REDACTED_SUBNET]', status: 'failed', errorCount: 2 },
    ]);
  });
});

// ---- sanitizeDiagnosticsData / apiTokens -----------------------------------

describe('sanitizeDiagnosticsData / apiTokens', () => {
  it('skips the apiTokens block entirely when apiTokens is absent (top guard false)', () => {
    const sanitized = sanitizeDiagnosticsData({ ...baseData() });
    expect(sanitized.apiTokens).toBeUndefined();
  });

  it('leaves apiTokens unchanged when neither tokens nor usage is an array', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      apiTokens: {
        enabled: true,
        tokenCount: 2,
        recommendTokenSetup: false,
        unusedTokenCount: 0,
      },
    });

    expect(sanitized.apiTokens).toStrictEqual({
      enabled: true,
      tokenCount: 2,
      recommendTokenSetup: false,
      unusedTokenCount: 0,
    });
  });

  it('redacts token hint/name/id by position and nulls usage hosts (both Array guards true)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      apiTokens: {
        enabled: true,
        tokenCount: 2,
        recommendTokenSetup: false,
        tokens: [
          { id: 'tok-aaa', name: 'Prod Token', hint: 'aaaa', extra: 'keep' },
          { id: 'tok-bbb', name: 'Dev Token', hint: 'bbbb' },
        ],
        usage: [
          { tokenId: 'tok-aaa', hosts: ['10.0.0.5', '10.0.0.6'], count: 3 },
          { tokenId: 'tok-bbb', hosts: [], count: 0 },
        ],
      } as ApiTokensDiagnosticWithExtras,
    });

    const apiTokens = sanitized.apiTokens as unknown as ApiTokensDiagnosticWithExtras;
    expect(apiTokens.tokens).toStrictEqual([
      { id: 'token-1', name: 'token-1', hint: '[REDACTED]', extra: 'keep' },
      { id: 'token-2', name: 'token-2', hint: '[REDACTED]' },
    ]);
    expect(apiTokens.usage).toStrictEqual([
      { tokenId: 'tok-aaa', hosts: undefined, count: 3 },
      { tokenId: 'tok-bbb', hosts: undefined, count: 0 },
    ]);
  });
});

// ---- sanitizeDiagnosticsData / dockerAgents --------------------------------

describe('sanitizeDiagnosticsData / dockerAgents', () => {
  it('skips the dockerAgents block entirely when dockerAgents is absent', () => {
    const sanitized = sanitizeDiagnosticsData({ ...baseData() });
    expect(sanitized.dockerAgents).toBeUndefined();
  });

  it('leaves dockerAgents unchanged when attention is not an array', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      dockerAgents: {
        agentsTotal: 3,
        agentsOnline: 2,
        agentsReportingVersion: 2,
        agentsWithTokenBinding: 2,
        agentsWithoutTokenBinding: 1,
        agentsNeedingAttention: 1,
      },
    });

    expect(sanitized.dockerAgents).toStrictEqual({
      agentsTotal: 3,
      agentsOnline: 2,
      agentsReportingVersion: 2,
      agentsWithTokenBinding: 2,
      agentsWithoutTokenBinding: 1,
      agentsNeedingAttention: 1,
    });
  });

  it('rekeys attention entries by position and exercises both tokenHint ternary arms', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      dockerAgents: {
        agentsTotal: 3,
        agentsOnline: 2,
        agentsReportingVersion: 2,
        agentsWithTokenBinding: 2,
        agentsWithoutTokenBinding: 1,
        agentsNeedingAttention: 2,
        attention: [
          {
            agentId: 'agent-7',
            name: 'docker-prod-01',
            tokenHint: 'deadbeef',
            reason: 'stale',
          },
          { agentId: 'agent-9', name: 'docker-dev-02', reason: 'offline' },
        ],
      } as DockerAgentsDiagnosticWithExtras,
    });

    const attention = (
      sanitized.dockerAgents as unknown as DockerAgentsDiagnosticWithExtras
    ).attention;
    expect(attention).toStrictEqual([
      {
        agentId: 'docker-host-1',
        name: 'docker-host-1',
        tokenHint: '[REDACTED]',
        reason: 'stale',
      },
      {
        agentId: 'docker-host-2',
        name: 'docker-host-2',
        reason: 'offline',
        tokenHint: undefined,
      },
    ]);
  });
});

// ---- sanitizeDiagnosticsData / aiChat --------------------------------------

describe('sanitizeDiagnosticsData / aiChat', () => {
  it('redacts aiChat.url when present (truthy optional-chain guard)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      aiChat: {
        enabled: true,
        running: true,
        healthy: true,
        assistantRuntimeConnected: true,
        url: 'http://10.0.0.5:11434',
        model: 'gpt',
      },
    });

    expect(sanitized.aiChat?.url).toBe('[REDACTED]');
    expect(sanitized.aiChat?.model).toBe('gpt');
  });

  it('leaves aiChat untouched when url is absent (optional-chain guard false)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      aiChat: {
        enabled: true,
        running: false,
        healthy: false,
        assistantRuntimeConnected: false,
      },
    });

    expect(sanitized.aiChat?.url).toBeUndefined();
    expect(sanitized.aiChat).toStrictEqual({
      enabled: true,
      running: false,
      healthy: false,
      assistantRuntimeConnected: false,
    });
  });
});

// ---- sanitizeDiagnosticsData / alerts --------------------------------------

describe('sanitizeDiagnosticsData / alerts', () => {
  it('skips the alerts block entirely when alerts is absent (guard false)', () => {
    const sanitized = sanitizeDiagnosticsData({ ...baseData() });
    expect(sanitized.alerts).toBeUndefined();
  });

  it('keeps alerts but skips the overrides map when overrides is not an array', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      alerts: { missingCooldown: true, missingGroupingWindow: false },
    });

    expect(sanitized.alerts).toStrictEqual({
      missingCooldown: true,
      missingGroupingWindow: false,
    });
  });

  it('rekeys every override by position (overrides guard true)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      alerts: {
        missingCooldown: false,
        missingGroupingWindow: false,
        overrides: [
          { key: 'pve5-ceph', thresholds: { usage: 50 } },
          { key: 'pve5-101', disabled: true },
        ],
      },
    });

    expect(sanitized.alerts?.overrides).toStrictEqual([
      { key: 'override-1', thresholds: { usage: 50 } },
      { key: 'override-2', disabled: true },
    ]);
  });
});

// ---- sanitizeDiagnosticsData / errors --------------------------------------

describe('sanitizeDiagnosticsData / errors', () => {
  it('leaves errors untouched when errors is not an array (Array.isArray false)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      errors: 'oops' as unknown as DiagnosticsData['errors'],
    });

    expect(sanitized.errors as unknown as string).toBe('oops');
  });

  it('redacts bare IPs and the CIDR-suffix arm across every error string', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      errors: [
        'probe failed for 10.0.0.10 after timeout',
        'cidr scan 10.0.0.0/24 rejected',
        'no ip here',
      ],
    });

    expect(sanitized.errors).toStrictEqual([
      'probe failed for [REDACTED_IP] after timeout',
      'cidr scan [REDACTED_IP] rejected',
      'no ip here',
    ]);
  });
});

// ---- sanitizeDiagnosticsData / snapshot arrays -----------------------------

describe('sanitizeDiagnosticsData / snapshot arrays', () => {
  it('stamps an instance placeholder on every nodeSnapshots entry', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      nodeSnapshots: [
        { instance: '10.0.0.5', cpu: 0.4 },
        { instance: '10.0.0.6', mem: 8192 },
      ],
    } as DiagnosticsDataWithSnapshots);

    const out = sanitized as unknown as DiagnosticsDataWithSnapshots;
    expect(out.nodeSnapshots).toStrictEqual([
      { instance: 'node-1', cpu: 0.4 },
      { instance: 'node-2', mem: 8192 },
    ]);
  });

  it('stamps an instance placeholder on every guestSnapshots entry', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      guestSnapshots: [{ instance: 'vm-100', status: 'running' }],
    } as DiagnosticsDataWithSnapshots);

    const out = sanitized as unknown as DiagnosticsDataWithSnapshots;
    expect(out.guestSnapshots).toStrictEqual([
      { instance: 'node-1', status: 'running' },
    ]);
  });

  it('stamps an instance placeholder on every memorySources entry', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      memorySources: [{ instance: '10.0.0.7', source: 'balloon' }],
    } as DiagnosticsDataWithSnapshots);

    const out = sanitized as unknown as DiagnosticsDataWithSnapshots;
    expect(out.memorySources).toStrictEqual([
      { instance: 'node-1', source: 'balloon' },
    ]);
  });
});

// ---- sanitizeDiagnosticsData / defensive Array.isArray guards --------------

describe('sanitizeDiagnosticsData / defensive guards', () => {
  it('leaves nodes/pbs in place when they are not arrays (Array.isArray false arms)', () => {
    const sanitized = sanitizeDiagnosticsData({
      ...baseData(),
      nodes: 'not-array' as unknown as DiagnosticsData['nodes'],
      pbs: null as unknown as DiagnosticsData['pbs'],
      errors: 42 as unknown as DiagnosticsData['errors'],
    });

    expect(sanitized.nodes as unknown as string).toBe('not-array');
    expect(sanitized.pbs).toBeNull();
    expect(sanitized.errors as unknown as number).toBe(42);
  });
});
