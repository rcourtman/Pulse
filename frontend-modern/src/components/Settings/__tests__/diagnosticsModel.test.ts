import { describe, expect, it } from 'vitest';
import {
  buildDiagnosticsExportFilename,
  sanitizeDiagnosticsData,
  stripInternalAnalyticsDiagnosticsFields,
  type DiagnosticsData,
} from '@/components/Settings/diagnosticsModel';

const createDiagnosticsData = (): DiagnosticsData =>
  ({
    version: '6.0.0',
    runtime: 'go',
    uptime: 3600,
    nodes: [
      {
        id: 'node-raw-id',
        name: 'pve-01',
        host: '10.0.0.5',
        type: 'pve',
        authMethod: 'token',
        connected: false,
        error: 'dial tcp 10.0.0.5:8006: connect: connection refused',
      },
    ],
    pbs: [
      {
        id: 'pbs-raw-id',
        name: 'pbs-01',
        host: '10.0.0.15',
        connected: false,
        error: 'Get https://10.0.0.15:8007: EOF',
      },
    ],
    system: {
      os: 'linux',
      arch: 'amd64',
      goVersion: 'go1.25',
      numCPU: 8,
      numGoroutine: 32,
      memoryMB: 128,
    },
    discovery: {
      enabled: true,
      configuredSubnet: '10.0.0.0/24',
      activeSubnet: '10.0.1.0/24',
      environmentOverride: 'PULSE_DISCOVERY_SUBNET=10.0.2.0/24',
      subnetAllowlist: ['10.0.0.0/24'],
      subnetBlocklist: ['10.0.3.0/24'],
      history: [
        {
          startedAt: '2026-04-20T10:00:00Z',
          completedAt: '2026-04-20T10:00:10Z',
          duration: '10s',
          durationMs: 10000,
          subnet: '10.0.0.0/24',
          serverCount: 4,
          errorCount: 1,
          blocklistLength: 1,
          status: 'completed',
        },
      ],
    },
    commercialFunnel: {
      enabled: true,
      summary: { pricing_viewed: 2, checkout_clicked: 1 },
    },
    infrastructureOnboarding: {
      enabled: true,
      status: 'warning',
      windowDays: 30,
      summary: {
        opened: 4,
        api_path_selected: 2,
        agent_path_selected: 1,
        probe_detected: 1,
        probe_no_match: 2,
        probe_error: 0,
        catalog_selected: 2,
        credentials_opened: 1,
        period: {
          from: '2026-03-19T00:00:00Z',
          to: '2026-04-18T00:00:00Z',
        },
      },
      daily: [
        {
          day: '2026-04-18',
          opened: 2,
          api_path_selected: 1,
          agent_path_selected: 0,
          probe_detected: 1,
          probe_no_match: 1,
          probe_error: 0,
          catalog_selected: 1,
          credentials_opened: 1,
        },
      ],
      paths: [{ key: 'api', count: 2 }],
      platforms: [{ key: 'truenas', catalog_selected: 2, credentials_opened: 1 }],
      notes: ['Some probed addresses did not match a supported API-backed platform.'],
    },
    errors: ['probe failed for 10.0.0.10 after timeout'],
  }) as DiagnosticsData;

describe('diagnosticsModel', () => {
  it('sanitizes infrastructure diagnostics while stripping internal analytics fields', () => {
    const raw = createDiagnosticsData();

    const sanitized = sanitizeDiagnosticsData(raw);

    expect(raw.nodes[0].host).toBe('10.0.0.5');
    expect(sanitized.nodes).toEqual([
      expect.objectContaining({
        id: 'node-1',
        name: 'node-1',
        host: 'node-1',
        error: 'dial tcp [REDACTED_IP]:8006: connect: connection refused',
      }),
    ]);
    expect(sanitized.pbs).toEqual([
      expect.objectContaining({
        id: 'pbs-1',
        name: 'pbs-1',
        host: 'pbs-1',
        error: 'Get https://[REDACTED_IP]:8007: EOF',
      }),
    ]);
    expect(sanitized.discovery).toEqual(
      expect.objectContaining({
        configuredSubnet: '[REDACTED_SUBNET]',
        activeSubnet: '[REDACTED_SUBNET]',
        environmentOverride: '[REDACTED]',
        subnetAllowlist: ['[REDACTED_SUBNET]'],
        subnetBlocklist: ['[REDACTED_SUBNET]'],
        history: [
          expect.objectContaining({
            subnet: '[REDACTED_SUBNET]',
          }),
        ],
      }),
    );
    expect(sanitized.errors).toEqual(['probe failed for [REDACTED_IP] after timeout']);
    expect(sanitized).not.toHaveProperty('commercialFunnel');
    expect(sanitized).not.toHaveProperty('infrastructureOnboarding');
  });

  it('strips internal analytics fields before diagnostics state or full export use', () => {
    const stripped = stripInternalAnalyticsDiagnosticsFields(createDiagnosticsData());

    expect(stripped).not.toHaveProperty('commercialFunnel');
    expect(stripped).not.toHaveProperty('infrastructureOnboarding');
    expect(stripped.nodes[0].host).toBe('10.0.0.5');
  });

  it('builds stable diagnostics export filenames', () => {
    const now = new Date('2026-04-22T12:34:56Z');

    expect(buildDiagnosticsExportFilename(false, now)).toBe(
      'pulse-diagnostics-full-2026-04-22.json',
    );
    expect(buildDiagnosticsExportFilename(true, now)).toBe(
      'pulse-diagnostics-sanitized-2026-04-22.json',
    );
  });
});
