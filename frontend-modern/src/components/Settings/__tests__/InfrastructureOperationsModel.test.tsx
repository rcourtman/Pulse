import { describe, expect, it } from 'vitest';
import type { ConnectedInfrastructureItem } from '@/types/api';
import {
  getPowerShellInstallProfileEnvFromFlags,
  getStopMonitoringScopeLabel,
  rowFromConnectedInfrastructureItem,
} from '../infrastructureOperationsModel';

describe('infrastructure operations model', () => {
  it('builds unified rows from connected infrastructure surfaces', () => {
    const item: ConnectedInfrastructureItem = {
      id: 'agent-1',
      name: 'node-a',
      hostname: 'node-a.internal',
      status: 'active',
      scopeAgentId: 'agent-1',
      surfaces: [
        {
          id: 'surface-agent',
          kind: 'agent',
          label: 'Host telemetry',
          detail: 'Pulse is receiving host telemetry.',
          controlId: 'agent-1',
          action: 'stop-monitoring',
          idLabel: 'Agent ID',
          idValue: 'agent-1',
        },
        {
          id: 'surface-pbs',
          kind: 'pbs',
          label: 'PBS data',
          detail: 'Pulse is receiving PBS telemetry.',
          controlId: 'pbs-1',
          action: 'stop-monitoring',
          idLabel: 'PBS node ID',
          idValue: 'pbs-1',
        },
      ],
    };

    const row = rowFromConnectedInfrastructureItem(item, {
      label: 'Default',
      detail: 'Auto-detect',
      category: 'default',
    });

    expect(row.rowKey).toBe('agent-agent-1');
    expect(row.capabilities).toEqual(['agent', 'pbs']);
    expect(row.installFlags).toEqual(['--enable-proxmox', '--proxmox-type pbs']);
    expect(row.searchText).toContain('node-a.internal');
  });

  it('keeps host-managed stop monitoring scoped to the full host surface set', () => {
    const row = {
      rowKey: 'agent-agent-1',
      id: 'agent-1',
      name: 'node-a',
      hostname: 'node-a.internal',
      capabilities: ['agent', 'docker', 'pbs'],
      status: 'active' as const,
      upgradePlatform: 'linux' as const,
      scope: {
        label: 'Default',
        detail: 'Auto-detect',
        category: 'default' as const,
      },
      installFlags: ['--enable-docker', '--disable-host', '--enable-proxmox', '--proxmox-type pbs'],
      searchText: 'node-a node-a.internal agent-1',
      surfaces: [
        {
          key: 'agent',
          kind: 'agent' as const,
          label: 'Host telemetry',
          detail: 'Pulse is receiving host telemetry.',
          action: 'stop-monitoring' as const,
        },
        {
          key: 'docker',
          kind: 'docker' as const,
          label: 'Docker runtime data',
          detail: 'Pulse is receiving Docker telemetry.',
          action: 'stop-monitoring' as const,
        },
        {
          key: 'pbs',
          kind: 'pbs' as const,
          label: 'PBS data',
          detail: 'Pulse is receiving PBS telemetry.',
          action: 'stop-monitoring' as const,
        },
      ],
    };

    expect(getStopMonitoringScopeLabel(row)).toBe('Host telemetry, Docker runtime data, and PBS data');
  });

  it('maps install-profile flags into PowerShell installer env assignments', () => {
    expect(
      getPowerShellInstallProfileEnvFromFlags([
        '--enable-docker',
        '--disable-host',
        '--enable-proxmox',
        '--proxmox-type',
        'pbs',
      ]),
    ).toEqual([
      '$env:PULSE_ENABLE_DOCKER="true"',
      '$env:PULSE_ENABLE_HOST="false"',
      '$env:PULSE_ENABLE_PROXMOX="true"',
      '$env:PULSE_PROXMOX_TYPE="pbs"',
    ]);
  });
});
