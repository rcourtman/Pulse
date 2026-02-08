import { describe, expect, it } from 'vitest';
import {
  buildStorageBackupsRoutingPlan,
  resolveStorageBackupsDefaultMode,
  resolveStorageBackupsRoutingPlan,
  shouldRedirectBackupsV2Route,
  shouldRedirectStorageV2Route,
} from '@/routing/storageBackupsMode';

describe('storageBackupsMode routing plan', () => {
  it('resolves v2-default mode when full v2 flag is enabled', () => {
    expect(resolveStorageBackupsDefaultMode(true)).toBe('v2-default');
    expect(resolveStorageBackupsRoutingPlan(true)).toEqual({
      mode: 'v2-default',
      showV2DefaultTabs: true,
      primaryStorageView: 'v2',
      primaryBackupsView: 'v2',
    });
  });

  it('resolves v2-default when full v2 is off and no rollback flags are set (default)', () => {
    expect(resolveStorageBackupsDefaultMode(false)).toBe('v2-default');
    expect(resolveStorageBackupsRoutingPlan(false)).toEqual({
      mode: 'v2-default',
      showV2DefaultTabs: true,
      primaryStorageView: 'v2',
      primaryBackupsView: 'v2',
    });
  });

  it('resolves backups-v2-default when storage is rolled back only', () => {
    expect(resolveStorageBackupsDefaultMode(false, true, false)).toBe('backups-v2-default');
    expect(resolveStorageBackupsRoutingPlan(false, true, false)).toEqual({
      mode: 'backups-v2-default',
      showV2DefaultTabs: false,
      primaryStorageView: 'legacy',
      primaryBackupsView: 'v2',
    });
  });

  it('resolves storage-v2-default when backups is rolled back only', () => {
    expect(resolveStorageBackupsDefaultMode(false, false, true)).toBe('storage-v2-default');
    expect(resolveStorageBackupsRoutingPlan(false, false, true)).toEqual({
      mode: 'storage-v2-default',
      showV2DefaultTabs: false,
      primaryStorageView: 'v2',
      primaryBackupsView: 'legacy',
    });
  });

  it('resolves legacy-default when both storage and backups are rolled back', () => {
    expect(resolveStorageBackupsDefaultMode(false, true, true)).toBe('legacy-default');
    expect(resolveStorageBackupsRoutingPlan(false, true, true)).toEqual({
      mode: 'legacy-default',
      showV2DefaultTabs: false,
      primaryStorageView: 'legacy',
      primaryBackupsView: 'legacy',
    });
  });

  it('full v2 flag overrides both rollback flags', () => {
    expect(resolveStorageBackupsDefaultMode(true, true, true)).toBe('v2-default');
    expect(resolveStorageBackupsRoutingPlan(true, true, true)).toEqual({
      mode: 'v2-default',
      showV2DefaultTabs: true,
      primaryStorageView: 'v2',
      primaryBackupsView: 'v2',
    });
  });

  it('builds consistent plan from explicit mode input', () => {
    expect(buildStorageBackupsRoutingPlan('v2-default')).toEqual({
      mode: 'v2-default',
      showV2DefaultTabs: true,
      primaryStorageView: 'v2',
      primaryBackupsView: 'v2',
    });
    expect(buildStorageBackupsRoutingPlan('backups-v2-default')).toEqual({
      mode: 'backups-v2-default',
      showV2DefaultTabs: false,
      primaryStorageView: 'legacy',
      primaryBackupsView: 'v2',
    });
    expect(buildStorageBackupsRoutingPlan('storage-v2-default')).toEqual({
      mode: 'storage-v2-default',
      showV2DefaultTabs: false,
      primaryStorageView: 'v2',
      primaryBackupsView: 'legacy',
    });
    expect(buildStorageBackupsRoutingPlan('legacy-default')).toEqual({
      mode: 'legacy-default',
      showV2DefaultTabs: false,
      primaryStorageView: 'legacy',
      primaryBackupsView: 'legacy',
    });
  });

  it('redirects V2 alias routes only when the primary view is already v2', () => {
    const v2DefaultPlan = buildStorageBackupsRoutingPlan('v2-default');
    expect(shouldRedirectStorageV2Route(v2DefaultPlan)).toBe(true);
    expect(shouldRedirectBackupsV2Route(v2DefaultPlan)).toBe(true);

    const backupsV2DefaultPlan = buildStorageBackupsRoutingPlan('backups-v2-default');
    expect(shouldRedirectStorageV2Route(backupsV2DefaultPlan)).toBe(false);
    expect(shouldRedirectBackupsV2Route(backupsV2DefaultPlan)).toBe(true);

    const storageV2DefaultPlan = buildStorageBackupsRoutingPlan('storage-v2-default');
    expect(shouldRedirectStorageV2Route(storageV2DefaultPlan)).toBe(true);
    expect(shouldRedirectBackupsV2Route(storageV2DefaultPlan)).toBe(false);

    const legacyDefaultPlan = buildStorageBackupsRoutingPlan('legacy-default');
    expect(shouldRedirectStorageV2Route(legacyDefaultPlan)).toBe(false);
    expect(shouldRedirectBackupsV2Route(legacyDefaultPlan)).toBe(false);
  });
});

describe('GA contract regression gates', () => {
  it('GA default: resolving with all-false inputs produces v2-default', () => {
    expect(resolveStorageBackupsDefaultMode(false, false, false)).toBe('v2-default');
  });

  it('GA default: v2-default plan always selects v2 primary views', () => {
    const plan = buildStorageBackupsRoutingPlan('v2-default');

    expect(plan.primaryStorageView).toBe('v2');
    expect(plan.primaryBackupsView).toBe('v2');
  });

  it('GA default: v2-default plan redirects both V2 alias routes', () => {
    const plan = buildStorageBackupsRoutingPlan('v2-default');

    expect(shouldRedirectStorageV2Route(plan)).toBe(true);
    expect(shouldRedirectBackupsV2Route(plan)).toBe(true);
  });

  it('rollback isolation: rollback flags do not leak across views', () => {
    const storageRolledBackPlan = resolveStorageBackupsRoutingPlan(false, true, false);
    expect(storageRolledBackPlan.primaryStorageView).toBe('legacy');
    expect(storageRolledBackPlan.primaryBackupsView).toBe('v2');

    const backupsRolledBackPlan = resolveStorageBackupsRoutingPlan(false, false, true);
    expect(backupsRolledBackPlan.primaryStorageView).toBe('v2');
    expect(backupsRolledBackPlan.primaryBackupsView).toBe('legacy');
  });
});
