import { describe, expect, it } from 'vitest';
import {
  deriveAgentFromPath,
  deriveTabFromPath,
  deriveTabFromQuery,
  resolveCanonicalSettingsPath,
  settingsTabPath,
} from '../settingsRouting';

describe('resolveCanonicalSettingsPath', () => {
  it('handles empty cases', () => {
    expect(resolveCanonicalSettingsPath('')).toBe('/settings');
    expect(resolveCanonicalSettingsPath('not-a-path')).toBeNull();
  });

  it('drops trailing slashes', () => {
    expect(resolveCanonicalSettingsPath('/settings/')).toBe('/settings');
    expect(resolveCanonicalSettingsPath('/settings/workspace/')).toBe('/settings/workspace');
  });

  it('maps legacy platform proxies to Proxmox equivalents', () => {
    expect(resolveCanonicalSettingsPath('/settings/pve')).toBe('/settings/infrastructure/pve');
    expect(resolveCanonicalSettingsPath('/settings/pbs')).toBe('/settings/infrastructure/pbs');
    expect(resolveCanonicalSettingsPath('/settings/pmg')).toBe('/settings/infrastructure/pmg');
  });
  
  it('maps remaining legacy endpoints to their new taxonomy', () => {
    expect(resolveCanonicalSettingsPath('/settings/system-general')).toBe('/settings/workspace');
    expect(resolveCanonicalSettingsPath('/settings/system-network')).toBe('/settings/integrations');
  });

  it('keeps canonical routes as they are', () => {
    expect(resolveCanonicalSettingsPath('/settings/infrastructure/pve')).toBe('/settings/infrastructure/pve');
    expect(resolveCanonicalSettingsPath('/settings/integrations')).toBe('/settings/integrations');
  });
});

describe('deriveTabFromPath', () => {
  it('falls back to proxmox if unknown component mounted', () => {
    expect(deriveTabFromPath('/settings')).toBe('proxmox');
    expect(deriveTabFromPath('/settings/unknown')).toBe('proxmox');
  });

  it('handles infrastructure routing', () => {
    expect(deriveTabFromPath('/settings/infrastructure')).toBe('proxmox');
    expect(deriveTabFromPath('/settings/proxmox')).toBe('proxmox');
    expect(deriveTabFromPath('/settings/storage')).toBe('proxmox');
    expect(deriveTabFromPath('/settings/pve')).toBe('proxmox');
  });

  it('handles workload routing', () => {
    expect(deriveTabFromPath('/settings/workloads')).toBe('agents');
    expect(deriveTabFromPath('/settings/workloads/docker')).toBe('docker');
    expect(deriveTabFromPath('/settings/docker')).toBe('docker');
    expect(deriveTabFromPath('/settings/agents')).toBe('agents');
    expect(deriveTabFromPath('/settings/linuxServers')).toBe('agents');
  });

  it('handles system unified routes', () => {
    expect(deriveTabFromPath('/settings/workspace')).toBe('workspace');
    expect(deriveTabFromPath('/settings/system-general')).toBe('workspace');
    expect(deriveTabFromPath('/settings/integrations')).toBe('integrations');
    expect(deriveTabFromPath('/settings/api')).toBe('integrations');
    expect(deriveTabFromPath('/settings/maintenance')).toBe('maintenance');
    expect(deriveTabFromPath('/settings/system-updates')).toBe('maintenance');
    expect(deriveTabFromPath('/settings/system-recovery')).toBe('maintenance');
  });

  it('handles security and organization unified routes', () => {
    expect(deriveTabFromPath('/settings/authentication')).toBe('authentication');
    expect(deriveTabFromPath('/settings/security-overview')).toBe('authentication');
    expect(deriveTabFromPath('/settings/security-sso')).toBe('authentication');
    expect(deriveTabFromPath('/settings/team')).toBe('team');
    expect(deriveTabFromPath('/settings/organization')).toBe('team');
    expect(deriveTabFromPath('/settings/security-roles')).toBe('team');
    expect(deriveTabFromPath('/settings/audit')).toBe('audit');
    expect(deriveTabFromPath('/settings/security-audit')).toBe('audit');
  });
});

describe('deriveAgentFromPath', () => {
  it('correctly derives agent sub-tabs for infrastructure pages', () => {
    expect(deriveAgentFromPath('/settings/infrastructure/pve')).toBe('pve');
    expect(deriveAgentFromPath('/settings/pve')).toBe('pve');
    expect(deriveAgentFromPath('/settings/storage')).toBe('pbs');
    expect(deriveAgentFromPath('/settings/infrastructure')).toBeNull();
  });
});

describe('deriveTabFromQuery', () => {
  it('handles empty cases', () => {
    expect(deriveTabFromQuery('')).toBeNull();
    expect(deriveTabFromQuery('?other=foo')).toBeNull();
  });

  it('derives from modern parameters', () => {
    expect(deriveTabFromQuery('?tab=workspace')).toBe('workspace');
    expect(deriveTabFromQuery('?tab=integrations')).toBe('integrations');
    expect(deriveTabFromQuery('?tab=authentication')).toBe('authentication');
  });

  it('maps legacy query parameters to new taxonomy tabs', () => {
    expect(deriveTabFromQuery('?tab=general')).toBe('workspace');
    expect(deriveTabFromQuery('?tab=network')).toBe('integrations');
    expect(deriveTabFromQuery('?tab=api')).toBe('integrations');
    expect(deriveTabFromQuery('?tab=updates')).toBe('maintenance');
    expect(deriveTabFromQuery('?tab=recovery')).toBe('maintenance');
    expect(deriveTabFromQuery('?tab=security-auth')).toBe('authentication');
    expect(deriveTabFromQuery('?tab=security-users')).toBe('team');
    expect(deriveTabFromQuery('?tab=security-audit')).toBe('audit');
  });
});

describe('settingsTabPath', () => {
  it('returns clean paths for platform roots', () => {
    expect(settingsTabPath('proxmox')).toBe('/settings/infrastructure');
    expect(settingsTabPath('agents')).toBe('/settings/workloads');
    expect(settingsTabPath('docker')).toBe('/settings/workloads/docker');
  });

  it('returns unified paths', () => {
    expect(settingsTabPath('workspace')).toBe('/settings/workspace');
    expect(settingsTabPath('integrations')).toBe('/settings/integrations');
    expect(settingsTabPath('maintenance')).toBe('/settings/maintenance');
    expect(settingsTabPath('authentication')).toBe('/settings/authentication');
    expect(settingsTabPath('team')).toBe('/settings/team');
    expect(settingsTabPath('audit')).toBe('/settings/audit');
  });
});
