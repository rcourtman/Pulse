import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  sessionCanReadInfrastructureSettings,
  sessionSettingsCapabilities,
  syncSessionSettingsCapabilities,
} from '@/stores/sessionSettingsCapabilities';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import type { SecurityStatusSettingsCapabilities } from '@/types/config';

function capabilities(
  overrides: Partial<SecurityStatusSettingsCapabilities>,
): SecurityStatusSettingsCapabilities {
  return {
    infrastructureRead: false,
    systemSettingsRead: false,
    apiAccessRead: false,
    apiAccessWrite: false,
    authenticationRead: false,
    authenticationWrite: false,
    singleSignOnRead: false,
    singleSignOnWrite: false,
    roles: false,
    users: false,
    auditLog: false,
    auditWebhooksRead: false,
    auditWebhooksWrite: false,
    relayRead: false,
    relayWrite: false,
    billingAdmin: false,
    ...overrides,
  } as SecurityStatusSettingsCapabilities;
}

describe('sessionSettingsCapabilities', () => {
  beforeEach(() => {
    syncSessionSettingsCapabilities(null);
  });

  it('keeps infrastructure links available until the capability set resolves', async () => {
    // The signals live at module scope and every other case resolves them, so
    // the pre-bootstrap state is only observable through a fresh module.
    vi.resetModules();
    const fresh = await import('@/stores/sessionSettingsCapabilities');

    expect(fresh.sessionSettingsCapabilitiesResolved()).toBe(false);
    expect(fresh.sessionSettingsCapabilities()).toBeNull();
    // Fail open while unresolved: an admin must not flicker through the
    // restricted copy if the bootstrap fetch is in flight or has failed.
    expect(fresh.sessionCanReadInfrastructureSettings()).toBe(true);
  });

  it('reports infrastructure read access for an admin session', () => {
    syncSessionSettingsCapabilities({
      settingsCapabilities: capabilities({ infrastructureRead: true }),
    });

    expect(sessionCanReadInfrastructureSettings()).toBe(true);
    expect(sessionSettingsCapabilities()?.infrastructureRead).toBe(true);
  });

  it('withholds infrastructure read access for a viewer session', () => {
    syncSessionSettingsCapabilities({
      settingsCapabilities: capabilities({ infrastructureRead: false }),
    });

    expect(sessionCanReadInfrastructureSettings()).toBe(false);
  });

  it('treats a security status without settingsCapabilities as no access', () => {
    syncSessionSettingsCapabilities({ settingsCapabilities: undefined });

    expect(sessionSettingsCapabilities()).toBeNull();
    expect(sessionCanReadInfrastructureSettings()).toBe(false);
  });

  it('is published by the shared presentation-policy sync used at bootstrap', () => {
    // useAppRuntimeState resolves /api/security/status through
    // syncSessionPresentationPolicy on mount; piggybacking there is what makes
    // the capability available to surfaces that never mount Settings.
    syncSessionPresentationPolicy({
      settingsCapabilities: capabilities({ infrastructureRead: true }),
    });

    expect(sessionCanReadInfrastructureSettings()).toBe(true);

    syncSessionPresentationPolicy({
      settingsCapabilities: capabilities({ infrastructureRead: false }),
    });

    expect(sessionCanReadInfrastructureSettings()).toBe(false);
  });
});
