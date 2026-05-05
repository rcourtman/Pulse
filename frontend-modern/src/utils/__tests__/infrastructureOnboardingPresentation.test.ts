import { describe, expect, it } from 'vitest';
import {
  getInfrastructureCoverageCompleteActionPresentation,
  getInfrastructureApiProductsByGovernanceState,
  getInfrastructureAutoDetectLabels,
  getInfrastructureEmptyStateDetail,
  getInfrastructureEmptyStateSummary,
  getInfrastructureOnboardingProductPresentation,
  getInfrastructureSourceManagerProducts,
  getInfrastructureSourcePickerGroups,
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

    expect(agent.bestFor).toContain('full node-local telemetry');
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
    });
    expect(getInfrastructureSourceStrategyPresentation('api-agent')).toMatchObject({
      label: 'API first',
      summary: 'Platform API, agent optional',
    });

    expect(getInfrastructureOnboardingProductPresentation('vmware').sourceStrategy).toBe('api');
    expect(getInfrastructureOnboardingProductPresentation('pbs').sourceStrategy).toBe('api-agent');
  });

  it('keeps supported API products separate from the admitted VMware path', () => {
    expect(
      getInfrastructureApiProductsByGovernanceState('supported').map((product) => product.label),
    ).toEqual(['TrueNAS SCALE', 'Proxmox VE', 'Proxmox Backup Server', 'Proxmox Mail Gateway']);

    expect(
      getInfrastructureApiProductsByGovernanceState('admitted').map((product) => product.label),
    ).toEqual(['VMware vCenter']);
  });

  it('derives picker groups, auto-detect copy, and landing summaries from the shared helper', () => {
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
        type: 'agent',
        label: 'Standalone hosts',
        actionLabel: 'Install Pulse Agent',
      }),
    ]);

    expect(getInfrastructureSourcePickerGroups()).toEqual([
      {
        id: 'virtualization',
        label: 'Virtualization',
        description: 'Hypervisors, VM inventory, and cluster health.',
        types: ['vmware', 'pve'],
        products: [
          expect.objectContaining({ type: 'vmware', label: 'VMware vCenter' }),
          expect.objectContaining({ type: 'pve', label: 'Proxmox VE' }),
        ],
      },
      {
        id: 'storage',
        label: 'Storage',
        description: 'Storage appliances and dataset visibility.',
        types: ['truenas'],
        products: [expect.objectContaining({ type: 'truenas', label: 'TrueNAS SCALE' })],
      },
      {
        id: 'backup-mail',
        label: 'Backup and Mail',
        description: 'Backup infrastructure and mail-gateway operations.',
        types: ['pbs', 'pmg'],
        products: [
          expect.objectContaining({ type: 'pbs', label: 'Proxmox Backup Server' }),
          expect.objectContaining({ type: 'pmg', label: 'Proxmox Mail Gateway' }),
        ],
      },
      {
        id: 'host-monitoring',
        label: 'Host monitoring',
        description: 'Low-overhead machine telemetry and local service discovery.',
        types: ['agent'],
        products: [expect.objectContaining({ type: 'agent', label: 'Pulse Agent' })],
      },
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

  it('owns the source-manager coverage-complete copy outside the component', () => {
    expect(getInfrastructureCoverageCompleteActionPresentation()).toEqual({
      label: 'Coverage coherent',
      detail: 'Coverage looks coherent for the connected systems.',
    });
  });
});
