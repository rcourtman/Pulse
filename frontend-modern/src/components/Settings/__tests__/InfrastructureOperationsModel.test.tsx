import { describe, expect, it } from 'vitest';
import type { ConnectedInfrastructureItem } from '@/types/api';
import type { UnifiedAgentRow } from '../infrastructureOperationsModel';
import infrastructureInstallerSectionSource from '../InfrastructureInstallerSection.tsx?raw';
import infrastructureOperationsModelSource from '../infrastructureOperationsModel.tsx?raw';
import useInfrastructureConfiguredNodesStateSource from '../useInfrastructureConfiguredNodesState.ts?raw';
import useInfrastructureInstallStateSource from '../useInfrastructureInstallState.tsx?raw';
import { resolveAgentCommandPlatform } from '@/utils/agentInstallCommand';
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
      installFlags: ['--enable-docker', '--enable-proxmox', '--proxmox-type pbs'],
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
    expect(getCapabilityManagementPath('availability')).toBe('/settings/monitoring/availability');
    expect(getCapabilityManagementPath('proxmox')).toBe('/settings/infrastructure');
    expect(getCapabilityManagementPath('pbs')).toBe('/settings/infrastructure');
    expect(getCapabilityManagementPath('pmg')).toBe('/settings/infrastructure');
    expect(getCapabilityManagementPath('truenas')).toBe('/settings/infrastructure');
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

  it('keeps generic host onboarding explicit about the API-first Proxmox path', () => {
    expect(infrastructureInstallerSectionSource).toContain(
      'Adding Proxmox? Start with the API connection.',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'No root agent is required for normal platform inventory and metrics',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      "buildInfrastructureOnboardingPath('pve')",
    );
    expect(infrastructureInstallerSectionSource).toContain(
      "buildInfrastructureOnboardingPath('pbs')",
    );
    expect(infrastructureInstallerSectionSource).toContain('/docs/PRODUCTION_SECURITY');
  });

  it('keeps the Docker install profile aligned with the shared Docker and Podman label', () => {
    const dockerProfile = INSTALL_PROFILE_OPTIONS.find((option) => option.value === 'docker');

    expect(dockerProfile).toBeDefined();
    expect(dockerProfile?.label).toBe('Docker / Podman runtime');
    expect(dockerProfile?.description).toBe(
      'Force Docker / Podman monitoring when automatic detection is restricted, while keeping host telemetry enabled.',
    );
    expect(dockerProfile?.flags).toEqual(['--enable-docker']);
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
    expect(infrastructureInstallerSectionSource).toContain('Install for Docker / Podman');
    expect(infrastructureInstallerSectionSource).toContain('Docker inside Proxmox LXCs');
    expect(infrastructureInstallerSectionSource).toContain(
      'PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true',
    );
    expect(infrastructureInstallerSectionSource).toContain('bounded <code>pct exec</code>');
    expect(infrastructureInstallerSectionSource).toContain('Install on a Kubernetes node');
    expect(infrastructureInstallerSectionSource).toContain(
      'state.handleInstallProfileChange(presentation().preferredProfile)',
    );
    expect(infrastructureInstallerSectionSource).toContain('Generate install token');
    expect(infrastructureInstallerSectionSource).toContain('Generate token');
    expect(infrastructureInstallerSectionSource).toContain('Show token only');
    expect(infrastructureInstallerSectionSource).toContain(
      'This is the Pulse Agent handoff from first-run setup inside Add infrastructure.',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'Pulse Agent is a low-overhead background service.',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'API-backed platforms such as Proxmox start under Platform connections.',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'Use Availability checks for ping-only or agentless device monitoring.',
    );
    expect(infrastructureInstallerSectionSource).toContain('checks this Pulse URL and');
    expect(infrastructureInstallerSectionSource).toContain(
      'before asking for administrator privileges',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'For Proxmox, start with a dedicated read-only or narrowly scoped API token',
    );
    expect(infrastructureInstallerSectionSource).toContain('host-local');
    expect(infrastructureInstallerSectionSource).toContain('augmentation.');
    expect(infrastructureInstallerSectionSource).toContain('Installation commands');
    expect(infrastructureInstallerSectionSource).toContain(
      'Generate an install token first. Pulse will then build copy-ready commands',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'Allow Pulse-scoped command requests on this agent for Patrol actions and opted-in Proxmox LXC Docker inventory',
    );
    expect(infrastructureInstallerSectionSource).toContain(
      'Enable Pulse command execution (Patrol actions and Proxmox LXC Docker inventory)',
    );
    expect(infrastructureInstallerSectionSource).not.toContain('Patrol auto-fix');
    expect(infrastructureInstallerSectionSource).not.toContain('auto-fix requires Pulse Pro');
    expect(infrastructureInstallerSectionSource).not.toContain('<api-token>');
    expect(infrastructureInstallerSectionSource).not.toContain(
      'Copy disabled until an install token is generated',
    );
    expect(infrastructureOperationsModelSource).toContain(
      'preflights this Pulse URL, verifies the matching agent binary is available',
    );
    expect(infrastructureOperationsModelSource).toContain(
      'verifies the matching Windows agent binary is available',
    );
    expect(infrastructureOperationsModelSource).toContain('token-file handoff');
    expect(infrastructureOperationsModelSource).toContain('macOS may ask for your');
    expect(infrastructureOperationsModelSource).toContain('admin password');
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

  it('keeps setup-handoff token cleanup from masking API failures', () => {
    expect(useInfrastructureInstallStateSource).toContain(
      [
        '} finally {',
        '      if (!disposed) {',
        '        setIsGeneratingToken(false);',
        "        if (source === 'setup_handoff') {",
        '          setSetupHandoffAutoTokenPending(false);',
        '        }',
        '      }',
        '    }',
      ].join('\n'),
    );
    expect(useInfrastructureInstallStateSource).not.toMatch(
      /finally \{[\s\S]*?if \(disposed\) \{[\s\S]*?return;/,
    );
  });

  it('mirrors saved cluster member overrides onto cached node config', () => {
    // Both cluster override collections are write-only PUT fields. The nodes
    // cache must apply them to clusterEndpoints instead of spreading either
    // raw payload key onto config state.
    expect(useInfrastructureConfiguredNodesStateSource).toContain(
      'clusterNodeDisplayNameOverrides,',
    );
    expect(useInfrastructureConfiguredNodesStateSource).toContain(
      'applyClusterEndpointOverridesLocally(',
    );
    expect(useInfrastructureConfiguredNodesStateSource).toContain(
      'applyClusterNodeDisplayNamesLocally(',
    );
    expect(useInfrastructureConfiguredNodesStateSource).toContain(
      'clusterNodeDisplayNameOverrides?: ClusterNodeDisplayNameOverridePayload[]',
    );
    expect(useInfrastructureConfiguredNodesStateSource).not.toContain('...nodeData,');
  });

  it('keeps the retained settings state off the full-estate resource list', () => {
    expect(useInfrastructureConfiguredNodesStateSource).toContain("query: 'type=agent'");
    expect(useInfrastructureConfiguredNodesStateSource).toContain(
      "cacheKey: 'settings-configured-nodes'",
    );
    expect(useInfrastructureConfiguredNodesStateSource).toContain('aggregations()?.byType[type]');
    expect(useInfrastructureConfiguredNodesStateSource).not.toContain('useResources');
  });

  it('delegates install-token scopes to the server mint endpoint', () => {
    // The server decides scopes from the command-execution choice at mint
    // time (#1586); the frontend must not compose install-token scope lists.
    expect(useInfrastructureInstallStateSource).toContain('createHostAgentInstallToken');
    expect(useInfrastructureInstallStateSource).toContain('enableCommands: withCommands');
    expect(useInfrastructureInstallStateSource).not.toContain('SecurityAPI.createToken');
    expect(useInfrastructureInstallStateSource).not.toContain('AGENT_EXEC_SCOPE');
    expect(useInfrastructureInstallStateSource).not.toContain('AGENT_REPORT_SCOPE');
  });

  it('keeps install token scope and rendered command options atomic', () => {
    const lockIndex = useInfrastructureInstallStateSource.indexOf('setCurrentToken(null);');
    const replacementIndex = useInfrastructureInstallStateSource.indexOf(
      'generateInstallToken(source, { notifySuccess: false })',
    );

    expect(lockIndex).toBeGreaterThan(-1);
    expect(replacementIndex).toBeGreaterThan(lockIndex);
    expect(useInfrastructureInstallStateSource).toContain(
      'setEnableCommands(previousEnableCommands);',
    );
    expect(useInfrastructureInstallStateSource).toContain('setCurrentToken(supersededToken);');
    expect(infrastructureInstallerSectionSource).toContain('disabled={state.isGeneratingToken()}');
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

  it('routes Windows upgrade commands through the shared seamless installer command builder', async () => {
    const operationsStateSource = await import('../useInfrastructureOperationsState?raw').then(
      (mod) => (mod as { default: string }).default,
    );

    expect(operationsStateSource).toContain('buildWindowsAgentInstallCommand({');
    expect(operationsStateSource).toContain('extraEnvAssignments: envAssignments');
    expect(operationsStateSource).not.toContain('const tokenEnv = token ?');
  });

  it('pins the existing agent identity in the Unix credential repair command', async () => {
    const operationsStateSource = await import('../useInfrastructureOperationsState?raw').then(
      (mod) => (mod as { default: string }).default,
    );
    const repairStart = operationsStateSource.indexOf('if (replaceCredential) {');
    expect(repairStart).toBeGreaterThanOrEqual(0);
    const repairEnd = operationsStateSource.indexOf('buildUnixAgentInstallCommand({', repairStart);
    expect(repairEnd).toBeGreaterThan(repairStart);
    const repairBranch = operationsStateSource.slice(repairStart, repairEnd);
    // A repair reinstall without the identity can register a fresh suffixed
    // agent for the same machine instead of converging. Refs discussion #1748.
    expect(repairBranch).toContain('--agent-id ${shellQuoteArg(agentId)}');
    expect(repairBranch).toContain('--hostname ${shellQuoteArg(hostname)}');
  });

  it('keeps stale Unix agent update commands on the saved-state update path', async () => {
    const operationsStateSource = await import('../useInfrastructureOperationsState?raw').then(
      (mod) => (mod as { default: string }).default,
    );
    const agentUpgradeStart = operationsStateSource.indexOf(
      'const getAgentConnectionUpgradeCommand =',
    );
    const agentUpgradeEnd = operationsStateSource.indexOf(
      'const getAgentConnectionUpgradeCommandRequiresToken',
      agentUpgradeStart,
    );
    expect(agentUpgradeStart).toBeGreaterThanOrEqual(0);
    expect(agentUpgradeEnd).toBeGreaterThan(agentUpgradeStart);
    const agentUpgradeSource = operationsStateSource.slice(agentUpgradeStart, agentUpgradeEnd);
    const unixUpgradeStart = agentUpgradeSource.indexOf('let command = `curl');
    expect(unixUpgradeStart).toBeGreaterThanOrEqual(0);
    const unixUpgradeSource = agentUpgradeSource.slice(unixUpgradeStart);

    expect(agentUpgradeSource).toContain(
      '| bash -s -- --update --url ${shellQuoteArg(url)} --non-interactive',
    );
    const requiresTokenEnd = operationsStateSource.indexOf('return {', agentUpgradeEnd);
    expect(requiresTokenEnd).toBeGreaterThan(agentUpgradeEnd);
    const requiresTokenSource = operationsStateSource.slice(agentUpgradeEnd, requiresTokenEnd);
    expect(requiresTokenSource).toContain(
      'platformOverride ?? getConnectionUpgradePlatform(connection)',
    );
    expect(requiresTokenSource).toContain("=== 'windows'");
    expect(requiresTokenSource).toContain('installState.requiresToken()');
    expect(unixUpgradeSource).not.toContain('command += ` --token ${shellQuoteArg(token)}`;');
    expect(unixUpgradeSource).not.toContain('--agent-id');
    expect(unixUpgradeSource).not.toContain('--hostname');
  });

  it('resolves connection upgrade platforms through the shared caption-tolerant resolver', async () => {
    const operationsStateSource = await import('../useInfrastructureOperationsState?raw').then(
      (mod) => (mod as { default: string }).default,
    );
    // Legacy agents report gopsutil OS captions ("microsoft windows 11 pro"),
    // so the hook must route through resolveAgentCommandPlatform instead of
    // exact-matching platform tokens locally (refs #1555).
    expect(operationsStateSource).toContain(
      'resolveAgentCommandPlatform(connection.agentIdentity?.platform)',
    );
    expect(resolveAgentCommandPlatform('microsoft windows 11 pro')).toBe('windows');
    expect(resolveAgentCommandPlatform('linux')).toBe('linux');
  });

  it('keeps discovered-node filtering anchored to canonical represented-host dedupe', async () => {
    const discoveryStateSource = await import('../useInfrastructureDiscoveryRuntimeState?raw').then(
      (mod) => (mod as { default: string }).default,
    );
    expect(discoveryStateSource).toContain('filterRepresentedDiscoveredServers');
    expect(discoveryStateSource).toContain('nodes()');
  });

  // /api/discover is RequireAdmin + settings:write and Settings mounts this
  // hook for every settings tab, so both the one-shot read and the 30s poller
  // must consult the served infrastructure capability. Without the gate a
  // non-admin session reprinted a warn-level denial every 30 seconds for as
  // long as any settings page stayed open.
  it('gates the discovery read and its poller on the served infrastructure capability', async () => {
    const discoveryStateSource = await import('../useInfrastructureDiscoveryRuntimeState?raw').then(
      (mod) => (mod as { default: string }).default,
    );
    expect(discoveryStateSource).toContain('canReadInfrastructure: Accessor<boolean>');
    // The read guard sits ahead of the fetch...
    expect(discoveryStateSource).toMatch(
      /const loadDiscoveredNodes = async \(\) => \{\s*if \(!canReadInfrastructure\(\)\) \{\s*return;/,
    );
    // ...and the interval is only armed once the capability is granted.
    expect(discoveryStateSource).toMatch(
      /if \(!canReadInfrastructure\(\)\) \{\s*return;\s*\}\s*discoveryInterval = setInterval\(/,
    );
  });
});
