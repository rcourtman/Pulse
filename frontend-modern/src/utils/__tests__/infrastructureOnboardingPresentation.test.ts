import { describe, expect, it } from 'vitest';
import {
  getInfrastructureAgentHostProfileSupportText,
  getInfrastructureCoverageCompleteActionPresentation,
  getInfrastructureApiProductsByGovernanceState,
  getInfrastructureAutoDetectLabels,
  getInfrastructureEmptyStateDetail,
  getInfrastructureGovernanceBadgeLabel,
  getInfrastructureEmptyStateSummary,
  INFRASTRUCTURE_ONBOARDING_PATHS,
  getInfrastructureOnboardingProductPresentation,
  getInfrastructureSourceManagerProducts,
  getInfrastructureSourcePickerItemPresentation,
  getInfrastructureSourcePickerItems,
  getInfrastructureSourceStrategyPresentation,
  getInfrastructureSupportSummaryBadges,
} from '@/utils/infrastructureOnboardingPresentation';

describe('infrastructureOnboardingPresentation', () => {
  it('keeps VMware on the admitted vCenter-only path', () => {
    const vmware = getInfrastructureOnboardingProductPresentation('vmware');

    expect(vmware.label).toBe('VMware vCenter');
    expect(vmware.governanceState).toBe('admitted');
    expect(vmware.readinessStage).toBe('first-lab-ready');
    expect(vmware.primaryMode).toBe('api-backed');
    expect(vmware.canonicalProjections).toEqual(['agent', 'vm', 'storage']);
    expect(vmware.supportFloor).toMatchObject({
      setup: 'supported',
      visibility: 'supported',
      workloads: 'supported',
      storage: 'supported',
      recovery: 'n/a',
      alerts: 'supported',
      assistantRead: 'supported',
      assistantControl: 'read-only',
    });
  });

  it('frames Pulse Agent as the low-overhead per-machine path for full node-local telemetry', () => {
    const agent = getInfrastructureOnboardingProductPresentation('agent');
    const pve = getInfrastructureOnboardingProductPresentation('pve');

    expect(getInfrastructureAgentHostProfileSupportText()).toBe(
      'Linux, macOS, Windows, FreeBSD, and Unraid host/appliance profiles',
    );
    expect(agent.bestFor).toContain('host/appliance profiles');
    expect(agent.bestFor).toContain('low-overhead node-local telemetry');
    expect(agent.coverage).toContain('Low-overhead host telemetry');
    expect(agent.catalogDescription).toContain('Low-overhead host telemetry');
    expect(agent.sourceStrategy).toBe('agent');
    expect(pve.sourceStrategy).toBe('api-agent');
    expect(pve.coverage).toContain('through the Proxmox API');
    expect(pve.coverage).toContain('Install Pulse Agent only on nodes');
    expect(pve.coverage).toContain('SMART data');
  });

  it('keeps the shared source-strategy vocabulary explicit for add flows', () => {
    expect(getInfrastructureSourceStrategyPresentation('api')).toMatchObject({
      label: 'API inventory',
      summary: 'Platform API',
    });
    expect(getInfrastructureSourceStrategyPresentation('agent')).toMatchObject({
      label: 'Agent telemetry',
      summary: 'Pulse Agent',
      detail:
        'Installs Pulse Agent for Linux, macOS, Windows, FreeBSD, and Unraid host/appliance profiles, local services, Docker, and Kubernetes.',
    });
    expect(getInfrastructureSourceStrategyPresentation('api-agent')).toMatchObject({
      label: 'API first',
      summary: 'Platform API, agent optional',
    });
    expect(getInfrastructureSourceStrategyPresentation('probe')).toMatchObject({
      label: 'Availability probe',
      summary: 'Agentless probe',
    });

    expect(getInfrastructureOnboardingProductPresentation('vmware').sourceStrategy).toBe('api');
    expect(getInfrastructureOnboardingProductPresentation('truenas').sourceStrategy).toBe('api');
    expect(getInfrastructureOnboardingProductPresentation('pbs').sourceStrategy).toBe('api-agent');
    expect(getInfrastructureOnboardingProductPresentation('availability')).toMatchObject({
      sourceStrategy: 'probe',
      primaryMode: 'api-backed',
      canonicalProjections: ['network-endpoint'],
    });
    expect(INFRASTRUCTURE_ONBOARDING_PATHS.api.title).toBe('Connect platform API');
    expect(INFRASTRUCTURE_ONBOARDING_PATHS.agent.title).toBe('Install Pulse Agent');
  });

  it('keeps supported API products separate from the admitted VMware path', () => {
    expect(
      getInfrastructureApiProductsByGovernanceState('supported').map((product) => product.label),
    ).toEqual([
      'TrueNAS SCALE',
      'Proxmox VE',
      'Proxmox Backup Server',
      'Proxmox Mail Gateway',
      'Network endpoint',
    ]);

    expect(
      getInfrastructureApiProductsByGovernanceState('admitted').map((product) => product.label),
    ).toEqual(['VMware vCenter']);
    expect(
      getInfrastructureGovernanceBadgeLabel(
        getInfrastructureOnboardingProductPresentation('vmware').governanceState,
        getInfrastructureOnboardingProductPresentation('vmware').readinessStage,
      ),
    ).toBe('First lab ready');
    expect(getInfrastructureGovernanceBadgeLabel('supported', 'supported')).toBeNull();
  });

  it('derives picker items, auto-detect copy, and landing summaries from the shared helper', () => {
    expect(getInfrastructureSourceManagerProducts()).toEqual([
      expect.objectContaining({
        type: 'vmware',
        label: 'VMware vCenter',
        actionLabel: 'Add VMware vCenter',
      }),
      expect.objectContaining({
        type: 'truenas',
        label: 'TrueNAS SCALE',
        actionLabel: 'Add TrueNAS SCALE',
      }),
      expect.objectContaining({
        type: 'pve',
        label: 'Proxmox VE',
        actionLabel: 'Add Proxmox VE',
      }),
      expect.objectContaining({
        type: 'pbs',
        label: 'Proxmox Backup Server',
        actionLabel: 'Add Proxmox Backup Server',
      }),
      expect.objectContaining({
        type: 'pmg',
        label: 'Proxmox Mail Gateway',
        actionLabel: 'Add Proxmox Mail Gateway',
      }),
      expect.objectContaining({
        type: 'availability',
        label: 'Network endpoint',
        actionLabel: 'Add Network endpoint',
      }),
      expect.objectContaining({
        type: 'agent',
        label: 'Standalone hosts',
        actionLabel: 'Install Pulse Agent',
      }),
    ]);

    expect(getInfrastructureSourcePickerItems()).toEqual([
      expect.objectContaining({
        id: 'unraid',
        connectionType: 'agent',
        label: 'Unraid',
        catalogDescription: 'Array health, disks, Docker, host telemetry',
      }),
      expect.objectContaining({
        id: 'truenas',
        connectionType: 'truenas',
        label: 'TrueNAS SCALE',
      }),
      expect.objectContaining({ id: 'pve', connectionType: 'pve', label: 'Proxmox VE' }),
      expect.objectContaining({ id: 'docker', connectionType: 'agent', label: 'Docker' }),
      expect.objectContaining({
        id: 'linux-host',
        connectionType: 'agent',
        label: 'Linux, macOS, Windows host',
      }),
      expect.objectContaining({
        id: 'vmware',
        connectionType: 'vmware',
        label: 'VMware vCenter',
      }),
      expect.objectContaining({
        id: 'kubernetes',
        connectionType: 'agent',
        label: 'Kubernetes',
      }),
      expect.objectContaining({
        id: 'pbs',
        connectionType: 'pbs',
        label: 'Proxmox Backup Server',
      }),
      expect.objectContaining({
        id: 'pmg',
        connectionType: 'pmg',
        label: 'Proxmox Mail Gateway',
      }),
      expect.objectContaining({
        id: 'availability',
        connectionType: 'availability',
        label: 'Network endpoint',
      }),
    ]);

    expect(getInfrastructureAutoDetectLabels()).toEqual([
      'VMware vCenter',
      'TrueNAS SCALE',
      'Proxmox VE',
      'Proxmox Backup Server',
      'Proxmox Mail Gateway',
    ]);

    expect(getInfrastructureSupportSummaryBadges()).toMatchObject({
      supportedToday: [
        'TrueNAS SCALE',
        'Proxmox VE',
        'Proxmox Backup Server',
        'Proxmox Mail Gateway',
        'Network endpoint',
        'Pulse Agent hosts',
        'Docker',
        'Kubernetes',
      ],
      currentAdmissionPath: ['VMware vCenter'],
      installPath: expect.arrayContaining([
        'Linux',
        'FreeBSD',
        'Unraid',
        'Pulse Agent hosts',
        'Docker',
        'Kubernetes',
      ]),
    });

    expect(getInfrastructureEmptyStateSummary()).toBe(
      'Choose an infrastructure source to start monitoring your environment.',
    );
    expect(getInfrastructureEmptyStateDetail()).toContain(
      'Supported source types include VMware vCenter',
    );
    expect(getInfrastructureEmptyStateDetail()).toContain('standalone hosts through Pulse Agent');
    expect(getInfrastructureEmptyStateDetail()).toContain('Docker and Kubernetes are discovered');
  });

  it('keeps user-facing agent-backed catalog choices on the agent setup route', () => {
    expect(getInfrastructureSourcePickerItemPresentation('unraid')).toMatchObject({
      routeStep: 'unraid',
      connectionType: 'agent',
      label: 'Unraid',
      sourceStrategy: 'agent',
      catalogDescription: 'Array health, disks, Docker, host telemetry',
      searchAliases: expect.arrayContaining(['nas', 'home server']),
    });
    expect(getInfrastructureSourcePickerItemPresentation('docker')).toMatchObject({
      routeStep: 'docker',
      connectionType: 'agent',
      label: 'Docker',
    });
    expect(getInfrastructureSourcePickerItemPresentation('kubernetes')).toMatchObject({
      routeStep: 'kubernetes',
      connectionType: 'agent',
      label: 'Kubernetes',
    });
  });

  it('owns the source-manager coverage-complete copy outside the component', () => {
    expect(getInfrastructureCoverageCompleteActionPresentation()).toEqual({
      label: 'Coverage coherent',
      detail: 'Coverage looks coherent for the connected systems.',
    });
  });
});
