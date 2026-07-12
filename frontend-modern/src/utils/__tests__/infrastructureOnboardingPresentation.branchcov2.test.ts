import { describe, expect, it } from 'vitest';
import type { InfrastructureOnboardingConnectionType } from '@/utils/infrastructureOnboardingPresentation';
import {
  getInfrastructureAgentHostProfileSupportText,
  getInfrastructureGovernanceBadgeLabel,
  getInfrastructureOnboardingProductPresentation,
} from '@/utils/infrastructureOnboardingPresentation';

// DEFAULT_AGENT_SUPPORT_FLOOR is module-private in the source; restate the
// canonical shape so the manifest-null fallback arm of
// getInfrastructureOnboardingProductPresentation can be asserted concretely.
const DEFAULT_AGENT_SUPPORT_FLOOR = {
  setup: 'supported',
  visibility: 'supported',
  workloads: 'n/a',
  storage: 'supported',
  recovery: 'n/a',
  alerts: 'supported',
  assistantRead: 'supported',
  assistantControl: 'supported',
} as const;

describe('infrastructureOnboardingPresentation — branch coverage (branchcov2)', () => {
  describe('formatJoinedLabelList (via getInfrastructureAgentHostProfileSupportText)', () => {
    it('joins the multi-label (length >= 3) set with an Oxford comma', () => {
      // formatJoinedLabelList is module-private; its only public caller threads
      // the agent host-profile label set, which currently resolves to 5 labels.
      // This pins the >=3 arm: `${labels.slice(0, -1).join(', ')}, and ${last}`.
      expect(getInfrastructureAgentHostProfileSupportText()).toBe(
        'Linux, macOS, Windows, FreeBSD, and Unraid host/appliance profiles',
      );
    });
  });

  describe('getInfrastructureGovernanceBadgeLabel', () => {
    it('returns null for the supported governance state regardless of readiness stage', () => {
      // governanceState === 'supported' early-return arm.
      expect(getInfrastructureGovernanceBadgeLabel('supported', 'supported')).toBeNull();
      // readinessStage is ignored on this arm.
      expect(getInfrastructureGovernanceBadgeLabel('supported', 'first-lab-ready')).toBeNull();
    });

    it('labels the presentation-only governance state explicitly', () => {
      // governanceState === 'presentation-only' arm — not exercised by the sibling test.
      expect(getInfrastructureGovernanceBadgeLabel('presentation-only', 'supported')).toBe(
        'Presentation only',
      );
      expect(getInfrastructureGovernanceBadgeLabel('presentation-only', 'first-lab-ready')).toBe(
        'Presentation only',
      );
    });

    it('returns "Preview" once a non-supported platform clears the first-lab readiness gate', () => {
      // readinessStage === 'first-lab-ready' arm for a state that is neither
      // 'supported' nor 'presentation-only'.
      expect(getInfrastructureGovernanceBadgeLabel('admitted', 'first-lab-ready')).toBe('Preview');
    });

    it('falls back to "Preview" for admitted platforms that are not first-lab-ready', () => {
      // Final default `return 'Preview'` arm — readinessStage is NOT
      // 'first-lab-ready'. Not exercised by the sibling test.
      expect(getInfrastructureGovernanceBadgeLabel('admitted', 'supported')).toBe('Preview');
    });
  });

  describe('governanceStateForType (via getInfrastructureOnboardingProductPresentation)', () => {
    it('reports "supported" for agent because it carries no sourcePlatformId', () => {
      // governanceStateForType's `if (!sourcePlatformId) return 'supported'` arm.
      // Reached because agent has no sourcePlatformId, so manifestEntry is null
      // and getInfrastructureOnboardingProductPresentation delegates to it.
      expect(getInfrastructureOnboardingProductPresentation('agent').governanceState).toBe(
        'supported',
      );
    });

    it('reports "supported" for availability because it carries no sourcePlatformId', () => {
      expect(getInfrastructureOnboardingProductPresentation('availability').governanceState).toBe(
        'supported',
      );
    });

    it('reads governance straight from the manifest for API-backed platforms', () => {
      // For these types manifestEntry is non-null, so governanceStateForType is
      // never invoked; governance comes from manifestEntry.governanceState.
      expect(getInfrastructureOnboardingProductPresentation('vmware').governanceState).toBe(
        'admitted',
      );
      expect(getInfrastructureOnboardingProductPresentation('truenas').governanceState).toBe(
        'supported',
      );
      expect(getInfrastructureOnboardingProductPresentation('pve').governanceState).toBe(
        'supported',
      );
      expect(getInfrastructureOnboardingProductPresentation('pbs').governanceState).toBe(
        'supported',
      );
      expect(getInfrastructureOnboardingProductPresentation('pmg').governanceState).toBe(
        'supported',
      );
    });
  });

  describe('getInfrastructureOnboardingProductPresentation', () => {
    it('uses the manifest-null fallback chain for the agent product', () => {
      // agent has no sourcePlatformId, so manifestEntry is null and every
      // manifest-driven field falls through its ?? chain.
      const agent = getInfrastructureOnboardingProductPresentation('agent');
      expect(agent).toStrictEqual({
        type: 'agent',
        label: 'Pulse Agent',
        bestFor:
          'Linux, macOS, Windows, FreeBSD, and Unraid host/appliance profiles where you want low-overhead node-local telemetry.',
        coverage: 'Low-overhead host telemetry, SMART, services, Docker, and Kubernetes',
        catalogDescription: 'Low-overhead host telemetry, services, Docker, Kubernetes',
        searchAliases: ['host', 'server', 'machine', 'node', 'ubuntu', 'debian', 'windows', 'mac'],
        sourceStrategy: 'agent',
        autoDetect: false,
        // governanceState: undefined ?? governanceStateForType('agent') -> 'supported'
        governanceState: 'supported',
        // readinessStage: undefined ?? 'supported'
        readinessStage: 'supported',
        // primaryMode: undefined ?? undefined ?? 'agent-backed'  (final fallback arm)
        primaryMode: 'agent-backed',
        // canonicalProjections: undefined ?? undefined ?? ['agent']  (final fallback arm)
        canonicalProjections: ['agent'],
        // supportFloor: undefined ?? DEFAULT_AGENT_SUPPORT_FLOOR
        supportFloor: DEFAULT_AGENT_SUPPORT_FLOOR,
        defaultSurfaceKeys: ['host'],
      });
    });

    it('uses the presentation-side fallback values for the availability probe product', () => {
      // availability has no sourcePlatformId, so manifestEntry is null. Its
      // presentation DOES define primaryMode + canonicalProjections, so those
      // feed the middle arm of each ?? chain instead of the final defaults.
      const availability = getInfrastructureOnboardingProductPresentation('availability');
      expect(availability).toStrictEqual({
        type: 'availability',
        label: 'Network endpoint',
        bestFor: 'Devices that expose ICMP, TCP, or HTTP but cannot run Pulse Agent',
        coverage: 'Agentless availability checks and downtime alerts',
        catalogDescription: 'Ping, TCP port, and HTTP availability checks',
        searchAliases: [
          'ping',
          'icmp',
          'tcp',
          'http',
          'endpoint',
          'probe',
          'port',
          'website',
          'ip address',
          'mqtt',
          'esphome',
        ],
        sourceStrategy: 'probe',
        autoDetect: false,
        governanceState: 'supported',
        readinessStage: 'supported',
        // primaryMode: undefined ?? presentation.primaryMode('api-backed')
        primaryMode: 'api-backed',
        // canonicalProjections: undefined ?? presentation.canonicalProjections
        canonicalProjections: ['network-endpoint'],
        supportFloor: DEFAULT_AGENT_SUPPORT_FLOOR,
        defaultSurfaceKeys: ['availability'],
      });
    });

    it('reads readiness, canonicalProjections and supportFloor from the manifest for vmware', () => {
      // vmware is admitted/first-lab-ready, exercising the manifest side of
      // every ?? (manifestEntry?.X is non-null for each field).
      const vmware = getInfrastructureOnboardingProductPresentation('vmware');
      expect(vmware.governanceState).toBe('admitted');
      expect(vmware.readinessStage).toBe('first-lab-ready');
      expect(vmware.primaryMode).toBe('api-backed');
      expect(vmware.canonicalProjections).toStrictEqual(['agent', 'vm', 'storage', 'network']);
      expect(vmware.supportFloor).toStrictEqual({
        setup: 'supported',
        visibility: 'supported',
        workloads: 'supported',
        storage: 'supported',
        recovery: 'n/a',
        alerts: 'supported',
        assistantRead: 'supported',
        assistantControl: 'read-only',
      });
      expect(vmware.searchAliases).toStrictEqual(['vsphere', 'esxi', 'vcenter', 'vmware cluster']);
    });

    it('pulls platform-specific manifest canonicalProjections for every supported API platform', () => {
      // Each platform exercises a distinct manifestEntry.canonicalProjections,
      // confirming the manifest side of the ?? is data-driven per type.
      expect(getInfrastructureOnboardingProductPresentation('truenas').canonicalProjections)
        .toStrictEqual([
          'agent',
          'vm',
          'app-container',
          'network-share',
          'storage',
          'physical-disk',
        ]);
      expect(getInfrastructureOnboardingProductPresentation('pve').canonicalProjections)
        .toStrictEqual(['agent', 'vm', 'system-container', 'storage', 'ceph', 'physical-disk']);
      expect(getInfrastructureOnboardingProductPresentation('pbs').canonicalProjections)
        .toStrictEqual(['pbs', 'storage']);
      expect(getInfrastructureOnboardingProductPresentation('pmg').canonicalProjections).toStrictEqual(
        ['pmg'],
      );
    });

    it('reads the manifest supportFloor for supported API platforms', () => {
      // pve assistantControl ('augmentation-only') and pbs workloads ('n/a')
      // differ from DEFAULT_AGENT_SUPPORT_FLOOR, pinning the manifest side of
      // the supportFloor ?? rather than the fallback.
      expect(getInfrastructureOnboardingProductPresentation('pve').supportFloor).toStrictEqual({
        setup: 'supported',
        visibility: 'supported',
        workloads: 'supported',
        storage: 'supported',
        recovery: 'supported',
        alerts: 'supported',
        assistantRead: 'supported',
        assistantControl: 'augmentation-only',
      });
      expect(getInfrastructureOnboardingProductPresentation('pbs').supportFloor).toStrictEqual({
        setup: 'supported',
        visibility: 'supported',
        workloads: 'n/a',
        storage: 'supported',
        recovery: 'supported',
        alerts: 'supported',
        assistantRead: 'supported',
        assistantControl: 'read-only',
      });
    });

    it('keeps the presentation searchAliases (never the [] fallback) for every type', () => {
      // The `presentation.searchAliases ?? []` expression: every product
      // defines searchAliases, so the [] fallback arm never fires for a valid
      // type. Pin each exact array to prove the presentation side is used.
      const expectedAliases: Record<InfrastructureOnboardingConnectionType, readonly string[]> = {
        agent: ['host', 'server', 'machine', 'node', 'ubuntu', 'debian', 'windows', 'mac'],
        availability: [
          'ping',
          'icmp',
          'tcp',
          'http',
          'endpoint',
          'probe',
          'port',
          'website',
          'ip address',
          'mqtt',
          'esphome',
        ],
        pve: ['proxmox', 'pve', 'hypervisor', 'vm host', 'cluster'],
        pbs: ['backup', 'proxmox backup', 'pbs'],
        pmg: ['mail', 'email', 'gateway', 'proxmox mail', 'pmg'],
        truenas: ['nas', 'storage', 'zfs', 'truenas scale'],
        vmware: ['vsphere', 'esxi', 'vcenter', 'vmware cluster'],
      };
      const types = Object.keys(expectedAliases) as InfrastructureOnboardingConnectionType[];
      for (const type of types) {
        expect(getInfrastructureOnboardingProductPresentation(type).searchAliases).toStrictEqual(
          expectedAliases[type],
        );
      }
    });
  });
});
