import { describe, expect, it } from 'vitest';
import {
  getInfrastructureApiProductsByGovernanceState,
  getInfrastructureAutoDetectLabels,
  getInfrastructureEmptyStateDetail,
  getInfrastructureOnboardingProductPresentation,
  getInfrastructureSupportSummaryBadges,
} from '@/utils/infrastructureOnboardingPresentation';

describe('infrastructureOnboardingPresentation', () => {
  it('keeps VMware on the admitted vCenter-only path', () => {
    const vmware = getInfrastructureOnboardingProductPresentation('vmware');

    expect(vmware.label).toBe('VMware vCenter');
    expect(vmware.governanceState).toBe('admitted');
  });

  it('keeps supported API products separate from the admitted VMware path', () => {
    expect(
      getInfrastructureApiProductsByGovernanceState('supported').map((product) => product.label),
    ).toEqual(['TrueNAS SCALE', 'Proxmox VE', 'Proxmox Backup Server', 'Proxmox Mail Gateway']);

    expect(
      getInfrastructureApiProductsByGovernanceState('admitted').map((product) => product.label),
    ).toEqual(['VMware vCenter']);
  });

  it('derives auto-detect copy and main-page support summary from the shared helper', () => {
    expect(getInfrastructureAutoDetectLabels()).toEqual([
      'VMware vCenter',
      'TrueNAS SCALE',
      'Proxmox VE',
      'Proxmox Backup Server',
      'Proxmox Mail Gateway',
    ]);

    expect(getInfrastructureSupportSummaryBadges()).toEqual({
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
      installPath: ['Linux', 'FreeBSD', 'Unraid', 'Pulse Agent hosts', 'Docker', 'Kubernetes'],
    });

    expect(getInfrastructureEmptyStateDetail()).toContain('Supported today: TrueNAS SCALE');
    expect(getInfrastructureEmptyStateDetail()).toContain('VMware vCenter is also available now.');
  });
});
