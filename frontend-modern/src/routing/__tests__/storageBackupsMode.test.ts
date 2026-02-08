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
