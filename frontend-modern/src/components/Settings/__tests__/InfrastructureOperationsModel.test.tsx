import { describe, expect, it } from 'vitest';
import type { ConnectedInfrastructureItem } from '@/types/api';
import type { UnifiedAgentRow } from '../infrastructureOperationsModel';
import infrastructureInstallerSectionSource from '../InfrastructureInstallerSection.tsx?raw';
import infrastructureOperationsModelSource from '../infrastructureOperationsModel.tsx?raw';
import useInfrastructureInstallStateSource from '../useInfrastructureInstallState.tsx?raw';
import {
  INSTALL_PROFILE_OPTIONS,
  getCapabilityManagementPath,
  getCapabilitySurfaceLabel,
  getPlatformConnectionsViewForCapability,
  hasMachineInstallActions,
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
      linkedVmId: '101',
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
    expect(row.linkedVmId).toBe('101');
    expect(row.searchText).toContain('node-a.internal');
  });

  it('keeps host-managed stop monitoring scoped to the full host surface set', () => {
    const row: UnifiedAgentRow = {
      rowKey: 'agent-agent-1',
      id: 'agent-1',
      name: 'node-a',
      hostname: 'node-a.internal',
      capabilities: ['agent', 'docker', 'pbs'],
      status: 'active',
      upgradePlatform: 'linux',
      scope: {
        label: 'Default',
        detail: 'Auto-detect',
        category: 'default',
      },
      installFlags: ['--enable-docker', '--disable-host', '--enable-proxmox', '--proxmox-type pbs'],
      searchText: 'node-a node-a.internal agent-1',
      surfaces: [
        {
          key: 'agent',
          kind: 'agent',
          label: 'Host telemetry',
          detail: 'Pulse is receiving host telemetry.',
          action: 'stop-monitoring',
        },
        {
          key: 'docker',
          kind: 'docker',
          label: 'Docker runtime data',
          detail: 'Pulse is receiving Docker telemetry.',
          action: 'stop-monitoring',
        },
        {
          key: 'pbs',
          kind: 'pbs',
          label: 'PBS data',
          detail: 'Pulse is receiving PBS telemetry.',
          action: 'stop-monitoring',
        },
      ],
    };

    expect(getStopMonitoringScopeLabel(row)).toBe('Host telemetry and Docker runtime data');
  });

  it('treats truenas surfaces as platform-managed items instead of machine installs', () => {
    const item: ConnectedInfrastructureItem = {
      id: 'truenas-main',
      name: 'Tower NAS',
      hostname: 'truenas.local',
      status: 'active',
      version: '25.04.0',
      surfaces: [
        {
          id: 'truenas:truenas.local',
          kind: 'truenas',
          label: 'TrueNAS data',
          detail:
            'System, storage, app, and recovery telemetry polled through the configured TrueNAS connection.',
          idLabel: 'Hostname',
          idValue: 'truenas.local',
        },
      ],
    };

    const row = rowFromConnectedInfrastructureItem(item, {
      label: 'N/A',
      detail: '',
      category: 'na',
    });

    expect(row.capabilities).toEqual(['truenas']);
    expect(row.installFlags).toEqual([]);
    expect(hasMachineInstallActions(row)).toBe(false);
    expect(getCapabilityManagementPath('truenas')).toBe('/settings/infrastructure');
  });

  it('treats availability probes as agentless platform-managed items', () => {
    const item: ConnectedInfrastructureItem = {
      id: 'availability:energy-meter',
      name: 'Energy meter',
      hostname: '192.0.2.44',
      status: 'active',
      surfaces: [
        {
          id: 'availability:energy-meter',
          kind: 'availability',
          label: 'Availability data',
          detail: 'Pulse is checking this network endpoint with an agentless probe.',
          idLabel: 'Target ID',
          idValue: 'energy-meter',
        },
      ],
    };

    const row = rowFromConnectedInfrastructureItem(item, {
      label: 'N/A',
      detail: '',
      category: 'na',
    });

    expect(row.capabilities).toEqual(['availability']);
    expect(row.installFlags).toEqual([]);
    expect(hasMachineInstallActions(row)).toBe(false);
    expect(getCapabilitySurfaceLabel('availability')).toBe('Availability data');
    expect(getCapabilityManagementPath('availability')).toBe('/settings/infrastructure');
    expect(getPlatformConnectionsViewForCapability('availability')).toBeNull();
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

  it('keeps api-backed TrueNAS out of the host install profile list', () => {
    expect(INSTALL_PROFILE_OPTIONS.map((option) => option.value)).not.toContain('truenas');
  });

  it('keeps the recommended auto profile aligned with unpinned Proxmox detection', () => {
    const autoProfile = INSTALL_PROFILE_OPTIONS.find((option) => option.value === 'auto');

    expect(autoProfile).toBeDefined();
    expect(autoProfile?.flags).toEqual([]);
    expect(autoProfile?.description).toContain('recommended low-overhead per-machine install path');
    expect(autoProfile?.description).toContain('leaves the type unpinned');
    expect(autoProfile?.description).toContain('every detected PVE / PBS service');
  });

  it('keeps the Docker install profile aligned with the shared Docker and Podman label', () => {
    const dockerProfile = INSTALL_PROFILE_OPTIONS.find((option) => option.value === 'docker');

    expect(dockerProfile).toBeDefined();
    expect(dockerProfile?.label).toBe('Docker / Podman runtime');
    expect(dockerProfile?.description).toBe(
      'Force Docker / Podman monitoring when automatic detection is restricted.',
    );
    expect(dockerProfile?.description).not.toContain('container runtime');
  });

  it('keeps Proxmox node profiles explicit about per-node telemetry coverage', () => {
    const pveProfile = INSTALL_PROFILE_OPTIONS.find((option) => option.value === 'proxmox-pve');
    const pbsProfile = INSTALL_PROFILE_OPTIONS.find((option) => option.value === 'proxmox-pbs');

    expect(pveProfile?.description).toContain('each cluster member');
    expect(pveProfile?.description).toContain('SMART data');
    expect(pbsProfile?.description).toContain('local host telemetry');
  });

  it('keeps the embedded installer section on the canonical host-install framing', () => {
    expect(infrastructureInstallerSectionSource).toContain(
      "title={state.isEmbedded() ? presentation().title : 'Infrastructure'}",
    );
    expect(infrastructureInstallerSectionSource).toContain('Install on Unraid');
    expect(infrastructureInstallerSectionSource).toContain('Run on Unraid');
    expect(infrastructureInstallerSectionSource).toContain('Install on the Docker host');
    expect(infrastructureInstallerSectionSource).toContain('Install on a Kubernetes node');
    expect(infrastructureInstallerSectionSource).toContain(
      'state.handleInstallProfileChange(presentation().preferredProfile)',
    );
    expect(infrastructureInstallerSectionSource).toContain('Generate install token');
    expect(infrastructureInstallerSectionSource).toContain('Generate token');
    expect(infrastructureInstallerSectionSource).toContain(
      'This is the Pulse Agent handoff from first-run setup inside Add infrastructure.',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'Pulse Agent is a low-overhead background service.',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'For Proxmox clusters, keep the cluster API',
    );
    expect(infrastructureInstallerSectionSource).toContain('host-level');
    expect(infrastructureInstallerSectionSource).toContain('augmentation.');
    expect(infrastructureInstallerSectionSource).toContain('Installation commands');
    expect(infrastructureInstallerSectionSource).toContain(
      'Generate an install token first. Pulse will then build copy-ready commands',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'Allow Pulse Patrol to execute diagnostic and fix commands on this agent',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'Enable Pulse command execution (for Patrol remediation)',
    );
    expect(infrastructureInstallerSectionSource).not.toContain('Patrol auto-fix');
    expect(infrastructureInstallerSectionSource).not.toContain('auto-fix requires Pulse Pro');
    expect(infrastructureInstallerSectionSource).not.toContain('<api-token>');
    expect(infrastructureInstallerSectionSource).not.toContain(
      'Copy disabled until an install token is generated',
    );
  });

  it('keeps first-host completion handoff on Infrastructure instead of the retired dashboard', async () => {
    const installStateSource = await import('../useInfrastructureInstallState.tsx?raw').then(
      (mod) => (mod as { default: string }).default,
    );

    expect(infrastructureInstallerSectionSource).toContain('Open infrastructure');
    expect(infrastructureInstallerSectionSource).not.toContain('Open dashboard');
    expect(installStateSource).toContain('const openInfrastructure = () => {');
    expect(installStateSource).toContain('navigate(buildInfrastructureWorkspacePath())');
    expect(installStateSource).not.toContain('openDashboard');
    expect(installStateSource).not.toContain("navigate('/dashboard')");
  });

  it('only auto-creates setup handoff install tokens on installer routes', async () => {
    const installStateSource = await import('../useInfrastructureInstallState.tsx?raw').then(
      (mod) => (mod as { default: string }).default,
    );

    expect(installStateSource).toContain('const SETUP_HANDOFF_INSTALL_STEPS');
    expect(installStateSource).toContain(
      'deriveAddStepFromLocation(location.pathname, location.search)',
    );
    expect(installStateSource).toContain('setupHandoffInstallStepActive() &&');
  });

  it('keeps infrastructure install and operations surfaces free of retired commercial telemetry wrappers', () => {
    for (const source of [
      infrastructureOperationsModelSource,
      infrastructureInstallerSectionSource,
      useInfrastructureInstallStateSource,
    ]) {
      expect(source).not.toContain('upgradeMetrics');
      expect(source).not.toContain('conversionEvents');
      expect(source).not.toContain('infrastructureOnboardingMetrics');
      expect(source).not.toContain('UNIFIED_AGENT_TELEMETRY_SURFACE');
      expect(source).not.toContain('normalizeTelemetryPart');
      expect(source).not.toContain('trackAgentInstallTokenCreated');
      expect(source).not.toContain('trackAgentInstallCommandsCopied');
      expect(source).not.toContain('trackAgentFirstConnected');
      expect(source).not.toContain('/api/upgrade-metrics/events');
    }
  });

  it('does not reintroduce the retired reporting state hook on the operations state', async () => {
    const operationsStateSource = await import('../useInfrastructureOperationsState?raw').then(
      (mod) => (mod as { default: string }).default,
    );
    expect(operationsStateSource).not.toContain('useInfrastructureReportingState');
  });

  it('keeps discovered-node filtering anchored to canonical represented-host dedupe', async () => {
    const discoveryStateSource = await import('../useInfrastructureDiscoveryRuntimeState?raw').then(
      (mod) => (mod as { default: string }).default,
    );
    expect(discoveryStateSource).toContain('filterRepresentedDiscoveredServers');
    expect(discoveryStateSource).toContain('nodes()');
  });
});
