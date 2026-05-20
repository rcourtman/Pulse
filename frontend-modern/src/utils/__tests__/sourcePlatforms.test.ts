import { describe, expect, it } from 'vitest';
import {
  getSourcePlatformLabel,
  getSourcePlatformPresentation,
  normalizeSourcePlatformScopes,
  readSourcePlatformFlags,
  normalizeSourcePlatformQueryValue,
  normalizeSourcePlatformKey,
  resolvePlatformTypeFromSources,
  resolveResourcePlatformType,
  resolveSourceTypeFromSources,
} from '@/utils/sourcePlatforms';
import {
  AGENT_HOST_PROFILE_IDS,
  PLATFORM_TYPE_KEYS,
  PRESENTATION_ONLY_PLATFORM_IDS,
  SOURCE_AGENT_HOST_PROFILE_HOST_IDENTITY_TOKENS,
  getAgentHostProfileManifestEntry,
  getAgentHostProfileFamily,
  getAgentHostProfileRuntimePlatform,
  getSourcePlatformCanonicalProjections,
  getSourcePlatformReadinessStage,
  getSourcePlatformSurfaceKind,
  getSourcePlatformSupportFloor,
  sourcePlatformIsOwningPlatform,
  sourcePlatformIsRuntimeLens,
  SUPPORTED_OWNING_PLATFORM_IDS,
  SUPPORTED_RUNTIME_LENS_IDS,
} from '@/utils/platformSupportManifest';

describe('sourcePlatforms', () => {
  describe('normalizeSourcePlatformKey', () => {
    it('canonicalizes supported aliases to canonical platform keys', () => {
      expect(normalizeSourcePlatformKey('pve')).toBe('proxmox-pve');
      expect(normalizeSourcePlatformKey('proxmox')).toBe('proxmox-pve');
      expect(normalizeSourcePlatformKey('pbs')).toBe('proxmox-pbs');
      expect(normalizeSourcePlatformKey('pmg')).toBe('proxmox-pmg');
      expect(normalizeSourcePlatformKey('k8s')).toBe('kubernetes');
    });
  });

  describe('getSourcePlatformPresentation', () => {
    it('returns presentation metadata for canonical platforms', () => {
      expect(getSourcePlatformPresentation('proxmox-pve')).toMatchObject({ label: 'PVE' });
      expect(getSourcePlatformPresentation('proxmox-pbs')).toMatchObject({ label: 'PBS' });
      expect(getSourcePlatformPresentation('agent')).toMatchObject({ label: 'Agent' });
      expect(getSourcePlatformPresentation('agent')?.tone).toContain('emerald');
    });

    it('matches governed display tokens from the platform manifest', () => {
      expect(getSourcePlatformPresentation('Unraid')).toMatchObject({ label: 'Unraid' });
      expect(getSourcePlatformPresentation('Proxmox VE')).toMatchObject({ label: 'PVE' });
      expect(getSourcePlatformPresentation('unraid')?.tone).toContain('yellow');
      expect(getSourcePlatformPresentation('proxmox-pve')?.tone).toContain('orange');
    });

    it('returns null for unknown platforms', () => {
      expect(getSourcePlatformPresentation('custom-platform')).toBeNull();
      expect(getSourcePlatformPresentation('')).toBeNull();
      expect(getSourcePlatformPresentation(undefined)).toBeNull();
    });
  });

  describe('getSourcePlatformLabel', () => {
    it('returns label for known platforms', () => {
      expect(getSourcePlatformLabel('docker')).toBe('Docker / Podman');
      expect(getSourcePlatformLabel('kubernetes')).toBe('K8s');
    });

    it('returns canonical labels for normalized aliases', () => {
      expect(getSourcePlatformLabel('proxmox')).toBe('PVE');
      expect(getSourcePlatformLabel('pbs')).toBe('PBS');
      expect(getSourcePlatformLabel('k8s')).toBe('K8s');
    });

    it('returns titleized label for unknown platforms and Unknown for empty input', () => {
      expect(getSourcePlatformLabel('custom-platform')).toBe('Custom Platform');
      expect(getSourcePlatformLabel('')).toBe('Unknown');
      expect(getSourcePlatformLabel('   ')).toBe('Unknown');
      expect(getSourcePlatformLabel(undefined)).toBe('Unknown');
    });
  });

  describe('normalizeSourcePlatformQueryValue', () => {
    it('canonicalizes query values while preserving all and unknown tokens', () => {
      expect(normalizeSourcePlatformQueryValue('pve')).toBe('proxmox-pve');
      expect(normalizeSourcePlatformQueryValue('pbs')).toBe('proxmox-pbs');
      expect(normalizeSourcePlatformQueryValue(' all ')).toBe('all');
      expect(normalizeSourcePlatformQueryValue('custom-platform')).toBe('custom-platform');
      expect(normalizeSourcePlatformQueryValue('')).toBe('');
    });
  });

  describe('normalizeSourcePlatformScopes', () => {
    it('canonicalizes, dedupes, and ignores non-scope tokens', () => {
      expect(
        normalizeSourcePlatformScopes(['docker', 'pve', 'docker', ' all ', '', null], 'truenas'),
      ).toEqual(['docker', 'proxmox-pve']);
    });

    it('falls back to the resolved platform when backend scopes are absent', () => {
      expect(normalizeSourcePlatformScopes(undefined, 'truenas')).toEqual(['truenas']);
      expect(normalizeSourcePlatformScopes([], 'proxmox')).toEqual(['proxmox-pve']);
      expect(normalizeSourcePlatformScopes([], undefined)).toEqual([]);
    });
  });

  describe('readSourcePlatformFlags', () => {
    it('tracks canonical source families from mixed source arrays', () => {
      expect(readSourcePlatformFlags(['agent', 'pbs', 'docker', 'custom-source'])).toEqual({
        hasAgent: true,
        hasProxmox: false,
        hasDocker: true,
        hasKubernetes: false,
        hasPbs: true,
        hasPmg: false,
        hasTrueNAS: false,
        hasVMware: false,
      });
    });

    it('returns an empty flag set for empty input', () => {
      expect(readSourcePlatformFlags()).toEqual({
        hasAgent: false,
        hasProxmox: false,
        hasDocker: false,
        hasKubernetes: false,
        hasPbs: false,
        hasPmg: false,
        hasTrueNAS: false,
        hasVMware: false,
      });
    });

    it('tracks TrueNAS as a canonical API-backed platform source', () => {
      expect(readSourcePlatformFlags(['agent', 'truenas'])).toEqual({
        hasAgent: true,
        hasProxmox: false,
        hasDocker: false,
        hasKubernetes: false,
        hasPbs: false,
        hasPmg: false,
        hasTrueNAS: true,
        hasVMware: false,
      });
    });

    it('tracks VMware as a canonical API-backed platform source', () => {
      expect(readSourcePlatformFlags(['agent', 'vmware'])).toEqual({
        hasAgent: true,
        hasProxmox: false,
        hasDocker: false,
        hasKubernetes: false,
        hasPbs: false,
        hasPmg: false,
        hasTrueNAS: false,
        hasVMware: true,
      });
    });
  });

  describe('governed platform support projection', () => {
    it('keeps VMware on the supported vCenter-backed floor', () => {
      expect(getSourcePlatformReadinessStage('vmware')).toBe('supported');
      expect(getSourcePlatformCanonicalProjections('vmware')).toEqual(['agent', 'vm', 'storage']);
      expect(getSourcePlatformSupportFloor('vmware')).toMatchObject({
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

    it('keeps Unraid as an agent host profile instead of a platform type', () => {
      expect(AGENT_HOST_PROFILE_IDS).toEqual(['unraid']);
      expect(getAgentHostProfileFamily('unraid')).toBe('Unraid');
      expect(getAgentHostProfileRuntimePlatform('unraid')).toBe('linux');
      expect(SOURCE_AGENT_HOST_PROFILE_HOST_IDENTITY_TOKENS.unraid).toEqual([
        'unraid',
        'unraid-os',
        'unraid os',
      ]);
      expect(getAgentHostProfileManifestEntry('unraid-os')?.id).toBe('unraid');
      expect(getAgentHostProfileManifestEntry('Unraid OS')?.id).toBe('unraid');
      expect(PLATFORM_TYPE_KEYS).not.toContain('unraid');
      expect(PRESENTATION_ONLY_PLATFORM_IDS).toContain('unraid');
    });

    it('classifies Docker as a runtime lens instead of an owning platform', () => {
      expect(getSourcePlatformSurfaceKind('docker')).toBe('runtime-lens');
      expect(sourcePlatformIsRuntimeLens('Docker / Podman')).toBe(true);
      expect(sourcePlatformIsOwningPlatform('docker')).toBe(false);
      expect(SUPPORTED_RUNTIME_LENS_IDS).toEqual(['docker']);
      expect(SUPPORTED_OWNING_PLATFORM_IDS).toContain('proxmox-pve');
      expect(SUPPORTED_OWNING_PLATFORM_IDS).not.toContain('docker');
    });
  });

  describe('resolvePlatformTypeFromSources', () => {
    it('prefers canonical platform priority from source arrays', () => {
      expect(resolvePlatformTypeFromSources(['agent', 'docker'])).toBe('docker');
      expect(resolvePlatformTypeFromSources(['agent', 'pbs'])).toBe('proxmox-pbs');
      expect(resolvePlatformTypeFromSources(['pmg', 'docker'])).toBe('proxmox-pmg');
      expect(resolvePlatformTypeFromSources(['agent', 'vmware'])).toBe('vmware-vsphere');
      expect(resolvePlatformTypeFromSources(['agent', 'truenas'])).toBe('truenas');
      expect(resolvePlatformTypeFromSources(['agent'])).toBe('agent');
      expect(resolvePlatformTypeFromSources(['availability'])).toBe('availability');
      expect(resolvePlatformTypeFromSources(['custom-source'])).toBeUndefined();
    });
  });

  describe('resolveResourcePlatformType', () => {
    it('prefers the resource platformType field when present', () => {
      expect(resolveResourcePlatformType({ platformType: 'proxmox-pve', sources: ['agent'] })).toBe(
        'proxmox-pve',
      );
      expect(resolveResourcePlatformType({ platformType: 'docker', sources: [] })).toBe('docker');
    });

    it('falls back to the source-derived platform when platformType is empty', () => {
      expect(resolveResourcePlatformType({ platformType: null, sources: ['docker'] })).toBe(
        'docker',
      );
      expect(resolveResourcePlatformType({ sources: ['kubernetes'] })).toBe('kubernetes');
      expect(resolveResourcePlatformType({ platformType: '', sources: ['truenas', 'agent'] })).toBe(
        'truenas',
      );
      expect(resolveResourcePlatformType({ sources: ['vmware'] })).toBe('vmware-vsphere');
      expect(resolveResourcePlatformType({ sources: ['pbs'] })).toBe('proxmox-pbs');
    });

    it('returns undefined when neither field nor sources resolve a known platform', () => {
      expect(resolveResourcePlatformType({})).toBeUndefined();
      expect(resolveResourcePlatformType({ sources: ['mystery-source'] })).toBeUndefined();
    });
  });

  describe('resolveSourceTypeFromSources', () => {
    it('derives agent, api, and hybrid source modes from source arrays', () => {
      expect(resolveSourceTypeFromSources(['agent'])).toBe('agent');
      expect(resolveSourceTypeFromSources(['pve'])).toBe('api');
      expect(resolveSourceTypeFromSources(['agent', 'proxmox'])).toBe('hybrid');
      expect(resolveSourceTypeFromSources(['agent', 'vmware'])).toBe('hybrid');
      expect(resolveSourceTypeFromSources(['agent', 'truenas'])).toBe('hybrid');
      expect(resolveSourceTypeFromSources(['agent', 'availability'])).toBe('hybrid');
      expect(resolveSourceTypeFromSources(['availability'])).toBe('api');
      expect(resolveSourceTypeFromSources(['custom-source'])).toBe('api');
    });
  });
});
