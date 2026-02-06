import { describe, expect, it } from 'vitest';
import {
  deriveAgentFromPath,
  deriveTabFromPath,
  deriveTabFromQuery,
  settingsTabPath,
} from '../settingsRouting';

describe('settingsRouting', () => {
  it('maps legacy updates route to system updates tab', () => {
    expect(deriveTabFromPath('/settings/updates')).toBe('system-updates');
    expect(deriveTabFromPath('/settings/system-updates')).toBe('system-updates');
  });

  it('maps unified resource aliases', () => {
    expect(deriveTabFromPath('/settings/infrastructure')).toBe('proxmox');
    expect(deriveTabFromPath('/settings/workloads')).toBe('agents');
    expect(deriveTabFromPath('/settings/workloads/docker')).toBe('docker');
    expect(deriveTabFromPath('/settings/backups')).toBe('system-backups');
  });

  it('maps security fallback paths', () => {
    expect(deriveTabFromPath('/settings/security')).toBe('security-overview');
    expect(deriveTabFromPath('/settings/security-auth')).toBe('security-auth');
  });

  it('maps legacy agent sections', () => {
    expect(deriveAgentFromPath('/settings/pve')).toBe('pve');
    expect(deriveAgentFromPath('/settings/pbs')).toBe('pbs');
    expect(deriveAgentFromPath('/settings/storage')).toBe('pbs');
  });

  it('maps query params for deep-links', () => {
    expect(deriveTabFromQuery('?tab=security')).toBe('security-overview');
    expect(deriveTabFromQuery('?tab=updates')).toBe('system-updates');
    expect(deriveTabFromQuery('?tab=workloads')).toBe('agents');
    expect(deriveTabFromQuery('?tab=unknown')).toBeNull();
  });

  it('returns canonical paths for tabs', () => {
    expect(settingsTabPath('proxmox')).toBe('/settings/infrastructure');
    expect(settingsTabPath('agents')).toBe('/settings/workloads');
    expect(settingsTabPath('system-backups')).toBe('/settings/backups');
    expect(settingsTabPath('security-overview')).toBe('/settings/security-overview');
  });
});
