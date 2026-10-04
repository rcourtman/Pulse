import { describe, expect, it } from 'vitest';
import {
  sanitizeDiagnosticsData,
  stripInternalAnalyticsDiagnosticsFields,
  type DiagnosticsData,
} from '../diagnosticsModel';

type ExportData = Omit<DiagnosticsData, 'nodes' | 'pbs' | 'apiTokens' | 'dockerAgents'> &
  Record<string, any> & {
    nodes: Array<DiagnosticsData['nodes'][number] & Record<string, any>>;
    pbs: Array<DiagnosticsData['pbs'][number] & { lastError: { kind: string; message: string } }>;
    apiTokens: NonNullable<DiagnosticsData['apiTokens']> & {
      tokens: Array<{ id: string; name: string; hint: string }>;
      usage: Array<{ tokenId: string; agentCount: number; agents: string[] }>;
    };
    dockerAgents: NonNullable<DiagnosticsData['dockerAgents']> & {
      attention: Array<{
        agentId: string;
        name: string;
        tokenHint: string;
        status: string;
        issues: string[];
      }>;
    };
  };
// Synthetic fields use the CURRENT server JSON names, not a second exporter
// schema. No real report, credential, host or customer data enters this fixture.
const currentPayload = (): ExportData => ({
  version: '6.4.5',
  runtime: 'go',
  uptime: 300,
  nodes: [
    {
      id: 'private-connection-a',
      name: 'private-pve-a',
      host: '10.20.30.40',
      type: 'pve',
      authMethod: 'token',
      connected: false,
      error: 'dial tcp 10.20.30.40:8006 failed',
      vmDiskCheck: {
        vmsFound: 2,
        vmsWithAgent: 1,
        vmsWithDiskData: 0,
        testVMID: 9051,
        testVMName: 'private-guest-a',
        testResult: 'Guest agent request timed out',
        permissions: ['VM.GuestAgent.Audit'],
        recommendations: ['Read 10.20.30.40 locally'],
        problematicVMs: [
          {
            vmid: 9051,
            name: 'private-guest-a',
            status: 'running',
            issue: 'Guest agent unavailable',
          },
        ],
        filesystemsFound: [
          {
            mountpoint: '/private-team/data',
            type: 'ext4',
            total: 4096,
            used: 2048,
            filtered: false,
          },
        ],
      },
      physicalDisks: {
        nodesChecked: 2,
        totalDisks: 1,
        nodeResults: [
          {
            nodeName: 'private-host-a',
            diskCount: 1,
            diskDevices: ['/dev/disk/by-id/private-disk-a'],
            apiResponse: 'private upstream body',
          },
          {
            nodeName: 'private-host-b',
            diskCount: 0,
            diskDevices: [],
            apiResponse: 'Permission denied',
          },
        ],
      },
    },
  ],
  pbs: [
    {
      id: 'private-pbs-id',
      name: 'private-pbs-a',
      host: '10.20.30.41',
      connected: false,
      lastError: { kind: 'network', message: 'dial tcp 10.20.30.41:8007 failed' },
      probe: { connected: false, errorKind: 'network' },
    },
  ],
  system: {
    os: 'linux',
    arch: 'amd64',
    goVersion: 'go1.26',
    numCPU: 4,
    numGoroutine: 30,
    memoryMB: 128,
  },
  apiTokens: {
    enabled: true,
    tokenCount: 2,
    recommendTokenSetup: false,
    tokens: [
      { id: 'private-token-a', name: 'private-purpose-a', hint: 'private-hint-a' },
      { id: 'private-token-b', name: 'private-purpose-b', hint: 'private-hint-b' },
    ],
    usage: [
      { tokenId: 'private-token-b', agentCount: 2, agents: ['private-agent-b', 'private-agent-a'] },
      { tokenId: 'private-token-a', agentCount: 1, agents: ['private-agent-a'] },
    ],
  },
  dockerAgents: {
    agentsTotal: 2,
    agentsOnline: 1,
    agentsReportingVersion: 2,
    agentsWithTokenBinding: 2,
    agentsWithoutTokenBinding: 0,
    agentsNeedingAttention: 1,
    attention: [
      {
        agentId: 'private-agent-id-a',
        name: 'private-agent-a',
        tokenHint: 'private-hint-a',
        status: 'offline',
        issues: ['dial tcp 10.20.30.42 failed'],
      },
    ],
  },
  nodeSnapshots: [
    {
      instance: 'private-instance-b',
      node: 'private-host-b',
      memorySource: 'agent',
      memory: { used: 512 },
      raw: { total: 1024 },
    },
    {
      instance: 'private-instance-a',
      node: 'private-host-a',
      memorySource: 'proxmox',
      memory: { used: 2048 },
      raw: { total: 4096 },
    },
  ],
  guestSnapshots: [
    {
      instance: 'private-instance-a',
      node: 'private-host-a',
      guestType: 'vm',
      vmid: 9051,
      name: 'private-guest-a',
      memorySource: 'agent',
      memory: { used: 256 },
      raw: { hostAgentUsed: 256 },
      notes: [],
    },
  ],
  memorySources: [{ instance: 'private-instance-a', source: 'proxmox', nodeCount: 1 }],
  memorySourceBreakdown: [
    {
      instance: 'private-instance-b',
      scope: 'node',
      source: 'agent',
      trust: 'guest',
      count: 1,
      fallback: false,
      fallbackReasons: [],
    },
  ],
  metricsStore: {
    enabled: true,
    status: 'buffering',
    bufferSize: 17,
    notes: ['Read 10.20.30.43 locally'],
  },
  errors: [],
});

describe('current diagnostics export privacy', () => {
  it('anonymises actual tokenId/agents fields and preserves token and agent joins across reordered arrays', () => {
    const out = sanitizeDiagnosticsData(currentPayload()) as ExportData;
    expect(out.apiTokens.usage[0].tokenId).toBe(out.apiTokens.tokens[1].id);
    expect(out.apiTokens.usage[1].tokenId).toBe(out.apiTokens.tokens[0].id);
    expect(out.apiTokens.usage[0].agents[1]).toBe(out.apiTokens.usage[1].agents[0]);
    expect(out.apiTokens.usage[1].agents[0]).toBe(out.dockerAgents.attention[0].name);
    expect(out.apiTokens.usage[0].agentCount).toBe(2);
    expect(JSON.stringify(out.apiTokens)).not.toMatch(/private-/);
  });

  it('redacts node/guest identities and the actual memorySourceBreakdown without inventing position-based joins', () => {
    const out = sanitizeDiagnosticsData(currentPayload()) as ExportData;
    expect(out.guestSnapshots[0].instance).toBe(out.nodeSnapshots[1].instance);
    expect(out.memorySources[0].instance).toBe(out.nodeSnapshots[1].instance);
    expect(out.memorySourceBreakdown[0].instance).toBe(out.nodeSnapshots[0].instance);
    expect(out.guestSnapshots[0].node).toBe(out.nodeSnapshots[1].node);
    expect(out.guestSnapshots[0]).not.toHaveProperty('vmid');
    expect(out.guestSnapshots[0].memory).toEqual({ used: 256 });
    expect(out.guestSnapshots[0].raw).toEqual({ hostAgentUsed: 256 });
    expect(out.memorySourceBreakdown[0]).toMatchObject({
      count: 1,
      source: 'agent',
      fallback: false,
      trust: 'guest',
    });
    for (const key of ['nodeSnapshots', 'guestSnapshots', 'memorySources', 'memorySourceBreakdown'])
      expect(JSON.stringify(out[key])).not.toMatch(/private-/);
  });

  it('removes nested guest IDs/names and mount paths while retaining the observed disk failure and measurements', () => {
    const out = sanitizeDiagnosticsData(currentPayload()) as ExportData;
    const check = out.nodes[0].vmDiskCheck;
    expect(check).not.toHaveProperty('testVMID');
    expect(check.problematicVMs[0]).not.toHaveProperty('vmid');
    expect(JSON.stringify(check)).not.toMatch(/private-/);
    expect(check).toMatchObject({
      vmsFound: 2,
      vmsWithAgent: 1,
      vmsWithDiskData: 0,
      testResult: 'Guest agent request timed out',
    });
    expect(check.permissions).toEqual(['VM.GuestAgent.Audit']);
    expect(check.filesystemsFound[0]).toMatchObject({
      total: 4096,
      used: 2048,
      type: 'ext4',
      filtered: false,
    });
  });

  it('redacts private disk paths and unknown response bodies but keeps known failure categories', () => {
    const out = sanitizeDiagnosticsData(currentPayload()) as ExportData;
    const disks = out.nodes[0].physicalDisks;
    expect(JSON.stringify(disks)).not.toMatch(/private/);
    expect(disks).toMatchObject({ nodesChecked: 2, totalDisks: 1 });
    expect(disks.nodeResults[0]).toMatchObject({ diskCount: 1, apiResponse: '[REDACTED]' });
    expect(disks.nodeResults[1].apiResponse).toBe('Permission denied');
  });

  it('redacts IPs in nested errors/notes without erasing state or failure kinds', () => {
    const out = sanitizeDiagnosticsData(currentPayload()) as ExportData;
    expect(JSON.stringify(out)).not.toMatch(/10\.20\.30\./);
    expect(out.pbs[0].lastError).toEqual({
      kind: 'network',
      message: 'dial tcp [REDACTED_IP]:8007 failed',
    });
    expect(out.pbs[0].probe).toEqual({ connected: false, errorKind: 'network' });
    expect(out.metricsStore).toMatchObject({ status: 'buffering', bufferSize: 17 });
    expect(out.dockerAgents.attention[0].issues).toEqual(['dial tcp [REDACTED_IP] failed']);
  });

  it('keeps the full download and on-screen source unchanged and scopes aliases to each download', () => {
    const raw = currentPayload();
    const before = JSON.stringify(raw);
    const out = sanitizeDiagnosticsData(raw);
    expect(JSON.stringify(raw)).toBe(before);
    expect(stripInternalAnalyticsDiagnosticsFields(raw)).toEqual(raw);
    expect(sanitizeDiagnosticsData(raw)).toEqual(out);
    expect(raw.guestSnapshots[0].vmid).toBe(9051);
    expect(raw.apiTokens.usage[0].tokenId).toBe('private-token-b');
  });

  it('retains useful arbitrary prose rather than claiming complete automatic privacy', () => {
    const raw = currentPayload();
    raw.errors = ['DNS problem; review this free text before sharing'];
    expect(sanitizeDiagnosticsData(raw).errors).toEqual(raw.errors);
  });
});
