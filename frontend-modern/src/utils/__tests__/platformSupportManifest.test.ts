import { describe, expect, it } from 'vitest';
import {
  getSourcePlatformManifestEntry,
  getSourcePlatformReadinessStage,
  getSourcePlatformSurfaceKind,
  getSourcePlatformPrimaryMode,
  getSourcePlatformCanonicalProjections,
  getSourcePlatformOnboardingPaths,
  getSourcePlatformFamily,
  getSourcePlatformStorageFamily,
  getAgentHostProfileRuntimePlatform,
  sourcePlatformIsRuntimeLens,
  sourcePlatformIsOwningPlatform,
  sourcePlatformSupportsOnboardingPath,
} from '@/utils/platformSupportManifest';

describe('platformSupportManifest', () => {
  describe('getSourcePlatformManifestEntry', () => {
    it('resolves a known platform by canonical id', () => {
      expect(getSourcePlatformManifestEntry('agent')?.id).toBe('agent');
      expect(getSourcePlatformManifestEntry('truenas')?.id).toBe('truenas');
    });

    it('resolves alias tokens through the alias map before any display-token fallback', () => {
      expect(getSourcePlatformManifestEntry('pve')?.id).toBe('proxmox-pve');
      expect(getSourcePlatformManifestEntry('proxmox')?.id).toBe('proxmox-pve');
      expect(getSourcePlatformManifestEntry('pbs')?.id).toBe('proxmox-pbs');
      expect(getSourcePlatformManifestEntry('pmg')?.id).toBe('proxmox-pmg');
      expect(getSourcePlatformManifestEntry('k8s')?.id).toBe('kubernetes');
      expect(getSourcePlatformManifestEntry('vmware')?.id).toBe('vmware-vsphere');
      expect(getSourcePlatformManifestEntry('hyper-v')?.id).toBe('microsoft-hyperv');
    });

    it('normalizes case and surrounding whitespace before lookup', () => {
      expect(getSourcePlatformManifestEntry('AGENT')?.id).toBe('agent');
      expect(getSourcePlatformManifestEntry('  Proxmox-PVE  ')?.id).toBe('proxmox-pve');
      expect(getSourcePlatformManifestEntry('\tK8S\n')?.id).toBe('kubernetes');
    });

    it('falls back to display tokens when no id or alias matches', () => {
      expect(getSourcePlatformManifestEntry('Container runtime')?.id).toBe('docker');
      expect(getSourcePlatformManifestEntry('podman')?.id).toBe('docker');
      expect(getSourcePlatformManifestEntry('Proxmox VE')?.id).toBe('proxmox-pve');
      expect(getSourcePlatformManifestEntry('vmware vsphere')?.id).toBe('vmware-vsphere');
    });

    it('returns null for empty, whitespace-only, null, and undefined inputs', () => {
      expect(getSourcePlatformManifestEntry(null)).toBeNull();
      expect(getSourcePlatformManifestEntry(undefined)).toBeNull();
      expect(getSourcePlatformManifestEntry('')).toBeNull();
      expect(getSourcePlatformManifestEntry('   ')).toBeNull();
    });

    it('returns null for unknown tokens that are neither ids, aliases, nor display tokens', () => {
      expect(getSourcePlatformManifestEntry('mystery-platform')).toBeNull();
      expect(getSourcePlatformManifestEntry('docker-host')).toBeNull();
    });
  });

  describe('getSourcePlatformReadinessStage', () => {
    it('returns the readiness stage for supported and presentation-only platforms', () => {
      expect(getSourcePlatformReadinessStage('agent')).toBe('supported');
      expect(getSourcePlatformReadinessStage('docker')).toBe('supported');
      expect(getSourcePlatformReadinessStage('proxmox-pve')).toBe('supported');
      expect(getSourcePlatformReadinessStage('unraid')).toBe('presentation-only');
      expect(getSourcePlatformReadinessStage('synology-dsm')).toBe('presentation-only');
    });

    it('resolves aliases and case-insensitive tokens to the canonical stage', () => {
      expect(getSourcePlatformReadinessStage('pve')).toBe('supported');
      expect(getSourcePlatformReadinessStage('k8s')).toBe('supported');
      expect(getSourcePlatformReadinessStage('AGENT')).toBe('supported');
      expect(getSourcePlatformReadinessStage('Truenas')).toBe('supported');
    });

    it('returns null for unknown or empty input', () => {
      expect(getSourcePlatformReadinessStage(null)).toBeNull();
      expect(getSourcePlatformReadinessStage('')).toBeNull();
      expect(getSourcePlatformReadinessStage('mystery-platform')).toBeNull();
    });
  });

  describe('getSourcePlatformSurfaceKind', () => {
    it('classifies owning platforms, runtime lenses, and presentation-only surfaces', () => {
      expect(getSourcePlatformSurfaceKind('agent')).toBe('platform');
      expect(getSourcePlatformSurfaceKind('kubernetes')).toBe('platform');
      expect(getSourcePlatformSurfaceKind('proxmox-pve')).toBe('platform');
      expect(getSourcePlatformSurfaceKind('unraid')).toBe('presentation-only');
      expect(getSourcePlatformSurfaceKind('aws')).toBe('presentation-only');
    });

    it('resolves aliases and case-insensitive tokens to the canonical surface kind', () => {
      expect(getSourcePlatformSurfaceKind('vmware')).toBe('platform');
      expect(getSourcePlatformSurfaceKind('hyper-v')).toBe('presentation-only');
      expect(getSourcePlatformSurfaceKind('AGENT')).toBe('platform');
      expect(getSourcePlatformSurfaceKind('KUBERNETES')).toBe('platform');
    });

    it('returns null for unknown or empty input', () => {
      expect(getSourcePlatformSurfaceKind(null)).toBeNull();
      expect(getSourcePlatformSurfaceKind(undefined)).toBeNull();
      expect(getSourcePlatformSurfaceKind('')).toBeNull();
      expect(getSourcePlatformSurfaceKind('mystery-platform')).toBeNull();
    });
  });

  describe('sourcePlatformIsRuntimeLens', () => {
    it('returns true for the runtime-lens surface kind', () => {
      expect(sourcePlatformIsRuntimeLens('docker')).toBe(true);
      expect(sourcePlatformIsRuntimeLens('Container runtime')).toBe(true);
      expect(sourcePlatformIsRuntimeLens('podman')).toBe(true);
    });

    it('returns false for owning platforms and presentation-only surfaces', () => {
      expect(sourcePlatformIsRuntimeLens('agent')).toBe(false);
      expect(sourcePlatformIsRuntimeLens('kubernetes')).toBe(false);
      expect(sourcePlatformIsRuntimeLens('proxmox-pve')).toBe(false);
      expect(sourcePlatformIsRuntimeLens('unraid')).toBe(false);
      expect(sourcePlatformIsRuntimeLens('aws')).toBe(false);
    });

    it('returns false for unknown or empty input', () => {
      expect(sourcePlatformIsRuntimeLens(null)).toBe(false);
      expect(sourcePlatformIsRuntimeLens('')).toBe(false);
      expect(sourcePlatformIsRuntimeLens('mystery-platform')).toBe(false);
    });
  });

  describe('sourcePlatformIsOwningPlatform', () => {
    it('returns true for platform surface kinds', () => {
      expect(sourcePlatformIsOwningPlatform('agent')).toBe(true);
      expect(sourcePlatformIsOwningPlatform('kubernetes')).toBe(true);
      expect(sourcePlatformIsOwningPlatform('proxmox-pve')).toBe(true);
      expect(sourcePlatformIsOwningPlatform('truenas')).toBe(true);
    });

    it('resolves aliases and case-insensitive tokens', () => {
      expect(sourcePlatformIsOwningPlatform('pve')).toBe(true);
      expect(sourcePlatformIsOwningPlatform('k8s')).toBe(true);
      expect(sourcePlatformIsOwningPlatform('vmware')).toBe(true);
      expect(sourcePlatformIsOwningPlatform('AGENT')).toBe(true);
      expect(sourcePlatformIsOwningPlatform('KUBERNETES')).toBe(true);
    });

    it('returns false for runtime lenses, presentation-only surfaces, and unknown input', () => {
      expect(sourcePlatformIsOwningPlatform('docker')).toBe(false);
      expect(sourcePlatformIsOwningPlatform('unraid')).toBe(false);
      expect(sourcePlatformIsOwningPlatform('gcp')).toBe(false);
      expect(sourcePlatformIsOwningPlatform(null)).toBe(false);
      expect(sourcePlatformIsOwningPlatform('')).toBe(false);
      expect(sourcePlatformIsOwningPlatform('mystery-platform')).toBe(false);
    });
  });

  describe('getSourcePlatformPrimaryMode', () => {
    it('returns agent-backed, api-backed, and presentation-only modes for known platforms', () => {
      expect(getSourcePlatformPrimaryMode('agent')).toBe('agent-backed');
      expect(getSourcePlatformPrimaryMode('docker')).toBe('agent-backed');
      expect(getSourcePlatformPrimaryMode('kubernetes')).toBe('agent-backed');
      expect(getSourcePlatformPrimaryMode('proxmox-pve')).toBe('api-backed');
      expect(getSourcePlatformPrimaryMode('truenas')).toBe('api-backed');
      expect(getSourcePlatformPrimaryMode('vmware-vsphere')).toBe('api-backed');
      expect(getSourcePlatformPrimaryMode('unraid')).toBe('presentation-only');
      expect(getSourcePlatformPrimaryMode('aws')).toBe('presentation-only');
    });

    it('resolves aliases and case-insensitive tokens to the canonical mode', () => {
      expect(getSourcePlatformPrimaryMode('pve')).toBe('api-backed');
      expect(getSourcePlatformPrimaryMode('pbs')).toBe('api-backed');
      expect(getSourcePlatformPrimaryMode('pmg')).toBe('api-backed');
      expect(getSourcePlatformPrimaryMode('k8s')).toBe('agent-backed');
      expect(getSourcePlatformPrimaryMode('vmware')).toBe('api-backed');
      expect(getSourcePlatformPrimaryMode('AGENT')).toBe('agent-backed');
      expect(getSourcePlatformPrimaryMode('PROXMOX-PBS')).toBe('api-backed');
    });

    it('returns null for unknown or empty input', () => {
      expect(getSourcePlatformPrimaryMode(null)).toBeNull();
      expect(getSourcePlatformPrimaryMode('')).toBeNull();
      expect(getSourcePlatformPrimaryMode('mystery-platform')).toBeNull();
    });
  });

  describe('getSourcePlatformCanonicalProjections', () => {
    it('returns the canonical projection list for known platforms', () => {
      expect(getSourcePlatformCanonicalProjections('agent')).toEqual([
        'agent',
        'storage',
        'physical-disk',
      ]);
      expect(getSourcePlatformCanonicalProjections('proxmox-pbs')).toEqual(['pbs', 'storage']);
      expect(getSourcePlatformCanonicalProjections('proxmox-pmg')).toEqual(['pmg']);
    });

    it('resolves aliases and case-insensitive tokens to the canonical projections', () => {
      expect(getSourcePlatformCanonicalProjections('pve')).toEqual([
        'agent',
        'vm',
        'system-container',
        'storage',
        'ceph',
        'physical-disk',
      ]);
      expect(getSourcePlatformCanonicalProjections('pbs')).toEqual(['pbs', 'storage']);
      expect(getSourcePlatformCanonicalProjections('AGENT')).toEqual([
        'agent',
        'storage',
        'physical-disk',
      ]);
    });

    it('returns an empty array for presentation-only platforms with no projections', () => {
      expect(getSourcePlatformCanonicalProjections('unraid')).toEqual([]);
      expect(getSourcePlatformCanonicalProjections('aws')).toEqual([]);
      expect(getSourcePlatformCanonicalProjections('gcp')).toEqual([]);
    });

    it('returns an empty array for unknown and empty input', () => {
      expect(getSourcePlatformCanonicalProjections(null)).toEqual([]);
      expect(getSourcePlatformCanonicalProjections('')).toEqual([]);
      expect(getSourcePlatformCanonicalProjections('mystery-platform')).toEqual([]);
    });
  });

  describe('getSourcePlatformOnboardingPaths', () => {
    it('returns install-workspace for agent-backed platforms', () => {
      expect(getSourcePlatformOnboardingPaths('agent')).toEqual(['install-workspace']);
      expect(getSourcePlatformOnboardingPaths('docker')).toEqual(['install-workspace']);
      expect(getSourcePlatformOnboardingPaths('kubernetes')).toEqual(['install-workspace']);
    });

    it('returns platform-connections for api-backed platforms', () => {
      expect(getSourcePlatformOnboardingPaths('proxmox-pve')).toEqual(['platform-connections']);
      expect(getSourcePlatformOnboardingPaths('proxmox-pbs')).toEqual(['platform-connections']);
      expect(getSourcePlatformOnboardingPaths('truenas')).toEqual(['platform-connections']);
      expect(getSourcePlatformOnboardingPaths('vmware-vsphere')).toEqual(['platform-connections']);
    });

    it('resolves aliases and case-insensitive tokens to the canonical paths', () => {
      expect(getSourcePlatformOnboardingPaths('pve')).toEqual(['platform-connections']);
      expect(getSourcePlatformOnboardingPaths('k8s')).toEqual(['install-workspace']);
      expect(getSourcePlatformOnboardingPaths('vmware')).toEqual(['platform-connections']);
      expect(getSourcePlatformOnboardingPaths('AGENT')).toEqual(['install-workspace']);
    });

    it('returns an empty array for presentation-only platforms and unknown input', () => {
      expect(getSourcePlatformOnboardingPaths('unraid')).toEqual([]);
      expect(getSourcePlatformOnboardingPaths('aws')).toEqual([]);
      expect(getSourcePlatformOnboardingPaths(null)).toEqual([]);
      expect(getSourcePlatformOnboardingPaths('')).toEqual([]);
      expect(getSourcePlatformOnboardingPaths('mystery-platform')).toEqual([]);
    });
  });

  describe('getSourcePlatformFamily', () => {
    it('returns the family label for known platforms', () => {
      expect(getSourcePlatformFamily('agent')).toBe('Pulse-managed host');
      expect(getSourcePlatformFamily('docker')).toBe('Container runtime');
      expect(getSourcePlatformFamily('kubernetes')).toBe('Cluster runtime');
      expect(getSourcePlatformFamily('proxmox-pve')).toBe('Proxmox');
      expect(getSourcePlatformFamily('proxmox-pbs')).toBe('Proxmox');
      expect(getSourcePlatformFamily('proxmox-pmg')).toBe('Proxmox');
      expect(getSourcePlatformFamily('aws')).toBe('AWS');
    });

    it('resolves aliases and case-insensitive tokens to the canonical family', () => {
      expect(getSourcePlatformFamily('pve')).toBe('Proxmox');
      expect(getSourcePlatformFamily('pbs')).toBe('Proxmox');
      expect(getSourcePlatformFamily('pmg')).toBe('Proxmox');
      expect(getSourcePlatformFamily('k8s')).toBe('Cluster runtime');
      expect(getSourcePlatformFamily('vmware')).toBe('VMware');
      expect(getSourcePlatformFamily('hyper-v')).toBe('Hyper-V');
      expect(getSourcePlatformFamily('AGENT')).toBe('Pulse-managed host');
      expect(getSourcePlatformFamily('DOCKER')).toBe('Container runtime');
    });

    it('falls back to display tokens for non-alias labels', () => {
      expect(getSourcePlatformFamily('Container runtime')).toBe('Container runtime');
      expect(getSourcePlatformFamily('podman')).toBe('Container runtime');
    });

    it('returns null for unknown or empty input', () => {
      expect(getSourcePlatformFamily(null)).toBeNull();
      expect(getSourcePlatformFamily('')).toBeNull();
      expect(getSourcePlatformFamily('mystery-platform')).toBeNull();
    });
  });

  describe('getSourcePlatformStorageFamily', () => {
    it('returns the storage family for known platforms', () => {
      expect(getSourcePlatformStorageFamily('agent')).toBe('onprem');
      expect(getSourcePlatformStorageFamily('truenas')).toBe('onprem');
      expect(getSourcePlatformStorageFamily('docker')).toBe('container');
      expect(getSourcePlatformStorageFamily('kubernetes')).toBe('container');
      expect(getSourcePlatformStorageFamily('proxmox-pve')).toBe('virtualization');
      expect(getSourcePlatformStorageFamily('vmware-vsphere')).toBe('virtualization');
      expect(getSourcePlatformStorageFamily('aws')).toBe('cloud');
      expect(getSourcePlatformStorageFamily('azure')).toBe('cloud');
      expect(getSourcePlatformStorageFamily('gcp')).toBe('cloud');
    });

    it('resolves aliases and case-insensitive tokens to the canonical storage family', () => {
      expect(getSourcePlatformStorageFamily('pve')).toBe('virtualization');
      expect(getSourcePlatformStorageFamily('pbs')).toBe('virtualization');
      expect(getSourcePlatformStorageFamily('k8s')).toBe('container');
      expect(getSourcePlatformStorageFamily('vmware')).toBe('virtualization');
      expect(getSourcePlatformStorageFamily('hyper-v')).toBe('virtualization');
      expect(getSourcePlatformStorageFamily('DOCKER')).toBe('container');
      expect(getSourcePlatformStorageFamily('AGENT')).toBe('onprem');
      expect(getSourcePlatformStorageFamily('AWS')).toBe('cloud');
    });

    it('returns null for unknown or empty input', () => {
      expect(getSourcePlatformStorageFamily(null)).toBeNull();
      expect(getSourcePlatformStorageFamily('')).toBeNull();
      expect(getSourcePlatformStorageFamily('mystery-platform')).toBeNull();
    });
  });

  describe('getAgentHostProfileRuntimePlatform', () => {
    it('resolves the runtime platform via host identity tokens and case variants', () => {
      expect(getAgentHostProfileRuntimePlatform('unraid-os')).toBe('linux');
      expect(getAgentHostProfileRuntimePlatform('unraid os')).toBe('linux');
      expect(getAgentHostProfileRuntimePlatform('Unraid')).toBe('linux');
      expect(getAgentHostProfileRuntimePlatform('UNRAID-OS')).toBe('linux');
    });

    it('returns null for unknown or empty input', () => {
      expect(getAgentHostProfileRuntimePlatform(null)).toBeNull();
      expect(getAgentHostProfileRuntimePlatform('')).toBeNull();
      expect(getAgentHostProfileRuntimePlatform('mystery-host')).toBeNull();
      expect(getAgentHostProfileRuntimePlatform('docker')).toBeNull();
    });
  });

  describe('sourcePlatformSupportsOnboardingPath', () => {
    it('returns true when the platform declares the requested onboarding path', () => {
      expect(sourcePlatformSupportsOnboardingPath('agent', 'install-workspace')).toBe(true);
      expect(sourcePlatformSupportsOnboardingPath('docker', 'install-workspace')).toBe(true);
      expect(sourcePlatformSupportsOnboardingPath('kubernetes', 'install-workspace')).toBe(true);
      expect(sourcePlatformSupportsOnboardingPath('proxmox-pve', 'platform-connections')).toBe(
        true,
      );
      expect(sourcePlatformSupportsOnboardingPath('truenas', 'platform-connections')).toBe(true);
      expect(sourcePlatformSupportsOnboardingPath('vmware-vsphere', 'platform-connections')).toBe(
        true,
      );
    });

    it('returns false when the platform declares a different onboarding path', () => {
      expect(sourcePlatformSupportsOnboardingPath('agent', 'platform-connections')).toBe(false);
      expect(sourcePlatformSupportsOnboardingPath('docker', 'platform-connections')).toBe(false);
      expect(sourcePlatformSupportsOnboardingPath('proxmox-pve', 'install-workspace')).toBe(false);
      expect(sourcePlatformSupportsOnboardingPath('truenas', 'install-workspace')).toBe(false);
    });

    it('resolves aliases and case-insensitive tokens before checking paths', () => {
      expect(sourcePlatformSupportsOnboardingPath('pve', 'platform-connections')).toBe(true);
      expect(sourcePlatformSupportsOnboardingPath('k8s', 'install-workspace')).toBe(true);
      expect(sourcePlatformSupportsOnboardingPath('vmware', 'platform-connections')).toBe(true);
      expect(sourcePlatformSupportsOnboardingPath('AGENT', 'install-workspace')).toBe(true);
      expect(sourcePlatformSupportsOnboardingPath('Proxmox-PVE', 'platform-connections')).toBe(
        true,
      );
    });

    it('returns false for presentation-only platforms, unknown input, and empty input', () => {
      expect(sourcePlatformSupportsOnboardingPath('unraid', 'install-workspace')).toBe(false);
      expect(sourcePlatformSupportsOnboardingPath('unraid', 'platform-connections')).toBe(false);
      expect(sourcePlatformSupportsOnboardingPath('aws', 'install-workspace')).toBe(false);
      expect(sourcePlatformSupportsOnboardingPath(null, 'install-workspace')).toBe(false);
      expect(sourcePlatformSupportsOnboardingPath('', 'install-workspace')).toBe(false);
      expect(sourcePlatformSupportsOnboardingPath('mystery-platform', 'install-workspace')).toBe(
        false,
      );
    });
  });
});
