import { describe, expect, it } from 'vitest';
import {
  getSourcePlatformLabel,
  getSourcePlatformPresentation,
  readSourcePlatformFlags,
  normalizeSourcePlatformQueryValue,
  normalizeSourcePlatformKey,
  resolvePlatformTypeFromSources,
  resolveSourceTypeFromSources,
} from '@/utils/sourcePlatforms';

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

    it('returns null for unknown platforms', () => {
      expect(getSourcePlatformPresentation('custom-platform')).toBeNull();
      expect(getSourcePlatformPresentation('')).toBeNull();
      expect(getSourcePlatformPresentation(undefined)).toBeNull();
    });
  });

  describe('getSourcePlatformLabel', () => {
    it('returns label for known platforms', () => {
      expect(getSourcePlatformLabel('docker')).toBe('Containers');
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

  describe('readSourcePlatformFlags', () => {
    it('tracks canonical source families from mixed source arrays', () => {
      expect(readSourcePlatformFlags(['agent', 'pbs', 'docker', 'custom-source'])).toEqual({
        hasAgent: true,
        hasProxmox: false,
        hasDocker: true,
        hasKubernetes: false,
        hasPbs: true,
        hasPmg: false,
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
      });
    });
  });

  describe('resolvePlatformTypeFromSources', () => {
    it('prefers canonical platform priority from source arrays', () => {
      expect(resolvePlatformTypeFromSources(['agent', 'docker'])).toBe('docker');
      expect(resolvePlatformTypeFromSources(['agent', 'pbs'])).toBe('proxmox-pbs');
      expect(resolvePlatformTypeFromSources(['pmg', 'docker'])).toBe('proxmox-pmg');
      expect(resolvePlatformTypeFromSources(['agent'])).toBe('agent');
      expect(resolvePlatformTypeFromSources(['custom-source'])).toBeUndefined();
    });
  });

  describe('resolveSourceTypeFromSources', () => {
    it('derives agent, api, and hybrid source modes from source arrays', () => {
      expect(resolveSourceTypeFromSources(['agent'])).toBe('agent');
      expect(resolveSourceTypeFromSources(['pve'])).toBe('api');
      expect(resolveSourceTypeFromSources(['agent', 'proxmox'])).toBe('hybrid');
      expect(resolveSourceTypeFromSources(['custom-source'])).toBe('api');
    });
  });
});
